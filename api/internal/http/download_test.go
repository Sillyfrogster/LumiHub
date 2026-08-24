package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/google/uuid"
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

func TestAnonymousSourceDownloadRecordsTheAuthorizedHandoff(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)

	before := time.Now()
	download := send(t, router, httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil))
	after := time.Now()
	if download.Code != http.StatusOK || download.Header().Get("X-Accel-Redirect") == "" {
		t.Fatalf("download = %d, headers %v", download.Code, download.Header())
	}

	var revisionID, currentRevisionID uuid.UUID
	var target, authorizationClass, discovery string
	var handedOffAt time.Time
	err := pool.QueryRow(context.Background(), `
		select revision_id, export_target, handed_off_at, authorization_class, discovery
		  from download_events
		 where asset_id = $1
	`, assetID).Scan(&revisionID, &target, &handedOffAt, &authorizationClass, &discovery)
	if err != nil {
		t.Fatalf("read download event: %v", err)
	}
	if err := pool.QueryRow(context.Background(), `
		select current_revision_id from assets where id = $1
	`, assetID).Scan(&currentRevisionID); err != nil {
		t.Fatalf("read current revision: %v", err)
	}
	if revisionID != currentRevisionID || target != "raw" ||
		authorizationClass != "anonymous" || discovery != "listed" {
		t.Fatalf(
			"download event = revision %s, target %q, class %q, discovery %q",
			revisionID, target, authorizationClass, discovery,
		)
	}
	if handedOffAt.Before(before) || handedOffAt.After(after) {
		t.Fatalf("handoff time %s is outside request interval %s to %s", handedOffAt, before, after)
	}
	var eventCount int
	if err := pool.QueryRow(context.Background(), `
		select count(*) from download_events where asset_id = $1
	`, assetID).Scan(&eventCount); err != nil {
		t.Fatalf("count download events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("one handoff wrote %d download events", eventCount)
	}
}

func TestExportFromAnAssetMadeInIllarinRecordsTheHandoff(t *testing.T) {
	router, session, _, pool := newCharacterIngestRouterWithPool(t)
	started := startCharacter(t, router, session)
	writeCharacterFloor(t, router, session, started)
	if published := publishAsset(t, router, session, started.ID); published.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", published.Code, published.Body.String())
	}

	download := send(t, router, httptest.NewRequest(
		http.MethodGet, "/download/"+started.ID+"/chara_card_v3", nil,
	))
	if download.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200: %s", download.Code, download.Body.String())
	}

	var revisionMissing bool
	var target, authorizationClass string
	if err := pool.QueryRow(context.Background(), `
		select revision_id is null, export_target, authorization_class
		  from download_events
		 where asset_id = $1
	`, started.ID).Scan(&revisionMissing, &target, &authorizationClass); err != nil {
		t.Fatalf("read download event: %v", err)
	}
	if !revisionMissing || target != "chara_card_v3" || authorizationClass != "anonymous" {
		t.Fatalf("event = revision missing %t, target %q, class %q",
			revisionMissing, target, authorizationClass)
	}
}

type blockingRedirectStore struct {
	storage.Store
	reached chan struct{}
	release chan struct{}
}

func (s *blockingRedirectStore) InternalRedirect(ctx context.Context, id uuid.UUID) (string, error) {
	s.reached <- struct{}{}
	select {
	case <-s.release:
		return s.Store.InternalRedirect(ctx, id)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestDownloadSnapshotsDiscoveryAtHandoff(t *testing.T) {
	var blocker *blockingRedirectStore
	router, session, assets, pool := newVerifiedIngestRouterWithStore(
		t,
		format.NewRegistry(),
		asset.DefaultIngestSettings(),
		func(store storage.Store) storage.Store {
			blocker = &blockingRedirectStore{
				Store: store, reached: make(chan struct{}, 1), release: make(chan struct{}),
			}
			return blocker
		},
	)
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)

	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response <- send(t, router, httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil))
	}()
	select {
	case <-blocker.reached:
	case <-time.After(time.Second):
		t.Fatal("download did not reach the handoff boundary")
	}
	changed := send(t, router, authorizedJSONRequest(
		t, http.MethodPut, "/v1/assets/"+assetID+"/discovery",
		`{"discovery":"unlisted"}`, session,
	))
	if changed.Code != http.StatusNoContent {
		t.Fatalf("change discovery status = %d, want 204", changed.Code)
	}
	close(blocker.release)
	if download := <-response; download.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200", download.Code)
	}

	var discovery string
	if err := pool.QueryRow(context.Background(), `
		select discovery from download_events where asset_id = $1
	`, assetID).Scan(&discovery); err != nil {
		t.Fatalf("read download event: %v", err)
	}
	if discovery != "unlisted" {
		t.Fatalf("discovery at handoff = %q, want unlisted", discovery)
	}
}

