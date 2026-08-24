package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieAuthenticatedMutationsRequireTheIllarinBrowserOrigin(t *testing.T) {
	router, session := newVerifiedTestRouter(t)

	request := func(origin, marker string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/sign-out", nil)
		req.AddCookie(session)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if marker != "" {
			req.Header.Set(browserMutationHeader, marker)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := request("", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("unmarked browser mutation = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := request("https://elsewhere.example", "1"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin browser mutation = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := request(testBrowserOrigin, "1"); rec.Code != http.StatusNoContent {
		t.Fatalf("same-origin browser mutation = %d %s, want %d", rec.Code, rec.Body.String(), http.StatusNoContent)
	}
}

func TestBrowserMutationGuardLeavesPublicAccountEntryPointsAvailable(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/sign-in", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("public sign-in was rejected by browser mutation guard: %s", rec.Body.String())
	}
}
