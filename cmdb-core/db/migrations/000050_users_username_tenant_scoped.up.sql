-- Phase 1.3: scope users.username uniqueness to (tenant_id, username).
--
-- Before this migration, `username` was globally unique across all tenants,
-- which meant two tenants couldn't both have a user named "admin". The new
-- constraint allows per-tenant namespaces while keeping deterministic lookup
-- via (tenant_id, username).
--
-- The login API gains an optional `tenant_slug` field for explicit
-- disambiguation. When omitted, the server falls back to a global-unique
-- lookup and logs a deprecation warning; ambiguous usernames in that path
-- fail closed.

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;

CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_username_unique
    ON users (tenant_id, username);
