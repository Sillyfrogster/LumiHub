-- +goose Up
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

-- +goose Down
drop table file_field_patches;
