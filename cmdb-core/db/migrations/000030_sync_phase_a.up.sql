-- 000030_sync_phase_a.up.sql

-- 1. alert_rules sync support
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_alert_rules_sync_version ON alert_rules(tenant_id, sync_version);

-- 2. RBAC: add sync permissions to ops-admin (super-admin already has "*":"*")
UPDATE roles SET permissions = permissions || '{"sync":["read","write"]}'::jsonb
WHERE name = 'ops-admin';

UPDATE roles SET permissions = permissions || '{"sync":["read"]}'::jsonb
WHERE name = 'viewer';
