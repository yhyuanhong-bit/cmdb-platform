-- 000075_users_partial_unique_username.down.sql
--
-- Restore the unconditional UNIQUE constraint. If any soft-deleted
-- rows share a username with an active row, the ADD CONSTRAINT will
-- fail — operator must reconcile manually before downgrading.

DROP INDEX IF EXISTS users_tenant_username_active_unique;

-- Re-create as a unique index (matching how it lived before 000075,
-- not as an ALTER TABLE constraint).
CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_username_unique
    ON users (tenant_id, username);

-- Restore the original trigger function (ON CONFLICT without partial
-- predicate, since the index it points to is unconditional again).
CREATE OR REPLACE FUNCTION seed_system_user_for_new_tenant() RETURNS trigger AS $$
BEGIN
    INSERT INTO users (tenant_id, username, display_name, email, password_hash, status, source)
    VALUES (NEW.id, 'system', 'System', 'system@' || NEW.slug || '.local', '!', 'active', 'system')
    ON CONFLICT (tenant_id, username) DO NOTHING;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
