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

	var sourceAccounts int
	if err := source.QueryRow(context.Background(), `select count(*) from users`).Scan(&sourceAccounts); err != nil {
		t.Fatalf("count the source accounts: %v", err)
	}
	if report.Accounts != sourceAccounts {
		t.Errorf("the account report does not match the source")
	}
	assertCompleteBreakdown(t, "role", report.Accounts, report.Roles,
		[]string{"user", "moderator", "admin"})
	assertCompleteBreakdown(t, "NSFW visibility", report.Accounts, report.Visibility,
		[]string{"hidden", "blurred", "shown"})
	assertHandlesCarriedVerbatim(t, source, target)
	assertPreferencesCarried(t, target)
	assertLedgerNamesEveryException(t, target, report)
}

func assertCompleteBreakdown(
	t *testing.T,
	subject string,
	total int,
	got map[string]int,
	allowed []string,
) {
	t.Helper()
	found := 0
	for _, key := range allowed {
		found += got[key]
	}
	if found != total || len(got) != len(allowed) {
		t.Errorf("%s totals do not account for every migrated account", subject)
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
	if profile == 0 || customNames == 0 || includeLists == 0 || excludeLists == 0 {
		t.Error("the source no longer exercises every migrated account preference")
	}
	if credentials != 0 {
		t.Errorf("%d accounts carry a credential, want none until the next sign-in", credentials)
	}
	if avatars == 0 || banners == 0 {
		t.Error("the source no longer exercises both Discord image fields")
	}
}

func assertLedgerNamesEveryException(t *testing.T, target *pgxpool.Pool, report AccountReport) {
	t.Helper()
	counted := map[string]int{}
	for _, entry := range report.Exceptions {
		counted[entry.Kind]++
	}
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
	if grandfathered == 0 {
		t.Error("the source no longer exercises grandfathered handles")
	}
	if counted["dropped_column"] != len(wantColumns) ||
		counted["grandfathered_handle"] != grandfathered || len(counted) != 2 {
		t.Error("the account exception report does not match the stored ledger")
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
