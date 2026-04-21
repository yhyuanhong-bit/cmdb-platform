-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
--
-- Legacy global-unique lookup. Phase 1.3 scoped username uniqueness to
-- (tenant_id, username); prefer GetUserByTenantAndUsername when a tenant
-- context is known. This query now returns multiple rows when two tenants
-- reuse the same username — callers must treat "more than one match" as
-- ambiguous and fail closed.
SELECT * FROM users WHERE username = $1;

-- name: GetUserByTenantAndUsername :one
SELECT * FROM users WHERE tenant_id = $1 AND username = $2;

-- name: ListUsersByUsername :many
--
-- Disambiguation helper for the legacy login path: fetch every user that
-- matches a given username across all tenants. A result of length 1 means
-- the username is globally unique and the login can proceed; length > 1
-- means the caller must require a tenant_slug.
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
--
-- The per-tenant source='system' user (seeded by migration 000052) is
-- filtered out here so UI pickers and user lists don't expose it. It's a
-- FK-safe sentinel, not a human identity.
SELECT * FROM users
WHERE tenant_id = $1
  AND source <> 'system'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE tenant_id = $1
  AND source <> 'system';

-- name: CreateUser :one
INSERT INTO users (
    tenant_id, dept_id, username, display_name, email,
    phone, password_hash, status, source
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9
) RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
    dept_id       = COALESCE(sqlc.narg('dept_id'), dept_id),
    display_name  = COALESCE(sqlc.narg('display_name'), display_name),
    email         = COALESCE(sqlc.narg('email'), email),
    phone         = COALESCE(sqlc.narg('phone'), phone),
    password_hash = COALESCE(sqlc.narg('password_hash'), password_hash),
    status        = COALESCE(sqlc.narg('status'), status),
    updated_at    = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeactivateUser :exec
UPDATE users SET status = 'deleted', updated_at = now() WHERE id = $1 AND tenant_id = $2;
