-- +goose Up
-- created_at is the asset's own date. indexed_at is when our copy appeared.
alter table assets add column indexed_at timestamptz not null default now();

-- Browse reads kind, then the date, then the id, so the index follows that order.
drop index assets_browse_idx;
create index assets_browse_idx
    on assets (kind, created_at desc, id desc)
    where publication = 'public';

-- +goose Down
drop index assets_browse_idx;
create index assets_browse_idx on assets (kind, created_at desc) where publication = 'public';
alter table assets drop column indexed_at;
