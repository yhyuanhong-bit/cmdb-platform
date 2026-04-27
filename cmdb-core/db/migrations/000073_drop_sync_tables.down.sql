-- 000073_drop_sync_tables.down.sql
--
-- Restore the replication state tables and RBAC grants that 000073 dropped.
-- Mirrors the original definitions from 000027 / 000030.

CREATE TABLE IF NOT EXISTS sync_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id VARCHAR(100) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    entity_type VARCHAR(50) NOT NULL,
    last_sync_version BIGINT DEFAULT 0,
    last_sync_at TIMESTAMPTZ,
    status VARCHAR(20) DEFAULT 'active',
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(node_id, entity_type)
);

CREATE TABLE IF NOT EXISTS sync_conflicts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    local_version BIGINT NOT NULL,
    remote_version BIGINT NOT NULL,
    local_diff JSONB NOT NULL,
    remote_diff JSONB NOT NULL,
    resolution VARCHAR(20) DEFAULT 'pending',
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_pending ON sync_conflicts(tenant_id, resolution) WHERE resolution = 'pending';

UPDATE roles SET permissions = permissions || '{"sync":["read","write"]}'::jsonb
WHERE name = 'ops-admin';

UPDATE roles SET permissions = permissions || '{"sync":["read"]}'::jsonb
WHERE name = 'viewer';
