package block

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

// definitions returns the definition of every block in page order.
func definitions(blocks []Block) []DefinitionID {
	out := make([]DefinitionID, len(blocks))
	for i, b := range blocks {
		out[i] = b.Definition
	}
	return out
}

func find(t *testing.T, blocks []Block, id DefinitionID) Block {
	t.Helper()
	for _, b := range blocks {
		if b.Definition == id {
			return b
		}
	}
	t.Fatalf("no %s block in %v", id, definitions(blocks))
	return Block{}
}

func roles(block Block) []Role {
	out := make([]Role, len(block.Elements))
	for i, element := range block.Elements {
		out[i] = element.Role
	}
	return out
}

func TestACharacterWithNothingInItStillHasItsRequiredBlocks(t *testing.T) {
	blocks, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place nothing: %v", err)
	}

	if got := definitions(blocks); len(got) != 2 || got[0] != CharacterCore || got[1] != Messages {
		t.Fatalf("blocks = %v, want the two required ones in catalog order", got)
	}
	for _, block := range blocks {
		if !block.Empty() {
			t.Errorf("%s holds content on an asset built from nothing", block.Definition)
		}
	}

	core := find(t, blocks, CharacterCore)
	wantRoles := []Role{RoleDescription, RolePersonality, RoleScenario}
	if got := roles(core); len(got) != 3 || got[0] != wantRoles[0] ||
		got[1] != wantRoles[1] || got[2] != wantRoles[2] {
		t.Errorf("character core roles = %v, want %v", got, wantRoles)
	}
	for _, element := range core.Elements {
		if element.Type != TypeProse {
			t.Errorf("%s is a %s element, want prose", element.Role, element.Type)
		}
		if element.ID == uuid.Nil {
			t.Errorf("%s has no id", element.Role)
		}
	}
}

func TestAnEmptyCharacterTakesTheWidthsAndLayoutsTheCatalogDeclares(t *testing.T) {
	blocks, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place nothing: %v", err)
	}

	core := find(t, blocks, CharacterCore)
	if core.Width != TwoThirds {
		t.Errorf("character core width = %q, want two thirds", core.Width)
	}
	if core.Layout != Stack3 {
		t.Errorf("character core layout = %q, want stack-3", core.Layout)
	}
	if got := core.Elements[0].Slot; got != "top" {
		t.Errorf("first character core slot = %q, want the layout's first", got)
	}

	messages := find(t, blocks, Messages)
	if messages.Width != Full {
		t.Errorf("messages width = %q, want full", messages.Width)
	}
	if core.Position != 0 || messages.Position != 1 {
		t.Errorf("positions = %d and %d, want catalog order", core.Position, messages.Position)
	}
}

func TestGroupOnlyGreetingsDecideHowManySlotsMessagesTakes(t *testing.T) {
	withoutGroup := []Element{{
		Type: TypeTextSet, Role: RoleGreetings,
		Content: TextSet{Texts: []TextItem{{Text: "Hello there."}}},
	}}
	withGroup := append(withoutGroup, Element{
		Type: TypeTextSet, Role: RoleGroupGreetings,
		Content: TextSet{Texts: []TextItem{{Text: "You all made it."}}},
	})

	two, err := Place("character", withoutGroup)
	if err != nil {
		t.Fatalf("place greetings: %v", err)
	}
	if got := find(t, two, Messages).Layout; got != Stack2 {
		t.Errorf("layout without group greetings = %q, want stack-2", got)
	}

	three, err := Place("character", withGroup)
	if err != nil {
		t.Fatalf("place greetings and group greetings: %v", err)
	}
	messages := find(t, three, Messages)
	if messages.Layout != Stack3 {
		t.Errorf("layout with group greetings = %q, want stack-3", messages.Layout)
	}
	if got := roles(messages); len(got) != 3 || got[2] != RoleGroupGreetings {
		t.Errorf("messages roles = %v, want group greetings last", got)
	}
}

