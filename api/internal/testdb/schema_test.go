package testdb

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCoreTablesExist(t *testing.T) {
	pool := Connect(t)

	want := []string{
		"assets",
		"asset_revisions",
		"asset_media",
		"asset_blocks",
		"asset_projections",
		"blobs",
		"blob_sweep_marks",
		"blob_tombstones",
		"users",
		"retired_handles",
		"email_verification_tokens",
		"oauth_identities",
		"oauth_states",
		"password_reset_tokens",
		"sessions",
	}
	for _, table := range want {
		var exists bool
		err := pool.QueryRow(context.Background(),
			`select exists (select 1 from information_schema.tables where table_name = $1)`,
			table).Scan(&exists)
		if err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s is missing", table)
		}
	}
}

func TestBlobRowsContainOnlyContentAndLocation(t *testing.T) {
	pool := Connect(t)
	rows, err := pool.Query(context.Background(),
		`select column_name
		   from information_schema.columns
		  where table_schema = 'public' and table_name = 'blobs'
		  order by ordinal_position`)
	if err != nil {
		t.Fatalf("read blob columns: %v", err)
	}
	columns, err := rowsToStrings(rows)
	if err != nil {
		t.Fatalf("scan blob columns: %v", err)
	}

	want := []string{"id", "sha256", "byte_size", "storage_key"}
	if !slices.Equal(columns, want) {
		t.Errorf("blob columns = %v, want %v", columns, want)
	}
}

func TestBlobDigestIsUnique(t *testing.T) {
	pool := Connect(t)
	digest := make([]byte, 32)

	_, err := pool.Exec(context.Background(),
		`insert into blobs (id, sha256, byte_size, storage_key)
		 values (gen_random_uuid(), $1, 1, 'first')`, digest)
	if err != nil {
		t.Fatalf("insert first blob: %v", err)
	}
	_, err = pool.Exec(context.Background(),
		`insert into blobs (id, sha256, byte_size, storage_key)
		 values (gen_random_uuid(), $1, 1, 'second')`, digest)
	if err == nil {
		t.Fatal("the same digest was stored twice")
	}
}

func TestSchemaHasNoMutableReferenceCount(t *testing.T) {
	pool := Connect(t)
	var count int
	if err := pool.QueryRow(context.Background(), `
		select count(*)
		  from information_schema.columns
		 where table_schema = 'public'
		   and lower(column_name) in ('reference_count', 'ref_count', 'references_count')
	`).Scan(&count); err != nil {
		t.Fatalf("inspect reference-count columns: %v", err)
	}
	if count != 0 {
		t.Fatalf("schema contains %d mutable reference-count columns", count)
	}
}

func TestIngestFailureReasonsStayAtTheClosedFive(t *testing.T) {
	pool := Connect(t)
	var definition string
	if err := pool.QueryRow(context.Background(), `
		select pg_get_constraintdef(oid)
		  from pg_constraint
		 where conname = 'ingest_operations_failure_reason_check'
	`).Scan(&definition); err != nil {
		t.Fatalf("read ingest failure reason constraint: %v", err)
	}
	for _, reason := range []string{
		"malformed_input", "unsupported_format", "unsupported_version",
		"safety_violation", "internal_failure",
	} {
		if !strings.Contains(definition, reason) {
			t.Errorf("ingest failure constraint is missing %q: %s", reason, definition)
		}
	}
	if strings.Contains(definition, "purged_content") {
		t.Errorf("ingest failure constraint added a public purge reason: %s", definition)
	}
}

func TestDurableRecordsReferenceBlobs(t *testing.T) {
	pool := Connect(t)

	for _, table := range []string{"asset_revisions", "asset_media"} {
		rows, err := pool.Query(context.Background(),
			`select column_name
			   from information_schema.columns
			  where table_schema = 'public' and table_name = $1`, table)
		if err != nil {
			t.Fatalf("read %s columns: %v", table, err)
		}
		columns, err := rowsToStrings(rows)
		if err != nil {
			t.Fatalf("scan %s columns: %v", table, err)
		}

		if !slices.Contains(columns, "blob_id") {
			t.Errorf("%s has no blob_id", table)
		}
		for _, forbidden := range []string{"content_hash", "byte_size", "storage_key", "sha256"} {
			if slices.Contains(columns, forbidden) {
				t.Errorf("%s still stores %s instead of a blob reference", table, forbidden)
			}
		}
	}
}

type stringRows interface {
	Next() bool
	Scan(...any) error
	Close()
	Err() error
}

