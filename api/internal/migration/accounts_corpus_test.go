package migration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEveryRealCreatorArrivesWithNothingLostSilently(t *testing.T) {
	target := testdb.Connect(t)
	source := restoredV1Dump(t)

	report, err := MigrateAccounts(context.Background(), source, target)
	if err != nil {
		t.Fatalf("migrate the real accounts: %v", err)
	}

	if report.Accounts != 1174 {
		t.Errorf("accounts = %d, want 1174", report.Accounts)
	}
	assertCounts(t, "role", report.Roles, map[string]int{"user": 1170, "moderator": 2, "admin": 2})
	assertCounts(t, "NSFW visibility", report.Visibility,
		map[string]int{"hidden": 710, "blurred": 60, "shown": 404})
	assertHandlesCarriedVerbatim(t, source, target)
	assertPreferencesCarried(t, target)
	assertLedgerNamesEveryException(t, target, report)
}

func assertCounts(t *testing.T, subject string, got, want map[string]int) {
	t.Helper()
	for key, total := range want {
		if got[key] != total {
			t.Errorf("%s %s = %d, want %d", subject, key, got[key], total)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%s values = %v, want exactly %v", subject, got, want)
	}
}

func assertHandlesCarriedVerbatim(t *testing.T, source, target *pgxpool.Pool) {
	t.Helper()
	before := handles(t, source, `select username from users`)
	after := handles(t, target, `select username from users`)
	if len(before) != len(after) {
		t.Fatalf("handles = %d, want the %d that arrived", len(after), len(before))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("the handle at position %d changed on the way across", i)
		}
	}

	var retired int
	if err := target.QueryRow(context.Background(),
		`select count(*) from retired_handles`).Scan(&retired); err != nil {
		t.Fatalf("count the retired handles: %v", err)
	}
	if retired != 0 {
		t.Errorf("retired handles = %d, want none", retired)
	}
}

func assertPreferencesCarried(t *testing.T, target *pgxpool.Pool) {
	t.Helper()
	var profile, customNames, includeLists, excludeLists, credentials, avatars, banners int
	if err := target.QueryRow(context.Background(),
		`select count(*) filter (where not show_nsfw_contributions_on_profile),
		        count(*) filter (where custom_display_name <> ''),
		        count(*) filter (where default_include_tags <> '[]'::jsonb),
		        count(*) filter (where default_exclude_tags <> '[]'::jsonb),
		        count(*) filter (where email is not null or password_hash is not null
		                            or email_source is not null or email_verified_at is not null),
		        count(*) filter (where avatar_url like 'https://cdn.discordapp.com/avatars/%'),
		        count(*) filter (where banner_url like 'https://cdn.discordapp.com/banners/%')
		   from users`,
	).Scan(&profile, &customNames, &includeLists, &excludeLists, &credentials,
		&avatars, &banners); err != nil {
		t.Fatalf("read the migrated preferences: %v", err)
	}
	if profile != 1168 {
		t.Errorf("accounts keeping NSFW work off their profile = %d, want 1168", profile)
	}
	if customNames != 14 {
		t.Errorf("creator-set display names = %d, want 14", customNames)
	}
	if includeLists != 2 || excludeLists != 29 {
		t.Errorf("tag preference lists = %d include and %d exclude, want 2 and 29",
			includeLists, excludeLists)
	}
	if credentials != 0 {
		t.Errorf("%d accounts carry a credential, want none until the next sign-in", credentials)
	}
	if avatars != 1006 || banners != 175 {
		t.Errorf("Discord CDN URLs = %d avatars and %d banners, want 1006 and 175",
			avatars, banners)
	}
}

func assertLedgerNamesEveryException(t *testing.T, target *pgxpool.Pool, report AccountReport) {
	t.Helper()
	counted := map[string]int{}
	for _, entry := range report.Exceptions {
		counted[entry.Kind]++
	}
	assertCounts(t, "ledger entry", counted,
		map[string]int{"dropped_column": 5, "grandfathered_handle": 3})

	rows, err := target.Query(context.Background(),
		`select kind, subject from migration_exceptions order by kind, subject`)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	defer rows.Close()
	var droppedColumns []string
	grandfathered := 0
	for rows.Next() {
		var kind, subject string
		if err := rows.Scan(&kind, &subject); err != nil {
			t.Fatalf("scan a ledger row: %v", err)
		}
		switch kind {
		case "dropped_column":
			droppedColumns = append(droppedColumns, subject)
		case "grandfathered_handle":
			grandfathered++
			if !handleIsAllDigits(subject) {
				t.Error("a grandfathered handle is not the all-digit case the rule exempts")
			}
		default:
			t.Errorf("the ledger holds an unexpected %s entry", kind)
		}
	}
	wantColumns := []string{
		"users.banned", "users.banned_at", "users.banned_by", "users.banned_reason",
		"users.refresh_token",
	}
	if !slices.Equal(droppedColumns, wantColumns) {
		t.Errorf("dropped columns = %v, want %v", droppedColumns, wantColumns)
	}
	if grandfathered != 3 {
		t.Errorf("grandfathered handles = %d, want 3", grandfathered)
	}
}

func handles(t *testing.T, pool *pgxpool.Pool, query string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("read the handles: %v", err)
	}
	defer rows.Close()
	var found []string
	for rows.Next() {
		var handle string
		if err := rows.Scan(&handle); err != nil {
			t.Fatalf("scan a handle: %v", err)
		}
		found = append(found, handle)
	}
	sort.Strings(found)
	return found
}

func restoredV1Dump(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dump := repositoryFile(t, ".ai", "dump", "db_backup.sql")
	if _, err := os.Stat(dump); errors.Is(err, os.ErrNotExist) {
		t.Skip("the local v1 dump is absent")
	} else if err != nil {
		t.Fatal("the local v1 dump cannot be inspected")
	}
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	scratch, err := NewScratchDatabase(context.Background(), base, "v1_accounts_")
	if err != nil {
		t.Skipf("the test database cannot create a scratch database: %v", err)
	}
	t.Cleanup(func() { _ = scratch.Drop(context.Background()) })
	if err := scratch.RestoreDump(context.Background(), dump); err != nil {
		t.Fatalf("restore the v1 dump: %v", err)
	}
	return scratch.Pool
}

func repositoryFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("the repository path is unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	return filepath.Join(append([]string{root}, parts...)...)
}
