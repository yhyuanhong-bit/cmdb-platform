-- name: ListQualityRules :many
SELECT * FROM quality_rules
WHERE tenant_id = $1
ORDER BY dimension, field_name;

-- name: CreateQualityRule :one
INSERT INTO quality_rules (tenant_id, ci_type, dimension, field_name, rule_type, rule_config, weight, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CreateQualityScore :exec
INSERT INTO quality_scores (tenant_id, asset_id, completeness, accuracy, timeliness, consistency, total_score, issue_details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetQualityDashboard :one
SELECT
    coalesce(avg(total_score), 0) as avg_total,
    coalesce(avg(completeness), 0) as avg_completeness,
    coalesce(avg(accuracy), 0) as avg_accuracy,
    coalesce(avg(timeliness), 0) as avg_timeliness,
    coalesce(avg(consistency), 0) as avg_consistency,
    count(*) as total_scanned
FROM quality_scores
WHERE tenant_id = $1
  AND scan_date > now() - interval '24 hours';

-- name: GetWorstAssets :many
SELECT qs.*, a.name as asset_name, a.asset_tag
FROM quality_scores qs
JOIN assets a ON qs.asset_id = a.id
WHERE qs.tenant_id = $1
  AND qs.scan_date > now() - interval '24 hours'
ORDER BY qs.total_score ASC
LIMIT 10;

-- name: GetAssetQualityHistory :many
SELECT * FROM quality_scores
WHERE asset_id = $1
ORDER BY scan_date DESC
LIMIT 30;

-- name: AvgLatestQualityScore :one
-- Average of each asset's most-recent total_score. DISTINCT ON picks the
-- newest row per asset_id; averaging over that avoids skew from assets
-- that were scanned many times vs. a single historical run pulling the
-- mean down. Returns 0 when a tenant has no scores yet (coalesce wraps
-- the outer avg).
SELECT coalesce(avg(total_score), 0)::float8
FROM (
    SELECT DISTINCT ON (asset_id) total_score
    FROM quality_scores
    WHERE tenant_id = $1
    ORDER BY asset_id, scan_date DESC
) latest;
