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
		Gallery, Usage, Changelog, Attributes, AuthorNotes, RunsBestWith, CustomSection,
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
	if offerFor(t, offers, CustomSection).Group != GroupOther {
		t.Errorf("a custom section is not offered under anything else")
	}
	for _, group := range Groups() {
		if group.Title() == "" {
			t.Errorf("group %q has no wording", group)
		}
	}
}

func TestOnlyACustomSectionRepeats(t *testing.T) {
	offers, _ := Offers("character")
	for _, offer := range offers {
		if offer.Repeatable != (offer.Definition == CustomSection) {
			t.Errorf("%s repeatable = %v", offer.Definition, offer.Repeatable)
		}
	}
	if got := len(offerFor(t, offers, CustomSection).Choices); got < 4 {
		t.Errorf("a custom section offers %d elements, want text, images, links and a list", got)
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

	custom, err := NewBlock("character", CustomSection, TypeLinkList, append(page, gallery))
	if err != nil {
		t.Fatalf("add a custom section: %v", err)
	}
	if len(custom.Elements) != 1 || custom.Elements[0].Type != TypeLinkList {
		t.Fatalf("a custom section arrived holding %v", custom.Elements)
	}
	if custom.Elements[0].Role != "" {
		t.Errorf("a custom section's element carries role %q, want none", custom.Elements[0].Role)
	}
}

func TestASecondCustomSectionIsAllowedAndASecondGalleryIsNot(t *testing.T) {
	page, _ := Place("character", nil)
	gallery, err := NewBlock("character", Gallery, TypeImageSet, page)
	if err != nil {
		t.Fatalf("add a gallery: %v", err)
	}
	page = append(page, gallery)

	if _, err := NewBlock("character", Gallery, TypeImageSet, page); err == nil {
		t.Errorf("a second gallery was allowed")
	}

	first, err := NewBlock("character", CustomSection, TypeProse, page)
	if err != nil {
		t.Fatalf("add a custom section: %v", err)
	}
	if _, err := NewBlock("character", CustomSection, TypeProse, append(page, first)); err != nil {
		t.Errorf("a second custom section was refused: %v", err)
	}
}

func TestABlockCannotStartWithAnElementItsDefinitionDoesNotOffer(t *testing.T) {
	page, _ := Place("character", nil)
	if _, err := NewBlock("character", Gallery, TypeProse, page); err == nil {
		t.Errorf("a gallery was started with prose")
	}
	if _, err := NewBlock("character", CustomSection, TypeDialogueSample, page); err == nil {
		t.Errorf("a custom section was started with a dialogue sample")
	}
	if _, err := NewBlock("character", CharacterCore, TypeProse, page); err == nil {
		t.Errorf("a required block was added from the tray")
	}
}

func TestAPageWithTwoCustomSectionsPassesValidationAndTwoGalleriesDoNot(t *testing.T) {
	page, _ := Place("character", nil)
	first, _ := NewBlock("character", CustomSection, TypeProse, page)
	second, _ := NewBlock("character", CustomSection, TypeProse, append(page, first))
	repeated := append(slices.Clone(page), first, second)
	if err := ValidateBuilderConstraints("character", repeated, repeated); err != nil {
		t.Errorf("two custom sections were refused: %v", err)
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
