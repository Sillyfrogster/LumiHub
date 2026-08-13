-- +goose Up
create table password_reset_tokens (
    token_hash bytea primary key,
    user_id    uuid not null unique references users (id) on delete cascade,
    expires_at timestamptz not null,
    constraint password_reset_token_hash_check check (octet_length(token_hash) = 32)
);

create index password_reset_tokens_expires_at_idx on password_reset_tokens (expires_at);

-- +goose Down
drop table password_reset_tokens;
