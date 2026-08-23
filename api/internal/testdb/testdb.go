package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// immutableTables refuse the mutations a reset needs, so the reset lifts their guard and puts it straight back.
var immutableTables = []string{"download_events", "migration_legacy_counters"}

// Connect opens a pool on the test database with the settings the server runs
// on, and empties every table.
func Connect(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return ConnectWith(t, nil)
}

// ConnectWith is Connect with one setting changed, so a test can prove a limit
// without waiting for the real one.
func ConnectWith(t *testing.T, tune func(*postgres.Settings)) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	settings := postgres.DefaultSettings(url)
	if tune != nil {
		tune(&settings)
	}

	pool, err := postgres.NewPool(context.Background(), settings)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	for _, frozen := range immutableTables {
		if _, err = pool.Exec(context.Background(),
			"alter table "+frozen+" disable trigger user"); err != nil {
			t.Fatalf("allow %s test reset: %v", frozen, err)
		}
	}
	_, truncateErr := pool.Exec(context.Background(),
		`truncate migration_exceptions, download_events, ingest_operations,
		          assets, asset_revisions, asset_media,
		          migration_legacy_counters, migration_preserved_records,
		          blob_tombstones, blob_sweep_marks, blobs,
		          link_rate_limits, instance_access_tokens, instance_refresh_history,
		          link_authorizations, link_requests, linked_instances,
		          password_reset_tokens, oauth_states, oauth_identities, sessions,
		          email_verification_tokens,
		          retired_handles, users cascade`)
	var enableErr error
	for _, frozen := range immutableTables {
		if _, err := pool.Exec(context.Background(),
			"alter table "+frozen+" enable trigger user"); err != nil && enableErr == nil {
			enableErr = err
		}
	}
	if truncateErr != nil {
		t.Fatalf("reset test database: %v", truncateErr)
	}
	if enableErr != nil {
		t.Fatalf("restore table immutability: %v", enableErr)
	}

	return pool
}
