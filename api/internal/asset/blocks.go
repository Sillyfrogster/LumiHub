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
	Layout   block.Layout
	Width    block.Width
	Elements []block.Element
}

// SavedBlock is the saved row and the kind catalog that describes it.
type SavedBlock struct {
	Kind  string
	Block block.Block
}

// BlockArrangement is one row in the page outline.
type BlockArrangement struct {
	ID     uuid.UUID
	Hidden bool
	Width  block.Width
}

// SavedBlocks is the whole saved page and the kind catalog that describes it.
type SavedBlocks struct {
	Kind   string
	Blocks []block.Block
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

	kind, err := lockEditableAsset(ctx, tx, ownerID, assetID)
	if err != nil {
		return SavedBlock{}, err
	}
	fingerprint, err := s.contentFingerprint(ctx, tx, assetID)
	if err != nil {
		return SavedBlock{}, err
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
		blocks[i].Layout = update.Layout
		blocks[i].Width = update.Width
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
		   set title = $3, layout = $4, width = $5, elements = $6
		 where id = $1 and asset_id = $2
	`, blockID, assetID, saved.Title, saved.Layout, saved.Width, elements)
	if err != nil {
		return SavedBlock{}, fmt.Errorf("save block: %w", err)
	}
	if result.RowsAffected() != 1 {
		return SavedBlock{}, ErrNotFound
	}
	if err := dropUnownedPreservedData(ctx, tx, assetID, blocks); err != nil {
		return SavedBlock{}, err
	}
	if err := s.writeExportProjection(ctx, tx, assetID); err != nil {
		return SavedBlock{}, err
	}
	if err := s.moveContentGeneration(ctx, tx, assetID, fingerprint); err != nil {
		return SavedBlock{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedBlock{}, err
	}
	return SavedBlock{Kind: kind, Block: *saved}, nil
}

// AddBlock puts one optional section at the foot of the page, holding the
// element the creator chose.
func (s *Service) AddBlock(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
	definition block.DefinitionID,
	elementType block.Type,
) (SavedBlock, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SavedBlock{}, err
	}
	defer tx.Rollback(ctx)

	kind, err := lockEditableAsset(ctx, tx, ownerID, assetID)
	if err != nil {
		return SavedBlock{}, err
	}
	fingerprint, err := s.contentFingerprint(ctx, tx, assetID)
	if err != nil {
		return SavedBlock{}, err
	}
	page, err := readBlocks(ctx, tx, assetID)
	if err != nil {
		return SavedBlock{}, err
	}
	added, err := block.NewBlock(kind, definition, elementType, page)
	if err != nil {
		return SavedBlock{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	if err := block.ValidateStructure(added); err != nil {
		return SavedBlock{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	// The new element has no earlier identity to keep, so the page it joins is
	// both sides of the check.
	after := append(page, added)
	if err := block.ValidateBuilderConstraints(kind, after, after); err != nil {
		return SavedBlock{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	if err := insertBlocks(ctx, tx, assetID, []block.Block{added}); err != nil {
		return SavedBlock{}, err
	}
	if err := s.writeExportProjection(ctx, tx, assetID); err != nil {
		return SavedBlock{}, err
	}
	if err := s.moveContentGeneration(ctx, tx, assetID, fingerprint); err != nil {
		return SavedBlock{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedBlock{}, err
	}
	return SavedBlock{Kind: kind, Block: added}, nil
}

// ArrangeBlocks changes page presentation without changing exported content.
func (s *Service) ArrangeBlocks(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
	arrangement []BlockArrangement,
) (SavedBlocks, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SavedBlocks{}, err
	}
	defer tx.Rollback(ctx)

	kind, err := lockEditableAsset(ctx, tx, ownerID, assetID)
	if err != nil {
		return SavedBlocks{}, err
	}
	blocks, err := readBlocks(ctx, tx, assetID)
	if err != nil {
		return SavedBlocks{}, err
	}
	if len(arrangement) != len(blocks) {
		return SavedBlocks{}, fmt.Errorf("%w: include every section once before saving the arrangement", ErrInvalidBlock)
	}
	byID := make(map[uuid.UUID]block.Block, len(blocks))
	for _, holder := range blocks {
		byID[holder.ID] = holder
	}
	after := make([]block.Block, len(arrangement))
	seen := make(map[uuid.UUID]struct{}, len(arrangement))
	for position, choice := range arrangement {
		holder, ok := byID[choice.ID]
		if !ok {
			return SavedBlocks{}, fmt.Errorf("%w: the arrangement includes a section that is not on this page", ErrInvalidBlock)
		}
		if _, duplicate := seen[choice.ID]; duplicate {
			return SavedBlocks{}, fmt.Errorf("%w: include each section once before saving the arrangement", ErrInvalidBlock)
		}
		seen[choice.ID] = struct{}{}
		definition, _ := holder.Definition.Definition(kind)
		if choice.Hidden && definition.Required && !definition.Hideable {
			return SavedBlocks{}, fmt.Errorf("%w: %s is always shown and cannot be hidden", ErrInvalidBlock, definition.Title)
		}
		holder.Position = position
		holder.Hidden = choice.Hidden
		holder.Width = choice.Width
		after[position] = holder
	}
	if err := block.ValidateBuilderConstraints(kind, blocks, after); err != nil {
		return SavedBlocks{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	for _, holder := range after {
		if _, err := tx.Exec(ctx, `
			update asset_blocks
			   set position = $3, hidden = $4, width = $5
			 where id = $1 and asset_id = $2
		`, holder.ID, assetID, holder.Position, holder.Hidden, holder.Width); err != nil {
			return SavedBlocks{}, fmt.Errorf("save block arrangement: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedBlocks{}, err
	}
	return SavedBlocks{Kind: kind, Blocks: after}, nil
}

// RemoveBlock deletes one optional block and closes the gap in page order.
func (s *Service) RemoveBlock(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
	blockID uuid.UUID,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	kind, err := lockEditableAsset(ctx, tx, ownerID, assetID)
	if err != nil {
		return err
	}
	fingerprint, err := s.contentFingerprint(ctx, tx, assetID)
	if err != nil {
		return err
	}
	blocks, err := readBlocks(ctx, tx, assetID)
	if err != nil {
		return err
	}
	remaining := make([]block.Block, 0, len(blocks)-1)
	found := false
	for _, holder := range blocks {
		if holder.ID != blockID {
			remaining = append(remaining, holder)
			continue
		}
		found = true
		definition, _ := holder.Definition.Definition(kind)
		if definition.Required {
			return fmt.Errorf("%w: %s is required and cannot be removed", ErrInvalidBlock, definition.Title)
		}
	}
	if !found {
		return ErrNotFound
	}
	if err := block.ValidateBuilderConstraints(kind, blocks, remaining); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	if err := deleteBlockAndClosePositions(ctx, tx, assetID, blockID, remaining); err != nil {
		return err
	}
	if err := dropUnownedPreservedData(ctx, tx, assetID, remaining); err != nil {
		return err
	}
	if err := s.writeExportProjection(ctx, tx, assetID); err != nil {
		return err
	}
	if err := s.moveContentGeneration(ctx, tx, assetID, fingerprint); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MoveBlockContent moves unpinned elements, then removes their old block.
func (s *Service) MoveBlockContent(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
	blockID uuid.UUID,
	destinationID uuid.UUID,
) (SavedBlocks, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SavedBlocks{}, err
	}
	defer tx.Rollback(ctx)

	kind, err := lockEditableAsset(ctx, tx, ownerID, assetID)
	if err != nil {
		return SavedBlocks{}, err
	}
	fingerprint, err := s.contentFingerprint(ctx, tx, assetID)
	if err != nil {
		return SavedBlocks{}, err
	}
	before, err := readBlocks(ctx, tx, assetID)
	if err != nil {
		return SavedBlocks{}, err
	}
	var source *block.Block
	var destination *block.Block
	for i := range before {
		switch before[i].ID {
		case blockID:
			source = &before[i]
		case destinationID:
			destination = &before[i]
		}
	}
	if source == nil || destination == nil || source.ID == destination.ID {
		return SavedBlocks{}, ErrNotFound
	}
	definition, _ := source.Definition.Definition(kind)
	if definition.Required {
		return SavedBlocks{}, fmt.Errorf("%w: %s is required and cannot be removed", ErrInvalidBlock, definition.Title)
	}
	for _, element := range source.Elements {
		if source.Pinned(element.Role, kind) {
			return SavedBlocks{}, fmt.Errorf("%w: %s is pinned and cannot move", ErrInvalidBlock, element.Role.Label())
		}
	}
	if len(source.Elements) == 0 {
		return SavedBlocks{}, fmt.Errorf("%w: this section has no content to move", ErrInvalidBlock)
	}
	occupied := make(map[block.Slot]struct{}, len(destination.Elements))
	for _, element := range destination.Elements {
		occupied[element.Slot] = struct{}{}
	}
	free := make([]block.Slot, 0)
	for _, slot := range destination.Layout.Slots() {
		if _, used := occupied[slot]; !used {
			free = append(free, slot)
		}
	}
	if len(free) < len(source.Elements) {
		return SavedBlocks{}, fmt.Errorf("%w: %s does not have room for this content", ErrInvalidBlock, destination.Definition)
	}
	for i, element := range source.Elements {
		element.Slot = free[i]
		destination.Elements = append(destination.Elements, element)
	}
	after := make([]block.Block, 0, len(before)-1)
	for _, holder := range before {
		if holder.ID == source.ID {
			continue
		}
		holder.Position = len(after)
		after = append(after, holder)
	}
	if err := block.ValidateStructure(*destination); err != nil {
		return SavedBlocks{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	if err := block.ValidateBuilderConstraints(kind, before, after); err != nil {
		return SavedBlocks{}, fmt.Errorf("%w: %v", ErrInvalidBlock, err)
	}
	elements, err := json.Marshal(destination.Elements)
	if err != nil {
		return SavedBlocks{}, fmt.Errorf("write moved elements: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		update asset_blocks set elements = $3 where id = $1 and asset_id = $2
	`, destination.ID, assetID, elements); err != nil {
		return SavedBlocks{}, fmt.Errorf("save moved elements: %w", err)
	}
	if err := deleteBlockAndClosePositions(ctx, tx, assetID, source.ID, after); err != nil {
		return SavedBlocks{}, err
	}
	if err := dropUnownedPreservedData(ctx, tx, assetID, after); err != nil {
		return SavedBlocks{}, err
	}
	if err := s.writeExportProjection(ctx, tx, assetID); err != nil {
		return SavedBlocks{}, err
	}
	if err := s.moveContentGeneration(ctx, tx, assetID, fingerprint); err != nil {
		return SavedBlocks{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedBlocks{}, err
	}
	return SavedBlocks{Kind: kind, Blocks: after}, nil
}

func deleteBlockAndClosePositions(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	blockID uuid.UUID,
	remaining []block.Block,
) error {
	if _, err := tx.Exec(ctx, `delete from asset_blocks where id = $1 and asset_id = $2`, blockID, assetID); err != nil {
		return fmt.Errorf("remove block: %w", err)
	}
	for position := range remaining {
		if _, err := tx.Exec(ctx, `
			update asset_blocks set position = $3 where id = $1 and asset_id = $2
		`, remaining[position].ID, assetID, position); err != nil {
			return fmt.Errorf("close block positions: %w", err)
		}
	}
	return nil
}

func lockEditableAsset(
	ctx context.Context,
	tx pgx.Tx,
	ownerID uuid.UUID,
	assetID uuid.UUID,
) (string, error) {
	var kind string
	var withheld bool
	err := tx.QueryRow(ctx, `
		select kind, withheld_at is not null
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
		 for update
	`, assetID, ownerID).Scan(&kind, &withheld)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read block owner: %w", err)
	}
	if withheld {
		return "", ErrAssetFrozen
	}
	return kind, nil
}

// insertBlocks writes an asset's blocks, one row each.
func insertBlocks(ctx context.Context, tx pgx.Tx, assetID uuid.UUID, blocks []block.Block) error {
	queries := db.New(tx)
	for _, b := range blocks {
		if err := block.ValidateStructure(b); err != nil {
			return fmt.Errorf("validate %s block: %w", b.Definition, err)
		}
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
