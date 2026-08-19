// Package book holds the lorebook entry vocabulary the CharacterBook shape
// uses. A character card carries a book under `character_book` and a
// standalone lorebook file carries the same object at its own top level, so
// the field names, the position wording and the round trip are one piece of
// code rather than two that drift.
//
// Nothing here knows what holds the book. A caller hands over the entry
// payloads it found and gets entries plus whatever each one carried that the
// entry table has no place for.
package book

import (
	"encoding/json"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/google/uuid"
)

// Read turns one book's entry payloads into entries, and returns what each
// entry carried that the entry table has no place for, keyed by the id Illarin
// minted for that entry. A format's own identifier, such as SillyTavern's uid,
// is in there like anything else.
func Read(payloads []json.RawMessage) ([]block.Entry, map[uuid.UUID]json.RawMessage) {
	entries := make([]block.Entry, 0, len(payloads))
	leftovers := make(map[uuid.UUID]json.RawMessage)
	for _, payload := range payloads {
		item := block.Entry{ID: block.NewItemID(), Enabled: true}
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload, &fields) != nil || fields == nil {
			// An entry that is not an object still arrived, so it is kept as
			// an entry with nothing read and the whole payload preserved.
			leftovers[item.ID] = payload
			entries = append(entries, item)
			continue
		}
		ReadEntry(fields, &item)
		entries = append(entries, item)
		if len(fields) > 0 {
			leftovers[item.ID], _ = json.Marshal(fields)
		}
	}
	return entries, leftovers
}

// ReadEntry takes what the entry table models out of one entry's fields and
// leaves the rest behind for preservation. A declared field the file wrote as
// the wrong shape is not consumed, so a bad value costs that field and leaves
// the rest of the entry, and every other entry, whole.
func ReadEntry(fields map[string]json.RawMessage, item *block.Entry) {
	keys.Take(fields, "name", &item.Name)
	keys.Take(fields, "keys", &item.Keys)
	keys.Take(fields, "secondary_keys", &item.SecondaryKeys)
	keys.Take(fields, "selective", &item.Selective)
	keys.Take(fields, "case_sensitive", &item.CaseSensitive)
	keys.Take(fields, "constant", &item.Constant)
	keys.Take(fields, "enabled", &item.Enabled)
	keys.Take(fields, "insertion_order", &item.Order)
	keys.Take(fields, "content", &item.Text)
	var position string
	if !keys.Take(fields, "position", &position) {
		return
	}
	switch position {
	case "", "before_char", "before_character":
		if position != "" {
			item.Position = block.BeforeCharacter
		}
	case "after_char", "after_character":
		item.Position = block.AfterCharacter
	default:
		fields["position"], _ = json.Marshal(position)
	}
}

// Write writes the entries a book carries. Everything a book held that the
// entry table has no place for comes back afterwards, from preservation.
func Write(entries []block.Entry) []map[string]json.RawMessage {
	written := make([]map[string]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		fields := map[string]json.RawMessage{
			"keys":            keys.Must(OrEmptyStrings(entry.Keys)),
			"content":         keys.Must(entry.Text),
			"enabled":         keys.Must(entry.Enabled),
			"insertion_order": keys.Must(entry.Order),
		}
		keys.WriteIfSet(fields, "name", entry.Name != "", entry.Name)
		keys.WriteIfSet(fields, "secondary_keys", len(entry.SecondaryKeys) > 0, entry.SecondaryKeys)
		keys.WriteIfSet(fields, "selective", entry.Selective, entry.Selective)
		keys.WriteIfSet(fields, "case_sensitive", entry.CaseSensitive, entry.CaseSensitive)
		keys.WriteIfSet(fields, "constant", entry.Constant, entry.Constant)
		keys.WriteIfSet(fields, "position", entry.Position != "", writtenPosition(entry.Position))
		written = append(written, fields)
	}
	return written
}

// writtenPosition is the wording a book uses for where an entry's text goes.
func writtenPosition(position block.EntryPosition) string {
	if position == block.AfterCharacter {
		return "after_char"
	}
	return "before_char"
}

// OrEmptyStrings writes an absent list as an empty one, because a book's keys
// are a list a reader expects to find even when it holds nothing.
func OrEmptyStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
