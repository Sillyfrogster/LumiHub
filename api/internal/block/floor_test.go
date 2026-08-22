package block

import (
	"testing"

	"github.com/google/uuid"
)

// filled returns the asset's blocks with one role carrying content.
func filled(t *testing.T, role Role, content Content) []Block {
	t.Helper()
	blocks, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place a character: %v", err)
	}
	for i := range blocks {
		for j := range blocks[i].Elements {
			if blocks[i].Elements[j].Role == role {
				blocks[i].Elements[j].Content = content
				return blocks
			}
		}
	}
	t.Fatalf("no %s element on a character", role)
	return nil
}

func checkNamed(t *testing.T, checks []Check, id string) Check {
	t.Helper()
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("no %s requirement in the character floor", id)
	return Check{}
}

func TestTheCharacterFloorAsksForADescriptionAndAGreeting(t *testing.T) {
	blocks, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place a character: %v", err)
	}

	checks := ContentFloor("character", blocks)

	if len(checks) != 2 {
		t.Fatalf("character floor = %d requirements, want two", len(checks))
	}
	for _, check := range checks {
		if check.Met {
			t.Errorf("%s reads as met on an empty character", check.ID)
		}
		if check.Label == "" || check.Detail == "" {
			t.Errorf("%s = %+v, want wording a creator can act on", check.ID, check)
		}
		if check.BlockID == nil {
			t.Errorf("%s names no block for a creator to open", check.ID)
		}
	}
}

func TestTheFloorReadsContentRatherThanThePresenceOfABlock(t *testing.T) {
	written := filled(t, RoleDescription, Prose{Text: "She keeps the books that forget themselves."})

	description := checkNamed(t, ContentFloor("character", written), "description")
	greeting := checkNamed(t, ContentFloor("character", written), "greetings")

	if !description.Met {
		t.Error("a description with content does not meet the floor")
	}
	if greeting.Met {
		t.Error("an empty greeting element meets the floor")
	}
}

func TestAnEmptyGreetingDoesNotStandInForAWrittenOne(t *testing.T) {
	blank := filled(t, RoleGreetings, TextSet{Texts: []TextItem{{Text: ""}}})
	written := filled(t, RoleGreetings, TextSet{Texts: []TextItem{{Text: "Come in."}}})

	if checkNamed(t, ContentFloor("character", blank), "greetings").Met {
		t.Error("a greeting with no text meets the floor")
	}
	if !checkNamed(t, ContentFloor("character", written), "greetings").Met {
		t.Error("a written greeting does not meet the floor")
	}
}

func TestTheFloorPointsAtTheBlockHoldingTheContent(t *testing.T) {
	blocks, err := Place("character", nil)
	if err != nil {
		t.Fatalf("place a character: %v", err)
	}
	byDefinition := map[DefinitionID]uuid.UUID{}
	for _, b := range blocks {
		byDefinition[b.Definition] = b.ID
	}

	checks := ContentFloor("character", blocks)

	if got := *checkNamed(t, checks, "description").BlockID; got != byDefinition[CharacterCore] {
		t.Errorf("description points at %s, want the character core", got)
	}
	if got := *checkNamed(t, checks, "greetings").BlockID; got != byDefinition[Messages] {
		t.Errorf("greetings points at %s, want messages", got)
	}
}

func TestAKindWithNoFloorAsksForNothingBeyondTheHeader(t *testing.T) {
	if checks := ContentFloor("pack", nil); len(checks) != 0 {
		t.Errorf("pack floor = %+v, want nothing until the kind is built", checks)
	}
}
