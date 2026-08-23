package migration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const v1AccountsSchema = `
create table users (
    id uuid primary key,
    discord_id character varying(255) not null,
    username character varying(255) not null,
    banner character varying(512),
    display_name character varying(255),
    avatar character varying(255),
    role character varying(255) default 'user' not null,
    refresh_token character varying(255),
    created_at timestamp without time zone default now() not null,
    custom_display_name character varying(255),
    nsfw_enabled boolean default false not null,
    nsfw_unblurred boolean default false not null,
    default_include_tags jsonb default '[]'::jsonb not null,
    default_exclude_tags jsonb default '[]'::jsonb not null,
    banned boolean default false not null,
    banned_at timestamp without time zone,
    banned_reason text,
    banned_by uuid,
    show_nsfw_contributions_on_profile boolean default false not null
)`

type v1AccountFixture struct {
	Handle        string
	DiscordID     string
	Role          string
	NSFWEnabled   bool
	NSFWUnblurred bool
	Avatar        string
	Banner        *string
	CustomName    *string
	IncludeTags   string
	ExcludeTags   string
	ShowOnProfile bool
	RefreshToken  *string
	Banned        bool
}

func v1Source(t *testing.T, fixtures []v1AccountFixture, extraColumn string) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	scratch, err := NewScratchDatabase(context.Background(), base, "v1_accounts_")
	if err != nil {
		t.Skipf("the test database cannot create a scratch database: %v", err)
	}
	t.Cleanup(func() { _ = scratch.Drop(context.Background()) })

	if _, err := scratch.Pool.Exec(context.Background(), v1AccountsSchema); err != nil {
		t.Fatalf("create the v1 accounts table: %v", err)
	}
	if extraColumn != "" {
		if _, err := scratch.Pool.Exec(context.Background(),
			"alter table users add column "+extraColumn); err != nil {
			t.Fatalf("add the undeclared column: %v", err)
		}
	}
	for i, fixture := range fixtures {
		if _, err := scratch.Pool.Exec(context.Background(),
			`insert into users (id, discord_id, username, banner, display_name, avatar, role,
			                    refresh_token, created_at, custom_display_name, nsfw_enabled,
			                    nsfw_unblurred, default_include_tags, default_exclude_tags,
			                    banned, show_nsfw_contributions_on_profile)
			 values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			uuid.New(), fixture.DiscordID, fixture.Handle, fixture.Banner,
			"Display "+fixture.Handle, fixture.Avatar, fixture.Role, fixture.RefreshToken,
			time.Date(2026, 5, 6, 16, 49, 13, 0, time.UTC).Add(time.Duration(i)*time.Hour),
			fixture.CustomName, fixture.NSFWEnabled, fixture.NSFWUnblurred,
			orEmptyList(fixture.IncludeTags), orEmptyList(fixture.ExcludeTags),
			fixture.Banned, fixture.ShowOnProfile,
		); err != nil {
			t.Fatalf("insert the v1 account %s: %v", fixture.Handle, err)
		}
	}
	return scratch.Pool
}

func orEmptyList(value string) string {
	if value == "" {
		return "[]"
	}
	return value
}

func TestEveryAccountArrivesWithItsHandleAndPreferencesUnchanged(t *testing.T) {
	target := testdb.Connect(t)
	banner := "https://cdn.discordapp.com/banners/1/deadbeef.png?size=1024"
	custom := "Marbleframe"
	source := v1Source(t, []v1AccountFixture{{
		Handle: "quillbark.oc", DiscordID: "100000000000000001", Role: "user",
		NSFWEnabled: true, NSFWUnblurred: false,
		Avatar: "https://cdn.discordapp.com/avatars/1/cafe.png",
		Banner: &banner, CustomName: &custom,
		ExcludeTags: `["mecha", "noir"]`, ShowOnProfile: true,
	}}, "")

	report, err := MigrateAccounts(context.Background(), source, target)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Accounts != 1 {
		t.Fatalf("accounts = %d, want 1", report.Accounts)
	}

	var handle, avatar, bannerURL, customName, visibility, excludeTags string
	var showOnProfile bool
	if err := target.QueryRow(context.Background(),
		`select username, avatar_url, banner_url, custom_display_name, nsfw_visibility,
		        default_exclude_tags::text, show_nsfw_contributions_on_profile from users`,
	).Scan(&handle, &avatar, &bannerURL, &customName, &visibility, &excludeTags,
		&showOnProfile); err != nil {
		t.Fatalf("read the migrated account: %v", err)
	}
	if handle != "quillbark.oc" {
		t.Errorf("handle = %q, want it unchanged", handle)
	}
	if avatar != "https://cdn.discordapp.com/avatars/1/cafe.png" || bannerURL != banner {
		t.Errorf("avatar = %q and banner = %q, want the Discord CDN URLs", avatar, bannerURL)
	}
	if customName != "Marbleframe" || visibility != "blurred" || !showOnProfile {
		t.Errorf("name = %q, visibility = %q, profile = %v", customName, visibility, showOnProfile)
	}
	if !strings.Contains(excludeTags, "mecha") || !strings.Contains(excludeTags, "noir") {
		t.Errorf("exclude tags = %s, want them carried as-is", excludeTags)
	}
}

func TestTheDiscordIdentityCarriesSoAnExistingCreatorSignsBackIn(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "tallowmoth", DiscordID: "100000000000000002", Role: "user"},
	}, "")

	if _, err := MigrateAccounts(context.Background(), source, target); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var subject, providerEmail string
	if err := target.QueryRow(context.Background(),
		`select subject, coalesce(provider_email, '') from oauth_identities
		  where provider = 'discord'`).Scan(&subject, &providerEmail); err != nil {
		t.Fatalf("read the Discord identity: %v", err)
	}
	if subject != "100000000000000002" {
		t.Errorf("subject = %q, want the v1 Discord id", subject)
	}
	if providerEmail != "" {
		t.Errorf("provider email = %q, want none until the next sign-in", providerEmail)
	}
}

func TestNoEmailAddressOrPasswordIsMigrated(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "pinegloss", DiscordID: "100000000000000003", Role: "user"},
	}, "")

	if _, err := MigrateAccounts(context.Background(), source, target); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var withCredentials int
	if err := target.QueryRow(context.Background(),
		`select count(*) from users
		  where email is not null or email_source is not null
		     or email_verified_at is not null or password_hash is not null`,
	).Scan(&withCredentials); err != nil {
		t.Fatalf("read the migrated accounts: %v", err)
	}
	if withCredentials != 0 {
		t.Errorf("%d accounts carry a credential, want none", withCredentials)
	}
}

func TestAPureDigitHandleIsGrandfatheredRatherThanRewritten(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "4820193", DiscordID: "1", Role: "user"},
	}, "")

	report, err := MigrateAccounts(context.Background(), source, target)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var handle string
	if err := target.QueryRow(context.Background(),
		`select username from users`).Scan(&handle); err != nil {
		t.Fatalf("read the migrated account: %v", err)
	}
	if handle != "4820193" {
		t.Errorf("handle = %q, want it unchanged", handle)
	}
	if !ledgered(report, "grandfathered_handle", "4820193") {
		t.Errorf("the ledger does not name the grandfathered handle: %v", report.Exceptions)
	}
}

func TestTheNSFWPairCollapsesToOneVisibility(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "hides.it", DiscordID: "1", Role: "user"},
		{Handle: "blurs.it", DiscordID: "2", Role: "user", NSFWEnabled: true},
		{Handle: "shows.it", DiscordID: "3", Role: "user", NSFWEnabled: true, NSFWUnblurred: true},
	}, "")

	report, err := MigrateAccounts(context.Background(), source, target)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	want := map[string]int{"hidden": 1, "blurred": 1, "shown": 1}
	for visibility, total := range want {
		if report.Visibility[visibility] != total {
			t.Errorf("%s = %d, want %d", visibility, report.Visibility[visibility], total)
		}
	}
}

func TestAContradictoryNSFWPairKeepsTheSaferAnswerAndIsRecorded(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "contra.dicts", DiscordID: "1", Role: "user", NSFWUnblurred: true},
	}, "")

	report, err := MigrateAccounts(context.Background(), source, target)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if report.Visibility["hidden"] != 1 {
		t.Errorf("visibility = %v, want the account hidden", report.Visibility)
	}
	if !ledgered(report, "contradictory_nsfw_pair", "contra.dicts") {
		t.Errorf("the ledger does not name the contradiction: %v", report.Exceptions)
	}
}

func TestTheDroppedColumnsAreNamedInTheLedger(t *testing.T) {
	target := testdb.Connect(t)
	token := "a-live-discord-refresh-token"
	source := v1Source(t, []v1AccountFixture{
		{Handle: "dropped.one", DiscordID: "1", Role: "user", RefreshToken: &token},
	}, "")

	report, err := MigrateAccounts(context.Background(), source, target)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, column := range []string{
		"users.banned", "users.banned_at", "users.banned_reason", "users.banned_by",
		"users.refresh_token",
	} {
		if !ledgered(report, "dropped_column", column) {
			t.Errorf("the ledger does not name %s: %v", column, report.Exceptions)
		}
	}

	var recorded int
	if err := target.QueryRow(context.Background(),
		`select count(*) from migration_exceptions where kind = 'dropped_column'`,
	).Scan(&recorded); err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if recorded != 5 {
		t.Errorf("ledger rows = %d, want 5", recorded)
	}
}

func TestAnUndeclaredSourceColumnStopsTheRun(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "surprise.me", DiscordID: "1", Role: "user"},
	}, "secret_note text")

	_, err := MigrateAccounts(context.Background(), source, target)

	if err == nil {
		t.Fatal("an undeclared column did not stop the run")
	}
	if !strings.Contains(err.Error(), "secret_note") {
		t.Errorf("error = %q, want it to name the column", err)
	}
	assertTargetUntouched(t, target)
}

func TestARoleOutsideTheThreeStopsTheRun(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "self.promoted", DiscordID: "1", Role: "owner"},
	}, "")

	if _, err := MigrateAccounts(context.Background(), source, target); err == nil {
		t.Fatal("an unknown role did not stop the run")
	}
	assertTargetUntouched(t, target)
}

func TestMigrationRefusesATargetThatAlreadyHoldsAccounts(t *testing.T) {
	target := testdb.Connect(t)
	if _, err := target.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'already.here')`, uuid.New()); err != nil {
		t.Fatalf("seed the target: %v", err)
	}
	source := v1Source(t, []v1AccountFixture{
		{Handle: "arriving.late", DiscordID: "1", Role: "user"},
	}, "")

	_, err := MigrateAccounts(context.Background(), source, target)

	if err == nil {
		t.Fatal("migration ran against a target that already held accounts")
	}
	var accounts int
	if err := target.QueryRow(context.Background(),
		`select count(*) from users`).Scan(&accounts); err != nil {
		t.Fatalf("count the accounts: %v", err)
	}
	if accounts != 1 {
		t.Errorf("accounts = %d, want the target left as it was", accounts)
	}
}

func TestRetiredHandlesShipEmpty(t *testing.T) {
	target := testdb.Connect(t)
	source := v1Source(t, []v1AccountFixture{
		{Handle: "keeps.it", DiscordID: "1", Role: "user"},
	}, "")

	if _, err := MigrateAccounts(context.Background(), source, target); err != nil {
		t.Fatalf("migrate: %v", err)
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

func assertTargetUntouched(t *testing.T, target *pgxpool.Pool) {
	t.Helper()
	var accounts, identities, exceptions int
	if err := target.QueryRow(context.Background(),
		`select (select count(*) from users), (select count(*) from oauth_identities),
		        (select count(*) from migration_exceptions)`,
	).Scan(&accounts, &identities, &exceptions); err != nil {
		t.Fatalf("read the target: %v", err)
	}
	if accounts != 0 || identities != 0 || exceptions != 0 {
		t.Errorf("a failed run left %d accounts, %d identities and %d ledger rows",
			accounts, identities, exceptions)
	}
}

func ledgered(report AccountReport, kind, subject string) bool {
	for _, entry := range report.Exceptions {
		if entry.Kind == kind && entry.Subject == subject {
			return true
		}
	}
	return false
}
