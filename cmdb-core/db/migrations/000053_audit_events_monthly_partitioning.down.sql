-- Revert 000053: convert audit_events back to a single heap table.
--
-- Pre-requisites before running this down migration:
--   1. No rows can still live in S3/cold storage — if any month was
--      already archived + dropped, this down migration will silently
--      lose those rows. Operators must explicitly accept that loss
--      or restore the archived partitions first.
--   2. Append-only triggers are removed temporarily to allow the row
--      copy; the final block reinstates them on the restored heap.
--
-- All steps are idempotent so a failed rollback can be retried.

BEGIN;

-- 1. Drop append-only triggers off the partitioned parent so the
--    COPY below can run.
DROP TRIGGER IF EXISTS audit_events_no_delete ON audit_events;
DROP TRIGGER IF EXISTS audit_events_no_update ON audit_events;

-- 2. Copy every still-attached row back to the legacy heap table.
--    ON CONFLICT DO NOTHING makes this safe to retry if the partitioned
--    table was only partially drained before an earlier down attempt.
INSERT INTO audit_events_legacy
    (id, tenant_id, action, module, target_type, target_id,
     operator_id, diff, source, created_at, operator_type)
SELECT id, tenant_id, action, module, target_type, target_id,
       operator_id, diff, source, created_at, operator_type
FROM audit_events
ON CONFLICT (id) DO NOTHING;

-- 3. Tear down the partitioned table; CASCADE drops every child
--    partition along with it.
DROP TABLE audit_events CASCADE;

-- 4. Restore the original table name.
ALTER TABLE audit_events_legacy RENAME TO audit_events;

-- 5. Reinstate append-only triggers on the restored heap.
CREATE TRIGGER audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();
CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

-- 6. Reinstate the 000023 purge function (dead code, but part of the
--    pre-000053 schema for symmetry).
CREATE OR REPLACE FUNCTION purge_old_audit_events() RETURNS void
LANGUAGE SQL AS $$
    DELETE FROM audit_events WHERE created_at < now() - interval '365 days';
$$;

COMMIT;
