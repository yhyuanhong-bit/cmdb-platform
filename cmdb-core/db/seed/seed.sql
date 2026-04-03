-- Tenant
INSERT INTO tenants (id, name, slug) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Taipei Campus', 'tw')
ON CONFLICT (slug) DO NOTHING;

-- Admin user (password: admin123, bcrypt hash)
INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source) VALUES
    ('b0000000-0000-0000-0000-000000000001',
     'a0000000-0000-0000-0000-000000000001',
     'admin', 'System Admin', 'admin@cmdb.local',
     '$2a$12$LJ3m4ys/ApX6iBHaRfMwWeSS.lCGHXBxWKCy/OPPIa4IhzRo.mJHq',
     'active', 'local')
ON CONFLICT (username) DO NOTHING;

-- Super admin role (global)
INSERT INTO roles (id, tenant_id, name, description, permissions, is_system) VALUES
    ('c0000000-0000-0000-0000-000000000001', NULL, 'super-admin', 'Full system access', '{"*": ["*"]}', true)
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;

-- Location hierarchy: Taiwan > North > Taipei > Neihu Campus
INSERT INTO locations (id, tenant_id, name, name_en, slug, level, parent_id, path, metadata, sort_order) VALUES
    ('d0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', '台灣', 'Taiwan', 'tw', 'country', NULL, 'tw', '{}', 1),
    ('d0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', '北部', 'North', 'north', 'region', 'd0000000-0000-0000-0000-000000000001', 'tw.north', '{}', 1),
    ('d0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', '台北', 'Taipei', 'taipei', 'city', 'd0000000-0000-0000-0000-000000000002', 'tw.north.taipei', '{}', 1),
    ('d0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', '内湖園區', 'Neihu Campus', 'neihu', 'campus', 'd0000000-0000-0000-0000-000000000003', 'tw.north.taipei.neihu', '{"pue": 1.45}', 1)
ON CONFLICT DO NOTHING;

-- 3 Racks
INSERT INTO racks (id, tenant_id, location_id, name, row_label, total_u, power_capacity_kw, status) VALUES
    ('e0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-A01', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-A02', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-B01', 'B', 42, 12.0, 'active')
ON CONFLICT DO NOTHING;

-- 4 Assets
INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, bia_level, location_id, rack_id, vendor, model, serial_number) VALUES
    ('f0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'SRV-PROD-001', 'Production Server 01', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Dell', 'PowerEdge R750', 'SN-DELL-001'),
    ('f0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'SRV-PROD-002', 'Production Server 02', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Dell', 'PowerEdge R750', 'SN-DELL-002'),
    ('f0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'NET-SW-A01', 'Core Switch A01', 'network', 'switch', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000002', 'Cisco', 'Nexus 9336C-FX2', 'SN-CISCO-001'),
    ('f0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'STG-NAS-001', 'NAS Storage 01', 'storage', 'nas', 'operational', 'important', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000003', 'Synology', 'SA3600', 'SN-SYN-001')
ON CONFLICT (asset_tag) DO NOTHING;

-- 2 Alert events
INSERT INTO alert_events (id, tenant_id, asset_id, status, severity, message, fired_at) VALUES
    ('10000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'firing', 'warning', 'CPU usage above 85% for 10 minutes', now() - interval '2 hours'),
    ('10000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003', 'firing', 'critical', 'Interface Eth1/1 down', now() - interval '30 minutes')
ON CONFLICT DO NOTHING;
