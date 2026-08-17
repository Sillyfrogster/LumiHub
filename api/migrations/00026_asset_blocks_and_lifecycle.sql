-- +goose Up
-- One row per block. Elements have no rows of their own, so one edit rewrites
-- one block and leaves every other block untouched.
create table asset_blocks (
    id         uuid primary key,
    asset_id   uuid not null references assets (id) on delete cascade,
    definition text not null,
    -- Null means the definition's current default, so improved wording reaches
    -- everybody who never wrote their own.
    title      text,
    position   integer not null,
    hidden     boolean not null default false,
    layout     text not null,
    width      text not null,
    elements   jsonb not null default '[]'::jsonb,
    constraint asset_blocks_layout_check
        check (layout in ('single', 'duo', 'main-aside', 'trio', 'stack-2', 'stack-3')),
    constraint asset_blocks_width_check
        check (width in ('full', 'two_thirds', 'half', 'third')),
    constraint asset_blocks_elements_check check (jsonb_typeof(elements) = 'array'),
    -- Deferred because a reorder rewrites every position in one statement.
    constraint asset_blocks_position_unique unique (asset_id, position)
        deferrable initially deferred
);

create index asset_blocks_page_idx on asset_blocks (asset_id, position);

-- Lifecycle is its own axis. Discovery keeps its two values and applies to a
-- published asset only.
alter table assets add column lifecycle text not null default 'published';
alter table assets alter column lifecycle drop default;
alter table assets
    add constraint assets_lifecycle_check check (lifecycle in ('draft', 'published'));

-- The adult content answer has a real unanswered state while an asset is a
-- draft, so nothing answers it on a creator's behalf.
alter table assets alter column is_nsfw drop not null;
alter table assets alter column is_nsfw drop default;

-- +goose Down
alter table assets alter column is_nsfw set default false;
update assets set is_nsfw = false where is_nsfw is null;
alter table assets alter column is_nsfw set not null;

alter table assets drop constraint assets_lifecycle_check;
alter table assets drop column lifecycle;

drop table asset_blocks;
