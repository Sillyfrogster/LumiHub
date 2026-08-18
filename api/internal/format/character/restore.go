package character

import (
	"encoding/json"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

// RestorePreserved writes an asset's preserved data back into a card body a
// writer has built from the creator's content. It is the other half of
// remainder: every key a reader could not model goes back where it came from,
// byte for byte.
//
//   - the card namespace goes back at the body's own top level
//   - every other namespace goes back under `extensions`, one key each
//   - the book's own keys go back beside its entries
//   - one entry's own keys go back into that entry, found by the entry's id
//
// The book's entries are expected in the order the writer wrote them, which is
// the order of the entry table it wrote them from. An entry the creator
// deleted is simply not there, and its preserved keys go nowhere.
//
// Content the writer already wrote wins. A preserved copy of something the
// creator can now edit would put a stale value back on top of a live one.
func RestorePreserved(
	body map[string]json.RawMessage,
	entries []block.Entry,
	rows []format.Remainder,
) error {
	extensions := readObject(body[extensionsKey])
	book, bookEntries, err := readWrittenBook(body)
	if err != nil {
		return err
	}
	position := make(map[uuid.UUID]int, len(entries))
	for index, entry := range entries {
		position[entry.ID] = index
	}

	for _, row := range rows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(row.Payload, &fields); err != nil {
			fields = nil
		}
		switch {
		case row.Owner == format.OwnerAsset && row.Namespace == cardNamespace:
			mergeAbsent(body, fields)
		case row.Owner == format.OwnerAsset:
			if _, written := extensions[row.Namespace]; !written {
				extensions[row.Namespace] = row.Payload
			}
		case row.Owner == format.OwnerElement && row.Namespace == bookKey:
			mergeAbsent(book, fields)
		case row.Owner == format.OwnerItem && row.Namespace == bookKey:
			index, kept := position[row.OwnerID]
			if !kept || index >= len(bookEntries) {
				continue
			}
			mergeAbsent(bookEntries[index], fields)
		}
	}

	if len(extensions) > 0 {
		body[extensionsKey], _ = json.Marshal(extensions)
	}
	if book == nil {
		return nil
	}
	written := make([]json.RawMessage, len(bookEntries))
	for index, entry := range bookEntries {
		written[index], _ = json.Marshal(entry)
	}
	book["entries"], _ = json.Marshal(written)
	body[bookKey], _ = json.Marshal(book)
	return nil
}

// readWrittenBook takes apart the book the writer put in the body, so
// preserved keys can go back into it and into its entries.
func readWrittenBook(
	body map[string]json.RawMessage,
) (map[string]json.RawMessage, []map[string]json.RawMessage, error) {
	raw, present := body[bookKey]
	if !present {
		return nil, nil, nil
	}
	var book map[string]json.RawMessage
	if err := json.Unmarshal(raw, &book); err != nil {
		return nil, nil, fmt.Errorf("read the written book: %w", err)
	}
	var entries []map[string]json.RawMessage
	if raw, held := book["entries"]; held {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, nil, fmt.Errorf("read the written entries: %w", err)
		}
	}
	return book, entries, nil
}

// mergeAbsent adds the keys a target does not already carry.
func mergeAbsent(target, source map[string]json.RawMessage) {
	for key, value := range source {
		if _, written := target[key]; !written {
			target[key] = value
		}
	}
}

// readObject reads an object out of raw JSON, giving back an empty one where there
// is nothing to read.
func readObject(raw json.RawMessage) map[string]json.RawMessage {
	object := make(map[string]json.RawMessage)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &object)
	}
	return object
}
