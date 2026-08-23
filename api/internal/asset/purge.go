package asset

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/postgres"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInvalidPurgeReason = errors.New("invalid purge reason")

func (s *Service) Purge(
	ctx context.Context,
	digest [sha256.Size]byte,
	reasonCode string,
	actorID uuid.UUID,
) error {
	reasonCode = strings.TrimSpace(reasonCode)
	if reasonCode == "" {
		return ErrInvalidPurgeReason
	}
	blobID, err := s.preparePurge(ctx, digest, reasonCode, actorID)
	if err != nil {
		return err
	}
	return s.deletePurgedBlob(ctx, blobID, digest)
}

func (s *Service) preparePurge(
	ctx context.Context,
	digest [sha256.Size]byte,
	reasonCode string,
	actorID uuid.UUID,
) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin purge: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := postgres.LockBlobDigest(ctx, tx, digest[:]); err != nil {
		return uuid.Nil, fmt.Errorf("lock purge digest: %w", err)
	}

	var blobID uuid.UUID
	if err := tx.QueryRow(ctx,
		`select id from blobs where sha256 = $1 for update`, digest[:],
	).Scan(&blobID); errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	} else if err != nil {
		return uuid.Nil, fmt.Errorf("find blob for purge: %w", err)
	}
	now := s.now()
	if _, err := tx.Exec(ctx, `
		insert into blob_tombstones (sha256, reason_code, purged_at, actor_id)
		values ($1, $2, $3, $4)
		on conflict (sha256) do nothing
	`, digest[:], reasonCode, now, actorID); err != nil {
		return uuid.Nil, fmt.Errorf("record purge tombstone: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		update ingest_operations
		   set status = 'failed', failure_reason = 'internal_failure', blob_id = null,
		       lease_token = null, lease_expires_at = null, updated_at = $2
		 where blob_id = $1 and status <> 'success'
	`, blobID, now); err != nil {
		return uuid.Nil, fmt.Errorf("break purged ingest references: %w", err)
	}
	for _, statement := range []string{
		`update asset_media set blob_id = null where blob_id = $1`,
		`update asset_revisions set blob_id = null where blob_id = $1`,
	} {
		if _, err := tx.Exec(ctx, statement, blobID); err != nil {
			return uuid.Nil, fmt.Errorf("break purged record references: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit purge record: %w", err)
	}
	return blobID, nil
}

func (s *Service) deletePurgedBlob(
	ctx context.Context,
	blobID uuid.UUID,
	digest [sha256.Size]byte,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin purged blob deletion: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := postgres.LockBlobDigest(ctx, tx, digest[:]); err != nil {
		return fmt.Errorf("lock purged digest deletion: %w", err)
	}
	var recordedID uuid.UUID
	if err := tx.QueryRow(ctx,
		`select id from blobs where sha256 = $1 for update`, digest[:],
	).Scan(&recordedID); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("find purged blob for deletion: %w", err)
	}
	if recordedID != blobID {
		return fmt.Errorf("purged blob identity changed")
	}
	if err := postgres.LockBlobDeletionAgainstBackup(ctx, tx); err != nil {
		return fmt.Errorf("lock physical blob deletion against backup: %w", err)
	}
	if err := s.store.DeleteDerivatives(ctx, digest); err != nil {
		return fmt.Errorf("delete purged derivatives: %w", err)
	}
	if err := s.store.Delete(ctx, blobID); err != nil && !errors.Is(err, storage.ErrBlobNotFound) {
		return fmt.Errorf("delete purged bytes: %w", err)
	}
	if _, err := tx.Exec(ctx, `delete from blobs where id = $1`, blobID); err != nil {
		return fmt.Errorf("delete purged blob record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit purge: %w", err)
	}
	return nil
}
