// Package protected keeps content that leaves Illarin only through an explicit view.
package protected

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	promptOwnerKind = "prompt_fragment"
	promptPayload   = "prompt_fragment_text"
	AppLumiverse    = "lumiverse"
)

var ErrPolicyRequired = errors.New("choose at least one allowed app before sealing a prompt")

// AppTargets names the code-owned export targets each allowed app can receive.
func AppTargets(kind, app string) []string {
	if kind == "preset" && app == AppLumiverse {
		return []string{"preset_lumiverse"}
	}
	return nil
}

// EligibleApps returns the code-known apps with at least one offered target.
func EligibleApps(kind string, offered []string) []string {
	apps := []string{}
	for _, app := range []string{AppLumiverse} {
		eligible := false
		for _, target := range AppTargets(kind, app) {
			for _, candidate := range offered {
				if candidate == target {
					eligible = true
					break
				}
			}
			if eligible {
				break
			}
		}
		if eligible {
			apps = append(apps, app)
		}
	}
	return apps
}

// HasPromptFragments reports whether this page needs a protected-content policy.
func HasPromptFragments(blocks []block.Block) bool {
	for _, holder := range blocks {
		for _, element := range holder.Elements {
			list, ok := element.Content.(block.PromptList)
			if !ok {
				continue
			}
			for _, fragment := range list.Fragments {
				if fragment.Protected {
					return true
				}
			}
		}
	}
	return false
}

// SyncPromptFragments splits sealed prompt text from the public blocks in the
// transaction that saves them. A nil policy keeps an existing policy only.
func SyncPromptFragments(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	blocks []block.Block,
	allowedApps *[]string,
) error {
	sealed := make(map[uuid.UUID]string)
	for blockIndex := range blocks {
		for elementIndex := range blocks[blockIndex].Elements {
			element := &blocks[blockIndex].Elements[elementIndex]
			list, ok := element.Content.(block.PromptList)
			if !ok {
				continue
			}
			for itemIndex := range list.Fragments {
				fragment := &list.Fragments[itemIndex]
				if !fragment.Protected {
					continue
				}
				if fragment.Marker != "" {
					return fmt.Errorf("a prompt marker cannot be sealed")
				}
				sealed[fragment.ID] = fragment.Text
				fragment.Text = ""
			}
			element.Content = list
		}
	}

	if len(sealed) == 0 {
		if _, err := tx.Exec(ctx, `delete from protected_content where asset_id = $1`, assetID); err != nil {
			return fmt.Errorf("remove protected prompts: %w", err)
		}
		if _, err := tx.Exec(ctx, `delete from protected_delivery_apps where asset_id = $1`, assetID); err != nil {
			return fmt.Errorf("remove protected delivery policy: %w", err)
		}
		return nil
	}

	apps, err := policy(ctx, tx, assetID, allowedApps)
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		return ErrPolicyRequired
	}
	if allowedApps != nil {
		if _, err := tx.Exec(ctx, `delete from protected_delivery_apps where asset_id = $1`, assetID); err != nil {
			return fmt.Errorf("replace protected delivery policy: %w", err)
		}
		for _, app := range apps {
			if _, err := tx.Exec(ctx, `insert into protected_delivery_apps (asset_id, app) values ($1, $2)`, assetID, app); err != nil {
				return fmt.Errorf("save protected delivery policy: %w", err)
			}
		}
	}

	for id, text := range sealed {
		payload, err := json.Marshal(map[string]string{"text": text})
		if err != nil {
			return fmt.Errorf("encode protected prompt: %w", err)
		}
		digest := sha256.Sum256([]byte(text))
		if _, err := tx.Exec(ctx, `
			insert into protected_content (asset_id, owner_kind, owner_id, payload_type, payload, digest)
			values ($1, $2, $3, $4, $5, $6)
			on conflict (asset_id, owner_kind, owner_id) do update
			set payload = excluded.payload, payload_type = excluded.payload_type, digest = excluded.digest
		`, assetID, promptOwnerKind, id, promptPayload, payload, digest[:]); err != nil {
			return fmt.Errorf("save protected prompt: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		delete from protected_content
		 where asset_id = $1 and owner_kind = $2 and not (owner_id = any($3::uuid[]))
	`, assetID, promptOwnerKind, ids(sealed)); err != nil {
		return fmt.Errorf("remove unsealed prompts: %w", err)
	}
	return nil
}

func policy(ctx context.Context, tx pgx.Tx, assetID uuid.UUID, supplied *[]string) ([]string, error) {
	if supplied != nil {
		seen := map[string]bool{}
		apps := make([]string, 0, len(*supplied))
		for _, app := range *supplied {
			if len(AppTargets("preset", app)) == 0 || seen[app] {
				return nil, fmt.Errorf("%q is not an allowed app", app)
			}
			seen[app] = true
			apps = append(apps, app)
		}
		return apps, nil
	}
	rows, err := tx.Query(ctx, `select app from protected_delivery_apps where asset_id = $1 order by app`, assetID)
	if err != nil {
		return nil, fmt.Errorf("read protected delivery policy: %w", err)
	}
	defer rows.Close()
	var apps []string
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

// RestorePromptFragments returns the owner or delivery view. Readers never call it.
func RestorePromptFragments(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, assetID uuid.UUID, blocks []block.Block) error {
	rows, err := q.Query(ctx, `
		select owner_id, payload from protected_content
		 where asset_id = $1 and owner_kind = $2 and payload_type = $3
	`, assetID, promptOwnerKind, promptPayload)
	if err != nil {
		return fmt.Errorf("read protected prompts: %w", err)
	}
	defer rows.Close()
	texts := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var payload []byte
		if err := rows.Scan(&id, &payload); err != nil {
			return fmt.Errorf("read protected prompt: %w", err)
		}
		var item struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(payload, &item); err != nil {
			return fmt.Errorf("decode protected prompt: %w", err)
		}
		texts[id] = item.Text
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read protected prompts: %w", err)
	}
	for blockIndex := range blocks {
		for elementIndex := range blocks[blockIndex].Elements {
			element := &blocks[blockIndex].Elements[elementIndex]
			list, ok := element.Content.(block.PromptList)
			if !ok {
				continue
			}
			for itemIndex := range list.Fragments {
				fragment := &list.Fragments[itemIndex]
				if !fragment.Protected {
					continue
				}
				text, found := texts[fragment.ID]
				if !found {
					return fmt.Errorf("protected prompt %s has no payload", fragment.ID)
				}
				fragment.Text = text
			}
			element.Content = list
		}
	}
	return nil
}

// Apps returns an asset's policy. An ordinary asset has no entries.
func Apps(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, assetID uuid.UUID) ([]string, error) {
	rows, err := q.Query(ctx, `select app from protected_delivery_apps where asset_id = $1 order by app`, assetID)
	if err != nil {
		return nil, fmt.Errorf("read protected delivery policy: %w", err)
	}
	defer rows.Close()
	apps := []string{}
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func ids(values map[uuid.UUID]string) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	return result
}
