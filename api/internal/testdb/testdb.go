package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	if _, err = pool.Exec(context.Background(),
		`alter table download_events disable trigger user`); err != nil {
		t.Fatalf("allow download event test reset: %v", err)
	}
	_, truncateErr := pool.Exec(context.Background(),
		`truncate download_events, ingest_operations, assets, asset_revisions, asset_media,
		          blob_tombstones, blob_sweep_marks, blobs,
		          link_rate_limits, instance_access_tokens, instance_refresh_history,
		          link_authorizations, link_requests, linked_instances,
		          password_reset_tokens, oauth_states, oauth_identities, sessions,
		          email_verification_tokens,
		          retired_handles, users cascade`)
	_, enableErr := pool.Exec(context.Background(),
		`alter table download_events enable trigger user`)
	if truncateErr != nil {
		t.Fatalf("reset test database: %v", truncateErr)
	}
	if enableErr != nil {
		t.Fatalf("restore download event immutability: %v", enableErr)
	}

	return pool
}
