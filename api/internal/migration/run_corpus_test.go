package migration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format/modules"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTheRealCorpusBecomesTheIllarinCatalog(t *testing.T) {
	ctx := context.Background()
	target := testdb.Connect(t)
	source := restoredV1Dump(t)
	settings := migrationSettings(t, source, target)

	aborted, err := Run(ctx, settings)
	if err == nil {
		t.Fatal("the migration committed with no accounts to own the assets")
	}
	if aborted.Staging.Stored == 0 {
		t.Fatal("the aborted run reports staging nothing")
	}
	assertNothingCommitted(t, target)
	if staged := countRows(t, target, "migration_staged_media"); staged == 0 {
		t.Fatal("phase one staged nothing before the fatal")
	}

	if _, err := MigrateAccounts(ctx, source, target); err != nil {
		t.Fatalf("migrate the accounts the assets belong to: %v", err)
	}
	report, err := Run(ctx, settings)
	if err != nil {
		t.Fatalf("migrate the real corpus: %v", err)
	}
	if report.Staging.Stored != 0 || report.Staging.Fetched != 0 {
		t.Errorf(
			"the second run stored %d images and fetched %d, want nothing",
			report.Staging.Stored, report.Staging.Fetched,
		)
	}

	assertExpectedShape(t, target, report)
	assertLorebookKeysSurvive(t, source, target)
	assertBlocksMatchTheirCatalog(t, target)
	assertContentPlaced(t, target)
	assertShortfallIsPublishedAndMarked(t, target, report)
	assertRowTextWon(t, source, target)
	assertNoDisplayNamespacesSurvive(t, target)
	assertReturningPresetActivated(t, source, target, settings.Assets)
	assertLedgerNamesEveryDiscrepancy(t, target, report)
	assertOldAddressesResolve(t, target, report)
}

func TestAChangedCurrentSealedSourceAbortsBeforeCommit(t *testing.T) {
	ctx := context.Background()
	source := restoredV1Dump(t)
	target := testdb.Connect(t)
	settings := migrationSettings(t, source, target)
	if _, err := source.Exec(ctx, `create table sealed_source_backup as table preset_sealed_blocks`); err != nil {
		t.Fatal("copy the sealed source rows inside the scratch database")
	}

	matchingRow := `
		select sealed.id
		  from preset_sealed_blocks sealed
		  join presets preset on preset.id = sealed.preset_id
		 where sealed.version = preset.latest_version
		   and position('{{presetBlock::' || sealed.block_key || '}}' in preset.preset::text) > 0
		 order by sealed.id
		 limit 1`
	for _, test := range []struct {
		name   string
		change string
	}{
		{name: "missing row", change: `delete from preset_sealed_blocks where id = (` + matchingRow + `)`},
		{name: "duplicate key", change: `
			update presets preset
			   set preset = jsonb_set(
			       preset.preset,
			       '{blocks}',
			       preset.preset -> 'blocks' || jsonb_build_array((
			           select block
			             from jsonb_array_elements(preset.preset -> 'blocks') block
			            where block ->> 'content' like '{{presetBlock::%}}'
			            limit 1
			       ))
			   )
			 where preset.preset::text like '%{{presetBlock::%'
			   and preset.id = (select preset_id from preset_sealed_blocks where id = (` + matchingRow + `))`},
		{name: "mismatched key", change: `
			update preset_sealed_blocks set block_key = block_key || '-mismatch'
			 where id = (` + matchingRow + `)`},
		{name: "mismatched digest", change: `
			update preset_sealed_blocks set content_sha256 = repeat('0', 64)
			 where id = (` + matchingRow + `)`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := source.Exec(ctx, `
				truncate preset_sealed_blocks;
				insert into preset_sealed_blocks select * from sealed_source_backup;
			`); err != nil {
				t.Fatal("restore the sealed source rows inside the scratch database")
			}
			if _, err := source.Exec(ctx, test.change); err != nil {
				t.Fatal("change one sealed source row inside the scratch database")
			}
			if _, err := Run(ctx, settings); err == nil || !strings.Contains(err.Error(), "current sealed prompt") {
				t.Fatal("migration did not reject the changed sealed source before commit")
			}
			assertNothingCommitted(t, target)
		})
	}
}

