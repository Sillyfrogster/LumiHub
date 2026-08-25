-- +goose Up
create table assets (
    id                uuid primary key,
    kind              text not null,
    platform          text,
    format            text not null,
    format_version    text not null default '',
    current_source_id uuid,
    owner_id          uuid,
    name              text not null,
    description       text not null default '',
    tags              text[] not null default '{}',
    preview_media_id  uuid,
    is_nsfw           boolean not null default false,
    publication       text not null default 'unlisted',
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now(),
    constraint assets_publication_check check (publication in ('public', 'unlisted'))
);

create table asset_sources (
    id           uuid primary key,
    asset_id     uuid not null references assets (id) on delete cascade,
    revision     integer not null,
    content_hash text not null,
    byte_size    bigint not null,
    storage_key  text not null,
    media_type   text not null,
    created_at   timestamptz not null default now(),
    unique (asset_id, revision)
);

alter table assets
    add constraint assets_current_source_fk
    foreign key (current_source_id) references asset_sources (id);

create table asset_facets (
    asset_id  uuid not null references assets (id) on delete cascade,
    source_id uuid not null references asset_sources (id) on delete cascade,
    key       text not null,
    value     text not null,
    primary key (asset_id, key, value)
);

create index asset_facets_key_value_idx on asset_facets (key, value);

create table asset_media (
    id          uuid primary key,
    asset_id    uuid not null references assets (id) on delete cascade,
    source_id   uuid not null references asset_sources (id) on delete cascade,
    role        text not null,
    storage_key text not null,
    width       integer,
    height      integer,
    created_at  timestamptz not null default now()
);

create table asset_moderation (
    asset_id   uuid primary key references assets (id) on delete cascade,
    state      text not null default 'allowed',
    updated_at timestamptz not null default now(),
    constraint asset_moderation_state_check check (state in ('allowed', 'hidden', 'removed'))
);

create index assets_browse_idx on assets (kind, created_at desc) where publication = 'public';
create index assets_platform_idx on assets (platform) where platform is not null;
create index assets_tags_idx on assets using gin (tags);

-- +goose Down
drop table asset_moderation;
drop table asset_media;
drop table asset_facets;
alter table assets drop constraint assets_current_source_fk;
drop table asset_sources;
drop table assets;
