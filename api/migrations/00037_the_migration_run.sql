-- +goose Up
-- The v1 public path, `creator/name`, kept so a link pasted a year ago lands.
create table asset_legacy_paths (
    path       text primary key,
    asset_id   uuid not null references assets (id) on delete cascade,
    created_at timestamptz not null default now(),
    constraint asset_legacy_paths_path_check check (path <> '')
);

create index asset_legacy_paths_asset_idx on asset_legacy_paths (asset_id);

-- v1 rows kept whole because deleting is irreversible and keeping commits
-- Illarin to nothing. Nothing reads these at runtime.
create table migration_preserved_records (
    id           uuid primary key,
    source_table text  not null,
    source_id    text  not null,
    asset_id     uuid  references assets (id) on delete cascade,
    owner_id     uuid  references users (id) on delete set null,
    payload      jsonb not null,
    constraint migration_preserved_records_source_check check (source_table <> '' and source_id <> ''),
    unique (source_table, source_id)
);

create index migration_preserved_records_asset_idx
    on migration_preserved_records (asset_id)
 where asset_id is not null;

-- v1's own counters, kept beside the asset rather than inside creator content.
create table migration_legacy_counters (
    asset_id   uuid primary key references assets (id) on delete cascade,
    downloads  integer not null,
    views      integer not null,
    favorites  integer not null,
    updated_at timestamptz not null
);

-- What phase one already put on disk. The sweep cannot see this table, so an
-- aborted run leaves unreferenced blobs it collects, and the cascade takes the
-- staging row with them.
create table migration_staged_media (
    source    text primary key,
    blob_id   uuid    not null references blobs (id) on delete cascade,
    width     integer not null,
    height    integer not null,
    staged_at timestamptz not null default now(),
    constraint migration_staged_media_source_check check (source <> ''),
    constraint migration_staged_media_size_check check (width > 0 and height > 0)
);

-- +goose Down
drop table migration_staged_media;
drop table migration_legacy_counters;
drop table migration_preserved_records;
drop table asset_legacy_paths;
