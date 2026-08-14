package asset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/postgres"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const sweepDelay = 24 * time.Hour
const sweepInterval = time.Hour

type SweepResult struct {
	Marked  int64
	Deleted int
}

func (s *Service) Sweep(ctx context.Context) (SweepResult, error) {
	now := s.now()
	if _, err := s.store.RecordOrphans(ctx); err != nil {
		return SweepResult{}, fmt.Errorf("record filesystem orphans: %w", err)
	}
	if err := s.resumePurges(ctx); err != nil {
		return SweepResult{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		delete from ingest_operations
		 where status = 'needs_kind' and expires_at <= $1
	`, now); err != nil {
		return SweepResult{}, fmt.Errorf("expire abandoned ingests: %w", err)
	}

	marked, err := s.pool.Exec(ctx, `
		insert into blob_sweep_marks (blob_id, marked_at)
		select blob.id, $1
		  from blobs blob
		 where not exists (select 1 from blob_sweep_marks mark where mark.blob_id = blob.id)
		   and not exists (
		       select 1
		         from ingest_operations operation
		        where operation.blob_id = blob.id
		          and (operation.status in ('pending', 'processing')
		               or (operation.status = 'needs_kind' and operation.expires_at > $1))
		   )
		   and not exists (
		       select 1
		         from asset_revisions revision
		         join assets asset on asset.id = revision.asset_id
		        where revision.blob_id = blob.id
		          and (asset.deleted_at is null or asset.recoverable_until > $1)
		   )
		   and not exists (
		       select 1
		         from asset_media media
		         join assets asset on asset.id = media.asset_id
		        where media.blob_id = blob.id
		          and (asset.deleted_at is null or asset.recoverable_until > $1)
		   )
		   and not exists (
		       select 1
		         from asset_media media
		         join asset_revisions revision on revision.id = media.revision_id
		         join assets asset on asset.id = revision.asset_id
		        where media.blob_id = blob.id
		          and (asset.deleted_at is null or asset.recoverable_until > $1)
		   )
	`, now)
	if err != nil {
		return SweepResult{}, fmt.Errorf("mark unreferenced blobs: %w", err)
	}
	result := SweepResult{Marked: marked.RowsAffected()}

	rows, err := s.pool.Query(ctx, `
		select blob_id from blob_sweep_marks where marked_at <= $1 order by marked_at, blob_id
	`, now.Add(-sweepDelay))
	if err != nil {
		return SweepResult{}, fmt.Errorf("list marked blobs: %w", err)
	}
	var candidates []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return SweepResult{}, fmt.Errorf("read marked blob: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SweepResult{}, fmt.Errorf("list marked blobs: %w", err)
	}
	rows.Close()

	for _, id := range candidates {
		deleted, err := s.deleteMarkedBlob(ctx, id, now)
		if err != nil {
			return result, err
		}
		if deleted {
			result.Deleted++
		}
	}
	return result, nil
}

func (s *Service) resumePurges(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		select blob.id, blob.sha256
		  from blobs blob
		  join blob_tombstones tombstone on tombstone.sha256 = blob.sha256
		 order by tombstone.purged_at, blob.id
	`)
	if err != nil {
		return fmt.Errorf("list interrupted purges: %w", err)
	}
	type interruptedPurge struct {
		blobID uuid.UUID
		digest [32]byte
	}
	var interrupted []interruptedPurge
	for rows.Next() {
		var purge interruptedPurge
		var digestBytes []byte
		if err := rows.Scan(&purge.blobID, &digestBytes); err != nil {
			rows.Close()
			return fmt.Errorf("read interrupted purge: %w", err)
		}
		copy(purge.digest[:], digestBytes)
		interrupted = append(interrupted, purge)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("list interrupted purges: %w", err)
	}
	rows.Close()
	for _, purge := range interrupted {
		if err := s.deletePurgedBlob(ctx, purge.blobID, purge.digest); err != nil {
			return fmt.Errorf("resume interrupted purge: %w", err)
		}
	}
	return nil
}

func (s *Service) RunSweeper(ctx context.Context, report func(error)) {
	s.runSweeper(ctx, sweepInterval, report)
}

func (s *Service) runSweeper(ctx context.Context, interval time.Duration, report func(error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.Sweep(ctx); err != nil && !errors.Is(err, context.Canceled) && report != nil {
			report(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) deleteMarkedBlob(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	ready, err := s.prepareMarkedBlob(ctx, id, now)
	if err != nil || !ready {
		return false, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin blob sweep: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockBlobDigest(ctx, tx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	var marked pgtype.Timestamptz
	if err := tx.QueryRow(ctx,
		`select marked_at from blob_sweep_marks where blob_id = $1 for update`, id,
	).Scan(&marked); errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("lock marked blob: %w", err)
	}
	if !marked.Valid || marked.Time.After(now.Add(-sweepDelay)) {
		return false, nil
	}

	referenced, err := blobHasLiveReference(ctx, tx, id, now)
	if err != nil {
		return false, err
	}
	if referenced {
		if _, err := tx.Exec(ctx, `delete from blob_sweep_marks where blob_id = $1`, id); err != nil {
			return false, fmt.Errorf("clear blob sweep mark: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit cancelled blob sweep: %w", err)
		}
		return false, nil
	}

	var digestBytes []byte
	if err := tx.QueryRow(ctx, `select sha256 from blobs where id = $1`, id).Scan(&digestBytes); err != nil {
		return false, fmt.Errorf("read swept blob digest: %w", err)
	}
	var digest [32]byte
	copy(digest[:], digestBytes)
	if err := postgres.LockBlobDeletionAgainstBackup(ctx, tx); err != nil {
		return false, fmt.Errorf("lock physical blob deletion against backup: %w", err)
	}
	if err := s.store.DeleteDerivatives(ctx, digest); err != nil {
		return false, fmt.Errorf("delete swept blob derivatives: %w", err)
	}
	if err := s.store.Delete(ctx, id); err != nil && !errors.Is(err, storage.ErrBlobNotFound) {
		return false, fmt.Errorf("delete swept blob bytes: %w", err)
	}
	if _, err := tx.Exec(ctx, `delete from blobs where id = $1`, id); err != nil {
		return false, fmt.Errorf("delete swept blob record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit blob sweep: %w", err)
	}
	return true, nil
}

func (s *Service) prepareMarkedBlob(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin blob sweep preparation: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockBlobDigest(ctx, tx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	var marked pgtype.Timestamptz
	if err := tx.QueryRow(ctx,
		`select marked_at from blob_sweep_marks where blob_id = $1 for update`, id,
	).Scan(&marked); errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("lock marked blob for preparation: %w", err)
	}
	if !marked.Valid || marked.Time.After(now.Add(-sweepDelay)) {
		return false, nil
	}
	referenced, err := blobHasLiveReference(ctx, tx, id, now)
	if err != nil {
		return false, err
	}
	if referenced {
		if _, err := tx.Exec(ctx, `delete from blob_sweep_marks where blob_id = $1`, id); err != nil {
			return false, fmt.Errorf("clear blob sweep mark: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit cancelled blob sweep: %w", err)
		}
		return false, nil
	}
	if err := releaseExpiredReferences(ctx, tx, id, now); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit blob sweep preparation: %w", err)
	}
	return true, nil
}

func lockBlobDigest(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var digest []byte
	if err := tx.QueryRow(ctx, `select sha256 from blobs where id = $1`, id).Scan(&digest); err != nil {
		return err
	}
	return postgres.LockBlobDigest(ctx, tx, digest)
}

func blobHasLiveReference(ctx context.Context, tx pgx.Tx, id uuid.UUID, now time.Time) (bool, error) {
	var referenced bool
	err := tx.QueryRow(ctx, `
		select exists (
		    select 1 from ingest_operations operation
		     where operation.blob_id = $1
		       and (operation.status in ('pending', 'processing')
		            or (operation.status = 'needs_kind' and operation.expires_at > $2))
		    union all
		    select 1 from asset_revisions revision
		      join assets asset on asset.id = revision.asset_id
		     where revision.blob_id = $1
		       and (asset.deleted_at is null or asset.recoverable_until > $2)
		    union all
		    select 1 from asset_media media
		      join assets asset on asset.id = media.asset_id
		     where media.blob_id = $1
		       and (asset.deleted_at is null or asset.recoverable_until > $2)
		    union all
		    select 1 from asset_media media
		      join asset_revisions revision on revision.id = media.revision_id
		      join assets asset on asset.id = revision.asset_id
		     where media.blob_id = $1
		       and (asset.deleted_at is null or asset.recoverable_until > $2)
		)
	`, id, now).Scan(&referenced)
	if err != nil {
		return false, fmt.Errorf("recheck blob references: %w", err)
	}
	return referenced, nil
}

func releaseExpiredReferences(ctx context.Context, tx pgx.Tx, id uuid.UUID, now time.Time) error {
	statements := []string{
		`update asset_revisions revision set blob_id = null
		   from assets asset
		  where revision.asset_id = asset.id and revision.blob_id = $1
		    and asset.deleted_at is not null and asset.recoverable_until <= $2`,
		`update asset_media media set blob_id = null
		   from assets asset
		  where media.asset_id = asset.id and media.blob_id = $1
		    and asset.deleted_at is not null and asset.recoverable_until <= $2`,
		`update asset_media media set blob_id = null
		   from asset_revisions revision, assets asset
		  where media.revision_id = revision.id and revision.asset_id = asset.id
		    and media.blob_id = $1
		    and asset.deleted_at is not null and asset.recoverable_until <= $2`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement, id, now); err != nil {
			return fmt.Errorf("release expired blob reference: %w", err)
		}
	}
	return nil
}
