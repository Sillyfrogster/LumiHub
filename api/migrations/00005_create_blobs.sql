-- +goose Up
create table blobs (
    id          uuid primary key,
    sha256      bytea not null unique,
    byte_size   bigint not null check (byte_size >= 0),
    storage_key text not null unique,
    constraint blobs_sha256_length_check check (octet_length(sha256) = 32)
);

-- +goose Down
drop table blobs;
