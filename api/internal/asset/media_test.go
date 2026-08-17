package asset

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	mediaproc "github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
)

func TestCreatorAddedMediaKeepsNativeDimensionsAndPreGeneratesVariants(t *testing.T) {
	svc, pool := newTestService(t)
	ownerID := uuid.New()
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "theme.bin",
		File: bytes.NewReader([]byte("theme")), Name: "Theme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	source := testPNG(t, 1200, 600, color.RGBA{R: 12, G: 34, B: 56, A: 255})

	added, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaGallery,
		File: bytes.NewReader(source),
	})
	if err != nil {
		t.Fatalf("AddMedia: %v", err)
	}
	if added.ID == uuid.Nil {
		t.Fatal("media id is empty")
	}
	if added.AssetID != created.ID {
		t.Fatalf("media asset = %v, want %s", added.AssetID, created.ID)
	}
	if added.Width != 1200 || added.Height != 600 {
		t.Fatalf("dimensions = %dx%d, want native 1200x600", added.Width, added.Height)
	}
	if added.DerivativeVersion != mediaproc.DerivativeVersion {
		t.Fatalf("derivative version = %d, want %d", added.DerivativeVersion, mediaproc.DerivativeVersion)
	}

	var blobID uuid.UUID
	var digestBytes []byte
	var storedAsset uuid.UUID
	err = pool.QueryRow(context.Background(), `
		select media.blob_id, blob.sha256, media.asset_id
		  from asset_media media
		  join blobs blob on blob.id = media.blob_id
		 where media.id = $1
	`, added.ID).Scan(&blobID, &digestBytes, &storedAsset)
	if err != nil {
		t.Fatalf("read media row: %v", err)
	}
	if storedAsset != created.ID {
		t.Fatalf("stored media asset = %v, want %s", storedAsset, created.ID)
	}
	var digest [sha256.Size]byte
	copy(digest[:], digestBytes)
	for _, variant := range mediaproc.VariantNames() {
		opened, err := svc.store.OpenDerivative(context.Background(), storage.DerivativeID{
			SourceDigest: digest, Variant: variant, Version: mediaproc.DerivativeVersion,
		})
		if err != nil {
			t.Fatalf("open pre-generated %s derivative: %v", variant, err)
		}
		opened.Close()
	}

	canonical, err := svc.store.Open(context.Background(), blobID)
	if err != nil {
		t.Fatalf("open canonical media: %v", err)
	}
	var canonicalBytes bytes.Buffer
	if _, err := canonicalBytes.ReadFrom(canonical); err != nil {
		t.Fatalf("read canonical media: %v", err)
	}
	canonical.Close()
	if !bytes.Equal(canonicalBytes.Bytes(), source) {
		t.Fatal("canonical media was changed while making variants")
	}
}

func TestAddingAReplacementMintsANewImmutableMediaRecord(t *testing.T) {
	svc, _ := newTestService(t)
	ownerID := uuid.New()
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "theme.bin",
		File: bytes.NewReader([]byte("theme")), Name: "Theme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	first, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaAvatar,
		File: bytes.NewReader(testPNG(t, 20, 10, color.Black)),
	})
	if err != nil {
		t.Fatalf("Add first media: %v", err)
	}
	second, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaAvatar,
		File: bytes.NewReader(testPNG(t, 30, 15, color.White)),
	})
	if err != nil {
		t.Fatalf("Add replacement media: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("replacement reused the old media id")
	}
	if first.Width != 20 || first.Height != 10 {
		t.Fatalf("first media mutated to %dx%d", first.Width, first.Height)
	}
	var coverID uuid.UUID
	if err := svc.pool.QueryRow(context.Background(),
		`select cover_media_id from assets where id = $1`, created.ID,
	).Scan(&coverID); err != nil {
		t.Fatalf("read cover: %v", err)
	}
	if coverID != second.ID {
		t.Fatalf("cover = %s, want replacement %s", coverID, second.ID)
	}
}

