-- Rollback Phase 1.3. Only succeeds if no cross-tenant username duplicates
-- exist; if they do, the UNIQUE constraint creation will fail and the
-- operator must resolve duplicates before downgrading.

DROP INDEX IF EXISTS users_tenant_username_unique;

ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
