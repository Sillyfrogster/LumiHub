package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type startedBlock struct {
	ID             string `json:"id"`
	Definition     string `json:"definition"`
	Title          string `json:"title"`
	TitleIsDefault bool   `json:"titleIsDefault"`
	Position       int    `json:"position"`
	Hidden         bool   `json:"hidden"`
	Layout         string `json:"layout"`
	Width          string `json:"width"`
	Required       bool   `json:"required"`
	Hideable       bool   `json:"hideable"`
	IsEmpty        bool   `json:"isEmpty"`
	Elements       []struct {
		ID      string          `json:"id"`
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Slot    string          `json:"slot"`
		Label   string          `json:"label"`
		Pinned  bool            `json:"pinned"`
		Display string          `json:"display"`
		IsEmpty bool            `json:"isEmpty"`
		Content json.RawMessage `json:"content"`
	} `json:"elements"`
}

type startedAsset struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Lifecycle string         `json:"lifecycle"`
	IsOwner   bool           `json:"isOwner"`
	IsNSFW    *bool          `json:"isNsfw"`
	Blocks    []startedBlock `json:"blocks"`
}

func startCharacter(t *testing.T, r http.Handler, session *http.Cookie) startedAsset {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/assets",
		strings.NewReader(`{"kind":"character"}`))
	request.Header.Set("Content-Type", "application/json")
	response := send(t, r, authorized(request, session))
	if response.Code != http.StatusCreated {
		t.Fatalf("start a character: status = %d, want 201: %s",
			response.Code, response.Body.String())
	}
	var started startedAsset
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode the started asset: %v", err)
	}
	return started
}

func blockNamed(t *testing.T, blocks []startedBlock, definition string) startedBlock {
	t.Helper()
	for _, b := range blocks {
		if b.Definition == definition {
			return b
		}
	}
	t.Fatalf("no %s block on the page", definition)
	return startedBlock{}
}

func TestACharacterBuiltFromNothingLandsOnItsTwoRequiredSections(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	started := startCharacter(t, r, session)

	if started.Kind != "character" || started.Name != "" {
		t.Fatalf("started asset = %+v, want an unnamed character", started)
	}
	if len(started.Blocks) != 2 {
		t.Fatalf("blocks = %d, want the two the kind requires", len(started.Blocks))
	}
	if started.Blocks[0].Definition != "character_core" || started.Blocks[0].Position != 0 {
		t.Errorf("first block = %+v, want the character core", started.Blocks[0])
	}
	if started.Blocks[1].Definition != "messages" || started.Blocks[1].Position != 1 {
		t.Errorf("second block = %+v, want messages", started.Blocks[1])
	}

	core := blockNamed(t, started.Blocks, "character_core")
	if !core.Required || !core.Hideable || !core.IsEmpty {
		t.Errorf("character core = %+v, want required, hideable and empty", core)
	}
	if core.Title != "The character" || !core.TitleIsDefault {
		t.Errorf("character core title = %q, want the definition's default", core.Title)
	}
	if core.Layout != "stack-3" || core.Width != "two_thirds" {
		t.Errorf("character core = %q at %q, want stack-3 at two thirds", core.Layout, core.Width)
	}
	wantRoles := []string{"description", "personality", "scenario"}
	if len(core.Elements) != 3 {
		t.Fatalf("character core elements = %d, want three", len(core.Elements))
	}
	for i, element := range core.Elements {
		if element.Role != wantRoles[i] {
			t.Errorf("element %d role = %q, want %q", i, element.Role, wantRoles[i])
		}
		if element.Type != "prose" || element.Display != "rich" {
			t.Errorf("%s is a %q element displayed %q, want rich prose",
				element.Role, element.Type, element.Display)
		}
		if !element.Pinned || !element.IsEmpty || element.Label == "" {
			t.Errorf("%s = %+v, want a pinned, empty, labelled element", element.Role, element)
		}
	}

	messages := blockNamed(t, started.Blocks, "messages")
	if !messages.Required || messages.Hideable {
		t.Errorf("messages = %+v, want required and never hidden", messages)
	}
	if messages.Layout != "stack-2" {
		t.Errorf("messages layout = %q, want stack-2 with no group-only greetings",
			messages.Layout)
	}
	if len(messages.Elements) != 2 {
		t.Fatalf("messages elements = %d, want greetings and example dialogue",
			len(messages.Elements))
	}
}

func TestAnAssetBuiltFromNothingStartsAsAnUnansweredDraft(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	started := startCharacter(t, r, session)

	if started.Lifecycle != "draft" {
		t.Errorf("lifecycle = %q, want draft", started.Lifecycle)
	}
	if started.IsNSFW != nil {
		t.Errorf("the adult content question was answered for the creator: %v", *started.IsNSFW)
	}
	if !started.IsOwner {
		t.Errorf("the creator is not read as the owner of the asset they just made")
	}
}

func TestADraftResolvesForItsOwnerAndReturnsTheUniform404ForEveryoneElse(t *testing.T) {
	setup, r, session, _ := newVerifiedTestRoutersWithService(t, 1<<20, DefaultDeadlines())
	started := startCharacter(t, r, session)

	owner := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, "/v1/assets/"+started.ID, nil), session))
	if owner.Code != http.StatusOK {
		t.Fatalf("the owner got %d for their own draft", owner.Code)
	}

	stranger := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/assets/"+started.ID, nil))
	if stranger.Code != http.StatusNotFound {
		t.Errorf("a signed-out reader got %d for a draft, want 404", stranger.Code)
	}

	other := signUp(t, setup, "other@example.com", "other.creator")
	signedIn := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, "/v1/assets/"+started.ID, nil), other))
	if signedIn.Code != http.StatusNotFound {
		t.Errorf("another account got %d for someone else's draft, want 404", signedIn.Code)
	}
}

func TestADraftIsInNoBrowseOrSearchResult(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)

	for _, path := range []string{
		"/v1/assets",
		"/v1/assets?kind=character",
		"/v1/assets?creator=verified.creator",
		"/v1/assets?q=character",
	} {
		listing := send(t, r, authorized(httptest.NewRequest(http.MethodGet, path, nil), session))
		if listing.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200: %s", path, listing.Code, listing.Body.String())
		}
		if strings.Contains(listing.Body.String(), started.ID) {
			t.Errorf("a draft appeared in %s", path)
		}
	}
}

func TestAKindIllarinCannotBuildIsRefusedRatherThanStarted(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	for _, body := range []string{`{"kind":"preset"}`, `{"kind":"nonsense"}`, `{"kind":""}`, `{}`} {
		request := httptest.NewRequest(http.MethodPost, "/v1/assets", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := send(t, r, authorized(request, session))
		if response.Code != http.StatusBadRequest {
			t.Errorf("POST %s status = %d, want 400: %s", body, response.Code, response.Body.String())
		}
	}
}

func TestStartingAnAssetNeedsAVerifiedAccount(t *testing.T) {
	r := newTestRouter(t)

	request := httptest.NewRequest(http.MethodPost, "/v1/assets",
		strings.NewReader(`{"kind":"character"}`))
	request.Header.Set("Content-Type", "application/json")
	response := send(t, r, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 with no account signed in", response.Code)
	}
}
