package http

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
)

func rasterSources(t *testing.T) map[string][]byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{R: 48, G: 76, B: 115, A: 255})

	encode := func(write func(*bytes.Buffer) error) []byte {
		var out bytes.Buffer
		if err := write(&out); err != nil {
			t.Fatalf("encode raster fixture: %v", err)
		}
		return out.Bytes()
	}
	webp, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatalf("decode WebP fixture: %v", err)
	}
	return map[string][]byte{
		"image/png":  encode(func(out *bytes.Buffer) error { return png.Encode(out, picture) }),
		"image/jpeg": encode(func(out *bytes.Buffer) error { return jpeg.Encode(out, picture, nil) }),
		"image/gif":  encode(func(out *bytes.Buffer) error { return gif.Encode(out, picture, nil) }),
		"image/webp": webp,
	}
}

func TestDownloadHandsTheCurrentSourceToNginx(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())

	// Not valid UTF-8 and not valid JSON, so any re-encoding would show up.
	original := []byte{0x00, 0xff, 0xfe, 0x10, 0x80}

	metadata := exampleMetadata("Exact")
	metadata["filename"] = "exact.lumitheme"
	rec := uploadAndFinish(t, r, session, assets, metadata, original)

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
		t.Fatalf("download status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Accel-Redirect"); got == "" {
		t.Fatal("X-Accel-Redirect is missing")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("Go returned %d bytes instead of handing the file to nginx", rec.Body.Len())
	}
}

func TestDownloadUnknownAssetIs404(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/download/11111111-1111-1111-1111-111111111111", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUnverifiedSourceTypeDownloadsAsAnOpaqueAttachment(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())

	// A file that a browser would happily render if we let it.
	payload := []byte(`<script>alert(1)</script>`)

	metadata := exampleMetadata("Evil")
	metadata["filename"] = "evil.lumitheme"
	rec := uploadAndFinish(t, r, session, assets, metadata, payload)

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

	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment" {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, download URLs must not be immutable", got)
	}
}

func TestProbeVerifiedRasterSourcesMayRenderInline(t *testing.T) {
	for wantType, source := range rasterSources(t) {
		t.Run(wantType, func(t *testing.T) {
			r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
			metadata := exampleMetadata("Raster")
			metadata["filename"] = "misleading.lumitheme"
			created := uploadAndFinish(t, r, session, assets, metadata, source)

			var operation struct {
				Asset *struct {
					ID string `json:"id"`
				} `json:"asset"`
			}
			if err := json.Unmarshal(created.Body.Bytes(), &operation); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if operation.Asset == nil {
				t.Fatal("completed ingest has no asset")
			}

			download := send(t, r, httptest.NewRequest(
				http.MethodGet, "/download/"+operation.Asset.ID, nil,
			))
			if download.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", download.Code)
			}
			if got := download.Header().Get("Content-Type"); got != wantType {
				t.Errorf("Content-Type = %q, want %q", got, wantType)
			}
			if got := download.Header().Get("Content-Disposition"); got != "inline" {
				t.Errorf("Content-Disposition = %q, want inline", got)
			}
		})
	}
}

func TestFilenameExtensionAndDeclaredTypeCannotMakeSVGInline(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	writeMetadataPart(t, form, exampleMetadata("Claimed image"))
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="claimed.png"`)
	header.Set("Content-Type", "image/jpeg")
	part, err := form.CreatePart(header)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script /></svg>`)); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/assets", body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	accepted := send(t, r, authorized(request, session))
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want 202", accepted.Code)
	}
	if processed, err := assets.ProcessNextIngest(request.Context()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}

	completed := send(t, r, authorizedJSONRequest(
		t, http.MethodPatch, accepted.Header().Get("Location"),
		`{"kind":"theme","name":"Claimed image"}`, session,
	))
	if completed.Code != http.StatusAccepted {
		t.Fatalf("complete status = %d, want 202", completed.Code)
	}
	if processed, err := assets.ProcessNextIngest(request.Context()); err != nil || !processed {
		t.Fatalf("resume ingest = %v, %v; want true, nil", processed, err)
	}
	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, accepted.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Asset *struct {
			ID string `json:"id"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if operation.Asset == nil {
		t.Fatal("completed ingest has no asset")
	}

	download := send(t, r, httptest.NewRequest(
		http.MethodGet, "/download/"+operation.Asset.ID, nil,
	))
	if got := download.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := download.Header().Get("Content-Disposition"); got != "attachment" {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}
}
