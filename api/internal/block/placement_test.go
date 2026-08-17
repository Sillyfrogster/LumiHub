package block

import (
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
	_, err := Place("pack", nil)
	if err == nil {
		t.Errorf("a kind with no catalog was placed anyway")
	}

	_, err = Place("character", []Element{{
		Type: TypeProse, Role: "system_prompt", Content: Prose{Text: "Stay in character."},
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
