package migration

// TableDisposition is what one v1 table becomes, declared so nothing lost silently is checkable against the whole database.
type TableDisposition struct {
	Table   string
	Becomes TableOutcome
	Reason  string
}

type TableOutcome string

const (
	TableMapped    TableOutcome = "mapped"
	TablePreserved TableOutcome = "preserved"
	TableDeferred  TableOutcome = "deferred"
	TableDropped   TableOutcome = "dropped"
)

// v1Tables accounts for every table in the v1 database.
func v1Tables() []TableDisposition {
	return []TableDisposition{
		{"characters", TableMapped, "character assets"},
		{"themes", TableMapped, "theme assets"},
		{"presets", TableMapped, "preset assets"},
		{"worldbooks", TableMapped, "lorebook assets"},
		{"dlc_packs", TableMapped, "pack assets"},
		{"character_images", TableMapped, "covers, expressions and gallery items"},
		{"preset_versions", TablePreserved, "changelog blocks and asset-bound snapshots"},
		{"preset_sealed_blocks", TablePreserved, "asset-bound sealed records under ADR-0009"},
		{"favorites", TablePreserved, "relationships intact, so a saved library can be designed later"},
		{"comments", TablePreserved, "the only creator-written text on the deferred list"},
		{"comment_edits", TableDropped, "the table is empty"},
		{"users", TableDeferred, "account migration"},
		{"install_manifests", TableDropped, "each application reports its own library after relinking"},
		{"linked_instances", TableDropped, "every creator links again at cutover"},
		{"link_codes", TableDropped, "every creator links again at cutover"},
		{"account_identity_transfer_audit_log", TableDeferred, "account migration"},
		{"api_keys", TableDeferred, "account migration"},
		{"profile_assets", TableDeferred, "account migration"},
		{"blog_posts", TableDeferred, "the blog is not part of this effort"},
		{"asset_usage_daily", TableDropped, "the creator dashboard is out of scope"},
		{"moderation_audit_log", TableDropped, "moderation is out of scope"},
		{"nsfw_jobs", TableDropped, "moderation is out of scope"},
		{"nsfw_score_cache", TableDropped, "moderation is out of scope"},
		{"jobs", TableDropped, "transient queue state"},
		{"schema_migrations", TableDropped, "v1 bookkeeping"},
	}
}

// preservedTables names the v1 tables kept as records with no asset content of their own.
func preservedTables() []string {
	return []string{"favorites", "comments"}
}
