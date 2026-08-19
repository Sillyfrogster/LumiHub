package block

import (
	"slices"
	"testing"
)

// A lorebook is its entries. The kind has one section of its own, and a
// creator starting from nothing sees it before they have typed anything.
func TestAnEmptyLorebookIsOneRequiredSectionHoldingItsEntries(t *testing.T) {
	blocks, err := Place("lorebook", nil)
	if err != nil {
		t.Fatalf("place a lorebook: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("placed %d blocks, want one", len(blocks))
	}
	holder := blocks[0]
	if holder.Definition != LorebookCore {
		t.Fatalf("definition = %q, want the lorebook core", holder.Definition)
	}
	if holder.Layout != Single || holder.Width != Full {
		t.Errorf("arrangement = %s at %s, want a single slot at full width",
			holder.Layout, holder.Width)
	}
	if len(holder.Elements) != 1 {
		t.Fatalf("elements = %+v, want the entry table alone", holder.Elements)
	}
	entries := holder.Elements[0]
	if entries.Role != RoleLorebookEntries || entries.Type != TypeEntryTable {
		t.Errorf("element = %s/%s, want an entry table carrying the entries role",
			entries.Role, entries.Type)
	}
	if entries.Content == nil || !entries.Content.Empty() {
		t.Errorf("content = %+v, want an empty book to start from", entries.Content)
	}
}

// A lorebook with its entries withheld is not a page, so the one section it
// has can be neither hidden nor emptied out.
func TestTheEntriesSectionCannotBeHiddenOrHollowedOut(t *testing.T) {
	definition, ok := LorebookCore.Definition("lorebook")
	if !ok {
		t.Fatal("the lorebook catalog has no core section")
	}
	if !definition.Required || definition.Hideable {
		t.Errorf("definition = required %v hideable %v, want required and not hideable",
			definition.Required, definition.Hideable)
	}
	blocks, err := Place("lorebook", nil)
	if err != nil {
		t.Fatalf("place a lorebook: %v", err)
	}
	if !blocks[0].Pinned(RoleLorebookEntries, "lorebook") {
		t.Error("the entries can be taken off a lorebook")
	}
}

// Every kind lists the seven shared sections, so a lorebook can carry a
// gallery, a note on how to use it and the rest without a catalog of its own
// repeating them.
func TestALorebookIsOfferedTheSevenSharedSections(t *testing.T) {
	offers, ok := Offers("lorebook")
	if !ok {
		t.Fatal("the lorebook kind offers nothing")
	}
	offered := make([]DefinitionID, 0, len(offers))
	for _, offer := range offers {
		offered = append(offered, offer.Definition)
	}
	for _, shared := range []DefinitionID{
		Gallery, Usage, Changelog, Attributes, AuthorNotes, RunsBestWith, CustomSection,
	} {
		if !slices.Contains(offered, shared) {
			t.Errorf("%s is not offered on a lorebook", shared)
		}
	}
	// The add tray never offers a section that is on the page already, and it
	// never offers another kind's.
	for _, unwanted := range []DefinitionID{LorebookCore, CharacterCore, Messages, Lorebook} {
		if slices.Contains(offered, unwanted) {
			t.Errorf("%s is offered on a lorebook", unwanted)
		}
	}
}

func TestTheLorebookFloorAsksForOneEntry(t *testing.T) {
	blocks, err := Place("lorebook", nil)
	if err != nil {
		t.Fatalf("place a lorebook: %v", err)
	}
	checks := ContentFloor("lorebook", blocks)
	if len(checks) != 1 {
		t.Fatalf("lorebook floor = %d requirements, want one", len(checks))
	}
	entry := checks[0]
	if entry.Met {
		t.Error("an empty book is ready to publish")
	}
	if entry.Label == "" || entry.Detail == "" || entry.BlockID == nil {
		t.Errorf("requirement = %+v, want wording and a block a creator can open", entry)
	}

	blocks[0].Elements[0].Content = EntryTable{Entries: []Entry{{Keys: []string{"Eridu"}}}}
	if ContentFloor("lorebook", blocks)[0].Met {
		t.Error("an entry with keys and no text meets the floor")
	}
	blocks[0].Elements[0].Content = EntryTable{
		Entries: []Entry{{Keys: []string{"Eridu"}, Text: "The last city."}},
	}
	if !ContentFloor("lorebook", blocks)[0].Met {
		t.Error("a written entry does not meet the floor")
	}
}
