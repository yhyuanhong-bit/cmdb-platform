DROP INDEX IF EXISTS idx_assets_warranty_end;
ALTER TABLE assets DROP COLUMN IF EXISTS eol_date;
ALTER TABLE assets DROP COLUMN IF EXISTS expected_lifespan_months;
ALTER TABLE assets DROP COLUMN IF EXISTS warranty_contract;
ALTER TABLE assets DROP COLUMN IF EXISTS warranty_vendor;
ALTER TABLE assets DROP COLUMN IF EXISTS warranty_end;
ALTER TABLE assets DROP COLUMN IF EXISTS warranty_start;
ALTER TABLE assets DROP COLUMN IF EXISTS purchase_cost;
ALTER TABLE assets DROP COLUMN IF EXISTS purchase_date;