func assertExpectedShape(t *testing.T, target *pgxpool.Pool, report Report) {
	t.Helper()
	if report.Assets != 152 {
		t.Errorf("migrated %d assets, want 152", report.Assets)
	}
	want := map[string]int{"character": 121, "theme": 11, "preset": 9, "lorebook": 2, "pack": 9}
	for kind, count := range want {
		if report.Kinds[kind] != count {
			t.Errorf("%s assets = %d, want %d", kind, report.Kinds[kind], count)
		}
	}
	if len(report.Kinds) != len(want) {
		t.Errorf("migrated kinds = %v, want exactly %v", report.Kinds, want)
	}
	if got := countRows(t, target, "assets"); got != 152 {
		t.Errorf("asset rows = %d, want 152", got)
	}
	if got := countRows(t, target, "asset_revisions"); got != 0 {
		t.Errorf("revision rows = %d, want none: the row is the source", got)
	}
	roles := roleCounts(t, target)
	if roles["expression"] != 195 || roles["gallery"] != 125 || roles["avatar"] != 65 {
		t.Errorf(
			"media = %d expressions, %d gallery and %d avatars, want 195, 125 and 65",
			roles["expression"], roles["gallery"], roles["avatar"],
		)
	}
	if roles["pack_item"] != 51 {
		t.Errorf("pack item images = %d, want 51", roles["pack_item"])
	}
	var covers, published, listed int
	if err := target.QueryRow(context.Background(),
		`select count(*) filter (where cover_media_id is not null),
		        count(*) filter (where lifecycle = 'published'),
		        count(*) filter (where discovery = 'listed')
		   from assets`).Scan(&covers, &published, &listed); err != nil {
		t.Fatalf("read the migrated catalog: %v", err)
	}
	if covers != 65 {
		t.Errorf("assets with a cover = %d, want 65", covers)
	}
	if published != 152 || listed != 152 {
		t.Errorf("published = %d and listed = %d, want 152 of each", published, listed)
	}
	if got := countRows(t, target, "migration_legacy_counters"); got != 152 {
		t.Errorf("legacy counter rows = %d, want 152", got)
	}
	var cutovers int
	if err := target.QueryRow(context.Background(),
		`select count(distinct migrated_at) from migration_legacy_counters`).Scan(&cutovers); err != nil {
		t.Fatalf("read the cutover stamps: %v", err)
	}
	if cutovers != 1 {
		t.Errorf("the legacy records carry %d cutover stamps, want one for the whole run", cutovers)
	}
	var generations int
	if err := target.QueryRow(context.Background(),
		`select count(*) from assets where content_generation = 1`).Scan(&generations); err != nil {
		t.Fatalf("read the content generations: %v", err)
	}
	if generations != 152 {
		t.Errorf("assets at content generation 1 = %d, want 152", generations)
	}
}

func assertContentPlaced(t *testing.T, target *pgxpool.Pool) {
	t.Helper()
	entries, fragments, records := 0, 0, 0
	for _, holder := range everyBlock(t, target) {
		for _, element := range holder.Elements {
			switch content := element.Content.(type) {
			case block.EntryTable:
				if element.Role == block.RoleLorebookEntries {
					entries += len(content.Entries)
				}
			case block.PromptList:
				fragments += len(content.Groups) + len(content.Fragments)
			case block.RecordList:
				records += len(content.Records)
			}
		}
	}
	if fragments != 817 || records != 76 {
		t.Errorf("placed %d prompt fragments and %d lumia records, want 817 and 76",
			fragments, records)
	}
	if entries < 342 {
		t.Errorf("placed %d lorebook entries, want at least the 342 the two books hold", entries)
	}
}

