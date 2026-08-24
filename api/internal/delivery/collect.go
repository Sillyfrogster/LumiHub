package delivery

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/linking"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// holdMargin keeps the wait inside the route deadline, so an empty wait ends in an answer.
const holdMargin = 2 * time.Second

// Collect acknowledges finished work and waits briefly for more, holding no database connection.
func (s *Service) Collect(
	ctx context.Context,
	instance linking.Instance,
	acknowledged []uuid.UUID,
) ([]Work, error) {
	if len(acknowledged) > s.settings.MaxAcknowledged {
		return nil, ErrAcknowledgement
	}
	if err := s.instances.Throttle(
		ctx, actionCollect, instance.ID.String(), collectLimit, time.Hour,
	); err != nil {
		return nil, throttled(err)
	}
	if len(acknowledged) > 0 {
		if _, err := db.New(s.pool).AcknowledgeDeliveries(ctx, db.AcknowledgeDeliveriesParams{
			InstanceID: uuidValue(instance.ID), DeliveryIds: uuidValues(acknowledged),
		}); err != nil {
			return nil, fmt.Errorf("acknowledge deliveries: %w", err)
		}
	}

	held, admitted := s.waiting.hold(instance.ID)
	if !admitted {
		return nil, ErrTooManyCollectors
	}
	defer s.waiting.release(instance.ID, held)

	recheck := time.NewTicker(s.settings.Recheck)
	defer recheck.Stop()
	waitedOut := time.NewTimer(s.hold(ctx))
	defer waitedOut.Stop()
	for {
		work, err := s.claim(ctx, instance)
		if err != nil {
			return nil, err
		}
		if len(work) > 0 {
			return work, nil
		}
		select {
		case <-held.work:
		case <-recheck.C:
		case <-held.superseded:
			return []Work{}, nil
		case <-waitedOut.C:
			return []Work{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// hold is this request's wait, jittered so a fleet does not stay in step and clamped to the deadline.
func (s *Service) hold(ctx context.Context) time.Duration {
	spread := s.settings.HoldCeiling - s.settings.HoldFloor
	wait := s.settings.HoldFloor
	if spread > 0 {
		wait += rand.N(spread)
	}
	deadline, set := ctx.Deadline()
	if !set {
		return wait
	}
	allowed := time.Until(deadline) - holdMargin
	if allowed < wait {
		wait = allowed
	}
	if wait < 0 {
		wait = 0
	}
	return wait
}

// claim takes waiting and lease-expired work, deciding authorization again at the moment of release.
func (s *Service) claim(ctx context.Context, instance linking.Instance) ([]Work, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin a delivery claim: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	if _, err := queries.AbandonExhaustedDeliveries(ctx, db.AbandonExhaustedDeliveriesParams{
		InstanceID: uuidValue(instance.ID), MaxAttempts: int32(s.settings.MaxAttempts),
	}); err != nil {
		return nil, fmt.Errorf("abandon exhausted deliveries: %w", err)
	}
	claimed, err := queries.ClaimDeliveries(ctx, db.ClaimDeliveriesParams{
		LeaseExpiresAt: timestamptz(s.now().Add(s.settings.Lease)),
		InstanceID:     uuidValue(instance.ID),
		MaxAttempts:    int32(s.settings.MaxAttempts),
		BatchSize:      int32(s.settings.Batch),
	})
	if err != nil {
		return nil, fmt.Errorf("claim deliveries: %w", err)
	}
	work := make([]Work, 0, len(claimed))
	for _, row := range claimed {
		released, err := s.release(ctx, tx, instance, row)
		if err != nil {
			return nil, err
		}
		if released != nil {
			work = append(work, *released)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit a delivery claim: %w", err)
	}
	return work, nil
}

func (s *Service) release(
	ctx context.Context,
	tx pgx.Tx,
	instance linking.Instance,
	row db.ClaimDeliveriesRow,
) (*Work, error) {
	queries := db.New(tx)
	deliveryID := uuid.UUID(row.ID.Bytes)
	assetID := uuid.UUID(row.AssetID.Bytes)
	sendable, err := s.catalog.DeliverableAsset(ctx, tx, assetID)
	if errors.Is(err, asset.ErrNotDeliverable) || errors.Is(err, pgx.ErrNoRows) {
		return nil, stop(ctx, queries, row.ID, ReasonWithdrawn)
	}
	if err != nil {
		return nil, err
	}
	target, label, chosen := chooseTarget(
		instance.AcceptedTargets, sendable.Targets, sendable.HasOriginal,
	)
	if !chosen {
		return nil, stop(ctx, queries, row.ID, ReasonUnsupported)
	}
	if err := queries.SetDeliveryTarget(ctx, db.SetDeliveryTargetParams{
		ChosenTarget: textValue(target), ID: row.ID,
	}); err != nil {
		return nil, fmt.Errorf("record the chosen format: %w", err)
	}
	return &Work{
		ID: deliveryID, AssetID: assetID,
		ContentGeneration: sendable.ContentGeneration,
		Kind:              sendable.Kind, Name: sendable.Name,
		Format: target, Label: label,
		QueuedAt: row.QueuedAt.Time, LeaseExpiresAt: row.LeaseExpiresAt.Time,
		Artifacts: s.artifacts(deliveryID, sendable),
	}, nil
}

// stop settles a delivery that will never arrive, so a creator sees why rather than waiting on it.
func stop(
	ctx context.Context,
	queries *db.Queries,
	id pgtype.UUID,
	reason Reason,
) error {
	if err := queries.FailDelivery(ctx, db.FailDeliveryParams{
		SettledReason: textValue(string(reason)), ID: id,
	}); err != nil {
		return fmt.Errorf("stop a delivery: %w", err)
	}
	return nil
}

// artifacts names the export and every picture beside it, since no format carries them all.
func (s *Service) artifacts(deliveryID uuid.UUID, sendable asset.Deliverable) []Artifact {
	artifacts := make([]Artifact, 0, len(sendable.Pictures)+1)
	artifacts = append(artifacts, Artifact{
		Kind: ArtifactExport,
		URL:  s.catalog.SignedURL(deliveryPathStart + deliveryID.String() + "/export"),
	})
	for _, picture := range sendable.Pictures {
		mediaID := picture.MediaID
		artifacts = append(artifacts, Artifact{
			Kind: ArtifactPicture, URL: picture.URL, MediaID: &mediaID,
			Role: picture.Role, IsCover: picture.IsCover,
		})
	}
	return artifacts
}

func uuidValues(values []uuid.UUID) []pgtype.UUID {
	converted := make([]pgtype.UUID, len(values))
	for index, value := range values {
		converted[index] = uuidValue(value)
	}
	return converted
}

func textValue(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}
