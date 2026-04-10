-- Migration 000023: Database Hardening
-- Addresses findings from 7-dimension database audit

-- ============================================================
-- 1. Missing Foreign Key Indexes (16 indexes)
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_roles_tenant_id ON roles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rack_slots_asset_id ON rack_slots(asset_id);
CREATE INDEX IF NOT EXISTS idx_work_order_logs_order_id ON work_order_logs(order_id);
CREATE INDEX IF NOT EXISTS idx_work_order_logs_operator_id ON work_order_logs(operator_id);
CREATE INDEX IF NOT EXISTS idx_work_orders_location_id ON work_orders(location_id);
CREATE INDEX IF NOT EXISTS idx_work_orders_requestor_id ON work_orders(requestor_id);
CREATE INDEX IF NOT EXISTS idx_work_orders_assignee_id ON work_orders(assignee_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_tenant_id ON alert_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_alert_events_rule_id ON alert_events(rule_id);
CREATE INDEX IF NOT EXISTS idx_incidents_tenant_id ON incidents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_incidents_tenant_status ON incidents(tenant_id, status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_sub_delivered ON webhook_deliveries(subscription_id, delivered_at DESC);
CREATE INDEX IF NOT EXISTS idx_quality_scores_tenant_id ON quality_scores(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rca_analyses_incident_id ON rca_analyses(incident_id);
CREATE INDEX IF NOT EXISTS idx_rca_analyses_tenant_id ON rca_analyses(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_dept_id ON users(dept_id);
CREATE INDEX IF NOT EXISTS idx_inventory_items_rack_id ON inventory_items(rack_id);
CREATE INDEX IF NOT EXISTS idx_bia_deps_tenant ON bia_dependencies(tenant_id);

-- Composite index for inventory items (common query pattern)
CREATE INDEX IF NOT EXISTS idx_inventory_items_task_status ON inventory_items(task_id, status);

-- ============================================================
-- 2. Text Search Support (pg_trgm for ILIKE queries)
-- ============================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_assets_name_trgm ON assets USING GIN(name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_assets_tag_trgm ON assets USING GIN(asset_tag gin_trgm_ops);

-- ============================================================
-- 3. UNIQUE Constraints (prevent duplicate entries)
-- ============================================================

-- Prevent duplicate department slugs within a tenant
DO $$ BEGIN
  ALTER TABLE departments ADD CONSTRAINT uq_departments_tenant_slug UNIQUE(tenant_id, slug);
EXCEPTION WHEN duplicate_table THEN NULL;
END $$;

-- Prevent duplicate location slugs at the same level within a tenant
DO $$ BEGIN
  ALTER TABLE locations ADD CONSTRAINT uq_locations_tenant_slug_level UNIQUE(tenant_id, slug, level);
EXCEPTION WHEN duplicate_table THEN NULL;
END $$;

-- ============================================================
-- 4. NOT NULL Fixes (data integrity)
-- ============================================================

ALTER TABLE prediction_models ALTER COLUMN enabled SET NOT NULL;
ALTER TABLE prediction_models ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE prediction_results ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE rca_analyses ALTER COLUMN created_at SET NOT NULL;

-- ============================================================
-- 5. Assets Soft Delete Support
-- ============================================================

ALTER TABLE assets ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_assets_not_deleted ON assets(tenant_id) WHERE deleted_at IS NULL;

-- ============================================================
-- 6. Webhook Lookup Index
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_webhooks_enabled ON webhook_subscriptions(enabled) WHERE enabled = true;

-- ============================================================
-- 7. Metrics Labels GIN Index
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_metrics_labels ON metrics USING GIN(labels);

-- ============================================================
-- 8. Audit Events Retention (partial index for recent data)
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_audit_events_recent
ON audit_events(tenant_id, created_at DESC)
WHERE created_at > now() - interval '90 days';

-- Retention function: purge audit events older than 365 days
CREATE OR REPLACE FUNCTION purge_old_audit_events() RETURNS void
LANGUAGE SQL AS $$
  DELETE FROM audit_events WHERE created_at < now() - interval '365 days';
$$;
