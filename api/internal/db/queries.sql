-- name: InsertAsset :one
-- indexed_at is left to its default so nothing a caller sends can reach it.
insert into assets
  (id, kind, owner_id, name, blurb, tags, is_nsfw, discovery, created_at)
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
       a.name, a.blurb, a.tags, a.is_nsfw, a.discovery,
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

-- name: BrowseAssets :many
select a.id, a.name, coalesce(owner.username, 'unknown') as creator,
       a.kind, a.is_nsfw, a.created_at,
       cover.id as cover_id, cover.width as cover_width, cover.height as cover_height,
       a.discovery, a.withheld_at, a.withheld_reason, actor.username as withheld_by
  from assets a
  join asset_revisions revision on revision.id = a.current_revision_id
  left join users owner on owner.id = a.owner_id
  left join users actor on actor.id = a.withheld_by
  left join lateral (
      select media.id, media.width, media.height
        from asset_media media
       where (media.asset_id = a.id or media.revision_id = a.current_revision_id)
         and media.width is not null and media.height is not null
       order by case media.role
                  when 'avatar' then 1
                  when 'avatar_alt' then 2
                  when 'gallery' then 3
                  when 'expression' then 4
                  else 5
                end,
                media.created_at desc, media.id desc
       limit 1
  ) cover on true
 where a.deleted_at is null
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
        or revision.format = any(sqlc.arg('formats')::text[])
        or (revision.format = 'unknown'
            and revision.passthrough_platform = sqlc.arg('platform')::text))
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
  join asset_revisions revision on revision.id = a.current_revision_id
  left join users owner on owner.id = a.owner_id
 where a.deleted_at is null
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
        or revision.format = any(sqlc.arg('formats')::text[])
        or (revision.format = 'unknown'
            and revision.passthrough_platform = sqlc.arg('platform')::text))
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
  join asset_revisions revision on revision.id = a.current_revision_id
  left join users owner on owner.id = a.owner_id
 where a.discovery = 'listed'
   and a.withheld_at is null
   and a.deleted_at is null
   and (sqlc.narg('creator_id')::uuid is null or a.owner_id = sqlc.narg('creator_id')::uuid)
   and (sqlc.narg('creator_id')::uuid is null
        or sqlc.arg('creator_allows_nsfw')::boolean or not a.is_nsfw)
   and (sqlc.arg('kind')::text = '' or a.kind = sqlc.arg('kind')::text)
   and (sqlc.arg('platform')::text = ''
        or sqlc.arg('platform')::text = 'raw'
        or revision.format = any(sqlc.arg('formats')::text[])
        or (revision.format = 'unknown'
            and revision.passthrough_platform = sqlc.arg('platform')::text))
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
select a.id, a.kind, a.name, a.blurb, a.tags, a.is_nsfw, a.discovery, a.created_at,
       coalesce(owner.username, 'unknown') as creator,
       a.withheld_reason, a.withheld_at, actor.username as withheld_by
  from assets a
  left join users owner on owner.id = a.owner_id
  left join users actor on actor.id = a.withheld_by
 where a.id = $1
   and a.deleted_at is null
   and (a.withheld_at is null or a.owner_id = sqlc.narg('viewer_id')::uuid);

-- name: AssetPageMedia :many
-- The role order matches BrowseAssets, so the first image is the card's cover.
select media.id, media.role, media.width, media.height
  from assets a
  join asset_media media
    on media.asset_id = a.id or media.revision_id = a.current_revision_id
 where a.id = $1
   and media.width is not null
   and media.height is not null
 order by case media.role
            when 'avatar' then 1
            when 'avatar_alt' then 2
            when 'gallery' then 3
            when 'expression' then 4
            else 5
          end,
          media.created_at desc, media.id desc;

-- name: CurrentRevisionLocation :one
select r.blob_id, r.media_type
  from assets a
  join asset_revisions r on r.id = a.current_revision_id
 where a.id = $1
   and a.deleted_at is null
   and (a.withheld_at is null or a.owner_id = sqlc.narg('viewer_id')::uuid);

-- name: AssetByID :one
select a.id, a.kind, revision.passthrough_platform, revision.format,
       a.name, a.blurb, a.tags, a.is_nsfw, a.discovery,
       a.current_revision_id, a.created_at
  from assets a
  join asset_revisions revision on revision.id = a.current_revision_id
 where a.id = $1;

-- name: SetAssetDiscovery :execrows
update assets
   set discovery = $3, updated_at = now()
 where id = $1 and owner_id = $2
   and withheld_at is null and deleted_at is null;

-- name: AssetWithheldAtForOwner :one
select withheld_at
  from assets
 where id = $1 and owner_id = $2 and deleted_at is null;

-- name: WithholdAsset :execrows
update assets
   set withheld_at = now(), withheld_by = $2, withheld_reason = $3,
       updated_at = now()
 where id = $1 and withheld_at is null and deleted_at is null;

-- name: ClearAssetWithhold :execrows
update assets
   set withheld_at = null, withheld_by = null, withheld_reason = null,
       updated_at = now()
 where id = $1 and withheld_at is not null and deleted_at is null;

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