func TestAlternateAvatarCoversUntilAPrimaryTakesItsPlace(t *testing.T) {
	svc, _ := newTestService(t)
	ownerID := uuid.New()
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "theme.bin",
		File: bytes.NewReader([]byte("theme")), Name: "Theme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaAvatarAlt,
		File: bytes.NewReader(testPNG(t, 20, 10, color.Black)),
	})
	if err != nil {
		t.Fatalf("Add alternate avatar: %v", err)
	}
	alternate, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaAvatarAlt,
		File: bytes.NewReader(testPNG(t, 30, 15, color.White)),
	})
	if err != nil {
		t.Fatalf("Add second alternate avatar: %v", err)
	}
	var coverID uuid.UUID
	if err := svc.pool.QueryRow(context.Background(),
		`select cover_media_id from assets where id = $1`, created.ID,
	).Scan(&coverID); err != nil {
		t.Fatalf("read alternate cover: %v", err)
	}
	if coverID != alternate.ID {
		t.Fatalf("cover = %s, want latest alternate %s", coverID, alternate.ID)
	}
	primary, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaAvatar,
		File: bytes.NewReader(testPNG(t, 40, 20, color.Gray{Y: 128})),
	})
	if err != nil {
		t.Fatalf("Add primary avatar: %v", err)
	}
	if _, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaAvatarAlt,
		File: bytes.NewReader(testPNG(t, 50, 25, color.White)),
	}); err != nil {
		t.Fatalf("Add alternate after primary: %v", err)
	}
	if err := svc.pool.QueryRow(context.Background(),
		`select cover_media_id from assets where id = $1`, created.ID,
	).Scan(&coverID); err != nil {
		t.Fatalf("read cover: %v", err)
	}
	if coverID != primary.ID || coverID == alternate.ID {
		t.Fatalf("cover = %s, want primary %s", coverID, primary.ID)
	}
}

func TestMediaVariantRegeneratesABoundedCacheMiss(t *testing.T) {
	svc, _ := newTestService(t)
	ownerID := uuid.New()
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "theme.bin",
		File: bytes.NewReader([]byte("theme")), Name: "Theme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	added, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaGallery,
		File: bytes.NewReader(testPNG(t, 320, 180, color.White)),
	})
	if err != nil {
		t.Fatalf("AddMedia: %v", err)
	}
	if err := svc.store.ClearDerivatives(context.Background()); err != nil {
		t.Fatalf("clear derivative cache: %v", err)
	}

	download, err := svc.MediaVariant(
		context.Background(), added.ID, "grid", mediaproc.DerivativeVersion, nil,
	)
	if err != nil {
		t.Fatalf("MediaVariant cache miss: %v", err)
	}
	if download.InternalRedirect == "" || download.MediaType != mediaproc.DerivativeType {
		t.Fatalf("download = %+v", download)
	}

	for _, request := range []struct {
		variant string
		version uint32
	}{
		{variant: "1200x630", version: mediaproc.DerivativeVersion},
		{variant: "grid", version: mediaproc.DerivativeVersion + 1},
	} {
		_, err := svc.MediaVariant(context.Background(), added.ID, request.variant, request.version, nil)
		if !errors.Is(err, ErrMediaNotFound) {
			t.Fatalf("MediaVariant(%q, %d) error = %v, want ErrMediaNotFound",
				request.variant, request.version, err)
		}
	}
}

func TestCreatorCannotAddMediaToSomebodyElsesAsset(t *testing.T) {
	svc, _ := newTestService(t)
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "theme", Filename: "theme.bin",
		File: bytes.NewReader([]byte("theme")), Name: "Theme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: uuid.New(), AssetID: created.ID, Role: MediaGallery,
		File: bytes.NewReader(testPNG(t, 20, 10, color.White)),
	})
	if !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("AddMedia error = %v, want ErrMediaNotFound", err)
	}
}

