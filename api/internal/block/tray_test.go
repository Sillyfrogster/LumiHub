package block

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func offerFor(t *testing.T, offers []Offer, id DefinitionID) Offer {
	t.Helper()
	for _, offer := range offers {
		if offer.Definition == id {
			return offer
		}
	}
	t.Fatalf("no %s in the tray", id)
	return Offer{}
}

func TestTheTrayOffersTheSharedSevenGroupedByWhereTheContentEndsUp(t *testing.T) {
	offers, ok := Offers("character")
	if !ok {
		t.Fatalf("character has no tray")
	}

	for _, id := range []DefinitionID{
		Gallery, Usage, Changelog, Attributes, AuthorNotes, RunsBestWith, CustomBlock,
	} {
		offer := offerFor(t, offers, id)
		if offer.Title == "" || offer.Summary == "" {
			t.Errorf("%s is offered without a name and a line saying what it is for", id)
		}
		if !slices.Contains(Groups(), offer.Group) {
			t.Errorf("%s is in group %q, which the tray does not name", id, offer.Group)
		}
		if len(offer.Choices) == 0 {
			t.Errorf("%s offers no element to start with", id)
		}
	}

	if offerFor(t, offers, Gallery).Group != GroupFile {
		t.Errorf("a gallery is not offered as content that travels with the file")
	}
	if offerFor(t, offers, CustomBlock).Group != GroupOther {
		t.Errorf("a custom block is not offered under anything else")
	}
	for _, group := range Groups() {
		if group.Title() == "" {
			t.Errorf("group %q has no wording", group)
		}
	}
}

func TestOnlyACustomBlockRepeats(t *testing.T) {
	offers, _ := Offers("character")
	for _, offer := range offers {
		if offer.Repeatable != (offer.Definition == CustomBlock) {
			t.Errorf("%s repeatable = %v", offer.Definition, offer.Repeatable)
		}
	}
	if got := len(offerFor(t, offers, CustomBlock).Choices); got < 4 {
		t.Errorf("a custom block offers %d elements, want text, images, links and a list", got)
	}
}

func TestARequiredDefinitionIsNeverOffered(t *testing.T) {
	offers, _ := Offers("character")
	for _, offer := range offers {
		if offer.Definition == CharacterCore || offer.Definition == Messages {
			t.Errorf("%s is required and cannot be added", offer.Definition)
		}
	}
}

func TestANewBlockArrivesHoldingTheElementTheCreatorAskedFor(t *testing.T) {
	page, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place nothing: %v", err)
	}

	gallery, err := NewBlock("character", Gallery, TypeImageSet, page)
	if err != nil {
		t.Fatalf("add a gallery: %v", err)
	}
	if gallery.Position != len(page) {
		t.Errorf("the new block is at position %d, want the foot of the page", gallery.Position)
	}
	if len(gallery.Elements) != 1 || gallery.Elements[0].Type != TypeImageSet {
		t.Fatalf("a gallery arrived holding %d elements", len(gallery.Elements))
	}
	if gallery.Elements[0].Role != RoleGallery {
		t.Errorf("the gallery's element carries role %q", gallery.Elements[0].Role)
	}
	if gallery.Elements[0].Slot != Single.Slots()[0] {
		t.Errorf("the element sits in slot %q, want the layout's first", gallery.Elements[0].Slot)
	}
	if gallery.Width != Half || gallery.Layout != Single {
		t.Errorf("the gallery arrived %s at %s, want the catalog's declared pair", gallery.Layout, gallery.Width)
	}
	if !gallery.Empty() {
		t.Errorf("a new gallery already holds content")
	}
	if err := ValidateStructure(gallery); err != nil {
		t.Errorf("a new gallery does not pass structural validation: %v", err)
	}

	custom, err := NewBlock("character", CustomBlock, TypeLinkList, append(page, gallery))
	if err != nil {
		t.Fatalf("add a custom block: %v", err)
	}
	if len(custom.Elements) != 1 || custom.Elements[0].Type != TypeLinkList {
		t.Fatalf("a custom block arrived holding %v", custom.Elements)
	}
	if custom.Elements[0].Role != "" {
		t.Errorf("a custom block's element carries role %q, want none", custom.Elements[0].Role)
	}
}

func TestASecondCustomBlockIsAllowedAndASecondGalleryIsNot(t *testing.T) {
	page, _ := Place("character", nil)
	gallery, err := NewBlock("character", Gallery, TypeImageSet, page)
	if err != nil {
		t.Fatalf("add a gallery: %v", err)
	}
	page = append(page, gallery)

	if _, err := NewBlock("character", Gallery, TypeImageSet, page); err == nil {
		t.Errorf("a second gallery was allowed")
	}

	first, err := NewBlock("character", CustomBlock, TypeProse, page)
	if err != nil {
		t.Fatalf("add a custom block: %v", err)
	}
	if _, err := NewBlock("character", CustomBlock, TypeProse, append(page, first)); err != nil {
		t.Errorf("a second custom block was refused: %v", err)
	}
}

