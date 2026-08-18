-- name: InsertAsset :one
-- indexed_at is left to its default so nothing a caller sends can reach it.
insert into assets
  (id, kind, owner_id, name, blurb, tags, is_nsfw, discovery, lifecycle,
   asset_version, credited_author, nickname, origin_format, created_at)
values ($1, $2, $3, $4, $5, $6, sqlc.narg('is_nsfw')::boolean, $7, $8,
        $9, $10, $11, sqlc.narg('origin_format')::text,
        coalesce(sqlc.narg('created_at')::timestamptz, now()))
returning created_at;

-- name: InsertAssetBlock :exec
insert into asset_blocks
  (id, asset_id, definition, title, position, hidden, layout, width, elements)
values ($1, $2, $3, sqlc.narg('title')::text, $4, $5, $6, $7, $8);

-- name: AssetBlocks :many
select id, definition, title, position, hidden, layout, width, elements
  from asset_blocks
 where asset_id = $1
 order by position;

-- name: InsertRevision :exec
insert into asset_revisions
  (id, asset_id, revision, blob_id, media_type, format)
values ($1, $2, $3, $4, $5, $6);

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
select a.id, a.kind, revision.format, a.origin_format,
       a.asset_version, a.credited_author, a.nickname, a.lifecycle,
       a.name, a.blurb, a.tags,
       -- Only a draft leaves the question unanswered and no draft reaches a
       -- listing. If one ever did, the safe reading is the one that blurs it.
       coalesce(a.is_nsfw, true)::boolean as is_nsfw, a.discovery,
       a.current_revision_id, a.created_at
  from assets a
  left join asset_revisions revision on revision.id = a.current_revision_id
 where a.lifecycle = 'published'
   and a.discovery = 'listed'
   and a.withheld_at is null
   and a.deleted_at is null
   and ($1 = '' or a.kind = $1)
   and (not $2::boolean or revision.format is not distinct from $3)
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

-- name: BrowseAssets :many
-- A creator's own listing is the one place a draft appears, so the adult
-- content answer comes back as it is stored, unanswered included.
select a.id, a.name, coalesce(owner.username, 'unknown') as creator,
       a.kind, a.is_nsfw, a.created_at, a.lifecycle,
       cover.id as cover_id, cover.width as cover_width, cover.height as cover_height,
       a.discovery, a.withheld_at, a.withheld_reason, actor.username as withheld_by
  from assets a
  left join asset_revisions revision on revision.id = a.current_revision_id
  left join users owner on owner.id = a.owner_id
  left join users actor on actor.id = a.withheld_by
  left join asset_media cover
    on cover.id = a.cover_media_id and cover.asset_id = a.id
   and cover.is_current
   and cover.width is not null and cover.height is not null
   and cover.blob_id is not null
 where (a.lifecycle = 'published'
        or (sqlc.arg('own_profile')::boolean
            and a.owner_id = sqlc.narg('creator_id')::uuid))
   and a.deleted_at is null
   and (
       (sqlc.narg('creator_id')::uuid is null
        and a.discovery = 'listed' and a.withheld_at is null)
       or
       (a.owner_id = sqlc.narg('creator_id')::uuid
        and (sqlc.arg('own_profile')::boolean
             or (a.discovery = 'listed' and a.withheld_at is null)))
   )
   and (sqlc.narg('creator_id')::uuid is null or sqlc.arg('own_profile')::boolean
        or sqlc.arg('creator_allows_nsfw')::boolean or not a.is_nsfw)
   and (sqlc.arg('kind')::text = '' or a.kind = sqlc.arg('kind')::text)
   and (sqlc.arg('own_profile')::boolean
        or sqlc.arg('nsfw_visibility')::text <> 'hidden' or not a.is_nsfw)
   and (sqlc.arg('platform')::text = ''
        or sqlc.arg('platform')::text = 'raw'
        or revision.format = any(sqlc.arg('formats')::text[]))
   and (cardinality(sqlc.arg('facet_keys')::text[]) = 0 or not exists (
        select 1
          from (select unnest(sqlc.arg('facet_keys')::text[]) as key,
                       unnest(sqlc.arg('facet_values')::text[]) as value) selected
         where not exists (
             select 1 from asset_facets stored
              where stored.revision_id = a.current_revision_id
                and stored.key = selected.key and stored.value = selected.value
         )
   ))
   and (sqlc.arg('search_text')::text = ''
        or position(sqlc.arg('search_text')::text in lower(a.name)) > 0
        or position(sqlc.arg('search_text')::text in lower(a.blurb)) > 0
        or position(sqlc.arg('search_text')::text in lower(coalesce(owner.username, ''))) > 0)
   and (sqlc.arg('author')::text = '' or lower(coalesce(owner.username, '')) = sqlc.arg('author')::text)
   and (cardinality(sqlc.arg('tags')::text[]) = 0 or not exists (
        select 1 from unnest(sqlc.arg('tags')::text[]) wanted(tag)
         where not exists (
             select 1 from unnest(a.tags) stored(tag)
              where lower(btrim(stored.tag)) = wanted.tag
         )
   ))
   and (sqlc.narg('before')::timestamptz is null
        or (a.created_at, a.id)
           < (sqlc.narg('before')::timestamptz, sqlc.narg('before_id')::uuid))
 order by a.created_at desc, a.id desc
 limit sqlc.arg('page_size');

