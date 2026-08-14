package asset

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin purge: %w", err)
	}
	defer tx.Rollback(ctx)
	var locked int
	if err := tx.QueryRow(ctx, `
		select 1 from pg_advisory_xact_lock(
		    hashtextextended('lumihub-blob:' || encode($1::bytea, 'hex'), 0)
		)
	`, digest[:]).Scan(&locked); err != nil {
		return fmt.Errorf("lock purge digest: %w", err)
	}

	var blobID uuid.UUID
	if err := tx.QueryRow(ctx,
		`select id from blobs where sha256 = $1 for update`, digest[:],
	).Scan(&blobID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("find blob for purge: %w", err)
	}
	now := s.now()
	if _, err := tx.Exec(ctx, `
		insert into blob_tombstones (sha256, reason_code, purged_at, actor_id)
		values ($1, $2, $3, $4)
	`, digest[:], reasonCode, now, actorID); err != nil {
		return fmt.Errorf("record purge tombstone: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		update ingest_operations
		   set status = 'failed', failure_reason = 'purged_content', blob_id = null,
		       lease_token = null, lease_expires_at = null, updated_at = $2
		 where blob_id = $1 and status <> 'success'
	`, blobID, now); err != nil {
		return fmt.Errorf("break purged ingest references: %w", err)
	}
	for _, statement := range []string{
		`update asset_media set blob_id = null where blob_id = $1`,
		`update asset_revisions set blob_id = null where blob_id = $1`,
	} {
		if _, err := tx.Exec(ctx, statement, blobID); err != nil {
			return fmt.Errorf("break purged record references: %w", err)
		}
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
