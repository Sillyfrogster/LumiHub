-- +goose Up
alter table assets
    add column asset_version text not null default '',
    add column credited_author text not null default '',
    add column nickname text not null default '',
    add column origin_format text,
    add column content_generation integer not null default 1,
    add constraint assets_content_generation_check check (content_generation > 0);

create table asset_preserved_data (
    id         uuid primary key,
    asset_id   uuid not null references assets (id) on delete cascade,
    owner_kind text not null,
    owner_id   uuid not null,
    namespace  text not null,
    payload    jsonb not null,
    constraint asset_preserved_data_owner_kind_check
        check (owner_kind in ('asset', 'element', 'item')),
    unique (asset_id, owner_kind, owner_id, namespace)
);

create index asset_preserved_data_namespaces_idx
    on asset_preserved_data (asset_id, namespace);

delete from ingest_operations where status = 'needs_kind';
drop index ingest_operations_needs_kind_expiry_idx;
alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
alter table ingest_operations drop constraint ingest_operations_result_check;
alter table ingest_operations drop constraint ingest_operations_status_check;
alter table ingest_operations
    add constraint ingest_operations_status_check
        check (status in ('pending', 'processing', 'failed', 'success')),
    add constraint ingest_operations_result_check
        check (
            (status = 'success' and asset_id is not null and failure_reason is null)
            or (status = 'failed' and asset_id is null and failure_reason is not null)
            or (status in ('pending', 'processing')
                and asset_id is null and failure_reason is null)
        ),
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input', 'unsupported_format', 'unsupported_version',
            'safety_violation', 'wrong_kind', 'internal_failure'
        )),
    add column failure_message text,
    drop column kind,
    drop column expires_at;

alter table asset_revisions drop column passthrough_platform;

-- +goose Down
alter table asset_revisions add column passthrough_platform text;

alter table ingest_operations drop constraint ingest_operations_result_check;
alter table ingest_operations drop constraint ingest_operations_status_check;
alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
alter table ingest_operations
    add column kind text,
    add column expires_at timestamptz,
    drop column failure_message,
    add constraint ingest_operations_status_check
        check (status in ('pending', 'processing', 'needs_kind', 'failed', 'success')),
    add constraint ingest_operations_result_check
        check (
            (status = 'success' and asset_id is not null and failure_reason is null)
            or (status = 'failed' and asset_id is null and failure_reason is not null)
            or (status in ('pending', 'processing', 'needs_kind')
                and asset_id is null and failure_reason is null)
        ),
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input', 'unsupported_format', 'unsupported_version',
            'safety_violation', 'wrong_kind', 'internal_failure'
        ));
create index ingest_operations_needs_kind_expiry_idx
    on ingest_operations (expires_at)
    where status = 'needs_kind';

drop table asset_preserved_data;
alter table assets
    drop constraint assets_content_generation_check,
    drop column content_generation,
    drop column origin_format,
    drop column nickname,
    drop column credited_author,
    drop column asset_version;