// assertLorebookKeysSurvive counts against the source rather than a number written here.
func assertLorebookKeysSurvive(t *testing.T, source, target *pgxpool.Pool) {
	t.Helper()
	var wanted int
	if err := source.QueryRow(context.Background(), `
		select count(*) from worldbooks book, lateral jsonb_array_elements(book.entries) entry
		 where jsonb_array_length(coalesce(entry->'key', '[]'::jsonb)) > 0`).Scan(&wanted); err != nil {
		t.Fatalf("count the v1 lorebook keys: %v", err)
	}
	if wanted == 0 {
		t.Fatal("no v1 lorebook entry carries a key, so the check proves nothing")
	}
	keyed := 0
	rows, err := target.Query(context.Background(),
		`select id from assets where kind = 'lorebook' order by id`)
	if err != nil {
		t.Fatalf("read the migrated lorebooks: %v", err)
	}
	defer rows.Close()
	books := make([]uuid.UUID, 0, 2)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("read a migrated lorebook: %v", err)
		}
		books = append(books, id)
	}
	for _, id := range books {
		for _, holder := range blocksOf(t, target, id) {
			for _, element := range holder.Elements {
				table, ok := element.Content.(block.EntryTable)
				if !ok {
					continue
				}
				for _, entry := range table.Entries {
					if len(entry.Keys) > 0 {
						keyed++
					}
				}
			}
		}
	}
	if keyed != wanted {
		t.Errorf("%d migrated lorebook entries carry keys, want the %d the rows held", keyed, wanted)
	}
}

func assertBlocksMatchTheirCatalog(t *testing.T, target *pgxpool.Pool) {
	t.Helper()
	rows, err := target.Query(context.Background(),
		`select id, kind from assets order by id`)
	if err != nil {
		t.Fatalf("read the migrated assets: %v", err)
	}
	defer rows.Close()
	type migrated struct {
		ID   uuid.UUID
		Kind string
	}
	assets := make([]migrated, 0, 152)
	for rows.Next() {
		var one migrated
		if err := rows.Scan(&one.ID, &one.Kind); err != nil {
			t.Fatalf("read a migrated asset: %v", err)
		}
		assets = append(assets, one)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the migrated assets: %v", err)
	}
	for _, one := range assets {
		definitions, known := block.Catalog(one.Kind)
		if !known {
			t.Fatalf("the %s catalog is absent", one.Kind)
		}
		order := make(map[block.DefinitionID]int, len(definitions))
		required := make(map[block.DefinitionID]bool, len(definitions))
		for at, definition := range definitions {
			order[definition.ID] = at
			required[definition.ID] = definition.Required
		}
		last := -1
		for at, holder := range blocksOf(t, target, one.ID) {
			position, placed := order[holder.Definition]
			if !placed || position <= last || holder.Position != at {
				t.Fatalf("a %s page is not in its catalog's default order", one.Kind)
			}
			last = position
			delete(required, holder.Definition)
		}
		for definition, missing := range required {
			if missing {
				t.Fatalf("a %s page has no %s block", one.Kind, definition)
			}
		}
	}
}

func assertShortfallIsPublishedAndMarked(t *testing.T, target *pgxpool.Pool, report Report) {
	t.Helper()
	if report.BelowFloor != 5 {
		t.Errorf("assets below the publish floor = %d, want 5", report.BelowFloor)
	}
	marked := make(map[uuid.UUID]struct{})
	for _, entry := range report.Exceptions {
		if entry.Kind == "below_publish_floor" && entry.AssetID != nil {
			marked[*entry.AssetID] = struct{}{}
		}
	}
	if len(marked) != 5 {
		t.Errorf("the ledger marks %d assets below the floor, want 5", len(marked))
	}
	for id := range marked {
		var kind, name, lifecycle string
		var isNSFW *bool
		if err := target.QueryRow(context.Background(),
			`select kind, name, is_nsfw, lifecycle from assets where id = $1`, id,
		).Scan(&kind, &name, &isNSFW, &lifecycle); err != nil {
			t.Fatalf("read a marked asset: %v", err)
		}
		if lifecycle != "published" {
			t.Error("an asset below the floor did not arrive published")
		}
		if kind != "character" {
			t.Errorf("a %s asset fell below the floor, and only characters should", kind)
		}
		if len(asset.MigratedShortfall(kind, name, isNSFW, blocksOf(t, target, id))) == 0 {
			t.Error("a marked asset reads as meeting the floor")
		}
	}
}

