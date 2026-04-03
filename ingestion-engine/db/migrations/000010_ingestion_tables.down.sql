-- Remove seeded field authorities
DELETE FROM asset_field_authorities
WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001';

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS discovery_candidates;
DROP TABLE IF EXISTS discovery_tasks;
DROP TABLE IF EXISTS import_jobs;
DROP TABLE IF EXISTS import_conflicts;
DROP TABLE IF EXISTS asset_field_authorities;
