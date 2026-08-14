package http

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/linking"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type parseFailureModule struct{}

func (parseFailureModule) ID() string { return "claimed" }
func (parseFailureModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}

type typedParseFailureModule struct {
	err error
}

func (typedParseFailureModule) ID() string { return "typed_failure" }
func (typedParseFailureModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (m typedParseFailureModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{}, m.err
}
func (parseFailureModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{}, errors.New("the claimed payload is malformed")
}

func TestUploadReturnsAPendingOperation(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	rec := send(t, r, authorized(
		uploadRequest(t, exampleMetadata("Evening Theme"), []byte("theme bytes")),
		session,
	))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202. body: %s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/v1/ingests/") {
		t.Fatalf("Location = %q, want an ingest operation URL", location)
	}

	var operation struct {
		Status string `json:"status"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Status != "pending" {
		t.Errorf("status = %q, want pending", operation.Status)
	}
	if operation.URL != location {
		t.Errorf("url = %q, want Location %q", operation.URL, location)
	}
}

func authorizedJSONRequest(
	t *testing.T,
	method string,
	path string,
	body string,
	session *http.Cookie,
) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	return req
}

func uploadAndFinish(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assets *asset.Service,
	metadata map[string]any,
	file []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	accepted := send(t, r, authorized(uploadRequest(t, metadata, file), session))
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want 202. body: %s", accepted.Code, accepted.Body.String())
	}
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}
	return send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, accepted.Header().Get("Location"), nil), session,
	))
}

func newVerifiedIngestRouter(
	t *testing.T,
	registry *format.Registry,
) (*gin.Engine, *http.Cookie, *asset.Service) {
	t.Helper()
	router, session, assets, _ := newVerifiedIngestRouterWithSettings(
		t, registry, asset.DefaultIngestSettings(),
	)
	return router, session, assets
}

// newVerifiedIngestRouterWithPool also returns the pool, for a test that needs
// a row in a state no route can reach.
func newVerifiedIngestRouterWithPool(
	t *testing.T,
	registry *format.Registry,
) (*gin.Engine, *http.Cookie, *asset.Service, *pgxpool.Pool) {
	t.Helper()
	return newVerifiedIngestRouterWithSettings(t, registry, asset.DefaultIngestSettings())
}

func newVerifiedIngestRouterWithSettings(
	t *testing.T,
	registry *format.Registry,
	settings asset.IngestSettings,
) (*gin.Engine, *http.Cookie, *asset.Service, *pgxpool.Pool) {
	t.Helper()
	return newVerifiedIngestRouterWithStore(t, registry, settings, nil)
}

func newVerifiedIngestRouterWithStore(
	t *testing.T,
	registry *format.Registry,
	settings asset.IngestSettings,
	decorate func(storage.Store) storage.Store,
) (*gin.Engine, *http.Cookie, *asset.Service, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testdb.Connect(t)
	blobs, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	if decorate != nil {
		blobs = decorate(blobs)
	}
	assets := asset.NewServiceWithIngestSettings(pool, registry, blobs, settings)
	outbox := &verificationOutbox{}
	accounts := account.NewService(pool, outbox, nil, "http://localhost:3000")
	links := linking.NewService(pool, "http://localhost:3000")
	handlers := NewHandlers(assets, accounts, links, 1<<20)
	setup := registerTestRouter(t, handlers, DefaultDeadlines())
	session := signUp(t, setup, "verified@example.com", "verified.creator")
	verificationURL, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	verified := sendJSON(t, setup, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify test account: %d %s", verified.Code, verified.Body.String())
	}
	return registerTestRouter(t, handlers, DefaultDeadlines()), session, assets, pool
}

func TestCreatorCanPollTheirPendingIngest(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	upload := send(t, r, authorized(
		uploadRequest(t, exampleMetadata("Evening Theme"), []byte("theme bytes")),
		session,
	))

	poll := httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil)
	rec := send(t, r, authorized(poll, session))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var operation struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Status != "pending" {
		t.Errorf("status = %q, want pending", operation.Status)
	}
}

func TestLumithemeIngestFinishesAsAHintedPassthroughAsset(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Evening Theme")
	metadata["filename"] = "evening.lumitheme"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte("theme bytes")), session,
	))

	processed, err := assets.ProcessNextIngest(context.Background())
	if err != nil {
		t.Fatalf("process ingest: %v", err)
	}
	if !processed {
		t.Fatal("the pending ingest was not processed")
	}

	poll := httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil)
	rec := send(t, r, authorized(poll, session))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var operation struct {
		Status string `json:"status"`
		Asset  *struct {
			Kind                string  `json:"kind"`
			PassthroughPlatform *string `json:"passthroughPlatform"`
			Name                string  `json:"name"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Status != "success" || operation.Asset == nil {
		t.Fatalf("operation = %#v, want a successful asset", operation)
	}
	if operation.Asset.Kind != "theme" {
		t.Errorf("kind = %q, want theme", operation.Asset.Kind)
	}
	if operation.Asset.PassthroughPlatform == nil || *operation.Asset.PassthroughPlatform != "lumiverse" {
		t.Errorf("passthrough platform = %v, want lumiverse", operation.Asset.PassthroughPlatform)
	}
	if operation.Asset.Name != "Evening Theme" {
		t.Errorf("name = %q, want the confirmed catalog name", operation.Asset.Name)
	}
}

func TestUnknownUploadAsksForKindAndFilenameDerivedName(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Ignored initial name")
	metadata["filename"] = "velvet-night.bundle"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte("unrecognised bytes")), session,
	))
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}

	pollPath := upload.Header().Get("Location")
	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, pollPath, nil), session,
	))
	var waiting struct {
		Status    string         `json:"status"`
		NeedsKind map[string]any `json:"needsKind"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &waiting); err != nil {
		t.Fatalf("decode needs_kind operation: %v", err)
	}
	if waiting.Status != "needs_kind" {
		t.Fatalf("status = %q, want needs_kind. body: %s", waiting.Status, poll.Body.String())
	}
	if len(waiting.NeedsKind) != 2 || waiting.NeedsKind["kind"] != nil ||
		waiting.NeedsKind["name"] != "velvet-night" {
		t.Fatalf("needsKind = %#v, want only an empty kind and filename-derived name", waiting.NeedsKind)
	}

	completed := send(t, r, authorizedJSONRequest(t, http.MethodPatch, pollPath,
		`{"kind":"preset","name":"Velvet Night"}`, session))
	if completed.Code != http.StatusAccepted {
		t.Fatalf("complete status = %d, want 202. body: %s", completed.Code, completed.Body.String())
	}
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("resume ingest = %v, %v; want true, nil", processed, err)
	}

	finished := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, pollPath, nil), session,
	))
	var operation struct {
		Status string `json:"status"`
		Asset  *struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(finished.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode completed operation: %v", err)
	}
	if operation.Status != "success" || operation.Asset == nil ||
		operation.Asset.Kind != "preset" || operation.Asset.Name != "Velvet Night" {
		t.Fatalf("operation = %#v, want the completed preset", operation)
	}
}

func TestNeedsKindCompletionAcceptsOnlyKindAndName(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Ignored")
	metadata["filename"] = "unknown.bundle"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte("unrecognised bytes")), session,
	))
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}

	for _, body := range []string{
		`{"kind":"theme","name":"Unknown","extra":true}`,
		`{"kind":"theme","name":"Unknown"} {}`,
	} {
		completed := send(t, r, authorizedJSONRequest(
			t, http.MethodPatch, upload.Header().Get("Location"), body, session,
		))
		if completed.Code != http.StatusBadRequest {
			t.Fatalf("PATCH body %q status = %d, want 400. body: %s",
				body, completed.Code, completed.Body.String())
		}
	}
}

func TestClaimedFileThatFailsToParseIsRejectedWithoutAnAsset(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(parseFailureModule{}); err != nil {
		t.Fatalf("register module: %v", err)
	}
	r, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Broken card")
	metadata["filename"] = "broken.json"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte(`{"payload":true}`)), session,
	))
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}

	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Status  string `json:"status"`
		Failure *struct {
			Reason string `json:"reason"`
		} `json:"failure"`
		Asset any `json:"asset"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode failed operation: %v", err)
	}
	if operation.Status != "failed" || operation.Failure == nil ||
		operation.Failure.Reason != "malformed_input" {
		t.Fatalf("operation = %#v, want malformed_input", operation)
	}
	if operation.Asset != nil {
		t.Fatalf("failed ingest returned asset %#v", operation.Asset)
	}
	if listed := get(t, r, "/v1/assets"); len(listed) != 0 {
		t.Fatalf("browse found %d assets after a failed parse, want none", len(listed))
	}
}

