package asset

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
)

func TestSweepMarksThenDeletesOnlyBlobsWithoutLiveOrRecoverableReferences(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, format.NewRegistry(), store)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	ownerID := uuid.New()
	if _, err := pool.Exec(ctx, `insert into users (id, username) values ($1, 'sweep.owner')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}

	recoverable, err := service.Create(ctx, CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "recoverable.lumitheme",
		File: bytes.NewReader([]byte("recoverable")), Name: "Recoverable",
	})
	if err != nil {
		t.Fatalf("create recoverable asset: %v", err)
	}
	if err := service.Delete(ctx, ownerID, recoverable.ID); err != nil {
		t.Fatalf("delete recoverable asset: %v", err)
	}
	var recoverableBlob uuid.UUID
	if err := pool.QueryRow(ctx,
		`select blob_id from asset_revisions where asset_id = $1`, recoverable.ID,
	).Scan(&recoverableBlob); err != nil {
		t.Fatalf("read recoverable blob: %v", err)
	}

	orphan, err := store.Put(ctx, bytes.NewReader([]byte("worker crashed before recording a reference")))
	if err != nil {
		t.Fatalf("put orphan: %v", err)
	}
	rejected, err := service.AcceptIngest(ctx, IngestInput{
		OwnerID: ownerID, Filename: "rejected.bin", File: bytes.NewReader([]byte("rejected")),
	})
	if err != nil {
		t.Fatalf("accept rejected ingest: %v", err)
	}
	var rejectedBlob uuid.UUID
	if err := pool.QueryRow(ctx,
		`select blob_id from ingest_operations where id = $1`, rejected.ID,
	).Scan(&rejectedBlob); err != nil {
		t.Fatalf("read rejected blob: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		update ingest_operations
		   set status = 'failed', failure_reason = 'malformed_input', blob_id = null
		 where id = $1
	`, rejected.ID); err != nil {
		t.Fatalf("reject ingest: %v", err)
	}
	abandoned, err := service.AcceptIngest(ctx, IngestInput{
		OwnerID: ownerID, Filename: "unknown.bin", File: bytes.NewReader([]byte("abandoned")),
	})
	if err != nil {
		t.Fatalf("accept abandoned ingest: %v", err)
	}
	var abandonedBlob uuid.UUID
	if err := pool.QueryRow(ctx,
		`select blob_id from ingest_operations where id = $1`, abandoned.ID,
	).Scan(&abandonedBlob); err != nil {
		t.Fatalf("read abandoned blob: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		update ingest_operations
		   set status = 'needs_kind', expires_at = $2
		 where id = $1
	`, abandoned.ID, now); err != nil {
		t.Fatalf("abandon ingest: %v", err)
	}

	first, err := service.Sweep(ctx)
	if err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if first.Marked != 3 || first.Deleted != 0 {
		t.Fatalf("first sweep = %+v, want three marked and none deleted", first)
	}
	for _, id := range []uuid.UUID{recoverableBlob, orphan.ID, rejectedBlob, abandonedBlob} {
		opened, err := store.Open(ctx, id)
		if err != nil {
			t.Fatalf("blob %s disappeared on the mark pass: %v", id, err)
		}
		opened.Close()
	}

	now = now.Add(sweepDelay + time.Second)
	second, err := service.Sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if second.Deleted != 3 {
		t.Fatalf("second sweep = %+v, want three deleted", second)
	}
	if _, err := store.Open(ctx, recoverableBlob); err != nil {
		t.Fatalf("recoverable blob was swept: %v", err)
	}
	for _, id := range []uuid.UUID{orphan.ID, rejectedBlob, abandonedBlob} {
		if _, err := store.Open(ctx, id); !errors.Is(err, storage.ErrBlobNotFound) {
			t.Fatalf("collected blob %s error = %v, want ErrBlobNotFound", id, err)
		}
	}
}

func TestSweepRechecksReferencesImmediatelyBeforeDeleting(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, format.NewRegistry(), store)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	ownerID := uuid.New()
	if _, err := pool.Exec(ctx, `insert into users (id, username) values ($1, 'race.owner')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	stored, err := store.Put(ctx, bytes.NewReader([]byte("converged bytes")))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	if _, err := service.Sweep(ctx); err != nil {
		t.Fatalf("mark sweep: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into ingest_operations (id, owner_id, blob_id, filename, status)
		values ($1, $2, $3, 'converged.bin', 'pending')
	`, uuid.New(), ownerID, stored.ID); err != nil {
		t.Fatalf("add concurrent reference: %v", err)
	}

	now = now.Add(sweepDelay + time.Second)
	result, err := service.Sweep(ctx)
	if err != nil {
		t.Fatalf("delete sweep: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("delete sweep = %+v, want the new reference to cancel deletion", result)
	}
	opened, err := store.Open(ctx, stored.ID)
	if err != nil {
		t.Fatalf("newly referenced blob was swept: %v", err)
	}
	opened.Close()
}

func TestConcurrentConvergenceClearsAnOldSweepMark(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, format.NewRegistry(), store)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	stored, err := store.Put(ctx, bytes.NewReader([]byte("converging upload")))
	if err != nil {
		t.Fatalf("put orphan: %v", err)
	}
	if _, err := service.Sweep(ctx); err != nil {
		t.Fatalf("mark orphan: %v", err)
	}
	now = now.Add(sweepDelay + time.Second)

	converged, err := store.Put(ctx, bytes.NewReader([]byte("converging upload")))
	if err != nil {
		t.Fatalf("converge upload: %v", err)
	}
	if converged.ID != stored.ID {
		t.Fatalf("converged blob = %s, want %s", converged.ID, stored.ID)
	}
	result, err := service.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep after convergence: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("sweep after convergence = %+v, want no deletion", result)
	}
	opened, err := store.Open(ctx, stored.ID)
	if err != nil {
		t.Fatalf("converging upload lost its bytes: %v", err)
	}
	opened.Close()
}

func TestSweepCollectsAnAssetAfterItsRecoveryWindow(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	service := NewService(pool, format.NewRegistry(), store)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	ownerID := uuid.New()
	if _, err := pool.Exec(ctx, `insert into users (id, username) values ($1, 'expired.owner')`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	created, err := service.Create(ctx, CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: "expired.lumitheme",
		File: bytes.NewReader([]byte("expired source")), Name: "Expired",
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := service.Delete(ctx, ownerID, created.ID); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	var blobID uuid.UUID
	if err := pool.QueryRow(ctx,
		`select blob_id from asset_revisions where asset_id = $1`, created.ID,
	).Scan(&blobID); err != nil {
		t.Fatalf("read blob id: %v", err)
	}

	now = now.Add(recoveryWindow + time.Second)
	marked, err := service.Sweep(ctx)
	if err != nil || marked.Marked != 1 || marked.Deleted != 0 {
		t.Fatalf("mark expired asset = %+v, %v; want one mark", marked, err)
	}
	now = now.Add(sweepDelay + time.Second)
	deleted, err := service.Sweep(ctx)
	if err != nil || deleted.Deleted != 1 {
		t.Fatalf("collect expired asset = %+v, %v; want one deletion", deleted, err)
	}
	if _, err := store.Open(ctx, blobID); !errors.Is(err, storage.ErrBlobNotFound) {
		t.Fatalf("expired asset blob error = %v, want ErrBlobNotFound", err)
	}
	if err := service.Restore(ctx, ownerID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restore after recovery error = %v, want ErrNotFound", err)
	}
}
