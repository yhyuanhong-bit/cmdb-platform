DROP TRIGGER IF EXISTS audit_events_no_update ON audit_events;
DROP TRIGGER IF EXISTS audit_events_no_delete ON audit_events;
DROP FUNCTION IF EXISTS prevent_audit_mutation();

ALTER TABLE assets DROP CONSTRAINT IF EXISTS chk_assets_status;
ALTER TABLE work_orders DROP CONSTRAINT IF EXISTS chk_work_orders_status;
ALTER TABLE alert_events DROP CONSTRAINT IF EXISTS chk_alert_events_status;
ALTER TABLE inventory_tasks DROP CONSTRAINT IF EXISTS chk_inventory_tasks_status;
