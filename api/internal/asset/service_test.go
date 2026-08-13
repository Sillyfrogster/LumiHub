package asset

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()
	return newTestServiceWithRegistry(t, format.NewRegistry())
}

type claimsFirstPayload struct{}

func (claimsFirstPayload) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}

func registryWithModule(t *testing.T, module format.Module) *format.Registry {
	t.Helper()
	registry := format.NewRegistry()
	if err := registry.Register(module); err != nil {
		t.Fatalf("register module: %v", err)
	}
	return registry
}

func newTestServiceWithRegistry(t *testing.T, registry *format.Registry) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return NewService(pool, registry, blob), pool
}

func TestCreateStoresUploaderMetadataForAnUnparseableFile(t *testing.T) {
	svc, pool := newTestService(t)

	got, err := svc.Create(context.Background(), CreateInput{
		OwnerID:   uuid.New(),
		Kind:      "character",
		Filename:  "mystery.bin",
		File:      bytes.NewReader([]byte{0x00, 0xff, 0x10}),
		Name:      "Uploader chose this name",
		Blurb:     "A short catalog pitch",
		Tags:      []string{"fantasy"},
		Discovery: "listed",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.Format != "unknown" {
		t.Errorf("Format = %q, want unknown", got.Format)
	}
	if got.Name != "Uploader chose this name" {
		t.Errorf("Name = %q, the uploader's value must win", got.Name)
	}
	if got.CurrentRevisionID == uuid.Nil {
		t.Error("CurrentRevisionID must be set, callers never compute MAX(revision)")
	}

	var name, blurb, format string
	var tags []string
	var currentRevision uuid.UUID
	err = pool.QueryRow(context.Background(),
		`select asset.name, asset.blurb, revision.format, asset.tags, asset.current_revision_id
		   from assets asset
		   join asset_revisions revision on revision.id = asset.current_revision_id
		  where asset.id = $1`,
		got.ID).Scan(&name, &blurb, &format, &tags, &currentRevision)
	if err != nil {
		t.Fatalf("asset row was not written: %v", err)
	}
	if name != "Uploader chose this name" {
		t.Errorf("stored name = %q, the uploader's value must win", name)
	}
	if blurb != "A short catalog pitch" {
		t.Errorf("stored blurb = %q", blurb)
	}
	if format != "unknown" {
		t.Errorf("stored format = %q, want unknown", format)
	}
	if len(tags) != 1 || tags[0] != "fantasy" {
		t.Errorf("stored tags = %v, want [fantasy]", tags)
	}
	if currentRevision != got.CurrentRevisionID {
		t.Errorf("stored current_revision_id = %v, want %v", currentRevision, got.CurrentRevisionID)
	}

	var revision int
	var byteSize int64
	err = pool.QueryRow(context.Background(),
		`select r.revision, b.byte_size
		   from asset_revisions r
		   join blobs b on b.id = r.blob_id
		  where r.id = $1`,
		got.CurrentRevisionID).Scan(&revision, &byteSize)
	if err != nil {
		t.Fatalf("revision row was not written: %v", err)
	}
	if revision != 1 || byteSize != 3 {
		t.Errorf("revision = %d, byte_size = %d, want 1 and 3", revision, byteSize)
	}
}

func TestCreateWritesNothingWhenParsingFails(t *testing.T) {
	pool := testdb.Connect(t)
	blob, _ := storage.NewStore(pool, t.TempDir())
	reg := registryWithModule(t, failingModule{})
	svc := NewService(pool, reg, blob)

	_, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "a.bin",
		File: bytes.NewReader([]byte("{}")), Name: "A", Discovery: "listed",
	})
	if err == nil {
		t.Fatal("expected Create to fail when parsing fails")
	}

	var count int
	if err := pool.QueryRow(context.Background(), `select count(*) from assets`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("found %d asset rows, a failed parse must leave nothing behind", count)
	}
}

type recognizedModule struct {
	claimsFirstPayload
	parsed format.Parsed
}

func (recognizedModule) ID() string { return "recognized" }
func (m recognizedModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return m.parsed, nil
}

func serviceWithRecognizedModule(t *testing.T, parsed format.Parsed) *Service {
	t.Helper()
	registry := registryWithModule(t, recognizedModule{parsed: parsed})
	service, _ := newTestServiceWithRegistry(t, registry)
	return service
}

func TestCreateDerivesKindFromARecognizedFormat(t *testing.T) {
	svc := serviceWithRecognizedModule(t, format.Parsed{Kind: "character", Format: "recognized"})

	got, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "theme", Filename: "card.bin",
		File: bytes.NewReader([]byte("{}")), Name: "Card",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Kind != "character" {
		t.Errorf("kind = %q, want the module's character kind", got.Kind)
	}
}

func TestRecognizedFormatCannotCarryAPassthroughPlatform(t *testing.T) {
	platform := "lumiverse"
	svc := serviceWithRecognizedModule(t, format.Parsed{
		Kind: "character", Format: "recognized", PassthroughPlatform: &platform,
	})

	_, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "card.bin",
		File: bytes.NewReader([]byte("{}")), Name: "Card",
	})
	if err == nil {
		t.Fatal("recognized format stored a passthrough platform label")
	}
}

