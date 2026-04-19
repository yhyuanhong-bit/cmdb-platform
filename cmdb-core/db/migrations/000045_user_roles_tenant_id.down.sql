-- Revert migration 000045.

DROP TRIGGER IF EXISTS trg_user_roles_tenant_check ON user_roles;
DROP FUNCTION IF EXISTS user_roles_tenant_check();
DROP INDEX IF EXISTS idx_user_roles_tenant_id;
ALTER TABLE user_roles DROP COLUMN IF EXISTS tenant_id;