func TestASourceWithNoGalleryGetsNoGalleryBlock(t *testing.T) {
	without, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place nothing: %v", err)
	}
	for _, block := range without {
		if block.Definition == Gallery {
			t.Fatalf("a source with no images produced an empty gallery block")
		}
	}

	with, err := Place("character", []Element{{
		Type: TypeImageSet, Role: RoleGallery,
		Content: ImageSet{Images: []ImageItem{{MediaID: uuid.New(), Name: "In the archive"}}},
	}})
	if err != nil {
		t.Fatalf("place a gallery: %v", err)
	}
	gallery := find(t, with, Gallery)
	if gallery.Width != Half || gallery.Layout != Single {
		t.Errorf("gallery = %q at %q, want a single layout at half width",
			gallery.Layout, gallery.Width)
	}
	if gallery.Position != 2 {
		t.Errorf("gallery position = %d, want it after the two required blocks", gallery.Position)
	}
}

func TestRequiredCoreElementsArePinnedToTheirBlock(t *testing.T) {
	blocks, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place nothing: %v", err)
	}
	core := find(t, blocks, CharacterCore)
	for _, role := range []Role{RoleDescription, RolePersonality, RoleScenario} {
		if !core.Pinned(role, "character") {
			t.Errorf("%s is not pinned to the character core", role)
		}
	}
	messages := find(t, blocks, Messages)
	if messages.Pinned(RoleGroupGreetings, "character") {
		t.Errorf("group-only greetings are pinned, so a creator could not remove them")
	}
}

func TestPlacementRefusesRatherThanDroppingWhatItCannotPut(t *testing.T) {
	_, err := Place("unknown", nil)
	if err == nil {
		t.Errorf("a kind with no catalog was placed anyway")
	}

	// A sampler setting belongs to a preset, and a character has nowhere for
	// it, so placement says so rather than dropping it quietly.
	_, err = Place("character", []Element{{
		Type: TypeFieldList, Role: "sampler_settings",
		Content: FieldList{Fields: []FieldItem{{Name: "Temperature", Value: "0.9"}}},
	}})
	if err == nil {
		t.Errorf("an element the catalog has nowhere for was accepted")
	}

	_, err = Place("character", []Element{{
		Type: TypeTextSet, Role: RoleDescription,
		Content: TextSet{Texts: []TextItem{{Text: "Wrong shape."}}},
	}})
	if err == nil {
		t.Errorf("a description arrived as a text set and was accepted")
	}
}

func TestASingularRoleTakesOneElementHoldingItsList(t *testing.T) {
	if RoleGreetings.Cardinality() != Singular {
		t.Fatalf("greetings is not singular")
	}
	blocks, err := Place("character", []Element{{
		Type: TypeTextSet, Role: RoleGreetings,
		Content: TextSet{Texts: []TextItem{
			{Text: "Hello there."}, {Text: "You again."}, {Text: "Late, as ever."},
		}},
	}})
	if err != nil {
		t.Fatalf("place three greetings: %v", err)
	}
	messages := find(t, blocks, Messages)
	greetings := 0
	for _, element := range messages.Elements {
		if element.Role == RoleGreetings {
			greetings++
		}
	}
	if greetings != 1 {
		t.Errorf("greetings elements = %d, want one holding the whole list", greetings)
	}
}

func TestTheCharacterCatalogDeclaresItsFiveOptionalSections(t *testing.T) {
	definitions, ok := Catalog("character")
	if !ok {
		t.Fatalf("a character has no catalog")
	}
	byID := make(map[DefinitionID]Definition, len(definitions))
	order := make([]DefinitionID, 0, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = definition
		order = append(order, definition.ID)
	}

	want := []struct {
		id      DefinitionID
		roles   []Role
		types   []Type
		layouts []Layout
		width   Width
	}{
		{Expressions, []Role{RoleExpressions}, []Type{TypeImageSet}, []Layout{Single}, Full},
		{Lorebook, []Role{RoleLorebookEntries}, []Type{TypeEntryTable}, []Layout{Single}, TwoThirds},
		{ImagePrompts, []Role{"", "", ""}, []Type{TypeFieldList, TypeProse, TypeProse}, []Layout{Stack3}, TwoThirds},
		{ModelInstructions, []Role{RoleSystemPrompt, RolePostHistoryInstructions},
			[]Type{TypeProse, TypeProse}, []Layout{Stack2, Duo}, Half},
		{Relationships, []Role{""}, []Type{TypeLinkList}, []Layout{Single}, Third},
	}
	for _, expected := range want {
		definition, listed := byID[expected.id]
		if !listed {
			t.Errorf("%s is not in the character catalog", expected.id)
			continue
		}
		if definition.Required {
			t.Errorf("%s is required, and the catalog declares it optional", expected.id)
		}
		if len(definition.Elements) != len(expected.roles) {
			t.Errorf("%s declares %d elements, want %d",
				expected.id, len(definition.Elements), len(expected.roles))
			continue
		}
		for i, element := range definition.Elements {
			if element.Role != expected.roles[i] || element.Type != expected.types[i] {
				t.Errorf("%s element %d is %s/%s, want %s/%s", expected.id, i+1,
					element.Role, element.Type, expected.roles[i], expected.types[i])
			}
		}
		if !slices.Equal(definition.Layouts, expected.layouts) {
			t.Errorf("%s offers layouts %v, want %v", expected.id, definition.Layouts, expected.layouts)
		}
		if definition.Width != expected.width {
			t.Errorf("%s is %s wide, want %s", expected.id, definition.Width, expected.width)
		}
	}

	wantOrder := []DefinitionID{
		CharacterCore, Messages, Expressions, Lorebook,
		ImagePrompts, ModelInstructions, Relationships,
		Gallery, Usage, Changelog, Attributes, AuthorNotes, RunsBestWith, CustomSection,
	}
	if !slices.Equal(order, wantOrder) {
		t.Errorf("the character catalog reads %v, want %v", order, wantOrder)
	}
}

