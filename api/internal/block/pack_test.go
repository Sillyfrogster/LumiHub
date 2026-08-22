package block

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestPackCatalogPlacesItsItemsAndOffersTheSharedSections(t *testing.T) {
	blocks, err := Place("pack", nil)
	if err != nil {
		t.Fatalf("place an empty pack: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("empty pack has %d blocks, want its core", len(blocks))
	}
	core := blocks[0]
	definition, ok := core.Definition.Definition("pack")
	if !ok {
		t.Fatal("the pack core is absent from the pack catalog")
	}
	if core.Definition != PackCore || !definition.Required || definition.Hideable ||
		core.Layout != Single || core.Width != Full {
		t.Fatalf("pack core = %+v / %+v, want a fixed full-width required block", core, definition)
	}
	if len(core.Elements) != 1 || core.Elements[0].Type != TypeRecordList ||
		core.Elements[0].Role != RolePackItems ||
		!core.Pinned(RolePackItems, "pack") {
		t.Fatalf("pack core elements = %+v, want pinned Lumia records", core.Elements)
	}

	offers, ok := Offers("pack")
	if !ok {
		t.Fatal("the pack kind has no add tray")
	}
	want := []DefinitionID{
		Gallery, Usage, Changelog, Attributes, AuthorNotes, RunsBestWith, CustomSection,
	}
	got := make([]DefinitionID, 0, len(offers))
	for _, offer := range offers {
		got = append(got, offer.Definition)
	}
	if !slices.Equal(got, want) {
		t.Errorf("pack add tray = %v, want %v", got, want)
	}
}

func TestPackRecordListUsesTheClosedLumiaSchema(t *testing.T) {
	decoded, err := DecodeContent(TypeRecordList, json.RawMessage(`{
		"schema":"lumia",
		"records":[{
			"lumiaName":"Archivist",
			"lumiaDefinition":"A guide to a quiet archive.",
			"lumiaPersonality":"Patient and exact.",
			"lumiaBehavior":"Answers with citations.",
			"genderIdentity":2,
			"authorName":"A creator",
			"version":3
		}]
	}`))
	if err != nil {
		t.Fatalf("decode Lumia records: %v", err)
	}
	records, ok := decoded.(RecordList)
	if !ok || records.Schema != LumiaRecordSchema || len(records.Records) != 1 {
		t.Fatalf("record list = %+v, want one Lumia record", decoded)
	}
	item := records.Records[0]
	if item.ID.String() == "00000000-0000-0000-0000-000000000000" ||
		item.LumiaName != "Archivist" || item.GenderIdentity != 2 || item.Version != 3 {
		t.Errorf("decoded Lumia = %+v", item)
	}

	if _, err := DecodeContent(TypeRecordList, json.RawMessage(`{"schema":"creator_defined","records":[]}`)); err == nil {
		t.Fatal("a creator-defined record schema was accepted")
	}
}

func TestPackNeedsOneItemBeforePublishingAndCountsItemsFromContent(t *testing.T) {
	blocks, err := Place("pack", nil)
	if err != nil {
		t.Fatalf("place an empty pack: %v", err)
	}
	checks := ContentFloor("pack", blocks)
	if len(checks) != 1 || checks[0].Met || checks[0].Role != RolePackItems {
		t.Fatalf("empty pack floor = %+v, want one unmet item requirement", checks)
	}

	records := RecordList{Schema: LumiaRecordSchema, Records: []LumiaRecord{
		{ID: NewItemID(), LumiaName: "Archivist", GenderIdentity: 2, Version: 1},
		{ID: NewItemID(), LumiaName: "Cartographer", GenderIdentity: 1, Version: 1},
	}}
	blocks[0].Elements[0].Content = records
	if !ContentFloor("pack", blocks)[0].Met {
		t.Error("a pack with items still cannot publish")
	}
	facts := blocks[0].Elements[0].Facts()
	if !slices.Equal(facts, []string{"2 items"}) {
		t.Errorf("pack facts = %v, want a computed item count", facts)
	}
}