// assertRowTextWon proves the surviving cards changed nothing except the one greeting the allowlist recovers.
func assertRowTextWon(t *testing.T, source, target *pgxpool.Pool) {
	t.Helper()
	rows, err := source.Query(context.Background(),
		`select id, description, first_mes from characters order by id`)
	if err != nil {
		t.Fatalf("read the v1 character text: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var description, greeting string
		if err := rows.Scan(&id, &description, &greeting); err != nil {
			t.Fatalf("read a v1 character's text: %v", err)
		}
		migrated := blocksOf(t, target, id)
		if got := prose(migrated, block.RoleDescription); got != description {
			t.Fatal("a character's description is not the one its row held")
		}
		if got := firstGreeting(migrated); greeting != "" && got != greeting {
			t.Fatal("a character's first message is not the one its row held")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the v1 character text: %v", err)
	}
}

func assertNoDisplayNamespacesSurvive(t *testing.T, target *pgxpool.Pool) {
	t.Helper()
	lifted := v1.LiftedNamespaces()
	var namespaced int
	if err := target.QueryRow(context.Background(),
		`select count(*) from asset_preserved_data where namespace = any($1)`, lifted,
	).Scan(&namespaced); err != nil {
		t.Fatalf("read the preserved namespaces: %v", err)
	}
	if namespaced != 0 {
		t.Errorf("%d preserved rows are LumiHub's own display state", namespaced)
	}
	rows, err := target.Query(context.Background(),
		`select payload::text from asset_preserved_data`)
	if err != nil {
		t.Fatalf("read the preserved payloads: %v", err)
	}
	defer rows.Close()
	duplicated := 0
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			t.Fatalf("read a preserved payload: %v", err)
		}
		for _, namespace := range lifted {
			if strings.Contains(payload, `"`+namespace+`"`) {
				t.Fatal("a LumiHub display namespace survived inside a preserved payload")
			}
		}
		if strings.Contains(payload, `"world_books"`) {
			duplicated++
		}
	}
	if duplicated != 0 {
		t.Errorf("%d preserved payloads hold a second frozen lorebook", duplicated)
	}
}

