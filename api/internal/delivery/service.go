package delivery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/linking"
	"github.com/Sillyfrogster/Illarin/api/internal/signing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Catalog is the whole of what delivery knows about an asset, so visibility stays one decision.
type Catalog interface {
	DeliverableAsset(ctx context.Context, q db.DBTX, assetID uuid.UUID) (asset.Deliverable, error)
	SignedURL(path string) string
	ValidSignature(path, expires, signature string) bool
}

// Instances is everything delivery needs from the linking layer.
type Instances interface {
	Live(ctx context.Context, userID uuid.UUID) ([]linking.Instance, error)
	LiveByID(ctx context.Context, userID, instanceID uuid.UUID) (linking.Instance, error)
	Throttle(ctx context.Context, action, source string, limit int32, window time.Duration) error
}

// Settings are the bounds the queue runs under, each a wall rather than a target.
type Settings struct {
	HoldFloor          time.Duration
	HoldCeiling        time.Duration
	Recheck            time.Duration
	Lease              time.Duration
	Retention          time.Duration
	SweepInterval      time.Duration
	Batch              int
	MaxAttempts        int
	PendingPerInstance int
	ConcurrentHolds    int
	MaxAcknowledged    int
	MaxLibraryEntries  int
}

// DefaultSettings match the lease to a signature's life, so a delivery and its addresses expire together.
func DefaultSettings() Settings {
	return Settings{
		HoldFloor:          25 * time.Second,
		HoldCeiling:        30 * time.Second,
		Recheck:            5 * time.Second,
		Lease:              signing.Life,
		Retention:          7 * 24 * time.Hour,
		SweepInterval:      5 * time.Minute,
		Batch:              10,
		MaxAttempts:        5,
		PendingPerInstance: 100,
		ConcurrentHolds:    512,
		MaxAcknowledged:    32,
		MaxLibraryEntries:  2000,
	}
}

// The named limits every caller of these routes is counted against.
const (
	actionQueue       = "delivery-queue"
	actionCollect     = "delivery-collect"
	actionSyncPart    = "library-sync"
	actionSyncWhole   = "library-snapshot"
	queueLimit        = 120
	collectLimit      = 400
	syncPartLimit     = 120
	syncWholeLimit    = 24
	deliveryPathStart = "/delivery/"
)

// Service owns the delivery queue and the mirror of what instances installed.
type Service struct {
	pool      *pgxpool.Pool
	catalog   Catalog
	instances Instances
	settings  Settings
	waiting   *hub
	now       func() time.Time
}

func NewService(
	pool *pgxpool.Pool,
	catalog Catalog,
	instances Instances,
	settings Settings,
) *Service {
	return &Service{
		pool: pool, catalog: catalog, instances: instances, settings: settings,
		waiting: newHub(settings.ConcurrentHolds), now: time.Now,
	}
}

