-- +goose Up
alter table oauth_identities
    drop constraint oauth_identities_user_id_provider_key;

-- +goose Down
alter table oauth_identities
    add constraint oauth_identities_user_id_provider_key unique (user_id, provider);
