-- +goose Up
alter table download_events
    alter column revision_id drop not null;

-- +goose Down
-- +goose StatementBegin
do $$
begin
    if exists (select 1 from download_events where revision_id is null) then
        raise exception 'download events without a source revision prevent this rollback';
    end if;
end;
$$;
-- +goose StatementEnd

alter table download_events
    alter column revision_id set not null;
