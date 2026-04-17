DROP INDEX IF EXISTS idx_integration_adapters_next_attempt;

ALTER TABLE integration_adapters
    DROP COLUMN IF EXISTS consecutive_failures,
    DROP COLUMN IF EXISTS last_failure_at,
    DROP COLUMN IF EXISTS last_failure_reason,
    DROP COLUMN IF EXISTS next_attempt_at;
