-- +goose Up
alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
update ingest_operations
   set failure_reason = 'internal_failure'
 where failure_reason = 'purged_content';
alter table ingest_operations
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'internal_failure'
        ));

-- +goose Down
alter table ingest_operations drop constraint ingest_operations_failure_reason_check;
alter table ingest_operations
    add constraint ingest_operations_failure_reason_check
        check (failure_reason is null or failure_reason in (
            'malformed_input',
            'unsupported_format',
            'unsupported_version',
            'safety_violation',
            'internal_failure',
            'purged_content'
        ));
