-- Rollback migration 000023

DROP FUNCTION IF EXISTS purge_old_audit_events();
DROP INDEX IF EXISTS idx_audit_events_recent;
DROP INDEX IF EXISTS idx_metrics_labels;
DROP INDEX IF EXISTS idx_webhooks_enabled;
DROP INDEX IF EXISTS idx_assets_not_deleted;
ALTER TABLE assets DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE rca_analyses ALTER COLUMN created_at DROP NOT NULL;
ALTER TABLE prediction_results ALTER COLUMN created_at DROP NOT NULL;
ALTER TABLE prediction_models ALTER COLUMN created_at DROP NOT NULL;
ALTER TABLE prediction_models ALTER COLUMN enabled DROP NOT NULL;

ALTER TABLE locations DROP CONSTRAINT IF EXISTS uq_locations_tenant_slug_level;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS uq_departments_tenant_slug;

DROP INDEX IF EXISTS idx_assets_tag_trgm;
DROP INDEX IF EXISTS idx_assets_name_trgm;

DROP INDEX IF EXISTS idx_inventory_items_task_status;
DROP INDEX IF EXISTS idx_bia_deps_tenant;
DROP INDEX IF EXISTS idx_inventory_items_rack_id;
DROP INDEX IF EXISTS idx_users_dept_id;
DROP INDEX IF EXISTS idx_rca_analyses_tenant_id;
DROP INDEX IF EXISTS idx_rca_analyses_incident_id;
DROP INDEX IF EXISTS idx_quality_scores_tenant_id;
DROP INDEX IF EXISTS idx_webhook_deliveries_sub_delivered;
DROP INDEX IF EXISTS idx_incidents_tenant_status;
DROP INDEX IF EXISTS idx_incidents_tenant_id;
DROP INDEX IF EXISTS idx_alert_events_rule_id;
DROP INDEX IF EXISTS idx_alert_rules_tenant_id;
DROP INDEX IF EXISTS idx_work_orders_assignee_id;
DROP INDEX IF EXISTS idx_work_orders_requestor_id;
DROP INDEX IF EXISTS idx_work_orders_location_id;
DROP INDEX IF EXISTS idx_work_order_logs_operator_id;
DROP INDEX IF EXISTS idx_work_order_logs_order_id;
DROP INDEX IF EXISTS idx_rack_slots_asset_id;
DROP INDEX IF EXISTS idx_roles_tenant_id;
DROP INDEX IF EXISTS idx_user_roles_role_id;
