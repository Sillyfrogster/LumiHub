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
		"asset_facets",
		"asset_media",
		"file_field_patches",
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

func TestFileFieldPatchesKeepAClosedVocabularyAndProvenance(t *testing.T) {
	pool := Connect(t)
	assetID, revisionID, _ := insertAssetRevision(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		insert into file_field_patches (asset_id, field, value, provenance)
		values ($1, 'description', 'Creator text', 'creator')
	`, assetID); err != nil {
		t.Fatalf("insert creator patch: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into file_field_patches (asset_id, revision_id, field, value, provenance)
		values ($1, $2, 'creator_notes', 'Reconciled text', 'reconciliation')
	`, assetID, revisionID); err != nil {
		t.Fatalf("insert reconciliation patch: %v", err)
	}

	for name, statement := range map[string]string{
		"arbitrary path": `insert into file_field_patches (asset_id, field, value, provenance)
			values ($1, 'extensions.depth_prompt', $2::uuid::text, 'creator')`,
		"creator revision": `insert into file_field_patches (asset_id, revision_id, field, value, provenance)
			values ($1, $2, 'scenario', 'wrong scope', 'creator')`,
		"unscoped reconciliation": `insert into file_field_patches (asset_id, field, value, provenance)
			values ($1, 'scenario', $2::uuid::text, 'reconciliation')`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, statement, assetID, revisionID); err == nil {
				t.Fatal("invalid patch row was accepted")
			}
		})
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

func TestAssetKindIsClosedToFourValues(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()

	for _, kind := range []string{"character", "lorebook", "preset", "theme"} {
		_, err := pool.Exec(ctx,
			`insert into assets (id, kind, name) values (gen_random_uuid(), $1, $2)`,
			kind, kind)
		if err != nil {
			t.Errorf("insert kind %q: %v", kind, err)
		}
	}

	_, err := pool.Exec(ctx,
		`insert into assets (id, kind, name) values (gen_random_uuid(), 'pack', 'Pack')`)
	if err == nil {
		t.Fatal("kind outside the catalog vocabulary was accepted")
	}
}

func TestDiscoveryDefaultsToListedAndRejectsUnknownValues(t *testing.T) {
	pool := Connect(t)
	ctx := context.Background()

	var discovery string
	err := pool.QueryRow(ctx,
		`insert into assets (id, kind, name)
		 values (gen_random_uuid(), 'character', 'Listed by default')
		 returning discovery`).Scan(&discovery)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if discovery != "listed" {
		t.Errorf("discovery = %q, want listed", discovery)
	}

	_, err = pool.Exec(ctx,
		`insert into assets (id, kind, name, discovery)
		 values (gen_random_uuid(), 'theme', 'Quiet', 'unlisted')`)
	if err != nil {
		t.Errorf("insert unlisted asset: %v", err)
	}

	_, err = pool.Exec(ctx,
		`insert into assets (id, kind, name, discovery)
		 values (gen_random_uuid(), 'theme', 'Unknown', 'private')`)
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
		`insert into assets (id, kind, name) values ($1, 'character', 'Held')`, assetID)
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
				`insert into assets (id, kind, name) values ($1, 'character', 'Partial')`, id)
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

func TestFormatAndPassthroughPlatformBelongToARevision(t *testing.T) {
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
	for _, column := range []string{"kind", "discovery", "withheld_at", "withheld_by", "withheld_reason", "deleted_at"} {
		if !slices.Contains(assetColumns, column) {
			t.Errorf("assets has no %s", column)
		}
	}

	revisionColumns, err := tableColumns(pool, "asset_revisions")
	if err != nil {
		t.Fatalf("read revision columns: %v", err)
	}
	for _, column := range []string{"format", "passthrough_platform"} {
		if !slices.Contains(revisionColumns, column) {
			t.Errorf("asset_revisions has no %s", column)
		}
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

func TestFacetsBindOnlyToARevision(t *testing.T) {
	pool := Connect(t)
	assetID, revisionID, _ := insertAssetRevision(t, pool)

	columns, err := tableColumns(pool, "asset_facets")
	if err != nil {
		t.Fatalf("read facet columns: %v", err)
	}
	if !slices.Equal(columns, []string{"revision_id", "key", "value"}) {
		t.Errorf("facet columns = %v, want revision_id, key and value", columns)
	}

	_, err = pool.Exec(context.Background(),
		`insert into asset_facets (revision_id, key, value) values ($1, 'spec', 'chara_card_v3')`,
		revisionID)
	if err != nil {
		t.Fatalf("insert revision facet for asset %s: %v", assetID, err)
	}
	_, err = pool.Exec(context.Background(),
		`insert into asset_facets (revision_id, key, value) values ($1, 'spec', 'unknown')`,
		uuid.New())
	if err == nil {
		t.Fatal("facet without a revision was accepted")
	}
}

func TestMediaBindsToExactlyOneProvenance(t *testing.T) {
	pool := Connect(t)
	assetID, revisionID, blobID := insertAssetRevision(t, pool)
	ctx := context.Background()

	roles := []string{"avatar", "expression", "gallery", "avatar_alt", "perspective_layer"}
	for _, role := range roles {
		_, err := pool.Exec(ctx,
			`insert into asset_media (id, revision_id, role, blob_id)
			 values (gen_random_uuid(), $1, $2, $3)`, revisionID, role, blobID)
		if err != nil {
			t.Fatalf("insert revision-scoped %s media: %v", role, err)
		}
		_, err = pool.Exec(ctx,
			`insert into asset_media (id, asset_id, role, blob_id)
			 values (gen_random_uuid(), $1, $2, $3)`, assetID, role, blobID)
		if err != nil {
			t.Fatalf("insert asset-scoped %s media: %v", role, err)
		}
	}

	_, err := pool.Exec(ctx,
		`insert into asset_media (id, asset_id, revision_id, role, blob_id)
		 values (gen_random_uuid(), $1, $2, 'avatar', $3)`, assetID, revisionID, blobID)
	if err == nil {
		t.Error("media bound to both asset and revision was accepted")
	}
	_, err = pool.Exec(ctx,
		`insert into asset_media (id, role, blob_id)
		 values (gen_random_uuid(), 'gallery', $1)`, blobID)
	if err == nil {
		t.Error("media without asset or revision provenance was accepted")
	}
	_, err = pool.Exec(ctx,
		`insert into asset_media (id, asset_id, role, blob_id)
		 values (gen_random_uuid(), $1, 'cover', $2)`, assetID, blobID)
	if err == nil {
		t.Error("media with a role outside the closed vocabulary was accepted")
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
			`insert into assets (id, kind, name) values ($1, 'character', 'Card')`, assetID)
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