func TestTerminalIngestFailuresStayDistinct(t *testing.T) {
	cases := []struct {
		name       string
		registry   func(t *testing.T) *format.Registry
		file       []byte
		wantReason string
	}{
		{
			name: "unsupported format",
			registry: func(*testing.T) *format.Registry {
				return format.NewRegistry()
			},
			file:       []byte(`{"spec":"future_card"}`),
			wantReason: "unsupported_format",
		},
		{
			name: "unsupported version",
			registry: func(t *testing.T) *format.Registry {
				registry := format.NewRegistry()
				err := registry.Register(typedParseFailureModule{
					err: format.UnsupportedVersion(errors.New("version 99")),
				})
				if err != nil {
					t.Fatalf("register module: %v", err)
				}
				return registry
			},
			file:       []byte(`{"payload":true}`),
			wantReason: "unsupported_version",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			r, session, assets := newVerifiedIngestRouter(t, test.registry(t))
			metadata := exampleMetadata("Refused")
			metadata["filename"] = "refused.json"
			upload := send(t, r, authorized(uploadRequest(t, metadata, test.file), session))
			if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
				t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
			}
			poll := send(t, r, authorized(
				httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
			))
			var operation struct {
				Status  string `json:"status"`
				Failure *struct {
					Reason string `json:"reason"`
				} `json:"failure"`
			}
			if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
				t.Fatalf("decode operation: %v", err)
			}
			if operation.Status != "failed" || operation.Failure == nil ||
				operation.Failure.Reason != test.wantReason {
				t.Fatalf("operation = %#v, want %s", operation, test.wantReason)
			}
		})
	}
}

