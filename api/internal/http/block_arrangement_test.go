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

type arrangedBlock struct {
	ID     string `json:"id"`
	Hidden bool   `json:"hidden"`
	Width  string `json:"width"`
}

func arrangeBlocks(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID string,
	blocks []arrangedBlock,
) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{"blocks": blocks})
	if err != nil {
		t.Fatalf("encode arrangement: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPut, "/v1/assets/"+assetID+"/blocks", strings.NewReader(string(body)),
	)
	request.Header.Set("Content-Type", "application/json")
	return send(t, r, authorized(request, session))
}

func insertEmptyGallery(t *testing.T, pool *pgxpool.Pool, assetID string) string {
	t.Helper()
	id := uuid.New()
	elementID := uuid.New()
	elements := `[{"id":"` + elementID.String() + `","type":"image_set","role":"gallery","slot":"main","version":1,"options":{},"content":{"images":[]}}]`
	if _, err := pool.Exec(t.Context(), `
		insert into asset_blocks
		  (id, asset_id, definition, position, hidden, layout, width, elements)
		values ($1, $2, 'gallery', 2, false, 'single', 'half', $3)
	`, id, assetID, elements); err != nil {
		t.Fatalf("insert gallery: %v", err)
	}
	return id.String()
}

func TestCreatorReordersAndHidesBlocksAsOneArrangement(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	core := blockNamed(t, started.Blocks, "character_core")
	messages := blockNamed(t, started.Blocks, "messages")

	response := arrangeBlocks(t, r, session, started.ID, []arrangedBlock{
		{ID: messages.ID, Width: "half"},
		{ID: core.ID, Hidden: true, Width: "half"},
	})

	if response.Code != http.StatusOK {
		t.Fatalf("arrange status = %d, want 200: %s", response.Code, response.Body.String())
	}
	saved := fetchStartedAsset(t, r, session, started.ID)
	if saved.Blocks[0].ID != messages.ID || saved.Blocks[0].Position != 0 || saved.Blocks[0].Width != "half" {
		t.Errorf("first arranged block = %+v, want Messages at half width", saved.Blocks[0])
	}
	if saved.Blocks[1].ID != core.ID || saved.Blocks[1].Position != 1 || !saved.Blocks[1].Hidden {
		t.Errorf("second arranged block = %+v, want hidden character core", saved.Blocks[1])
	}
}

func TestArrangementRefusesToHideTheAlwaysShownBlock(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	core := blockNamed(t, started.Blocks, "character_core")
	messages := blockNamed(t, started.Blocks, "messages")

	response := arrangeBlocks(t, r, session, started.ID, []arrangedBlock{
		{ID: core.ID, Width: core.Width},
		{ID: messages.ID, Hidden: true, Width: messages.Width},
	})

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "always shown") {
		t.Fatalf("hide Messages = %d, want an always-shown refusal: %s", response.Code, response.Body.String())
	}
}

func TestArrangementRequiresEveryCurrentBlockExactlyOnce(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startCharacter(t, r, session)
	core := blockNamed(t, started.Blocks, "character_core")

	response := arrangeBlocks(t, r, session, started.ID, []arrangedBlock{
		{ID: core.ID, Width: core.Width},
		{ID: core.ID, Width: core.Width},
	})

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "each section once") {
		t.Fatalf("duplicate arrangement = %d, want exact-membership refusal: %s", response.Code, response.Body.String())
	}
}

func TestCreatorRemovesAnOptionalBlockAndRequiredBlocksStay(t *testing.T) {
	_, r, session, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	started := startCharacter(t, r, session)
	galleryID := insertEmptyGallery(t, pool, started.ID)

	request := httptest.NewRequest(
		http.MethodDelete, "/v1/assets/"+started.ID+"/blocks/"+galleryID, nil,
	)
	response := send(t, r, authorized(request, session))
	if response.Code != http.StatusNoContent {
		t.Fatalf("remove Gallery status = %d, want 204: %s", response.Code, response.Body.String())
	}
	saved := fetchStartedAsset(t, r, session, started.ID)
	if len(saved.Blocks) != 2 || saved.Blocks[0].Position != 0 || saved.Blocks[1].Position != 1 {
		t.Errorf("blocks after remove = %+v, want two required blocks in gapless order", saved.Blocks)
	}

	core := blockNamed(t, saved.Blocks, "character_core")
	request = httptest.NewRequest(
		http.MethodDelete, "/v1/assets/"+started.ID+"/blocks/"+core.ID, nil,
	)
	response = send(t, r, authorized(request, session))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "required") {
		t.Fatalf("remove required block = %d, want required refusal: %s", response.Code, response.Body.String())
	}
}

func TestSavingAnOptionalBlockEmptyReturnsItToAbsent(t *testing.T) {
	_, r, session, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	started := startCharacter(t, r, session)
	insertEmptyGallery(t, pool, started.ID)
	gallery := blockNamed(t, fetchStartedAsset(t, r, session, started.ID).Blocks, "gallery")

	response := saveBlock(t, r, session, started.ID, gallery.ID, editableBlock(gallery))

	if response.Code != http.StatusNoContent {
		t.Fatalf("save empty Gallery status = %d, want 204: %s", response.Code, response.Body.String())
	}
	saved := fetchStartedAsset(t, r, session, started.ID)
	if len(saved.Blocks) != 2 {
		t.Errorf("blocks after empty save = %d, want the two required blocks", len(saved.Blocks))
	}
}
