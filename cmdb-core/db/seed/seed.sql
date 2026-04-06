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
    ('c0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'ops-admin', 'Operations admin', '{"assets":["read","write","delete"],"maintenance":["read","write"],"monitoring":["read","write"],"topology":["read"],"inventory":["read","write"],"audit":["read"],"dashboard":["read"],"prediction":["read"],"system":["read"]}', false),
    ('c0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'viewer', 'Read-only access', '{"assets":["read"],"topology":["read"],"maintenance":["read"],"monitoring":["read"],"inventory":["read"],"audit":["read"],"dashboard":["read"]}', false)
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

-- ============================================================
-- Departments (4 departments)
-- ============================================================
INSERT INTO departments (id, tenant_id, name, slug, permissions) VALUES
    ('c0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'Infrastructure Operations', 'infra-ops', '{"modules": ["asset", "topology", "monitoring", "maintenance"]}'),
    ('c0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'Network Engineering', 'net-eng', '{"modules": ["asset", "topology", "monitoring"]}'),
    ('c0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'Security & Compliance', 'security', '{"modules": ["monitoring", "audit", "inventory"]}'),
    ('c0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'Platform Engineering', 'platform-eng', '{"modules": ["asset", "integration", "prediction"]}')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Rack Slots (20 slots mapping assets to rack U positions)
-- ============================================================
INSERT INTO rack_slots (rack_id, asset_id, start_u, end_u, side) VALUES
    -- RACK-A01: SRV-PROD-001 (2U), SRV-PROD-002 (2U), PDU (3U back)
    ('e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 1, 2, 'front'),
    ('e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002', 3, 4, 'front'),
    ('e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000013', 40, 42, 'back'),
    -- RACK-A02: SRV-DB-001 (2U), SRV-APP-001 (2U)
    ('e0000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000005', 1, 2, 'front'),
    ('e0000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000006', 3, 4, 'front'),
    -- RACK-B01: SRV-DEV-001 (2U), SRV-BACKUP-001 (4U)
    ('e0000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000007', 1, 2, 'front'),
    ('e0000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000008', 3, 6, 'front'),
    -- RACK-B02: Switches (1U each)
    ('e0000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000003', 1, 1, 'front'),
    ('e0000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000009', 2, 2, 'front'),
    -- RACK-C01: Firewall (2U), NAS (4U), SAN (4U)
    ('e0000000-0000-0000-0000-000000000005', 'f0000000-0000-0000-0000-000000000010', 1, 2, 'front'),
    ('e0000000-0000-0000-0000-000000000005', 'f0000000-0000-0000-0000-000000000004', 3, 6, 'front'),
    ('e0000000-0000-0000-0000-000000000005', 'f0000000-0000-0000-0000-000000000011', 7, 10, 'front'),
    -- RACK-H01: HSIP servers
    ('e0000000-0000-0000-0000-000000000007', 'f0000000-0000-0000-0000-000000000014', 1, 2, 'front'),
    ('e0000000-0000-0000-0000-000000000007', 'f0000000-0000-0000-0000-000000000015', 3, 4, 'front'),
    -- RACK-H02: HSIP switch + storage
    ('e0000000-0000-0000-0000-000000000008', 'f0000000-0000-0000-0000-000000000016', 1, 1, 'front'),
    ('e0000000-0000-0000-0000-000000000008', 'f0000000-0000-0000-0000-000000000017', 2, 5, 'front'),
    -- RACK-Q01: Kaohsiung servers
    ('e0000000-0000-0000-0000-000000000009', 'f0000000-0000-0000-0000-000000000018', 1, 2, 'front'),
    ('e0000000-0000-0000-0000-000000000009', 'f0000000-0000-0000-0000-000000000019', 3, 4, 'front'),
    -- RACK-Q02: Kaohsiung switch
    ('e0000000-0000-0000-0000-000000000010', 'f0000000-0000-0000-0000-000000000020', 1, 1, 'front')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Alert Rules (5 monitoring thresholds)
-- ============================================================
INSERT INTO alert_rules (id, tenant_id, name, metric_name, condition, severity, enabled) VALUES
    ('40000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'CPU High', 'cpu_usage', '{"op": ">", "threshold": 85}', 'warning', true),
    ('40000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'CPU Critical', 'cpu_usage', '{"op": ">", "threshold": 95}', 'critical', true),
    ('40000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'Temp High', 'temperature', '{"op": ">", "threshold": 40}', 'warning', true),
    ('40000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'Disk Full', 'disk_usage', '{"op": ">", "threshold": 90}', 'critical', true),
    ('40000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001', 'Memory High', 'memory_usage', '{"op": ">", "threshold": 90}', 'warning', true)
ON CONFLICT DO NOTHING;

-- ============================================================
-- Incidents (3 incidents in various states)
-- ============================================================
INSERT INTO incidents (id, tenant_id, title, status, severity, started_at, resolved_at) VALUES
    ('50000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'Network Core Switch Failure', 'open', 'critical', now() - interval '2 hours', NULL),
    ('50000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'Storage Latency Degradation', 'investigating', 'warning', now() - interval '4 hours', NULL),
    ('50000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'UPS Battery Alert', 'resolved', 'warning', now() - interval '1 day', now() - interval '20 hours')
ON CONFLICT DO NOTHING;

-- ============================================================
-- Prediction Models (2 models - note: migration already seeds one with id 20000000-...-001)
-- ============================================================
INSERT INTO prediction_models (id, name, type, provider, config, enabled) VALUES
    ('60000000-0000-0000-0000-000000000001', 'Dify RCA Analyzer', 'rca', 'dify', '{"workflow_id": "rca-v1", "endpoint": "https://dify.example.com/api"}', true),
    ('60000000-0000-0000-0000-000000000002', 'Local Failure Predictor', 'failure_prediction', 'local', '{"model_path": "/models/failure-pred-v2.onnx", "threshold": 0.7}', true)
ON CONFLICT DO NOTHING;

-- ============================================================
-- Prediction Results (5 failure predictions)
-- ============================================================
INSERT INTO prediction_results (id, tenant_id, model_id, asset_id, prediction_type, result, severity, recommended_action, expires_at) VALUES
    ('70000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000001', 'failure_prediction', '{"probability": 0.82, "component": "disk", "mtbf_hours": 720}', 'warning', 'Schedule preventive disk replacement within 30 days', now() + interval '30 days'),
    ('70000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000005', 'failure_prediction', '{"probability": 0.91, "component": "memory", "mtbf_hours": 360}', 'critical', 'Replace memory module immediately - high failure risk', now() + interval '15 days'),
    ('70000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000012', 'failure_prediction', '{"probability": 0.65, "component": "battery", "mtbf_hours": 1440}', 'info', 'Monitor UPS battery health, schedule replacement in Q3', now() + interval '60 days'),
    ('70000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000003', 'failure_prediction', '{"probability": 0.45, "component": "fan", "mtbf_hours": 2880}', 'info', 'No immediate action required - standard wear', now() + interval '90 days'),
    ('70000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000014', 'failure_prediction', '{"probability": 0.73, "component": "power_supply", "mtbf_hours": 960}', 'warning', 'Order replacement PSU, schedule swap during next maintenance window', now() + interval '40 days')
ON CONFLICT DO NOTHING;

-- ============================================================
-- RCA Analyses (2 root cause analyses)
-- ============================================================
INSERT INTO rca_analyses (id, tenant_id, incident_id, model_id, reasoning, conclusion_asset_id, confidence, human_verified, verified_by) VALUES
    ('80000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000001',
     '{"steps": ["Analyzed network topology", "Identified switch port errors", "Correlated with recent firmware update"], "root_cause": "Firmware bug causing intermittent port flapping", "evidence": ["Error logs show CRC errors on Eth1/1", "Issue started after firmware v3.2.1 update"]}',
     'f0000000-0000-0000-0000-000000000003', 0.87, true, 'b0000000-0000-0000-0000-000000000002'),
    ('80000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000002', '60000000-0000-0000-0000-000000000001',
     '{"steps": ["Checked storage IOPS metrics", "Analyzed disk queue depth", "Reviewed recent workload changes"], "root_cause": "Database backup job causing I/O contention", "evidence": ["IOPS spike at 03:00 correlates with backup schedule", "Queue depth exceeds 64 during backup window"]}',
     'f0000000-0000-0000-0000-000000000011', 0.72, false, NULL)
ON CONFLICT DO NOTHING;

-- ============================================================
-- Inventory Items (10 items with scanned/pending/discrepancy)
-- ============================================================
INSERT INTO inventory_items (task_id, asset_id, rack_id, expected, actual, status, scanned_at, scanned_by) VALUES
    -- Task 1 (Neihu full - completed): all scanned
    ('30000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001',
     '{"location": "RACK-A01 U1-2", "serial": "SN-DELL-001"}',
     '{"location": "RACK-A01 U1-2", "serial": "SN-DELL-001"}',
     'scanned', now() - interval '78 days', 'b0000000-0000-0000-0000-000000000002'),
    ('30000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000001',
     '{"location": "RACK-A01 U3-4", "serial": "SN-DELL-002"}',
     '{"location": "RACK-A01 U3-4", "serial": "SN-DELL-002"}',
     'scanned', now() - interval '78 days', 'b0000000-0000-0000-0000-000000000002'),
    ('30000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000004',
     '{"location": "RACK-B02 U1", "serial": "SN-CISCO-001"}',
     '{"location": "RACK-B01 U5", "serial": "SN-CISCO-001"}',
     'discrepancy', now() - interval '78 days', 'b0000000-0000-0000-0000-000000000002'),
    ('30000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000010', 'e0000000-0000-0000-0000-000000000005',
     '{"location": "RACK-C01 U1-2", "serial": "SN-PA-001"}',
     '{"location": "RACK-C01 U1-2", "serial": "SN-PA-001"}',
     'scanned', now() - interval '78 days', 'b0000000-0000-0000-0000-000000000002'),
    ('30000000-0000-0000-0000-000000000001', NULL, 'e0000000-0000-0000-0000-000000000006',
     '{"location": "RACK-C02 U10", "serial": "UNKNOWN"}',
     '{"location": "RACK-C02 U10", "serial": "SN-ROGUE-001"}',
     'discrepancy', now() - interval '78 days', 'b0000000-0000-0000-0000-000000000002'),
    -- Task 2 (HSIP spot check - in progress): some scanned, some pending
    ('30000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000014', 'e0000000-0000-0000-0000-000000000007',
     '{"location": "RACK-H01 U1-2", "serial": "SN-DELL-H01"}',
     '{"location": "RACK-H01 U1-2", "serial": "SN-DELL-H01"}',
     'scanned', now() - interval '7 days', 'b0000000-0000-0000-0000-000000000003'),
    ('30000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000015', 'e0000000-0000-0000-0000-000000000007',
     '{"location": "RACK-H01 U3-4", "serial": "SN-DELL-H02"}',
     NULL,
     'pending', NULL, NULL),
    ('30000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000016', 'e0000000-0000-0000-0000-000000000008',
     '{"location": "RACK-H02 U1", "serial": "SN-ARI-001"}',
     NULL,
     'pending', NULL, NULL),
    -- Task 3 (Q2 full - planned): all pending
    ('30000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000018', 'e0000000-0000-0000-0000-000000000009',
     '{"location": "RACK-Q01 U1-2", "serial": "SN-HP-K01"}',
     NULL,
     'pending', NULL, NULL),
    ('30000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000019', 'e0000000-0000-0000-0000-000000000009',
     '{"location": "RACK-Q01 U3-4", "serial": "SN-HP-K02"}',
     NULL,
     'pending', NULL, NULL)
ON CONFLICT DO NOTHING;

-- ============================================================
-- Webhook Deliveries (5 delivery records using subquery for subscription IDs)
-- ============================================================
INSERT INTO webhook_deliveries (subscription_id, event_type, payload, status_code, response_body)
SELECT ws.id, v.event_type, v.payload::jsonb, v.status_code, v.response_body
FROM webhook_subscriptions ws
CROSS JOIN (VALUES
    ('Slack Alerts', 'alert.fired', '{"alert_id": "10000000-0000-0000-0000-000000000001", "severity": "critical", "message": "CPU Critical on SRV-PROD-001"}', 200, '{"ok": true}'),
    ('Slack Alerts', 'alert.fired', '{"alert_id": "10000000-0000-0000-0000-000000000003", "severity": "warning", "message": "Temperature High in RACK-A01"}', 200, '{"ok": true}'),
    ('Slack Alerts', 'alert.resolved', '{"alert_id": "10000000-0000-0000-0000-000000000006", "severity": "warning", "message": "Memory alert resolved on SRV-DEV-001"}', 200, '{"ok": true}'),
    ('Slack Alerts', 'alert.fired', '{"alert_id": "10000000-0000-0000-0000-000000000007", "severity": "critical", "message": "Disk Full on SRV-DB-001"}', 500, '{"error": "channel_not_found"}'),
    ('Teams Notifications', 'maintenance.order_created', '{"work_order_id": "20000000-0000-0000-0000-000000000001", "title": "UPS Battery Replacement"}', 200, '{"id": "msg-001"}')
) AS v(sub_name, event_type, payload, status_code, response_body)
WHERE ws.name = v.sub_name;

-- ============================================================
-- Asset Attributes Enrichment (W4)
-- AssetDetail pages read from attributes JSONB for cpu, memory, etc.
-- ============================================================

UPDATE assets SET attributes = '{
  "cpu": "2x Intel Xeon Gold 6348 (56C/112T)",
  "memory": "512GB DDR4-3200 ECC",
  "storage": "8x 3.84TB NVMe SSD (RAID-10)",
  "network": "4x 25GbE + 2x 100GbE",
  "os": "Rocky Linux 9.3",
  "primary_ip": "10.134.143.101",
  "mgmt_ip": "10.134.144.101",
  "form_factor": "2U Rackmount",
  "warranty_expiry": "2028-03-15",
  "purchase_date": "2025-03-15",
  "uptime_days": 342,
  "mtbf_hours": 45000
}' WHERE asset_tag = 'SRV-PROD-001';

UPDATE assets SET attributes = '{
  "cpu": "2x Intel Xeon Gold 6348 (56C/112T)",
  "memory": "512GB DDR4-3200 ECC",
  "storage": "8x 3.84TB NVMe SSD (RAID-10)",
  "network": "4x 25GbE + 2x 100GbE",
  "os": "Rocky Linux 9.3",
  "primary_ip": "10.134.143.102",
  "mgmt_ip": "10.134.144.102",
  "form_factor": "2U Rackmount",
  "warranty_expiry": "2028-03-15",
  "purchase_date": "2025-03-15",
  "uptime_days": 342,
  "mtbf_hours": 48000
}' WHERE asset_tag = 'SRV-PROD-002';

UPDATE assets SET attributes = '{
  "cpu": "2x Intel Xeon Gold 6330 (56C/112T)",
  "memory": "1TB DDR4-3200 ECC",
  "storage": "12x 7.68TB NVMe SSD (RAID-60)",
  "network": "2x 25GbE + 2x 100GbE",
  "os": "Rocky Linux 9.3 + PostgreSQL 16",
  "primary_ip": "10.134.143.105",
  "mgmt_ip": "10.134.144.105",
  "form_factor": "2U Rackmount",
  "warranty_expiry": "2028-06-01",
  "purchase_date": "2025-06-01",
  "uptime_days": 256,
  "mtbf_hours": 52000
}' WHERE asset_tag = 'SRV-DB-001';

UPDATE assets SET attributes = '{
  "cpu": "2x Intel Xeon Silver 4314 (32C/64T)",
  "memory": "256GB DDR4-3200 ECC",
  "storage": "4x 1.92TB NVMe SSD (RAID-10)",
  "network": "4x 25GbE",
  "os": "Rocky Linux 9.3 + Kubernetes 1.29",
  "primary_ip": "10.134.143.106",
  "mgmt_ip": "10.134.144.106",
  "form_factor": "2U Rackmount",
  "warranty_expiry": "2028-06-01",
  "purchase_date": "2025-06-01",
  "uptime_days": 198,
  "mtbf_hours": 40000
}' WHERE asset_tag = 'SRV-APP-001';

UPDATE assets SET attributes = '{
  "ports": "36x 400GbE QSFP-DD",
  "firmware": "NX-OS 10.3(4a)",
  "primary_ip": "10.134.143.1",
  "mgmt_ip": "10.134.144.1",
  "form_factor": "1U Rackmount",
  "warranty_expiry": "2027-09-01",
  "purchase_date": "2024-09-01",
  "uptime_days": 520,
  "mtbf_hours": 87000,
  "ha_status": "active"
}' WHERE asset_tag = 'NET-SW-A01';

UPDATE assets SET attributes = '{
  "ports": "36x 400GbE QSFP-DD",
  "firmware": "NX-OS 10.3(4a)",
  "primary_ip": "10.134.143.2",
  "mgmt_ip": "10.134.144.2",
  "form_factor": "1U Rackmount",
  "warranty_expiry": "2027-09-01",
  "purchase_date": "2024-09-01",
  "uptime_days": 520,
  "mtbf_hours": 87000,
  "ha_status": "standby"
}' WHERE asset_tag = 'NET-SW-A02';

UPDATE assets SET attributes = '{
  "throughput": "72 Gbps",
  "firmware": "PAN-OS 11.1.2",
  "primary_ip": "10.134.143.10",
  "mgmt_ip": "10.134.144.10",
  "form_factor": "2U Rackmount",
  "warranty_expiry": "2027-06-01",
  "purchase_date": "2024-06-01",
  "ha_status": "active/active",
  "uptime_days": 380,
  "mtbf_hours": 62000,
  "sessions_active": 45000,
  "sessions_max": 100000
}' WHERE asset_tag = 'NET-FW-001';

UPDATE assets SET attributes = '{
  "capacity": "100kW / 360kWh",
  "battery_type": "Lithium-ion",
  "efficiency": "96%",
  "primary_ip": "10.134.145.1",
  "form_factor": "Floor Standing",
  "warranty_expiry": "2029-01-15",
  "purchase_date": "2024-01-15",
  "runtime_at_full_load": "18 minutes",
  "battery_health": "78%",
  "last_test_date": "2026-03-01"
}' WHERE asset_tag = 'PWR-UPS-001';

UPDATE assets SET attributes = '{
  "raw_capacity": "144TB (12x 12TB)",
  "usable_capacity": "108TB (RAID-6)",
  "protocol": "NFS/SMB/iSCSI",
  "primary_ip": "10.134.143.20",
  "mgmt_ip": "10.134.144.20",
  "form_factor": "4U Rackmount",
  "warranty_expiry": "2028-02-01",
  "purchase_date": "2025-02-01",
  "uptime_days": 410,
  "usage_percent": 68
}' WHERE asset_tag = 'STG-NAS-001';

UPDATE assets SET attributes = '{
  "raw_capacity": "460TB (24x 19.2TB NVMe)",
  "usable_capacity": "300TB",
  "protocol": "FC 32Gbps / NVMe-oF",
  "primary_ip": "10.134.143.21",
  "mgmt_ip": "10.134.144.21",
  "form_factor": "4U Rackmount",
  "warranty_expiry": "2028-08-01",
  "purchase_date": "2025-08-01",
  "uptime_days": 280,
  "iops": 500000,
  "latency_us": 200,
  "usage_percent": 42
}' WHERE asset_tag = 'STG-SAN-001';

-- ============================================================
-- Metrics seed data: 24-hour hourly readings for first 3 assets
-- cpu_usage (%), temperature (C), power_kw, memory_usage (%)
-- ============================================================
INSERT INTO metrics (asset_id, name, value, time) VALUES
  -- Asset 1: SRV-PROD-001 (f0000000-0000-0000-0000-000000000001)
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 72.5, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 68.3, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 55.1, now() - interval '21 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 48.7, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 42.3, now() - interval '19 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 44.9, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 51.2, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 63.8, now() - interval '16 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 78.4, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 82.1, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 79.6, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 74.2, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 71.8, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 68.5, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 65.3, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 59.7, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 54.1, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 47.6, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 43.2, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 52.8, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 61.4, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 70.9, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 76.3, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000001', 'cpu_usage', 73.1, now()),
  -- temperature for asset 1
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 24.2, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 23.8, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 23.1, now() - interval '21 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 22.5, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 22.8, now() - interval '19 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 23.4, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 24.6, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 26.1, now() - interval '16 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 28.3, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 29.7, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 30.2, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 29.4, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 28.1, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 27.3, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 26.5, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 25.8, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 24.9, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 24.1, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 23.5, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 24.3, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 25.7, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 27.2, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 28.6, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000001', 'temperature', 27.9, now()),
  -- power_kw for asset 1
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.1, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.9, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.6, now() - interval '21 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.4, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.3, now() - interval '19 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.5, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.7, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.0, now() - interval '16 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.4, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.7, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.6, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.3, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.2, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.0, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.9, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.7, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.5, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.3, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.2, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.6, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 1.9, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.2, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.5, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000001', 'power_kw', 2.3, now()),
  -- memory_usage for asset 1
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 62.4, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 61.8, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 58.3, now() - interval '21 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 55.1, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 52.7, now() - interval '19 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 53.9, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 56.2, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 60.5, now() - interval '16 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 67.8, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 72.3, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 74.1, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 71.6, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 68.9, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 66.2, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 63.5, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 59.8, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 56.4, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 53.1, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 50.8, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 54.6, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 58.9, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 64.7, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 69.3, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000001', 'memory_usage', 67.5, now()),

  -- Asset 2: SRV-PROD-002 (f0000000-0000-0000-0000-000000000002)
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 45.2, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 43.8, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 41.5, now() - interval '21 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 40.1, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 42.7, now() - interval '19 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 48.3, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 55.6, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 62.1, now() - interval '16 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 68.9, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 73.4, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 76.8, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 74.2, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 70.5, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 66.8, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 61.3, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 57.9, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 52.4, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 48.1, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 44.6, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 49.8, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 56.3, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 63.7, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 69.1, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000002', 'cpu_usage', 65.4, now()),
  -- temperature for asset 2
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 23.1, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 22.7, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 22.3, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 24.8, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 27.5, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 29.1, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 28.3, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 26.9, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 25.4, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 24.1, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 23.5, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 25.8, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'temperature', 26.4, now()),
  -- power_kw for asset 2
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 1.8, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 1.5, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 1.7, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 2.2, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 2.5, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 2.3, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 1.9, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 1.4, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 1.8, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 2.1, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000002', 'power_kw', 2.0, now()),
  -- memory_usage for asset 2
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 48.5, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 46.2, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 50.8, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 58.3, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 63.7, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 60.1, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 55.4, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 49.9, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 54.2, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000002', 'memory_usage', 57.8, now()),

  -- Asset 3: SRV-DEV-001 (f0000000-0000-0000-0000-000000000003)
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 58.3, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 54.7, now() - interval '22 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 49.2, now() - interval '21 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 44.8, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 41.3, now() - interval '19 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 46.5, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 53.8, now() - interval '17 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 61.2, now() - interval '16 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 69.7, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 75.3, now() - interval '14 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 78.9, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 80.4, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 77.1, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 72.6, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 67.4, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 62.8, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 57.3, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 51.9, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 47.5, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 53.1, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 59.6, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 66.2, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 71.8, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000003', 'cpu_usage', 68.5, now()),
  -- temperature for asset 3
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 25.3, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 24.6, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 23.8, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 26.2, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 29.8, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 31.4, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 30.1, now() - interval '11 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 28.5, now() - interval '9 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 26.7, now() - interval '7 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 25.1, now() - interval '5 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 26.9, now() - interval '3 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 28.7, now() - interval '1 hour'),
  ('f0000000-0000-0000-0000-000000000003', 'temperature', 27.4, now()),
  -- power_kw for asset 3
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 1.9, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 1.6, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 1.4, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 2.0, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 2.6, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 2.9, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 2.5, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 2.1, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 1.7, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 1.3, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 1.8, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'power_kw', 2.3, now()),
  -- memory_usage for asset 3
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 55.7, now() - interval '23 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 52.3, now() - interval '20 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 49.8, now() - interval '18 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 56.4, now() - interval '15 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 64.9, now() - interval '13 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 70.2, now() - interval '12 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 68.1, now() - interval '10 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 63.5, now() - interval '8 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 58.2, now() - interval '6 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 53.6, now() - interval '4 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 59.1, now() - interval '2 hours'),
  ('f0000000-0000-0000-0000-000000000003', 'memory_usage', 65.8, now())