func assertReturningPresetActivated(
	t *testing.T,
	source, target *pgxpool.Pool,
	assets *asset.Service,
) {
	t.Helper()
	const activePrompts = 85
	current := map[string]string{}
	rows, err := source.Query(context.Background(), `
		select sealed.block_key, sealed.content_sha256
		  from preset_sealed_blocks sealed
		  join presets preset on preset.id = sealed.preset_id
		 where sealed.version = preset.latest_version`)
	if err != nil {
		t.Fatal("read the current sealed source rows")
	}
	defer rows.Close()
	for rows.Next() {
		var key, digest string
		if err := rows.Scan(&key, &digest); err != nil {
			t.Fatal("read a current sealed source row")
		}
		if _, duplicate := current[key]; duplicate {
			t.Fatal("a current sealed source key is not unique")
		}
		current[key] = digest
	}
	if err := rows.Err(); err != nil {
		t.Fatal("the current sealed source rows did not finish")
	}
	if len(current) < activePrompts {
		t.Fatalf("current sealed source rows = %d, want at least %d", len(current), activePrompts)
	}

	var assetID uuid.UUID
	if err := target.QueryRow(context.Background(), `
		select asset_id
		  from protected_content
		 group by asset_id
		having count(*) = $1`, activePrompts).Scan(&assetID); err != nil {
		t.Fatalf("find the activated preset: %v", err)
	}
	stubs := map[uuid.UUID]bool{}
	for _, holder := range blocksOf(t, target, assetID) {
		for _, element := range holder.Elements {
			list, ok := element.Content.(block.PromptList)
			if !ok {
				continue
			}
			for _, fragment := range list.Fragments {
				if !fragment.Protected {
					continue
				}
				if fragment.Text != "" {
					t.Fatal("an activated prompt left private text in its public fragment")
				}
				stubs[fragment.ID] = true
			}
		}
	}
	if len(stubs) != activePrompts {
		t.Fatalf("protected stubs = %d, want %d", len(stubs), activePrompts)
	}

	rows, err = target.Query(context.Background(), `
		select owner_id, source_key, encode(digest, 'hex'), payload ->> 'text'
		  from protected_content
		 where asset_id = $1`, assetID)
	if err != nil {
		t.Fatalf("read the activated prompts: %v", err)
	}
	defer rows.Close()
	activated := 0
	privateTexts := make([]string, 0, activePrompts)
	for rows.Next() {
		var ownerID uuid.UUID
		var key, digest, text string
		if err := rows.Scan(&ownerID, &key, &digest, &text); err != nil {
			t.Fatal("read an activated prompt")
		}
		wantDigest, found := current[key]
		if !found || wantDigest != digest {
			t.Fatal("an activated prompt does not match its current source digest")
		}
		if !stubs[ownerID] {
			t.Fatal("an activated prompt is not owned by its public fragment")
		}
		if strings.HasPrefix(strings.TrimSpace(text), "{{presetBlock::") {
			t.Fatal("an activated prompt stored its placeholder as private text")
		}
		privateTexts = append(privateTexts, text)
		activated++
	}
	if err := rows.Err(); err != nil {
		t.Fatal("the activated prompts did not finish")
	}
	if activated != activePrompts {
		t.Fatalf("activated prompts = %d, want %d", activated, activePrompts)
	}

	var apps []string
	if err := target.QueryRow(context.Background(), `
		select array_agg(app order by app)
		  from protected_delivery_apps
		 where asset_id = $1`, assetID).Scan(&apps); err != nil {
		t.Fatalf("read the activated preset policy: %v", err)
	}
	if len(apps) != 1 || apps[0] != "lumiverse" {
		t.Errorf("initial allowed apps = %v, want Lumiverse", apps)
	}
	if _, err := assets.OpenExport(
		context.Background(), assetID, nil, "preset_lumiverse",
	); !errors.Is(err, asset.ErrLinkedInstallOnly) {
		t.Errorf("ordinary export error = %v, want linked-install-only", err)
	}
	linked, err := assets.OpenExportForLinkedInstance(
		context.Background(), assetID, "preset_lumiverse",
	)
	if err != nil {
		t.Fatalf("write the activated preset for a linked instance: %v", err)
	}
	if bytes.Contains(linked.Body, []byte("{{presetBlock::")) {
		t.Fatal("the linked artifact retained a sealed placeholder")
	}
	for _, privateText := range privateTexts {
		encoded, err := json.Marshal(privateText)
		if err != nil {
			t.Fatal("encode an activated prompt for comparison")
		}
		if !bytes.Contains(linked.Body, encoded) {
			t.Fatal("the linked artifact omitted an activated prompt")
		}
	}

	var sourceRows, archivedRows int
	if err := source.QueryRow(context.Background(),
		`select count(*) from preset_sealed_blocks`).Scan(&sourceRows); err != nil {
		t.Fatal("count the sealed source archive")
	}
	if err := target.QueryRow(context.Background(), `
		select count(*) from migration_preserved_records
		 where source_table = 'preset_sealed_blocks'`).Scan(&archivedRows); err != nil {
		t.Fatal("count the preserved sealed archive")
	}
	if archivedRows != sourceRows {
		t.Fatalf("archived sealed rows = %d, want all %d source rows", archivedRows, sourceRows)
	}
	var ownerID uuid.UUID
	if err := target.QueryRow(context.Background(),
		`select owner_id from assets where id = $1`, assetID).Scan(&ownerID); err != nil {
		t.Fatal("read the activated preset owner")
	}
	exported, err := assets.OpenSealedContent(context.Background(), ownerID, assetID)
	if err != nil {
		t.Fatalf("open the preserved sealed archive: %v", err)
	}
	if exported.Blocks != sourceRows {
		t.Errorf("archival export rows = %d, want %d", exported.Blocks, sourceRows)
	}
}