func TestABlockCannotStartWithAnElementItsDefinitionDoesNotOffer(t *testing.T) {
	page, _ := Place("character", nil)
	if _, err := NewBlock("character", Gallery, TypeProse, page); err == nil {
		t.Errorf("a gallery was started with prose")
	}
	if _, err := NewBlock("character", CustomBlock, TypeDialogueSample, page); err == nil {
		t.Errorf("a custom block was started with a dialogue sample")
	}
	if _, err := NewBlock("character", CharacterCore, TypeProse, page); err == nil {
		t.Errorf("a required block was added from the tray")
	}
}

func TestAPageWithTwoCustomBlocksPassesValidationAndTwoGalleriesDoNot(t *testing.T) {
	page, _ := Place("character", nil)
	first, _ := NewBlock("character", CustomBlock, TypeProse, page)
	second, _ := NewBlock("character", CustomBlock, TypeProse, append(page, first))
	repeated := append(slices.Clone(page), first, second)
	if err := ValidateBuilderConstraints("character", repeated, repeated); err != nil {
		t.Errorf("two custom blocks were refused: %v", err)
	}

	gallery, _ := NewBlock("character", Gallery, TypeImageSet, page)
	twin := gallery
	twin.ID = uuid.New()
	twin.Position = gallery.Position + 1
	twin.Elements = []Element{{
		ID: uuid.New(), Type: TypeImageSet, Role: RoleGallery, Slot: Single.Slots()[0],
		Options: Options{ItemSize: ItemMedium}, Content: ImageSet{Images: []ImageItem{}},
	}}
	doubled := append(slices.Clone(page), gallery, twin)
	if err := ValidateBuilderConstraints("character", doubled, doubled); err == nil {
		t.Errorf("two galleries were allowed on one page")
	}
}

func TestABlockWithMoreThanOneElementArrivesWholeRatherThanInPieces(t *testing.T) {
	// There is no route that adds an element to a block later, so a
	// definition that declares three of them has to arrive holding all three.
	prompts, err := NewBlock("character", ImagePrompts, TypeFieldList, nil)
	if err != nil {
		t.Fatalf("add image prompts: %v", err)
	}
	if got := len(prompts.Elements); got != 3 {
		t.Fatalf("image prompts arrived with %d elements, want the three it declares", got)
	}
	if prompts.Layout != Stack3 {
		t.Errorf("image prompts arrived in %s, want the layout with room for three", prompts.Layout)
	}
	wantTypes := []Type{TypeFieldList, TypeProse, TypeProse}
	for i, element := range prompts.Elements {
		if element.Type != wantTypes[i] {
			t.Errorf("element %d is a %s, want %s", i+1, element.Type, wantTypes[i])
		}
		if element.Slot != Stack3.Slots()[i] {
			t.Errorf("element %d is in slot %q, want %q", i+1, element.Slot, Stack3.Slots()[i])
		}
	}
	if prompts.Elements[1].Options.Display != DisplayVerbatim {
		t.Errorf("a prompt body is shown as %q, want verbatim", prompts.Elements[1].Options.Display)
	}

	instructions, err := NewBlock("character", ModelInstructions, TypeProse, nil)
	if err != nil {
		t.Fatalf("add model instructions: %v", err)
	}
	if got := roles(instructions); len(got) != 2 ||
		got[0] != RoleSystemPrompt || got[1] != RolePostHistoryInstructions {
		t.Errorf("model instructions arrived with roles %v, want both prompts", got)
	}
}

func TestABlockThatArrivesWholeIsOfferedAsOneChoice(t *testing.T) {
	offers, ok := Offers("character")
	if !ok {
		t.Fatalf("a character has no add tray")
	}
	byDefinition := make(map[DefinitionID]Offer, len(offers))
	for _, offer := range offers {
		byDefinition[offer.Definition] = offer
	}
	prompts, offered := byDefinition[ImagePrompts]
	if !offered {
		t.Fatalf("image prompts are not offered on a character")
	}
	if len(prompts.Choices) != 1 {
		t.Errorf("image prompts are offered as %d choices, want one whole block", len(prompts.Choices))
	}
	if len(byDefinition[CustomBlock].Choices) < 2 {
		t.Errorf("a custom block is offered without the elements it can start with")
	}
}
