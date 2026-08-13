-- name: InsertAsset :one
-- indexed_at is left to its default so nothing a caller sends can reach it.
insert into assets
  (id, kind, owner_id, name, description, tags, is_nsfw, discovery, created_at)
values ($1, $2, $3, $4, $5, $6, $7, $8,
        coalesce(sqlc.narg('created_at')::timestamptz, now()))
returning created_at;

-- name: InsertRevision :exec
insert into asset_revisions
  (id, asset_id, revision, blob_id, media_type, format, passthrough_platform)
values ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertFacet :exec
insert into asset_facets (revision_id, key, value)
values ($1, $2, $3)
on conflict do nothing;

-- name: SetCurrentRevision :exec
update assets set current_revision_id = $2, updated_at = now() where id = $1;

-- name: ListAssets :many
with facet_pairs as (
  select unnest($5::text[]) as k, unnest($6::text[]) as v
)
select a.id, a.kind, revision.passthrough_platform, revision.format,
       a.name, a.description, a.tags, a.is_nsfw, a.discovery,
       a.current_revision_id, a.created_at
  from assets a
  join asset_revisions revision on revision.id = a.current_revision_id
 where a.discovery = 'listed'
   and a.withheld_at is null
   and a.deleted_at is null
   and ($1 = '' or a.kind = $1)
   and (not $2::boolean or revision.passthrough_platform is not distinct from $3)
   and ($4::text[] is null or a.tags @> $4)
   and (array_length($5::text[], 1) is null or (
         select count(*) from asset_facets af
          where af.revision_id = a.current_revision_id
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

-- name: LockHandle :one
select 1 from pg_advisory_xact_lock(
    hashtextextended('lumihub-handle:' || sqlc.arg('handle')::text, 0)
);

-- name: LockEmail :one
select 1 from pg_advisory_xact_lock(
    hashtextextended('lumihub-email:' || sqlc.arg('email')::text, 0)
);

-- name: HandleUnavailable :one
select exists (
    select 1 from users where username = $1
    union all
    select 1 from retired_handles where handle = $1
);

-- name: VerifiedEmailExists :one
select exists (
    select 1 from users where email = $1 and email_verified_at is not null
);

-- name: InsertUser :one
insert into users (id, username, email, password_hash, email_source)
values ($1, $2, $3, $4, 'creator')
returning id, username, email, email_verified_at;

-- name: InsertEmailVerificationToken :exec
insert into email_verification_tokens (token_hash, user_id, email, expires_at)
values ($1, $2, $3, $4);

-- name: InsertSession :exec
insert into sessions (token_hash, user_id, expires_at)
values ($1, $2, $3);

-- name: UserBySessionHash :one
select u.id, u.username, u.email, u.email_verified_at
  from sessions s
  join users u on u.id = s.user_id
 where s.token_hash = $1 and s.expires_at > now();

-- name: VerificationByHash :one
select user_id, email
  from email_verification_tokens
 where token_hash = $1 and expires_at > now()
 for update;

-- name: VerificationEmailByHash :one
select email
  from email_verification_tokens
 where token_hash = $1 and expires_at > now();

-- name: VerifyUserEmail :one
update users
   set email_verified_at = now(), updated_at = now()
 where id = $1 and email = $2 and email_verified_at is null
returning id, username, email, email_verified_at;

-- name: ClearPendingEmailCopies :exec
update users
   set email = null, email_source = null, updated_at = now()
 where email = $1 and id <> $2 and email_verified_at is null;

-- name: DeleteVerificationTokensForEmail :exec
delete from email_verification_tokens where email = $1;

-- name: DeleteVerificationTokensForUser :exec
delete from email_verification_tokens where user_id = $1;

-- name: UsersForSignIn :many
select id, username, email, email_verified_at, password_hash
  from users
 where email = $1 and password_hash is not null;

-- name: DeleteSession :exec
delete from sessions where token_hash = $1;

-- name: UserHandleForUpdate :one
select username from users where id = $1 for update;

-- name: InsertRetiredHandle :exec
insert into retired_handles (handle) values ($1);

-- name: UpdateUserHandle :one
update users set username = $2, updated_at = now() where id = $1
returning id, username, email, email_verified_at;

-- name: ProfileByHandle :one
select id, username from users where username = $1;

-- name: UpdateUnverifiedEmail :one
update users
   set email = $2, email_source = 'creator', updated_at = now()
 where id = $1 and email_verified_at is null
returning id, username, email, email_verified_at;
