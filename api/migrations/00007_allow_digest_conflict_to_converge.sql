-- +goose Up
alter table blobs drop constraint blobs_storage_key_key;

-- +goose Down
alter table blobs add constraint blobs_storage_key_key unique (storage_key);
