package asset

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

func revisionOwner(t *testing.T, svc *Service, handle string) uuid.UUID {
	t.Helper()
	ownerID := uuid.New()
	if _, err := svc.pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, $2)`, ownerID, handle); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	return ownerID
}

func ingestOne(t *testing.T, svc *Service, ownerID uuid.UUID, filename string, file []byte) Asset {
	t.Helper()
	operation, err := svc.AcceptIngest(context.Background(), IngestInput{
		OwnerID: ownerID, Filename: filename, File: bytes.NewReader(file),
	})
	if err != nil {
		t.Fatalf("AcceptIngest: %v", err)
	}
	if processed, err := svc.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("ProcessNextIngest = %v, %v; want true, nil", processed, err)
	}
	operation, err = svc.GetIngest(context.Background(), ownerID, operation.ID)
	if err != nil {
		t.Fatalf("GetIngest: %v", err)
	}
	if operation.Asset == nil {
		t.Fatalf("ingest did not create an asset: %+v", operation)
	}
	return *operation.Asset
}

func publishImported(t *testing.T, svc *Service, ownerID uuid.UUID, created Asset) {
	t.Helper()
	name := created.Name
	if name == "" {
		name = "Test asset"
	}
	nsfw := false
	if err := svc.SetIdentity(context.Background(), Identity{
		OwnerID: ownerID, AssetID: created.ID, Name: name, IsNSFW: &nsfw,
	}); err != nil {
		t.Fatalf("SetIdentity imported asset: %v", err)
	}
	if _, err := svc.Publish(context.Background(), ownerID, created.ID); err != nil {
		t.Fatalf("Publish imported asset: %v", err)
	}
}

func addRevision(
	t *testing.T,
	svc *Service,
	ownerID, assetID uuid.UUID,
	filename string,
	file []byte,
) IngestOperation {
	t.Helper()
	operation, err := svc.AcceptRevision(context.Background(), RevisionInput{
		OwnerID: ownerID, AssetID: assetID, Filename: filename, File: bytes.NewReader(file),
	})
	if err != nil {
		t.Fatalf("AcceptRevision: %v", err)
	}
	if processed, err := svc.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("ProcessNextIngest = %v, %v; want true, nil", processed, err)
	}
	got, err := svc.GetIngest(context.Background(), ownerID, operation.ID)
	if err != nil {
		t.Fatalf("GetIngest: %v", err)
	}
	return got
}

func TestANewRevisionReplacesTheCurrentBytesAndKeepsTheCatalogEntry(t *testing.T) {
	registry := registryWithModule(t, recognizedModule{parsed: format.Parsed{
		Kind: "character", Format: "recognized", Header: format.Header{Name: "Seeded", Blurb: "Seeded blurb"},
		Elements: []block.Element{
			{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: "Description"}},
			{Type: block.TypeTextSet, Role: block.RoleGreetings, Content: block.TextSet{Texts: []block.TextItem{{ID: block.NewItemID(), Text: "Hello"}}}},
		},
	}})
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "revision.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{"spec":"x","take":1}`))
	publishImported(t, svc, ownerID, created)

	operation := addRevision(t, svc, ownerID, created.ID, "card.json", []byte(`{"spec":"x","take":2}`))
	if operation.Status != IngestSuccess || operation.Asset == nil {
		t.Fatalf("revision operation = %+v, want success", operation)
	}
	if operation.Asset.ID != created.ID {
		t.Fatalf("revision made asset %s, want %s", operation.Asset.ID, created.ID)
	}
	if operation.Asset.CurrentRevisionID == created.CurrentRevisionID {
		t.Fatal("the asset still points at its first revision")
	}
	if operation.Asset.Name != created.Name || operation.Asset.Blurb != created.Blurb {
		t.Fatalf("catalog metadata was re-seeded: %+v", operation.Asset)
	}

	var revisions int
	if err := pool.QueryRow(context.Background(),
		`select count(*) from asset_revisions where asset_id = $1`, created.ID).Scan(&revisions); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revisions != 2 {
		t.Fatalf("revision count = %d, want 2", revisions)
	}

	source, err := svc.OpenSource(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("OpenSource: %v", err)
	}
	defer source.Close()
	var served bytes.Buffer
	if _, err := served.ReadFrom(source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if served.String() != `{"spec":"x","take":2}` {
		t.Fatalf("source = %s, want the new revision's bytes", served.String())
	}
}

