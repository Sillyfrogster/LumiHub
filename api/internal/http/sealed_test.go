package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sealPresetBlock writes one preserved sealed block the way the migration does, which is a state no route can reach because nothing in Illarin seals anything.
func sealPresetBlock(
	t *testing.T,
	pool *pgxpool.Pool,
	assetID string,
	ownerID uuid.UUID,
	version, key, content string,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"id": uuid.NewString(), "preset_id": assetID, "version": version,
		"block_key": key, "content": content,
	})
	if err != nil {
		t.Fatalf("write a sealed block payload: %v", err)
	}
	_, err = pool.Exec(t.Context(), `
		insert into migration_preserved_records
			(id, source_table, source_id, asset_id, owner_id, payload)
		values ($1, 'preset_sealed_blocks', $2, $3, $4, $5)
	`, uuid.New(), uuid.NewString(), assetID, ownerID, payload)
	if err != nil {
		t.Fatalf("preserve a sealed block: %v", err)
	}
}

func ownerOfAsset(t *testing.T, pool *pgxpool.Pool, assetID string) uuid.UUID {
	t.Helper()
	var ownerID uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`select owner_id from assets where id = $1`, assetID).Scan(&ownerID); err != nil {
		t.Fatalf("read the asset owner: %v", err)
	}
	return ownerID
}

// sealedStack is two verified creators, one preset, and the pool the sealed rows go in through.
type sealedStack struct {
	router   *gin.Engine
	session  *http.Cookie
	stranger *http.Cookie
	assetID  string
	pool     *pgxpool.Pool
}

func newSealedStack(t *testing.T) sealedStack {
	t.Helper()
	outbox := &verificationOutbox{}
	router, pool, _ := newTestRouterWithSenderPoolAndHandlers(
		t, 1<<20, DefaultDeadlines(), outbox,
	)
	session := verifiedSignUp(t, router, outbox, "sealed@example.com", "sealed.creator")
	stranger := verifiedSignUp(t, router, outbox, "other@example.com", "other.creator")
	started := startPreset(t, router, session, "sillytavern")
	return sealedStack{
		router: router, session: session, stranger: stranger,
		assetID: started.ID, pool: pool,
	}
}

func (stack sealedStack) seal(t *testing.T, version, key, content string) {
	t.Helper()
	sealPresetBlock(
		t, stack.pool, stack.assetID,
		ownerOfAsset(t, stack.pool, stack.assetID), version, key, content,
	)
}

// The owner opens the set and every sealed block comes back, in version and key order.
func TestAnOwnerExportsEverySealedBlockTheirPresetPreserves(t *testing.T) {
	stack := newSealedStack(t)
	stack.seal(t, "1.0.0", "jailbreak", "The withheld one.")
	stack.seal(t, "1.0.0", "authors_note", "The other one.")
	stack.seal(t, "0.9.0", "jailbreak", "An older take.")

	response := send(t, stack.router, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+stack.assetID+"/sealed", nil,
	), stack.session))
	if response.Code != http.StatusOK {
		t.Fatalf("export sealed content: status = %d: %s", response.Code, response.Body.String())
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.Contains(
		disposition, "attachment",
	) || !strings.Contains(disposition, ".json") {
		t.Errorf("Content-Disposition = %q, want a named json attachment", disposition)
	}
	if cache := response.Header().Get("Cache-Control"); cache != "private, no-store" {
		t.Errorf("Cache-Control = %q, want the export uncached", cache)
	}

	var exported struct {
		AssetID string `json:"asset_id"`
		Blocks  []struct {
			Version string `json:"version"`
			Key     string `json:"block_key"`
			Content string `json:"content"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &exported); err != nil {
		t.Fatalf("decode the export: %v", err)
	}
	if exported.AssetID != stack.assetID {
		t.Errorf("the export names asset %s, want %s", exported.AssetID, stack.assetID)
	}
	if len(exported.Blocks) != 3 {
		t.Fatalf("the export holds %d blocks, want all three", len(exported.Blocks))
	}
	order := make([]string, 0, 3)
	for _, sealed := range exported.Blocks {
		order = append(order, sealed.Version+"/"+sealed.Key)
	}
	want := []string{"0.9.0/jailbreak", "1.0.0/authors_note", "1.0.0/jailbreak"}
	for index, expected := range want {
		if order[index] != expected {
			t.Fatalf("export order = %v, want %v", order, want)
		}
	}
	if exported.Blocks[0].Content != "An older take." {
		t.Errorf("a sealed block came back as %q", exported.Blocks[0].Content)
	}
}

// The content was withheld from readers in v1 and stays withheld, so nobody but the owner learns it is even there.
func TestSealedContentAnswersNobodyButItsOwner(t *testing.T) {
	stack := newSealedStack(t)
	stack.seal(t, "1.0.0", "jailbreak", "The withheld one.")

	signedOut := send(t, stack.router, httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+stack.assetID+"/sealed", nil,
	))
	if signedOut.Code != http.StatusUnauthorized {
		t.Errorf("a signed-out reader asked for sealed content and got %d", signedOut.Code)
	}

	stranger := send(t, stack.router, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+stack.assetID+"/sealed", nil,
	), stack.stranger))
	if stranger.Code != http.StatusNotFound {
		t.Errorf("another creator asked for sealed content and got %d, want 404", stranger.Code)
	}
	if strings.Contains(stranger.Body.String(), "The withheld one.") {
		t.Error("the refusal carried the sealed content")
	}
}

// An asset holding nothing sealed has no export, and says so the same way a stranger is answered.
func TestAnAssetHoldingNothingSealedHasNoExport(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startPreset(t, r, session, "sillytavern")

	response := send(t, r, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+started.ID+"/sealed", nil,
	), session))
	if response.Code != http.StatusNotFound {
		t.Errorf("an asset with nothing sealed exported %d, want 404", response.Code)
	}
}

// The count reaches the owner's own page so their menu can offer the export, and reaches nobody else.
func TestTheSealedCountStandsOnlyForTheOwner(t *testing.T) {
	stack := newSealedStack(t)
	stack.seal(t, "1.0.0", "jailbreak", "The withheld one.")

	owner := send(t, stack.router, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+stack.assetID, nil,
	), stack.session))
	var page struct {
		SealedBlocks *int `json:"sealedBlocks"`
	}
	if err := json.Unmarshal(owner.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode the owner's page: %v", err)
	}
	if page.SealedBlocks == nil || *page.SealedBlocks != 1 {
		t.Errorf("the owner's page counts %v sealed blocks, want 1", page.SealedBlocks)
	}
	if strings.Contains(owner.Body.String(), "The withheld one.") {
		t.Error("the page rendered sealed content")
	}

	stranger := send(t, stack.router, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+stack.assetID, nil,
	), stack.stranger))
	if strings.Contains(stranger.Body.String(), "sealedBlocks") {
		t.Error("a stranger's page says the asset is withholding something")
	}
}
