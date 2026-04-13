CREATE INDEX IF NOT EXISTS idx_inventory_items_task_id ON inventory_items(task_id);
CREATE INDEX IF NOT EXISTS idx_inventory_items_status ON inventory_items(status);
CREATE INDEX IF NOT EXISTS idx_inventory_items_asset_id ON inventory_items(asset_id);
