package asset

import (
	"context"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func assetToInsertParams(a Asset, ownerID uuid.UUID, madeAt *time.Time) db.InsertAssetParams {
	return db.InsertAssetParams{
		Lifecycle:      string(a.Lifecycle),
		AssetVersion:   a.AssetVersion,
		CreditedAuthor: a.CreditedAuthor,
		Nickname:       a.Nickname,
		OriginFormat:   textToNullable(a.OriginFormat),
		ID:             pgtype.UUID{Bytes: a.ID, Valid: true},
		Kind:           a.Kind,
		OwnerID:        pgtype.UUID{Bytes: ownerID, Valid: true},
		Name:           a.Name,
		Blurb:          a.Blurb,
		Tags:           a.Tags,
		IsNsfw:         boolToNullable(a.IsNSFW),
		Discovery:      string(a.Discovery),
		CreatedAt:      timeToNullable(madeAt),
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
