-- name: GetTenant :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: ListTenants :many
SELECT * FROM tenants ORDER BY name;

-- name: ListActiveTenants :many
-- Returns tenants currently eligible for scheduled per-tenant work
-- (governance scans, etc.). `status = 'active'` is the tenants-table
-- soft-active signal; the table has no `deleted_at` column.
SELECT id, name, slug FROM tenants WHERE status = 'active' ORDER BY name;
