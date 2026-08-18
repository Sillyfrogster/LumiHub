package character

import (
	"encoding/json"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

// Where a card's leftovers sit. `card` is Illarin's name for the card body
// itself; every other namespace is a key of `extensions`, which is where the
// tools that wrote a card put their own data.
const (
	cardNamespace = "card"
	extensionsKey = "extensions"
	bookKey       = "character_book"
)

// lorebook is a card's embedded book, read once: the entries that became
// content, and what the book carried that the entry table has no place for.
type lorebook struct {
	found bool
	// insideExtensions records that the card kept its book under `extensions`
	// rather than beside its other fields.
	insideExtensions bool
	table            block.EntryTable
	// bookFields are the book's own keys, minus its entries.
	bookFields json.RawMessage
	// entryFields are one entry's leftover keys, by the id Illarin gave that
	// entry. A format's own identifier, such as SillyTavern's uid, is in here
	// like anything else.
	entryFields map[uuid.UUID]json.RawMessage
}

// preserved turns what the book could not model into rows: one for the book,
// owned by the element that holds it, and one per entry, owned by that entry.
func (l lorebook) preserved(elements []block.Element) []format.Remainder {
	if !l.found {
		return nil
	}
	var holder uuid.UUID
	for _, element := range elements {
		if element.Role == block.RoleLorebookEntries {
			holder = element.ID
			break
		}
	}
	rows := make([]format.Remainder, 0, len(l.entryFields)+1)
	if len(l.bookFields) > 0 {
		rows = append(rows, format.Remainder{
			Owner: format.OwnerElement, OwnerID: holder,
			Namespace: bookKey, Payload: l.bookFields,
		})
	}
	for _, entry := range l.table.Entries {
		fields, ok := l.entryFields[entry.ID]
		if !ok {
			continue
		}
		rows = append(rows, format.Remainder{
			Owner: format.OwnerItem, OwnerID: entry.ID,
			Namespace: bookKey, Payload: fields,
		})
	}
	return rows
}

// lorebook reads the card's embedded book. A book the card kept inside its
// extensions is the same book: it becomes content once, and the extensions key
// that held it keeps no second copy.
func (c card) lorebook() lorebook {
	raw := c.fields[bookKey]
	insideExtensions := false
	if len(raw) == 0 {
		raw = c.extensions()[bookKey]
		insideExtensions = true
	}
	var source map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &source) != nil {
		return lorebook{}
	}
	var entryPayloads []json.RawMessage
	if entriesRaw, present := source["entries"]; !present || json.Unmarshal(entriesRaw, &entryPayloads) != nil {
		return lorebook{}
	}
	delete(source, "entries")

	book := lorebook{
		found:            true,
		insideExtensions: insideExtensions,
		entryFields:      make(map[uuid.UUID]json.RawMessage),
	}
	entries := make([]block.Entry, 0, len(entryPayloads))
	for _, payload := range entryPayloads {
		item := block.Entry{ID: block.NewItemID(), Enabled: true}
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload, &fields) != nil || fields == nil {
			// An entry that is not an object still arrived, so it is kept as
			// an entry with nothing read and the whole payload preserved.
			book.entryFields[item.ID] = payload
			entries = append(entries, item)
			continue
		}
		readEntry(fields, &item)
		entries = append(entries, item)
		if len(fields) > 0 {
			book.entryFields[item.ID], _ = json.Marshal(fields)
		}
	}
	book.table = block.EntryTable{Entries: entries}
	if len(source) > 0 {
		book.bookFields, _ = json.Marshal(source)
	}
	return book
}

// readEntry takes what the entry table models out of one entry's fields and
// leaves the rest behind for preservation.
func readEntry(fields map[string]json.RawMessage, item *block.Entry) {
	consumeLorebookField(fields, "name", &item.Name)
	consumeLorebookField(fields, "keys", &item.Keys)
	consumeLorebookField(fields, "secondary_keys", &item.SecondaryKeys)
	consumeLorebookField(fields, "selective", &item.Selective)
	consumeLorebookField(fields, "case_sensitive", &item.CaseSensitive)
	consumeLorebookField(fields, "constant", &item.Constant)
	consumeLorebookField(fields, "enabled", &item.Enabled)
	consumeLorebookField(fields, "insertion_order", &item.Order)
	consumeLorebookField(fields, "content", &item.Text)
	var position string
	if !consumeLorebookField(fields, "position", &position) {
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

func consumeLorebookField[T any](fields map[string]json.RawMessage, name string, target *T) bool {
	raw, present := fields[name]
	if !present || json.Unmarshal(raw, target) != nil {
		return false
	}
	delete(fields, name)
	return true
}
