package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrNotDeliverable is an asset a linked instance may not be handed.
var ErrNotDeliverable = errors.New("that asset cannot be sent to an instance")

// DeliveryTarget is one format an asset is offered in, as delivery reads it.
type DeliveryTarget struct {
	Format string
	Label  string
}

// DeliveryPicture is one of an asset's images behind a short-lived signed URL.
type DeliveryPicture struct {
	MediaID uuid.UUID
	Role    string
	IsCover bool
	URL     string
}

// Deliverable is what delivery reads of an asset, with its formats and its pictures.
type Deliverable struct {
	Kind              string
	Name              string
	ContentGeneration int
	Targets           []DeliveryTarget
	HasOriginal       bool
	Pictures          []DeliveryPicture
}

// DeliverableAsset reads one sendable asset through the connection the caller already holds.
func (s *Service) DeliverableAsset(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
) (Deliverable, error) {
	var found Deliverable
	var generation int32
	var revisionID, coverID pgtype.UUID
	err := q.QueryRow(ctx, `
		select kind, name, content_generation, current_revision_id, cover_media_id
		  from assets
		 where id = $1
		   and deleted_at is null
		   and withheld_at is null
		   and lifecycle = 'published'
	`, assetID).Scan(&found.Kind, &found.Name, &generation, &revisionID, &coverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deliverable{}, ErrNotDeliverable
	}
	if err != nil {
		return Deliverable{}, fmt.Errorf("read the asset to deliver: %w", err)
	}
	found.ContentGeneration = int(generation)
	found.HasOriginal = revisionID.Valid

	offered, err := deliveryTargets(ctx, q, assetID)
	if err != nil {
		return Deliverable{}, err
	}
	found.Targets = offered
	found.Pictures, err = s.deliveryPictures(ctx, q, assetID, uuidOrNil(coverID))
	if err != nil {
		return Deliverable{}, err
	}
	return found, nil
}

// deliveryTargets reads the same projection a download reads, so both offer the same formats.
func deliveryTargets(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
) ([]DeliveryTarget, error) {
	var stored []byte
	err := q.QueryRow(ctx,
		`select export from asset_projections where asset_id = $1`, assetID,
	).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return []DeliveryTarget{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the export projection to deliver: %w", err)
	}
	offered := make([]format.Target, 0)
	if err := json.Unmarshal(stored, &offered); err != nil {
		return nil, fmt.Errorf("read the stored export projection: %w", err)
	}
	targets := make([]DeliveryTarget, 0, len(offered))
	for _, target := range offered {
		targets = append(targets, DeliveryTarget{Format: target.Format, Label: target.Label})
	}
	return targets, nil
}

// deliveryPictures lists every current image, because no format carries all of them.
func (s *Service) deliveryPictures(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
	coverID *uuid.UUID,
) ([]DeliveryPicture, error) {
	rows, err := q.Query(ctx, `
		select id, role
		  from asset_media
		 where asset_id = $1
		   and is_current
		   and blob_id is not null
		 order by created_at desc, id desc
	`, assetID)
	if err != nil {
		return nil, fmt.Errorf("list the pictures to deliver: %w", err)
	}
	defer rows.Close()
	pictures := make([]DeliveryPicture, 0)
	for rows.Next() {
		var picture DeliveryPicture
		if err := rows.Scan(&picture.MediaID, &picture.Role); err != nil {
			return nil, fmt.Errorf("read a picture to deliver: %w", err)
		}
		picture.IsCover = coverID != nil && *coverID == picture.MediaID
		picture.URL = s.exportMediaURL(picture.MediaID, true)
		pictures = append(pictures, picture)
	}
	return pictures, rows.Err()
}

// SignedURL stamps a private path with a short-lived signature and makes it absolute.
func (s *Service) SignedURL(path string) string {
	return s.siteURL + s.signer.Sign(path, s.now())
}

// ValidSignature reports whether a request carries a live signature written for this path.
func (s *Service) ValidSignature(path, expires, signature string) bool {
	return s.signer.Valid(path, expires, signature, s.now())
}

// DownloadSourceForLinkedInstance prepares the creator's own file for a raw delivery.
func (s *Service) DownloadSourceForLinkedInstance(
	ctx context.Context,
	assetID uuid.UUID,
) (SourceDownload, error) {
	download, err := s.DownloadSource(ctx, assetID, nil)
	if err != nil {
		return SourceDownload{}, err
	}
	download.Event.AuthorizationClass = AuthorizationLinkedInstance
	return download, nil
}
