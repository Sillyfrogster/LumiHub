package lorebook

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/book"
	"github.com/google/uuid"
)

// Write builds a lorebook document out of the asset's roles. Nothing here
// reads another format's bytes: the file is the name above the builder and the
// one element the kind is built on, and everything the book arrived with that
// Illarin has no place for comes back afterwards from preservation.
func (Module) Write(_ context.Context, asset format.ExportAsset) (format.Artifact, error) {
	entries := bookEntries(asset)
	body := map[string]json.RawMessage{
		"name":     mustJSON(asset.Header.Name),
		entriesKey: mustJSON(book.Write(entries)),
	}
	if err := restorePreserved(body, entries, asset.Preserved); err != nil {
		return format.Artifact{}, err
	}
	document, err := json.Marshal(body)
	if err != nil {
		return format.Artifact{}, fmt.Errorf("write the lorebook: %w", err)
	}
	return format.Artifact{
		Body: document, MediaType: "application/json", Extension: ".json",
	}, nil
}

func bookEntries(asset format.ExportAsset) []block.Entry {
	content, ok := asset.Content(block.RoleLorebookEntries)
	if !ok {
		return nil
	}
	table, isTable := content.(block.EntryTable)
	if !isTable {
		return nil
	}
	return table.Entries
}

// restorePreserved writes an asset's preserved data back into the document the
// writer has built from the creator's content:
//
//   - the lorebook namespace goes back at the document's own top level
//   - every other namespace goes back under `extensions`, one key each
//   - one entry's own keys go back into that entry, found by the entry's id
//
// The entries are expected in the order the writer wrote them, which is the
// order of the entry table it wrote them from. An entry the creator deleted is
// simply not there, and its preserved keys go nowhere.
//
// Content the writer already wrote wins. A preserved copy of something the
// creator can now edit would put a stale value back on top of a live one.
func restorePreserved(
	body map[string]json.RawMessage,
	entries []block.Entry,
	rows []format.Remainder,
) error {
	extensions := readObject(body[extensionsKey])
	written, err := writtenEntries(body)
	if err != nil {
		return err
	}
	position := make(map[uuid.UUID]int, len(entries))
	for index, entry := range entries {
		position[entry.ID] = index
	}

	for _, row := range rows {
		var fields map[string]json.RawMessage
		if json.Unmarshal(row.Payload, &fields) != nil {
			fields = nil
		}
		switch {
		case row.Owner == format.OwnerAsset && row.Namespace == bookNamespace:
			mergeAbsent(body, fields)
		case row.Owner == format.OwnerAsset:
			if _, held := extensions[row.Namespace]; !held {
				extensions[row.Namespace] = row.Payload
			}
		case row.Owner == format.OwnerItem && row.Namespace == entryNamespace:
			index, kept := position[row.OwnerID]
			if !kept || index >= len(written) {
				continue
			}
			mergeAbsent(written[index], fields)
		}
	}

	if len(extensions) > 0 {
		body[extensionsKey], _ = json.Marshal(extensions)
	}
	body[entriesKey], _ = json.Marshal(written)
	return nil
}

// writtenEntries takes apart the entry list the writer put in the document, so
// preserved keys can go back into the entries they came from.
func writtenEntries(body map[string]json.RawMessage) ([]map[string]json.RawMessage, error) {
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(body[entriesKey], &entries); err != nil {
		return nil, fmt.Errorf("read the written entries: %w", err)
	}
	return entries, nil
}

// mergeAbsent adds the keys a target does not already carry.
func mergeAbsent(target, source map[string]json.RawMessage) {
	for key, value := range source {
		if _, written := target[key]; !written {
			target[key] = value
		}
	}
}

// readObject reads an object out of raw JSON, giving back an empty one where
// there is nothing to read.
func readObject(raw json.RawMessage) map[string]json.RawMessage {
	object := make(map[string]json.RawMessage)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &object)
	}
	return object
}

// mustJSON encodes a value the writer built itself. Every call site passes a
// string or a list of objects the writer just made, neither of which can fail.
func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("write a lorebook value: %v", err))
	}
	return encoded
}
