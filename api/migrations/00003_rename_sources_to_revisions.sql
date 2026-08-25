-- +goose Up
alter table asset_sources rename to asset_revisions;
alter table assets rename column current_source_id to current_revision_id;
alter table asset_facets rename column source_id to revision_id;
alter table asset_media rename column source_id to revision_id;

alter index asset_sources_pkey rename to asset_revisions_pkey;
alter index asset_sources_asset_id_revision_key rename to asset_revisions_asset_id_revision_key;
alter table asset_revisions rename constraint asset_sources_asset_id_fkey to asset_revisions_asset_id_fkey;
alter table assets rename constraint assets_current_source_fk to assets_current_revision_fk;
alter table asset_facets rename constraint asset_facets_source_id_fkey to asset_facets_revision_id_fkey;
alter table asset_media rename constraint asset_media_source_id_fkey to asset_media_revision_id_fkey;

-- +goose Down
alter table asset_media rename constraint asset_media_revision_id_fkey to asset_media_source_id_fkey;
alter table asset_facets rename constraint asset_facets_revision_id_fkey to asset_facets_source_id_fkey;
alter table assets rename constraint assets_current_revision_fk to assets_current_source_fk;
alter table asset_revisions rename constraint asset_revisions_asset_id_fkey to asset_sources_asset_id_fkey;
alter index asset_revisions_asset_id_revision_key rename to asset_sources_asset_id_revision_key;
alter index asset_revisions_pkey rename to asset_sources_pkey;

alter table asset_media rename column revision_id to source_id;
alter table asset_facets rename column revision_id to source_id;
alter table assets rename column current_revision_id to current_source_id;
alter table asset_revisions rename to asset_sources;
