-- 000027_sync_system.down.sql
DROP TABLE IF EXISTS sync_conflicts;
DROP TABLE IF EXISTS sync_state;

ALTER TABLE work_orders DROP COLUMN IF EXISTS execution_status;
ALTER TABLE work_orders DROP COLUMN IF EXISTS governance_status;

DROP INDEX IF EXISTS idx_assets_sync_version;
DROP INDEX IF EXISTS idx_locations_sync_version;
DROP INDEX IF EXISTS idx_racks_sync_version;
DROP INDEX IF EXISTS idx_work_orders_sync_version;
DROP INDEX IF EXISTS idx_alert_events_sync_version;
DROP INDEX IF EXISTS idx_inventory_tasks_sync_version;

ALTER TABLE assets DROP COLUMN IF EXISTS sync_version;
ALTER TABLE locations DROP COLUMN IF EXISTS sync_version;
ALTER TABLE racks DROP COLUMN IF EXISTS sync_version;
ALTER TABLE work_orders DROP COLUMN IF EXISTS sync_version;
ALTER TABLE alert_events DROP COLUMN IF EXISTS sync_version;
ALTER TABLE inventory_tasks DROP COLUMN IF EXISTS sync_version;
