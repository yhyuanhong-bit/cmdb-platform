DELETE FROM asset_field_authorities
WHERE field_name IN ('bmc_ip', 'bmc_type', 'bmc_firmware')
AND tenant_id = 'a0000000-0000-0000-0000-000000000001';

DELETE FROM asset_field_authorities
WHERE field_name = 'ip_address'
AND tenant_id = 'a0000000-0000-0000-0000-000000000001';

DELETE FROM asset_field_authorities
WHERE source_type = 'excel'
AND field_name IN ('vendor', 'model', 'serial_number')
AND tenant_id = 'a0000000-0000-0000-0000-000000000001';
