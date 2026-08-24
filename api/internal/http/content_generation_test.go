package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func contentGeneration(t *testing.T, pool *pgxpool.Pool, assetID string) int {
	t.Helper()
	id, err := uuid.Parse(assetID)
	if err != nil {
		t.Fatalf("parse the asset id: %v", err)
	}
	var generation int
	if err := pool.QueryRow(t.Context(),
		`select content_generation from assets where id = $1`, id,
	).Scan(&generation); err != nil {
		t.Fatalf("read the content generation: %v", err)
	}
	return generation
}

func TestEditingAnElementMovesTheCounterAndRearrangingThePageDoesNot(t *testing.T) {
	_, r, session, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	started := startCharacter(t, r, session)
	if got := contentGeneration(t, pool, started.ID); got != 1 {
		t.Fatalf("a new draft is at content generation %d, want 1", got)
	}

	coreBlock := blockNamed(t, started.Blocks, "character_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Content = []byte(`{"text":"She keeps the memories that books forget."}`)
	if response := saveBlock(t, r, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("save the description: %d %s", response.Code, response.Body.String())
	}
	edited := contentGeneration(t, pool, started.ID)
	if edited != 2 {
		t.Fatalf("content generation = %d, want 2 after an edit", edited)
	}

	messages := blockNamed(t, started.Blocks, "messages")
	response := arrangeBlocks(t, r, session, started.ID, []arrangedBlock{
		{ID: messages.ID, Width: "full"},
		{ID: coreBlock.ID, Width: "half"},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("rearrange the page: %d %s", response.Code, response.Body.String())
	}
	if got := contentGeneration(t, pool, started.ID); got != edited {
		t.Fatalf("content generation = %d, want %d after a reorder and a width change", got, edited)
	}

	// The adult content answer is what a page tells a reader, not what a file
	// carries, so it moves nothing either.
	request := httptest.NewRequest(http.MethodPut, "/v1/assets/"+started.ID+"/identity",
		strings.NewReader(`{"name":"","isNsfw":true}`))
	request.Header.Set("Content-Type", "application/json")
	if answered := send(t, r, authorized(request, session)); answered.Code != http.StatusNoContent {
		t.Fatalf("answer the adult content question: %d %s", answered.Code, answered.Body.String())
	}
	if got := contentGeneration(t, pool, started.ID); got != edited {
		t.Fatalf("content generation = %d, want %d after the adult content answer", got, edited)
	}
}

func TestProtectedPromptGenerationFollowsCompleteArtifactBytes(t *testing.T) {
	_, router, session, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	started := startPreset(t, router, session, "lumiverse")
	coreBlock := blockNamed(t, started.Blocks, "preset_core")
	core := editableBlock(coreBlock)
	const original = "Keep every room quiet."
	core.Elements[0].Content = json.RawMessage(`{"groups":[],"fragments":[
		{"name":"House rule","role":"system","text":"` + original + `","enabled":true}
	]}`)
	if response := saveBlock(t, router, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("save public prompt: %d %s", response.Code, response.Body.String())
	}
	publicGeneration := contentGeneration(t, pool, started.ID)

	owner := fetchStartedAsset(t, router, session, started.ID)
	core = editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	core.Elements[0].Content = json.RawMessage(strings.Replace(
		string(core.Elements[0].Content), `"enabled":true`, `"protected":true,"enabled":true`, 1,
	))
	apps := []string{"lumiverse"}
	core.AllowedApps = &apps
	if response := saveBlock(t, router, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("seal unchanged prompt: %d %s", response.Code, response.Body.String())
	}
	if got := contentGeneration(t, pool, started.ID); got != publicGeneration {
		t.Fatalf("content generation after sealing unchanged text = %d, want %d", got, publicGeneration)
	}
	owner = fetchStartedAsset(t, router, session, started.ID)
	if !owner.LinkedInstallOnly || len(owner.AllowedApps) != 1 || owner.AllowedApps[0] != "lumiverse" {
		t.Fatalf("sealed prompt availability = linked install only %t, apps %v",
			owner.LinkedInstallOnly, owner.AllowedApps)
	}

	core = editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	core.Elements[0].Content = json.RawMessage(strings.Replace(
		string(core.Elements[0].Content), original, "Keep every room completely quiet.", 1,
	))
	core.AllowedApps = &apps
	if response := saveBlock(t, router, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("edit sealed prompt: %d %s", response.Code, response.Body.String())
	}
	editedGeneration := contentGeneration(t, pool, started.ID)
	if editedGeneration != publicGeneration+1 {
		t.Fatalf("content generation after editing sealed text = %d, want %d", editedGeneration, publicGeneration+1)
	}

	owner = fetchStartedAsset(t, router, session, started.ID)
	core = editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	core.Elements[0].Content = json.RawMessage(strings.Replace(
		string(core.Elements[0].Content), `,"protected":true`, "", 1,
	))
	core.AllowedApps = &[]string{}
	if response := saveBlock(t, router, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("unseal unchanged prompt: %d %s", response.Code, response.Body.String())
	}
	if got := contentGeneration(t, pool, started.ID); got != editedGeneration {
		t.Fatalf("content generation after unsealing unchanged text = %d, want %d", got, editedGeneration)
	}
	owner = fetchStartedAsset(t, router, session, started.ID)
	if owner.LinkedInstallOnly || len(owner.AllowedApps) != 0 {
		t.Fatalf("unsealed prompt availability = linked install only %t, apps %v",
			owner.LinkedInstallOnly, owner.AllowedApps)
	}
}
