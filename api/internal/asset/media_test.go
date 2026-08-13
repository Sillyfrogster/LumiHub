package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	mediaproc "github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
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
	if added.AssetID == nil || *added.AssetID != created.ID || added.RevisionID != nil {
		t.Fatalf("media provenance = asset %v revision %v, want asset %s only",
			added.AssetID, added.RevisionID, created.ID)
	}
	if added.Width != 1200 || added.Height != 600 {
		t.Fatalf("dimensions = %dx%d, want native 1200x600", added.Width, added.Height)
	}
	if added.DerivativeVersion != mediaproc.DerivativeVersion {
		t.Fatalf("derivative version = %d, want %d", added.DerivativeVersion, mediaproc.DerivativeVersion)
	}

	var blobID uuid.UUID
	var digestBytes []byte
	var storedAsset, storedRevision *uuid.UUID
	err = pool.QueryRow(context.Background(), `
		select media.blob_id, blob.sha256, media.asset_id, media.revision_id
		  from asset_media media
		  join blobs blob on blob.id = media.blob_id
		 where media.id = $1
	`, added.ID).Scan(&blobID, &digestBytes, &storedAsset, &storedRevision)
	if err != nil {
		t.Fatalf("read media row: %v", err)
	}
	if storedAsset == nil || *storedAsset != created.ID || storedRevision != nil {
		t.Fatalf("stored provenance = asset %v revision %v", storedAsset, storedRevision)
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
		context.Background(), added.ID, "grid", mediaproc.DerivativeVersion,
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
		{variant: "og", version: mediaproc.DerivativeVersion},
		{variant: "grid", version: mediaproc.DerivativeVersion + 1},
	} {
		_, err := svc.MediaVariant(context.Background(), added.ID, request.variant, request.version)
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

func TestIngestStoresExtractedMediaAtRevisionScope(t *testing.T) {
	extracted := testPNG(t, 90, 45, color.White)
	registry := registryWithModule(t, recognizedModule{parsed: format.Parsed{
		Kind: "character", Format: "recognized",
		Media: []format.Media{{Role: "expression", Bytes: extracted}},
	}})
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'media.extractor')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	operation, err := svc.AcceptIngest(context.Background(), IngestInput{
		OwnerID: ownerID, Filename: "card.json", File: bytes.NewReader([]byte("{}")),
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

	var assetID, revisionID *uuid.UUID
	var role string
	var width, height int
	err = pool.QueryRow(context.Background(), `
		select asset_id, revision_id, role, width, height
		  from asset_media
		 where revision_id = $1
	`, operation.Asset.CurrentRevisionID).Scan(&assetID, &revisionID, &role, &width, &height)
	if err != nil {
		t.Fatalf("read extracted media: %v", err)
	}
	if assetID != nil || revisionID == nil || *revisionID != operation.Asset.CurrentRevisionID {
		t.Fatalf("extracted provenance = asset %v revision %v", assetID, revisionID)
	}
	if role != "expression" || width != 90 || height != 45 {
		t.Fatalf("extracted media = %s %dx%d", role, width, height)
	}
}

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