func rowsToStrings(rows stringRows) ([]string, error) {
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func TestBlockRowsCarryOnlyWhatTheCreatorDid(t *testing.T) {
	pool := Connect(t)
	rows, err := pool.Query(context.Background(),
		`select column_name
		   from information_schema.columns
		  where table_schema = 'public' and table_name = 'asset_blocks'
		  order by ordinal_position`)
	if err != nil {
		t.Fatalf("read block columns: %v", err)
	}
	columns, err := rowsToStrings(rows)
	if err != nil {
		t.Fatalf("scan block columns: %v", err)
	}

	want := []string{
		"id", "asset_id", "definition", "title", "position", "hidden",
		"layout", "width", "elements",
	}
	if !slices.Equal(columns, want) {
		t.Errorf("block columns = %v, want %v", columns, want)
	}

	var elementTables int
	if err := pool.QueryRow(context.Background(), `
		select count(*)
		  from information_schema.tables
		 where table_schema = 'public' and table_name like '%element%'
	`).Scan(&elementTables); err != nil {
		t.Fatalf("look for element tables: %v", err)
	}
	if elementTables != 0 {
		t.Errorf("elements have %d tables of their own; the block is the unit of persistence",
			elementTables)
	}
}

func TestTwoBlocksOnOneAssetCannotShareAPosition(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()
	assetID := uuid.New()
	if _, err := pool.Exec(ctx,
		`insert into assets (id, kind, name, lifecycle)
		 values ($1, 'character', 'Ordered', 'draft')`, assetID); err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	insert := func(definition string) error {
		_, err := pool.Exec(ctx,
			`insert into asset_blocks (id, asset_id, definition, position, layout, width)
			 values (gen_random_uuid(), $1, $2, 0, 'single', 'full')`,
			assetID, definition)
		return err
	}
	if err := insert("character_core"); err != nil {
		t.Fatalf("insert first block: %v", err)
	}
	if err := insert("messages"); err == nil {
		t.Fatal("two blocks on one asset took the same position")
	}
}

func TestAssetKindIsClosedToKnownValues(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()

	for _, kind := range []string{"character", "lorebook", "preset", "theme", "pack"} {
		_, err := pool.Exec(ctx,
			`insert into assets (id, kind, name, lifecycle)
			 values (gen_random_uuid(), $1, $2, 'published')`,
			kind, kind)
		if err != nil {
			t.Errorf("insert kind %q: %v", kind, err)
		}
	}

	_, err := pool.Exec(ctx,
		`insert into assets (id, kind, name, lifecycle)
		 values (gen_random_uuid(), 'bundle', 'Bundle', 'published')`)
	if err == nil {
		t.Fatal("kind outside the catalog vocabulary was accepted")
	}
}

func TestDiscoveryDefaultsToListedAndRejectsUnknownValues(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()

	var discovery string
	err := pool.QueryRow(ctx,
		`insert into assets (id, kind, name, lifecycle)
		 values (gen_random_uuid(), 'character', 'Listed by default', 'published')
		 returning discovery`).Scan(&discovery)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if discovery != "listed" {
		t.Errorf("discovery = %q, want listed", discovery)
	}

	_, err = pool.Exec(ctx,
		`insert into assets (id, kind, name, discovery, lifecycle)
		 values (gen_random_uuid(), 'theme', 'Quiet', 'unlisted', 'published')`)
	if err != nil {
		t.Errorf("insert unlisted asset: %v", err)
	}

	_, err = pool.Exec(ctx,
		`insert into assets (id, kind, name, discovery, lifecycle)
		 values (gen_random_uuid(), 'theme', 'Unknown', 'private', 'published')`)
	if err == nil {
		t.Fatal("discovery outside listed and unlisted was accepted")
	}
}

func TestWithholdingFieldsPopulateTogether(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()

	assetID := uuid.New()
	actorID := uuid.New()
	_, err := pool.Exec(ctx,
		`insert into users (id, username) values ($1, 'withhold.actor')`, actorID)
	if err != nil {
		t.Fatalf("insert withhold actor: %v", err)
	}
	_, err = pool.Exec(ctx,
		`insert into assets (id, kind, name, lifecycle)
		 values ($1, 'character', 'Held', 'published')`, assetID)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	_, err = pool.Exec(ctx,
		`update assets
		    set withheld_at = now(), withheld_by = $2, withheld_reason = 'review'
		  where id = $1`, assetID, actorID)
	if err != nil {
		t.Fatalf("set complete withhold: %v", err)
	}

	partial := []struct {
		name string
		set  string
	}{
		{name: "time only", set: "withheld_at = now()"},
		{name: "actor only", set: "withheld_by = gen_random_uuid()"},
		{name: "reason only", set: "withheld_reason = 'review'"},
		{name: "without time", set: "withheld_by = gen_random_uuid(), withheld_reason = 'review'"},
		{name: "without actor", set: "withheld_at = now(), withheld_reason = 'review'"},
		{name: "without reason", set: "withheld_at = now(), withheld_by = gen_random_uuid()"},
	}
	for _, test := range partial {
		t.Run(test.name, func(t *testing.T) {
			id := uuid.New()
			_, err := pool.Exec(ctx,
				`insert into assets (id, kind, name, lifecycle)
				 values ($1, 'character', 'Partial', 'published')`, id)
			if err != nil {
				t.Fatalf("insert asset: %v", err)
			}
			_, err = pool.Exec(ctx, `update assets set `+test.set+` where id = $1`, id)
			if err == nil {
				t.Fatal("partial withhold was accepted")
			}
		})
	}
}

func TestOriginAndRevisionFormatsHaveSeparateHomes(t *testing.T) {
	pool := Connect(t)

	assetColumns, err := tableColumns(pool, "assets")
	if err != nil {
		t.Fatalf("read asset columns: %v", err)
	}
	for _, column := range []string{"format", "format_version", "platform", "publication"} {
		if slices.Contains(assetColumns, column) {
			t.Errorf("assets still has %s", column)
		}
	}
	for _, column := range []string{
		"kind", "discovery", "withheld_at", "withheld_by", "withheld_reason", "deleted_at", "origin_format",
	} {
		if !slices.Contains(assetColumns, column) {
			t.Errorf("assets has no %s", column)
		}
	}

	revisionColumns, err := tableColumns(pool, "asset_revisions")
	if err != nil {
		t.Fatalf("read revision columns: %v", err)
	}
	if !slices.Contains(revisionColumns, "format") {
		t.Error("asset_revisions has no format")
	}
	if slices.Contains(revisionColumns, "passthrough_platform") {
		t.Error("asset_revisions still has passthrough_platform")
	}
	if slices.Contains(revisionColumns, "format_version") {
		t.Error("asset_revisions still has format_version")
	}
}

func TestRevisionIdentityCanBackACompositeForeignKey(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `drop table if exists revision_reference_probe`)
	if err != nil {
		t.Fatalf("clear revision reference probe: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `drop table if exists revision_reference_probe`)
	})

	_, err = pool.Exec(ctx,
		`create table revision_reference_probe (
		    revision_id uuid not null,
		    asset_id uuid not null,
		    foreign key (revision_id, asset_id)
		        references asset_revisions (id, asset_id)
		)`)
	if err != nil {
		t.Fatalf("asset_revisions (id, asset_id) cannot back a composite foreign key: %v", err)
	}
}

