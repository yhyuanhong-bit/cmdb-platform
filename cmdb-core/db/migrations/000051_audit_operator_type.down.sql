BEGIN;
ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS chk_audit_operator_type_id_match;
DROP INDEX IF EXISTS idx_audit_events_type_created;
ALTER TABLE audit_events DROP COLUMN IF EXISTS operator_type;
DROP TYPE IF EXISTS audit_operator_type;
COMMIT;