func TestExportDownloadRecordsTheFormatItHandedOver(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)

	download := send(t, router, httptest.NewRequest(
		http.MethodGet, "/download/"+assetID+"/test_opaque", nil,
	))
	if download.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200: %s", download.Code, download.Body.String())
	}
	if got := download.Header().Get("X-Illarin-Export-Target"); got != "test_opaque" {
		t.Fatalf("target header = %q, want the format handed over", got)
	}
	// An export is a response, not stored content, so nothing was handed to
	// nginx and no blob was created.
	if redirect := download.Header().Get("X-Accel-Redirect"); redirect != "" {
		t.Fatalf("a generated export was served from disk at %q", redirect)
	}
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, ".txt") {
		t.Fatalf("disposition = %q, want a filename the format chose", got)
	}

	var target string
	if err := pool.QueryRow(context.Background(), `
		select export_target from download_events where asset_id = $1
	`, assetID).Scan(&target); err != nil {
		t.Fatalf("read download event: %v", err)
	}
	if target != "test_opaque" {
		t.Fatalf("recorded target = %q, want the format handed over", target)
	}
}

// A format outside the offered list was never a choice, so asking for it is a
// miss rather than a quieter answer in some other format.
func TestATargetTheAssetIsNotOfferedInIs404(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)

	response := send(t, router, httptest.NewRequest(
		http.MethodGet, "/download/"+assetID+"/chara_card_v2", nil,
	))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
}

func TestDownloadRecordsOneExclusiveBrowserAuthorizationClass(t *testing.T) {
	router, ownerSession, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, ownerSession, assets, asset.DiscoveryListed)
	readerSession := signUp(t, router, "reader@example.com", "signed.reader")

	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil),
		authorized(httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil), ownerSession),
		authorized(httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil), readerSession),
	}
	for _, request := range requests {
		if response := send(t, router, request); response.Code != http.StatusOK {
			t.Fatalf("download status = %d, want 200", response.Code)
		}
	}

	rows, err := pool.Query(context.Background(), `
		select authorization_class from download_events order by id
	`)
	if err != nil {
		t.Fatalf("read authorization classes: %v", err)
	}
	defer rows.Close()
	var classes []string
	for rows.Next() {
		var class string
		if err := rows.Scan(&class); err != nil {
			t.Fatalf("scan authorization class: %v", err)
		}
		classes = append(classes, class)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read authorization classes: %v", err)
	}
	if want := []string{"anonymous", "owner", "signed_in"}; !slices.Equal(classes, want) {
		t.Fatalf("authorization classes = %v, want %v", classes, want)
	}
}

func TestDownloadSnapshotsUnlistedAndOwnerWithheldAssets(t *testing.T) {
	router, ownerSession, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	unlistedID := uploadDiscoveryTestAsset(
		t, router, ownerSession, assets, asset.DiscoveryUnlisted,
	)
	withheldID := uploadDiscoveryTestAsset(
		t, router, ownerSession, assets, asset.DiscoveryListed,
	)
	if _, err := pool.Exec(context.Background(), `
		update assets asset
		   set withheld_at = now(), withheld_by = owner.id, withheld_reason = 'review'
		  from users owner
		 where asset.id = $1 and owner.username = 'verified.creator'
	`, withheldID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}

	unlisted := send(t, router, httptest.NewRequest(
		http.MethodGet, "/download/"+unlistedID, nil,
	))
	withheldRequest := authorized(httptest.NewRequest(
		http.MethodGet, "/download/"+withheldID, nil,
	), ownerSession)
	withheld := send(t, router, withheldRequest)
	if unlisted.Code != http.StatusOK || withheld.Code != http.StatusOK {
		t.Fatalf("download statuses = unlisted %d, withheld owner %d", unlisted.Code, withheld.Code)
	}

	for _, want := range []struct {
		assetID       string
		discovery     string
		authorization string
	}{
		{assetID: unlistedID, discovery: "unlisted", authorization: "anonymous"},
		{assetID: withheldID, discovery: "listed", authorization: "owner"},
	} {
		var discovery, authorization string
		if err := pool.QueryRow(context.Background(), `
			select discovery, authorization_class
			  from download_events
			 where asset_id = $1
		`, want.assetID).Scan(&discovery, &authorization); err != nil {
			t.Fatalf("read download event for %s: %v", want.assetID, err)
		}
		if discovery != want.discovery || authorization != want.authorization {
			t.Fatalf(
				"event for %s = discovery %q, class %q; want %q, %q",
				want.assetID, discovery, authorization, want.discovery, want.authorization,
			)
		}
	}
}

