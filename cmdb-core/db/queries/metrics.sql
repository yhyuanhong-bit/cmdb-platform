-- name: QueryMetricsByAsset :many
SELECT time, name, value
FROM metrics
WHERE asset_id = $1
  AND name = $2
  AND time > $3
ORDER BY time DESC
LIMIT 500;

-- name: InsertMetric :exec
INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: SumLatestPowerKW :one
-- Sum of each asset's most-recent power.current_w reading within the
-- last 10 minutes, converted to kilowatts. DISTINCT ON picks the
-- newest value per asset; stale assets (no reading in the window)
-- drop out of the sum rather than contributing zeroed-out energy.
-- Returns 0 when no readings exist. The 10-minute lookback matches
-- the typical scrape cadence with headroom for one missed interval.
SELECT (COALESCE(SUM(value), 0) / 1000.0)::float8
FROM (
    SELECT DISTINCT ON (asset_id) value
    FROM metrics
    WHERE tenant_id = sqlc.arg('tenant_id')::uuid
      AND name = 'power.current_w'
      AND time > now() - interval '10 minutes'
    ORDER BY asset_id, time DESC
) latest;

-- name: AggregateMetricPerAsset :many
-- Tenant-scoped aggregation used by the alert evaluator. Every call is
-- strictly scoped by tenant_id so rules never read another tenant's metrics.
-- The aggregation is chosen via $3 ('avg'|'max'|'min'|'p95'|'p99'); anything
-- else short-circuits to NULL and the evaluator treats the row as missing.
-- We group by asset_id so each asset is judged against the threshold
-- independently — one rule, one metric, many assets.
SELECT
    asset_id,
    CASE $3::text
        WHEN 'avg' THEN avg(value)
        WHEN 'max' THEN max(value)
        WHEN 'min' THEN min(value)
        WHEN 'p95' THEN percentile_cont(0.95) WITHIN GROUP (ORDER BY value)
        WHEN 'p99' THEN percentile_cont(0.99) WITHIN GROUP (ORDER BY value)
        ELSE NULL
    END AS aggregated_value,
    count(*) AS sample_count
FROM metrics
WHERE tenant_id = $1
  AND name = $2
  AND time > $4
GROUP BY asset_id;