ON CONFLICT DO NOTHING;

-- ============================================================
-- BIA (Business Impact Analysis) Seed Data
-- ============================================================

-- BIA Scoring Rules (4 tiers)
INSERT INTO bia_scoring_rules (id, tenant_id, tier_name, tier_level, display_name, min_score, max_score, rto_threshold, rpo_threshold, description, color, icon) VALUES
    ('90000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'critical', 1, 'Tier 1 - CRITICAL', 85, 100, 4, 15,
     '核心支付系統、棟宇監控等，停機即產生重大財務或安全影響', '#ff6b6b', 'error'),
    ('90000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'important', 2, 'Tier 2 - IMPORTANT', 60, 84, 12, 60,
     '核心系統群組（CRM、ERP），停機影響業務運作效率', '#ffa94d', 'warning'),
    ('90000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'normal', 3, 'Tier 3 - NORMAL', 30, 59, 24, 240,
     '一般業務系統，停機可使用替代方案', '#9ecaff', 'info'),
    ('90000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'minor', 4, 'Tier 4 - MINOR', 0, 29, 72, null,
     '測試、沙箱環境，停機無業務衝擊', '#8e9196', 'expand_circle_down')
ON CONFLICT DO NOTHING;

-- BIA Assessments (4 business systems matching screenshot)
INSERT INTO bia_assessments (id, tenant_id, system_name, system_code, owner, bia_score, tier, rto_hours, rpo_minutes, mtpd_hours, data_compliance, asset_compliance, audit_compliance, description, assessed_by) VALUES
    ('91000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     '核心支付閘道器 (Payment Gateway)', 'SYS-PROD-PAY-001', '運大文',
     98, 'critical', 4, 15, 8,
     true, true, true,
     '處理所有線上支付交易，連接銀行API和清算系統',
     'b0000000-0000-0000-0000-000000000001'),
    ('91000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     '客戶關係中心 (CRM Core)', 'SYS-PROD-CRM-001', '林昇',
     85, 'important', 12, 120, 24,
     true, true, false,
     '管理客戶資料、服務歷史和溝通記錄',
     'b0000000-0000-0000-0000-000000000002'),
    ('91000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     '內部管理系統 (Admin Panel)', 'SYS-CORP-ADM-001', '王志',
     62, 'normal', 24, 240, 48,
     true, false, false,
     '員工入職、請假、報銷等內部流程管理',
     'b0000000-0000-0000-0000-000000000001'),
    ('91000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'QA 測試環境 (QA Sandbox)', 'SYS-TEST-QA-001', null,
     15, 'minor', 72, null, null,
     false, false, false,
     'QA 團隊測試用環境',
     'b0000000-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;

-- BIA Dependencies (link business systems to infrastructure assets)
INSERT INTO bia_dependencies (tenant_id, assessment_id, asset_id, dependency_type, criticality) VALUES
    -- Payment Gateway → servers + network
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'runs_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002', 'runs_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000005', 'depends_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003', 'depends_on', 'medium'),
    -- CRM → servers + storage
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000006', 'runs_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000004', 'depends_on', 'medium'),
    -- Admin Panel → app server
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000007', 'runs_on', 'low'),
    -- QA Sandbox → dev server
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000008', 'runs_on', 'low')
ON CONFLICT DO NOTHING;

-- Quality Rules
INSERT INTO quality_rules (tenant_id, ci_type, dimension, field_name, rule_type, rule_config, weight) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'server', 'completeness', 'serial_number', 'required', '{}', 15),
    ('a0000000-0000-0000-0000-000000000001', 'server', 'completeness', 'vendor', 'required', '{}', 10),
    ('a0000000-0000-0000-0000-000000000001', NULL, 'accuracy', 'serial_number', 'regex', '{"regex": "^[A-Z0-9\\-]{5,30}$"}', 20),
    ('a0000000-0000-0000-0000-000000000001', 'network', 'completeness', 'serial_number', 'required', '{}', 15),
    ('a0000000-0000-0000-0000-000000000001', NULL, 'consistency', 'rack_id', 'required', '{}', 25)
ON CONFLICT DO NOTHING;
