-- +goose Up

create table instance_deliveries (
    id               uuid primary key,
    instance_id      uuid not null references linked_instances (id) on delete cascade,
    asset_id         uuid not null references assets (id) on delete cascade,
    state            text not null default 'queued',
    attempts         integer not null default 0,
    queued_at        timestamptz not null default now(),
    lease_expires_at timestamptz,
    chosen_target    text,
    settled_at       timestamptz,
    settled_reason   text,
    expires_at       timestamptz not null,
    constraint instance_deliveries_state_check
        check (state in ('queued', 'released', 'failed')),
    constraint instance_deliveries_attempts_check check (attempts >= 0),
    constraint instance_deliveries_lease_check
        check ((state = 'released') = (lease_expires_at is not null)),
    constraint instance_deliveries_settled_check
        check ((state = 'failed') = (settled_at is not null)
           and (state = 'failed') = (settled_reason is not null)),
    constraint instance_deliveries_reason_check
        check (settled_reason is null
            or settled_reason in ('withdrawn', 'unsupported', 'abandoned')),
    constraint instance_deliveries_target_check
        check (chosen_target is null
            or (chosen_target = btrim(chosen_target)
            and char_length(chosen_target) between 1 and 64)),
    constraint instance_deliveries_expiry_check check (expires_at > queued_at)
);

-- One live delivery per asset per instance, so pressing send twice queues once.
create unique index instance_deliveries_live_idx
    on instance_deliveries (instance_id, asset_id)
 where state in ('queued', 'released');

create index instance_deliveries_collect_idx
    on instance_deliveries (instance_id, state, queued_at);
create index instance_deliveries_expires_at_idx on instance_deliveries (expires_at);
create index instance_deliveries_asset_id_idx on instance_deliveries (asset_id);

-- The mirror an instance writes, whose generation against the asset's answers installed and out of date.
create table instance_library_entries (
    instance_id        uuid not null references linked_instances (id) on delete cascade,
    asset_id           uuid not null references assets (id) on delete cascade,
    content_generation integer not null,
    reported_at        timestamptz not null default now(),
    primary key (instance_id, asset_id),
    constraint instance_library_entries_generation_check
        check (content_generation > 0)
);

create index instance_library_entries_asset_id_idx
    on instance_library_entries (asset_id);

-- +goose Down
drop table instance_library_entries;
drop table instance_deliveries;
