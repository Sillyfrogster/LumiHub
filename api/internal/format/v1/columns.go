package v1

import "github.com/Sillyfrogster/LumiHub/api/internal/format"

func v1Columns() []format.ColumnDisposition {
	columns := make([]format.ColumnDisposition, 0, 220)
	base := func(table, description string, tags bool) {
		columns = append(columns,
			mapped(table, "id", "asset id"),
			mapped(table, "owner_id", "owner account"),
			mapped(table, "name", "header name"),
			mapped(table, "description", description),
			mapped(table, "image_path", "cover or recovery source reference"),
			mapped(table, "is_nsfw", "catalog adult-content answer"),
			mapped(table, "created_at", "asset creation and initial content update"),
			preserved(table, "updated_at", "legacy record"),
			preserved(table, "downloads", "legacy record"),
			preserved(table, "views", "legacy record"),
			preserved(table, "favorites", "legacy record"),
			dropped(table, "hidden", "all migrated assets remain published"),
			dropped(table, "status", "moderation is out of scope"),
			dropped(table, "moderation_score", "moderation is out of scope"),
			dropped(table, "moderation_reason", "moderation is out of scope"),
			dropped(table, "moderation_completed_at", "moderation is out of scope"),
			dropped(table, "moderation_reviewed_at", "moderation is out of scope"),
			dropped(table, "moderation_reviewed_by", "moderation is out of scope"),
			dropped(table, "moderation_revision", "moderation is out of scope"),
			dropped(table, "comments_count", "the derived discussion count is out of scope"),
			dropped(table, "quality_score", "quality scoring is out of scope"),
			dropped(table, "quality_tier", "quality scoring is out of scope"),
			dropped(table, "quality_breakdown", "quality scoring is out of scope"),
			dropped(table, "quality_version", "quality scoring is out of scope"),
			dropped(table, "quality_completed_at", "quality scoring is out of scope"),
		)
		if tags {
			columns = append(columns, mapped(table, "tags", "catalog tags verbatim"))
		}
	}

	base("characters", "description role", true)
	columns = append(columns,
		mapped("characters", "nickname", "header nickname"),
		mapped("characters", "personality", "personality role"),
		mapped("characters", "scenario", "scenario role"),
		mapped("characters", "first_mes", "greetings role"),
		mapped("characters", "alternate_greetings", "greetings role"),
		mapped("characters", "group_only_greetings", "group greetings role"),
		mapped("characters", "mes_example", "example dialogue role"),
		mapped("characters", "creator", "header credited author"),
		mapped("characters", "creator_notes", "creator notes role"),
		dropped("characters", "creator_notes_multilingual", "empty on the migration corpus"),
		mapped("characters", "character_version", "header asset version"),
		mapped("characters", "system_prompt", "system prompt role"),
		mapped("characters", "post_history_instructions", "post-history instructions role"),
		dropped("characters", "source", "empty and unable to prove a file origin"),
		preserved("characters", "assets", "v1 character remainder unless gallery names reconcile"),
		mapped("characters", "character_book", "lorebook entries role with per-entry remainder"),
		preserved("characters", "extensions", "per-namespace remainder after display state is lifted"),
		preserved("characters", "creation_date", "v1 character remainder"),
		preserved("characters", "modification_date", "v1 character remainder"),
		mapped("characters", "tagline", "header blurb"),
	)

	columns = append(columns,
		mapped("character_images", "id", "source media id"),
		mapped("character_images", "character_id", "owning character asset"),
		mapped("character_images", "image_type", "cover, expressions or gallery role"),
		mapped("character_images", "label", "image item name"),
		mapped("character_images", "file_path", "source media path"),
		mapped("character_images", "mime_type", "source media type"),
		mapped("character_images", "file_size", "source media byte size"),
		mapped("character_images", "sort_order", "image item position"),
		dropped("character_images", "created_at", "asset content uses the character's creation time"),
	)

	base("worldbooks", "header blurb", true)
	columns = append(columns,
		mapped("worldbooks", "entries", "lorebook entries role with per-entry remainder"),
		mapped("worldbooks", "creator", "header credited author"),
	)

	base("presets", "header blurb or usage block", true)
	columns = append(columns,
		dropped("presets", "settings", "empty dead v1 settings object"),
		mapped("presets", "preset", "preset semantic roles and remainder"),
		dropped("presets", "schema_version", "v1 bookkeeping for the replaced model"),
		dropped("presets", "compatibility", "v1 bookkeeping for the replaced model"),
		mapped("presets", "latest_version", "header asset version"),
	)

	for _, column := range []string{
		"id", "preset_id", "version", "snapshot", "blocks_added", "blocks_removed",
		"variables_added", "variables_removed", "block_count", "variable_count",
		"created_by", "created_at",
	} {
		columns = append(columns, preserved("preset_versions", column, "asset-bound version record"))
	}
	columns = append(columns,
		mapped("preset_versions", "changelog", "changelog block and asset-bound version record"),
	)
	for _, column := range []string{
		"id", "preset_id", "version", "block_key", "content", "content_sha256",
		"created_by", "created_at", "updated_at",
	} {
		columns = append(columns, preserved("preset_sealed_blocks", column, "asset-bound sealed record"))
	}

	base("themes", "header blurb", true)
	columns = append(columns,
		dropped("themes", "colors", "empty dead v1 palette field"),
		mapped("themes", "config", "theme tokens, controls and remainder"),
		dropped("themes", "schema_version", "v1 bookkeeping for the replaced model"),
		dropped("themes", "compatibility", "v1 bookkeeping for the replaced model"),
		mapped("themes", "custom_css", "stylesheets role"),
		mapped("themes", "asset_bundle_id", "generated bundle font lookup"),
	)

	base("dlc_packs", "header blurb", false)
	columns = append(columns,
		mapped("dlc_packs", "pack_author", "header credited author"),
		mapped("dlc_packs", "cover_url", "external cover reference"),
		mapped("dlc_packs", "pack_version", "header asset version"),
		dropped("dlc_packs", "pack_extras", "empty on the migration corpus"),
		mapped("dlc_packs", "lumia_items", "pack items role and external image references"),
		dropped("dlc_packs", "loom_items", "empty on the migration corpus"),
		dropped("dlc_packs", "pack_type", "every row restates the Lumia pack kind"),
		dropped("dlc_packs", "loom_tools", "empty on the migration corpus"),
	)
	return columns
}

func mapped(table, column, destination string) format.ColumnDisposition {
	return format.ColumnDisposition{
		Table: table, Column: column, Disposition: format.ColumnMapped, Destination: destination,
	}
}

func preserved(table, column, destination string) format.ColumnDisposition {
	return format.ColumnDisposition{
		Table: table, Column: column, Disposition: format.ColumnPreserved, Destination: destination,
	}
}

func dropped(table, column, reason string) format.ColumnDisposition {
	return format.ColumnDisposition{
		Table: table, Column: column, Disposition: format.ColumnDropped, Reason: reason,
	}
}
