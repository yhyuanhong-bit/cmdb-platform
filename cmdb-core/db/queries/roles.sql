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
UPDATE roles SET
    name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    permissions = COALESCE(sqlc.narg('permissions'), permissions)
WHERE id = sqlc.arg('id') AND is_system = false
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1 AND tenant_id = $2 AND is_system = false;

-- name: ListUserRoles :many
SELECT r.* FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1;

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
