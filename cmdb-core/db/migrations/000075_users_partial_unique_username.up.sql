-- 000075_users_partial_unique_username.up.sql
--
-- Fix the "delete user appears to do nothing" bug from System Settings.
--
-- Before: (tenant_id, username) was unconditionally UNIQUE, so a soft-
-- deleted row blocked re-creating the same username forever. Combined
-- with ListUsers/CountUsers/GetUserBy* queries that didn't filter
-- deleted rows out, the operator's experience was:
--   1. Click "Delete" → row stays visible in the list
--   2. Try to recreate the user → "username already exists" error
--   3. Conclude that delete doesn't work
--
-- Fix:
--   - Drop the unconditional UNIQUE on (tenant_id, username).
--   - Replace with a partial UNIQUE index that only enforces uniqueness
--     among NON-deleted rows. Soft-deleted rows are exempt, so
--     re-creating the username after delete works.
--
-- The status='active'-vs-'deleted' check at the SQL-query layer is
-- handled in a sibling sqlc-query update (commit accompanying this
-- migration). This file is only the schema half.
--
-- Note: any existing rows with status='deleted' that would conflict
-- with an active row are extremely unlikely to exist (the bug above
-- prevented operators from creating them in the first place), but the
-- DROP runs before the new index creation so order is safe regardless.
--
-- The original was created by `CREATE UNIQUE INDEX` in an earlier
-- migration (not via ALTER TABLE ADD CONSTRAINT), so it lives as a
-- plain unique index — DROP INDEX is the correct cleanup. An earlier
-- draft of this migration used DROP CONSTRAINT and was silently a
-- no-op; that's why the rename failed during dev verification.

-- Backfill: any rows that were soft-deleted before this migration set
-- only `status='deleted'` without populating `deleted_at`. Both filters
-- need to agree going forward, so stamp deleted_at = updated_at on the
-- legacy rows. Live deployments may have a handful (the bug masked
-- their existence in the UI but the DB rows are real).
UPDATE users
   SET deleted_at = updated_at
 WHERE status = 'deleted' AND deleted_at IS NULL;

DROP INDEX IF EXISTS users_tenant_username_unique;

CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_username_active_unique
    ON users (tenant_id, username)
    WHERE deleted_at IS NULL;

-- Migration 000052 created `trg_tenant_seed_system_user` which inserts a
-- per-tenant 'system' user with `ON CONFLICT (tenant_id, username) DO
-- NOTHING`. PostgreSQL requires the conflict spec to match an existing
-- unique constraint or index — after dropping `users_tenant_username_unique`,
-- it has to point to the new partial index instead. The fix is to spell
-- out the partial predicate `WHERE deleted_at IS NULL` in the ON CONFLICT
-- clause so PG can use the partial index for arbitration.
CREATE OR REPLACE FUNCTION seed_system_user_for_new_tenant() RETURNS trigger AS $$
BEGIN
    INSERT INTO users (tenant_id, username, display_name, email, password_hash, status, source)
    VALUES (NEW.id, 'system', 'System', 'system@' || NEW.slug || '.local', '!', 'active', 'system')
    ON CONFLICT (tenant_id, username) WHERE deleted_at IS NULL DO NOTHING;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