-- name: CountBrowseAssets :one
select count(*)
  from assets a
  left join asset_revisions revision on revision.id = a.current_revision_id
  left join users owner on owner.id = a.owner_id
 where (a.lifecycle = 'published'
        or (sqlc.arg('own_profile')::boolean
            and a.owner_id = sqlc.narg('creator_id')::uuid))
   and a.deleted_at is null
   and (
       (sqlc.narg('creator_id')::uuid is null
        and a.discovery = 'listed' and a.withheld_at is null)
       or
       (a.owner_id = sqlc.narg('creator_id')::uuid
        and (sqlc.arg('own_profile')::boolean
             or (a.discovery = 'listed' and a.withheld_at is null)))
   )
   and (sqlc.narg('creator_id')::uuid is null or sqlc.arg('own_profile')::boolean
        or sqlc.arg('creator_allows_nsfw')::boolean or not a.is_nsfw)
   and (sqlc.arg('kind')::text = '' or a.kind = sqlc.arg('kind')::text)
   and (sqlc.arg('own_profile')::boolean
        or sqlc.arg('nsfw_visibility')::text <> 'hidden' or not a.is_nsfw)
   and (sqlc.arg('platform')::text = ''
        or sqlc.arg('platform')::text = 'raw'
        or revision.format = any(sqlc.arg('formats')::text[]))
   and (cardinality(sqlc.arg('facet_keys')::text[]) = 0 or not exists (
        select 1
          from (select unnest(sqlc.arg('facet_keys')::text[]) as key,
                       unnest(sqlc.arg('facet_values')::text[]) as value) selected
         where not exists (
             select 1 from asset_facets stored
              where stored.revision_id = a.current_revision_id
                and stored.key = selected.key and stored.value = selected.value
         )
   ))
   and (sqlc.arg('search_text')::text = ''
        or position(sqlc.arg('search_text')::text in lower(a.name)) > 0
        or position(sqlc.arg('search_text')::text in lower(a.blurb)) > 0
        or position(sqlc.arg('search_text')::text in lower(coalesce(owner.username, ''))) > 0)
   and (sqlc.arg('author')::text = '' or lower(coalesce(owner.username, '')) = sqlc.arg('author')::text)
   and (cardinality(sqlc.arg('tags')::text[]) = 0 or not exists (
        select 1 from unnest(sqlc.arg('tags')::text[]) wanted(tag)
         where not exists (
             select 1 from unnest(a.tags) stored(tag)
              where lower(btrim(stored.tag)) = wanted.tag
         )
   ));

-- name: CountSuppressedBrowseAssets :one
select count(*)
  from assets a
  left join asset_revisions revision on revision.id = a.current_revision_id
  left join users owner on owner.id = a.owner_id
 where a.lifecycle = 'published'
   and a.discovery = 'listed'
   and a.withheld_at is null
   and a.deleted_at is null
   and (sqlc.narg('creator_id')::uuid is null or a.owner_id = sqlc.narg('creator_id')::uuid)
   and (sqlc.narg('creator_id')::uuid is null
        or sqlc.arg('creator_allows_nsfw')::boolean or not a.is_nsfw)
   and (sqlc.arg('kind')::text = '' or a.kind = sqlc.arg('kind')::text)
   and (sqlc.arg('platform')::text = ''
        or sqlc.arg('platform')::text = 'raw'
        or revision.format = any(sqlc.arg('formats')::text[]))
   and (cardinality(sqlc.arg('facet_keys')::text[]) = 0 or not exists (
        select 1
          from (select unnest(sqlc.arg('facet_keys')::text[]) as key,
                       unnest(sqlc.arg('facet_values')::text[]) as value) selected
         where not exists (
             select 1 from asset_facets stored
              where stored.revision_id = a.current_revision_id
                and stored.key = selected.key and stored.value = selected.value
         )
   ))
   and (sqlc.arg('search_text')::text = ''
        or position(sqlc.arg('search_text')::text in lower(a.name)) > 0
        or position(sqlc.arg('search_text')::text in lower(a.blurb)) > 0
        or position(sqlc.arg('search_text')::text in lower(coalesce(owner.username, ''))) > 0)
   and (sqlc.arg('author')::text = '' or lower(coalesce(owner.username, '')) = sqlc.arg('author')::text)
   and (cardinality(sqlc.arg('tags')::text[]) = 0 or not exists (
        select 1 from unnest(sqlc.arg('tags')::text[]) wanted(tag)
         where not exists (
             select 1 from unnest(a.tags) stored(tag)
              where lower(btrim(stored.tag)) = wanted.tag
         )
   ))
   and a.is_nsfw;