func makeZIP(t *testing.T, header *zip.FileHeader) []byte {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	part, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatalf("create ZIP entry: %v", err)
	}
	if _, err := part.Write([]byte("safe bytes")); err != nil {
		t.Fatalf("write ZIP entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return data.Bytes()
}

func encryptedZIP(t *testing.T) []byte {
	t.Helper()
	data := makeZIP(t, &zip.FileHeader{Name: "theme.json", Method: zip.Store})
	binary.LittleEndian.PutUint16(data[6:8], binary.LittleEndian.Uint16(data[6:8])|1)
	central := bytes.Index(data, []byte("PK\x01\x02"))
	if central < 0 {
		t.Fatal("ZIP has no central directory")
	}
	binary.LittleEndian.PutUint16(
		data[central+8:central+10],
		binary.LittleEndian.Uint16(data[central+8:central+10])|1,
	)
	return data
}

func TestArchiveStructuralFailuresAreReportedFromTheWorker(t *testing.T) {
	symlink := &zip.FileHeader{Name: "theme.json", Method: zip.Store}
	symlink.SetMode(os.ModeSymlink | 0o777)
	cases := []struct {
		name       string
		file       func(t *testing.T) []byte
		wantReason string
	}{
		{"traversal", func(t *testing.T) []byte {
			return makeZIP(t, &zip.FileHeader{Name: "../escape", Method: zip.Store})
		}, "safety_violation"},
		{"absolute path", func(t *testing.T) []byte {
			return makeZIP(t, &zip.FileHeader{Name: "/escape", Method: zip.Store})
		}, "safety_violation"},
		{"symlink", func(t *testing.T) []byte { return makeZIP(t, symlink) }, "safety_violation"},
		{"encrypted", encryptedZIP, "safety_violation"},
		{"malformed", func(*testing.T) []byte { return []byte("PK\x03\x04broken") }, "malformed_input"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
			metadata := exampleMetadata("Unsafe theme")
			metadata["filename"] = "unsafe.lumitheme"
			upload := send(t, r, authorized(uploadRequest(t, metadata, test.file(t)), session))
			if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
				t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
			}
			poll := send(t, r, authorized(
				httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
			))
			var operation struct {
				Failure *struct {
					Reason string `json:"reason"`
				} `json:"failure"`
			}
			if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
				t.Fatalf("decode operation: %v", err)
			}
			if operation.Failure == nil || operation.Failure.Reason != test.wantReason {
				t.Fatalf("failure = %#v, want %s", operation.Failure, test.wantReason)
			}
		})
	}
}

