package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLivenessDoesNotDependOnReadiness(t *testing.T) {
	r := gin.New()
	if err := Register(
		r,
		NewHandlers(nil, nil, nil, 1<<20),
		DefaultDeadlines(),
		func(context.Context) error { return errors.New("database password leaked here") },
	); err != nil {
		t.Fatalf("register: %v", err)
	}

	live := httptest.NewRecorder()
	r.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if live.Code != http.StatusOK {
		t.Fatalf("liveness status = %d", live.Code)
	}

	ready := httptest.NewRecorder()
	r.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d", ready.Code)
	}
	if body := ready.Body.String(); body != "{\"status\":\"unavailable\"}" {
		t.Fatalf("readiness exposed its dependency error: %s", body)
	}
}
