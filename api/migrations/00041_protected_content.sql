-- +goose Up

create table protected_content (
    asset_id     uuid not null references assets (id) on delete cascade,
    owner_kind   text not null,
    owner_id     uuid not null,
    payload_type text not null,
    payload      jsonb not null,
    source_key   text,
    digest       bytea not null,
    primary key (asset_id, owner_kind, owner_id),
    constraint protected_content_owner_check
        check (owner_kind = 'prompt_fragment'),
    constraint protected_content_payload_check
        check (payload_type = 'prompt_fragment_text'),
    constraint protected_content_source_key_check
        check (source_key is null or (source_key = btrim(source_key)
            and char_length(source_key) between 1 and 256)),
    constraint protected_content_digest_check check (octet_length(digest) = 32)
);

create table protected_delivery_apps (
    asset_id uuid not null references assets (id) on delete cascade,
    app      text not null,
    primary key (asset_id, app),
    constraint protected_delivery_apps_known_check check (app in ('lumiverse'))
);

-- +goose Down
drop table protected_delivery_apps;
drop table protected_content;