/** Always fails, so the test can prove nothing is left behind */
type failingModule struct{ claimsFirstPayload }

func (failingModule) ID() string { return "failing" }
func (failingModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{}, errors.New("cannot parse")
}

/** Emits a facet Postgres cannot store, so the failure lands mid transaction */
type badFacetModule struct{ claimsFirstPayload }

func (badFacetModule) ID() string { return "badfacet" }
func (badFacetModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{
		Kind:   "character",
		Format: "test",
		Facets: []format.Facet{{Key: "bad", Value: "\x00"}},
	}, nil
}

func TestCreateRollsBackAfterRowsAreWritten(t *testing.T) {
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := NewService(pool, registryWithModule(t, badFacetModule{}), blob)

	_, err = svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "a.bin",
		File: bytes.NewReader([]byte("{}")), Name: "A", Discovery: "listed",
	})
	if err == nil {
		t.Fatal("expected Create to fail when a facet cannot be stored")
	}

	for _, table := range []string{"assets", "asset_revisions"} {
		var count int
		if err := pool.QueryRow(context.Background(),
			"select count(*) from "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s has %d rows after a mid transaction failure, want 0", table, count)
		}
	}
}

// datedModule stands in for a module that reads a date out of a file.
type datedModule struct {
	claimsFirstPayload
	made time.Time
}

func (datedModule) ID() string { return "dated" }
func (m datedModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{Kind: "character", Format: "dated", CreatedAt: &m.made}, nil
}

func madeAndIndexed(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) (made, indexed time.Time) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`select created_at, indexed_at from assets where id = $1`, id).Scan(&made, &indexed)
	if err != nil {
		t.Fatalf("read dates: %v", err)
	}
	return made, indexed
}

func TestCreateKeepsTheDateTheFileCarries(t *testing.T) {
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	fileDate := time.Date(2019, 3, 14, 9, 30, 0, 0, time.UTC)
	svc := NewService(pool, registryWithModule(t, datedModule{made: fileDate}), blob)

	got, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "a.bin",
		File: bytes.NewReader([]byte("{}")), Name: "A", Discovery: "listed",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !got.CreatedAt.Equal(fileDate) {
		t.Errorf("returned made date = %v, want the file's %v", got.CreatedAt, fileDate)
	}

	made, indexed := madeAndIndexed(t, pool, got.ID)
	if !made.Equal(fileDate) {
		t.Errorf("stored made date = %v, want the file's %v", made, fileDate)
	}
	if time.Since(indexed) > time.Minute {
		t.Errorf("indexed date = %v, want our clock, not the file's date", indexed)
	}
}

func TestCreateFallsBackToWhenTheRowWasWritten(t *testing.T) {
	svc, pool := newTestService(t)

	got := create(t, svc, "no-date", "character", "listed")

	made, indexed := madeAndIndexed(t, pool, got.ID)
	if !made.Equal(indexed) {
		t.Errorf("made date %v and indexed date %v differ, a file with no date falls back", made, indexed)
	}
	if !got.CreatedAt.Equal(made) {
		t.Errorf("returned made date = %v, stored %v", got.CreatedAt, made)
	}
	if time.Since(indexed) > time.Minute {
		t.Errorf("indexed date = %v, want our clock", indexed)
	}
}

func TestCreateAcceptsAMadeDateInThePast(t *testing.T) {
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	fileDate := time.Date(2019, 3, 14, 9, 30, 0, 0, time.UTC)
	svc := NewService(pool, registryWithModule(t, datedModule{made: fileDate}), blob)

	caller := time.Date(2021, 7, 1, 12, 0, 0, 0, time.UTC)
	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "a.bin",
		File: bytes.NewReader([]byte("{}")), Name: "A", Discovery: "listed",
		CreatedAt: &caller,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	listed, err := svc.List(context.Background(), ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d assets, want 1", len(listed))
	}
	if !listed[0].CreatedAt.Equal(caller) {
		t.Errorf("read back made date = %v, want the caller's %v", listed[0].CreatedAt, caller)
	}

	_, indexed := madeAndIndexed(t, pool, created.ID)
	if time.Since(indexed) > time.Minute {
		t.Errorf("indexed date = %v, want our clock", indexed)
	}
}

func TestOpenSourceTellsMissingAssetFromBrokenStorage(t *testing.T) {
	pool := testdb.Connect(t)
	root := t.TempDir()
	blob, err := storage.NewStore(pool, root)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := NewService(pool, format.NewRegistry(), blob)

	created, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "a.bin",
		File: bytes.NewReader([]byte("x")), Name: "A", Discovery: "listed",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.OpenSource(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown asset gave %v, want ErrNotFound", err)
	}

	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove storage root: %v", err)
	}

	_, err = svc.OpenSource(context.Background(), created.ID)
	if err == nil {
		t.Fatal("expected an error when the stored file is gone")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("a missing file must not be reported as a missing asset")
	}
}
