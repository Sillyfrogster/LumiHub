-- +goose Up
alter table assets
    add column recoverable_until timestamptz,
    add constraint assets_deletion_window_check
        check ((deleted_at is null) = (recoverable_until is null));

alter table asset_revisions alter column blob_id drop not null;
alter table asset_media alter column blob_id drop not null;

create table blob_sweep_marks (
    blob_id   uuid primary key references blobs (id) on delete cascade,
    marked_at timestamptz not null
);

create table blob_tombstones (
    sha256      bytea primary key,
    reason_code text not null check (btrim(reason_code) <> ''),
    purged_at   timestamptz not null,
    actor_id    uuid not null,
    constraint blob_tombstones_sha256_length_check check (octet_length(sha256) = 32)
);

alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
alter table ingest_operations
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'internal_failure',
            'purged_content'
        ));

-- +goose Down
alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
delete from ingest_operations where failure_reason = 'purged_content';
alter table ingest_operations
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'internal_failure'
        ));

drop table blob_tombstones;
drop table if exists blob_sweep_marks;
alter table blobs drop column if exists sweep_marked_at;

delete from assets asset
 where exists (
     select 1 from asset_revisions revision
      where revision.asset_id = asset.id and revision.blob_id is null
 );
delete from asset_media where blob_id is null;

alter table asset_media alter column blob_id set not null;
alter table asset_revisions alter column blob_id set not null;

alter table assets drop constraint assets_deletion_window_check;
alter table assets drop column recoverable_until;
