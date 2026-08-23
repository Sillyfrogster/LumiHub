package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func addBlock(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID string,
	definition string,
	elementType string,
) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"definition": definition, "elementType": elementType,
	})
	if err != nil {
		t.Fatalf("encode the block to add: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost, "/v1/assets/"+assetID+"/blocks", strings.NewReader(string(body)),
	)
	request.Header.Set("Content-Type", "application/json")
	return send(t, r, authorized(request, session))
}

func addedBlock(t *testing.T, response *httptest.ResponseRecorder) startedBlock {
	t.Helper()
	if response.Code != http.StatusCreated {
		t.Fatalf("add a block: status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var added startedBlock
	if err := json.Unmarshal(response.Body.Bytes(), &added); err != nil {
		t.Fatalf("decode the new block: %v", err)
	}
	return added
}

func TestTheOwnerIsOfferedTheSharedBlocksGroupedByDestination(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	page := fetchStartedAsset(t, r, session, started.ID)
	byDefinition := make(map[string]addableBlock, len(page.AddableBlocks))
	for _, block := range page.AddableBlocks {
		byDefinition[block.Definition] = block
	}
	for _, definition := range []string{
		"gallery", "usage", "changelog", "attributes",
		"author_notes", "runs_best_with", "custom_block",
	} {
		block, ok := byDefinition[definition]
		if !ok {
			t.Fatalf("%s is not offered on a character", definition)
		}
		if block.Summary == "" || block.GroupTitle == "" || len(block.Choices) == 0 {
			t.Errorf("%s is offered as %+v, want a line, a group and an element", definition, block)
		}
	}
	if _, offered := byDefinition["character_core"]; offered {
		t.Errorf("a required block is offered in the add tray")
	}
	if byDefinition["gallery"].GroupTitle != "Content that travels with the file" {
		t.Errorf("a gallery is grouped under %q", byDefinition["gallery"].GroupTitle)
	}
	if !byDefinition["custom_block"].Repeatable {
		t.Errorf("a custom block does not repeat")
	}
}

func TestAddingABlockPutsItAtTheFootOfThePageHoldingItsElement(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	added := addedBlock(t, addBlock(t, r, session, started.ID, "gallery", "image_set"))

	if added.Definition != "gallery" || added.Position != 2 {
		t.Fatalf("the new block = %+v, want a gallery at the foot of the page", added)
	}
	if len(added.Elements) != 1 || added.Elements[0].Type != "image_set" {
		t.Fatalf("the new block holds %+v, want one image set", added.Elements)
	}
	if added.Elements[0].ItemSize == "" {
		t.Errorf("the gallery's images have no declared size")
	}
	if added.Required || !added.Hideable {
		t.Errorf("the new block is required = %v, hideable = %v", added.Required, added.Hideable)
	}
	if added.Width != "half" || added.Layout != "single" {
		t.Errorf("the new block arrived %s at %s, want the catalog's declared pair", added.Layout, added.Width)
	}

	page := fetchStartedAsset(t, r, session, started.ID)
	if len(page.Blocks) != 3 || page.Blocks[2].ID != added.ID {
		t.Errorf("the saved page = %d blocks, want the gallery last", len(page.Blocks))
	}
}

func TestABlockThatCannotRepeatIsRefusedTwice(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	addedBlock(t, addBlock(t, r, session, started.ID, "usage", "prose"))
	response := addBlock(t, r, session, started.ID, "usage", "prose")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("a second usage block: status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "already on this page") {
		t.Errorf("refusal = %s, want it to say the block is already there", response.Body.String())
	}
}

func TestACustomBlockRepeatsAndTakesTheElementTheCreatorChose(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	first := addedBlock(t, addBlock(t, r, session, started.ID, "custom_block", "prose"))
	second := addedBlock(t, addBlock(t, r, session, started.ID, "custom_block", "link_list"))

	if first.Elements[0].Type != "prose" || second.Elements[0].Type != "link_list" {
		t.Errorf("custom blocks hold %s and %s", first.Elements[0].Type, second.Elements[0].Type)
	}
	if second.Position != first.Position+1 {
		t.Errorf("the second custom block is at %d, want after the first", second.Position)
	}
}

func TestARequiredBlockAndAnUnofferedElementAreBothRefused(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	if response := addBlock(t, r, session, started.ID, "character_core", "prose"); response.Code != http.StatusBadRequest {
		t.Errorf("adding a required block: status = %d, want 400", response.Code)
	}
	if response := addBlock(t, r, session, started.ID, "gallery", "prose"); response.Code != http.StatusBadRequest {
		t.Errorf("starting a gallery with prose: status = %d, want 400", response.Code)
	}
	// A theme's core block belongs to another kind's catalog entirely.
	if response := addBlock(t, r, session, started.ID, "theme_core", "color_set"); response.Code != http.StatusBadRequest {
		t.Errorf("adding a block the kind has not got: status = %d, want 400", response.Code)
	}
}

func TestAnAddedBlockIsFilledAndReadBack(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	added := addedBlock(t, addBlock(t, r, session, started.ID, "runs_best_with", "link_list"))

	update := editableBlock(added)
	update.Elements[0].Content = json.RawMessage(
		`{"links":[{"label":"The winter lorebook","url":"https://illarin.xyz/a/1","note":"Load it first"}]}`,
	)
	if response := saveBlock(t, r, session, started.ID, added.ID, update); response.Code != http.StatusOK {
		t.Fatalf("save links: status = %d, want 200: %s", response.Code, response.Body.String())
	}

	page := fetchStartedAsset(t, r, session, started.ID)
	saved := blockNamed(t, page.Blocks, "runs_best_with")
	if saved.IsEmpty {
		t.Errorf("a block holding a link reads as empty")
	}
	if !strings.Contains(string(saved.Elements[0].Content), "The winter lorebook") {
		t.Errorf("saved links = %s", saved.Elements[0].Content)
	}
	if saved.Elements[0].Label != "Links" {
		t.Errorf("a roleless element is labelled %q, want its type's wording", saved.Elements[0].Label)
	}
}

func TestABlockSavedWithAScriptAddressIsRefused(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	added := addedBlock(t, addBlock(t, r, session, started.ID, "runs_best_with", "link_list"))

	update := editableBlock(added)
	update.Elements[0].Content = json.RawMessage(
		`{"links":[{"label":"Tap me","url":"javascript:alert(1)"}]}`,
	)
	response := saveBlock(t, r, session, started.ID, added.ID, update)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("a script address: status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestSavingAnEmptyAddedBlockKeepsEveryDefinition(t *testing.T) {
	tests := []struct {
		definition  string
		elementType string
	}{
		{"gallery", "image_set"},
		{"usage", "prose"},
		{"changelog", "text_set"},
		{"attributes", "field_list"},
		{"author_notes", "prose"},
		{"runs_best_with", "link_list"},
		{"custom_block", "prose"},
	}
	for _, test := range tests {
		t.Run(test.definition, func(t *testing.T) {
			r, session := newVerifiedTestRouter(t)
			started := startCharacter(t, r, session)
			added := addedBlock(t, addBlock(
				t, r, session, started.ID, test.definition, test.elementType,
			))
			update := editableBlock(added)
			update.Width = "full"

			response := saveBlock(t, r, session, started.ID, added.ID, update)
			if response.Code != http.StatusOK {
				t.Fatalf("save empty block: status = %d, want 200: %s", response.Code, response.Body.String())
			}

			page := fetchStartedAsset(t, r, session, started.ID)
			saved := blockNamed(t, page.Blocks, test.definition)
			if len(page.Blocks) != 3 || saved.ID != added.ID || saved.Width != "full" || !saved.IsEmpty {
				t.Errorf("saved page = %+v, want the empty block kept at full width", page.Blocks)
			}
		})
	}
}

func TestOnlyTheOwnerIsOfferedBlocksToAdd(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	response := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/assets/"+started.ID, nil))

	if response.Code == http.StatusOK {
		var page startedAsset
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode the reader's page: %v", err)
		}
		if len(page.AddableBlocks) > 0 {
			t.Errorf("a reader is offered %d blocks to add", len(page.AddableBlocks))
		}
	}
	_ = session
}
