package asset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const recoveryWindow = 30 * 24 * time.Hour

type DeletedAsset struct {
	ID               uuid.UUID
	Name             string
	Kind             string
	DeletedAt        time.Time
	RecoverableUntil time.Time
}

func (s *Service) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	queries := db.New(s.pool)
	state, err := queries.AssetDeletionState(ctx, db.AssetDeletionStateParams{
		ID: uuidToPgtype(id), OwnerID: uuidToPgtype(ownerID),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && state.DeletedAt.Valid) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read asset deletion state: %w", err)
	}
	if state.WithheldAt.Valid {
		return ErrAssetFrozen
	}
	now := s.now()
	changed, err := queries.SoftDeleteAsset(ctx, db.SoftDeleteAssetParams{
		ID: uuidToPgtype(id), OwnerID: uuidToPgtype(ownerID),
		DeletedAt: timeToNullable(&now), RecoverableUntil: timeToNullable(timePointer(now.Add(recoveryWindow))),
	})
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Restore(ctx context.Context, ownerID, id uuid.UUID) error {
	now := s.now()
	changed, err := db.New(s.pool).RestoreAsset(ctx, db.RestoreAssetParams{
		ID: uuidToPgtype(id), OwnerID: uuidToPgtype(ownerID), UpdatedAt: timeToNullable(&now),
	})
	if err != nil {
		return fmt.Errorf("restore asset: %w", err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Deleted(ctx context.Context, ownerID uuid.UUID, handle string) ([]DeletedAsset, error) {
	rows, err := db.New(s.pool).ListDeletedAssets(ctx, db.ListDeletedAssetsParams{
		OwnerID: uuidToPgtype(ownerID), Username: handle,
		RecoverableUntil: timeToNullable(timePointer(s.now())),
	})
	if err != nil {
		return nil, fmt.Errorf("list deleted assets: %w", err)
	}
	items := make([]DeletedAsset, len(rows))
	for i, row := range rows {
		items[i] = DeletedAsset{
			ID: uuidFromPgtype(row.ID), Name: row.Name, Kind: row.Kind,
			DeletedAt: timeFromPgtype(row.DeletedAt), RecoverableUntil: timeFromPgtype(row.RecoverableUntil),
		}
	}
	return items, nil
}

func timePointer(value time.Time) *time.Time { return &value }