func TestDownloadEventCarriesOnlyAuthorizedHandoffFacts(t *testing.T) {
	pool := Connect(t)
	assetID, revisionID, _ := insertAssetRevision(t, pool)

	columns, err := tableColumns(pool, "download_events")
	if err != nil {
		t.Fatalf("read download event columns: %v", err)
	}
	want := []string{
		"id",
		"asset_id",
		"revision_id",
		"export_target",
		"handed_off_at",
		"authorization_class",
		"discovery",
	}
	if !slices.Equal(columns, want) {
		t.Fatalf("download event columns = %v, want %v", columns, want)
	}

	_, err = pool.Exec(context.Background(), `
		insert into download_events
			(asset_id, revision_id, export_target, authorization_class, discovery)
		values ($1, $2, 'raw', 'anonymous', 'listed')
	`, assetID, revisionID)
	if err != nil {
		t.Fatalf("insert download event: %v", err)
	}
}

func TestDownloadEventsAreImmutable(t *testing.T) {
	pool := Connect(t)
	assetID, revisionID, _ := insertAssetRevision(t, pool)
	ctx := context.Background()

	var eventID int64
	err := pool.QueryRow(ctx, `
		insert into download_events
			(asset_id, revision_id, export_target, authorization_class, discovery)
		values ($1, $2, 'raw', 'anonymous', 'listed')
		returning id
	`, assetID, revisionID).Scan(&eventID)
	if err != nil {
		t.Fatalf("insert download event: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		update download_events set authorization_class = 'owner' where id = $1
	`, eventID); err == nil {
		t.Fatal("download event was updated")
	}
	if _, err := pool.Exec(ctx, `delete from download_events where id = $1`, eventID); err == nil {
		t.Fatal("download event was deleted")
	}
	if _, err := pool.Exec(ctx, `truncate download_events`); err == nil {
		t.Fatal("download events were truncated")
	}
}

func TestDownloadEventRevisionMustBelongToItsAsset(t *testing.T) {
	pool := Connect(t)
	firstAssetID, firstRevisionID, _ := insertAssetRevision(t, pool)
	secondAssetID, _, _ := insertAssetRevision(t, pool)

	_, err := pool.Exec(context.Background(), `
		insert into download_events
			(asset_id, revision_id, export_target, authorization_class, discovery)
		values ($1, $2, 'raw', 'anonymous', 'listed')
	`, secondAssetID, firstRevisionID)
	if err == nil {
		t.Fatalf("revision %s from asset %s was recorded for asset %s",
			firstRevisionID, firstAssetID, secondAssetID)
	}
}

func TestDownloadEventVocabularyIsClosed(t *testing.T) {
	pool := Connect(t)
	assetID, revisionID, _ := insertAssetRevision(t, pool)
	ctx := context.Background()

	for _, authorizationClass := range []string{
		"anonymous", "signed_in", "owner", "linked_instance",
	} {
		_, err := pool.Exec(ctx, `
			insert into download_events
				(asset_id, revision_id, export_target, authorization_class, discovery)
			values ($1, $2, 'raw', $3, 'listed')
		`, assetID, revisionID, authorizationClass)
		if err != nil {
			t.Fatalf("insert %s download event: %v", authorizationClass, err)
		}
	}

	for name, values := range map[string][3]string{
		"blank target":          {" ", "anonymous", "listed"},
		"unknown authorization": {"raw", "crawler", "listed"},
		"unknown discovery":     {"raw", "anonymous", "private"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx, `
				insert into download_events
					(asset_id, revision_id, export_target, authorization_class, discovery)
				values ($1, $2, $3, $4, $5)
			`, assetID, revisionID, values[0], values[1], values[2])
			if err == nil {
				t.Fatal("invalid download event was accepted")
			}
		})
	}
}

func TestTheProjectionCarriesTwoIndependentHalves(t *testing.T) {
	pool := Connect(t)
	assetID, _, _ := insertAssetRevision(t, pool)

	columns, err := tableColumns(pool, "asset_projections")
	if err != nil {
		t.Fatalf("read projection columns: %v", err)
	}
	want := []string{
		"asset_id", "export", "export_stamp", "export_computed_at",
		"facets", "facet_stamp", "facet_computed_at",
	}
	for _, column := range want {
		if !slices.Contains(columns, column) {
			t.Errorf("the projection has no %s column; it holds %v", column, columns)
		}
	}

	if _, err := pool.Exec(context.Background(),
		`insert into asset_projections (asset_id, facets, facet_stamp) values ($1, $2, 'stamp')`,
		assetID, `{"lorebook":3}`); err != nil {
		t.Fatalf("write only the facet half: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`insert into asset_projections (asset_id, facets, facet_stamp) values ($1, $2, 'stamp')`,
		uuid.New(), `{}`); err == nil {
		t.Fatal("a projection without an asset was accepted")
	}
	if _, err := pool.Exec(context.Background(), `
		update asset_projections set facets = '[]'::jsonb where asset_id = $1
	`, assetID); err == nil {
		t.Fatal("a facet half that is not an object was accepted")
	}
}

func TestMediaBelongsToOneAsset(t *testing.T) {
	pool := Connect(t)
	assetID, _, blobID := insertAssetRevision(t, pool)
	ctx := context.Background()

	columns, err := tableColumns(pool, "asset_media")
	if err != nil {
		t.Fatalf("read media columns: %v", err)
	}
	want := []string{
		"id", "asset_id", "role", "width", "height", "created_at", "blob_id",
		"is_extracted", "is_current",
	}
	if !slices.Equal(columns, want) {
		t.Fatalf("media columns = %v, want %v", columns, want)
	}

	roles := []string{
		"avatar", "expression", "gallery", "avatar_alt", "perspective_layer", "pack_item",
	}
	for _, role := range roles {
		_, err = pool.Exec(ctx,
			`insert into asset_media (id, asset_id, role, blob_id)
			 values (gen_random_uuid(), $1, $2, $3)`, assetID, role, blobID)
		if err != nil {
			t.Fatalf("insert %s media: %v", role, err)
		}
	}

	_, err = pool.Exec(ctx,
		`insert into asset_media (id, role, blob_id)
		 values (gen_random_uuid(), 'gallery', $1)`, blobID)
	if err == nil {
		t.Error("media without an asset was accepted")
	}
	_, err = pool.Exec(ctx,
		`insert into asset_media (id, asset_id, role, blob_id)
		 values (gen_random_uuid(), $1, 'gallery', $2)`, uuid.New(), blobID)
	if err == nil {
		t.Error("media for an unknown asset was accepted")
	}
	_, err = pool.Exec(ctx,
		`insert into asset_media (id, asset_id, role, blob_id)
		 values (gen_random_uuid(), $1, 'cover', $2)`, assetID, blobID)
	if err == nil {
		t.Error("media with a role outside the closed vocabulary was accepted")
	}
}

func TestCoverIsAnOptionalAssetReference(t *testing.T) {
	pool := Connect(t)
	assetID, _, blobID := insertAssetRevision(t, pool)
	ctx := context.Background()

	assetColumns, err := tableColumns(pool, "assets")
	if err != nil {
		t.Fatalf("read asset columns: %v", err)
	}
	if !slices.Contains(assetColumns, "cover_media_id") || slices.Contains(assetColumns, "preview_media_id") {
		t.Fatalf("asset columns do not carry the cover directly: %v", assetColumns)
	}

	var coverID *uuid.UUID
	if err := pool.QueryRow(ctx,
		`select cover_media_id from assets where id = $1`, assetID,
	).Scan(&coverID); err != nil {
		t.Fatalf("read empty cover: %v", err)
	}
	if coverID != nil {
		t.Fatalf("new asset has synthetic cover %s", *coverID)
	}

	mediaID := uuid.New()
	if _, err := pool.Exec(ctx,
		`insert into asset_media (id, asset_id, role, blob_id)
		 values ($1, $2, 'avatar', $3)`, mediaID, assetID, blobID,
	); err != nil {
		t.Fatalf("insert cover media: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`update assets set cover_media_id = $2 where id = $1`, assetID, mediaID,
	); err != nil {
		t.Fatalf("set cover: %v", err)
	}
}

func TestBrowseIndexStartsWithCreationTimeAndCarriesTheCatalogPredicate(t *testing.T) {
	pool := Connect(t)

	var columns []string
	var predicate string
	var definition string
	err := pool.QueryRow(context.Background(),
		`select array_agg(attribute.attname order by key.ordinality),
		        pg_get_expr(index.indpred, index.indrelid),
		        pg_get_indexdef(index.indexrelid)
		   from pg_index index
		   join pg_class index_class on index_class.oid = index.indexrelid
		  cross join lateral unnest(index.indkey) with ordinality key(attnum, ordinality)
		   join pg_attribute attribute
		     on attribute.attrelid = index.indrelid and attribute.attnum = key.attnum
		  where index_class.relname = 'assets_browse_idx'
		  group by index.indexrelid, index.indpred, index.indrelid`).Scan(&columns, &predicate, &definition)
	if err != nil {
		t.Fatalf("read browse index: %v", err)
	}
	if !slices.Equal(columns, []string{"created_at", "id"}) {
		t.Errorf("browse index columns = %v, want created_at and id", columns)
	}
	if !strings.Contains(definition, "(created_at DESC, id DESC)") {
		t.Errorf("browse index ordering = %q, want creation time and id descending", definition)
	}
	for _, clause := range []string{"discovery = 'listed'", "withheld_at IS NULL", "deleted_at IS NULL"} {
		if !strings.Contains(predicate, clause) {
			t.Errorf("browse index predicate %q does not contain %q", predicate, clause)
		}
	}
}

func tableColumns(pool *pgxpool.Pool, table string) ([]string, error) {
	rows, err := pool.Query(context.Background(),
		`select column_name
		   from information_schema.columns
		  where table_schema = 'public' and table_name = $1
		  order by ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	return rowsToStrings(rows)
}

func insertAssetRevision(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	assetID := uuid.New()
	revisionID := uuid.New()
	blobID := uuid.New()
	digest := make([]byte, 32)
	copy(digest, revisionID[:])
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`insert into blobs (id, sha256, byte_size, storage_key)
		 values ($1, $2, 1, $3)`, blobID, digest, uuid.NewString())
	if err == nil {
		_, err = pool.Exec(ctx,
			`insert into assets (id, kind, name, lifecycle)
			 values ($1, 'character', 'Card', 'published')`, assetID)
	}
	if err == nil {
		_, err = pool.Exec(ctx,
			`insert into asset_revisions
			     (id, asset_id, revision, blob_id, media_type, format)
			 values ($1, $2, 1, $3, 'application/json', 'chara_card_v3')`,
			revisionID, assetID, blobID)
	}
	if err != nil {
		t.Fatalf("insert asset revision: %v", err)
	}
	return assetID, revisionID, blobID
}
