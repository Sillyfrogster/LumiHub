package block

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Illarin mints an id for every item inside an element the moment the item is
// created, and never reads one out of a source file. A format's own identifier
// is preserved data like anything else: a file's id says nothing about items a
// creator adds afterwards in the builder, and nothing makes it unique.
//
// Preserved data keys against these ids rather than against a position, so
// reordering a book or deleting one entry cannot move a preserved field onto
// the entry next to it.

// NewItemID mints the id one new item carries for the rest of its life.
func NewItemID() uuid.UUID { return uuid.New() }

// itemID keeps the id an item arrived with and mints one where it has none.
func itemID(supplied uuid.UUID) uuid.UUID {
	if supplied != uuid.Nil {
		return supplied
	}
	return NewItemID()
}

// ItemIDs returns the ids of the items inside one element, in order. An
// element whose content is a single body has none. A block save reconciles
// preserved data against this list, so an item that has gone takes its
// preserved data with it.
func ItemIDs(content Content) []uuid.UUID {
	switch held := content.(type) {
	case TextSet:
		return collectItemIDs(held.Texts, func(item TextItem) uuid.UUID { return item.ID })
	case DialogueSample:
		return collectItemIDs(held.Turns, func(item DialogueTurn) uuid.UUID { return item.ID })
	case ImageSet:
		return collectItemIDs(held.Images, func(item ImageItem) uuid.UUID { return item.ID })
	case FieldList:
		return collectItemIDs(held.Fields, func(item FieldItem) uuid.UUID { return item.ID })
	case LinkList:
		return collectItemIDs(held.Links, func(item LinkItem) uuid.UUID { return item.ID })
	case EntryTable:
		return collectItemIDs(held.Entries, func(item Entry) uuid.UUID { return item.ID })
	default:
		return nil
	}
}

func collectItemIDs[T any](items []T, id func(T) uuid.UUID) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, id(item))
	}
	return ids
}

// mintItemIDs is the upgrade that carries content written before items had ids
// forward. It gives an id to every item in the named list that lacks one and
// leaves everything else in the item exactly as it was stored.
func mintItemIDs(list string) func(json.RawMessage) (json.RawMessage, error) {
	return func(stored json.RawMessage) (json.RawMessage, error) {
		var content map[string]json.RawMessage
		if err := json.Unmarshal(stored, &content); err != nil {
			return nil, err
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(content[list], &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			var id uuid.UUID
			if json.Unmarshal(item["id"], &id) == nil && id != uuid.Nil {
				continue
			}
			minted, err := json.Marshal(NewItemID())
			if err != nil {
				return nil, err
			}
			item["id"] = minted
		}
		upgraded, err := json.Marshal(items)
		if err != nil {
			return nil, err
		}
		content[list] = upgraded
		return json.Marshal(content)
	}
}
