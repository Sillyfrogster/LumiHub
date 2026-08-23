-- +goose Up
create table migration_exceptions (
    id          uuid primary key,
    kind        text not null,
    subject     text not null,
    detail      text not null,
    asset_id    uuid references assets (id) on delete cascade,
    recorded_at timestamptz not null default now(),
    constraint migration_exceptions_kind_check check (kind <> ''),
    constraint migration_exceptions_subject_check check (subject <> ''),
    constraint migration_exceptions_detail_check check (detail <> '')
);

create index migration_exceptions_kind_idx on migration_exceptions (kind);

create index migration_exceptions_asset_id_idx
    on migration_exceptions (asset_id)
 where asset_id is not null;

-- +goose Down
drop table migration_exceptions;
