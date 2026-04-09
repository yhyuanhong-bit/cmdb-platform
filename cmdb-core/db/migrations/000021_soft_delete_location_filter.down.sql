DROP INDEX IF EXISTS idx_inventory_tasks_deleted_at;
DROP INDEX IF EXISTS idx_work_orders_deleted_at;
ALTER TABLE inventory_tasks DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE work_orders DROP COLUMN IF EXISTS deleted_at;
