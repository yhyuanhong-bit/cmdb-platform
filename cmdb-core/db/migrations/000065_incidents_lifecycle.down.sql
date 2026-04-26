-- Reverses 000065. Drops the timeline table first so the FK doesn't block
-- the column drop on rollback.
DROP TABLE IF EXISTS incident_comments;

DROP TRIGGER IF EXISTS incidents_set_updated_at ON incidents;
DROP FUNCTION IF EXISTS trg_incidents_set_updated_at();

ALTER TABLE incidents
    DROP CONSTRAINT IF EXISTS chk_incidents_priority,
    DROP CONSTRAINT IF EXISTS chk_incidents_severity,
    DROP CONSTRAINT IF EXISTS chk_incidents_status;

DROP INDEX IF EXISTS idx_incidents_assignee;
DROP INDEX IF EXISTS idx_incidents_affected_service;
DROP INDEX IF EXISTS idx_incidents_affected_asset;
DROP INDEX IF EXISTS idx_incidents_priority;

ALTER TABLE incidents
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS assignee_user_id,
    DROP COLUMN IF EXISTS affected_asset_id,
    DROP COLUMN IF EXISTS affected_service_id,
    DROP COLUMN IF EXISTS acknowledged_at,
    DROP COLUMN IF EXISTS acknowledged_by,
    DROP COLUMN IF EXISTS resolved_by,
    DROP COLUMN IF EXISTS root_cause,
    DROP COLUMN IF EXISTS impact,
    DROP COLUMN IF EXISTS updated_at;
