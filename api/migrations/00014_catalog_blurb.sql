-- +goose Up
alter table assets rename column description to blurb;
alter table ingest_operations rename column description to blurb;

-- +goose Down
alter table ingest_operations rename column blurb to description;
alter table assets rename column blurb to description;
