package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func LockBlobDigest(ctx context.Context, tx pgx.Tx, digest []byte) error {
	var locked int
	return tx.QueryRow(ctx, `
		select 1 from pg_advisory_xact_lock(
		    hashtextextended('illarin-blob:' || encode($1::bytea, 'hex'), 0)
		)
	`, digest).Scan(&locked)
}

func LockBlobDeletionAgainstBackup(ctx context.Context, tx pgx.Tx) error {
	var locked int
	return tx.QueryRow(ctx, `
		select 1 from pg_advisory_xact_lock_shared(
		    hashtextextended('illarin-backup:blob-deletion', 0)
		)
	`).Scan(&locked)
}
