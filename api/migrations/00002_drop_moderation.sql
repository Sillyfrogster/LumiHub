-- +goose Up
drop table asset_moderation;

-- +goose Down
create table asset_moderation (
    asset_id   uuid primary key references assets (id) on delete cascade,
    state      text not null,
    updated_at timestamptz not null default now(),
    constraint asset_moderation_state_check check (state in ('allowed', 'hidden', 'removed'))
);
