-- +goose Up
alter table users
    add column show_nsfw_contributions_on_profile boolean not null default false;

-- +goose Down
alter table users drop column show_nsfw_contributions_on_profile;
