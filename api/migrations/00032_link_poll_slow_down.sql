-- +goose Up

alter table link_requests
    add column poll_interval_seconds integer not null default 5,
    add constraint link_requests_poll_interval_check
        check (poll_interval_seconds between 5 and 60);

-- +goose Down

alter table link_requests
    drop constraint link_requests_poll_interval_check,
    drop column poll_interval_seconds;
