-- ============================================================
-- CMDB Seed Data — Rich dataset for all pages
-- ============================================================

-- Tenant
INSERT INTO tenants (id, name, slug) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Taipei Campus', 'tw')
ON CONFLICT (slug) DO NOTHING;

-- Admin user (password: admin123)
INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'admin', 'System Admin', 'admin@cmdb.local',
     '$2b$12$niWDiwVIKZByjN77EhkxpekWRJdznin84cHR7WyyUT/TenYwl78SS',
     'active', 'local'),
    ('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'sarah.jenkins', 'Sarah Jenkins', 'sarah@cmdb.local',
     '$2b$12$niWDiwVIKZByjN77EhkxpekWRJdznin84cHR7WyyUT/TenYwl78SS',
     'active', 'local'),
    ('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'mike.chen', 'Mike Chen', 'mike@cmdb.local',
     '$2b$12$niWDiwVIKZByjN77EhkxpekWRJdznin84cHR7WyyUT/TenYwl78SS',
     'active', 'local')
ON CONFLICT (username) DO NOTHING;

-- Roles
INSERT INTO roles (id, tenant_id, name, description, permissions, is_system) VALUES
    ('c0000000-0000-0000-0000-000000000001', NULL, 'super-admin', 'Full system access', '{"*": ["*"]}', true),
    ('c0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'ops-admin', 'Operations admin', '{"assets": ["read","write"], "maintenance": ["read","write"], "monitoring": ["read","write"]}', false),
    ('c0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'viewer', 'Read-only access', '{"assets": ["read"], "monitoring": ["read"]}', false)
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001'),
    ('b0000000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000002'),
    ('b0000000-0000-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Location hierarchy: Taiwan > North/South > Cities > Campuses
-- metadata 包含頁面需要的統計信息
-- ============================================================
INSERT INTO locations (id, tenant_id, name, name_en, slug, level, parent_id, path, metadata, sort_order) VALUES
    -- Country
    ('d0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     '台灣', 'Taiwan', 'tw', 'country', NULL, 'tw',
     '{"idc_count": 3, "region_count": 2, "pue": 1.42, "rack_occupancy": 72, "power_trend": [82,79,84,81,78,80,77]}', 1),
    -- Regions
    ('d0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     '北部', 'North', 'north', 'region', 'd0000000-0000-0000-0000-000000000001', 'tw.north',
     '{"idc_count": 2, "pue": 1.38, "racks": 8, "assets": 17, "alerts": 5, "occupancy": 68}', 1),
    ('d0000000-0000-0000-0000-000000000010', 'a0000000-0000-0000-0000-000000000001',
     '南部', 'South', 'south', 'region', 'd0000000-0000-0000-0000-000000000001', 'tw.south',
     '{"idc_count": 1, "pue": 1.51, "racks": 2, "assets": 3, "alerts": 1, "occupancy": 45}', 2),
    -- Cities
    ('d0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     '台北', 'Taipei', 'taipei', 'city', 'd0000000-0000-0000-0000-000000000002', 'tw.north.taipei',
     '{"campuses": 1, "idc_count": 1, "racks": 6, "pue": 1.35, "occupancy": 72, "alerts": 4, "power": 740, "reliability": 99.97}', 1),
    ('d0000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000001',
     '新竹', 'Hsinchu', 'hsinchu', 'city', 'd0000000-0000-0000-0000-000000000002', 'tw.north.hsinchu',
     '{"campuses": 1, "idc_count": 1, "racks": 2, "pue": 1.42, "occupancy": 58, "alerts": 1, "power": 280, "reliability": 99.95}', 2),
    ('d0000000-0000-0000-0000-000000000012', 'a0000000-0000-0000-0000-000000000001',
     '高雄', 'Kaohsiung', 'kaohsiung', 'city', 'd0000000-0000-0000-0000-000000000010', 'tw.south.kaohsiung',
     '{"campuses": 1, "idc_count": 1, "racks": 2, "pue": 1.51, "occupancy": 45, "alerts": 1, "power": 160, "reliability": 99.92}', 1),
    -- Campuses / IDCs
    ('d0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     '内湖園區', 'Neihu Campus', 'neihu', 'campus', 'd0000000-0000-0000-0000-000000000003', 'tw.north.taipei.neihu',
     '{"pue": 1.35, "modules": 4, "racks": 6, "assets": 13, "alerts": 4, "occupancy": 72, "power": 740}', 1),
    ('d0000000-0000-0000-0000-000000000013', 'a0000000-0000-0000-0000-000000000001',
     '竹科園區', 'HSIP Campus', 'hsip', 'campus', 'd0000000-0000-0000-0000-000000000011', 'tw.north.hsinchu.hsip',
     '{"pue": 1.42, "modules": 2, "racks": 2, "assets": 4, "alerts": 1, "occupancy": 58, "power": 280}', 1),
    ('d0000000-0000-0000-0000-000000000014', 'a0000000-0000-0000-0000-000000000001',
     '前鎮園區', 'Qianzhen Campus', 'qianzhen', 'campus', 'd0000000-0000-0000-0000-000000000012', 'tw.south.kaohsiung.qianzhen',
     '{"pue": 1.51, "modules": 1, "racks": 2, "assets": 3, "alerts": 1, "occupancy": 45, "power": 160}', 1)
ON CONFLICT DO NOTHING;

-- ============================================================
-- Racks (10 racks across 3 campuses)
-- ============================================================
INSERT INTO racks (id, tenant_id, location_id, name, row_label, total_u, power_capacity_kw, status) VALUES
    -- Neihu (6 racks)
    ('e0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-A01', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-A02', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-B01', 'B', 42, 12.0, 'active'),
    ('e0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-B02', 'B', 42, 12.0, 'active'),
    ('e0000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-C01', 'C', 42, 15.0, 'active'),
    ('e0000000-0000-0000-0000-000000000006', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-C02', 'C', 42, 15.0, 'maintenance'),
    -- HSIP (2 racks)
    ('e0000000-0000-0000-0000-000000000007', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000013', 'RACK-H01', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000008', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000013', 'RACK-H02', 'A', 42, 10.0, 'active'),
    -- Qianzhen (2 racks)
    ('e0000000-0000-0000-0000-000000000009', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000014', 'RACK-Q01', 'A', 42, 8.0, 'active'),
    ('e0000000-0000-0000-0000-000000000010', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000014', 'RACK-Q02', 'A', 42, 8.0, 'active')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Assets (20 assets across all campuses)
-- ============================================================
INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, bia_level, location_id, rack_id, vendor, model, serial_number) VALUES
    -- Neihu servers (6)
    ('f0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'SRV-PROD-001', 'Production Server 01', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Dell', 'PowerEdge R750', 'SN-DELL-001'),
    ('f0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'SRV-PROD-002', 'Production Server 02', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Dell', 'PowerEdge R750', 'SN-DELL-002'),
    ('f0000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001', 'SRV-DB-001', 'Database Server 01', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000002', 'HP', 'ProLiant DL380 Gen11', 'SN-HP-001'),
    ('f0000000-0000-0000-0000-000000000006', 'a0000000-0000-0000-0000-000000000001', 'SRV-APP-001', 'App Server 01', 'server', 'rack_server', 'operational', 'important', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000002', 'Dell', 'PowerEdge R650', 'SN-DELL-003'),
    ('f0000000-0000-0000-0000-000000000007', 'a0000000-0000-0000-0000-000000000001', 'SRV-DEV-001', 'Dev Server 01', 'server', 'rack_server', 'operational', 'normal', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000003', 'Supermicro', 'SYS-620U', 'SN-SM-001'),
    ('f0000000-0000-0000-0000-000000000008', 'a0000000-0000-0000-0000-000000000001', 'SRV-BACKUP-001', 'Backup Server 01', 'server', 'rack_server', 'maintenance', 'normal', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000003', 'Dell', 'PowerEdge T640', 'SN-DELL-004'),
    -- Neihu network (3)
    ('f0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'NET-SW-A01', 'Core Switch A01', 'network', 'switch', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000004', 'Cisco', 'Nexus 9336C-FX2', 'SN-CISCO-001'),
    ('f0000000-0000-0000-0000-000000000009', 'a0000000-0000-0000-0000-000000000001', 'NET-SW-A02', 'Core Switch A02', 'network', 'switch', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000004', 'Cisco', 'Nexus 9336C-FX2', 'SN-CISCO-002'),
    ('f0000000-0000-0000-0000-000000000010', 'a0000000-0000-0000-0000-000000000001', 'NET-FW-001', 'Firewall 01', 'network', 'firewall', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000005', 'Palo Alto', 'PA-5260', 'SN-PA-001'),
    -- Neihu storage (2)
    ('f0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'STG-NAS-001', 'NAS Storage 01', 'storage', 'nas', 'operational', 'important', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000005', 'Synology', 'SA3600', 'SN-SYN-001'),
    ('f0000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000001', 'STG-SAN-001', 'SAN Storage 01', 'storage', 'san', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000005', 'NetApp', 'AFF A400', 'SN-NA-001'),
    -- Neihu power (2)
    ('f0000000-0000-0000-0000-000000000012', 'a0000000-0000-0000-0000-000000000001', 'PWR-UPS-001', 'UPS Main 01', 'power', 'ups', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', NULL, 'APC', 'Symmetra PX 100kW', 'SN-APC-001'),
    ('f0000000-0000-0000-0000-000000000013', 'a0000000-0000-0000-0000-000000000001', 'PWR-PDU-001', 'PDU Rack-A01', 'power', 'pdu', 'operational', 'important', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Raritan', 'PX3-5902V', 'SN-RAR-001'),
    -- HSIP (4 assets)
    ('f0000000-0000-0000-0000-000000000014', 'a0000000-0000-0000-0000-000000000001', 'SRV-HSIP-001', 'HSIP Server 01', 'server', 'rack_server', 'operational', 'important', 'd0000000-0000-0000-0000-000000000013', 'e0000000-0000-0000-0000-000000000007', 'Dell', 'PowerEdge R750', 'SN-DELL-H01'),
    ('f0000000-0000-0000-0000-000000000015', 'a0000000-0000-0000-0000-000000000001', 'SRV-HSIP-002', 'HSIP Server 02', 'server', 'rack_server', 'operational', 'normal', 'd0000000-0000-0000-0000-000000000013', 'e0000000-0000-0000-0000-000000000007', 'Dell', 'PowerEdge R650', 'SN-DELL-H02'),
    ('f0000000-0000-0000-0000-000000000016', 'a0000000-0000-0000-0000-000000000001', 'NET-HSIP-SW01', 'HSIP Switch 01', 'network', 'switch', 'operational', 'important', 'd0000000-0000-0000-0000-000000000013', 'e0000000-0000-0000-0000-000000000008', 'Arista', 'DCS-7280SR3-48YC8', 'SN-ARI-001'),
    ('f0000000-0000-0000-0000-000000000017', 'a0000000-0000-0000-0000-000000000001', 'STG-HSIP-001', 'HSIP Storage 01', 'storage', 'nas', 'operational', 'normal', 'd0000000-0000-0000-0000-000000000013', 'e0000000-0000-0000-0000-000000000008', 'QNAP', 'TS-h2490FU', 'SN-QN-001'),
    -- Qianzhen (3 assets)
    ('f0000000-0000-0000-0000-000000000018', 'a0000000-0000-0000-0000-000000000001', 'SRV-KH-001', 'Kaohsiung Server 01', 'server', 'rack_server', 'operational', 'normal', 'd0000000-0000-0000-0000-000000000014', 'e0000000-0000-0000-0000-000000000009', 'HP', 'ProLiant DL360 Gen11', 'SN-HP-K01'),
    ('f0000000-0000-0000-0000-000000000019', 'a0000000-0000-0000-0000-000000000001', 'SRV-KH-002', 'Kaohsiung Server 02', 'server', 'rack_server', 'deployed', 'normal', 'd0000000-0000-0000-0000-000000000014', 'e0000000-0000-0000-0000-000000000009', 'HP', 'ProLiant DL360 Gen11', 'SN-HP-K02'),
    ('f0000000-0000-0000-0000-000000000020', 'a0000000-0000-0000-0000-000000000001', 'NET-KH-SW01', 'Kaohsiung Switch 01', 'network', 'switch', 'operational', 'important', 'd0000000-0000-0000-0000-000000000014', 'e0000000-0000-0000-0000-000000000010', 'Cisco', 'Catalyst 9300', 'SN-CISCO-K01')
ON CONFLICT (asset_tag) DO NOTHING;

-- ============================================================
-- Alert Events (8 alerts)
-- ============================================================
INSERT INTO alert_events (id, tenant_id, asset_id, status, severity, message, fired_at) VALUES
    ('10000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'firing', 'warning', 'CPU usage above 85% for 10 minutes', now() - interval '2 hours'),
    ('10000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003', 'firing', 'critical', 'Interface Eth1/1 down', now() - interval '30 minutes'),
    ('10000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000005', 'firing', 'critical', 'Disk usage 95% on /data', now() - interval '15 minutes'),
    ('10000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000012', 'firing', 'warning', 'UPS battery capacity below 80%', now() - interval '4 hours'),
    ('10000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000011', 'acknowledged', 'warning', 'SAN latency spike: 12ms avg', now() - interval '1 hour'),
    ('10000000-0000-0000-0000-000000000006', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000010', 'resolved', 'critical', 'Firewall HA failover triggered', now() - interval '6 hours'),
    ('10000000-0000-0000-0000-000000000007', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000014', 'firing', 'warning', 'Memory usage 92%', now() - interval '45 minutes'),
    ('10000000-0000-0000-0000-000000000008', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000018', 'firing', 'info', 'Scheduled reboot pending', now() - interval '3 hours')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Work Orders (6 orders in various states)
-- ============================================================
INSERT INTO work_orders (id, tenant_id, code, title, type, status, priority, location_id, asset_id, requestor_id, assignee_id, description, reason, scheduled_start, scheduled_end) VALUES
    ('20000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'WO-2026-0001', 'UPS Battery Replacement', 'replacement', 'in_progress', 'critical',
     'd0000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000012',
     'b0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002',
     'Replace UPS battery pack due to degraded capacity', 'Battery capacity dropped below 80%',
     now(), now() + interval '4 hours'),
    ('20000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'WO-2026-0002', 'Core Switch Firmware Update', 'upgrade', 'approved', 'high',
     'd0000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000003',
     'b0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000003',
     'Update Nexus firmware to fix CVE-2026-1234', 'Security vulnerability patch',
     now() + interval '1 day', now() + interval '1 day 2 hours'),
    ('20000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'WO-2026-0003', 'Backup Server Repair', 'repair', 'pending', 'medium',
     'd0000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000008',
     'b0000000-0000-0000-0000-000000000002', NULL,
     'Diagnose and repair failed RAID controller', 'Server in maintenance status',
     now() + interval '2 days', now() + interval '2 days 4 hours'),
    ('20000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'WO-2026-0004', 'Quarterly Server Inspection', 'inspection', 'completed', 'low',
     'd0000000-0000-0000-0000-000000000004', NULL,
     'b0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002',
     'Q1 2026 routine inspection of all production servers', 'Scheduled quarterly maintenance',
     now() - interval '7 days', now() - interval '6 days 20 hours'),
    ('20000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001',
     'WO-2026-0005', 'HSIP Network Cabling', 'inspection', 'draft', 'low',
     'd0000000-0000-0000-0000-000000000013', NULL,
     'b0000000-0000-0000-0000-000000000003', NULL,
     'Inspect and label all network cables in HSIP campus', 'New cable management standard',
     now() + interval '5 days', now() + interval '5 days 8 hours'),
    ('20000000-0000-0000-0000-000000000006', 'a0000000-0000-0000-0000-000000000001',
     'WO-2026-0006', 'Firewall HA Config Review', 'inspection', 'in_progress', 'high',
     'd0000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000010',
     'b0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000003',
     'Review HA configuration after failover incident', 'Post-incident review',
     now() - interval '1 day', now())
ON CONFLICT DO NOTHING;

-- Work Order Logs
INSERT INTO work_order_logs (order_id, action, from_status, to_status, operator_id, comment) VALUES
    ('20000000-0000-0000-0000-000000000001', 'created', NULL, 'draft', 'b0000000-0000-0000-0000-000000000001', 'Created by admin'),
    ('20000000-0000-0000-0000-000000000001', 'transitioned', 'draft', 'pending', 'b0000000-0000-0000-0000-000000000001', NULL),
    ('20000000-0000-0000-0000-000000000001', 'transitioned', 'pending', 'approved', 'b0000000-0000-0000-0000-000000000001', 'Approved - batteries in stock'),
    ('20000000-0000-0000-0000-000000000001', 'transitioned', 'approved', 'in_progress', 'b0000000-0000-0000-0000-000000000002', 'Sarah started work'),
    ('20000000-0000-0000-0000-000000000004', 'created', NULL, 'draft', 'b0000000-0000-0000-0000-000000000001', NULL),
    ('20000000-0000-0000-0000-000000000004', 'transitioned', 'draft', 'completed', 'b0000000-0000-0000-0000-000000000002', 'All servers inspected, no issues found')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Inventory Tasks (3 tasks)
-- ============================================================
INSERT INTO inventory_tasks (id, tenant_id, code, name, scope_location_id, status, method, planned_date, completed_date, assigned_to) VALUES
    ('30000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'INV-2026-Q1-001', 'Q1 Neihu Full Inventory', 'd0000000-0000-0000-0000-000000000004',
     'completed', 'barcode', '2026-01-15', '2026-01-16', 'b0000000-0000-0000-0000-000000000002'),
    ('30000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'INV-2026-Q1-002', 'Q1 HSIP Spot Check', 'd0000000-0000-0000-0000-000000000013',
     'in_progress', 'rfid', '2026-03-28', NULL, 'b0000000-0000-0000-0000-000000000003'),
    ('30000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'INV-2026-Q2-001', 'Q2 Full Platform Inventory', 'd0000000-0000-0000-0000-000000000001',
     'planned', 'barcode', '2026-04-15', NULL, 'b0000000-0000-0000-0000-000000000002')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Audit Events (10 events)
-- ============================================================
INSERT INTO audit_events (tenant_id, action, module, target_type, target_id, operator_id, diff, source) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'asset.created', 'asset', 'asset', 'f0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', '{"name": {"old": null, "new": "Production Server 01"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'asset.created', 'asset', 'asset', 'f0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000001', '{"name": {"old": null, "new": "Core Switch A01"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'asset.status_changed', 'asset', 'asset', 'f0000000-0000-0000-0000-000000000008', 'b0000000-0000-0000-0000-000000000002', '{"status": {"old": "operational", "new": "maintenance"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'order.created', 'maintenance', 'work_order', '20000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', '{"title": {"old": null, "new": "UPS Battery Replacement"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'order.transitioned', 'maintenance', 'work_order', '20000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', '{"status": {"old": "draft", "new": "in_progress"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'rack.created', 'topology', 'rack', 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', '{"name": {"old": null, "new": "RACK-A01"}}', 'import'),
    ('a0000000-0000-0000-0000-000000000001', 'user.login', 'identity', 'user', 'b0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', '{}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'alert.acknowledged', 'monitoring', 'alert', '10000000-0000-0000-0000-000000000005', 'b0000000-0000-0000-0000-000000000002', '{"status": {"old": "firing", "new": "acknowledged"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'alert.resolved', 'monitoring', 'alert', '10000000-0000-0000-0000-000000000006', 'b0000000-0000-0000-0000-000000000003', '{"status": {"old": "firing", "new": "resolved"}}', 'web'),
    ('a0000000-0000-0000-0000-000000000001', 'inventory.completed', 'inventory', 'inventory_task', '30000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', '{"status": {"old": "in_progress", "new": "completed"}}', 'web')
ON CONFLICT DO NOTHING;
