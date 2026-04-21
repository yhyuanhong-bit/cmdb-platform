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

-- name: GetDownstreamDependencies :many
-- Recursive CTE: starting from @root_asset_id, follow source→target edges
-- up to @max_depth hops. Returns every reachable dependency edge with its
-- depth (1 = direct) and the full path of asset IDs visited from root.
-- Cycle-safe via the path accumulator: the recursive step rejects an edge
-- whose next node (the target) is already in path, so A→B→A terminates.
-- tenant_id is enforced at every hop; cross-tenant edges cannot leak in
-- even if an asset row somehow referenced a foreign tenant.
WITH RECURSIVE tree AS (
    SELECT
        ad.id,
        ad.source_asset_id,
        ad.target_asset_id,
        ad.dependency_type,
        1::int AS depth,
        ARRAY[ad.source_asset_id, ad.target_asset_id]::uuid[] AS path
    FROM asset_dependencies ad
    WHERE ad.tenant_id = sqlc.arg('tenant_id')
      AND ad.source_asset_id = sqlc.arg('root_asset_id')

    UNION ALL

    SELECT
        ad.id,
        ad.source_asset_id,
        ad.target_asset_id,
        ad.dependency_type,
        t.depth + 1,
        t.path || ad.target_asset_id
    FROM asset_dependencies ad
    JOIN tree t ON ad.source_asset_id = t.target_asset_id
    WHERE ad.tenant_id = sqlc.arg('tenant_id')
      AND t.depth < sqlc.arg('max_depth')::int
      AND NOT (ad.target_asset_id = ANY(t.path))
)
SELECT
    t.id,
    t.source_asset_id,
    sa.name AS source_asset_name,
    t.target_asset_id,
    ta.name AS target_asset_name,
    t.dependency_type,
    t.depth,
    t.path
FROM tree t
JOIN assets sa ON t.source_asset_id = sa.id
JOIN assets ta ON t.target_asset_id = ta.id
ORDER BY t.depth, sa.name, ta.name;

-- name: GetUpstreamDependents :many
-- Mirror of GetDownstreamDependencies traversing target→source edges: given
-- @root_asset_id, return every asset that (transitively) depends on it. Used
-- for impact analysis ("if I take this asset down, what else breaks?").
-- Same cycle-safety and tenant_id enforcement as the downstream variant.
WITH RECURSIVE tree AS (
    SELECT
        ad.id,
        ad.source_asset_id,
        ad.target_asset_id,
        ad.dependency_type,
        1::int AS depth,
        ARRAY[ad.target_asset_id, ad.source_asset_id]::uuid[] AS path
    FROM asset_dependencies ad
    WHERE ad.tenant_id = sqlc.arg('tenant_id')
      AND ad.target_asset_id = sqlc.arg('root_asset_id')

    UNION ALL

    SELECT
        ad.id,
        ad.source_asset_id,
        ad.target_asset_id,
        ad.dependency_type,
        t.depth + 1,
        t.path || ad.source_asset_id
    FROM asset_dependencies ad
    JOIN tree t ON ad.target_asset_id = t.source_asset_id
    WHERE ad.tenant_id = sqlc.arg('tenant_id')
      AND t.depth < sqlc.arg('max_depth')::int
      AND NOT (ad.source_asset_id = ANY(t.path))
)
SELECT
    t.id,
    t.source_asset_id,
    sa.name AS source_asset_name,
    t.target_asset_id,
    ta.name AS target_asset_name,
    t.dependency_type,
    t.depth,
    t.path
FROM tree t
JOIN assets sa ON t.source_asset_id = sa.id
JOIN assets ta ON t.target_asset_id = ta.id
ORDER BY t.depth, sa.name, ta.name;
