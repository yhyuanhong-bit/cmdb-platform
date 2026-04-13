-- 000031_inventory_items_sync.up.sql
ALTER TABLE inventory_items ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_inventory_items_sync_version ON inventory_items(task_id, sync_version);
