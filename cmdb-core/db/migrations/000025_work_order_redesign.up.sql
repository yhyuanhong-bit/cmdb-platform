-- Work Order Redesign Migration
-- Adds SLA tracking columns and notifications table

-- SLA columns on work_orders
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES users(id);
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS sla_deadline TIMESTAMPTZ;
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS sla_warning_sent BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS sla_breached BOOLEAN NOT NULL DEFAULT false;

-- Migrate existing statuses: draft/pending → submitted, closed → verified
UPDATE work_orders SET status = 'submitted' WHERE status IN ('draft', 'pending');
UPDATE work_orders SET status = 'verified' WHERE status = 'closed';

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id),
    user_id     UUID         NOT NULL REFERENCES users(id),
    type        VARCHAR(50)  NOT NULL,
    title       VARCHAR(255) NOT NULL,
    body        TEXT,
    resource_type VARCHAR(50),
    resource_id UUID,
    is_read     BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read) WHERE is_read = false;
CREATE INDEX IF NOT EXISTS idx_notifications_tenant ON notifications(tenant_id);
