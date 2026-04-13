-- 1. Audit log tamper protection: prevent DELETE and UPDATE
CREATE OR REPLACE FUNCTION prevent_audit_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events table is append-only: % operations are not allowed', TG_OP;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

-- 2. Asset status CHECK constraint
DO $$ BEGIN
    ALTER TABLE assets ADD CONSTRAINT chk_assets_status
        CHECK (status IN ('procurement', 'inventoried', 'deploying', 'deployed', 'operational', 'active', 'maintenance', 'decommission', 'retired', 'disposed', 'offline'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 3. Work order status CHECK constraint
DO $$ BEGIN
    ALTER TABLE work_orders ADD CONSTRAINT chk_work_orders_status
        CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'in_progress', 'completed', 'verified'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 4. Alert event status CHECK constraint
DO $$ BEGIN
    ALTER TABLE alert_events ADD CONSTRAINT chk_alert_events_status
        CHECK (status IN ('firing', 'acknowledged', 'resolved'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 5. Inventory task status CHECK constraint
DO $$ BEGIN
    ALTER TABLE inventory_tasks ADD CONSTRAINT chk_inventory_tasks_status
        CHECK (status IN ('planned', 'in_progress', 'completed'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