-- name: AssetPage :one
-- Unlisted is missing from this predicate on purpose. A stranger holding the
-- link gets a normal answer.
select a.id, a.kind, a.name, a.blurb, a.tags, a.is_nsfw, a.discovery,
       a.lifecycle, a.created_at,
       revision.format as original_format, revision.media_type as original_media_type,
       revision.created_at as original_arrived_at,
       coalesce(owner.username, 'unknown') as creator,
       coalesce(a.owner_id = sqlc.narg('viewer_id')::uuid, false)::boolean as is_owner,
       a.withheld_reason, a.withheld_at, actor.username as withheld_by
  from assets a
  left join users owner on owner.id = a.owner_id
  left join users actor on actor.id = a.withheld_by
  left join asset_revisions revision on revision.id = a.current_revision_id
 where a.id = $1
   and a.deleted_at is null
   and (a.lifecycle = 'published' or a.owner_id = sqlc.narg('viewer_id')::uuid)
   and (a.withheld_at is null or a.owner_id = sqlc.narg('viewer_id')::uuid);

-- name: AssetPageMedia :many
-- The direct cover comes first; remaining media follows the established role order.
select media.id, media.role, media.width, media.height,
       coalesce(media.id = a.cover_media_id, false)::boolean as is_cover
  from assets a
  join asset_media media on media.asset_id = a.id
 where a.id = $1
   and media.is_current
   and media.width is not null
   and media.height is not null
   and media.blob_id is not null
 order by (media.id = a.cover_media_id) desc,
          case media.role
            when 'avatar' then 1
            when 'avatar_alt' then 2
            when 'gallery' then 3
            when 'expression' then 4
            else 5
          end,
          media.created_at desc, media.id desc;

-- name: CurrentRevisionLocation :one
-- A draft has no download for anyone, its owner included.
select a.id as asset_id, r.id as revision_id, r.blob_id, r.media_type, a.owner_id
  from assets a
  join asset_revisions r on r.id = a.current_revision_id
 where a.id = $1
   and r.blob_id is not null
   and a.lifecycle = 'published'
   and a.deleted_at is null
   and (a.withheld_at is null or a.owner_id = sqlc.narg('viewer_id')::uuid);

-- name: AssetByID :one
select a.id, a.kind, revision.format, a.origin_format,
       a.asset_version, a.credited_author, a.nickname, a.lifecycle,
       a.name, a.blurb, a.tags,
       -- Imported drafts carry an answer. The fallback protects older rows
       -- that predate this invariant.
       coalesce(a.is_nsfw, true)::boolean as is_nsfw, a.discovery,
       a.current_revision_id, a.created_at
  from assets a
  join asset_revisions revision on revision.id = a.current_revision_id
 where a.id = $1;

-- name: SetAssetDiscovery :execrows
update assets
   set discovery = $3, updated_at = now()
 where id = $1 and owner_id = $2 and lifecycle = 'published'
   and withheld_at is null and deleted_at is null;

-- name: AssetStateForOwner :one
select withheld_at, lifecycle
  from assets
 where id = $1 and owner_id = $2 and deleted_at is null;

-- name: WithholdAsset :execrows
update assets
   set withheld_at = now(), withheld_by = $2, withheld_reason = $3,
       updated_at = now()
 where id = $1 and lifecycle = 'published'
   and withheld_at is null and deleted_at is null;

-- name: ClearAssetWithhold :execrows
update assets
   set withheld_at = null, withheld_by = null, withheld_reason = null,
       updated_at = now()
 where id = $1 and withheld_at is not null and deleted_at is null;

-- name: AssetDeletionState :one
select withheld_at, deleted_at
  from assets
 where id = $1 and owner_id = $2;

