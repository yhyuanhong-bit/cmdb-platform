ALTER TABLE assets ADD COLUMN IF NOT EXISTS purchase_date DATE;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS purchase_cost NUMERIC(12,2);
ALTER TABLE assets ADD COLUMN IF NOT EXISTS warranty_start DATE;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS warranty_end DATE;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS warranty_vendor VARCHAR(200);
ALTER TABLE assets ADD COLUMN IF NOT EXISTS warranty_contract VARCHAR(100);
ALTER TABLE assets ADD COLUMN IF NOT EXISTS expected_lifespan_months INT;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS eol_date DATE;

-- Index for warranty expiry queries (dashboard alerts)
CREATE INDEX IF NOT EXISTS idx_assets_warranty_end ON assets(warranty_end) WHERE warranty_end IS NOT NULL AND deleted_at IS NULL;
