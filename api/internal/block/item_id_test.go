package block

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// Preserved data keys against an item's id, so an item that arrives without
// one gets one before anything can point at its position instead.
func TestEveryItemInsideAnElementGetsAnIDWhenItIsSaved(t *testing.T) {
	cases := []struct {
		elementType Type
		body        string
	}{
		{TypeTextSet, `{"texts":[{"text":"Hello again."},{"text":"And again."}]}`},
		{TypeDialogueSample, `{"turns":[{"speaker":"Mira","text":"Sit down."}]}`},
		{TypeFieldList, `{"fields":[{"name":"Height","value":"Six feet"}]}`},
		{TypeLinkList, `{"links":[{"label":"The book","url":"https://illarin.xyz/a/1"}]}`},
		{TypeImageSet, `{"images":[{"mediaId":"` + uuid.New().String() + `"}]}`},
		{TypeEntryTable, `{"entries":[{"keys":["ledger"],"enabled":true,"text":"A debt."}]}`},
	}
	for _, item := range cases {
		content, err := DecodeContent(item.elementType, []byte(item.body))
		if err != nil {
			t.Fatalf("read %s: %v", item.elementType, err)
		}
		ids := ItemIDs(content)
		if len(ids) == 0 {
			t.Fatalf("%s reported no items", item.elementType)
		}
		seen := make(map[uuid.UUID]bool, len(ids))
		for _, id := range ids {
			if id == uuid.Nil {
				t.Errorf("%s saved an item with no id", item.elementType)
			}
			if seen[id] {
				t.Errorf("%s gave two items the same id", item.elementType)
			}
			seen[id] = true
		}
	}
}

// An id a creator's browser sends back is the id the item already had, so
// preserved data stays with the item across a save.
func TestASavedItemKeepsTheIDItArrivedWith(t *testing.T) {
	kept := uuid.New()
	content, err := DecodeContent(TypeTextSet, []byte(
		`{"texts":[{"id":"`+kept.String()+`","text":"Hello again."},{"text":"New."}]}`,
	))
	if err != nil {
		t.Fatalf("read a text set: %v", err)
	}
	texts := content.(TextSet).Texts
	if texts[0].ID != kept {
		t.Errorf("id = %s, want the id the item arrived with", texts[0].ID)
	}
	if texts[1].ID == uuid.Nil || texts[1].ID == kept {
		t.Errorf("the new item got id %s", texts[1].ID)
	}
}

// Content written before items had ids is read forward, so an entry saved by
// an older build cannot leave preserved data pointing at nothing.
func TestItemsWrittenBeforeIDsExistedAreReadForward(t *testing.T) {
	stored := `{"id":"` + uuid.New().String() + `","type":"entry_table","slot":"main",` +
		`"version":1,"options":{},"content":{"entries":[` +
		`{"keys":["ledger"],"enabled":true,"text":"A debt."},` +
		`{"keys":["mira"],"enabled":true,"text":"A name."}]}}`

	var element Element
	if err := json.Unmarshal([]byte(stored), &element); err != nil {
		t.Fatalf("read stored entries: %v", err)
	}
	entries := element.Content.(EntryTable).Entries
	if len(entries) != 2 {
		t.Fatalf("read %d entries, want both", len(entries))
	}
	if entries[0].ID == uuid.Nil || entries[1].ID == uuid.Nil {
		t.Errorf("an upgraded entry has no id: %+v", entries)
	}
	if entries[0].ID == entries[1].ID {
		t.Errorf("both upgraded entries got the same id")
	}
}

// A block save reconciles preserved data against the ids that survived, so
// this list is what the reconciliation reads.
func TestItemIDsReportsEveryItemInAnElement(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	content := TextSet{Texts: []TextItem{{ID: first, Text: "One"}, {ID: second, Text: "Two"}}}
	ids := ItemIDs(content)
	if len(ids) != 2 || ids[0] != first || ids[1] != second {
		t.Errorf("item ids = %v, want both in order", ids)
	}
	if got := ItemIDs(Prose{Text: "No items here."}); len(got) != 0 {
		t.Errorf("prose reported %v, want no items", got)
	}
}
