-- +goose Up

-- +goose StatementBegin
create function is_bounded_distinct_text_array(
    items text[],
    maximum_items integer,
    maximum_item_length integer
) returns boolean
language sql immutable strict parallel safe as $$
    select cardinality(items) <= maximum_items
       and cardinality(items) = (
           select count(*)
             from unnest(items) as item(value)
            where value is not null
              and value = btrim(value)
              and value <> ''
              and char_length(value) <= maximum_item_length
       )
       and cardinality(items) = (
           select count(distinct value) from unnest(items) as item(value)
       );
$$;
-- +goose StatementEnd

-- Pending link requests are short-lived and contain no durable user data. They
-- cannot be converted because the old schema stored human codes in plaintext.
drop table link_code_attempts;
drop table link_requests;

create table link_requests (
    device_code_hash   bytea primary key,
    user_code_hash     bytea not null unique,
    application_name   text not null,
    instance_name      text not null,
    application_version text,
    protocol_version   integer not null,
    capabilities       text[] not null default '{}',
    accepted_targets   text[] not null default '{}',
    scopes             text[] not null,
    review_token_hash  bytea unique,
    reviewed_by        uuid references users (id) on delete cascade,
    approved_by        uuid references users (id) on delete cascade,
    approved_at        timestamptz,
    denied_by          uuid references users (id) on delete cascade,
    denied_at          timestamptz,
    redeemed_at        timestamptz,
    last_polled_at     timestamptz,
    created_at         timestamptz not null default now(),
    expires_at         timestamptz not null,
    constraint link_requests_device_code_hash_check
        check (octet_length(device_code_hash) = 32),
    constraint link_requests_user_code_hash_check
        check (octet_length(user_code_hash) = 32),
    constraint link_requests_review_token_hash_check
        check (review_token_hash is null or octet_length(review_token_hash) = 32),
    constraint link_requests_application_name_check
        check (application_name = btrim(application_name)
           and char_length(application_name) between 1 and 64),
    constraint link_requests_instance_name_check
        check (instance_name = btrim(instance_name)
           and char_length(instance_name) between 1 and 64),
    constraint link_requests_application_version_check
        check (application_version is null
            or (application_version = btrim(application_version)
            and char_length(application_version) between 1 and 64)),
    constraint link_requests_protocol_version_check check (protocol_version > 0),
    constraint link_requests_capabilities_check
        check (is_bounded_distinct_text_array(capabilities, 32, 64)),
    constraint link_requests_accepted_targets_check
        check (is_bounded_distinct_text_array(accepted_targets, 32, 64)),
    constraint link_requests_scopes_check check (is_instance_scope_set(scopes)),
    constraint link_requests_review_check check (
        (reviewed_by is null and review_token_hash is null)
        or (reviewed_by is not null and review_token_hash is not null)
    ),
    constraint link_requests_approval_check check (
        (approved_by is null and approved_at is null)
        or (approved_by is not null and approved_at is not null)
    ),
    constraint link_requests_denial_check check (
        (denied_by is null and denied_at is null)
        or (denied_by is not null and denied_at is not null)
    ),
    constraint link_requests_reviewer_decision_check check (
        (approved_by is null or (reviewed_by is not null and approved_by = reviewed_by))
        and (denied_by is null or (reviewed_by is not null and denied_by = reviewed_by))
        and not (approved_at is not null and denied_at is not null)
    ),
    constraint link_requests_redemption_check check (
        redeemed_at is null or (approved_at is not null and denied_at is null)
    ),
    constraint link_requests_expiry_check check (expires_at > created_at)
);

create index link_requests_expires_at_idx on link_requests (expires_at);

