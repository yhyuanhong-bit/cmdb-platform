-- Reverses 000076_inventory_tenant_id.up.sql.
DROP INDEX IF EXISTS idx_inventory_notes_tenant_id;
DROP INDEX IF EXISTS idx_inventory_scan_history_tenant_id;

ALTER TABLE inventory_notes
    DROP CONSTRAINT IF EXISTS fk_inventory_notes_tenant,
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE inventory_scan_history
    DROP CONSTRAINT IF EXISTS fk_inventory_scan_history_tenant,
    DROP COLUMN IF EXISTS tenant_id;
