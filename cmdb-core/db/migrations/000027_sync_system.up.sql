-- 000027_sync_system.up.sql

-- 1. Add sync_version to syncable tables
ALTER TABLE assets ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE locations ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE racks ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE inventory_tasks ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;

-- Indexes for incremental sync queries
CREATE INDEX IF NOT EXISTS idx_assets_sync_version ON assets(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_locations_sync_version ON locations(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_racks_sync_version ON racks(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_work_orders_sync_version ON work_orders(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_alert_events_sync_version ON alert_events(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_inventory_tasks_sync_version ON inventory_tasks(tenant_id, sync_version);

-- 2. Work order dual-dimension status (keep original status column)
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS execution_status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS governance_status VARCHAR(20) NOT NULL DEFAULT 'submitted';

-- Backfill governance_status from existing status
UPDATE work_orders SET governance_status = status WHERE governance_status = 'submitted' AND status != 'submitted';
-- Backfill execution_status from existing status
UPDATE work_orders SET execution_status = CASE
    WHEN status IN ('in_progress') THEN 'working'
    WHEN status IN ('completed', 'verified') THEN 'done'
    ELSE 'pending'
END;

-- 3. Sync state tracking
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

-- 4. Sync conflicts
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
