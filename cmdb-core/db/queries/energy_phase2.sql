-- ---------------------------------------------------------------------------
-- Per-location daily PUE rollup.
-- ---------------------------------------------------------------------------

-- name: AggregateLocationDayPue :exec
-- Walks energy_daily_kwh for the given (tenant, day), splits assets into
-- IT vs non-IT by asset.type, and upserts one energy_location_daily row
-- per location. Idempotent — re-runs overwrite.
INSERT INTO energy_location_daily (
    tenant_id, location_id, day,
    it_kwh, non_it_kwh, total_kwh,
    it_asset_count, non_it_asset_count, computed_at
)
SELECT
    sqlc.arg('tenant_id')::uuid AS tenant_id,
    a.location_id              AS location_id,
    sqlc.arg('day')::date      AS day,
    COALESCE(SUM(d.kwh_total) FILTER (WHERE a.type IN ('server','network','storage')), 0)::numeric(14,4) AS it_kwh,
    COALESCE(SUM(d.kwh_total) FILTER (WHERE a.type NOT IN ('server','network','storage')), 0)::numeric(14,4) AS non_it_kwh,
    COALESCE(SUM(d.kwh_total), 0)::numeric(14,4) AS total_kwh,
    COUNT(DISTINCT a.id) FILTER (WHERE a.type IN ('server','network','storage'))::int AS it_asset_count,
    COUNT(DISTINCT a.id) FILTER (WHERE a.type NOT IN ('server','network','storage'))::int AS non_it_asset_count,
    now()
FROM energy_daily_kwh d
JOIN assets a ON a.id = d.asset_id AND a.tenant_id = d.tenant_id AND a.deleted_at IS NULL
WHERE d.tenant_id = sqlc.arg('tenant_id')::uuid
  AND d.day       = sqlc.arg('day')::date
  AND a.location_id IS NOT NULL
GROUP BY a.location_id
ON CONFLICT (tenant_id, location_id, day) DO UPDATE SET
    it_kwh             = EXCLUDED.it_kwh,
    non_it_kwh         = EXCLUDED.non_it_kwh,
    total_kwh          = EXCLUDED.total_kwh,
    it_asset_count     = EXCLUDED.it_asset_count,
    non_it_asset_count = EXCLUDED.non_it_asset_count,
    computed_at        = now();

-- name: ListLocationDailyPue :many
-- The PUE history view query. PUE is computed on read so the stored row
-- never disagrees with the inputs (a cached PUE could lag a recomputed
-- daily kWh). NULLIF guards against divide-by-zero when a location has
-- zero IT kWh — an empty PUE column is a more honest answer than
-- displaying ∞.
SELECT
    l.tenant_id,
    l.location_id,
    loc.name AS location_name,
    loc.level AS location_level,
    l.day,
    l.it_kwh,
    l.non_it_kwh,
    l.total_kwh,
    l.it_asset_count,
    l.non_it_asset_count,
    l.computed_at,
    CASE
        WHEN l.it_kwh > 0 THEN ROUND(l.total_kwh / l.it_kwh, 4)
        ELSE NULL
    END::numeric(8,4) AS pue
FROM energy_location_daily l
JOIN locations loc ON loc.id = l.location_id
WHERE l.tenant_id = $1
  AND l.day >= sqlc.arg('day_from')
  AND l.day <= sqlc.arg('day_to')
  AND (sqlc.narg('location_id')::uuid IS NULL OR l.location_id = sqlc.narg('location_id'))
ORDER BY l.day DESC, loc.name;

-- ---------------------------------------------------------------------------
-- Anomaly detection.
-- ---------------------------------------------------------------------------

-- name: ComputeAssetBaselineMedian :one
-- Returns the median daily kWh over the trailing N days *excluding*
-- the day being scored. Postgres has no MEDIAN aggregate built-in so
-- we use the percentile_cont(0.5) form, which is documented and
-- well-supported.
SELECT
    COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY kwh_total), 0)::numeric(12,4) AS median_kwh,
    COUNT(*)::int AS sample_count
FROM energy_daily_kwh
WHERE tenant_id = sqlc.arg('tenant_id')
  AND asset_id  = sqlc.arg('asset_id')
  AND day      <  sqlc.arg('day')
  AND day      >= sqlc.arg('day') - (sqlc.arg('window_days')::int || ' days')::interval;

-- name: UpsertEnergyAnomaly :exec
-- Idempotent insert. Re-running the detector overwrites the score
-- columns but preserves status + reviewed_by — an operator's ack must
-- survive a re-run.
INSERT INTO energy_anomalies (
    tenant_id, asset_id, day, kind,
    baseline_median, observed_kwh, score
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id, asset_id, day) DO UPDATE SET
    kind            = EXCLUDED.kind,
    baseline_median = EXCLUDED.baseline_median,
    observed_kwh    = EXCLUDED.observed_kwh,
    score           = EXCLUDED.score,
    detected_at     = now()
    -- status / reviewed_by / reviewed_at / note intentionally NOT
    -- touched — we only update the inputs.
;

-- name: ListEnergyAnomalies :many
SELECT
    a.tenant_id, a.asset_id, a.day, a.kind, a.baseline_median, a.observed_kwh,
    a.score, a.status, a.detected_at, a.reviewed_by, a.reviewed_at, a.note,
    asset.asset_tag, asset.name AS asset_name, asset.location_id
FROM energy_anomalies a
JOIN assets asset ON asset.id = a.asset_id
WHERE a.tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR a.status = sqlc.narg('status'))
  AND a.day >= sqlc.arg('day_from')
  AND a.day <= sqlc.arg('day_to')
ORDER BY a.detected_at DESC, a.score DESC
LIMIT $2 OFFSET $3;

-- name: CountEnergyAnomalies :one
SELECT count(*) FROM energy_anomalies
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND day >= sqlc.arg('day_from')
  AND day <= sqlc.arg('day_to');

-- name: TransitionEnergyAnomaly :one
-- Operator-driven status transitions. Only allows open→ack, open→resolved,
-- ack→resolved, ack→open (un-ack), resolved→open (re-open). The CHECK on
-- the table covers the value space; this query lets the domain layer
-- pin the source state.
UPDATE energy_anomalies SET
    status      = sqlc.arg('status'),
    reviewed_by = sqlc.arg('reviewed_by'),
    reviewed_at = now(),
    note        = COALESCE(sqlc.narg('note'), note)
WHERE tenant_id = sqlc.arg('tenant_id')
  AND asset_id  = sqlc.arg('asset_id')
  AND day       = sqlc.arg('day')
RETURNING *;

-- name: ListAssetsWithDayKwh :many
-- Used by the detector: every asset that has a daily row on the target
-- day, with its observed kWh.
SELECT d.asset_id, d.kwh_total
FROM energy_daily_kwh d
WHERE d.tenant_id = sqlc.arg('tenant_id')
  AND d.day       = sqlc.arg('day');

-- name: ListTenantsWithRecentMetrics :many
-- Scheduler picks this list each tick: tenants that have power_kw
-- samples in the last 48h. Empty tenants are skipped (no work).
SELECT DISTINCT m.tenant_id
FROM metrics m
WHERE m.name = 'power_kw'
  AND m.time >= now() - interval '48 hours';
