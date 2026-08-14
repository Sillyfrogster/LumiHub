-- +goose Up
alter table users
    add column role text not null default 'user',
    add constraint users_role_check check (role in ('user', 'moderator', 'admin'));

alter table assets
    add constraint assets_withheld_by_fkey
        foreign key (withheld_by) references users (id);

-- +goose Down
alter table assets drop constraint assets_withheld_by_fkey;
alter table users drop constraint users_role_check, drop column role;
