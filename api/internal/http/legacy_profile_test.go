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

func TestAV1UserAddressResolvesThroughTheDiscordLink(t *testing.T) {
	router, pool := legacyProfileStack(t)
	linkDiscordSubject(t, pool, "verified.creator", "314159265358979323")

	response := send(t, router,
		httptest.NewRequest(http.MethodGet, "/v1/legacy-profiles/314159265358979323", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var found struct {
		Handle string `json:"handle"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &found); err != nil {
		t.Fatalf("decode the resolved profile: %v", err)
	}
	if found.Handle != "verified.creator" {
		t.Errorf("the address resolved to %q, want verified.creator", found.Handle)
	}
}

func TestAnUnknownV1UserAddressIsAPlainMiss(t *testing.T) {
	router, _ := legacyProfileStack(t)

	response := send(t, router,
		httptest.NewRequest(http.MethodGet, "/v1/legacy-profiles/271828182845904523", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if response.Header().Get("Location") != "" {
		t.Error("the refusal carries a location header")
	}
}

func legacyProfileStack(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	_, router, _, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	return router, pool
}

func linkDiscordSubject(t *testing.T, pool *pgxpool.Pool, handle, subject string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		insert into oauth_identities (user_id, provider, subject)
		select id, 'discord', $2 from users where username = $1`, handle, subject); err != nil {
		t.Fatalf("link the Discord identity: %v", err)
	}
}