func assertLedgerNamesEveryDiscrepancy(t *testing.T, target *pgxpool.Pool, report Report) {
	t.Helper()
	counted := map[string]int{}
	for _, entry := range report.Exceptions {
		counted[entry.Kind]++
	}
	want := map[string]int{
		"below_publish_floor":          5,
		"missing_cover_file":           1,
		"missing_theme_status_colors":  1,
		"orphan_source_file":           40,
		"recovered_alternate_greeting": 1,
		"slug_collision":               1,
	}
	for kind, total := range want {
		if counted[kind] != total {
			t.Errorf("ledger entries of kind %s = %d, want %d", kind, counted[kind], total)
		}
	}
	for kind := range counted {
		if _, expected := want[kind]; !expected {
			t.Errorf("the ledger holds %d unexpected %s entries", counted[kind], kind)
		}
	}
	var stored int
	if err := target.QueryRow(context.Background(),
		`select count(*) from migration_exceptions where kind = any($1)`,
		keysOf(want)).Scan(&stored); err != nil {
		t.Fatalf("read the stored ledger: %v", err)
	}
	if stored != len(report.Exceptions) {
		t.Errorf("the ledger stored %d of %d entries", stored, len(report.Exceptions))
	}
	var recovered int
	if err := target.QueryRow(context.Background(),
		`select count(*) from migration_exceptions
		  where kind = 'recovered_alternate_greeting' and asset_id is not null`,
	).Scan(&recovered); err != nil {
		t.Fatalf("read the recovery entry: %v", err)
	}
	if recovered != 1 {
		t.Errorf("recovered greetings bound to an asset = %d, want 1", recovered)
	}
}

func assertOldAddressesResolve(t *testing.T, target *pgxpool.Pool, report Report) {
	t.Helper()
	if report.LegacyPaths != 151 {
		t.Errorf("stored v1 addresses = %d, want 151", report.LegacyPaths)
	}
	rows, err := target.Query(context.Background(), `
		select legacy.path, legacy.asset_id
		  from asset_legacy_paths legacy
		  join assets asset on asset.id = legacy.asset_id
		 where asset.lifecycle = 'published'`)
	if err != nil {
		t.Fatalf("read the stored v1 addresses: %v", err)
	}
	defer rows.Close()
	resolved := 0
	for rows.Next() {
		var path string
		var assetID uuid.UUID
		if err := rows.Scan(&path, &assetID); err != nil {
			t.Fatalf("read a stored v1 address: %v", err)
		}
		if !strings.Contains(path, "/") || strings.HasPrefix(path, "/") {
			t.Fatal("a stored v1 address is not an author and a name")
		}
		resolved++
	}
	if resolved != report.LegacyPaths {
		t.Errorf("%d of %d v1 addresses resolve to a published asset", resolved, report.LegacyPaths)
	}
}

func assertNothingCommitted(t *testing.T, target *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{
		"assets", "asset_blocks", "asset_media", "asset_legacy_paths",
		"protected_content", "protected_delivery_apps", "migration_legacy_counters",
		"migration_preserved_records", "migration_exceptions",
	} {
		if got := countRows(t, target, table); got != 0 {
			t.Errorf("%s holds %d rows after an aborted run, want none", table, got)
		}
	}
}

