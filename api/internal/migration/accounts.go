package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountReport is what one accounts migration carried across.
type AccountReport struct {
	Accounts   int
	Roles      map[string]int
	Visibility map[string]int
	Exceptions []Exception
}

type v1Account struct {
	ID            uuid.UUID
	DiscordID     string
	Handle        string
	Banner        string
	DisplayName   string
	Avatar        string
	Role          string
	RefreshToken  string
	CreatedAt     time.Time
	CustomName    string
	NSFWEnabled   bool
	NSFWUnblurred bool
	IncludeTags   []byte
	ExcludeTags   []byte
	Banned        bool
	BannedAt      *time.Time
	BannedReason  string
	BannedBy      *uuid.UUID
	ShowOnProfile bool
}

type account struct {
	ID             uuid.UUID
	Handle         string
	Role           string
	CreatedAt      time.Time
	DisplayName    string
	CustomName     string
	AvatarURL      string
	BannerURL      string
	Visibility     string
	ShowOnProfile  bool
	IncludeTags    []byte
	ExcludeTags    []byte
	DiscordSubject string
}

// MigrateAccounts carries every v1 account into an empty v2 schema and ledgers what it leaves behind.
func MigrateAccounts(ctx context.Context, source, target *pgxpool.Pool) (AccountReport, error) {
	ledger, err := NewLedger(accountAnomalies())
	if err != nil {
		return AccountReport{}, err
	}
	if err := requireEmptyTarget(ctx, target); err != nil {
		return AccountReport{}, err
	}
	if err := requireDeclaredColumns(ctx, source, ledger); err != nil {
		return AccountReport{}, err
	}

	sourceRows, err := readV1Accounts(ctx, source)
	if err != nil {
		return AccountReport{}, err
	}
	carried := make([]account, 0, len(sourceRows))
	for _, row := range sourceRows {
		one, err := carryAccount(row, ledger)
		if err != nil {
			return AccountReport{}, err
		}
		carried = append(carried, one)
	}
	if err := recordDroppedColumns(sourceRows, ledger); err != nil {
		return AccountReport{}, err
	}

	tx, err := target.Begin(ctx)
	if err != nil {
		return AccountReport{}, fmt.Errorf("begin the accounts migration: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	for _, one := range carried {
		if err := writeAccount(ctx, queries, one); err != nil {
			return AccountReport{}, err
		}
	}
	if err := ledger.Persist(ctx, tx); err != nil {
		return AccountReport{}, err
	}
	report, err := reconcileAccounts(ctx, queries, carried, ledger)
	if err != nil {
		return AccountReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AccountReport{}, fmt.Errorf("commit the accounts migration: %w", err)
	}
	return report, nil
}

func requireEmptyTarget(ctx context.Context, target *pgxpool.Pool) error {
	empty, err := db.New(target).MigrationTargetIsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check that the target is empty: %w", err)
	}
	if !empty {
		return errors.New("the target schema already holds accounts, so the migration will not run")
	}
	return nil
}

