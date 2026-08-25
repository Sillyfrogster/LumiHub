package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadReturnsTheExactUploadedBytes(t *testing.T) {
	r := newTestRouter(t)

	// Not valid UTF-8 and not valid JSON, so any re-encoding would show up.
	original := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff, 0xfe}

	rec := send(t, r, uploadRequest(t, exampleMetadata("Exact"), original))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/assets/"+created.ID+"/original", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), original) {
		t.Fatalf("downloaded bytes differ.\n got %x\nwant %x", rec.Body.Bytes(), original)
	}
}

func TestDownloadUnknownAssetIs404(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/v1/assets/11111111-1111-1111-1111-111111111111/original", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDownloadIsAlwaysSentAsAnAttachment(t *testing.T) {
	r := newTestRouter(t)

	// A file that a browser would happily render if we let it.
	payload := []byte(`<script>alert(1)</script>`)

	rec := send(t, r, uploadRequest(t, exampleMetadata("Evil"), payload))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/assets/"+created.ID+"/original", nil))

	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, originals must never be served as a renderable type", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got == "" {
		t.Error("Content-Disposition is missing, so a browser could render the file")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload) {
		t.Error("bytes changed")
	}
}
