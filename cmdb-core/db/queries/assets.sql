-- name: ListAssets :many
SELECT * FROM assets
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('location_id')::uuid IS NULL OR location_id = sqlc.narg('location_id'))
  AND (sqlc.narg('rack_id')::uuid IS NULL OR rack_id = sqlc.narg('rack_id'))
  AND (sqlc.narg('serial_number')::varchar IS NULL OR serial_number = sqlc.narg('serial_number'))
  AND (sqlc.narg('search')::varchar IS NULL OR (name ILIKE '%' || sqlc.narg('search') || '%' OR asset_tag ILIKE '%' || sqlc.narg('search') || '%'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAssets :one
SELECT count(*) FROM assets
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('location_id')::uuid IS NULL OR location_id = sqlc.narg('location_id'))
  AND (sqlc.narg('rack_id')::uuid IS NULL OR rack_id = sqlc.narg('rack_id'))
  AND (sqlc.narg('serial_number')::varchar IS NULL OR serial_number = sqlc.narg('serial_number'))
  AND (sqlc.narg('search')::varchar IS NULL OR (name ILIKE '%' || sqlc.narg('search') || '%' OR asset_tag ILIKE '%' || sqlc.narg('search') || '%'));

-- name: GetAsset :one
SELECT * FROM assets WHERE id = $1 AND tenant_id = $2;

-- name: GetAssetByTag :one
SELECT * FROM assets WHERE asset_tag = $1;

-- name: CreateAsset :one
INSERT INTO assets (
    tenant_id, asset_tag, property_number, control_number, name,
    type, sub_type, status, bia_level, location_id,
    rack_id, vendor, model, serial_number, attributes, tags,
    bmc_ip, bmc_type, bmc_firmware
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16,
    $17, $18, $19
) RETURNING *;

-- name: UpdateAsset :one
UPDATE assets SET
    asset_tag       = COALESCE(sqlc.narg('asset_tag'), asset_tag),
    property_number = COALESCE(sqlc.narg('property_number'), property_number),
    control_number  = COALESCE(sqlc.narg('control_number'), control_number),
    name            = COALESCE(sqlc.narg('name'), name),
    type            = COALESCE(sqlc.narg('type'), type),
    sub_type        = COALESCE(sqlc.narg('sub_type'), sub_type),
    status          = COALESCE(sqlc.narg('status'), status),
    bia_level       = COALESCE(sqlc.narg('bia_level'), bia_level),
    location_id     = COALESCE(sqlc.narg('location_id'), location_id),
    rack_id         = COALESCE(sqlc.narg('rack_id'), rack_id),
    vendor          = COALESCE(sqlc.narg('vendor'), vendor),
    model           = COALESCE(sqlc.narg('model'), model),
    serial_number   = COALESCE(sqlc.narg('serial_number'), serial_number),
    attributes      = COALESCE(sqlc.narg('attributes'), attributes),
    tags            = COALESCE(sqlc.narg('tags'), tags),
    bmc_ip          = COALESCE(sqlc.narg('bmc_ip'), bmc_ip),
    bmc_type        = COALESCE(sqlc.narg('bmc_type'), bmc_type),
    bmc_firmware    = COALESCE(sqlc.narg('bmc_firmware'), bmc_firmware),
    updated_at      = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = $1 AND tenant_id = $2;

-- name: FindAssetBySerialOrTag :one
SELECT * FROM assets
WHERE tenant_id = $1
  AND (serial_number = $2 OR asset_tag = $3)
LIMIT 1;

-- name: CountAssetsUnderLocation :one
-- Count all assets under a location and all its descendants (using ltree)
SELECT count(*) FROM assets a
JOIN locations l ON a.location_id = l.id
WHERE a.tenant_id = $1
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree;