func migrationSettings(t *testing.T, source, target *pgxpool.Pool) Settings {
	t.Helper()
	blob, err := storage.NewStore(target, t.TempDir())
	if err != nil {
		t.Fatalf("open a blob store: %v", err)
	}
	registry, err := modules.Registry()
	if err != nil {
		t.Fatalf("build the format registry: %v", err)
	}
	backup, err := OpenFileBackup(repositoryFile(t, ".ai", "dump", "backup_folder.tar.gz"))
	if err != nil {
		t.Skip("the local v1 file backup is absent")
	}
	return Settings{
		Source: source, Target: target, Backup: backup,
		Assets:  asset.NewService(target, registry, blob),
		Fetcher: everyImageArrives{},
	}
}

// everyImageArrives stands in for the third-party hosts, because the addresses come from the dump and never leave it.
type everyImageArrives struct{}

func (everyImageArrives) Fetch(_ context.Context, _ string) (FetchedMedia, error) {
	return FetchedMedia{MediaType: "image/png", Body: onePixel}, nil
}

var onePixel = func() []byte {
	data, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	)
	if err != nil {
		panic(err)
	}
	return data
}()

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "select count(*) from "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func roleCounts(t *testing.T, pool *pgxpool.Pool) map[string]int {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`select role, count(*) from asset_media group by role`)
	if err != nil {
		t.Fatalf("count the migrated media: %v", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var role string
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			t.Fatalf("read a media count: %v", err)
		}
		counts[role] = count
	}
	return counts
}

func blocksOf(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID) []block.Block {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, definition, title, position, hidden, layout, width, elements
		  from asset_blocks where asset_id = $1 order by position`, assetID)
	if err != nil {
		t.Fatalf("read the migrated blocks: %v", err)
	}
	defer rows.Close()
	held := make([]block.Block, 0)
	for rows.Next() {
		var holder block.Block
		var title *string
		var elements []byte
		if err := rows.Scan(
			&holder.ID, &holder.Definition, &title, &holder.Position,
			&holder.Hidden, &holder.Layout, &holder.Width, &elements,
		); err != nil {
			t.Fatalf("read a migrated block: %v", err)
		}
		if err := json.Unmarshal(elements, &holder.Elements); err != nil {
			t.Fatalf("read a migrated block's elements: %v", err)
		}
		holder.Title = title
		held = append(held, holder)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the migrated blocks: %v", err)
	}
	return held
}

func everyBlock(t *testing.T, pool *pgxpool.Pool) []block.Block {
	t.Helper()
	ids, err := pool.Query(context.Background(), `select id from assets order by id`)
	if err != nil {
		t.Fatalf("read the migrated assets: %v", err)
	}
	defer ids.Close()
	held := make([]block.Block, 0)
	found := make([]uuid.UUID, 0, 152)
	for ids.Next() {
		var id uuid.UUID
		if err := ids.Scan(&id); err != nil {
			t.Fatalf("read a migrated asset: %v", err)
		}
		found = append(found, id)
	}
	for _, id := range found {
		held = append(held, blocksOf(t, pool, id)...)
	}
	return held
}

func prose(blocks []block.Block, role block.Role) string {
	for _, holder := range blocks {
		for _, element := range holder.Elements {
			if element.Role != role {
				continue
			}
			if text, ok := element.Content.(block.Prose); ok {
				return text.Text
			}
		}
	}
	return ""
}

func firstGreeting(blocks []block.Block) string {
	for _, holder := range blocks {
		for _, element := range holder.Elements {
			if element.Role != block.RoleGreetings {
				continue
			}
			set, ok := element.Content.(block.TextSet)
			if !ok || len(set.Texts) == 0 {
				return ""
			}
			return set.Texts[0].Text
		}
	}
	return ""
}

func keysOf(counted map[string]int) []string {
	kinds := make([]string, 0, len(counted))
	for kind := range counted {
		kinds = append(kinds, kind)
	}
	return kinds
}