// Queue sends one asset to one of the creator's own instances, and a second press returns the first.
func (s *Service) Queue(
	ctx context.Context,
	userID uuid.UUID,
	instanceID uuid.UUID,
	assetID uuid.UUID,
) (Delivery, error) {
	if err := s.instances.Throttle(
		ctx, actionQueue, userID.String(), queueLimit, time.Hour,
	); err != nil {
		return Delivery{}, throttled(err)
	}
	instance, err := s.instances.LiveByID(ctx, userID, instanceID)
	if errors.Is(err, linking.ErrInstanceNotFound) {
		return Delivery{}, ErrInstanceNotFound
	}
	if err != nil {
		return Delivery{}, err
	}
	if !instance.Grants(linking.ScopeReceiveAssets) {
		return Delivery{}, ErrMissingScope
	}
	sendable, err := s.catalog.DeliverableAsset(ctx, s.pool, assetID)
	if errors.Is(err, asset.ErrNotDeliverable) {
		return Delivery{}, ErrAssetNotSendable
	}
	if err != nil {
		return Delivery{}, err
	}
	if _, _, chosen := chooseTarget(
		instance.AcceptedTargets, sendable.Targets, sendable.HasOriginal,
	); !chosen {
		return Delivery{}, ErrNoTarget
	}

	queries := db.New(s.pool)
	waiting, err := queries.CountLiveDeliveries(ctx, uuidValue(instanceID))
	if err != nil {
		return Delivery{}, fmt.Errorf("count waiting deliveries: %w", err)
	}
	if waiting >= int64(s.settings.PendingPerInstance) {
		return Delivery{}, ErrQueueFull
	}
	row, err := queries.QueueDelivery(ctx, db.QueueDeliveryParams{
		ID: uuidValue(uuid.New()), InstanceID: uuidValue(instanceID),
		AssetID:   uuidValue(assetID),
		ExpiresAt: timestamptz(s.now().Add(s.settings.Retention)),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.liveDelivery(ctx, instanceID, assetID)
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("queue a delivery: %w", err)
	}
	s.waiting.signal(instanceID)
	return deliveryFrom(
		row.ID, row.InstanceID, row.AssetID, row.State, row.SettledReason,
		row.QueuedAt, row.ExpiresAt,
	), nil
}

func (s *Service) liveDelivery(
	ctx context.Context,
	instanceID uuid.UUID,
	assetID uuid.UUID,
) (Delivery, error) {
	row, err := db.New(s.pool).LiveDeliveryForAsset(ctx, db.LiveDeliveryForAssetParams{
		InstanceID: uuidValue(instanceID), AssetID: uuidValue(assetID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Delivery{}, ErrDeliveryNotFound
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("read the waiting delivery: %w", err)
	}
	return deliveryFrom(
		row.ID, row.InstanceID, row.AssetID, row.State, row.SettledReason,
		row.QueuedAt, row.ExpiresAt,
	), nil
}

// Discard drops one of the creator's own deliveries, and its signed addresses stop answering with it.
func (s *Service) Discard(ctx context.Context, userID, deliveryID uuid.UUID) error {
	discarded, err := db.New(s.pool).DiscardDelivery(ctx, db.DiscardDeliveryParams{
		DeliveryID: uuidValue(deliveryID), UserID: uuidValue(userID),
	})
	if err != nil {
		return fmt.Errorf("discard a delivery: %w", err)
	}
	if discarded == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// AssetInstances is what an asset page shows, and an asset nobody may be sent is absent rather than described.
func (s *Service) AssetInstances(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
) (AssetInstances, error) {
	queries := db.New(s.pool)
	generation, err := queries.SendableAssetGeneration(ctx, uuidValue(assetID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AssetInstances{}, ErrAssetNotFound
	}
	if err != nil {
		return AssetInstances{}, fmt.Errorf("read the asset to send: %w", err)
	}
	sendable, err := s.catalog.DeliverableAsset(ctx, s.pool, assetID)
	if errors.Is(err, asset.ErrNotDeliverable) {
		return AssetInstances{}, ErrAssetNotFound
	}
	if err != nil {
		return AssetInstances{}, fmt.Errorf("read available delivery formats: %w", err)
	}
	rows, err := queries.AssetInstanceStates(ctx, db.AssetInstanceStatesParams{
		AssetID: uuidValue(assetID), UserID: uuidValue(userID),
	})
	if err != nil {
		return AssetInstances{}, fmt.Errorf("read instance state for an asset: %w", err)
	}
	found := AssetInstances{ContentGeneration: int(generation), Items: []InstanceState{}}
	for _, row := range rows {
		_, _, canReceive := chooseTarget(
			row.AcceptedTargets, sendable.Targets, sendable.HasOriginal,
		)
		state := InstanceState{
			InstanceID:      uuid.UUID(row.ID.Bytes),
			ApplicationName: row.ApplicationName,
			InstanceName:    row.InstanceName,
			LastSeenAt:      optionalTime(row.LastSeenAt),
			CanReceive:      holdsScope(row.Scopes, linking.ScopeReceiveAssets) && canReceive,
			ReportsLibrary:  holdsScope(row.Scopes, linking.ScopeSyncLibrary),
		}
		if row.DeliveryID.Valid {
			waiting := deliveryFrom(
				row.DeliveryID, row.ID, uuidValue(assetID), row.DeliveryState,
				row.SettledReason, row.QueuedAt, row.ExpiresAt,
			)
			state.Delivery = &waiting
		}
		if row.InstalledGeneration.Valid {
			installed := int(row.InstalledGeneration.Int32)
			state.InstalledGeneration = &installed
			state.UpdateAvailable = installed < found.ContentGeneration
		}
		found.Items = append(found.Items, state)
	}
	return found, nil
}

func holdsScope(scopes []string, wanted linking.Scope) bool {
	for _, scope := range scopes {
		if linking.Scope(scope) == wanted {
			return true
		}
	}
	return false
}

func deliveryFrom(
	id pgtype.UUID,
	instanceID pgtype.UUID,
	assetID pgtype.UUID,
	state string,
	reason pgtype.Text,
	queuedAt pgtype.Timestamptz,
	expiresAt pgtype.Timestamptz,
) Delivery {
	return Delivery{
		ID: uuid.UUID(id.Bytes), InstanceID: uuid.UUID(instanceID.Bytes),
		AssetID: uuid.UUID(assetID.Bytes), State: State(state),
		Reason: Reason(reason.String), QueuedAt: queuedAt.Time,
		ExpiresAt: expiresAt.Time,
	}
}

func throttled(err error) error {
	if errors.Is(err, linking.ErrTooManyRequests) {
		return fmt.Errorf("%w: %w", ErrTooManyRequests, err)
	}
	return err
}

func uuidValue(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	moment := value.Time
	return &moment
}
