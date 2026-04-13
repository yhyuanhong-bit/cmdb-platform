-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: ListUsers :many
SELECT * FROM users
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUsers :one
SELECT count(*) FROM users WHERE tenant_id = $1;

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