type internalFailureModule struct {
	failuresLeft int
}

type catalogModule struct{}

type invalidFinalizationModule struct{}

func (invalidFinalizationModule) ID() string { return "invalid_finalization" }
func (invalidFinalizationModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (invalidFinalizationModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{Kind: "not-a-kind", Format: "invalid_finalization"}, nil
}

func (catalogModule) ID() string { return "catalog" }
func (catalogModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (catalogModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	nsfw := true
	return format.Parsed{
		Kind: "character", Format: "catalog", Name: "Moonlit Visitor",
		Blurb: "A quiet visitor from the edge of the wood.",
		Tags:  []string{"folklore", "gentle"}, IsNSFW: &nsfw,
	}, nil
}

func (*internalFailureModule) ID() string { return "internal_failure" }
func (*internalFailureModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (m *internalFailureModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	if m.failuresLeft > 0 {
		m.failuresLeft--
		return format.Parsed{}, format.InternalFailure(errors.New("temporary module failure"))
	}
	return format.Parsed{Kind: "character", Format: "test"}, nil
}

func TestOnlyInternalFailuresRetry(t *testing.T) {
	settings := asset.DefaultIngestSettings()
	settings.RetryBase = 0
	settings.MaxAttempts = 2
	module := &internalFailureModule{failuresLeft: 1}
	registry := format.NewRegistry()
	if err := registry.Register(module); err != nil {
		t.Fatalf("register module: %v", err)
	}
	r, session, assets, _ := newVerifiedIngestRouterWithSettings(t, registry, settings)
	metadata := exampleMetadata("Recovered card")
	metadata["filename"] = "recovered.json"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte(`{"payload":true}`)), session,
	))

	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("first process = %v, %v; want true, nil", processed, err)
	}
	firstPoll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var first struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(firstPoll.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first poll: %v", err)
	}
	if first.Status != "pending" {
		t.Fatalf("status after retryable failure = %q, want pending", first.Status)
	}

	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("second process = %v, %v; want true, nil", processed, err)
	}
	secondPoll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var second struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(secondPoll.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second poll: %v", err)
	}
	if second.Status != "success" {
		t.Fatalf("status after retry = %q, want success. body: %s", second.Status, secondPoll.Body.String())
	}
}

