-- +goose Up
alter table asset_media
    add constraint asset_media_role_check
        check (role in ('avatar', 'expression', 'gallery', 'avatar_alt', 'perspective_layer')),
    add constraint asset_media_dimensions_check
        check (
            (width is null and height is null)
            or (width > 0 and height > 0)
        );

create index asset_media_asset_id_idx
    on asset_media (asset_id) where asset_id is not null;
create index asset_media_revision_id_idx
    on asset_media (revision_id) where revision_id is not null;

-- +goose Down
drop index asset_media_revision_id_idx;
drop index asset_media_asset_id_idx;
alter table asset_media
    drop constraint asset_media_dimensions_check,
    drop constraint asset_media_role_check;
