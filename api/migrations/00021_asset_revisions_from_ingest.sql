-- +goose Up
alter table ingest_operations
    add column target_asset_id uuid references assets (id) on delete cascade;

-- An ingest that creates an asset stays one to one with it. An ingest that adds
-- a revision names the asset it is adding to, and an asset takes many of those.
alter table ingest_operations drop constraint ingest_operations_asset_id_key;
create unique index ingest_operations_created_asset_idx
    on ingest_operations (asset_id)
    where target_asset_id is null and asset_id is not null;

alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
alter table ingest_operations
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'wrong_kind',
            'internal_failure'
        ));

alter table assets
    add constraint assets_preview_media_fk
        foreign key (preview_media_id) references asset_media (id) on delete set null;

-- +goose Down
alter table assets drop constraint assets_preview_media_fk;

alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
update ingest_operations
   set failure_reason = 'internal_failure'
 where failure_reason = 'wrong_kind';
alter table ingest_operations
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'internal_failure'
        ));

delete from ingest_operations where target_asset_id is not null;
drop index ingest_operations_created_asset_idx;
alter table ingest_operations add constraint ingest_operations_asset_id_key unique (asset_id);
alter table ingest_operations drop column target_asset_id;
