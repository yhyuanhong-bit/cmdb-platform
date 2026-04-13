-- Add soft-delete support for work_orders and inventory_tasks
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE inventory_tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Partial indexes for efficient querying of non-deleted records
CREATE INDEX IF NOT EXISTS idx_work_orders_deleted_at
  ON work_orders (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_tasks_deleted_at
  ON inventory_tasks (deleted_at) WHERE deleted_at IS NULL;
