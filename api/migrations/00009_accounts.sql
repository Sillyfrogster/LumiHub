-- +goose Up
create table users (
    id                uuid primary key,
    username          text not null unique,
    email             text,
    password_hash     text,
    email_verified_at timestamptz,
    email_source      text,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now(),
    constraint users_username_check check (
        username ~ '^[a-z0-9._]{3,32}$'
        and username !~ '^[0-9]+$'
        and username !~ '^[._]+$'
    ),
    constraint users_email_source_check check (email_source in ('creator', 'discord')),
    constraint users_email_shape_check check (
        (email is null and email_source is null and email_verified_at is null)
        or (email is not null and email_source is not null)
    ),
    constraint users_email_lowercase_check check (email = lower(email))
);

create unique index users_verified_email_idx
    on users (email)
    where email_verified_at is not null;

create table retired_handles (
    handle text primary key,
    constraint retired_handles_handle_check check (
        handle ~ '^[a-z0-9._]{3,32}$'
        and handle !~ '^[0-9]+$'
        and handle !~ '^[._]+$'
    )
);

create table email_verification_tokens (
    token_hash bytea primary key,
    user_id    uuid not null unique references users (id) on delete cascade,
    email      text not null,
    expires_at timestamptz not null,
    constraint email_verification_token_hash_check check (octet_length(token_hash) = 32)
);

create table sessions (
    token_hash bytea primary key,
    user_id    uuid not null references users (id) on delete cascade,
    expires_at timestamptz not null,
    constraint session_token_hash_check check (octet_length(token_hash) = 32)
);

create index sessions_user_id_idx on sessions (user_id);
create index sessions_expires_at_idx on sessions (expires_at);

-- +goose Down
drop table sessions;
drop table email_verification_tokens;
drop table retired_handles;
drop table users;