func TestARevisionResolvingToADifferentKindIsRejected(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range []format.Module{
		kindModule{id: "as_character", kind: "character"},
		kindModule{id: "as_lorebook", kind: "lorebook"},
	} {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "kind.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{"spec":"as_character"}`))

	operation := addRevision(t, svc, ownerID, created.ID, "book.json", []byte(`{"spec":"as_lorebook"}`))
	if operation.Status != IngestFailed {
		t.Fatalf("revision status = %s, want failed", operation.Status)
	}
	if operation.Failure == nil || operation.Failure.Reason != "wrong_kind" {
		t.Fatalf("revision failure = %+v, want wrong_kind", operation.Failure)
	}

	var kind string
	var currentRevisionID uuid.UUID
	var revisions int
	err := pool.QueryRow(context.Background(), `
		select kind, current_revision_id,
		       (select count(*) from asset_revisions where asset_id = assets.id)
		  from assets where id = $1
	`, created.ID).Scan(&kind, &currentRevisionID, &revisions)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if kind != "character" || currentRevisionID != created.CurrentRevisionID || revisions != 1 {
		t.Fatalf("asset changed: kind %s, current %s, revisions %d", kind, currentRevisionID, revisions)
	}
}

func TestAReplacementFileBecomesTheAssetsOrigin(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "replacement.origin.owner")
	created := ingestOne(t, svc, ownerID, "card-v2.json", []byte(`{
		"spec":"chara_card_v2","spec_version":"2.0",
		"data":{"name":"Ana","description":"Before","first_mes":"Hello","character_version":"v2"}
	}`))
	operation := addRevision(t, svc, ownerID, created.ID, "card-v3.json", []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"After","first_mes":"Hello","character_version":"v3"}
	}`))
	if operation.Status != IngestSuccess {
		t.Fatalf("replacement = %+v, want success", operation)
	}
	var origin, version string
	if err := pool.QueryRow(context.Background(), `
		select origin_format, asset_version from assets where id = $1
	`, created.ID).Scan(&origin, &version); err != nil {
		t.Fatalf("read origin: %v", err)
	}
	if origin != character.V3 || version != "v3" {
		t.Fatalf("origin and header = %q, %q; want %q, v3", origin, version, character.V3)
	}
}

func TestAnUnrecognisedRevisionIsRefusedWithoutChangingTheAsset(t *testing.T) {
	registry := registryWithModule(t, kindModule{id: "as_character", kind: "character"})
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "unsupported.revision.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{"spec":"as_character"}`))

	operation := addRevision(t, svc, ownerID, created.ID, "mystery.bin", []byte("nothing claims this"))
	if operation.Status != IngestFailed || operation.Asset != nil {
		t.Fatalf("revision operation = %+v, want failed without an asset", operation)
	}
	if operation.Failure == nil || operation.Failure.Reason != string(format.FailureUnsupportedFormat) {
		t.Fatalf("revision failure = %+v, want unsupported_format", operation.Failure)
	}

	var currentRevisionID uuid.UUID
	var revisions int
	err := pool.QueryRow(context.Background(), `
		select current_revision_id,
		       (select count(*) from asset_revisions where asset_id = assets.id)
		  from assets where id = $1
	`, created.ID).Scan(&currentRevisionID, &revisions)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if currentRevisionID != created.CurrentRevisionID || revisions != 1 {
		t.Fatalf("asset changed: current %s, revisions %d", currentRevisionID, revisions)
	}
}

