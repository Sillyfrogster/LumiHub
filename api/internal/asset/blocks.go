package asset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// insertBlocks writes an asset's blocks, one row each.
func insertBlocks(ctx context.Context, tx pgx.Tx, assetID uuid.UUID, blocks []block.Block) error {
	queries := db.New(tx)
	for _, b := range blocks {
		elements, err := json.Marshal(b.Elements)
		if err != nil {
			return fmt.Errorf("write %s elements: %w", b.Definition, err)
		}
		params := db.InsertAssetBlockParams{
			ID:         uuidToPgtype(b.ID),
			AssetID:    uuidToPgtype(assetID),
			Definition: string(b.Definition),
			Title:      textToNullable(b.Title),
			Position:   int32(b.Position),
			Hidden:     b.Hidden,
			Layout:     string(b.Layout),
			Width:      string(b.Width),
			Elements:   elements,
		}
		if err := queries.InsertAssetBlock(ctx, params); err != nil {
			return fmt.Errorf("insert %s block: %w", b.Definition, err)
		}
	}
	return nil
}

// readBlocks returns an asset's blocks in page order.
func readBlocks(ctx context.Context, q db.DBTX, assetID uuid.UUID) ([]block.Block, error) {
	rows, err := db.New(q).AssetBlocks(ctx, uuidToPgtype(assetID))
	if err != nil {
		return nil, fmt.Errorf("read asset blocks: %w", err)
	}
	blocks := make([]block.Block, 0, len(rows))
	for _, row := range rows {
		var elements []block.Element
		if err := json.Unmarshal(row.Elements, &elements); err != nil {
			return nil, fmt.Errorf("read %s elements: %w", row.Definition, err)
		}
		blocks = append(blocks, block.Block{
			ID:         uuidFromPgtype(row.ID),
			Definition: block.DefinitionID(row.Definition),
			Title:      textToPointer(row.Title),
			Position:   int(row.Position),
			Hidden:     row.Hidden,
			Layout:     block.Layout(row.Layout),
			Width:      block.Width(row.Width),
			Elements:   elements,
		})
	}
	return blocks, nil
}
