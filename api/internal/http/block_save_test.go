package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/google/uuid"
)

type saveBlockElement struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Role     string          `json:"role,omitempty"`
	Slot     string          `json:"slot"`
	Display  string          `json:"display,omitempty"`
	ItemSize string          `json:"itemSize,omitempty"`
	Content  json.RawMessage `json:"content"`
}

type saveBlockBody struct {
	Title    *string            `json:"title"`
	Layout   string             `json:"layout"`
	Width    string             `json:"width"`
	Elements []saveBlockElement `json:"elements"`
}

func editableBlock(block startedBlock) saveBlockBody {
	elements := make([]saveBlockElement, len(block.Elements))
	for i, element := range block.Elements {
		elements[i] = saveBlockElement{
			ID: element.ID, Type: element.Type, Role: element.Role,
			Slot: element.Slot, Display: element.Display, ItemSize: element.ItemSize,
			Content: element.Content,
		}
	}
	return saveBlockBody{
		Layout:   block.Layout,
		Width:    block.Width,
		Elements: elements,
	}
}

func saveBlock(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID string,
	blockID string,
	body saveBlockBody,
) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode block save: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/assets/"+assetID+"/blocks/"+blockID,
		strings.NewReader(string(encoded)),
	)
	request.Header.Set("Content-Type", "application/json")
	return send(t, r, authorized(request, session))
}

func fetchStartedAsset(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID string,
) startedAsset {
	t.Helper()
	response := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID, nil), session,
	))
	if response.Code != http.StatusOK {
		t.Fatalf("read saved asset status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var saved startedAsset
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved asset: %v", err)
	}
	return saved
}

func TestACreatorSavesDescriptionAndGreetingContent(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	core := editableBlock(blockNamed(t, started.Blocks, "character_core"))
	core.Elements[0].Content = json.RawMessage(`{"text":"She keeps the memories that books forget."}`)
	response := saveBlock(t, r, session, started.ID, started.Blocks[0].ID, core)
	if response.Code != http.StatusOK {
		t.Fatalf("save description status = %d, want 200: %s", response.Code, response.Body.String())
	}

	messagesBlock := blockNamed(t, started.Blocks, "messages")
	messages := editableBlock(messagesBlock)
	messages.Elements[0].Content = json.RawMessage(`{"texts":[{"text":"The west shelf moved again. Come in."}]}`)
	response = saveBlock(t, r, session, started.ID, messagesBlock.ID, messages)
	if response.Code != http.StatusOK {
		t.Fatalf("save greeting status = %d, want 200: %s", response.Code, response.Body.String())
	}

	saved := fetchStartedAsset(t, r, session, started.ID)
	core = editableBlock(blockNamed(t, saved.Blocks, "character_core"))
	messages = editableBlock(blockNamed(t, saved.Blocks, "messages"))
	if string(core.Elements[0].Content) != `{"text":"She keeps the memories that books forget."}` {
		t.Errorf("saved description = %s", core.Elements[0].Content)
	}
	var greetings struct {
		Texts []struct {
			ID   uuid.UUID `json:"id"`
			Text string    `json:"text"`
		} `json:"texts"`
	}
	if err := json.Unmarshal(messages.Elements[0].Content, &greetings); err != nil {
		t.Fatalf("read the saved greetings: %v", err)
	}
	if len(greetings.Texts) != 1 || greetings.Texts[0].Text != "The west shelf moved again. Come in." {
		t.Errorf("saved greetings = %s", messages.Elements[0].Content)
	}
	// The greeting is an item, so it left the save with an id of its own.
	if greetings.Texts[0].ID == uuid.Nil {
		t.Error("the saved greeting carries no id")
	}
}

func TestACreatorCanChooseAndReleaseABlockTitle(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)

	chosen := "Who she is"
	core.Title = &chosen
	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)
	if response.Code != http.StatusOK {
		t.Fatalf("save chosen title status = %d, want 200: %s", response.Code, response.Body.String())
	}
	saved := blockNamed(t, fetchStartedAsset(t, r, session, started.ID).Blocks, "character_core")
	if saved.Title != chosen || saved.TitleIsDefault {
		t.Fatalf("chosen title = %q, default = %t", saved.Title, saved.TitleIsDefault)
	}

	core.Title = nil
	response = saveBlock(t, r, session, started.ID, coreBlock.ID, core)
	if response.Code != http.StatusOK {
		t.Fatalf("release title status = %d, want 200: %s", response.Code, response.Body.String())
	}
	saved = blockNamed(t, fetchStartedAsset(t, r, session, started.ID).Blocks, "character_core")
	if saved.Title != "The character" || !saved.TitleIsDefault {
		t.Fatalf("released title = %q, default = %t", saved.Title, saved.TitleIsDefault)
	}
}

