package v1

import (
	"encoding/json"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format/keys"
	"github.com/google/uuid"
)

// v1's own field names for a standalone lorebook, close to SillyTavern's World Info but not the same spelling.
const (
	v1EntryKeys          = "key"
	v1EntrySecondaryKeys = "keysecondary"
	v1EntryName          = "comment"
	v1EntryText          = "content"
	v1EntryDisabled      = "disabled"
	v1EntryOrder         = "order_value"
	v1EntryPosition      = "position"
	v1EntryConstant      = "constant"
	v1EntrySelective     = "selective"
	v1EntryCaseSensitive = "case_sensitive"
)

// The two placements Illarin has wording for, with any other number left to preservation.
const (
	v1BeforeCharacter = 0
	v1AfterCharacter  = 1
)

// readLorebookEntries models what the entry table holds and preserves the rest, including the recursion switches no v1 lorebook writer can carry.
func readLorebookEntries(
	payloads []json.RawMessage,
) ([]block.Entry, map[uuid.UUID]json.RawMessage) {
	entries := make([]block.Entry, 0, len(payloads))
	leftovers := make(map[uuid.UUID]json.RawMessage)
	for _, payload := range payloads {
		entry, kept := readLorebookEntry(payload)
		entries = append(entries, entry)
		if len(kept) > 0 {
			leftovers[entry.ID] = kept
		}
	}
	return entries, leftovers
}

func readLorebookEntry(payload json.RawMessage) (block.Entry, json.RawMessage) {
	entry := block.Entry{ID: block.NewItemID(), Enabled: true}
	var fields map[string]json.RawMessage
	if json.Unmarshal(payload, &fields) != nil || fields == nil {
		return entry, payload
	}

	keys.Take(fields, v1EntryName, &entry.Name)
	keys.Take(fields, v1EntryKeys, &entry.Keys)
	keys.Take(fields, v1EntrySecondaryKeys, &entry.SecondaryKeys)
	keys.Take(fields, v1EntrySelective, &entry.Selective)
	keys.Take(fields, v1EntryCaseSensitive, &entry.CaseSensitive)
	keys.Take(fields, v1EntryConstant, &entry.Constant)
	keys.Take(fields, v1EntryOrder, &entry.Order)
	keys.Take(fields, v1EntryText, &entry.Text)

	var disabled bool
	if keys.Take(fields, v1EntryDisabled, &disabled) {
		entry.Enabled = !disabled
	}
	var placement int
	if keys.Take(fields, v1EntryPosition, &placement) {
		switch placement {
		case v1BeforeCharacter:
			entry.Position = block.BeforeCharacter
		case v1AfterCharacter:
			entry.Position = block.AfterCharacter
		default:
			fields[v1EntryPosition], _ = json.Marshal(placement)
		}
	}
	if len(fields) == 0 {
		return entry, nil
	}
	kept, _ := json.Marshal(fields)
	return entry, kept
}
