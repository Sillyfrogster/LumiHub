package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
)

type leasedModule struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func TestImportPayloadLimitNamesTheLimitAndActualBytes(t *testing.T) {
	pool := testdb.Connect(t)
	ownerID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'payload.limit.owner')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	blobs, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	service := NewService(pool, registry, blobs)
	payload, err := json.Marshal(map[string]any{
		"spec": "chara_card_v3", "spec_version": "3.0",
		"data": map[string]any{
			"description": strings.Repeat("x", block.MaxPayloadBytes), "first_mes": "Hello",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	operation, err := service.AcceptIngest(context.Background(), IngestInput{
		OwnerID: ownerID, Filename: "too-large.json", File: bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("AcceptIngest: %v", err)
	}
	if processed, err := service.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("ProcessNextIngest = %v, %v", processed, err)
	}
	got, err := service.GetIngest(context.Background(), ownerID, operation.ID)
	if err != nil {
		t.Fatalf("GetIngest: %v", err)
	}
	if got.Failure == nil || got.Failure.Reason != string(format.FailureLimitExceeded) ||
		!strings.Contains(got.Failure.Message, strconv.Itoa(block.MaxPayloadBytes)) ||
		!strings.Contains(got.Failure.Message, strconv.Itoa(len(payload))) {
		t.Fatalf("failure = %+v, want limit %d and actual %d", got.Failure, block.MaxPayloadBytes, len(payload))
	}
	var assets int
	if err := pool.QueryRow(context.Background(), `select count(*) from assets`).Scan(&assets); err != nil || assets != 0 {
		t.Fatalf("assets = %d, %v; over-limit import must store none", assets, err)
	}
}

func TestUnrecognisedImportFailsTerminallyAndReleasesItsBlobReference(t *testing.T) {
	pool := testdb.Connect(t)
	ownerID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'expiry.owner')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	blobs, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, format.NewRegistry(), blobs)
	operation, err := service.AcceptIngest(context.Background(), IngestInput{
		OwnerID: ownerID, Filename: "mystery.bundle", File: bytes.NewReader([]byte("mystery")),
	})
	if err != nil {
		t.Fatalf("accept ingest: %v", err)
	}
	if processed, err := service.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}

	got, err := service.GetIngest(context.Background(), ownerID, operation.ID)
	if err != nil {
		t.Fatalf("GetIngest: %v", err)
	}
	if got.Status != IngestFailed || got.Failure == nil ||
		got.Failure.Reason != string(format.FailureUnsupportedFormat) {
		t.Fatalf("operation = %+v, want terminal unsupported_format", got)
	}
	var blobReferences int
	if err := pool.QueryRow(context.Background(), `
		select count(*) from ingest_operations where id = $1 and blob_id is not null
	`, operation.ID).Scan(&blobReferences); err != nil {
		t.Fatalf("count references: %v", err)
	}
	if blobReferences != 0 {
		t.Fatalf("failed ingest blob references = %d, want 0", blobReferences)
	}
}

func (*leasedModule) ID() string { return "leased" }
func (*leasedModule) Declaration() format.Declaration {
	return testReaderDeclaration("leased", "character")
}
func (*leasedModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (m *leasedModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	if m.calls.Add(1) == 1 {
		close(m.started)
		<-m.release
	}
	return format.Parsed{Kind: "character", Format: "leased"}, nil
}

func TestExpiredLeaseIsReclaimedAndFinalizationIsIdempotent(t *testing.T) {
	pool := testdb.Connect(t)
	ownerID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'lease.owner')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	blobs, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	module := &leasedModule{started: make(chan struct{}), release: make(chan struct{})}
	registry := format.NewRegistry()
	if err := registry.Register(module); err != nil {
		t.Fatalf("register module: %v", err)
	}
	settings := DefaultIngestSettings()
	settings.LeaseDuration = time.Minute
	service := NewServiceWithIngestSettings(pool, registry, blobs, settings)
	name := "Leased card"
	_, err = service.AcceptIngest(context.Background(), IngestInput{
		OwnerID: ownerID, Filename: "leased.json", File: bytes.NewReader([]byte(`{"value":true}`)),
		Name: &name, Discovery: DiscoveryListed,
	})
	if err != nil {
		t.Fatalf("accept ingest: %v", err)
	}
	var clock atomic.Value
	clock.Store(time.Now().Add(time.Second))
	service.now = func() time.Time { return clock.Load().(time.Time) }

	firstDone := make(chan error, 1)
	go func() {
		_, processErr := service.ProcessNextIngest(context.Background())
		firstDone <- processErr
	}()
	<-module.started

	clock.Store(clock.Load().(time.Time).Add(2 * time.Minute))
	if processed, err := service.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("reclaimed process = %v, %v; want true, nil", processed, err)
	}
	close(module.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("stale worker finalization: %v", err)
	}

	for _, table := range []string{"assets", "asset_revisions"} {
		var count int
		if err := pool.QueryRow(context.Background(), "select count(*) from "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("%s count = %d, want 1", table, count)
		}
	}
	var status string
	if err := pool.QueryRow(context.Background(),
		`select status from ingest_operations`).Scan(&status); err != nil {
		t.Fatalf("read operation: %v", err)
	}
	if status != "success" {
		t.Errorf("operation status = %q, want success", status)
	}
}
