package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAV1AddressResolvesToTheAssetThatHeldIt(t *testing.T) {
	router, pool, session := legacyAddressStack(t)
	assetID := publishedCharacter(t, router, session)
	storeLegacyAddress(t, pool, "old-author/old-name", assetID)

	response := send(t, router,
		httptest.NewRequest(http.MethodGet, "/v1/legacy-assets/old-author/old-name", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var found struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &found); err != nil {
		t.Fatalf("decode the resolved asset: %v", err)
	}
	if found.ID != assetID {
		t.Errorf("the address resolved to %s, want %s", found.ID, assetID)
	}
	if found.Name == "" {
		t.Error("the answer carries no name to build the permalink from")
	}
}

func TestAnUnknownV1AddressIsAPlainMiss(t *testing.T) {
	router, _, _ := legacyAddressStack(t)
	response := send(t, router,
		httptest.NewRequest(http.MethodGet, "/v1/legacy-assets/nobody/nothing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestAV1AddressForAWithheldAssetSaysNothing(t *testing.T) {
	router, pool, session := legacyAddressStack(t)
	assetID := publishedCharacter(t, router, session)
	storeLegacyAddress(t, pool, "old-author/old-name", assetID)
	if _, err := pool.Exec(context.Background(), `
		update assets
		   set withheld_at = now(), withheld_by = owner_id, withheld_reason = 'checking'
		 where id = $1`, assetID); err != nil {
		t.Fatalf("withhold the asset: %v", err)
	}

	response := send(t, router,
		httptest.NewRequest(http.MethodGet, "/v1/legacy-assets/old-author/old-name", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if response.Header().Get("Location") != "" {
		t.Error("the refusal carries a location header")
	}
}

func TestAV1AddressForADraftIsAPlainMiss(t *testing.T) {
	router, pool, session := legacyAddressStack(t)
	started := startCharacter(t, router, session)
	storeLegacyAddress(t, pool, "old-author/old-name", started.ID)

	response := send(t, router,
		httptest.NewRequest(http.MethodGet, "/v1/legacy-assets/old-author/old-name", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func legacyAddressStack(t *testing.T) (*gin.Engine, *pgxpool.Pool, *http.Cookie) {
	t.Helper()
	_, router, session, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	return router, pool, session
}

func publishedCharacter(t *testing.T, router *gin.Engine, session *http.Cookie) string {
	t.Helper()
	started := startCharacter(t, router, session)
	writeCharacterFloor(t, router, session, started)
	if got := publishAsset(t, router, session, started.ID); got.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", got.Code, got.Body.String())
	}
	return started.ID
}

func storeLegacyAddress(t *testing.T, pool *pgxpool.Pool, address, assetID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`insert into asset_legacy_paths (path, asset_id) values ($1, $2)`,
		address, assetID); err != nil {
		t.Fatalf("store the v1 address: %v", err)
	}
}
