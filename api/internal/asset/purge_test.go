package asset

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type failingDeleteStore struct {
	storage.Store
}

func (failingDeleteStore) Delete(context.Context, uuid.UUID) error {
	return errors.New("storage unavailable")
}

func TestPurgeCommitsTheTombstoneAndBrokenReferencesBeforeDeletingBytes(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, registryWithModule(t, opaqueTestModule{}), failingDeleteStore{Store: store})
	ownerID := uuid.New()
	actorID := uuid.New()
	if _, err := pool.Exec(ctx, `
		insert into users (id, username) values ($1, 'durable.owner'), ($2, 'durable.actor')
	`, ownerID, actorID); err != nil {
		t.Fatalf("insert accounts: %v", err)
	}
	created, err := service.Create(ctx, CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "durable.lumitheme",
		File: bytes.NewReader([]byte("durably purged bytes")), Name: "Durable purge",
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	var digestBytes []byte
	if err := pool.QueryRow(ctx, `
		select blob.sha256 from asset_revisions revision
		join blobs blob on blob.id = revision.blob_id
		where revision.asset_id = $1
	`, created.ID).Scan(&digestBytes); err != nil {
		t.Fatalf("read digest: %v", err)
	}
	var digest [32]byte
	copy(digest[:], digestBytes)

	if err := service.Purge(ctx, digest, "legal_order", actorID); err == nil {
		t.Fatal("purge succeeded despite the storage failure")
	}
	var tombstoned bool
	if err := pool.QueryRow(ctx,
		`select exists (select 1 from blob_tombstones where sha256 = $1)`, digest[:],
	).Scan(&tombstoned); err != nil {
		t.Fatalf("read tombstone: %v", err)
	}
	if !tombstoned {
		t.Fatal("purge failure rolled back the tombstone")
	}
	var references int
	if err := pool.QueryRow(ctx,
		`select count(*) from asset_revisions where asset_id = $1 and blob_id is not null`, created.ID,
	).Scan(&references); err != nil {
		t.Fatalf("count references: %v", err)
	}
	if references != 0 {
		t.Fatalf("purge failure left %d live references", references)
	}

	service.store = store
	if _, err := service.Sweep(ctx); err != nil {
		t.Fatalf("resume purge from the sweeper: %v", err)
	}
	var blobID uuid.UUID
	if err := pool.QueryRow(ctx,
		`select id from blobs where sha256 = $1`, digest[:],
	).Scan(&blobID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("interrupted purge blob lookup = %v, want no row", err)
	}
}

func TestPurgeAndIngestFinalizationSerializeOnTheDigest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, registryWithModule(t, opaqueTestModule{}), store)
	ownerID := uuid.New()
	actorID := uuid.New()
	if _, err := pool.Exec(ctx, `
		insert into users (id, username) values ($1, 'race.owner'), ($2, 'race.actor')
	`, ownerID, actorID); err != nil {
		t.Fatalf("insert accounts: %v", err)
	}
	operation, err := service.AcceptIngest(ctx, IngestInput{
		OwnerID: ownerID, Filename: "race.lumitheme", File: bytes.NewReader([]byte("racing bytes")),
	})
	if err != nil {
		t.Fatalf("accept ingest: %v", err)
	}
	job, ok, err := service.leaseNextIngest(ctx)
	if err != nil || !ok || job.ID != operation.ID {
		t.Fatalf("lease ingest = %+v, %v, %v", job, ok, err)
	}
	var digestBytes []byte
	if err := pool.QueryRow(ctx, `select sha256 from blobs where id = $1`, job.BlobID).Scan(&digestBytes); err != nil {
		t.Fatalf("read digest: %v", err)
	}
	var digest [32]byte
	copy(digest[:], digestBytes)
	prepared := preparedIngest{
		Kind: "character", Format: "unknown", Name: "Race", Tags: []string{},
		Discovery: DiscoveryListed, MediaType: "application/octet-stream",
	}
	prepared.Blocks, err = block.Place(prepared.Kind, nil)
	if err != nil {
		t.Fatalf("place fixture: %v", err)
	}

	start := make(chan struct{})
	finalized := make(chan error, 1)
	purged := make(chan error, 1)
	go func() {
		<-start
		finalized <- service.finalizeIngest(ctx, job, prepared)
	}()
	go func() {
		<-start
		purged <- service.Purge(ctx, digest, "legal_order", actorID)
	}()
	close(start)
	if err := <-purged; err != nil {
		t.Fatalf("purge: %v", err)
	}
	if err := <-finalized; err != nil && !errors.Is(err, errIngestLeaseLost) {
		t.Fatalf("finalize: %v", err)
	}

	var references int
	if err := pool.QueryRow(ctx, `
		select (select count(*) from asset_revisions where blob_id is not null)
		     + (select count(*) from ingest_operations where blob_id is not null)
	`).Scan(&references); err != nil {
		t.Fatalf("count surviving references: %v", err)
	}
	if references != 0 {
		t.Fatalf("surviving references = %d, want 0", references)
	}
	var tombstones int
	if err := pool.QueryRow(ctx,
		`select count(*) from blob_tombstones where sha256 = $1`, digest[:],
	).Scan(&tombstones); err != nil {
		t.Fatalf("count tombstones: %v", err)
	}
	if tombstones != 1 {
		t.Fatalf("tombstones = %d, want 1", tombstones)
	}
}

