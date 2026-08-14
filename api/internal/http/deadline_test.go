package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// alreadyPast is a deadline that has gone by before the handler starts, so a
// test about a limit does not depend on how fast the database answers.
const alreadyPast = time.Nanosecond

func deadlines(json time.Duration) Deadlines {
	return Deadlines{JSON: json, Upload: time.Minute, Download: time.Minute}
}

func TestAListingPastItsDeadlineFailsRatherThanAnswers(t *testing.T) {
	answered := list(t, newTestRouterWith(t, 1<<20, deadlines(5*time.Second)))
	if answered.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with time to spare. body: %s",
			answered.Code, answered.Body.String())
	}

	gaveUp := list(t, newTestRouterWith(t, 1<<20, deadlines(alreadyPast)))
	if gaveUp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 once the deadline has gone. body: %s",
			gaveUp.Code, gaveUp.Body.String())
	}
}

func list(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/assets", nil))
	return rec
}

func TestARouteWithNoDeadlineIsRefused(t *testing.T) {
	err := Register(gin.New(), NewHandlers(nil, nil, nil, 1<<20), Deadlines{Upload: time.Minute, Download: time.Minute})

	if err == nil {
		t.Fatal("registered a listing route that may run for as long as it likes")
	}
}

func TestAnUploadIsNotHeldToTheListingDeadline(t *testing.T) {
	r, session := newVerifiedTestRouterWith(t, 1<<20, deadlines(alreadyPast))

	rec := send(t, r, authorized(
		uploadRequest(t, exampleMetadata("Patient"), []byte("bytes")), session,
	))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202. body: %s", rec.Code, rec.Body.String())
	}
}

func TestADownloadIsNotHeldToTheListingDeadline(t *testing.T) {
	setup, r, session, assets := newVerifiedTestRoutersWithService(t, 1<<20, deadlines(alreadyPast))

	file := []byte("bytes worth waiting for")
	metadata := exampleMetadata("Roomy")
	metadata["filename"] = "roomy.lumitheme"
	rec := uploadAndFinish(t, setup, session, assets, metadata, file)

	var created struct {
		Asset *struct {
			ID string `json:"id"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if created.Asset == nil {
		t.Fatal("completed ingest has no asset")
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download/"+created.Asset.ID, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Accel-Redirect") == "" {
		t.Fatal("download was not handed to nginx")
	}
}
