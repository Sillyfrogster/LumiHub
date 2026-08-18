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

// BlockUpdate is everything one block sheet can save at once.
type BlockUpdate struct {
	Title    *string
	Elements []block.Element
}

// SavedBlock is the saved row and the kind catalog that describes it.
type SavedBlock struct {
	Kind  string
	Block block.Block
}

// SaveBlock rewrites one block row and leaves every other block untouched.
func (s *Service) SaveBlock(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
	blockID uuid.UUID,
	update BlockUpdate,
) (SavedBlock, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SavedBlock{}, err
	}
	defer tx.Rollback(ctx)

	var kind string
	var withheld bool
	err = tx.QueryRow(ctx, `
		select kind, withheld_at is not null
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
		 for update
	`, assetID, ownerID).Scan(&kind, &withheld)
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedBlock{}, ErrNotFound
	}
	if err != nil {
		return SavedBlock{}, fmt.Errorf("read block owner: %w", err)
	}
	if withheld {
		return SavedBlock{}, ErrAssetFrozen
	}

	blocks, err := readBlocks(ctx, tx, assetID)
	if err != nil {
		return SavedBlock{}, err
	}
	before := append([]block.Block(nil), blocks...)
	var saved *block.Block
	for i := range blocks {
		if blocks[i].ID != blockID {
			continue
		}
		blocks[i].Title = update.Title
		blocks[i].Elements = update.Elements
		saved = &blocks[i]
		break
	}
	if saved == nil {
		return SavedBlock{}, ErrNotFound
	}
	if err := block.ValidateStructure(*saved); err != nil {
		return SavedBlock{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	if err := block.ValidateBuilderConstraints(kind, before, blocks); err != nil {
		return SavedBlock{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}

	elements, err := json.Marshal(saved.Elements)
	if err != nil {
		return SavedBlock{}, fmt.Errorf("write %s elements: %w", saved.Definition, err)
	}
	result, err := tx.Exec(ctx, `
		update asset_blocks
		   set title = $3, elements = $4
		 where id = $1 and asset_id = $2
	`, blockID, assetID, saved.Title, elements)
	if err != nil {
		return SavedBlock{}, fmt.Errorf("save block: %w", err)
	}
	if result.RowsAffected() != 1 {
		return SavedBlock{}, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedBlock{}, err
	}
	return SavedBlock{Kind: kind, Block: *saved}, nil
}

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
