package block

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const oneEntry = `{"entries":[{` +
	`"name":"The Ledger","keys":["ledger","the ledger"],"secondaryKeys":["Mira"],` +
	`"selective":true,"caseSensitive":false,"constant":false,"enabled":true,` +
	`"order":120,"position":"after_character",` +
	`"recursion":{"exclude":true,"delayUntil":true},` +
	`"text":"Every debt in the town is written in it."}]}`

func TestAnEntryKeepsItsKeyMatchingAndItsRecursionSettings(t *testing.T) {
	content, err := DecodeContent(TypeEntryTable, []byte(oneEntry))
	if err != nil {
		t.Fatalf("read an entry: %v", err)
	}
	entry := content.(EntryTable).Entries[0]
	if len(entry.Keys) != 2 || entry.Keys[1] != "the ledger" {
		t.Errorf("keys = %v, want both of them in order", entry.Keys)
	}
	if len(entry.SecondaryKeys) != 1 || !entry.Selective {
		t.Errorf("the secondary key match was lost: %+v", entry)
	}
	if entry.Order != 120 || entry.Position != AfterCharacter {
		t.Errorf("order and position = %d and %q", entry.Order, entry.Position)
	}
	if !entry.Recursion.Exclude || !entry.Recursion.DelayUntil || entry.Recursion.Prevent {
		t.Errorf("recursion settings = %+v, want exclude and delay only", entry.Recursion)
	}

	written, err := json.Marshal(Element{
		ID: uuid.New(), Type: TypeEntryTable, Role: RoleLorebookEntries, Slot: "main",
		Content: content,
	})
	if err != nil {
		t.Fatalf("write the book: %v", err)
	}
	var read Element
	if err := json.Unmarshal(written, &read); err != nil {
		t.Fatalf("read the book back: %v", err)
	}
	if got := read.Content.(EntryTable).Entries[0]; !reflect.DeepEqual(got, entry) {
		t.Errorf("the entry came back as %+v, want %+v", got, entry)
	}
}

func TestAnEntryAtAnUnknownPositionNamesTheChoices(t *testing.T) {
	_, err := DecodeContent(TypeEntryTable, []byte(
		`{"entries":[{"keys":["ledger"],"enabled":true,"position":"at_depth_4","text":"x"}]}`,
	))
	if err == nil {
		t.Fatalf("an entry was saved at a position nothing can read")
	}
	if !strings.Contains(err.Error(), "before") || !strings.Contains(err.Error(), "after") {
		t.Errorf("refusal = %q, want it to name where an entry can sit", err)
	}
}

func TestAnEntryWithoutTextCountsForNothing(t *testing.T) {
	empty, err := TypeEntryTable.Empty()
	if err != nil {
		t.Fatalf("empty entry table: %v", err)
	}
	if !empty.Empty() {
		t.Errorf("a new book already holds something")
	}
	keysOnly := EntryTable{Entries: []Entry{{Keys: []string{"ledger"}, Enabled: true}}}
	if !keysOnly.Empty() {
		t.Errorf("an entry with keys and no text reads as filled")
	}
	if (EntryTable{Entries: []Entry{{Text: "Written."}}}).Empty() {
		t.Errorf("an entry with text reads as empty")
	}
}

func TestAnElementSaysHowMuchItHoldsAndNeverHowManyTokens(t *testing.T) {
	entries := make([]Entry, 1004)
	for i := range entries {
		entries[i] = Entry{Text: "An entry.", Enabled: i >= 6}
	}
	book := Element{Type: TypeEntryTable, Role: RoleLorebookEntries, Content: EntryTable{Entries: entries}}
	facts := book.Facts()
	if len(facts) != 2 || facts[0] != "1,004 entries" || facts[1] != "998 switched on" {
		t.Errorf("a book of 1,004 entries says %v", facts)
	}

	whole := Element{Type: TypeEntryTable, Role: RoleLorebookEntries, Content: EntryTable{
		Entries: []Entry{{Text: "One.", Enabled: true}},
	}}
	if got := whole.Facts(); len(got) != 1 || got[0] != "1 entry" {
		t.Errorf("a book with every entry on says %v, want the count alone", got)
	}

	greetings := Element{Type: TypeTextSet, Role: RoleGreetings, Content: TextSet{
		Texts: []TextItem{{Text: "Hello."}, {Text: "You again."}},
	}}
	if got := greetings.Facts(); len(got) != 1 || got[0] != "2 greetings" {
		t.Errorf("two greetings say %v", got)
	}

	if got := (Element{Type: TypeProse, Content: Prose{Text: strings.Repeat("word ", 400)}}).Facts(); len(got) != 0 {
		t.Errorf("prose measured itself: %v", got)
	}
	if got := (Element{Type: TypeImageSet, Content: ImageSet{}}).Facts(); len(got) != 0 {
		t.Errorf("an empty element counted itself: %v", got)
	}
}
