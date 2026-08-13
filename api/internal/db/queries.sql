-- name: InsertAsset :one
-- indexed_at is left to its default so nothing a caller sends can reach it.
insert into assets
  (id, kind, platform, format, format_version, owner_id,
   name, description, tags, is_nsfw, publication, created_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
        coalesce(sqlc.narg('created_at')::timestamptz, now()))
returning created_at;

-- name: InsertRevision :exec
insert into asset_revisions
  (id, asset_id, revision, blob_id, media_type)
values ($1, $2, $3, $4, $5);

-- name: InsertFacet :exec
insert into asset_facets (asset_id, revision_id, key, value)
values ($1, $2, $3, $4)
on conflict do nothing;

-- name: SetCurrentRevision :exec
update assets set current_revision_id = $2, updated_at = now() where id = $1;

-- name: ListAssets :many
with facet_pairs as (
  select unnest($5::text[]) as k, unnest($6::text[]) as v
)
select a.id, a.kind, a.platform, a.format, a.format_version,
       a.name, a.description, a.tags, a.is_nsfw, a.publication,
       a.current_revision_id, a.created_at
  from assets a
 where a.publication = 'public'
   and ($1 = '' or a.kind = $1)
   and (not $2::boolean or a.platform is not distinct from $3)
   and ($4::text[] is null or a.tags @> $4)
   and (array_length($5::text[], 1) is null or (
         select count(*) from asset_facets af
          where af.asset_id = a.id
            and (af.key, af.value) in (
                  select k, v from facet_pairs
            )
       ) = array_length($5::text[], 1))
   and (sqlc.narg('before')::timestamptz is null
        or (a.created_at, a.id)
           < (sqlc.narg('before')::timestamptz, sqlc.narg('before_id')::uuid))
 order by a.created_at desc, a.id desc
 limit $7;

-- name: CurrentRevisionLocation :one
select r.blob_id, r.media_type
  from assets a
  join asset_revisions r on r.id = a.current_revision_id
 where a.id = $1;

-- name: UpsertBlob :one
insert into blobs (id, sha256, byte_size, storage_key)
values ($1, $2, $3, $4)
on conflict (sha256) do update set sha256 = excluded.sha256
returning id, sha256, byte_size, storage_key;

-- name: BlobLocation :one
select storage_key, byte_size from blobs where id = $1;
