-- +goose Up
create table oauth_identities (
    user_id        uuid not null references users (id) on delete cascade,
    provider       text not null,
    subject        text not null,
    provider_email text,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    primary key (provider, subject),
    unique (user_id, provider),
    constraint oauth_identities_provider_check check (provider = 'discord'),
    constraint oauth_identities_provider_email_lowercase_check check (
        provider_email = lower(provider_email)
    )
);

create table oauth_states (
    token_hash bytea primary key,
    intent     text not null,
    user_id    uuid references users (id) on delete cascade,
    expires_at timestamptz not null,
    constraint oauth_state_token_hash_check check (octet_length(token_hash) = 32),
    constraint oauth_state_intent_check check (intent in ('sign-in', 'attach')),
    constraint oauth_state_user_check check (
        (intent = 'sign-in' and user_id is null)
        or (intent = 'attach' and user_id is not null)
    )
);

create index oauth_states_expires_at_idx on oauth_states (expires_at);

-- +goose Down
drop table oauth_states;
drop table oauth_identities;
