package migration

import "github.com/Sillyfrogster/Illarin/api/internal/format"

// accountColumns accounts for every column of v1's users table.
func accountColumns() []format.ColumnDisposition {
	return []format.ColumnDisposition{
		format.MappedColumn("users", "id", "account id"),
		format.MappedColumn("users", "discord_id", "Discord identity subject"),
		format.MappedColumn("users", "username", "handle"),
		format.MappedColumn("users", "banner", "profile banner URL"),
		format.MappedColumn("users", "display_name", "Discord display name"),
		format.MappedColumn("users", "avatar", "profile avatar URL"),
		format.MappedColumn("users", "role", "account role"),
		format.MappedColumn("users", "created_at", "account creation"),
		format.MappedColumn("users", "custom_display_name", "creator-set display name"),
		format.MappedColumn("users", "nsfw_enabled", "NSFW visibility"),
		format.MappedColumn("users", "nsfw_unblurred", "NSFW visibility"),
		format.MappedColumn("users", "default_include_tags", "default include tag preference"),
		format.MappedColumn("users", "default_exclude_tags", "default exclude tag preference"),
		format.MappedColumn("users", "show_nsfw_contributions_on_profile", "profile listing preference"),
		format.DroppedColumn("users", "refresh_token", "Illarin keeps no provider token and re-authorizes at sign-in"),
		format.DroppedColumn("users", "banned", "moderation is out of scope"),
		format.DroppedColumn("users", "banned_at", "moderation is out of scope"),
		format.DroppedColumn("users", "banned_reason", "moderation is out of scope"),
		format.DroppedColumn("users", "banned_by", "moderation is out of scope"),
	}
}

// accountAnomalies classifies everything the accounts migration can meet, ahead of the run.
func accountAnomalies() []format.AnomalyDeclaration {
	return []format.AnomalyDeclaration{
		{
			Kind: "dropped_column", Disposition: format.AnomalyTolerated,
			Reason: "the column is declared dropped before the run",
		},
		{
			Kind: "grandfathered_handle", Disposition: format.AnomalyTolerated,
			Reason: "an existing handle is never rewritten",
		},
		{
			Kind: "contradictory_nsfw_pair", Disposition: format.AnomalyTolerated,
			Reason: "the safer of the two answers stands",
		},
		{
			Kind: "undeclared_column", Disposition: format.AnomalyFatal,
			Reason: "silent source-data loss is forbidden",
		},
		{
			Kind: "stale_column_declaration", Disposition: format.AnomalyFatal,
			Reason: "a declaration that does not describe the source proves nothing",
		},
		{
			Kind: "count_mismatch", Disposition: format.AnomalyFatal,
			Reason: "every account must reconcile exactly",
		},
		{
			Kind: "account_mismatch", Disposition: format.AnomalyFatal,
			Reason: "a migrated account must match the row it came from",
		},
	}
}