func requireDeclaredColumns(ctx context.Context, source *pgxpool.Pool, ledger *Ledger) error {
	declared := make(map[string]bool, len(accountColumns()))
	for _, column := range accountColumns() {
		declared[column.Column] = true
	}
	rows, err := source.Query(ctx,
		`select column_name from information_schema.columns
		  where table_schema = 'public' and table_name = 'users'`)
	if err != nil {
		return fmt.Errorf("read the source columns: %w", err)
	}
	defer rows.Close()
	present := make(map[string]bool, len(declared))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan a source column: %w", err)
		}
		present[name] = true
		if !declared[name] {
			if err := ledger.Raise(Exception{
				Kind: "undeclared_column", Subject: "users." + name,
				Detail: "the source column has no declared disposition",
			}); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read the source columns: %w", err)
	}
	for name := range declared {
		if !present[name] {
			if err := ledger.Raise(Exception{
				Kind: "stale_column_declaration", Subject: "users." + name,
				Detail: "the declaration names a column the source does not have",
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// readV1Accounts reads v1's timestamps as UTC, which is what its timezone-free column holds.
func readV1Accounts(ctx context.Context, source *pgxpool.Pool) ([]v1Account, error) {
	rows, err := source.Query(ctx,
		`select id, discord_id, username, coalesce(banner, ''), coalesce(display_name, ''),
		        coalesce(avatar, ''), role, coalesce(refresh_token, ''), created_at,
		        coalesce(custom_display_name, ''), nsfw_enabled, nsfw_unblurred,
		        default_include_tags, default_exclude_tags, banned, banned_at,
		        coalesce(banned_reason, ''), banned_by, show_nsfw_contributions_on_profile
		   from users order by created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 accounts: %w", err)
	}
	defer rows.Close()
	var accounts []v1Account
	for rows.Next() {
		var row v1Account
		if err := rows.Scan(
			&row.ID, &row.DiscordID, &row.Handle, &row.Banner, &row.DisplayName,
			&row.Avatar, &row.Role, &row.RefreshToken, &row.CreatedAt,
			&row.CustomName, &row.NSFWEnabled, &row.NSFWUnblurred,
			&row.IncludeTags, &row.ExcludeTags, &row.Banned, &row.BannedAt,
			&row.BannedReason, &row.BannedBy, &row.ShowOnProfile,
		); err != nil {
			return nil, fmt.Errorf("scan a v1 account: %w", err)
		}
		accounts = append(accounts, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 accounts: %w", err)
	}
	return accounts, nil
}

func carryAccount(row v1Account, ledger *Ledger) (account, error) {
	visibility, err := collapseNSFW(row, ledger)
	if err != nil {
		return account{}, err
	}
	if handleIsAllDigits(row.Handle) {
		if err := ledger.Raise(Exception{
			Kind: "grandfathered_handle", Subject: row.Handle,
			Detail: "an all-digit handle predates the rule and is kept rather than rewritten",
		}); err != nil {
			return account{}, err
		}
	}
	return account{
		ID: row.ID, Handle: row.Handle, Role: row.Role,
		CreatedAt: row.CreatedAt, DisplayName: row.DisplayName, CustomName: row.CustomName,
		AvatarURL: row.Avatar, BannerURL: row.Banner, Visibility: visibility,
		ShowOnProfile: row.ShowOnProfile, IncludeTags: row.IncludeTags,
		ExcludeTags: row.ExcludeTags, DiscordSubject: row.DiscordID,
	}, nil
}

func collapseNSFW(row v1Account, ledger *Ledger) (string, error) {
	switch {
	case row.NSFWEnabled && row.NSFWUnblurred:
		return "shown", nil
	case row.NSFWEnabled:
		return "blurred", nil
	case row.NSFWUnblurred:
		return "hidden", ledger.Raise(Exception{
			Kind: "contradictory_nsfw_pair", Subject: row.Handle,
			Detail: "adult content was off and unblurred at once, so it stays hidden",
		})
	default:
		return "hidden", nil
	}
}

func handleIsAllDigits(handle string) bool {
	for _, char := range handle {
		if char < '0' || char > '9' {
			return false
		}
	}
	return handle != ""
}

var accountColumnHoldsValue = map[string]func(v1Account) bool{
	"users.refresh_token": func(row v1Account) bool { return row.RefreshToken != "" },
	"users.banned":        func(row v1Account) bool { return row.Banned },
	"users.banned_at":     func(row v1Account) bool { return row.BannedAt != nil },
	"users.banned_reason": func(row v1Account) bool { return row.BannedReason != "" },
	"users.banned_by":     func(row v1Account) bool { return row.BannedBy != nil },
}

func recordDroppedColumns(rows []v1Account, ledger *Ledger) error {
	for _, column := range accountColumns() {
		if column.Disposition != format.ColumnDropped {
			continue
		}
		name := column.Table + "." + column.Column
		holdsValue, known := accountColumnHoldsValue[name]
		if !known {
			return fmt.Errorf("dropped column %s has no way to count what it held", name)
		}
		held := 0
		for _, row := range rows {
			if holdsValue(row) {
				held++
			}
		}
		if err := ledger.Raise(Exception{
			Kind: "dropped_column", Subject: name,
			Detail: fmt.Sprintf("%s; %d of %d accounts held a value", column.Reason, held, len(rows)),
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeAccount(ctx context.Context, queries *db.Queries, one account) error {
	if err := queries.InsertMigratedUser(ctx, db.InsertMigratedUserParams{
		ID:                             pgtype.UUID{Bytes: one.ID, Valid: true},
		Username:                       one.Handle,
		Role:                           one.Role,
		CreatedAt:                      pgtype.Timestamptz{Time: one.CreatedAt, Valid: true},
		DisplayName:                    one.DisplayName,
		CustomDisplayName:              one.CustomName,
		AvatarUrl:                      one.AvatarURL,
		BannerUrl:                      one.BannerURL,
		NsfwVisibility:                 one.Visibility,
		ShowNsfwContributionsOnProfile: one.ShowOnProfile,
		DefaultIncludeTags:             one.IncludeTags,
		DefaultExcludeTags:             one.ExcludeTags,
	}); err != nil {
		return fmt.Errorf("migrate the account %s: %w", one.Handle, err)
	}
	if err := queries.InsertMigratedDiscordIdentity(ctx, db.InsertMigratedDiscordIdentityParams{
		UserID:  pgtype.UUID{Bytes: one.ID, Valid: true},
		Subject: one.DiscordSubject,
	}); err != nil {
		return fmt.Errorf("migrate the Discord identity for %s: %w", one.Handle, err)
	}
	return nil
}

func reconcileAccounts(
	ctx context.Context,
	queries *db.Queries,
	carried []account,
	ledger *Ledger,
) (AccountReport, error) {
	migrated, err := queries.MigratedAccounts(ctx)
	if err != nil {
		return AccountReport{}, fmt.Errorf("read the migrated accounts: %w", err)
	}
	if len(migrated) != len(carried) {
		return AccountReport{}, ledger.Raise(Exception{
			Kind: "count_mismatch", Subject: "users",
			Detail: fmt.Sprintf("%d of %d accounts arrived", len(migrated), len(carried)),
		})
	}
	wanted := make(map[uuid.UUID]account, len(carried))
	for _, one := range carried {
		wanted[one.ID] = one
	}
	report := AccountReport{
		Accounts: len(migrated), Roles: map[string]int{}, Visibility: map[string]int{},
		Exceptions: ledger.Entries(),
	}
	for _, row := range migrated {
		one, found := wanted[uuid.UUID(row.ID.Bytes)]
		if !found {
			return AccountReport{}, ledger.Raise(Exception{
				Kind: "account_mismatch", Subject: row.Username,
				Detail: "the migrated account came from no v1 row",
			})
		}
		if difference := accountDifference(one, row); difference != "" {
			return AccountReport{}, ledger.Raise(Exception{
				Kind: "account_mismatch", Subject: one.Handle, Detail: difference,
			})
		}
		report.Roles[row.Role]++
		report.Visibility[row.NsfwVisibility]++
	}
	return report, nil
}

func accountDifference(one account, row db.MigratedAccountsRow) string {
	switch {
	case row.Username != one.Handle:
		return fmt.Sprintf("handle arrived as %q", row.Username)
	case row.Role != one.Role:
		return fmt.Sprintf("role arrived as %q", row.Role)
	case !row.CreatedAt.Time.Equal(one.CreatedAt):
		return fmt.Sprintf("creation time arrived as %s", row.CreatedAt.Time)
	case row.DisplayName != one.DisplayName:
		return fmt.Sprintf("display name arrived as %q", row.DisplayName)
	case row.CustomDisplayName != one.CustomName:
		return fmt.Sprintf("creator-set display name arrived as %q", row.CustomDisplayName)
	case row.AvatarUrl != one.AvatarURL:
		return fmt.Sprintf("avatar arrived as %q", row.AvatarUrl)
	case row.BannerUrl != one.BannerURL:
		return fmt.Sprintf("banner arrived as %q", row.BannerUrl)
	case row.NsfwVisibility != one.Visibility:
		return fmt.Sprintf("NSFW visibility arrived as %q", row.NsfwVisibility)
	case row.ShowNsfwContributionsOnProfile != one.ShowOnProfile:
		return fmt.Sprintf("profile listing preference arrived as %v", row.ShowNsfwContributionsOnProfile)
	case !sameJSON(row.DefaultIncludeTags, one.IncludeTags):
		return fmt.Sprintf("include tags arrived as %s", row.DefaultIncludeTags)
	case !sameJSON(row.DefaultExcludeTags, one.ExcludeTags):
		return fmt.Sprintf("exclude tags arrived as %s", row.DefaultExcludeTags)
	case !row.DiscordSubject.Valid || row.DiscordSubject.String != one.DiscordSubject:
		return fmt.Sprintf("Discord subject arrived as %q", row.DiscordSubject.String)
	case row.Email.Valid || row.EmailSource.Valid || row.EmailVerifiedAt.Valid:
		return "an email address arrived, and none is migrated"
	case row.PasswordHash.Valid:
		return "a password arrived, and none is migrated"
	}
	return ""
}

func sameJSON(left, right []byte) bool {
	var first, second any
	if err := json.Unmarshal(left, &first); err != nil {
		return false
	}
	if err := json.Unmarshal(right, &second); err != nil {
		return false
	}
	return reflect.DeepEqual(first, second)
}