func TestSavingMalformedElementContentNamesWhatMustChange(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Content = json.RawMessage(`{}`)

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed content status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "text"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSavingAnElementOutsideTheChosenLayoutNamesTheAvailableSlots(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Slot = "aside"

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid slot status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "aside", "top", "middle", "bottom"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSavingARoleOnTheWrongElementTypeNamesTheRequiredType(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Type = "text_set"
	core.Elements[0].Content = json.RawMessage(`{"texts":[]}`)

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong type status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "prose"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestASecondElementForASingularRoleIsRefusedWhereItIsCreated(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[2].Role = "description"

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("second description status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "once", "extra"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestAPinnedElementCannotBeRemovedFromItsBlock(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements = core.Elements[1:]

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("removed pinned element status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "The character", "Restore"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSavingAnElementWithAnUnknownDisplayNamesTheClosedChoices(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Display = "glowing"

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown display status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "display", "rich", "verbatim"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSavingTextWithoutDisplayNamesTheClosedChoices(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Display = ""

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing display status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "display", "rich", "verbatim"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSavingDuplicateElementIdentityNamesWhatMustChange(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[1].ID = core.Elements[0].ID

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate element id status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "Personality", "same id"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSavingAReplacementElementIdentityIsRefused(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].ID = "00000000-0000-4000-8000-000000000001"

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("replacement element id status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Description", "existing id"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestMalformedElementIdentityNamesTheRequiredShape(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].ID = "not-a-uuid"

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed element id status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"id", "UUID"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestBlockSaveDoesNotAcceptUnrelatedArrangementActions(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	messagesBlock := blockNamed(t, started.Blocks, "messages")
	body := struct {
		saveBlockBody
		Hidden bool `json:"hidden"`
	}{saveBlockBody: editableBlock(messagesBlock), Hidden: true}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode later action: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/assets/"+started.ID+"/blocks/"+messagesBlock.ID,
		strings.NewReader(string(encoded)),
	)
	request.Header.Set("Content-Type", "application/json")
	response := send(t, r, authorized(request, session))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("later arrangement fields status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"title", "elements"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestACreatorCanNarrowARequiredBlock(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Width = "half"

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusOK {
		t.Fatalf("narrow required block status = %d, want 200: %s", response.Code, response.Body.String())
	}
	saved := blockNamed(t, fetchStartedAsset(t, r, session, started.ID).Blocks, "character_core")
	if saved.Width != "half" || saved.Layout != "stack-3" {
		t.Errorf("saved arrangement = %s at %s, want stack-3 at half", saved.Layout, saved.Width)
	}
}

func TestChoosingALayoutThatNeedsMoreWidthNamesTheFirstFix(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Layout = "trio"
	for i, slot := range []string{"left", "middle", "right"} {
		core.Elements[i].Slot = slot
	}

	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("trio at two thirds status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"trio", "full width", "Widen it first"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestNarrowingBelowTheCurrentLayoutNamesTheFirstFix(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Layout = "trio"
	core.Width = "full"
	for i, slot := range []string{"left", "middle", "right"} {
		core.Elements[i].Slot = slot
	}
	response := saveBlock(t, r, session, started.ID, coreBlock.ID, core)
	if response.Code != http.StatusOK {
		t.Fatalf("prepare trio status = %d, want 200: %s", response.Code, response.Body.String())
	}

	core.Width = "half"
	response = saveBlock(t, r, session, started.ID, coreBlock.ID, core)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("narrow trio status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"trio", "full width", "Choose another layout before narrowing it"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestALayoutTheDefinitionDoesNotOfferNamesTheAvailableChoices(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	messagesBlock := blockNamed(t, started.Blocks, "messages")
	messages := editableBlock(messagesBlock)
	messages.Layout = "duo"
	for i, slot := range []string{"left", "right"} {
		messages.Elements[i].Slot = slot
	}

	response := saveBlock(t, r, session, started.ID, messagesBlock.ID, messages)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unoffered layout status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Messages", "stack-2", "stack-3"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}

func TestSwitchingThreeMessagesBackToStackTwoNamesTheStrandedElement(t *testing.T) {
	_, r, session, _, pool := newVerifiedTestRoutersWithPool(
		t, 1<<20, DefaultDeadlines(),
	)
	started := startCharacter(t, r, session)
	messagesBlock := blockNamed(t, started.Blocks, "messages")

	var stored []block.Element
	var encoded []byte
	if err := pool.QueryRow(t.Context(), `
		select elements from asset_blocks where id = $1
	`, messagesBlock.ID).Scan(&encoded); err != nil {
		t.Fatalf("read messages elements: %v", err)
	}
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatalf("decode messages elements: %v", err)
	}
	stored = append(stored, block.Element{
		ID: uuid.New(), Type: block.TypeTextSet, Role: block.RoleGroupGreetings,
		Slot: "bottom", Options: block.Options{Display: block.DisplayRich},
		Content: block.TextSet{Texts: []block.TextItem{{ID: block.NewItemID(), Text: "Only for the whole party."}}},
	})
	stored[0].Slot = "top"
	stored[1].Slot = "middle"
	encoded, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("encode messages elements: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `
		update asset_blocks set layout = 'stack-3', elements = $2 where id = $1
	`, messagesBlock.ID, encoded); err != nil {
		t.Fatalf("prepare three messages: %v", err)
	}

	savedAsset := fetchStartedAsset(t, r, session, started.ID)
	messagesBlock = blockNamed(t, savedAsset.Blocks, "messages")
	messages := editableBlock(messagesBlock)
	messages.Layout = "stack-2"

	response := saveBlock(t, r, session, started.ID, messagesBlock.ID, messages)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("three messages in stack-2 status = %d, want 400: %s", response.Code, response.Body.String())
	}
	for _, want := range []string{"Group-only greetings", "stack-2", "Move or remove"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("refusal %q does not name %q", response.Body.String(), want)
		}
	}
}