func TestExhaustedInternalFailureIsReported(t *testing.T) {
	settings := asset.DefaultIngestSettings()
	settings.RetryBase = 0
	settings.MaxAttempts = 2
	registry := format.NewRegistry()
	if err := registry.Register(&internalFailureModule{failuresLeft: 3}); err != nil {
		t.Fatalf("register module: %v", err)
	}
	r, session, assets, _ := newVerifiedIngestRouterWithSettings(t, registry, settings)
	metadata := exampleMetadata("Still broken")
	metadata["filename"] = "broken.json"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte(`{"payload":true}`)), session,
	))
	for attempt := 0; attempt < settings.MaxAttempts; attempt++ {
		if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
			t.Fatalf("process %d = %v, %v; want true, nil", attempt+1, processed, err)
		}
	}
	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Failure *struct {
			Reason string `json:"reason"`
		} `json:"failure"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Failure == nil || operation.Failure.Reason != "internal_failure" {
		t.Fatalf("failure = %#v, want internal_failure", operation.Failure)
	}
}

func TestFinalizationFailuresUseBoundedInternalRetries(t *testing.T) {
	settings := asset.DefaultIngestSettings()
	settings.RetryBase = 0
	settings.MaxAttempts = 2
	registry := format.NewRegistry()
	if err := registry.Register(invalidFinalizationModule{}); err != nil {
		t.Fatalf("register module: %v", err)
	}
	r, session, assets, _ := newVerifiedIngestRouterWithSettings(t, registry, settings)
	metadata := exampleMetadata("Invalid finalization")
	metadata["filename"] = "invalid.json"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte(`{"payload":true}`)), session,
	))

	for attempt, wantStatus := range []string{"pending", "failed"} {
		if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
			t.Fatalf("process %d = %v, %v; want true, nil", attempt+1, processed, err)
		}
		poll := send(t, r, authorized(
			httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
		))
		var operation struct {
			Status  string `json:"status"`
			Failure *struct {
				Reason string `json:"reason"`
			} `json:"failure"`
		}
		if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
			t.Fatalf("decode operation: %v", err)
		}
		if operation.Status != wantStatus {
			t.Fatalf("status after attempt %d = %q, want %q", attempt+1, operation.Status, wantStatus)
		}
		if wantStatus == "failed" &&
			(operation.Failure == nil || operation.Failure.Reason != "internal_failure") {
			t.Fatalf("failure = %#v, want internal_failure", operation.Failure)
		}
	}
}

func TestCatalogMetadataSeedsFromParseWithoutChangingTheFile(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(catalogModule{}); err != nil {
		t.Fatalf("register module: %v", err)
	}
	r, session, assets := newVerifiedIngestRouter(t, registry)
	file := []byte(`{"payload":"original"}`)
	upload := send(t, r, authorized(uploadRequest(t, map[string]any{
		"filename": "visitor.json", "confirmed": true,
	}, file), session))
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}
	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Asset *struct {
			ID     string   `json:"id"`
			Name   string   `json:"name"`
			Blurb  string   `json:"blurb"`
			Tags   []string `json:"tags"`
			IsNSFW bool     `json:"isNsfw"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Asset == nil || operation.Asset.Name != "Moonlit Visitor" ||
		operation.Asset.Blurb != "A quiet visitor from the edge of the wood." ||
		!operation.Asset.IsNSFW || strings.Join(operation.Asset.Tags, ",") != "folklore,gentle" {
		t.Fatalf("asset metadata = %#v, want the parsed catalog seed", operation.Asset)
	}

	assetID, err := uuid.Parse(operation.Asset.ID)
	if err != nil {
		t.Fatalf("parse asset id: %v", err)
	}
	stored, err := assets.OpenSource(context.Background(), assetID)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	got, err := io.ReadAll(stored)
	stored.Close()
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !bytes.Equal(got, file) {
		t.Fatalf("stored source = %q, want the original bytes", got)
	}
}

type blockingModule struct {
	started chan struct{}
	release chan struct{}
}

func (*blockingModule) ID() string { return "blocking" }
func (*blockingModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (m *blockingModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	close(m.started)
	<-m.release
	return format.Parsed{Kind: "character", Format: "blocking"}, nil
}

func TestPollingReportsProcessingWhileAWorkerHoldsTheLease(t *testing.T) {
	module := &blockingModule{started: make(chan struct{}), release: make(chan struct{})}
	registry := format.NewRegistry()
	if err := registry.Register(module); err != nil {
		t.Fatalf("register module: %v", err)
	}
	r, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Patient card")
	metadata["filename"] = "patient.json"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte(`{"payload":true}`)), session,
	))
	done := make(chan error, 1)
	go func() {
		_, err := assets.ProcessNextIngest(context.Background())
		done <- err
	}()
	<-module.started

	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Status != "processing" {
		t.Fatalf("status = %q, want processing", operation.Status)
	}
	close(module.release)
	if err := <-done; err != nil {
		t.Fatalf("finish ingest: %v", err)
	}
}

