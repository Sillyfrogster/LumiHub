package http

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/account"
	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/character"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sys/unix"
)

type parseFailureModule struct{}

type opaqueTestModule struct{}

type neverClaimsModule struct{}

func testReaderDeclaration(id, kind string) format.Declaration {
	return format.Declaration{
		ID: id, Kind: kind, Direction: format.Direction{Read: true},
		Recognition: []format.Recognition{{
			Kind: format.RecognitionSignature, Containers: []probe.Container{probe.JSON},
			Required: map[string]format.ValueType{"payload": format.ValueBoolean},
		}},
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		ConsumedKeys:  []string{"payload"},
		Preservation:  format.PreservationDeclaration{Body: "test"},
		TestedOrigins: []string{id},
	}
}

func (neverClaimsModule) ID() string { return "never" }
func (neverClaimsModule) Declaration() format.Declaration {
	return testReaderDeclaration("never", "character")
}
func (neverClaimsModule) Claim(probe.Inspection) (format.Claim, bool) { return format.Claim{}, false }
func (neverClaimsModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{}, errors.New("unreachable")
}

func (opaqueTestModule) ID() string { return "test_opaque" }

// The stand-in reads and writes, because an asset with no writer is offered no
// download at all.
func (opaqueTestModule) Declaration() format.Declaration {
	declaration := testReaderDeclaration("test_opaque", "character")
	declaration.Label = "Test format"
	declaration.Direction.Write = true
	declaration.TestedOrigins = append(declaration.TestedOrigins, format.OriginIllarin)
	declaration.Roles = map[block.Role]format.DirectionalRoleSupport{
		block.RoleDescription: {
			Read:  format.RoleSupport{Grade: format.SupportFull},
			Write: format.RoleSupport{Grade: format.SupportFull},
		},
		block.RoleGreetings: {
			Read:  format.RoleSupport{Grade: format.SupportFull},
			Write: format.RoleSupport{Grade: format.SupportFull},
		},
	}
	return declaration
}
func (opaqueTestModule) Write(_ context.Context, written format.ExportAsset) (format.Artifact, error) {
	return format.Artifact{
		Body:      []byte(written.Text(block.RoleDescription)),
		MediaType: "text/plain", Extension: ".txt",
	}, nil
}
func (opaqueTestModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.WholeFileCompatibilityClaim(file), true
}
func (opaqueTestModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{
		Kind: "character", Format: "test_opaque",
		Elements: []block.Element{
			{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: "Test description"}},
			{Type: block.TypeTextSet, Role: block.RoleGreetings, Content: block.TextSet{Texts: []block.TextItem{{ID: block.NewItemID(), Text: "Hello"}}}},
		},
	}, nil
}

func (parseFailureModule) ID() string { return "claimed" }
func (parseFailureModule) Declaration() format.Declaration {
	return testReaderDeclaration("claimed", "character")
}
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
func (typedParseFailureModule) Declaration() format.Declaration {
	return testReaderDeclaration("typed_failure", "character")
}
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

func TestUploadWaitsWhenItsMaximumWriteWouldCrossTheStorageReserve(t *testing.T) {
	root := t.TempDir()
	var filesystem unix.Statfs_t
	if err := unix.Statfs(root, &filesystem); err != nil {
		t.Fatalf("read free space: %v", err)
	}
	available := int64(filesystem.Bavail) * filesystem.Bsize
	const headroom = int64(8 << 20)
	if available <= headroom {
		t.Skip("test filesystem has less than 8 MB free")
	}

	r, session, _, pool := newVerifiedIngestRouterWithStoreFactory(
		t, format.NewRegistry(), asset.DefaultIngestSettings(),
		func(pool *pgxpool.Pool) (storage.Store, error) {
			return storage.NewStoreWithCapacity(pool, root, storage.Capacity{
				FreeSpaceReserveBytes: available - headroom,
				MaximumBlobWriteBytes: 32 << 20,
			})
		},
	)

	response := send(t, r, authorized(
		uploadRequest(t, exampleMetadata("Waiting theme"), []byte("small upload")),
		session,
	))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "" {
		t.Fatalf("refused upload has Location %q", response.Header().Get("Location"))
	}
	var operations int
	if err := pool.QueryRow(context.Background(), `select count(*) from ingest_operations`).Scan(&operations); err != nil {
		t.Fatalf("count ingest operations: %v", err)
	}
	if operations != 0 {
		t.Fatalf("refused upload recorded %d ingest operations", operations)
	}
}

