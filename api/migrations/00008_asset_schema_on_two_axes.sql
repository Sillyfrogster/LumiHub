-- +goose Up
alter table assets drop constraint assets_publication_check;
alter table assets alter column publication drop default;
update assets set publication = 'listed' where publication = 'public';
alter table assets rename column publication to discovery;
alter table assets alter column discovery set default 'listed';

alter table assets
    add column withheld_at timestamptz,
    add column withheld_by uuid,
    add column withheld_reason text,
    add column deleted_at timestamptz,
    add constraint assets_kind_check
        check (kind in ('character', 'lorebook', 'preset', 'theme')),
    add constraint assets_discovery_check
        check (discovery in ('listed', 'unlisted')),
    add constraint assets_withheld_check
        check (
            (withheld_at is null and withheld_by is null and withheld_reason is null)
            or
            (withheld_at is not null and withheld_by is not null and withheld_reason is not null)
        );

alter table asset_revisions
    add column format text,
    add column passthrough_platform text;

update asset_revisions revision
   set format = asset.format,
       passthrough_platform = asset.platform
  from assets asset
 where asset.id = revision.asset_id;

alter table asset_revisions
    alter column format set not null,
    add constraint asset_revisions_id_asset_id_key unique (id, asset_id);

drop index assets_platform_idx;
drop index assets_browse_idx;

alter table assets
    drop column format_version,
    drop column platform,
    drop column format;

alter table asset_facets drop constraint asset_facets_pkey;
alter table asset_facets drop constraint asset_facets_asset_id_fkey;
alter table asset_facets drop column asset_id;
alter table asset_facets add primary key (revision_id, key, value);

alter table asset_media
    alter column asset_id drop not null,
    alter column revision_id drop not null;
update asset_media set asset_id = null where revision_id is not null;
alter table asset_media
    add constraint asset_media_provenance_check
        check ((asset_id is null) <> (revision_id is null));

create index assets_browse_idx
    on assets (created_at desc, id desc)
    where discovery = 'listed' and withheld_at is null and deleted_at is null;

-- +goose Down
drop index assets_browse_idx;

alter table asset_media drop constraint asset_media_provenance_check;
update asset_media media
   set asset_id = revision.asset_id
  from asset_revisions revision
 where media.revision_id = revision.id and media.asset_id is null;
update asset_media media
   set revision_id = asset.current_revision_id
  from assets asset
 where media.asset_id = asset.id and media.revision_id is null;
alter table asset_media
    alter column asset_id set not null,
    alter column revision_id set not null;

alter table asset_facets drop constraint asset_facets_pkey;
alter table asset_facets add column asset_id uuid;
update asset_facets facet
   set asset_id = revision.asset_id
  from asset_revisions revision
 where facet.revision_id = revision.id;
alter table asset_facets
    alter column asset_id set not null,
    add constraint asset_facets_asset_id_fkey
        foreign key (asset_id) references assets (id) on delete cascade,
    add primary key (asset_id, key, value);

alter table assets
    add column platform text,
    add column format text,
    add column format_version text not null default '';

update assets asset
   set format = coalesce(revision.format, 'unknown'),
       platform = revision.passthrough_platform
  from asset_revisions revision
 where revision.id = asset.current_revision_id;
update assets set format = 'unknown' where format is null;
alter table assets alter column format set not null;

alter table asset_revisions
    drop constraint asset_revisions_id_asset_id_key,
    drop column passthrough_platform,
    drop column format;

alter table assets
    drop constraint assets_withheld_check,
    drop constraint assets_discovery_check,
    drop constraint assets_kind_check,
    drop column deleted_at,
    drop column withheld_reason,
    drop column withheld_by,
    drop column withheld_at;

alter table assets alter column discovery drop default;
update assets set discovery = 'public' where discovery = 'listed';
alter table assets rename column discovery to publication;
alter table assets alter column publication set default 'unlisted';
alter table assets add constraint assets_publication_check
    check (publication in ('public', 'unlisted'));

create index assets_browse_idx
    on assets (kind, created_at desc, id desc)
    where publication = 'public';
create index assets_platform_idx on assets (platform) where platform is not null;
