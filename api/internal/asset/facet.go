package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) writeFacetProjection(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
) error {
	counts, err := facetCounts(ctx, tx, assetID)
	if err != nil {
		return err
	}
	stored, err := json.Marshal(counts)
	if err != nil {
		return fmt.Errorf("write the facet projection: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into asset_projections (asset_id, facets, facet_stamp, facet_computed_at)
		values ($1, $2, $3, now())
		on conflict (asset_id) do update
		   set facets = excluded.facets,
		       facet_stamp = excluded.facet_stamp,
		       facet_computed_at = excluded.facet_computed_at
	`, assetID, stored, block.FacetStamp()); err != nil {
		return fmt.Errorf("store the facet projection: %w", err)
	}
	return nil
}

func facetCounts(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
) (map[block.FacetKey]int, error) {
	var kind string
	err := q.QueryRow(ctx, `
		select kind from assets where id = $1 and deleted_at is null
	`, assetID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read the asset to measure: %w", err)
	}
	blocks, err := readBlocks(ctx, q, assetID)
	if err != nil {
		return nil, err
	}
	return block.MeasureFacets(kind, shownElements(blocks)), nil
}

func shownElements(blocks []block.Block) []block.Element {
	elements := make([]block.Element, 0)
	for _, holder := range blocks {
		if holder.Hidden {
			continue
		}
		elements = append(elements, holder.Elements...)
	}
	return elements
}

func (s *Service) RecomputeStaleFacetProjections(ctx context.Context) (int, error) {
	stale, err := s.staleProjections(ctx, `
		select asset.id
		  from assets asset
		  left join asset_projections projection on projection.asset_id = asset.id
		 where asset.deleted_at is null
		   and (projection.asset_id is null or projection.facet_stamp <> $1)
		 order by asset.id
	`, block.FacetStamp())
	if err != nil {
		return 0, err
	}
	for _, assetID := range stale {
		if err := s.inTransaction(ctx, func(tx pgx.Tx) error {
			return s.writeFacetProjection(ctx, tx, assetID)
		}); err != nil {
			return 0, fmt.Errorf("recompute the facet projection for %s: %w", assetID, err)
		}
	}
	return len(stale), nil
}
