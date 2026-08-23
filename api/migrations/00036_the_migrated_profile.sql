-- +goose Up
alter table users
    add column display_name         text  not null default '',
    add column custom_display_name  text  not null default '',
    add column avatar_url           text  not null default '',
    add column banner_url           text  not null default '',
    add column default_include_tags jsonb not null default '[]'::jsonb,
    add column default_exclude_tags jsonb not null default '[]'::jsonb,
    add constraint users_default_include_tags_check
        check (jsonb_typeof(default_include_tags) = 'array'),
    add constraint users_default_exclude_tags_check
        check (jsonb_typeof(default_exclude_tags) = 'array');

alter table users drop constraint users_username_check;
alter table users add constraint users_username_check check (
    username ~ '^[a-z0-9._]{3,32}$'
    and username !~ '^[._]+$'
);

alter table retired_handles drop constraint retired_handles_handle_check;
alter table retired_handles add constraint retired_handles_handle_check check (
    handle ~ '^[a-z0-9._]{3,32}$'
    and handle !~ '^[._]+$'
);

-- +goose Down
alter table retired_handles drop constraint retired_handles_handle_check;
alter table retired_handles add constraint retired_handles_handle_check check (
    handle ~ '^[a-z0-9._]{3,32}$'
    and handle !~ '^[0-9]+$'
    and handle !~ '^[._]+$'
);

alter table users drop constraint users_username_check;
alter table users add constraint users_username_check check (
    username ~ '^[a-z0-9._]{3,32}$'
    and username !~ '^[0-9]+$'
    and username !~ '^[._]+$'
);

alter table users
    drop constraint users_default_exclude_tags_check,
    drop constraint users_default_include_tags_check,
    drop column default_exclude_tags,
    drop column default_include_tags,
    drop column banner_url,
    drop column avatar_url,
    drop column custom_display_name,
    drop column display_name;
