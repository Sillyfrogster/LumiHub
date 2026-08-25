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
	Revision    int
	ContentHash string
	ByteSize    int64
	StorageKey  string
	MediaType   string
}

func revisionStorageKey(assetID, revisionID uuid.UUID) string {
	return fmt.Sprintf("revisions/%s/%s", assetID, revisionID)
}

func insertRevision(ctx context.Context, tx pgx.Tx, id, assetID uuid.UUID, row revisionRow) error {
	queries := db.New(tx)
	params := db.InsertRevisionParams{
		ID:          uuidToPgtype(id),
		AssetID:     uuidToPgtype(assetID),
		Revision:    int32(row.Revision),
		ContentHash: row.ContentHash,
		ByteSize:    row.ByteSize,
		StorageKey:  row.StorageKey,
		MediaType:   row.MediaType,
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

// currentRevisionLocation returns where the current revision of an asset is
// kept.
func currentRevisionLocation(ctx context.Context, q db.DBTX, assetID uuid.UUID) (key, mediaType string, err error) {
	queries := db.New(q)
	row, err := queries.CurrentRevisionLocation(ctx, uuidToPgtype(assetID))
	if err != nil {
		return "", "", err
	}
	return row.StorageKey, row.MediaType, nil
}
