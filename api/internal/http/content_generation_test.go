package http

import (
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

// The counter a linked instance compares moves for the content of a download
// and stays still for the arrangement of a page (ADR-0023).
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
