-- Roll back 000062 by restoring the narrower allowlists from 000028.
-- This only restores the 4 constraints 028 created; the brand-new
-- chk_discovered_assets_status gets dropped outright.

BEGIN;

ALTER TABLE assets DROP CONSTRAINT IF EXISTS chk_assets_status;
ALTER TABLE assets ADD CONSTRAINT chk_assets_status
    CHECK (status IN ('procurement', 'inventoried', 'deploying', 'deployed', 'operational', 'active', 'maintenance', 'decommission', 'retired', 'disposed', 'offline'));

ALTER TABLE work_orders DROP CONSTRAINT IF EXISTS chk_work_orders_status;
ALTER TABLE work_orders ADD CONSTRAINT chk_work_orders_status
    CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'in_progress', 'completed', 'verified'));

ALTER TABLE alert_events DROP CONSTRAINT IF EXISTS chk_alert_events_status;
ALTER TABLE alert_events ADD CONSTRAINT chk_alert_events_status
    CHECK (status IN ('firing', 'acknowledged', 'resolved'));

ALTER TABLE inventory_tasks DROP CONSTRAINT IF EXISTS chk_inventory_tasks_status;
ALTER TABLE inventory_tasks ADD CONSTRAINT chk_inventory_tasks_status
    CHECK (status IN ('planned', 'in_progress', 'completed'));

-- discovered_assets.status was NEW in 000062, so just drop it.
ALTER TABLE discovered_assets DROP CONSTRAINT IF EXISTS chk_discovered_assets_status;

COMMIT;
