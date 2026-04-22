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

-- name: CreateQualityFlag :one
-- Record an external report that an asset's data is wrong. The scanner
-- reads open+acknowledged rows from the last 24h to penalize the asset
-- score until the flag is triaged.
INSERT INTO quality_flags (
    tenant_id, asset_id, reporter_type, reporter_id,
    severity, category, message
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetQualityFlag :one
SELECT * FROM quality_flags
WHERE id = $1 AND tenant_id = $2;

-- name: ListQualityFlagsForAsset :many
SELECT * FROM quality_flags
WHERE tenant_id = $1 AND asset_id = $2
ORDER BY created_at DESC
LIMIT 50;

-- name: ListOpenQualityFlags :many
-- Triage list — newest critical/high first, then medium/low.
SELECT qf.*, a.name AS asset_name, a.asset_tag
FROM quality_flags qf
JOIN assets a ON qf.asset_id = a.id
WHERE qf.tenant_id = $1 AND qf.status = 'open'
ORDER BY
    CASE qf.severity
        WHEN 'critical' THEN 0
        WHEN 'high'     THEN 1
        WHEN 'medium'   THEN 2
        WHEN 'low'      THEN 3
    END,
    qf.created_at DESC
LIMIT 100;

-- name: CountRecentFlagsByAsset :many
-- Aggregate in one pass for the scan loop. Only open+acknowledged
-- rows count — a rejected or resolved flag must not keep dragging
-- the score down forever.
SELECT asset_id, COUNT(*) AS flag_count
FROM quality_flags
WHERE tenant_id = $1
  AND status IN ('open', 'acknowledged')
  AND created_at > now() - interval '24 hours'
GROUP BY asset_id;

-- name: ResolveQualityFlag :one
UPDATE quality_flags
SET status          = $3,
    resolved_at     = now(),
    resolved_by     = $4,
    resolution_note = $5
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: AssetsLowQualityPersistent :many
-- D9-P0 auto-WO driver. Returns assets whose newest-per-day scores
-- have stayed below the threshold on every day in the lookback
-- window AND have at least one score per day (i.e. the scanner
-- actually ran), so we don't fire a work order on a scoring gap.
-- Caller passes threshold (typically 40) and required_days (7).
SELECT asset_id
FROM (
    SELECT asset_id,
           COUNT(DISTINCT DATE_TRUNC('day', scan_date)) AS days_covered,
           MAX(total_score)::float8 AS max_score
    FROM quality_scores
    WHERE tenant_id = $1
      AND scan_date > now() - ($3::int * interval '1 day')
    GROUP BY asset_id
) agg
WHERE agg.days_covered >= $3
  AND agg.max_score < $2::float8;

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
