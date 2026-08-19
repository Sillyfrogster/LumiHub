package character

import (
	"encoding/json"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/book"
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

	entries, entryFields := book.Read(entryPayloads)
	read := lorebook{
		found:            true,
		insideExtensions: insideExtensions,
		table:            block.EntryTable{Entries: entries},
		entryFields:      entryFields,
	}
	if len(source) > 0 {
		read.bookFields, _ = json.Marshal(source)
	}
	return read
}
