-- +goose Up
-- A creator's content lives in elements now, and a writer builds a file out of
-- them. Nothing layers a patch over a stored original any more, so the closed
-- eight-field list, the reconciliation patch and additions as a concept go.
drop table file_field_patches;

-- Preserved data is kept as `json` rather than `jsonb`. jsonb stores a parsed
-- value, so it reorders keys and drops duplicates, and a namespace that went in
-- as {"depth":4,"prompt":"","role":"system"} comes back in another order. The
-- promise this table exists for is that what a file carried returns exactly as
-- it arrived, and only the text type keeps it.
alter table asset_preserved_data alter column payload type json using payload::json;

-- One derived row per asset. The export half holds the targets a download
-- menu offers and what each one costs, under the stamp of the declarations it
-- was computed from: a deploy that changes a declaration recomputes what it
-- invalidated. Facets join it later with a stamp of their own, because the two
-- halves follow different rules.
create table asset_projections (
    asset_id     uuid primary key references assets (id) on delete cascade,
    export       jsonb not null,
    export_stamp text not null,
    computed_at  timestamptz not null default now(),
    constraint asset_projections_export_check check (jsonb_typeof(export) = 'array')
);

create index asset_projections_export_stamp_idx on asset_projections (export_stamp);

-- +goose Down
alter table asset_preserved_data alter column payload type jsonb using payload::jsonb;

drop table asset_projections;

create table file_field_patches (
    asset_id    uuid not null references assets (id) on delete cascade,
    revision_id uuid,
    field       text not null,
    value       text not null,
    provenance  text not null,
    constraint file_field_patches_field_check check (field in (
        'description',
        'personality',
        'scenario',
        'first_mes',
        'system_prompt',
        'post_history_instructions',
        'creator_notes',
        'character_version'
    )),
    constraint file_field_patches_provenance_check
        check (provenance in ('creator', 'reconciliation')),
    constraint file_field_patches_scope_check check (
        (provenance = 'creator' and revision_id is null)
        or
        (provenance = 'reconciliation' and revision_id is not null)
    ),
    constraint file_field_patches_revision_fk
        foreign key (revision_id, asset_id)
        references asset_revisions (id, asset_id) on delete cascade
);

create unique index file_field_patches_creator_idx
    on file_field_patches (asset_id, field)
    where provenance = 'creator';

create unique index file_field_patches_reconciliation_idx
    on file_field_patches (revision_id, field)
    where provenance = 'reconciliation';
