-- +goose Up
alter table asset_media
    add column is_extracted boolean not null default false,
    add column is_current boolean not null default true;

alter table asset_media drop constraint asset_media_provenance_check;

update asset_media media
   set asset_id = revision.asset_id,
       is_extracted = true,
       is_current = revision.id = asset.current_revision_id
  from asset_revisions revision
  join assets asset on asset.id = revision.asset_id
 where media.revision_id = revision.id;

update assets asset
   set preview_media_id = null
 where asset.preview_media_id is not null
   and not exists (
       select 1
         from asset_media media
        where media.id = asset.preview_media_id
          and media.asset_id = asset.id
          and media.is_current
   );

update assets asset
   set preview_media_id = (
       select media.id
         from asset_media media
        where media.asset_id = asset.id
          and media.is_current
          and media.role in ('avatar', 'avatar_alt')
          and media.blob_id is not null
        order by case media.role when 'avatar' then 1 else 2 end,
                 media.created_at desc, media.id desc
        limit 1
   )
 where asset.preview_media_id is null
   and exists (
       select 1
         from asset_media media
        where media.asset_id = asset.id
          and media.is_current
          and media.role in ('avatar', 'avatar_alt')
          and media.blob_id is not null
   );

drop index asset_media_revision_id_idx;
alter table asset_media
    drop constraint asset_media_revision_id_fkey,
    alter column asset_id set not null,
    drop column revision_id;

alter table assets
    rename constraint assets_preview_media_fk to assets_cover_media_fk;
alter table assets rename column preview_media_id to cover_media_id;

-- +goose Down
alter table assets rename column cover_media_id to preview_media_id;
alter table assets
    rename constraint assets_cover_media_fk to assets_preview_media_fk;

alter table asset_media
    alter column asset_id drop not null,
    add column revision_id uuid,
    add constraint asset_media_revision_id_fkey
        foreign key (revision_id) references asset_revisions (id) on delete cascade,
    add constraint asset_media_provenance_check
        check ((asset_id is null) <> (revision_id is null));
create index asset_media_revision_id_idx
    on asset_media (revision_id) where revision_id is not null;

alter table asset_media
    drop column if exists is_current,
    drop column if exists is_extracted;
