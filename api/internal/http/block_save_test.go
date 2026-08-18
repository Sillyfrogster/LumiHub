package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type saveBlockElement struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Role    string          `json:"role,omitempty"`
	Slot    string          `json:"slot"`
	Display string          `json:"display,omitempty"`
	Content json.RawMessage `json:"content"`
}

type saveBlockBody struct {
	Title    *string            `json:"title"`
	Elements []saveBlockElement `json:"elements"`
}

func editableBlock(block startedBlock) saveBlockBody {
	elements := make([]saveBlockElement, len(block.Elements))
	for i, element := range block.Elements {
		elements[i] = saveBlockElement{
			ID: element.ID, Type: element.Type, Role: element.Role,
			Slot: element.Slot, Display: element.Display, Content: element.Content,
		}
	}
	return saveBlockBody{Elements: elements}
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
	if string(messages.Elements[0].Content) != `{"texts":[{"text":"The west shelf moved again. Come in."}]}` {
		t.Errorf("saved greetings = %s", messages.Elements[0].Content)
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

func TestBlockSaveDoesNotAcceptLaterArrangementActions(t *testing.T) {
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