func TestIngestStoresExtractedMediaOnTheAsset(t *testing.T) {
	archive := archiveWithImage(t, testPNG(t, 90, 45, color.White))
	registry := registryWithModule(t, recognizedModule{parsed: format.Parsed{
		Kind: "character", Format: "recognized",
		Media: []format.Media{{Role: "expression", ImageID: 0}},
	}})
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'media.extractor')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	operation, err := svc.AcceptIngest(context.Background(), IngestInput{
		OwnerID: ownerID, Filename: "card.charx", File: bytes.NewReader(archive),
	})
	if err != nil {
		t.Fatalf("AcceptIngest: %v", err)
	}
	processed, err := svc.ProcessNextIngest(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessNextIngest = %v, %v; want true, nil", processed, err)
	}
	operation, err = svc.GetIngest(context.Background(), ownerID, operation.ID)
	if err != nil {
		t.Fatalf("GetIngest: %v", err)
	}
	if operation.Asset == nil {
		t.Fatal("ingest did not create an asset")
	}

	var assetID uuid.UUID
	var role string
	var width, height int
	err = pool.QueryRow(context.Background(), `
		select asset_id, role, width, height
		  from asset_media
		 where asset_id = $1
	`, operation.Asset.ID).Scan(&assetID, &role, &width, &height)
	if err != nil {
		t.Fatalf("read extracted media: %v", err)
	}
	if assetID != operation.Asset.ID {
		t.Fatalf("extracted media asset = %v, want %v", assetID, operation.Asset.ID)
	}
	if role != "expression" || width != 90 || height != 45 {
		t.Fatalf("extracted media = %s %dx%d", role, width, height)
	}
}

func TestConcurrentCacheMissesShareOneBoundedRender(t *testing.T) {
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	processor := &blockingMediaProcessor{
		renderStarted: make(chan struct{}),
		releaseRender: make(chan struct{}),
	}
	settings := DefaultIngestSettings()
	settings.MediaWorkers = 1
	svc := NewServiceWithMediaProcessor(
		pool, format.NewRegistry(), store, settings, processor,
	)
	ownerID := uuid.New()
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "theme.bin",
		File: bytes.NewReader([]byte("theme")), Name: "Theme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	added, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaGallery,
		File: bytes.NewReader([]byte("encoded image")),
	})
	if err != nil {
		t.Fatalf("AddMedia: %v", err)
	}

	const requests = 8
	start := make(chan struct{})
	errors := make(chan error, requests)
	var ready sync.WaitGroup
	ready.Add(requests)
	for range requests {
		go func() {
			ready.Done()
			<-start
			_, err := svc.MediaVariant(context.Background(), added.ID, "grid", 1, nil)
			errors <- err
		}()
	}
	ready.Wait()
	close(start)
	<-processor.renderStarted
	close(processor.releaseRender)
	for range requests {
		if err := <-errors; err != nil {
			t.Fatalf("MediaVariant: %v", err)
		}
	}
	if calls := processor.renderCalls.Load(); calls != 1 {
		t.Fatalf("render calls = %d, want one shared cache job", calls)
	}
}

type blockingMediaProcessor struct {
	renderCalls   atomic.Int32
	renderStarted chan struct{}
	releaseRender chan struct{}
	startedOnce   sync.Once
}

func (p *blockingMediaProcessor) Prepare(context.Context, io.Reader) (mediaproc.Prepared, error) {
	return mediaproc.Prepared{Width: 20, Height: 10}, nil
}

func (p *blockingMediaProcessor) Render(
	context.Context,
	io.Reader,
	string,
) (mediaproc.Derivative, error) {
	p.renderCalls.Add(1)
	p.startedOnce.Do(func() { close(p.renderStarted) })
	<-p.releaseRender
	return mediaproc.Derivative{Variant: "grid", Bytes: []byte("rendered")}, nil
}

func (p *blockingMediaProcessor) ComposeSocialPreview(
	context.Context,
	io.Reader,
	string,
) (mediaproc.Derivative, error) {
	return mediaproc.Derivative{}, errors.New("unexpected social preview")
}

func (p *blockingMediaProcessor) DerivativeType() string { return "image/png" }

func testPNG(t *testing.T, width, height int, fill color.Color) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.Set(x, y, fill)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return encoded.Bytes()
}

// archiveWithImage builds the shape a module extracts media from: a card
// payload the registry can claim beside a picture the probe can locate.
func archiveWithImage(t *testing.T, picture []byte) []byte {
	t.Helper()
	var file bytes.Buffer
	archive := zip.NewWriter(&file)
	for _, entry := range []struct{ name, body string }{
		{name: "card.json", body: `{"spec":"chara_card_v3"}`},
		{name: "assets/icon/main.png", body: string(picture)},
	} {
		writer, err := archive.Create(entry.name)
		if err != nil {
			t.Fatalf("create archive entry: %v", err)
		}
		if _, err := io.WriteString(writer, entry.body); err != nil {
			t.Fatalf("write archive entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return file.Bytes()
}
