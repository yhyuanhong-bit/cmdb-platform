-- 000078_tenant_settings.down.sql
--
-- Reverse of 000078: drop the tenant_settings table.
-- Code in internal/domain/settings falls back to hardcoded defaults
-- when the row (or table) does not exist, so dropping is safe — no
-- runtime errors, just a return to the pre-W3.2-backend behaviour.

DROP TABLE IF EXISTS tenant_settings;
