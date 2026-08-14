package asset

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrPatchUnsupported = errors.New("asset format does not support file patches")

// FilePatchInput replaces one creator's current patch for an asset.
type FilePatchInput struct {
	OwnerID uuid.UUID
	AssetID uuid.UUID
	Patch   format.Patch
}

// SetFilePatch validates and replaces the creator-authored file patch.
func (s *Service) SetFilePatch(ctx context.Context, input FilePatchInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin file patch: %w", err)
	}
	defer tx.Rollback(ctx)

	var formatID string
	var withheldAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		select revision.format, asset.withheld_at
		  from assets asset
		  join asset_revisions revision on revision.id = asset.current_revision_id
		 where asset.id = $1 and asset.owner_id = $2 and asset.deleted_at is null
		 for update of asset
	`, input.AssetID, input.OwnerID).Scan(&formatID, &withheldAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find file patch target: %w", err)
	}
	if withheldAt.Valid {
		return ErrAssetFrozen
	}
	if err := s.validateFilePatch(formatID, input.Patch); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`delete from file_field_patches where asset_id = $1 and provenance = 'creator'`,
		input.AssetID,
	); err != nil {
		return fmt.Errorf("clear creator file patch: %w", err)
	}
	if err := insertFilePatch(ctx, tx, input.AssetID, nil, "creator", input.Patch); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit file patch: %w", err)
	}
	return nil
}

func (s *Service) setReconciliationPatch(
	ctx context.Context,
	assetID, revisionID uuid.UUID,
	patch format.Patch,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reconciliation patch: %w", err)
	}
	defer tx.Rollback(ctx)
	var formatID string
	err = tx.QueryRow(ctx, `
		select format from asset_revisions where id = $1 and asset_id = $2 for update
	`, revisionID, assetID).Scan(&formatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find reconciliation revision: %w", err)
	}
	if err := s.validateFilePatch(formatID, patch); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`delete from file_field_patches where revision_id = $1 and provenance = 'reconciliation'`,
		revisionID,
	); err != nil {
		return fmt.Errorf("clear reconciliation patch: %w", err)
	}
	if err := insertFilePatch(ctx, tx, assetID, &revisionID, "reconciliation", patch); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reconciliation patch: %w", err)
	}
	return nil
}

func (s *Service) validateFilePatch(formatID string, patch format.Patch) error {
	module, ok := s.reg.ByID(formatID)
	if !ok {
		return ErrPatchUnsupported
	}
	patcher, ok := module.(format.Patcher)
	if !ok {
		return ErrPatchUnsupported
	}
	return patcher.ValidatePatch(patch)
}

func insertFilePatch(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	revisionID *uuid.UUID,
	provenance string,
	patch format.Patch,
) error {
	fields := make([]string, 0, len(patch))
	for field := range patch {
		fields = append(fields, string(field))
	}
	sort.Strings(fields)
	for _, field := range fields {
		if _, err := tx.Exec(ctx, `
			insert into file_field_patches (asset_id, revision_id, field, value, provenance)
			values ($1, $2, $3, $4, $5)
		`, assetID, revisionID, field, patch[format.Field(field)], provenance); err != nil {
			return fmt.Errorf("store %s file patch field %q: %w", provenance, field, err)
		}
	}
	return nil
}
