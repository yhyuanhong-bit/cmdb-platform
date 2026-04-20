-- user_sessions: per-user browser/session registry. The table has NO
-- tenant_id column — isolation is by user_id (and users themselves are
-- tenant-scoped). All queries below are therefore "cross-tenant" in the
-- strict TenantScoped sense; callers go through pool.* and own their
-- own authorization check before issuing queries.

-- name: ListUserSessions :many
-- cross-tenant: user_sessions has no tenant_id. Handler enforces that
-- the caller is either listing their own sessions or is admin.
SELECT id, ip_address, device_type, browser, created_at, last_active_at, is_current
FROM user_sessions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 20;

-- name: ClearCurrentUserSessions :exec
-- cross-tenant: by user_id. Called on login to mark all existing
-- sessions as non-current before inserting the fresh one.
UPDATE user_sessions SET is_current = false WHERE user_id = $1;

-- name: InsertUserSession :exec
-- cross-tenant: by user_id. Called on successful login. is_current
-- defaults to true for the session just created.
INSERT INTO user_sessions (user_id, ip_address, user_agent, device_type, browser, is_current)
VALUES ($1, $2, $3, $4, $5, true);

-- name: ExpireIdleUserSessions :execrows
-- cross-tenant by design: hourly cleanup sweep marks sessions whose
-- last_active_at is older than 7 days as expired.
UPDATE user_sessions
SET expired_at = now()
WHERE expired_at IS NULL
  AND last_active_at < now() - interval '7 days';

-- name: DeleteOldUserSessions :execrows
-- cross-tenant by design: hourly cleanup sweep deletes sessions older
-- than 30 days regardless of tenant.
DELETE FROM user_sessions WHERE created_at < now() - interval '30 days';

-- name: TrimUserSessionsPerUser :execrows
-- cross-tenant by design: keeps only the 20 most recent sessions per
-- user_id, deleting the excess. Run hourly by the cleanup loop.
DELETE FROM user_sessions WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) AS rn
        FROM user_sessions
    ) ranked WHERE rn > 20
);
