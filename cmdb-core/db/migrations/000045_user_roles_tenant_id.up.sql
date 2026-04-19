-- Migration 000045: add tenant_id to user_roles and enforce same-tenant
-- user↔role assignment at the database layer.
--
-- Rationale: user_roles previously had no tenant_id column, which made it
-- structurally possible to assign a role belonging to tenant A to a user
-- belonging to tenant B. The application layer did not reliably prevent this,
-- so we add a defense-in-depth trigger that validates the tenant match on
-- every INSERT/UPDATE and stamps user_roles.tenant_id from users.tenant_id.
--
-- System roles (roles.tenant_id IS NULL) remain assignable to any user —
-- they are intentionally global. The trigger only fires when the role has
-- a non-null tenant_id that differs from the user's tenant_id.

-- 1. Add the column as nullable so we can backfill in place.
ALTER TABLE user_roles ADD COLUMN IF NOT EXISTS tenant_id UUID;

-- 2. Backfill tenant_id from the owning user.
UPDATE user_roles ur
   SET tenant_id = u.tenant_id
  FROM users u
 WHERE ur.user_id = u.id
   AND ur.tenant_id IS NULL;

-- 3. Flip to NOT NULL once backfilled.
ALTER TABLE user_roles ALTER COLUMN tenant_id SET NOT NULL;

-- 4. Trigger function: validate tenant match and stamp the column.
--    Fires on INSERT and UPDATE. Looks up users.tenant_id and roles.tenant_id;
--    if the role is tenant-scoped (non-null) and differs from the user's
--    tenant, raise an exception. Otherwise, copy the user's tenant_id into
--    NEW.tenant_id so callers cannot spoof it.
CREATE OR REPLACE FUNCTION user_roles_tenant_check() RETURNS trigger AS $$
DECLARE
    utenant UUID;
    rtenant UUID;
BEGIN
    SELECT tenant_id INTO utenant FROM users WHERE id = NEW.user_id;
    IF utenant IS NULL THEN
        RAISE EXCEPTION 'user_roles_tenant_check: user % not found or has no tenant', NEW.user_id;
    END IF;

    SELECT tenant_id INTO rtenant FROM roles WHERE id = NEW.role_id;

    IF rtenant IS NOT NULL AND utenant <> rtenant THEN
        RAISE EXCEPTION 'cross-tenant role assignment: user=% (tenant=%) role=% (tenant=%)',
            NEW.user_id, utenant, NEW.role_id, rtenant;
    END IF;

    NEW.tenant_id := utenant;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_user_roles_tenant_check ON user_roles;
CREATE TRIGGER trg_user_roles_tenant_check
    BEFORE INSERT OR UPDATE ON user_roles
    FOR EACH ROW EXECUTE FUNCTION user_roles_tenant_check();

-- 5. Index for tenant-scoped lookups.
CREATE INDEX IF NOT EXISTS idx_user_roles_tenant_id ON user_roles (tenant_id);
