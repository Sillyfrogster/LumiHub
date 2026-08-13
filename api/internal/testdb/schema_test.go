package testdb

import (
	"context"
	"slices"
	"testing"
)

func TestCoreTablesExist(t *testing.T) {
	pool := Connect(t)

	want := []string{"assets", "asset_revisions", "asset_facets", "asset_media", "blobs"}
	for _, table := range want {
		var exists bool
		err := pool.QueryRow(context.Background(),
			`select exists (select 1 from information_schema.tables where table_name = $1)`,
			table).Scan(&exists)
		if err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s is missing", table)
		}
	}
}

func TestBlobRowsContainOnlyContentAndLocation(t *testing.T) {
	pool := Connect(t)
	rows, err := pool.Query(context.Background(),
		`select column_name
		   from information_schema.columns
		  where table_schema = 'public' and table_name = 'blobs'
		  order by ordinal_position`)
	if err != nil {
		t.Fatalf("read blob columns: %v", err)
	}
	columns, err := rowsToStrings(rows)
	if err != nil {
		t.Fatalf("scan blob columns: %v", err)
	}

	want := []string{"id", "sha256", "byte_size", "storage_key"}
	if !slices.Equal(columns, want) {
		t.Errorf("blob columns = %v, want %v", columns, want)
	}
}

func TestBlobDigestIsUnique(t *testing.T) {
	pool := Connect(t)
	digest := make([]byte, 32)

	_, err := pool.Exec(context.Background(),
		`insert into blobs (id, sha256, byte_size, storage_key)
		 values (gen_random_uuid(), $1, 1, 'first')`, digest)
	if err != nil {
		t.Fatalf("insert first blob: %v", err)
	}
	_, err = pool.Exec(context.Background(),
		`insert into blobs (id, sha256, byte_size, storage_key)
		 values (gen_random_uuid(), $1, 1, 'second')`, digest)
	if err == nil {
		t.Fatal("the same digest was stored twice")
	}
}

func TestDurableRecordsReferenceBlobs(t *testing.T) {
	pool := Connect(t)

	for _, table := range []string{"asset_revisions", "asset_media"} {
		rows, err := pool.Query(context.Background(),
			`select column_name
			   from information_schema.columns
			  where table_schema = 'public' and table_name = $1`, table)
		if err != nil {
			t.Fatalf("read %s columns: %v", table, err)
		}
		columns, err := rowsToStrings(rows)
		if err != nil {
			t.Fatalf("scan %s columns: %v", table, err)
		}

		if !slices.Contains(columns, "blob_id") {
			t.Errorf("%s has no blob_id", table)
		}
		for _, forbidden := range []string{"content_hash", "byte_size", "storage_key", "sha256"} {
			if slices.Contains(columns, forbidden) {
				t.Errorf("%s still stores %s instead of a blob reference", table, forbidden)
			}
		}
	}
}

type stringRows interface {
	Next() bool
	Scan(...any) error
	Close()
	Err() error
}

func rowsToStrings(rows stringRows) ([]string, error) {
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func TestPublicationRejectsUnknownValue(t *testing.T) {
	pool := Connect(t)

	_, err := pool.Exec(context.Background(),
		`insert into assets (id, kind, format, name, publication)
		 values (gen_random_uuid(), 'character', 'unknown', 'x', 'nonsense')`)
	if err == nil {
		t.Fatal("expected the publication check constraint to reject 'nonsense'")
	}
}