create table link_authorizations (
    request_hash        bytea primary key,
    authorization_code_hash bytea unique,
    redirect_uri        text not null,
    state               text not null,
    code_challenge      text not null,
    application_name    text not null,
    instance_name       text not null,
    application_version text,
    protocol_version    integer not null,
    capabilities        text[] not null default '{}',
    accepted_targets    text[] not null default '{}',
    scopes              text[] not null,
    reviewed_by         uuid references users (id) on delete cascade,
    approved_by         uuid references users (id) on delete cascade,
    approved_at         timestamptz,
    denied_by           uuid references users (id) on delete cascade,
    denied_at           timestamptz,
    redeemed_at         timestamptz,
    created_at          timestamptz not null default now(),
    expires_at          timestamptz not null,
    constraint link_authorizations_request_hash_check
        check (octet_length(request_hash) = 32),
    constraint link_authorizations_code_hash_check
        check (authorization_code_hash is null
            or octet_length(authorization_code_hash) = 32),
    constraint link_authorizations_redirect_uri_check
        check (redirect_uri = btrim(redirect_uri)
           and char_length(redirect_uri) between 1 and 2048),
    constraint link_authorizations_state_check
        check (char_length(state) between 32 and 128),
    constraint link_authorizations_code_challenge_check
        check (char_length(code_challenge) = 43),
    constraint link_authorizations_application_name_check
        check (application_name = btrim(application_name)
           and char_length(application_name) between 1 and 64),
    constraint link_authorizations_instance_name_check
        check (instance_name = btrim(instance_name)
           and char_length(instance_name) between 1 and 64),
    constraint link_authorizations_application_version_check
        check (application_version is null
            or (application_version = btrim(application_version)
            and char_length(application_version) between 1 and 64)),
    constraint link_authorizations_protocol_version_check check (protocol_version > 0),
    constraint link_authorizations_capabilities_check
        check (is_bounded_distinct_text_array(capabilities, 32, 64)),
    constraint link_authorizations_accepted_targets_check
        check (is_bounded_distinct_text_array(accepted_targets, 32, 64)),
    constraint link_authorizations_scopes_check check (is_instance_scope_set(scopes)),
    constraint link_authorizations_approval_check check (
        (approved_by is null and approved_at is null and authorization_code_hash is null)
        or (approved_by is not null and approved_at is not null
            and authorization_code_hash is not null)
    ),
    constraint link_authorizations_denial_check check (
        (denied_by is null and denied_at is null)
        or (denied_by is not null and denied_at is not null)
    ),
    constraint link_authorizations_reviewer_decision_check check (
        (approved_by is null or (reviewed_by is not null and approved_by = reviewed_by))
        and (denied_by is null or (reviewed_by is not null and denied_by = reviewed_by))
        and not (approved_at is not null and denied_at is not null)
    ),
    constraint link_authorizations_redemption_check check (
        redeemed_at is null or (approved_at is not null and denied_at is null)
    ),
    constraint link_authorizations_expiry_check check (expires_at > created_at)
);

create index link_authorizations_expires_at_idx on link_authorizations (expires_at);

alter table linked_instances rename column name to instance_name;
alter table linked_instances rename column token_hash to legacy_token_hash;
alter table linked_instances rename column token_prefix to refresh_token_prefix;
alter table linked_instances rename constraint linked_instances_name_check
    to linked_instances_instance_name_check;
alter table linked_instances rename constraint linked_instances_token_hash_key
    to linked_instances_legacy_token_hash_key;
alter table linked_instances rename constraint linked_instances_token_prefix_check
    to linked_instances_refresh_token_prefix_check;
alter table linked_instances drop constraint linked_instances_credential_check;

-- Old permanent tokens are no longer accepted by any live endpoint. Their
-- one-way hashes remain only for the bounded, one-use cutover in ticket 26.
alter table linked_instances add column refresh_token_hash bytea unique;
alter table linked_instances add constraint linked_instances_legacy_token_hash_check
    check (legacy_token_hash is null or octet_length(legacy_token_hash) = 32);

alter table linked_instances add column application_name text;
update linked_instances
   set application_name = left(btrim(instance_name), 64),
       instance_name = left(btrim(instance_name), 64);
alter table linked_instances alter column application_name set not null;
alter table linked_instances add column application_version text;
alter table linked_instances add column protocol_version integer;
alter table linked_instances add column capabilities text[] not null default '{}';
alter table linked_instances add column accepted_targets text[] not null default '{}';

alter table linked_instances add constraint linked_instances_application_name_check
    check (application_name = btrim(application_name)
       and char_length(application_name) between 1 and 64);
alter table linked_instances add constraint linked_instances_instance_name_length_check
    check (instance_name = btrim(instance_name)
       and char_length(instance_name) between 1 and 64);
alter table linked_instances add constraint linked_instances_application_version_check
    check (application_version is null
        or (application_version = btrim(application_version)
        and char_length(application_version) between 1 and 64));
alter table linked_instances add constraint linked_instances_protocol_version_check
    check (protocol_version is null or protocol_version > 0);
alter table linked_instances add constraint linked_instances_capabilities_check
    check (is_bounded_distinct_text_array(capabilities, 32, 64));
alter table linked_instances add constraint linked_instances_accepted_targets_check
    check (is_bounded_distinct_text_array(accepted_targets, 32, 64));
alter table linked_instances add constraint linked_instances_declaration_state_check
    check (
        (revoked_at is null and (
            protocol_version is not null
            or (legacy_token_hash is not null
                and application_version is null
                and protocol_version is null
                and cardinality(capabilities) = 0
                and cardinality(accepted_targets) = 0)
        ))
        or (revoked_at is not null
            and application_version is null
            and protocol_version is null
            and cardinality(capabilities) = 0
            and cardinality(accepted_targets) = 0)
    );
alter table linked_instances add constraint linked_instances_credential_state_check
    check (
        (revoked_at is not null
            and refresh_token_hash is null
            and legacy_token_hash is null)
        or (revoked_at is null and (
            (refresh_token_hash is not null
                and octet_length(refresh_token_hash) = 32
                and legacy_token_hash is null)
            or (refresh_token_hash is null
                and legacy_token_hash is not null)
        ))
    );

