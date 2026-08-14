package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/postgres"
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

	_, err = pool.Exec(context.Background(),
		`truncate ingest_operations, assets, asset_revisions, asset_facets, asset_media,
		          blob_tombstones, blob_sweep_marks, blobs,
		          password_reset_tokens, oauth_states, oauth_identities, sessions,
		          email_verification_tokens,
		          retired_handles, users cascade`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return pool
}