func TestDownloadUnknownAssetIs404(t *testing.T) {
	r, pool := newTestRouterWithSenderAndPool(
		t, 1<<20, DefaultDeadlines(), &verificationOutbox{},
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/download/11111111-1111-1111-1111-111111111111", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var eventCount int
	if err := pool.QueryRow(context.Background(), `select count(*) from download_events`).Scan(&eventCount); err != nil {
		t.Fatalf("count download events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("rejected request wrote %d download events", eventCount)
	}
}

// A creator edits a block, downloads the same format, and gets a file carrying
// the edit and every namespace the upload arrived with. The upload itself is
// still exactly what they handed over.
func TestEditingABlockChangesTheDownloadAndNotTheUpload(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	source := []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Before","first_mes":"Hello",
			"extensions": { "third_party": { "keep": true, "order": [3,1,2] } }}
	}`)
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.json"
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, source))

	page := fetchStartedAsset(t, r, session, assetID)
	core := editableBlock(blockNamed(t, page.Blocks, "character_core"))
	core.Elements[0].Content = json.RawMessage(`{"text":"After"}`)
	saved := saveBlock(t, r, session, assetID, blockNamed(t, page.Blocks, "character_core").ID, core)
	if saved.Code != http.StatusOK {
		t.Fatalf("save the description: %d %s", saved.Code, saved.Body.String())
	}

	download := send(t, r, httptest.NewRequest(
		http.MethodGet, "/download/"+assetID+"/chara_card_v3", nil,
	))
	if download.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200: %s", download.Code, download.Body.String())
	}
	var card struct {
		Data struct {
			Description string          `json:"description"`
			Extensions  json.RawMessage `json:"extensions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(download.Body.Bytes(), &card); err != nil {
		t.Fatalf("read the downloaded card: %v", err)
	}
	if card.Data.Description != "After" {
		t.Fatalf("downloaded description = %q, want the edit", card.Data.Description)
	}
	var sourceCard struct {
		Data struct {
			Extensions json.RawMessage `json:"extensions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(source, &sourceCard); err != nil {
		t.Fatalf("read the source fixture: %v", err)
	}
	if !bytes.Equal(compactJSON(t, card.Data.Extensions), compactJSON(t, sourceCard.Data.Extensions)) {
		t.Fatalf("third-party extensions changed\n got: %s\nwant: %s",
			card.Data.Extensions, sourceCard.Data.Extensions)
	}

	storedSource, err := assets.OpenSource(context.Background(), uuid.MustParse(assetID))
	if err != nil {
		t.Fatalf("OpenSource: %v", err)
	}
	storedBytes, readErr := io.ReadAll(storedSource)
	closeErr := storedSource.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(storedBytes, source) {
		t.Fatalf("the upload changed: read %v, close %v", readErr, closeErr)
	}
}

func compactJSON(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		t.Fatalf("compact %s: %v", raw, err)
	}
	return out.Bytes()
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

func TestFilenameExtensionAndDeclaredTypeCannotMakeAnUnknownSVGImportable(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(neverClaimsModule{}); err != nil {
		t.Fatalf("register non-claiming module: %v", err)
	}
	r, session, assets := newVerifiedIngestRouter(t, registry)
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

	removedCompletion := send(t, r, authorizedJSONRequest(
		t, http.MethodPatch, accepted.Header().Get("Location"),
		`{"kind":"theme","name":"Claimed image"}`, session,
	))
	if removedCompletion.Code != http.StatusNotFound {
		t.Fatalf("removed completion route status = %d, want 404", removedCompletion.Code)
	}
	poll := send(t, r, authorized(
		httptest.NewRequest(http.MethodGet, accepted.Header().Get("Location"), nil), session,
	))
	var operation struct {
		Failure *struct {
			Reason string `json:"reason"`
		} `json:"failure"`
	}
	if err := json.Unmarshal(poll.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if operation.Failure == nil || operation.Failure.Reason != "unsupported_format" {
		t.Fatalf("failure = %#v, want unsupported_format", operation.Failure)
	}
}
