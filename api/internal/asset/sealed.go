package asset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// sealedSourceTable is the v1 table the sealed blocks came out of, and the namespace they are kept under.
const sealedSourceTable = "preset_sealed_blocks"

// SealedContent is the withheld v1 preset content an asset preserves, written as one file rather than stored, so no blob is made and no download is recorded.
type SealedContent struct {
	Body      []byte
	MediaType string
	Filename  string
	Blocks    int
}

// sealedExport is what the file holds, with each block as it was preserved, so what comes out is what v1 held rather than a reading of it.
type sealedExport struct {
	AssetID   uuid.UUID         `json:"asset_id"`
	AssetName string            `json:"asset_name"`
	Source    string            `json:"source"`
	Blocks    []json.RawMessage `json:"blocks"`
}

// OpenSealedContent hands an owner the sealed blocks their asset preserves, and gives ErrNotFound to anyone else and to any asset holding nothing sealed.
func (s *Service) OpenSealedContent(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
) (SealedContent, error) {
	rows, err := s.pool.Query(ctx, `
		select owned.name, record.payload
		  from migration_preserved_records record
		  join assets owned on owned.id = record.asset_id
		 where record.asset_id = $1
		   and record.source_table = $2
		   and owned.owner_id = $3
		   and owned.deleted_at is null
		 order by record.payload ->> 'version', record.payload ->> 'block_key'
	`, assetID, sealedSourceTable, ownerID)
	if err != nil {
		return SealedContent{}, fmt.Errorf("read sealed content: %w", err)
	}
	defer rows.Close()

	name := ""
	blocks := make([]json.RawMessage, 0)
	for rows.Next() {
		var payload json.RawMessage
		if err := rows.Scan(&name, &payload); err != nil {
			return SealedContent{}, fmt.Errorf("read a sealed block: %w", err)
		}
		blocks = append(blocks, payload)
	}
	if err := rows.Err(); err != nil {
		return SealedContent{}, fmt.Errorf("read sealed content: %w", err)
	}
	if len(blocks) == 0 {
		return SealedContent{}, ErrNotFound
	}

	body, err := json.MarshalIndent(sealedExport{
		AssetID: assetID, AssetName: name, Source: sealedSourceTable, Blocks: blocks,
	}, "", "  ")
	if err != nil {
		return SealedContent{}, fmt.Errorf("write sealed content: %w", err)
	}
	return SealedContent{
		Body:      body,
		MediaType: "application/json",
		Filename:  downloadFilename(name, "sealed", ".json"),
		Blocks:    len(blocks),
	}, nil
}

// SealedBlockCount says whether an asset has sealed content for its owner to read, and answers nobody else, so a stranger cannot learn that an asset is withholding anything.
func (s *Service) SealedBlockCount(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
) (int, error) {
	return sealedBlockCount(ctx, s.pool, ownerID, assetID)
}

func sealedBlockCount(
	ctx context.Context,
	q interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	ownerID uuid.UUID,
	assetID uuid.UUID,
) (int, error) {
	var count int
	err := q.QueryRow(ctx, `
		select count(*)
		  from migration_preserved_records record
		  join assets owned on owned.id = record.asset_id
		 where record.asset_id = $1
		   and record.source_table = $2
		   and owned.owner_id = $3
		   and owned.deleted_at is null
	`, assetID, sealedSourceTable, ownerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count sealed content: %w", err)
	}
	return count, nil
}
