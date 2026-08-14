-- +goose Up
alter table users
    add column nsfw_visibility text not null default 'blurred',
    add constraint users_nsfw_visibility_check
        check (nsfw_visibility in ('hidden', 'blurred', 'shown'));

-- +goose Down
alter table users
    drop constraint users_nsfw_visibility_check,
    drop column nsfw_visibility;
