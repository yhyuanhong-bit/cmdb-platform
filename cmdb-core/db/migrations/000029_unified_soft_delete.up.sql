-- Add deleted_at to tables that lack it
ALTER TABLE locations ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE racks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Partial indexes for efficient queries excluding soft-deleted rows
CREATE INDEX IF NOT EXISTS idx_locations_not_deleted ON locations(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_racks_not_deleted ON racks(tenant_id, location_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_not_deleted ON users(tenant_id) WHERE deleted_at IS NULL;
