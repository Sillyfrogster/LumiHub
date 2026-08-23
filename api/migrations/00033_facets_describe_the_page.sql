-- +goose Up
alter table asset_projections
    alter column export set default '[]'::jsonb,
    alter column export_stamp set default '',
    add column facets jsonb not null default '{}'::jsonb,
    add column facet_stamp text not null default '',
    add column facet_computed_at timestamptz not null default now(),
    add constraint asset_projections_facets_check check (jsonb_typeof(facets) = 'object');

alter table asset_projections rename column computed_at to export_computed_at;

create index asset_projections_facet_stamp_idx on asset_projections (facet_stamp);

drop table asset_facets;

-- +goose Down
create table asset_facets (
    revision_id uuid not null references asset_revisions (id) on delete cascade,
    key         text not null,
    value       text not null,
    primary key (revision_id, key, value)
);

create index asset_facets_key_value_idx on asset_facets (key, value);

drop index asset_projections_facet_stamp_idx;

alter table asset_projections rename column export_computed_at to computed_at;

alter table asset_projections
    drop constraint asset_projections_facets_check,
    drop column facet_computed_at,
    drop column facet_stamp,
    drop column facets,
    alter column export_stamp drop default,
    alter column export drop default;
