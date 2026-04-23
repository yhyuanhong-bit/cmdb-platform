-- Roll back all status CHECK constraints from migration 000062.
BEGIN;

ALTER TABLE assets             DROP CONSTRAINT IF EXISTS chk_assets_status;
ALTER TABLE work_orders        DROP CONSTRAINT IF EXISTS chk_work_orders_status;
ALTER TABLE discovered_assets  DROP CONSTRAINT IF EXISTS chk_discovered_assets_status;
ALTER TABLE alert_events       DROP CONSTRAINT IF EXISTS chk_alert_events_status;
ALTER TABLE inventory_tasks    DROP CONSTRAINT IF EXISTS chk_inventory_tasks_status;

COMMIT;
