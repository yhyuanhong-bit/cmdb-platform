-- Add field authority definitions for BMC and new fields
INSERT INTO asset_field_authorities (tenant_id, field_name, source_type, priority) VALUES
    -- bmc_ip: IPMI scan is most authoritative, then SNMP, then manual/excel
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',       'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',       'snmp',   80),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',       'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_ip',       'manual', 50),
    -- bmc_type: IPMI detects this automatically
    ('a0000000-0000-0000-0000-000000000001', 'bmc_type',     'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_type',     'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_type',     'manual', 50),
    -- bmc_firmware: only IPMI scan knows the real version
    ('a0000000-0000-0000-0000-000000000001', 'bmc_firmware', 'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'bmc_firmware', 'manual', 30),
    -- ip_address: SNMP/MAC scan most reliable, then manual
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',   'snmp',   100),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',   'mac',    90),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',   'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'ip_address',   'manual', 70),
    -- vendor/model: add excel source (was missing)
    ('a0000000-0000-0000-0000-000000000001', 'vendor',       'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'model',        'excel',  60),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number','excel',  60)
ON CONFLICT DO NOTHING;
