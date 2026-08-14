-- +goose Up

-- +goose StatementBegin
create function is_instance_scope_set(scopes text[]) returns boolean
language sql immutable as $$
    select scopes <@ array['asset:receive', 'library:sync']
       and cardinality(scopes) > 0
       and cardinality(scopes) = (select count(distinct scope) from unnest(scopes) as scope);
$$;
-- +goose StatementEnd

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

create table linked_instances (
    id           uuid primary key,
    user_id      uuid not null references users (id) on delete cascade,
    name         text not null,
    token_hash   bytea unique,
    token_prefix text not null,
    scopes       text[] not null,
    linked_at    timestamptz not null default now(),
    last_seen_at timestamptz,
    revoked_at   timestamptz,
    constraint linked_instances_name_check check (btrim(name) <> ''),
    constraint linked_instances_token_prefix_check
        check (token_prefix ~ '^[BCDFGHJKLMNPQRSTVWXZ23456789]{8}$'),
    constraint linked_instances_scopes_check check (is_instance_scope_set(scopes)),
    -- A revoked instance keeps its name and dates so a creator sees what they
    -- cut, and loses the credential so the cut takes effect.
    constraint linked_instances_credential_check check (
        (revoked_at is null and token_hash is not null and octet_length(token_hash) = 32)
        or (revoked_at is not null and token_hash is null)
    )
);

create index linked_instances_user_id_idx on linked_instances (user_id);

create table link_code_attempts (
    user_id      uuid primary key references users (id) on delete cascade,
    failures     integer not null,
    window_start timestamptz not null,
    constraint link_code_attempts_failures_check check (failures > 0)
);

-- +goose Down
drop table link_code_attempts;
drop table linked_instances;
drop table link_requests;
drop function is_instance_scope_set(text[]);
