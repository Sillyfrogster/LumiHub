package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/account"
	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreatorAddsMediaAndAnyoneFetchesAnImmutableVariant(t *testing.T) {
	r, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	metadata := exampleMetadata("Theme with screenshots")
	metadata["filename"] = "theme.lumitheme"
	created := uploadAndFinish(t, r, session, assets, metadata, []byte("theme"))
	assetID := assetIDFromIngest(t, created)

	added := send(t, r, authorized(mediaUploadRequest(
		t, assetID, "gallery", httpTestPNG(t, 1200, 600),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add media status = %d, want 201: %s", added.Code, added.Body.String())
	}
	var media struct {
		ID                string `json:"id"`
		AssetID           string `json:"assetId"`
		Role              string `json:"role"`
		Width             int    `json:"width"`
		Height            int    `json:"height"`
		DerivativeVersion uint32 `json:"derivativeVersion"`
	}
	if err := json.Unmarshal(added.Body.Bytes(), &media); err != nil {
		t.Fatalf("decode media response: %v", err)
	}
	if media.ID == "" || media.AssetID != assetID {
		t.Fatalf("media response = %+v", media)
	}
	var fields map[string]any
	if err := json.Unmarshal(added.Body.Bytes(), &fields); err != nil {
		t.Fatalf("decode media fields: %v", err)
	}
	if _, exists := fields["revisionId"]; exists {
		t.Fatalf("media response still carries revisionId: %s", added.Body.String())
	}
	if media.Role != "gallery" || media.Width != 1200 || media.Height != 600 {
		t.Fatalf("media response = %+v", media)
	}
	listed := send(t, r, httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+assetID+"/media", nil,
	))
	if listed.Code != http.StatusOK {
		t.Fatalf("list media status = %d, want 200: %s", listed.Code, listed.Body.String())
	}
	var mediaList struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &mediaList); err != nil {
		t.Fatalf("decode media list: %v", err)
	}
	if len(mediaList.Items) != 1 || mediaList.Items[0].ID != media.ID {
		t.Fatalf("media list = %+v, want %s", mediaList.Items, media.ID)
	}

	variantURL := "/media/" + media.ID + "/grid/" + "1"
	variant := send(t, r, httptest.NewRequest(http.MethodGet, variantURL, nil))
	if variant.Code != http.StatusOK {
		t.Fatalf("media status = %d, want 200: %s", variant.Code, variant.Body.String())
	}
	if got := variant.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := variant.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if got := variant.Header().Get("Content-Disposition"); got != "inline" {
		t.Errorf("Content-Disposition = %q, want inline", got)
	}
	if got := variant.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := variant.Header().Get("X-Accel-Redirect"); !strings.HasPrefix(got, "/_illarin/derivatives/") {
		t.Errorf("X-Accel-Redirect = %q, want internal derivative path", got)
	}
	if variant.Body.Len() != 0 {
		t.Errorf("Go wrote %d media bytes instead of handing off to nginx", variant.Body.Len())
	}
	preview := send(t, r, httptest.NewRequest(
		http.MethodGet, "/media/"+media.ID+"/og/1", nil,
	))
	if preview.Code != http.StatusOK {
		t.Fatalf("og preview status = %d, want 200: %s", preview.Code, preview.Body.String())
	}
	if preview.Header().Get("X-Accel-Redirect") == variant.Header().Get("X-Accel-Redirect") {
		t.Fatal("composed og preview reused the grid derivative")
	}
	var eventCount int
	if err := pool.QueryRow(context.Background(), `select count(*) from download_events`).Scan(&eventCount); err != nil {
		t.Fatalf("count download events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("media handoffs wrote %d download events", eventCount)
	}
}

func TestMediaRouteRefusesArbitraryVariantsAndVersions(t *testing.T) {
	r := newTestRouter(t)
	for _, path := range []string{
		"/media/11111111-1111-1111-1111-111111111111/1200x630/1",
		"/media/11111111-1111-1111-1111-111111111111/grid/999",
	} {
		response := send(t, r, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestMissingDerivativeYieldsToTheStorageReserveAndEvictsTheCache(t *testing.T) {
	root := t.TempDir()
	r, session, assets, pool := newVerifiedIngestRouterWithStoreFactory(
		t, format.NewRegistry(), asset.DefaultIngestSettings(),
		func(pool *pgxpool.Pool) (storage.Store, error) {
			return storage.NewStore(pool, root)
		},
	)
	created := uploadAndFinish(
		t, r, session, assets, exampleMetadata("Theme with one image"), []byte("theme"),
	)
	assetID := assetIDFromIngest(t, created)
	added := send(t, r, authorized(mediaUploadRequest(
		t, assetID, "gallery", httpTestPNG(t, 120, 60),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add media status = %d, want 201: %s", added.Code, added.Body.String())
	}
	var media struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(added.Body.Bytes(), &media); err != nil {
		t.Fatalf("decode media: %v", err)
	}

	unlimited, err := storage.NewStore(pool, root)
	if err != nil {
		t.Fatalf("open unlimited store: %v", err)
	}
	if err := unlimited.ClearDerivatives(context.Background()); err != nil {
		t.Fatalf("clear generated derivatives: %v", err)
	}
	var digestBytes []byte
	if err := pool.QueryRow(context.Background(), `
		select blob.sha256
		  from asset_media media
		  join blobs blob on blob.id = media.blob_id
		 where media.id = $1
	`, media.ID).Scan(&digestBytes); err != nil {
		t.Fatalf("read media digest: %v", err)
	}
	var digest [32]byte
	copy(digest[:], digestBytes)
	disposable := storage.DerivativeID{SourceDigest: digest, Variant: "old", Version: 1}
	if err := unlimited.PutDerivative(context.Background(), disposable, []byte("old cache")); err != nil {
		t.Fatalf("seed disposable derivative: %v", err)
	}

	limited, err := storage.NewStoreWithCapacity(pool, root, storage.Capacity{
		FreeSpaceReserveBytes: int64(^uint64(0) >> 1),
		MaximumBlobWriteBytes: 1,
	})
	if err != nil {
		t.Fatalf("open limited store: %v", err)
	}
	limitedAssets := asset.NewServiceWithIngestSettings(
		pool, format.NewRegistry(), limited, asset.DefaultIngestSettings(),
	)
	accounts := account.NewService(pool, &verificationOutbox{}, nil, "http://localhost:3000")
	links := newTestLinkingService(pool)
	handlers := NewHandlers(
		limitedAssets, accounts, links, newTestDeliveryService(pool, limitedAssets, links), 1<<20,
	)
	limitedRouter := registerTestRouter(t, handlers, DefaultDeadlines())

	response := send(t, limitedRouter, httptest.NewRequest(
		http.MethodGet, "/media/"+media.ID+"/grid/1", nil,
	))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("media status = %d, want 503: %s", response.Code, response.Body.String())
	}
	if _, err := limited.OpenDerivative(context.Background(), disposable); !errors.Is(err, storage.ErrDerivativeNotFound) {
		t.Fatalf("disposable derivative survived low space: %v", err)
	}
}

func TestCreatorMediaCannotTakeTheAccountPastItsStorageCap(t *testing.T) {
	root := t.TempDir()
	var blobs storage.Store
	r, session, assets, pool := newVerifiedIngestRouterWithStoreFactory(
		t, format.NewRegistry(), asset.DefaultIngestSettings(),
		func(pool *pgxpool.Pool) (storage.Store, error) {
			var err error
			blobs, err = storage.NewStore(pool, root)
			return blobs, err
		},
	)
	source := []byte("theme")
	created := uploadAndFinish(
		t, r, session, assets, exampleMetadata("Theme at its cap"), source,
	)
	assetID := assetIDFromIngest(t, created)
	mediaBytes := httpTestPNG(t, 120, 60)

	settings := asset.DefaultIngestSettings()
	settings.AccountStorageCapBytes = int64(len(source) + len(mediaBytes) - 1)
	limitedAssets := asset.NewServiceWithIngestSettings(
		pool, format.NewRegistry(), blobs, settings,
	)
	accounts := account.NewService(pool, &verificationOutbox{}, nil, "http://localhost:3000")
	links := newTestLinkingService(pool)
	handlers := NewHandlers(
		limitedAssets, accounts, links, newTestDeliveryService(pool, limitedAssets, links), 1<<20,
	)
	limitedRouter := registerTestRouter(t, handlers, DefaultDeadlines())

	response := send(t, limitedRouter, authorized(
		mediaUploadRequest(t, assetID, "gallery", mediaBytes), session,
	))

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("add media status = %d, want 413: %s", response.Code, response.Body.String())
	}
	var mediaCount int
	if err := pool.QueryRow(context.Background(), `select count(*) from asset_media`).Scan(&mediaCount); err != nil {
		t.Fatalf("count media: %v", err)
	}
	if mediaCount != 0 {
		t.Fatalf("over-cap upload recorded %d media rows", mediaCount)
	}
}

func mediaUploadRequest(t *testing.T, assetID, role string, file []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	writeMetadataPart(t, form, map[string]any{"role": role})
	writeFilePartNamed(t, form, "screenshot.png", file)
	if err := form.Close(); err != nil {
		t.Fatalf("close media form: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/assets/"+assetID+"/media", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return request
}

func assetIDFromIngest(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var operation struct {
		Asset *struct {
			ID string `json:"id"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if operation.Asset == nil {
		t.Fatal("ingest response has no asset")
	}
	return operation.Asset.ID
}

func httpTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.Set(x, y, color.RGBA{R: 20, G: 60, B: 100, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return encoded.Bytes()
}