func TestOnlyTheOwnerOfALiveAssetCanAddARevision(t *testing.T) {
	registry := registryWithModule(t, kindModule{id: "as_character", kind: "character"})
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "guard.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{"spec":"as_character"}`))

	_, err := svc.AcceptRevision(context.Background(), RevisionInput{
		OwnerID: uuid.New(), AssetID: created.ID, Filename: "card.json",
		File: bytes.NewReader([]byte(`{"spec":"as_character"}`)),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger revision error = %v, want ErrNotFound", err)
	}

	if _, err := pool.Exec(context.Background(), `
		update assets set withheld_at = now(), withheld_by = $2, withheld_reason = 'held'
		 where id = $1
	`, created.ID, ownerID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}
	_, err = svc.AcceptRevision(context.Background(), RevisionInput{
		OwnerID: ownerID, AssetID: created.ID, Filename: "card.json",
		File: bytes.NewReader([]byte(`{"spec":"as_character"}`)),
	})
	if !errors.Is(err, ErrAssetFrozen) {
		t.Fatalf("withheld revision error = %v, want ErrAssetFrozen", err)
	}
}

func TestReimportedMediaFillsTheAsset(t *testing.T) {
	registry := registryWithModule(t, recognizedModule{parsed: format.Parsed{
		Kind: "character", Format: "recognized",
		Media: []format.Media{{Role: MediaAvatar, ImageID: 0}},
	}})
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "scoped.owner")
	first := archiveWithImage(t, testPNG(t, 40, 20, color.White))
	created := ingestOne(t, svc, ownerID, "card.charx", first)
	added, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaGallery,
		File: bytes.NewReader(testPNG(t, 50, 25, color.Gray{Y: 128})),
	})
	if err != nil {
		t.Fatalf("add creator media: %v", err)
	}

	second := archiveWithImage(t, testPNG(t, 60, 30, color.Black))
	operation := addRevision(t, svc, ownerID, created.ID, "card.charx", second)
	if operation.Status != IngestSuccess || operation.Asset == nil {
		t.Fatalf("revision operation = %+v, want success", operation)
	}
	media, err := svc.ListMedia(context.Background(), created.ID, &ownerID)
	if err != nil {
		t.Fatalf("list reimported media: %v", err)
	}
	if len(media) != 2 {
		t.Fatalf("media rows = %d, want creator media and the reimported image", len(media))
	}
	foundCreatorMedia := false
	for _, image := range media {
		if image.AssetID != created.ID {
			t.Fatalf("reimported media belongs to %s, want %s", image.AssetID, created.ID)
		}
		foundCreatorMedia = foundCreatorMedia || image.ID == added.ID
	}
	if !foundCreatorMedia {
		t.Fatalf("creator media %s disappeared on reimport", added.ID)
	}

	var previewAsset uuid.UUID
	var width int
	err = pool.QueryRow(context.Background(), `
		select media.asset_id, media.width
		  from assets asset
		  join asset_media media on media.id = asset.cover_media_id
		 where asset.id = $1
	`, created.ID).Scan(&previewAsset, &width)
	if err != nil {
		t.Fatalf("read cover media: %v", err)
	}
	if previewAsset != created.ID {
		t.Fatalf("cover belongs to asset %s, want %s", previewAsset, created.ID)
	}
	if width != 60 {
		t.Fatalf("cover media is %d wide, want the reimported picture", width)
	}
}

/** A module that claims a payload naming its own id and declares one kind */
type kindModule struct {
	id   string
	kind string
}

func (m kindModule) ID() string { return m.id }
func (m kindModule) Declaration() format.Declaration {
	return testReaderDeclaration(m.id, m.kind)
}

func (m kindModule) Claim(file probe.Inspection) (format.Claim, bool) {
	for _, payload := range file.Payloads {
		if spec, _ := payload.String("spec"); spec == m.id {
			return format.AuthoritativeClaim(payload, "spec")
		}
	}
	return format.Claim{}, false
}

func (m kindModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{Kind: m.kind, Format: m.id}, nil
}