func TestAccountStorageCapChargesSharedBytesPerAccountButNotRepeatedUse(t *testing.T) {
	shared := []byte("shared canonical bytes")
	root := t.TempDir()
	var blobs storage.Store
	r, firstSession, assets, pool := newVerifiedIngestRouterWithStoreFactory(
		t, format.NewRegistry(), asset.DefaultIngestSettings(),
		func(pool *pgxpool.Pool) (storage.Store, error) {
			var err error
			blobs, err = storage.NewStore(pool, root)
			return blobs, err
		},
	)
	seed := send(t, r, authorized(
		uploadRequest(t, exampleMetadata("First use"), shared), firstSession,
	))
	if seed.Code != http.StatusAccepted {
		t.Fatalf("seed upload status = %d, want 202: %s", seed.Code, seed.Body.String())
	}
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process seed ingest = %v, %v; want true, nil", processed, err)
	}
	created := pollIngestAsset(t, r, firstSession, seed.Header().Get("Location"))

	settings := asset.DefaultIngestSettings()
	settings.AccountStorageCapBytes = int64(len(shared) - 1)
	limitedAssets := asset.NewServiceWithIngestSettings(
		pool, format.NewRegistry(), blobs, settings,
	)
	outbox := &verificationOutbox{}
	accounts := account.NewService(pool, outbox, nil, "http://localhost:3000")
	links := newTestLinkingService(pool)
	handlers := NewHandlers(
		limitedAssets, accounts, links, newTestDeliveryService(pool, limitedAssets, links), 1<<20,
	)
	limitedRouter := registerTestRouter(t, handlers, DefaultDeadlines())

	repeated := send(t, limitedRouter, authorized(
		uploadRequest(t, exampleMetadata("Repeated use"), shared), firstSession,
	))
	if repeated.Code != http.StatusAccepted {
		t.Fatalf("repeated upload status = %d, want 202: %s", repeated.Code, repeated.Body.String())
	}
	repeatedRevision := send(t, limitedRouter, authorized(
		revisionRequest(t, created.ID, "same.bin", shared), firstSession,
	))
	if repeatedRevision.Code != http.StatusAccepted {
		t.Fatalf("repeated revision status = %d, want 202: %s", repeatedRevision.Code, repeatedRevision.Body.String())
	}
	distinctRevision := send(t, limitedRouter, authorized(
		revisionRequest(t, created.ID, "different.bin", []byte("different canonical bytes")), firstSession,
	))
	if distinctRevision.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("distinct revision status = %d, want 413: %s", distinctRevision.Code, distinctRevision.Body.String())
	}

	secondSession := signUp(t, limitedRouter, "second@example.com", "second.creator")
	verificationURL, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse second verification link: %v", err)
	}
	verified := sendJSON(t, limitedRouter, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify second account: %d %s", verified.Code, verified.Body.String())
	}
	charged := send(t, limitedRouter, authorized(
		uploadRequest(t, exampleMetadata("Somebody else's use"), shared), secondSession,
	))
	if charged.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("cross-account upload status = %d, want 413: %s", charged.Code, charged.Body.String())
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
	finished := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, accepted.Header().Get("Location"), nil), session,
	))
	if keep, _ := metadata["_keepDraft"].(bool); !keep {
		var operation struct {
			Status string `json:"status"`
			Asset  *struct {
				ID string `json:"id"`
			} `json:"asset"`
		}
		if json.Unmarshal(finished.Body.Bytes(), &operation) == nil &&
			operation.Status == "success" && operation.Asset != nil {
			_ = send(t, r, authorized(httptest.NewRequest(
				http.MethodPost, "/v1/assets/"+operation.Asset.ID+"/publish", nil,
			), session))
		}
	}
	return finished
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
	return newVerifiedIngestRouterWithStoreFactory(t, registry, settings,
		func(pool *pgxpool.Pool) (storage.Store, error) {
			blobs, err := storage.NewStore(pool, t.TempDir())
			if err == nil && decorate != nil {
				blobs = decorate(blobs)
			}
			return blobs, err
		},
	)
}

