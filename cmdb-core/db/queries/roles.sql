-- name: ListRoles :many
SELECT * FROM roles
WHERE tenant_id = $1 OR tenant_id IS NULL
ORDER BY is_system DESC, name;

-- name: CreateRole :one
INSERT INTO roles (
    tenant_id, name, description, permissions, is_system
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdateRole :one
--
-- Tenant-scoped role update. The (id, tenant_id) WHERE pair prevents
-- tenant A's admin from editing tenant B's custom role by guessing UUID.
-- System roles (tenant_id IS NULL) are protected by `is_system = false`.
UPDATE roles SET
    name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    permissions = COALESCE(sqlc.narg('permissions'), permissions)
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND is_system = false
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1 AND tenant_id = $2 AND is_system = false;

-- name: GetRole :one
SELECT * FROM roles WHERE id = $1;

-- name: ListUserRoles :many
SELECT r.* FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1;

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveRole :exec
DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2;

-- name: ListUserRoleIDs :many
SELECT role_id FROM user_roles WHERE user_id = $1;
