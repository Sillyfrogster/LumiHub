-- +goose Up

-- +goose StatementBegin
create function protected_content_requires_delivery_policy()
returns trigger
language plpgsql
as $$
declare
    protected_asset_id uuid;
begin
    if tg_op = 'DELETE' then
        protected_asset_id := old.asset_id;
    else
        protected_asset_id := new.asset_id;
    end if;
    if exists (
        select 1 from protected_content where asset_id = protected_asset_id
    ) then
        if not exists (
            select 1 from protected_delivery_apps where asset_id = protected_asset_id
        ) then
            raise exception 'protected content requires an allowed delivery app';
        end if;
    elsif exists (
        select 1 from protected_delivery_apps where asset_id = protected_asset_id
    ) then
        raise exception 'allowed delivery apps require protected content';
    end if;
    return null;
end;
$$;
-- +goose StatementEnd

create constraint trigger protected_content_policy_after_content_change
after insert or update or delete on protected_content
deferrable initially deferred
for each row execute function protected_content_requires_delivery_policy();

create constraint trigger protected_content_policy_after_policy_change
after insert or update or delete on protected_delivery_apps
deferrable initially deferred
for each row execute function protected_content_requires_delivery_policy();

-- +goose Down
drop trigger protected_content_policy_after_policy_change on protected_delivery_apps;
drop trigger protected_content_policy_after_content_change on protected_content;
drop function protected_content_requires_delivery_policy();