create table instance_access_tokens (
    token_hash  bytea primary key,
    instance_id uuid not null references linked_instances (id) on delete cascade,
    created_at  timestamptz not null default now(),
    expires_at  timestamptz not null,
    constraint instance_access_tokens_hash_check check (octet_length(token_hash) = 32),
    constraint instance_access_tokens_expiry_check check (expires_at > created_at)
);

create index instance_access_tokens_instance_id_idx
    on instance_access_tokens (instance_id);
create index instance_access_tokens_expires_at_idx
    on instance_access_tokens (expires_at);

create table instance_refresh_history (
    token_hash       bytea primary key,
    instance_id      uuid not null references linked_instances (id) on delete cascade,
    rotated_at       timestamptz not null default now(),
    detectable_until timestamptz not null,
    constraint instance_refresh_history_hash_check check (octet_length(token_hash) = 32),
    constraint instance_refresh_history_detection_check
        check (detectable_until > rotated_at)
);

create index instance_refresh_history_instance_id_idx
    on instance_refresh_history (instance_id);
create index instance_refresh_history_detectable_until_idx
    on instance_refresh_history (detectable_until);

create table link_rate_limits (
    key_hash     bytea not null,
    action       text not null,
    attempts     integer not null,
    window_start timestamptz not null,
    primary key (key_hash, action),
    constraint link_rate_limits_key_hash_check check (octet_length(key_hash) = 32),
    constraint link_rate_limits_action_check
        check (action = btrim(action) and char_length(action) between 1 and 64),
    constraint link_rate_limits_attempts_check check (attempts > 0)
);

create index link_rate_limits_window_start_idx on link_rate_limits (window_start);

-- +goose Down
drop table link_rate_limits;
drop table instance_refresh_history;
drop table instance_access_tokens;
drop table link_authorizations;
drop table link_requests;

-- A refresh token must never become the old permanent bearer credential if a
-- deployment rolls back after new instances have linked.
update linked_instances
   set refresh_token_hash = null,
       revoked_at = coalesce(revoked_at, now())
 where refresh_token_hash is not null;

alter table linked_instances drop constraint linked_instances_credential_state_check;
alter table linked_instances drop constraint linked_instances_declaration_state_check;
alter table linked_instances drop constraint linked_instances_accepted_targets_check;
alter table linked_instances drop constraint linked_instances_capabilities_check;
alter table linked_instances drop constraint linked_instances_protocol_version_check;
alter table linked_instances drop constraint linked_instances_application_version_check;
alter table linked_instances drop constraint linked_instances_instance_name_length_check;
alter table linked_instances drop constraint linked_instances_application_name_check;
alter table linked_instances drop constraint linked_instances_legacy_token_hash_check;
alter table linked_instances drop column accepted_targets;
alter table linked_instances drop column capabilities;
alter table linked_instances drop column protocol_version;
alter table linked_instances drop column application_version;
alter table linked_instances drop column application_name;
alter table linked_instances drop column refresh_token_hash;

alter table linked_instances rename constraint linked_instances_refresh_token_prefix_check
    to linked_instances_token_prefix_check;
alter table linked_instances rename constraint linked_instances_legacy_token_hash_key
    to linked_instances_token_hash_key;
alter table linked_instances rename constraint linked_instances_instance_name_check
    to linked_instances_name_check;
alter table linked_instances rename column refresh_token_prefix to token_prefix;
alter table linked_instances rename column legacy_token_hash to token_hash;
alter table linked_instances rename column instance_name to name;
alter table linked_instances add constraint linked_instances_credential_check check (
    (revoked_at is null and token_hash is not null and octet_length(token_hash) = 32)
    or (revoked_at is not null and token_hash is null)
);

create table link_requests (
    device_code_hash bytea primary key,
    user_code        text not null unique,
    client_name      text not null,
    scopes           text[] not null,
    approved_by      uuid references users (id) on delete cascade,
    approved_at      timestamptz,
    redeemed_at      timestamptz,
    last_polled_at   timestamptz,
    created_at       timestamptz not null default now(),
    expires_at       timestamptz not null,
    constraint link_requests_device_code_hash_check check (octet_length(device_code_hash) = 32),
    constraint link_requests_user_code_check check (user_code ~ '^[BCDFGHJKLMNPQRSTVWXZ23456789]{8}$'),
    constraint link_requests_client_name_check check (btrim(client_name) <> ''),
    constraint link_requests_scopes_check check (is_instance_scope_set(scopes)),
    constraint link_requests_approval_check check (
        (approved_by is null and approved_at is null)
        or (approved_by is not null and approved_at is not null)
    ),
    constraint link_requests_redemption_check check (redeemed_at is null or approved_at is not null)
);

create index link_requests_expires_at_idx on link_requests (expires_at);

create table link_code_attempts (
    user_id      uuid primary key references users (id) on delete cascade,
    failures     integer not null,
    window_start timestamptz not null,
    constraint link_code_attempts_failures_check check (failures > 0)
);

drop function is_bounded_distinct_text_array(text[], integer, integer);
