-- Phase 4.8: every tenant needs a FK-safe "system" user that workflow code
-- can plug into work_orders.requestor_id, work_order_logs.operator_id,
-- inventory_tasks.assigned_to etc. Password hash is '!' (unbcryptable) so
-- the row cannot log in; Login path also rejects source='system' explicitly.
-- ListUsers filters source!='system' by default so the row stays invisible
-- in UI user pickers.

BEGIN;

INSERT INTO users (tenant_id, username, display_name, email, password_hash, status, source)
SELECT id, 'system', 'System', 'system@' || slug || '.local', '!', 'active', 'system'
FROM tenants
ON CONFLICT (tenant_id, username) DO NOTHING;

CREATE OR REPLACE FUNCTION seed_system_user_for_new_tenant() RETURNS trigger AS $$
BEGIN
    INSERT INTO users (tenant_id, username, display_name, email, password_hash, status, source)
    VALUES (NEW.id, 'system', 'System', 'system@' || NEW.slug || '.local', '!', 'active', 'system')
    ON CONFLICT (tenant_id, username) DO NOTHING;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tenant_seed_system_user
    AFTER INSERT ON tenants
    FOR EACH ROW EXECUTE FUNCTION seed_system_user_for_new_tenant();

COMMIT;
