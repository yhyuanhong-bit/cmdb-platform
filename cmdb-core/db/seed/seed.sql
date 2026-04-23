-- ============================================================================
-- CMDB Production Seed — BOOTSTRAP + REFERENCE DATA ONLY
-- ============================================================================
--
-- This file contains the minimum data needed for a fresh install to function:
--   - 1 tenant (the primary tenant)
--   - 1 admin user (password must be changed on first login)
--   - System roles (super-admin, ops-admin, viewer)
--   - BIA scoring rules (reference data for BIA tier classification)
--   - Field authority definitions (reference data for import reconciliation)
--   - Default alert rules
--
-- Demo / fixture data (locations, racks, sample assets, work orders,
-- inventory tasks, audit events, predictions, etc.) lives in
-- `test-fixture.sql` and is loaded ONLY for development + integration
-- testing, NEVER for production.
--
-- Idempotency: every INSERT uses ON CONFLICT DO NOTHING so re-running
-- seed.sql is safe.

-- ============================================================================
-- 1. Primary tenant
-- ============================================================================

INSERT INTO tenants (id, name, slug) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Primary Tenant', 'tw')
ON CONFLICT (slug) DO NOTHING;

-- ============================================================================
-- 2. Admin user
-- ============================================================================
-- Password: admin123 (bcrypt hash below). MUST change on first login.
-- The startup seed_password.go flow generates a random password in
-- production and writes it to a 0600 file; this INSERT only runs when
-- the user does not yet exist, so that workflow still takes precedence.

INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'admin', 'System Admin', 'admin@example.com',
     '$2b$12$niWDiwVIKZByjN77EhkxpekWRJdznin84cHR7WyyUT/TenYwl78SS',
     'active', 'local')
ON CONFLICT (username) DO NOTHING;

-- ============================================================================
-- 3. System roles
-- ============================================================================

INSERT INTO roles (id, tenant_id, name, description, permissions, is_system) VALUES
    ('c0000000-0000-0000-0000-000000000001', NULL, 'super-admin', 'Full system access', '{"*": ["*"]}', true),
    ('c0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'ops-admin', 'Operations admin', '{"assets":["read","write","delete"],"maintenance":["read","write"],"monitoring":["read","write"],"topology":["read"],"inventory":["read","write"],"audit":["read"],"dashboard":["read"],"prediction":["read"],"system":["read"]}', false),
    ('c0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'viewer', 'Read-only access', '{"assets":["read"],"topology":["read"],"maintenance":["read"],"monitoring":["read"],"inventory":["read"],"audit":["read"],"dashboard":["read"]}', false)
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;

-- ============================================================================
-- 4. BIA scoring rules (reference data)
-- ============================================================================
-- The 4 tiers drive BIA score → tier classification used by alert priority,
-- RTO/RPO defaults, and service-level governance. These values are not
-- tenant-specific in concept, but are tenant-scoped by schema; for fresh
-- tenants created through the API, equivalent rows should be inserted.

INSERT INTO bia_scoring_rules (id, tenant_id, tier_name, tier_level, display_name, min_score, max_score, rto_threshold, rpo_threshold, description, color, icon) VALUES
    ('90000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'critical', 1, 'Tier 1 - CRITICAL', 85, 100, 4, 15,
     'Business-critical systems — downtime causes major financial or safety impact', '#ff6b6b', 'error'),
    ('90000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'important', 2, 'Tier 2 - IMPORTANT', 60, 84, 12, 60,
     'Core operational systems — downtime degrades business efficiency', '#ffa94d', 'warning'),
    ('90000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'normal', 3, 'Tier 3 - NORMAL', 30, 59, 24, 240,
     'Standard business systems — downtime has workaround options', '#9ecaff', 'info'),
    ('90000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'minor', 4, 'Tier 4 - MINOR', 0, 29, 72, null,
     'Test / sandbox systems — downtime has no business impact', '#8e9196', 'expand_circle_down')
ON CONFLICT DO NOTHING;

-- ============================================================================
-- 5. Default alert rules
-- ============================================================================
-- See internal/domain/monitoring/evaluator.go RuleCondition for schema.
-- These 5 rules cover standard infra alerting and are ready to fire
-- against metrics with name cpu_usage / temperature / disk_usage /
-- memory_usage the moment collectors start streaming data.

INSERT INTO alert_rules (id, tenant_id, name, metric_name, condition, severity, enabled) VALUES
    ('40000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'CPU High', 'cpu_usage', '{"operator": ">", "threshold": 85, "window_seconds": 300, "aggregation": "avg", "consecutive_triggers": 2}', 'warning', true),
    ('40000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'CPU Critical', 'cpu_usage', '{"operator": ">", "threshold": 95, "window_seconds": 60, "aggregation": "max", "consecutive_triggers": 1}', 'critical', true),
    ('40000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'Temp High', 'temperature', '{"operator": ">", "threshold": 40, "window_seconds": 300, "aggregation": "avg", "consecutive_triggers": 2}', 'warning', true),
    ('40000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'Disk Full', 'disk_usage', '{"operator": ">", "threshold": 90, "window_seconds": 600, "aggregation": "max", "consecutive_triggers": 1}', 'critical', true),
    ('40000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001', 'Memory High', 'memory_usage', '{"operator": ">", "threshold": 90, "window_seconds": 300, "aggregation": "avg", "consecutive_triggers": 2}', 'warning', true)
ON CONFLICT DO NOTHING;

-- ============================================================================
-- 6. Field Authority definitions (import reconciliation reference data)
-- ============================================================================
-- When an asset attribute differs between discovery sources (IPMI scan vs
-- SNMP vs Excel upload vs manual edit), the field with the highest
-- priority wins. These defaults reflect the standard confidence order:
--     IPMI > SNMP > Excel > Manual
-- for hardware attributes, with `manual` winning only for business
-- attributes like name / status / bia_level that are not automatically
-- discoverable.

INSERT INTO asset_field_authorities (tenant_id, field_name, source_type, priority) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'snmp',   80),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'vendor',        'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'vendor',        'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'vendor',        'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'model',         'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'model',         'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'model',         'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'name',          'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'status',        'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'bia_level',     'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',        'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',        'snmp',   80),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',        'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',        'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_type',      'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_type',      'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_type',      'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_firmware',  'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_firmware',  'manual', 30),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',    'snmp',   100),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',    'mac',    90),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',    'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',    'manual', 70)
ON CONFLICT DO NOTHING;