func TestIngestContinuesAfterTheUploadConnectionCloses(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	workerContext, stopWorkers := context.WithCancel(context.Background())
	workersDone := make(chan struct{})
	go func() {
		assets.RunIngestWorkers(workerContext, 1, nil)
		close(workersDone)
	}()
	defer func() {
		stopWorkers()
		<-workersDone
	}()

	metadata := exampleMetadata("Background theme")
	metadata["filename"] = "background.lumitheme"
	requestContext, closeConnection := context.WithCancel(context.Background())
	req := uploadRequest(t, metadata, []byte("theme bytes")).WithContext(requestContext)
	upload := send(t, r, authorized(req, session))
	closeConnection()

	deadline := time.Now().Add(3 * time.Second)
	for {
		poll := send(t, r, authorized(
			httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
		))
		var operation struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
			t.Fatalf("decode operation: %v", err)
		}
		if operation.Status == "success" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("operation stayed %q after the request closed", operation.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func revisionRequest(t *testing.T, assetID, filename string, file []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	writeFilePartNamed(t, form, filename, file)
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/assets/"+assetID+"/revisions", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func TestARevisionUploadReplacesTheBytesAndKeepsTheCatalogEntry(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Evening Theme")
	metadata["filename"] = "evening.lumitheme"
	upload := send(t, r, authorized(uploadRequest(t, metadata, []byte("first bytes")), session))
	if _, err := assets.ProcessNextIngest(context.Background()); err != nil {
		t.Fatalf("process ingest: %v", err)
	}
	created := pollIngestAsset(t, r, session, upload.Header().Get("Location"))
	firstFile := servedSourcePath(t, r, created.ID)

	revision := send(t, r, authorized(
		revisionRequest(t, created.ID, "evening.lumitheme", []byte("second bytes")), session,
	))
	if revision.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202. body: %s", revision.Code, revision.Body.String())
	}
	if _, err := assets.ProcessNextIngest(context.Background()); err != nil {
		t.Fatalf("process revision: %v", err)
	}
	updated := pollIngestAsset(t, r, session, revision.Header().Get("Location"))
	if updated.ID != created.ID {
		t.Fatalf("revision made asset %s, want %s", updated.ID, created.ID)
	}
	if updated.Name != created.Name {
		t.Fatalf("name = %q, want the creator's own %q", updated.Name, created.Name)
	}

	if servedSourcePath(t, r, created.ID) == firstFile {
		t.Fatal("the download still points at the first revision's file")
	}
}

// servedSourcePath is the file nginx is told to send, which is how a caller
// can see that a download changed without Go reading the bytes.
func servedSourcePath(t *testing.T, r *gin.Engine, assetID string) string {
	t.Helper()
	rec := send(t, r, httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200", rec.Code)
	}
	path := rec.Header().Get("X-Accel-Redirect")
	if path == "" {
		t.Fatal("X-Accel-Redirect is missing")
	}
	return path
}

func TestARevisionForSomebodyElsesAssetIsNotFound(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Evening Theme")
	metadata["filename"] = "evening.lumitheme"
	upload := send(t, r, authorized(uploadRequest(t, metadata, []byte("first bytes")), session))
	if _, err := assets.ProcessNextIngest(context.Background()); err != nil {
		t.Fatalf("process ingest: %v", err)
	}
	created := pollIngestAsset(t, r, session, upload.Header().Get("Location"))

	stranger := send(t, r, revisionRequest(t, created.ID, "evening.lumitheme", []byte("second")))
	if stranger.Code != http.StatusUnauthorized {
		t.Fatalf("signed-out status = %d, want 401", stranger.Code)
	}
}

func pollIngestAsset(t *testing.T, r *gin.Engine, session *http.Cookie, location string) struct {
	ID   string `json:"id"`
	Name string `json:"name"`
} {
	t.Helper()
	rec := send(t, r, authorized(httptest.NewRequest(http.MethodGet, location, nil), session))
	if rec.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var operation struct {
		Status string `json:"status"`
		Asset  *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode operation: %v", err)
	}
	if operation.Status != "success" || operation.Asset == nil {
		t.Fatalf("operation = %#v, want a successful asset", operation)
	}
	return *operation.Asset
}
