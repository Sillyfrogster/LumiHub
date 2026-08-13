package asset

import (
	"context"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func assetToInsertParams(a Asset, ownerID uuid.UUID, madeAt *time.Time) db.InsertAssetParams {
	return db.InsertAssetParams{
		ID:          pgtype.UUID{Bytes: a.ID, Valid: true},
		Kind:        a.Kind,
		OwnerID:     pgtype.UUID{Bytes: ownerID, Valid: true},
		Name:        a.Name,
		Description: a.Description,
		Tags:        a.Tags,
		IsNsfw:      a.IsNSFW,
		Discovery:   a.Discovery,
		CreatedAt:   timeToNullable(madeAt),
	}
}

// insertAsset returns the made date the row was given, or the time it was written.
func insertAsset(ctx context.Context, tx pgx.Tx, a Asset, ownerID uuid.UUID, madeAt *time.Time) (time.Time, error) {
	queries := db.New(tx)
	params := assetToInsertParams(a, ownerID, madeAt)
	made, err := queries.InsertAsset(ctx, params)
	if err != nil {
		return time.Time{}, fmt.Errorf("insert asset: %w", err)
	}
	return timeFromPgtype(made), nil
}

func insertFacets(ctx context.Context, tx pgx.Tx, revisionID uuid.UUID, facets []format.Facet) error {
	queries := db.New(tx)
	for _, f := range facets {
		params := db.InsertFacetParams{
			RevisionID: uuidToPgtype(revisionID),
			Key:        f.Key,
			Value:      f.Value,
		}
		if err := queries.InsertFacet(ctx, params); err != nil {
			return fmt.Errorf("insert facet %s: %w", f.Key, err)
		}
	}
	return nil
}