-- name: SoftDeleteAsset :execrows
update assets
   set deleted_at = $3, recoverable_until = $4, updated_at = $3
 where id = $1 and owner_id = $2
   and withheld_at is null and deleted_at is null;

-- name: RestoreAsset :execrows
update assets
   set deleted_at = null, recoverable_until = null, updated_at = $3
 where id = $1 and owner_id = $2
   and deleted_at is not null and recoverable_until > $3;

-- name: ListDeletedAssets :many
select asset.id, asset.name, asset.kind, asset.deleted_at, asset.recoverable_until
  from assets asset
  join users owner on owner.id = asset.owner_id
 where asset.owner_id = $1 and owner.username = $2
   and asset.deleted_at is not null and asset.recoverable_until > $3
 order by asset.deleted_at desc, asset.id desc;

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

-- name: VerifiedEmailBelongsToDiscordAccount :one
select exists (
    select 1
      from users u
      join oauth_identities oi on oi.user_id = u.id and oi.provider = 'discord'
     where u.email = $1 and u.email_verified_at is not null
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
select u.id, u.username, u.email, u.email_verified_at,
       u.role,
       case when u.password_hash is null then false else true end as has_password,
       exists (
           select 1 from oauth_identities oi
            where oi.user_id = u.id and oi.provider = 'discord'
       ) as discord_linked
  from sessions s
  join users u on u.id = s.user_id
 where s.token_hash = $1 and s.expires_at > now();

-- name: NSFWVisibilityBySessionHash :one
select u.nsfw_visibility
  from sessions session
  join users u on u.id = session.user_id
 where session.token_hash = $1 and session.expires_at > now();

-- name: SetNSFWVisibilityBySessionHash :execrows
update users u
   set nsfw_visibility = $1, updated_at = now()
  from sessions session
 where session.user_id = u.id and session.token_hash = $2
   and session.expires_at > now();

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
with verified as (
    update users
       set email_verified_at = now(), updated_at = now()
     where users.id = $1 and users.email = $2 and users.email_verified_at is null
    returning users.id, users.username, users.email,
              users.email_verified_at, users.password_hash
)
select v.id, v.username, v.email, v.email_verified_at,
       case when v.password_hash is null then false else true end as has_password,
       exists (
           select 1 from oauth_identities oi
            where oi.user_id = v.id and oi.provider = 'discord'
       ) as discord_linked
  from verified v;

-- name: ClearPendingEmailCopies :exec
update users
   set email = null, email_source = null, updated_at = now()
 where email = $1 and id <> $2 and email_verified_at is null;

-- name: DeleteVerificationTokensForEmail :exec
delete from email_verification_tokens where email = $1;

-- name: DeleteVerificationTokensForUser :exec
delete from email_verification_tokens where user_id = $1;

-- name: UsersForSignIn :many
select u.id, u.username, u.email, u.email_verified_at, u.password_hash,
       u.role,
       exists (
           select 1 from oauth_identities oi
            where oi.user_id = u.id and oi.provider = 'discord'
       ) as discord_linked
  from users u
 where u.email = $1 and u.password_hash is not null;

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
select id, username, show_nsfw_contributions_on_profile
  from users where username = $1;

-- name: UpdateUnverifiedEmail :one
update users
   set email = $2, email_source = 'creator', updated_at = now()
 where id = $1 and email_verified_at is null
returning id, username, email, email_verified_at;

-- name: InsertOAuthState :exec
insert into oauth_states (token_hash, intent, user_id, expires_at)
values ($1, $2, $3, $4);

-- name: TakeOAuthState :one
delete from oauth_states
 where token_hash = $1 and expires_at > now()
returning intent, user_id;

-- name: LockOAuthIdentity :one
select 1 from pg_advisory_xact_lock(
    hashtextextended('lumihub-oauth:' || sqlc.arg('provider')::text || ':' ||
                     sqlc.arg('subject')::text, 0)
);

-- name: LockOAuthUser :one
select 1 from pg_advisory_xact_lock(
    hashtextextended('lumihub-oauth-user:' || sqlc.arg('user_id')::uuid::text, 0)
);

-- name: UserByOAuthIdentity :one
select u.id, u.username, u.email, u.email_verified_at, u.email_source,
       u.role,
       case when u.password_hash is null then false else true end as has_password
  from oauth_identities oi
  join users u on u.id = oi.user_id
 where oi.provider = $1 and oi.subject = $2;

-- name: InsertDiscordUser :one
insert into users (id, username, email, email_verified_at, email_source)
values ($1, $2, $3, $4, case when $3::text is null then null else 'discord' end)
returning id, username, email, email_verified_at;

-- name: InsertOAuthIdentity :exec
insert into oauth_identities (user_id, provider, subject, provider_email)
values ($1, $2, $3, $4);

-- name: UpdateDiscordEmail :one
update users
   set email = $2, email_verified_at = now(), email_source = 'discord', updated_at = now()
 where id = $1
   and (email is null or email_source = 'discord')
returning id, username, email, email_verified_at, email_source;

-- name: UpdateOAuthIdentityEmail :exec
update oauth_identities
   set provider_email = $3, updated_at = now()
 where provider = $1 and subject = $2;

-- name: UserForDiscordAttach :one
select id, username, email, email_verified_at, email_source,
       role,
       case when password_hash is null then false else true end as has_password
  from users
 where id = $1
 for update;

-- name: SetFirstPassword :one
update users
   set password_hash = $2, updated_at = now()
 where id = $1 and password_hash is null
returning id, username, email, email_verified_at;

-- name: ReplacePassword :exec
update users
   set password_hash = $2, updated_at = now()
 where id = $1;

-- name: DiscordSubjectsForUser :many
select subject
  from oauth_identities
 where user_id = $1 and provider = 'discord'
 order by subject;

-- name: DeleteOAuthIdentitiesForUser :exec
delete from oauth_identities
 where user_id = $1 and provider = 'discord';

-- name: VerifiedUserIDByEmail :one
select id from users where email = $1 and email_verified_at is not null;

-- name: DeletePasswordResetForUser :exec
delete from password_reset_tokens where user_id = $1;

-- name: InsertPasswordReset :exec
insert into password_reset_tokens (token_hash, user_id, expires_at)
values ($1, $2, $3);

-- name: TakePasswordReset :one
delete from password_reset_tokens
 where token_hash = $1 and expires_at > now()
returning user_id;

-- name: DeleteExpiredLinkRequests :exec
delete from link_requests where expires_at <= now();

-- name: InsertLinkRequest :exec
insert into link_requests (device_code_hash, user_code, client_name, scopes, expires_at)
values ($1, $2, $3, $4, $5);

-- name: LinkRequestByUserCode :one
select client_name, scopes, expires_at
  from link_requests
 where user_code = $1 and expires_at > now() and approved_at is null;

-- name: ApproveLinkRequest :execrows
update link_requests
   set approved_by = $2, approved_at = now()
 where user_code = $1 and expires_at > now() and approved_at is null;

-- name: LockLinkRequest :one
select approved_by, client_name, scopes, approved_at, redeemed_at, last_polled_at
  from link_requests
 where device_code_hash = $1 and expires_at > now()
 for update;

-- name: RecordLinkPoll :exec
update link_requests set last_polled_at = now() where device_code_hash = $1;

-- name: RedeemLinkRequest :execrows
update link_requests
   set redeemed_at = now()
 where device_code_hash = $1 and approved_at is not null and redeemed_at is null;

-- name: LinkCodeFailures :one
select failures from link_code_attempts where user_id = $1 and window_start > $2;

-- name: RecordLinkCodeFailure :exec
insert into link_code_attempts (user_id, failures, window_start)
values ($1, 1, now())
on conflict (user_id) do update
   set failures = case when link_code_attempts.window_start > $2 then link_code_attempts.failures + 1 else 1 end,
       window_start = case when link_code_attempts.window_start > $2 then link_code_attempts.window_start else now() end;

-- name: ClearLinkCodeFailures :exec
delete from link_code_attempts where user_id = $1;

-- name: InsertLinkedInstance :one
insert into linked_instances (id, user_id, name, token_hash, token_prefix, scopes)
values ($1, $2, $3, $4, $5, $6)
returning id, name, token_prefix, scopes, linked_at, last_seen_at, revoked_at;

-- name: ListLinkedInstances :many
select id, name, token_prefix, scopes, linked_at, last_seen_at, revoked_at
  from linked_instances
 where user_id = $1
 order by (revoked_at is not null), coalesce(last_seen_at, linked_at) desc, linked_at desc;

-- name: RevokeLinkedInstance :execrows
update linked_instances
   set token_hash = null, revoked_at = now()
 where id = $1 and user_id = $2 and revoked_at is null;

-- name: TouchLinkedInstance :one
update linked_instances
   set last_seen_at = now()
 where token_hash = $1 and revoked_at is null
returning id, user_id, name, token_prefix, scopes, linked_at, last_seen_at;
