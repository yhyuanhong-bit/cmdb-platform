DROP INDEX IF EXISTS idx_locations_not_deleted;
DROP INDEX IF EXISTS idx_racks_not_deleted;
DROP INDEX IF EXISTS idx_users_not_deleted;

ALTER TABLE locations DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE racks DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
