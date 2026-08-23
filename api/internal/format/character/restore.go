package character

import (
	"encoding/json"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/keys"
	"github.com/google/uuid"
)

// RestorePreserved restores card, extension, book, and entry fields without
// overwriting current content. Deleted entries stay deleted.
func RestorePreserved(
	body map[string]json.RawMessage,
	entries []block.Entry,
	rows []format.Remainder,
) error {
	extensions := keys.Object(body[extensionsKey])
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
			keys.MergeAbsent(body, fields)
		case row.Owner == format.OwnerAsset:
			if _, written := extensions[row.Namespace]; !written {
				extensions[row.Namespace] = row.Payload
			}
		case row.Owner == format.OwnerElement && row.Namespace == bookKey:
			keys.MergeAbsent(book, fields)
		case row.Owner == format.OwnerItem && row.Namespace == bookKey:
			index, kept := position[row.OwnerID]
			if !kept || index >= len(bookEntries) {
				continue
			}
			keys.MergeAbsent(bookEntries[index], fields)
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
