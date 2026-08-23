package v1

import "github.com/Sillyfrogster/Illarin/api/internal/format"

func v1Columns() []format.ColumnDisposition {
	columns := make([]format.ColumnDisposition, 0, 220)
	base := func(table, description string, tags bool) {
		columns = append(columns,
			format.MappedColumn(table, "id", "asset id"),
			format.MappedColumn(table, "owner_id", "owner account"),
			format.MappedColumn(table, "name", "header name"),
			format.MappedColumn(table, "description", description),
			format.MappedColumn(table, "image_path", "cover or recovery source reference"),
			format.MappedColumn(table, "is_nsfw", "catalog adult-content answer"),
			format.MappedColumn(table, "created_at", "asset creation and initial content update"),
			format.PreservedColumn(table, "updated_at", "legacy record"),
			format.PreservedColumn(table, "downloads", "legacy record"),
			format.PreservedColumn(table, "views", "legacy record"),
			format.DroppedColumn(table, "favorites", "the recomputed count is not stored beside the rows it counts"),
			format.DroppedColumn(table, "hidden", "all migrated assets remain published"),
			format.DroppedColumn(table, "status", "moderation is out of scope"),
			format.DroppedColumn(table, "moderation_score", "moderation is out of scope"),
			format.DroppedColumn(table, "moderation_reason", "moderation is out of scope"),
			format.DroppedColumn(table, "moderation_completed_at", "moderation is out of scope"),
			format.DroppedColumn(table, "moderation_reviewed_at", "moderation is out of scope"),
			format.DroppedColumn(table, "moderation_reviewed_by", "moderation is out of scope"),
			format.DroppedColumn(table, "moderation_revision", "moderation is out of scope"),
			format.DroppedColumn(table, "comments_count", "the derived discussion count is out of scope"),
			format.DroppedColumn(table, "quality_score", "quality scoring is out of scope"),
			format.DroppedColumn(table, "quality_tier", "quality scoring is out of scope"),
			format.DroppedColumn(table, "quality_breakdown", "quality scoring is out of scope"),
			format.DroppedColumn(table, "quality_version", "quality scoring is out of scope"),
			format.DroppedColumn(table, "quality_completed_at", "quality scoring is out of scope"),
		)
		if tags {
			columns = append(columns, format.MappedColumn(table, "tags", "catalog tags verbatim"))
		}
	}

	base("characters", "description role", true)
	columns = append(columns,
		format.MappedColumn("characters", "nickname", "header nickname"),
		format.MappedColumn("characters", "personality", "personality role"),
		format.MappedColumn("characters", "scenario", "scenario role"),
		format.MappedColumn("characters", "first_mes", "greetings role"),
		format.MappedColumn("characters", "alternate_greetings", "greetings role"),
		format.MappedColumn("characters", "group_only_greetings", "group greetings role"),
		format.MappedColumn("characters", "mes_example", "example dialogue role"),
		format.MappedColumn("characters", "creator", "header credited author"),
		format.MappedColumn("characters", "creator_notes", "creator notes role"),
		format.DroppedColumn("characters", "creator_notes_multilingual", "empty on the migration corpus"),
		format.MappedColumn("characters", "character_version", "header asset version"),
		format.MappedColumn("characters", "system_prompt", "system prompt role"),
		format.MappedColumn("characters", "post_history_instructions", "post-history instructions role"),
		format.DroppedColumn("characters", "source", "empty and unable to prove a file origin"),
		format.PreservedColumn("characters", "assets", "v1 character remainder unless gallery names reconcile"),
		format.MappedColumn("characters", "character_book", "lorebook entries role with per-entry remainder"),
		format.PreservedColumn("characters", "extensions", "per-namespace remainder after display state is lifted"),
		format.PreservedColumn("characters", "creation_date", "v1 character remainder"),
		format.PreservedColumn("characters", "modification_date", "v1 character remainder"),
		format.MappedColumn("characters", "tagline", "header blurb"),
	)

	columns = append(columns,
		format.MappedColumn("character_images", "id", "source media id"),
		format.MappedColumn("character_images", "character_id", "owning character asset"),
		format.MappedColumn("character_images", "image_type", "cover, expressions or gallery role"),
		format.MappedColumn("character_images", "label", "image item name"),
		format.MappedColumn("character_images", "file_path", "source media path"),
		format.MappedColumn("character_images", "mime_type", "source media type"),
		format.MappedColumn("character_images", "file_size", "source media byte size"),
		format.MappedColumn("character_images", "sort_order", "image item position"),
		format.DroppedColumn("character_images", "created_at", "asset content uses the character's creation time"),
	)

	base("worldbooks", "header blurb", true)
	columns = append(columns,
		format.MappedColumn("worldbooks", "entries", "lorebook entries role with per-entry remainder"),
		format.MappedColumn("worldbooks", "creator", "header credited author"),
	)

	base("presets", "header blurb or usage block", true)
	columns = append(columns,
		format.DroppedColumn("presets", "settings", "empty dead v1 settings object"),
		format.MappedColumn("presets", "preset", "preset semantic roles and remainder"),
		format.DroppedColumn("presets", "schema_version", "v1 bookkeeping for the replaced model"),
		format.DroppedColumn("presets", "compatibility", "v1 bookkeeping for the replaced model"),
		format.MappedColumn("presets", "latest_version", "header asset version"),
	)

	for _, column := range []string{
		"id", "preset_id", "version", "snapshot", "blocks_added", "blocks_removed",
		"variables_added", "variables_removed", "block_count", "variable_count",
		"created_by", "created_at",
	} {
		columns = append(columns, format.PreservedColumn("preset_versions", column, "asset-bound version record"))
	}
	columns = append(columns,
		format.MappedColumn("preset_versions", "changelog", "changelog block and asset-bound version record"),
	)
	for _, column := range []string{
		"id", "preset_id", "version", "block_key", "content", "content_sha256",
		"created_by", "created_at", "updated_at",
	} {
		columns = append(columns, format.PreservedColumn("preset_sealed_blocks", column, "asset-bound sealed record"))
	}

	base("themes", "header blurb", true)
	columns = append(columns,
		format.DroppedColumn("themes", "colors", "empty dead v1 palette field"),
		format.MappedColumn("themes", "config", "theme tokens, controls and remainder"),
		format.DroppedColumn("themes", "schema_version", "v1 bookkeeping for the replaced model"),
		format.DroppedColumn("themes", "compatibility", "v1 bookkeeping for the replaced model"),
		format.MappedColumn("themes", "custom_css", "stylesheets role"),
		format.MappedColumn("themes", "asset_bundle_id", "generated bundle font lookup"),
	)

	base("dlc_packs", "header blurb", false)
	columns = append(columns,
		format.MappedColumn("dlc_packs", "pack_author", "header credited author"),
		format.MappedColumn("dlc_packs", "cover_url", "external cover reference"),
		format.MappedColumn("dlc_packs", "pack_version", "header asset version"),
		format.DroppedColumn("dlc_packs", "pack_extras", "empty on the migration corpus"),
		format.MappedColumn("dlc_packs", "lumia_items", "pack items role and external image references"),
		format.DroppedColumn("dlc_packs", "loom_items", "empty on the migration corpus"),
		format.DroppedColumn("dlc_packs", "pack_type", "every row restates the Lumia pack kind"),
		format.DroppedColumn("dlc_packs", "loom_tools", "empty on the migration corpus"),
	)
	return columns
}