func newVerifiedIngestRouterWithStoreFactory(
	t *testing.T,
	registry *format.Registry,
	settings asset.IngestSettings,
	storeFactory func(*pgxpool.Pool) (storage.Store, error),
) (*gin.Engine, *http.Cookie, *asset.Service, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testdb.Connect(t)
	blobs, err := storeFactory(pool)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	if registry.Empty() {
		if err := registry.Register(opaqueTestModule{}); err != nil {
			t.Fatalf("register test format: %v", err)
		}
	}
	assets := asset.NewServiceWithIngestSettings(pool, registry, blobs, settings)
	outbox := &verificationOutbox{}
	accounts := account.NewService(pool, outbox, nil, "http://localhost:3000")
	links := newTestLinkingService(pool)
	handlers := NewHandlers(assets, accounts, links, newTestDeliveryService(pool, assets, links), 1<<20)
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

func TestCharacterUploadLandsOnABuiltDraftPage(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	r, session, assets, pool := newVerifiedIngestRouterWithPool(t, registry)
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.json"
	metadata["_keepDraft"] = true
	card := []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{
			"name":"Ana","nickname":"Archivist","character_version":"main","creator":"A. Writer",
			"description":"Keeps the archive.","personality":"Patient",
			"scenario":"After closing","first_mes":"Welcome back.",
			"group_only_greetings":["All of you made it."],
			"system_prompt":"Stay in character.","future_structure":{"kept":"whole"}
		}
	}`)
	finished := uploadAndFinish(t, r, session, assets, metadata, card)
	assetID := assetIDFromIngest(t, finished)

	pageResponse := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID, nil), session,
	))
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("page status = %d, want 200: %s", pageResponse.Code, pageResponse.Body.String())
	}
	var page startedAsset
	if err := json.Unmarshal(pageResponse.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode imported page: %v", err)
	}
	if page.Lifecycle != "draft" {
		t.Errorf("lifecycle = %q, want draft", page.Lifecycle)
	}
	if len(page.Blocks) != 3 {
		t.Fatalf("blocks = %d, want character core, messages and model instructions", len(page.Blocks))
	}
	core := blockNamed(t, page.Blocks, "character_core")
	if core.Layout != "stack-3" || core.Width != "two_thirds" {
		t.Errorf("character core = %s at %s", core.Layout, core.Width)
	}
	if string(core.Elements[0].Content) != `{"text":"Keeps the archive."}` {
		t.Errorf("description = %s", core.Elements[0].Content)
	}
	messages := blockNamed(t, page.Blocks, "messages")
	if messages.Layout != "stack-3" || len(messages.Elements) != 3 {
		t.Errorf("messages = %+v, want three roles in stack-3", messages)
	}
	instructions := blockNamed(t, page.Blocks, "model_instructions")
	if instructions.Width != "half" || len(instructions.Elements) != 1 ||
		instructions.Elements[0].Role != "system_prompt" {
		t.Errorf("model instructions = %+v", instructions)
	}
	var origin, version, author, nickname string
	if err := pool.QueryRow(context.Background(), `
		select origin_format, asset_version, credited_author, nickname
		  from assets where id = $1
	`, assetID).Scan(&origin, &version, &author, &nickname); err != nil {
		t.Fatalf("read imported header: %v", err)
	}
	if origin != character.V3 || version != "main" || author != "A. Writer" || nickname != "Archivist" {
		t.Errorf("origin and header = %q, %q, %q, %q", origin, version, author, nickname)
	}
	var preserved []byte
	if err := pool.QueryRow(context.Background(), `
		select payload from asset_preserved_data where asset_id = $1 and namespace = 'card'
	`, assetID).Scan(&preserved); err != nil {
		t.Fatalf("read preserved remainder: %v", err)
	}
	if !bytes.Contains(preserved, []byte(`"future_structure"`)) {
		t.Errorf("preserved remainder = %s", preserved)
	}
}

func TestEveryCharacterReaderBuildsTheCatalogPage(t *testing.T) {
	cardBody := func(spec, description string) []byte {
		version := "3.0"
		if spec == character.V2 {
			version = "2.0"
		}
		return []byte(fmt.Sprintf(`{
			"spec":%q,"spec_version":%q,
			"data":{"name":"Ana","description":%q,"first_mes":"Hello"}
		}`, spec, version, description))
	}
	tests := []struct {
		name, filename, origin, description string
		file                                []byte
	}{
		{name: "CCv2", filename: "ana-v2.json", origin: character.V2,
			description: "From V2", file: cardBody(character.V2, "From V2")},
		{name: "CCv3", filename: "ana-v3.json", origin: character.V3,
			description: "From V3", file: cardBody(character.V3, "From V3")},
		{name: "CharX", filename: "ana.charx", origin: character.CharX,
			description: "From CharX", file: zipCharacterCard(t, cardBody(character.V3, "From CharX"))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := format.NewRegistry()
			for _, module := range character.Modules() {
				if err := registry.Register(module); err != nil {
					t.Fatalf("register %s: %v", module.ID(), err)
				}
			}
			r, session, assets, pool := newVerifiedIngestRouterWithPool(t, registry)
			metadata := exampleMetadata("Ana")
			metadata["filename"] = test.filename
			metadata["_keepDraft"] = true
			assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, test.file))
			response := send(t, r, authorized(httptest.NewRequest(
				http.MethodGet, "/v1/assets/"+assetID, nil,
			), session))
			var page startedAsset
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil {
				t.Fatalf("page = %d: %s", response.Code, response.Body.String())
			}
			if page.Lifecycle != "draft" || len(page.Blocks) != 2 {
				t.Fatalf("page = lifecycle %q, blocks %+v", page.Lifecycle, page.Blocks)
			}
			core := blockNamed(t, page.Blocks, "character_core")
			if core.Layout != "stack-3" || core.Width != "two_thirds" ||
				string(core.Elements[0].Content) != fmt.Sprintf(`{"text":%q}`, test.description) {
				t.Errorf("character core = %+v", core)
			}
			messages := blockNamed(t, page.Blocks, "messages")
			if messages.Layout != "stack-2" || messages.Width != "full" || len(messages.Elements) != 2 {
				t.Errorf("messages = %+v", messages)
			}
			var origin string
			if err := pool.QueryRow(context.Background(),
				`select origin_format from assets where id = $1`, assetID,
			).Scan(&origin); err != nil || origin != test.origin {
				t.Errorf("origin = %q, %v; want %q", origin, err, test.origin)
			}
		})
	}
}

func TestAnUnreadableOptionalCharXImageDoesNotRejectTheCharacter(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	r, session, assets, pool := newVerifiedIngestRouterWithPool(t, registry)
	card := []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"assets":[{"type":"emotion","uri":"embeded://assets/bad.png","name":"bad","ext":"png"}]}
	}`)
	file := zipCharacterCardWithFiles(t, card, map[string][]byte{
		"assets/bad.png": []byte("not an image"),
	})
	marker := []byte("not an image")
	position := bytes.Index(file, marker)
	if position < 0 {
		t.Fatal("stored image bytes are missing from the CharX fixture")
	}
	file[position] ^= 0xff
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.charx"
	metadata["_keepDraft"] = true
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, file))

	var mediaCount int
	if err := pool.QueryRow(context.Background(),
		`select count(*) from asset_media where asset_id = $1`, assetID,
	).Scan(&mediaCount); err != nil {
		t.Fatalf("count extracted media: %v", err)
	}
	if mediaCount != 0 {
		t.Errorf("extracted media = %d, want the unreadable optional image skipped", mediaCount)
	}
	var preserved []byte
	if err := pool.QueryRow(context.Background(), `
		select payload from asset_preserved_data where asset_id = $1 and namespace = 'card'
	`, assetID).Scan(&preserved); err != nil {
		t.Fatalf("read preserved assets: %v", err)
	}
	var cardRemainder map[string]json.RawMessage
	if err := json.Unmarshal(preserved, &cardRemainder); err != nil {
		t.Fatalf("decode preserved card data: %v", err)
	}
	assetsRemainder, ok := cardRemainder["assets"]
	if !ok || !bytes.Contains(assetsRemainder, []byte("bad.png")) {
		t.Fatalf("preserved card data = %s", preserved)
	}
}

