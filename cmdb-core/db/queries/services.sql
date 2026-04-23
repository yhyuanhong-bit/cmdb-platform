-- Business Service entity queries (Wave 2).
--
-- Every query is tenant-scoped. Soft-deleted rows (deleted_at IS NOT NULL)
-- are hidden from List/Get/Count; CRUD updates by id always include the
-- tenant filter to prevent cross-tenant access via UUID guessing.

-- ============================================================================
-- services: CRUD
-- ============================================================================

-- name: ListServices :many
SELECT * FROM services
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::text = '' OR tier = $2)
  AND ($3::text = '' OR status = $3)
  AND ($4::text = '' OR owner_team = $4)
ORDER BY name
LIMIT $5 OFFSET $6;

-- name: CountServices :one
SELECT count(*) FROM services
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::text = '' OR tier = $2)
  AND ($3::text = '' OR status = $3)
  AND ($4::text = '' OR owner_team = $4);

-- name: GetService :one
SELECT * FROM services
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: GetServiceByCode :one
SELECT * FROM services
WHERE tenant_id = $1 AND code = $2 AND deleted_at IS NULL;

-- name: CreateService :one
INSERT INTO services (
    tenant_id, code, name, description, tier, owner_team,
    bia_assessment_id, status, tags, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateService :one
UPDATE services SET
    name              = COALESCE($3, name),
    description       = COALESCE($4, description),
    tier              = COALESCE($5, tier),
    owner_team        = COALESCE($6, owner_team),
    bia_assessment_id = COALESCE($7, bia_assessment_id),
    status            = COALESCE($8, status),
    tags              = COALESCE($9, tags),
    updated_at        = now(),
    sync_version      = sync_version + 1
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteService :exec
UPDATE services SET
    deleted_at   = now(),
    updated_at   = now(),
    sync_version = sync_version + 1
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- ============================================================================
-- service_assets: membership
-- ============================================================================

-- name: ListServiceAssets :many
SELECT
    sa.service_id,
    sa.asset_id,
    sa.tenant_id,
    sa.role,
    sa.is_critical,
    sa.created_at,
    sa.created_by,
    a.asset_tag,
    a.name AS asset_name,
    a.status AS asset_status,
    a.type AS asset_type
FROM service_assets sa
JOIN assets a ON a.id = sa.asset_id AND a.deleted_at IS NULL
WHERE sa.service_id = $1 AND sa.tenant_id = $2
ORDER BY sa.is_critical DESC, a.name;

-- name: ListServicesForAsset :many
-- Reverse lookup: given an asset, return the services it belongs to.
-- Used by the Asset detail page to show "Belongs to services: ..."
SELECT
    s.id,
    s.code,
    s.name,
    s.tier,
    s.status,
    sa.role,
    sa.is_critical
FROM service_assets sa
JOIN services s ON s.id = sa.service_id AND s.deleted_at IS NULL
WHERE sa.asset_id = $1 AND sa.tenant_id = $2
ORDER BY s.tier, s.name;

-- name: AddServiceAsset :one
INSERT INTO service_assets (service_id, asset_id, tenant_id, role, is_critical, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateServiceAssetRole :one
UPDATE service_assets SET
    role        = $3,
    is_critical = $4
WHERE service_id = $1 AND asset_id = $2 AND tenant_id = $5
RETURNING *;

-- name: RemoveServiceAsset :exec
DELETE FROM service_assets
WHERE service_id = $1 AND asset_id = $2 AND tenant_id = $3;

-- name: CountCriticalServiceAssets :one
-- Used by the service-health endpoint. Returns how many critical assets
-- the service has and how many of them are currently unhealthy.
SELECT
    count(*) AS critical_total,
    count(*) FILTER (WHERE a.status NOT IN ('active', 'operational', 'deployed')) AS critical_unhealthy
FROM service_assets sa
JOIN assets a ON a.id = sa.asset_id AND a.deleted_at IS NULL
WHERE sa.service_id = $1
  AND sa.tenant_id = $2
  AND sa.is_critical = true;
