-- +goose Up
create table ingest_operations (
    id                   uuid primary key,
    owner_id             uuid not null references users (id) on delete cascade,
    blob_id              uuid references blobs (id),
    filename             text not null,
    status               text not null default 'pending',
    kind                 text,
    passthrough_platform text,
    name                 text,
    description          text,
    tags                 text[],
    is_nsfw              boolean,
    discovery            text not null default 'listed',
    asset_id             uuid unique references assets (id),
    failure_reason       text,
    attempts             integer not null default 0,
    available_at         timestamptz not null default now(),
    lease_token          uuid,
    lease_expires_at     timestamptz,
    expires_at           timestamptz,
    created_at           timestamptz not null default now(),
    updated_at           timestamptz not null default now(),
    constraint ingest_operations_status_check
        check (status in ('pending', 'processing', 'needs_kind', 'failed', 'success')),
    constraint ingest_operations_kind_check
        check (kind is null or kind in ('character', 'lorebook', 'preset', 'theme')),
    constraint ingest_operations_discovery_check
        check (discovery in ('listed', 'unlisted')),
    constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'internal_failure'
        )),
    constraint ingest_operations_lease_check
        check ((lease_token is null) = (lease_expires_at is null)),
    constraint ingest_operations_result_check
        check (
            (status = 'success' and asset_id is not null and failure_reason is null)
            or (status = 'failed' and asset_id is null and failure_reason is not null)
            or (status in ('pending', 'processing', 'needs_kind')
                and asset_id is null and failure_reason is null)
        )
);

create index ingest_operations_work_idx
    on ingest_operations (available_at, created_at)
    where status in ('pending', 'processing');

create index ingest_operations_needs_kind_expiry_idx
    on ingest_operations (expires_at)
    where status = 'needs_kind';

-- +goose Down
drop table ingest_operations;