func TestALorebookRoleIsNotNamedForTheFieldOneFormatUses(t *testing.T) {
	// The role is named for the content, so a standalone lorebook and a
	// character's embedded book are the same role.
	if !RoleLorebookEntries.Allows(TypeEntryTable) {
		t.Errorf("lorebook entries cannot be held in an entry table")
	}
	if RoleLorebookEntries.Allows(TypeProse) {
		t.Errorf("lorebook entries can be held in prose")
	}
	if RoleExpressions.Cardinality() != Singular {
		t.Errorf("a character may carry more than one expression set")
	}
	if RoleGallery.Cardinality() != Repeatable {
		t.Errorf("a gallery cannot repeat")
	}
	if RoleGroupGreetings == RoleGreetings {
		t.Errorf("group-only greetings share a role with ordinary ones")
	}
}

func TestExpressionsArePlacedWhereASourceCarriesThem(t *testing.T) {
	images := []Element{{
		ID: uuid.New(), Type: TypeImageSet, Role: RoleExpressions,
		Content: ImageSet{Images: []ImageItem{
			{MediaID: uuid.New(), Name: "hey there. do you feel better now?"},
			{MediaID: uuid.New(), Name: "Alexandra_neutral"},
		}},
	}}
	blocks, err := Place("character", images)
	if err != nil {
		t.Fatalf("place an expression set: %v", err)
	}
	set := find(t, blocks, Expressions)
	if len(set.Elements) != 1 || set.Elements[0].Role != RoleExpressions {
		t.Fatalf("the expressions block holds %v", roles(set))
	}
	// Free text, whatever the source called them. A closed vocabulary of
	// emotions would have refused or mangled eleven of twelve real sets.
	names := set.Elements[0].Content.(ImageSet).Images
	if names[0].Name != "hey there. do you feel better now?" || names[1].Name != "Alexandra_neutral" {
		t.Errorf("expression names came back as %q and %q", names[0].Name, names[1].Name)
	}

	// Nothing identified these as expressions, so nothing guesses.
	gallery := []Element{{
		ID: uuid.New(), Type: TypeImageSet, Role: RoleGallery,
		Content: ImageSet{Images: []ImageItem{{MediaID: uuid.New()}}},
	}}
	blocks, err = Place("character", gallery)
	if err != nil {
		t.Fatalf("place a gallery: %v", err)
	}
	for _, holder := range blocks {
		if holder.Definition == Expressions {
			t.Errorf("images nothing named as expressions became an expression set")
		}
	}
}

func TestUnroledPageContentUsesTheCatalogsTypeSlots(t *testing.T) {
	elements := []Element{
		{ID: uuid.New(), Type: TypeProse, Content: Prose{Text: "Read this first."}},
		{ID: uuid.New(), Type: TypeTextSet, Content: TextSet{Texts: []TextItem{{Text: "Changed."}}}},
	}
	blocks, err := Place("preset", elements)
	if err != nil {
		t.Fatalf("place unroled page content: %v", err)
	}
	if find(t, blocks, Usage).Elements[0].Type != TypeProse ||
		find(t, blocks, Changelog).Elements[0].Type != TypeTextSet {
		t.Error("the catalog did not place unroled content by its declared type")
	}
}
