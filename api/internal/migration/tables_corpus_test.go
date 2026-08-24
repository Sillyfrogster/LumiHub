package migration

import (
	"context"
	"sort"
	"testing"
)

func TestEveryV1TableHasADeclaredDisposition(t *testing.T) {
	source := restoredV1Dump(t)
	rows, err := source.Query(context.Background(),
		`select tablename from pg_tables where schemaname = 'public'`)
	if err != nil {
		t.Fatalf("read the v1 table list: %v", err)
	}
	defer rows.Close()

	present := map[string]bool{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("read a v1 table name: %v", err)
		}
		present[table] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the v1 table list: %v", err)
	}

	declared := map[string]bool{}
	for _, disposition := range v1Tables() {
		if declared[disposition.Table] {
			t.Errorf("%s is declared twice", disposition.Table)
		}
		declared[disposition.Table] = true
	}

	for _, table := range sorted(present) {
		if !declared[table] {
			t.Errorf("the v1 database holds %s and nothing declares what becomes of it", table)
		}
	}
	for _, table := range sorted(declared) {
		if !present[table] {
			t.Errorf("%s is declared but the v1 database does not hold it", table)
		}
	}
}

func sorted(names map[string]bool) []string {
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
