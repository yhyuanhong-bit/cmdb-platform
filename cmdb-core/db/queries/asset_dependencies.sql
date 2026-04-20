-- asset_dependencies: directed edges between assets (source depends on
-- target). tenant_id is a first-class column and indexed; ListAssetDependencies
-- and DeleteAssetDependency enforce it explicitly. ListAssetDependenciesByAssetIDs
-- (used by the topology graph builder) deliberately omits the tenant_id filter
-- because its caller has already narrowed the asset ID set to a single tenant;
-- adding one here would be a no-op in the common case and a behavior change
-- for the edge case where an external (cross-location, same-tenant) asset is
-- part of the graph. Pre-migration behavior preserved.

-- name: ListAssetDependencies :many
-- Returns dependency edges where the given asset is either source or target,
-- joined to the assets table on both ends for display names.
SELECT
    ad.id,
    ad.source_asset_id,
    sa.name AS source_asset_name,
    ad.target_asset_id,
    ta.name AS target_asset_name,
    ad.dependency_type,
    COALESCE(ad.description, '') AS description
FROM asset_dependencies ad
JOIN assets sa ON ad.source_asset_id = sa.id
JOIN assets ta ON ad.target_asset_id = ta.id
WHERE ad.tenant_id = $1
  AND (ad.source_asset_id = $2 OR ad.target_asset_id = $2);

-- name: CreateAssetDependency :exec
-- The unique index on (source_asset_id, target_asset_id, dependency_type)
-- means duplicate edges surface as a unique-violation error — callers
-- translate that to HTTP 409.
INSERT INTO asset_dependencies (id, tenant_id, source_asset_id, target_asset_id, dependency_type, description)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteAssetDependency :execrows
-- Returns rows affected so the caller can map 0 -> 404.
DELETE FROM asset_dependencies
WHERE id = $1 AND tenant_id = $2;

-- name: ListAssetDependenciesByAssetIDs :many
-- cross-tenant: no tenant_id filter by design. Caller (GetTopologyGraph)
-- has already scoped the asset ID list to a single tenant via an earlier
-- tenant-scoped query; asset_dependencies rows referencing IDs outside that
-- set cannot appear. Preserving pre-migration behavior verbatim.
SELECT id, source_asset_id, target_asset_id, dependency_type
FROM asset_dependencies
WHERE source_asset_id = ANY($1::uuid[]) OR target_asset_id = ANY($1::uuid[]);
