package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrPublishFloor is a draft that does not yet carry what its kind asks for.
// The unmet items come back with it, so a refusal reads as a checklist.
var ErrPublishFloor = errors.New("the draft is not ready to publish")

// ErrAlreadyPublished is a second publish of the same asset. Publishing is
// one-way and nothing returns an asset to draft.
var ErrAlreadyPublished = errors.New("the asset is already published")

// The two requirements every kind shares. The rest come from the kind catalog.
const (
	nameRequirement         = "name"
	adultContentRequirement = "adult_content"
)

// ReadinessItem is one thing publication waits on, and whether the asset
// carries it. A creator reads the whole list rather than the first refusal.
type ReadinessItem struct {
	ID     string
	Label  string
	Detail string
	Met    bool
	// BlockID is the block a creator fills the item in, and is nil for a
	// header field.
	BlockID *uuid.UUID
}

// Ready reports whether every item in a readiness list is met.
func Ready(items []ReadinessItem) bool {
	for _, item := range items {
		if !item.Met {
			return false
		}
	}
	return true
}

// readiness is the publish floor for one asset. Every kind asks for a name and
// an answered adult content question, and the kind catalog adds the rest.
func readiness(kind, name string, isNSFW *bool, blocks []block.Block) []ReadinessItem {
	items := []ReadinessItem{
		{
			ID:     nameRequirement,
			Label:  "Name",
			Detail: fmt.Sprintf("Give this %s a name.", kind),
			Met:    name != "",
		},
		{
			ID:     adultContentRequirement,
			Label:  "Adult content answer",
			Detail: "Say whether this contains adult content.",
			Met:    isNSFW != nil,
		},
	}
	for _, check := range block.ContentFloor(kind, blocks) {
		items = append(items, ReadinessItem{
			ID: check.ID, Label: check.Label, Detail: check.Detail,
			Met: check.Met, BlockID: check.BlockID,
		})
	}
	return items
}

// Publish makes a draft public, once. It answers with the readiness list
// either way, so a refusal names every missing item rather than the first.
func (s *Service) Publish(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
) ([]ReadinessItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var kind, name, lifecycle string
	var isNSFW *bool
	var withheld bool
	err = tx.QueryRow(ctx, `
		select kind, name, is_nsfw, lifecycle, withheld_at is not null
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
		 for update
	`, assetID, ownerID).Scan(&kind, &name, &isNSFW, &lifecycle, &withheld)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read asset to publish: %w", err)
	}
	if withheld {
		return nil, ErrAssetFrozen
	}
	if Lifecycle(lifecycle) != LifecycleDraft {
		return nil, ErrAlreadyPublished
	}

	blocks, err := readBlocks(ctx, tx, assetID)
	if err != nil {
		return nil, err
	}
	items := readiness(kind, name, isNSFW, blocks)
	if !Ready(items) {
		return items, ErrPublishFloor
	}

	if _, err := tx.Exec(ctx, `
		update assets set lifecycle = 'published', updated_at = now()
		 where id = $1
	`, assetID); err != nil {
		return nil, fmt.Errorf("publish asset: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
