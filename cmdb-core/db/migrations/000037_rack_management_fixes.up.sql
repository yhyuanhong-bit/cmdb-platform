-- Fix #1: rack_slots.asset_id ON DELETE SET NULL (instead of restrict/cascade)
ALTER TABLE rack_slots DROP CONSTRAINT IF EXISTS rack_slots_asset_id_fkey;
ALTER TABLE rack_slots ADD CONSTRAINT rack_slots_asset_id_fkey
    FOREIGN KEY (asset_id) REFERENCES assets(id) ON DELETE SET NULL;

-- Fix #4: Add updated_at to racks for proper audit trails
ALTER TABLE racks ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();
