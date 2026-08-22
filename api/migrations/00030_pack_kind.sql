-- +goose Up
alter table assets drop constraint assets_kind_check;
alter table assets add constraint assets_kind_check
    check (kind in ('character', 'lorebook', 'preset', 'theme', 'pack'));

alter table asset_media drop constraint asset_media_role_check;
alter table asset_media add constraint asset_media_role_check
    check (role in ('avatar', 'expression', 'gallery', 'avatar_alt', 'perspective_layer', 'pack_item'));

-- +goose Down
alter table asset_media drop constraint asset_media_role_check;
alter table asset_media add constraint asset_media_role_check
    check (role in ('avatar', 'expression', 'gallery', 'avatar_alt', 'perspective_layer'));

alter table assets drop constraint assets_kind_check;
alter table assets add constraint assets_kind_check
    check (kind in ('character', 'lorebook', 'preset', 'theme'));
