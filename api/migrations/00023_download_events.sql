-- +goose Up
create table download_events (
    id                  bigint generated always as identity primary key,
    asset_id            uuid not null,
    revision_id         uuid not null,
    export_target       text not null,
    handed_off_at       timestamptz not null default now(),
    authorization_class text not null,
    discovery           text not null,
    constraint download_events_revision_fk
        foreign key (revision_id, asset_id)
        references asset_revisions (id, asset_id),
    constraint download_events_export_target_check
        check (btrim(export_target) <> ''),
    constraint download_events_authorization_class_check
        check (authorization_class in ('anonymous', 'signed_in', 'owner', 'linked_instance')),
    constraint download_events_discovery_check
        check (discovery in ('listed', 'unlisted'))
);

-- +goose StatementBegin
create function reject_download_event_mutation() returns trigger
language plpgsql as $$
begin
    raise exception 'download events are append-only';
end;
$$;
-- +goose StatementEnd

create trigger download_events_are_immutable
before update or delete on download_events
for each row execute function reject_download_event_mutation();

create trigger download_events_cannot_be_truncated
before truncate on download_events
for each statement execute function reject_download_event_mutation();

-- +goose Down
drop table download_events;
drop function if exists reject_download_event_mutation();