func TestExtractedMediaCannotTakeTheAccountPastItsStorageCap(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	image := httpTestPNG(t, 120, 60)
	card := []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"assets":[{"type":"emotion","uri":"embeded://assets/happy.png","name":"happy","ext":"png"}]}
	}`)
	file := zipCharacterCardWithFiles(t, card, map[string][]byte{"assets/happy.png": image})
	settings := asset.DefaultIngestSettings()
	settings.AccountStorageCapBytes = int64(len(file) + len(image) - 1)
	r, session, assets, pool := newVerifiedIngestRouterWithSettings(t, registry, settings)
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.charx"
	upload := send(t, r, authorized(uploadRequest(t, metadata, file), session))
	if upload.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want 202: %s", upload.Code, upload.Body.String())
	}

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
		t.Fatalf("decode ingest: %v", err)
	}
	if operation.Status != "failed" || operation.Failure == nil || operation.Failure.Reason != "limit_exceeded" {
		t.Fatalf("operation = %#v, want a storage-limit failure", operation)
	}
	var assetCount int
	if err := pool.QueryRow(context.Background(), `select count(*) from assets`).Scan(&assetCount); err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if assetCount != 0 {
		t.Fatalf("over-cap ingest recorded %d assets", assetCount)
	}
}

func zipCharacterCard(t *testing.T, card []byte) []byte {
	return zipCharacterCardWithFiles(t, card, nil)
}

func zipCharacterCardWithFiles(t *testing.T, card []byte, files map[string][]byte) []byte {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	part, err := archive.Create("card.json")
	if err != nil {
		t.Fatalf("create card.json: %v", err)
	}
	if _, err := part.Write(card); err != nil {
		t.Fatalf("write card.json: %v", err)
	}
	for name, content := range files {
		part, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close CharX: %v", err)
	}
	return data.Bytes()
}

func TestUnknownUploadIsRefusedAndNothingIsStored(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(neverClaimsModule{}); err != nil {
		t.Fatalf("register non-claiming module: %v", err)
	}
	r, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Unknown")
	metadata["filename"] = "velvet-night.bundle"
	upload := send(t, r, authorized(
		uploadRequest(t, metadata, []byte("unrecognised bytes")), session,
	))
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}

	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, upload.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Status  string `json:"status"`
		Asset   any    `json:"asset"`
		Failure *struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"failure"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode refused operation: %v", err)
	}
	if operation.Status != "failed" || operation.Asset != nil || operation.Failure == nil {
		t.Fatalf("operation = %#v, want a refusal with no asset", operation)
	}
	if operation.Failure.Reason != "unsupported_format" ||
		!strings.Contains(operation.Failure.Message, "start from nothing") {
		t.Errorf("failure = %+v", operation.Failure)
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
			registry: func(t *testing.T) *format.Registry {
				registry := format.NewRegistry()
				if err := registry.Register(neverClaimsModule{}); err != nil {
					t.Fatalf("register non-claiming module: %v", err)
				}
				return registry
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
func (invalidFinalizationModule) Declaration() format.Declaration {
	return testReaderDeclaration("invalid_finalization", "not-a-kind")
}
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
func (catalogModule) Declaration() format.Declaration {
	return testReaderDeclaration("catalog", "character")
}
func (catalogModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (catalogModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	nsfw := true
	return format.Parsed{
		Kind: "character", Format: "catalog",
		Header: format.Header{
			Name: "Moonlit Visitor", Blurb: "A quiet visitor from the edge of the wood.",
		},
		Tags: []string{"folklore", "gentle"}, IsNSFW: &nsfw,
		Elements: []block.Element{
			{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: "A quiet visitor."}},
			{Type: block.TypeTextSet, Role: block.RoleGreetings, Content: block.TextSet{Texts: []block.TextItem{{ID: block.NewItemID(), Text: "Good evening."}}}},
		},
	}, nil
}

func (*internalFailureModule) ID() string { return "internal_failure" }
func (*internalFailureModule) Declaration() format.Declaration {
	return testReaderDeclaration("internal_failure", "character")
}
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
	return format.Parsed{Kind: "character", Format: "internal_failure"}, nil
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

func TestAClaimedKindWithoutABlockCatalogIsRefused(t *testing.T) {
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

	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process = %v, %v; want true, nil", processed, err)
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
		operation.Failure.Reason != "unsupported_format" {
		t.Fatalf("operation = %#v, want an unsupported-format refusal", operation)
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
	published := send(t, r, authorized(httptest.NewRequest(
		http.MethodPost, "/v1/assets/"+operation.Asset.ID+"/publish", nil,
	), session))
	if published.Code != http.StatusOK {
		t.Fatalf("publish imported asset = %d: %s", published.Code, published.Body.String())
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
func (*blockingModule) Declaration() format.Declaration {
	return testReaderDeclaration("blocking", "character")
}
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
	published := send(t, r, authorized(httptest.NewRequest(
		http.MethodPost, "/v1/assets/"+created.ID+"/publish", nil,
	), session))
	if published.Code != http.StatusOK {
		t.Fatalf("publish initial import = %d: %s", published.Code, published.Body.String())
	}
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
