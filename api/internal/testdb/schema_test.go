package testdb

import (
	"context"
	"testing"
)

func TestCoreTablesExist(t *testing.T) {
	pool := Connect(t)

	want := []string{"assets", "asset_revisions", "asset_facets", "asset_media"}
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

func TestPublicationRejectsUnknownValue(t *testing.T) {
	pool := Connect(t)

	_, err := pool.Exec(context.Background(),
		`insert into assets (id, kind, format, name, publication)
		 values (gen_random_uuid(), 'character', 'unknown', 'x', 'nonsense')`)
	if err == nil {
		t.Fatal("expected the publication check constraint to reject 'nonsense'")
	}
}
