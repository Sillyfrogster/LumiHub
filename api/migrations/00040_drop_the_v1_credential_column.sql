-- +goose Up
-- No v1 credential crosses the migration, so the hash column and the checks that admitted it are unreachable.
alter table linked_instances drop constraint linked_instances_declaration_state_check;
alter table linked_instances add constraint linked_instances_declaration_state_check
    check (
        (revoked_at is null and protocol_version is not null)
        or (revoked_at is not null
            and application_version is null
            and protocol_version is null
            and cardinality(capabilities) = 0
            and cardinality(accepted_targets) = 0)
    );

alter table linked_instances drop constraint linked_instances_credential_state_check;
alter table linked_instances add constraint linked_instances_credential_state_check
    check (
        (revoked_at is not null and refresh_token_hash is null)
        or (revoked_at is null
            and refresh_token_hash is not null
            and octet_length(refresh_token_hash) = 32)
    );

alter table linked_instances drop constraint linked_instances_legacy_token_hash_check;
alter table linked_instances drop column legacy_token_hash;

-- +goose Down
alter table linked_instances add column legacy_token_hash bytea unique;
alter table linked_instances add constraint linked_instances_legacy_token_hash_check
    check (legacy_token_hash is null or octet_length(legacy_token_hash) = 32);

alter table linked_instances drop constraint linked_instances_credential_state_check;
alter table linked_instances add constraint linked_instances_credential_state_check
    check (
        (revoked_at is not null
            and refresh_token_hash is null
            and legacy_token_hash is null)
        or (revoked_at is null and (
            (refresh_token_hash is not null
                and octet_length(refresh_token_hash) = 32
                and legacy_token_hash is null)
            or (refresh_token_hash is null
                and legacy_token_hash is not null)
        ))
    );

alter table linked_instances drop constraint linked_instances_declaration_state_check;
alter table linked_instances add constraint linked_instances_declaration_state_check
    check (
        (revoked_at is null and (
            protocol_version is not null
            or (legacy_token_hash is not null
                and application_version is null
                and protocol_version is null
                and cardinality(capabilities) = 0
                and cardinality(accepted_targets) = 0)
        ))
        or (revoked_at is not null
            and application_version is null
            and protocol_version is null
            and cardinality(capabilities) = 0
            and cardinality(accepted_targets) = 0)
    );
