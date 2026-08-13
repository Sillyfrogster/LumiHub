package asset

import (
	"context"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// revisionRow is one preserved copy of an asset's bytes.
type revisionRow struct {
	Revision            int
	BlobID              uuid.UUID
	MediaType           string
	Format              string
	PassthroughPlatform *string
}

func insertRevision(ctx context.Context, tx pgx.Tx, id, assetID uuid.UUID, row revisionRow) error {
	queries := db.New(tx)
	params := db.InsertRevisionParams{
		ID:                  uuidToPgtype(id),
		AssetID:             uuidToPgtype(assetID),
		Revision:            int32(row.Revision),
		BlobID:              uuidToPgtype(row.BlobID),
		MediaType:           row.MediaType,
		Format:              row.Format,
		PassthroughPlatform: textToNullable(row.PassthroughPlatform),
	}
	if err := queries.InsertRevision(ctx, params); err != nil {
		return fmt.Errorf("insert revision: %w", err)
	}
	return nil
}

// setCurrentRevision points the asset at its current revision. Callers never
// derive this.
func setCurrentRevision(ctx context.Context, tx pgx.Tx, assetID, revisionID uuid.UUID) error {
	queries := db.New(tx)
	params := db.SetCurrentRevisionParams{
		ID:                uuidToPgtype(assetID),
		CurrentRevisionID: uuidToPgtype(revisionID),
	}
	if err := queries.SetCurrentRevision(ctx, params); err != nil {
		return fmt.Errorf("set current revision: %w", err)
	}
	return nil
}

// currentRevisionLocation returns the current revision's blob and media type.
func currentRevisionLocation(ctx context.Context, q db.DBTX, assetID uuid.UUID) (blobID uuid.UUID, mediaType string, err error) {
	queries := db.New(q)
	row, err := queries.CurrentRevisionLocation(ctx, uuidToPgtype(assetID))
	if err != nil {
		return uuid.Nil, "", err
	}
	return uuidFromPgtype(row.BlobID), row.MediaType, nil
}
