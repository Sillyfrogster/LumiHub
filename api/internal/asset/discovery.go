package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) SetDiscovery(
	ctx context.Context,
	ownerID uuid.UUID,
	id uuid.UUID,
	discovery Discovery,
) error {
	if !discovery.Valid() {
		return ErrInvalidDiscovery
	}

	queries := db.New(s.pool)
	changed, err := queries.SetAssetDiscovery(ctx, db.SetAssetDiscoveryParams{
		ID:        uuidToPgtype(id),
		OwnerID:   uuidToPgtype(ownerID),
		Discovery: string(discovery),
	})
	if err != nil {
		return fmt.Errorf("set asset discovery: %w", err)
	}
	if changed == 1 {
		return nil
	}

	state, err := queries.AssetStateForOwner(ctx, db.AssetStateForOwnerParams{
		ID:      uuidToPgtype(id),
		OwnerID: uuidToPgtype(ownerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check asset discovery: %w", err)
	}
	if state.WithheldAt.Valid {
		return ErrAssetFrozen
	}
	if Lifecycle(state.Lifecycle) == LifecycleDraft {
		return ErrAssetIsDraft
	}
	return ErrNotFound
}
