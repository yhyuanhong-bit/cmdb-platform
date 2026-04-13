-- 000030_sync_phase_a.down.sql

ALTER TABLE alert_rules DROP COLUMN IF EXISTS sync_version;

UPDATE roles SET permissions = permissions - 'sync'
WHERE name IN ('ops-admin', 'viewer');
