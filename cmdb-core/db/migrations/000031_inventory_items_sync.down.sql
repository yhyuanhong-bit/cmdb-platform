-- 000031_inventory_items_sync.down.sql
DROP INDEX IF EXISTS idx_inventory_items_sync_version;
ALTER TABLE inventory_items DROP COLUMN IF EXISTS sync_version;
