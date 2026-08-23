package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) writeProjections(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
) error {
	if err := s.writeExportProjection(ctx, tx, assetID); err != nil {
		return err
	}
	return s.writeFacetProjection(ctx, tx, assetID)
}

// writeExportProjection recomputes an asset's offered targets and what each one
// costs it, and stores them.
//
// It runs inside the transaction that caused the change, for a draft as much as
// a published asset, because the builder's download panel reads the result
// before publish. Publishing itself computes nothing.
func (s *Service) writeExportProjection(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
) error {
	targets, err := s.exportCapability(ctx, tx, assetID)
	if err != nil {
		return err
	}
	stored, err := json.Marshal(targets)
	if err != nil {
		return fmt.Errorf("write the export projection: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into asset_projections (asset_id, export, export_stamp, export_computed_at)
		values ($1, $2, $3, now())
		on conflict (asset_id) do update
		   set export = excluded.export,
		       export_stamp = excluded.export_stamp,
		       export_computed_at = excluded.export_computed_at
	`, assetID, stored, s.reg.CapabilityStamp()); err != nil {
		return fmt.Errorf("store the export projection: %w", err)
	}
	return nil
}

// exportCapability measures every writer against what the asset really holds.
func (s *Service) exportCapability(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
) ([]format.Target, error) {
	var kind string
	var origin pgtype.Text
	err := q.QueryRow(ctx, `
		select kind, origin_format from assets where id = $1 and deleted_at is null
	`, assetID).Scan(&kind, &origin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read the asset to project: %w", err)
	}
	blocks, err := readBlocks(ctx, q, assetID)
	if err != nil {
		return nil, err
	}
	elements := make([]block.Element, 0)
	for _, holder := range blocks {
		elements = append(elements, holder.Elements...)
	}
	return s.reg.OfferedTargets(format.CapabilitySubject{
		Kind: kind, Origin: origin.String, Elements: elements,
	}), nil
}

// ExportTargets reads an asset's offered targets out of its projection, which
// is what the download menu and the creator's panel both render. Nothing
// computes compatibility per request.
func (s *Service) exportProjection(
	ctx context.Context,
	assetID uuid.UUID,
) ([]format.Target, error) {
	var stored []byte
	err := s.pool.QueryRow(ctx,
		`select export from asset_projections where asset_id = $1`, assetID,
	).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return []format.Target{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the export projection: %w", err)
	}
	targets := make([]format.Target, 0)
	if err := json.Unmarshal(stored, &targets); err != nil {
		return nil, fmt.Errorf("read the stored export projection: %w", err)
	}
	return targets, nil
}

// RecomputeStaleExportProjections rewrites every asset whose export section was
// computed under a contract that has since changed. Changing a declaration is a
// deploy, so the deploy is the trigger. At this catalog's size the whole sweep
// finishes in seconds, and being wrong means a stale sentence in a menu rather
// than stale bytes in somebody's hands.
func (s *Service) RecomputeStaleExportProjections(ctx context.Context) (int, error) {
	stale, err := s.staleProjections(ctx, `
		select asset.id
		  from assets asset
		  left join asset_projections projection on projection.asset_id = asset.id
		 where asset.deleted_at is null
		   and (projection.asset_id is null or projection.export_stamp <> $1)
		 order by asset.id
	`, s.reg.CapabilityStamp())
	if err != nil {
		return 0, err
	}
	for _, assetID := range stale {
		if err := s.inTransaction(ctx, func(tx pgx.Tx) error {
			return s.writeExportProjection(ctx, tx, assetID)
		}); err != nil {
			return 0, fmt.Errorf("recompute the export projection for %s: %w", assetID, err)
		}
	}
	return len(stale), nil
}

func (s *Service) staleProjections(
	ctx context.Context,
	query string,
	stamp string,
) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, query, stamp)
	if err != nil {
		return nil, fmt.Errorf("find stale projections: %w", err)
	}
	defer rows.Close()
	stale := make([]uuid.UUID, 0)
	for rows.Next() {
		var assetID uuid.UUID
		if err := rows.Scan(&assetID); err != nil {
			return nil, fmt.Errorf("read a stale projection: %w", err)
		}
		stale = append(stale, assetID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find stale projections: %w", err)
	}
	return stale, nil
}

func (s *Service) inTransaction(ctx context.Context, run func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := run(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
