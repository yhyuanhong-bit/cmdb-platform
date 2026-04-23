-- Roll back 000063. bia_assessments rows survive; only the service_id FK
-- column is dropped, along with the new services tables.
BEGIN;

-- Drop the reverse FK column on bia_assessments first; dropping services
-- while bia_assessments still reference it would fail on the FK guard.
ALTER TABLE bia_assessments DROP COLUMN IF EXISTS service_id;

-- Cascade removes service_assets entries automatically.
DROP TABLE IF EXISTS service_assets;
DROP TABLE IF EXISTS services;

COMMIT;