func TestPurgeDeletesSharedBytesBreaksReferencesAndRecordsATombstone(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, registryWithModule(t, opaqueTestModule{}), store)
	now := time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	ownerID := uuid.New()
	actorID := uuid.New()
	if _, err := pool.Exec(ctx, `
		insert into users (id, username) values ($1, 'purge.owner'), ($2, 'purge.actor')
	`, ownerID, actorID); err != nil {
		t.Fatalf("insert accounts: %v", err)
	}

	var blobID uuid.UUID
	var digest []byte
	for _, name := range []string{"First copy", "Second copy"} {
		created, err := service.Create(ctx, CreateInput{
			OwnerID: ownerID, Kind: "theme", Filename: name + ".lumitheme",
			File: bytes.NewReader([]byte("shared forbidden bytes")), Name: name,
		})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if err := pool.QueryRow(ctx, `
			select blob.id, blob.sha256
			  from asset_revisions revision
			  join blobs blob on blob.id = revision.blob_id
			 where revision.asset_id = $1
		`, created.ID).Scan(&blobID, &digest); err != nil {
			t.Fatalf("read %s blob: %v", name, err)
		}
	}
	var contentDigest [32]byte
	copy(contentDigest[:], digest)

	if err := service.Purge(ctx, contentDigest, "legal_order", actorID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, err := store.Open(ctx, blobID); !errors.Is(err, storage.ErrBlobNotFound) {
		t.Fatalf("purged blob error = %v, want ErrBlobNotFound", err)
	}
	var liveReferences int
	if err := pool.QueryRow(ctx,
		`select count(*) from asset_revisions where blob_id is not null`,
	).Scan(&liveReferences); err != nil {
		t.Fatalf("count revision references: %v", err)
	}
	if liveReferences != 0 {
		t.Fatalf("live revision references = %d, want 0", liveReferences)
	}
	var reason string
	var purgedAt time.Time
	var actor pgtype.UUID
	if err := pool.QueryRow(ctx, `
		select reason_code, purged_at, actor_id from blob_tombstones where sha256 = $1
	`, digest).Scan(&reason, &purgedAt, &actor); err != nil {
		t.Fatalf("read tombstone: %v", err)
	}
	if reason != "legal_order" || !purgedAt.Equal(now) || !actor.Valid || uuid.UUID(actor.Bytes) != actorID {
		t.Fatalf("tombstone = reason %q, at %v, actor %v", reason, purgedAt, actor)
	}
}

func TestPurgedBytesCannotBeStoredAgain(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, format.NewRegistry(), store)
	ownerID := uuid.New()
	actorID := uuid.New()
	if _, err := pool.Exec(ctx, `
		insert into users (id, username) values ($1, 'upload.owner'), ($2, 'upload.actor')
	`, ownerID, actorID); err != nil {
		t.Fatalf("insert accounts: %v", err)
	}
	stored, err := store.Put(ctx, bytes.NewReader([]byte("bytes that stay gone")))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	if err := service.Purge(ctx, stored.Digest, "illegal_content", actorID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	_, err = service.AcceptIngest(ctx, IngestInput{
		OwnerID: ownerID, Filename: "same.bin", File: bytes.NewReader([]byte("bytes that stay gone")),
	})
	if !errors.Is(err, storage.ErrTombstoned) {
		t.Fatalf("re-upload error = %v, want ErrTombstoned", err)
	}
}
