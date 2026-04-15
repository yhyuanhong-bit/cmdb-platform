-- Revert Fix #1: restore original FK (no ON DELETE clause = RESTRICT by default)
ALTER TABLE rack_slots DROP CONSTRAINT IF EXISTS rack_slots_asset_id_fkey;
ALTER TABLE rack_slots ADD CONSTRAINT rack_slots_asset_id_fkey
    FOREIGN KEY (asset_id) REFERENCES assets(id);

-- Revert Fix #4: remove updated_at
ALTER TABLE racks DROP COLUMN IF EXISTS updated_at;
