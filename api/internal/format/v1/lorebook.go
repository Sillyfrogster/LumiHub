package v1

import (
	"encoding/json"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/book"
	"github.com/google/uuid"
)

const lorebookEntryNamespace = "lorebook_entry"

func readLorebook(row LorebookRow) (readResult, error) {
	var payloads []json.RawMessage
	if json.Unmarshal(row.Entries, &payloads) != nil {
		return readResult{}, fmt.Errorf("v1 lorebook entries: expected a list")
	}
	entries, leftovers := book.Read(payloads)
	element := block.Element{
		ID: uuid.New(), Type: block.TypeEntryTable, Role: block.RoleLorebookEntries,
		Content: block.EntryTable{Entries: entries},
	}
	remainder := make([]format.Remainder, 0, len(leftovers))
	for _, entry := range entries {
		if fields, found := leftovers[entry.ID]; found {
			remainder = append(remainder, format.Remainder{
				Owner: format.OwnerItem, OwnerID: entry.ID,
				Namespace: lorebookEntryNamespace, Payload: fields,
			})
		}
	}
	answer := row.Common.IsNSFW
	created := row.Common.CreatedAt
	return readResult{parsed: format.Parsed{
		Kind: LorebookKind, Format: ID, Tags: append([]string(nil), row.Common.Tags...),
		IsNSFW: &answer, CreatedAt: &created,
		Header: format.Header{
			Name: row.Common.Name, Blurb: row.Common.Description, CreditedAuthor: row.Creator,
		},
		Elements: []block.Element{element}, Remainder: remainder,
	}}, nil
}
