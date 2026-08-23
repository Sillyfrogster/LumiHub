package migration

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// expectation is what the source asked for, counted from it rather than from the rows just inserted.
type expectation struct {
	Assets      int
	Images      int
	LegacyPaths int
	Preserved   int
	Exceptions  int
}

// reconcile proves the written rows are the rows the source asked for, and a count that does not add up is fatal.
func reconcile(ctx context.Context, tx pgx.Tx, wanted expectation, ledger *Ledger) error {
	counted := []struct {
		Table  string
		Query  string
		Wanted int
	}{
		{"assets", "select count(*) from assets", wanted.Assets},
		{"asset_media", "select count(*) from asset_media", wanted.Images},
		{"asset_legacy_paths", "select count(*) from asset_legacy_paths", wanted.LegacyPaths},
		{"migration_legacy_counters", "select count(*) from migration_legacy_counters", wanted.Assets},
		{"migration_preserved_records", "select count(*) from migration_preserved_records", wanted.Preserved},
		{"migration_exceptions", "select count(*) from migration_exceptions", wanted.Exceptions},
		{"content_generation", "select count(*) from assets where content_generation = 1", wanted.Assets},
	}
	for _, check := range counted {
		var found int
		if err := tx.QueryRow(ctx, check.Query).Scan(&found); err != nil {
			return fmt.Errorf("count the migrated %s: %w", check.Table, err)
		}
		if found != check.Wanted {
			return ledger.Raise(Exception{
				Kind: "count_mismatch", Subject: check.Table,
				Detail: fmt.Sprintf("%d rows arrived and the source asked for %d", found, check.Wanted),
			})
		}
	}
	return nil
}
