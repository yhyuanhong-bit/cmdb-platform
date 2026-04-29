-- ============================================================================
-- 000077 (down): Remove racks.status CHECK constraint
-- ============================================================================
BEGIN;

ALTER TABLE racks DROP CONSTRAINT IF EXISTS chk_racks_status;

COMMIT;
