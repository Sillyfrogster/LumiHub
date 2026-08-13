-- +goose Up
alter table asset_revisions add column blob_id uuid;

insert into blobs (id, sha256, byte_size, storage_key)
select gen_random_uuid(), decode(content_hash, 'hex'), byte_size, storage_key
from (
    select distinct on (content_hash)
           content_hash, byte_size, storage_key
      from asset_revisions
     order by content_hash, id
) existing_blobs;

update asset_revisions r
   set blob_id = b.id
  from blobs b
 where b.sha256 = decode(r.content_hash, 'hex');

alter table asset_revisions
    alter column blob_id set not null,
    add constraint asset_revisions_blob_id_fkey
        foreign key (blob_id) references blobs (id);

alter table asset_revisions
    drop column content_hash,
    drop column byte_size,
    drop column storage_key;

alter table asset_media
    add column blob_id uuid not null references blobs (id),
    drop column storage_key;

-- +goose Down
alter table asset_media add column storage_key text;
update asset_media m set storage_key = b.storage_key from blobs b where b.id = m.blob_id;
alter table asset_media alter column storage_key set not null;
alter table asset_media drop column blob_id;

alter table asset_revisions
    add column content_hash text,
    add column byte_size bigint,
    add column storage_key text;

update asset_revisions r
   set content_hash = encode(b.sha256, 'hex'),
       byte_size = b.byte_size,
       storage_key = b.storage_key
  from blobs b
 where b.id = r.blob_id;

alter table asset_revisions
    alter column content_hash set not null,
    alter column byte_size set not null,
    alter column storage_key set not null,
    drop column blob_id;
