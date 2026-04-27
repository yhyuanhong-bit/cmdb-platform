-- ---------------------------------------------------------------------------
-- Tariffs.
-- ---------------------------------------------------------------------------

-- name: ListEnergyTariffs :many
SELECT * FROM energy_tariffs
WHERE tenant_id = $1
ORDER BY effective_from DESC, location_id NULLS FIRST;

-- name: ListEnergyTariffsForLocation :many
-- All tariffs (current + historic) for a specific location, plus tenant
-- default (location_id IS NULL) so the resolver can pick the right one.
SELECT * FROM energy_tariffs
WHERE tenant_id = $1
  AND (location_id = sqlc.narg('location_id') OR location_id IS NULL)
ORDER BY effective_from DESC;

-- name: GetEnergyTariff :one
SELECT * FROM energy_tariffs WHERE id = $1 AND tenant_id = $2;

-- name: CreateEnergyTariff :one
INSERT INTO energy_tariffs (
    tenant_id, location_id, currency, rate_per_kwh,
    effective_from, effective_to, notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateEnergyTariff :one
UPDATE energy_tariffs SET
    currency       = COALESCE(sqlc.narg('currency'), currency),
    rate_per_kwh   = COALESCE(sqlc.narg('rate_per_kwh'), rate_per_kwh),
    effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
    effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to),
    notes          = COALESCE(sqlc.narg('notes'), notes)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: DeleteEnergyTariff :exec
DELETE FROM energy_tariffs WHERE id = $1 AND tenant_id = $2;

-- name: CountOverlappingTariffs :one
-- Used by the domain layer before INSERT/UPDATE to enforce no-overlap.
-- Two tariffs on the same (tenant, location) overlap when their date
-- ranges intersect. NULL effective_to means "open-ended", treated as
-- +infinity here. The exclude_id parameter lets UPDATE skip the row
-- being modified.
SELECT count(*) FROM energy_tariffs
WHERE tenant_id = $1
  AND (
        (location_id IS NULL AND sqlc.narg('location_id')::uuid IS NULL)
     OR (location_id = sqlc.narg('location_id'))
      )
  AND id <> COALESCE(sqlc.narg('exclude_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
  AND effective_from <= COALESCE(sqlc.narg('window_to')::date, DATE '9999-12-31')
  AND COALESCE(effective_to, DATE '9999-12-31') >= sqlc.arg('window_from');

-- name: ResolveTariffForDay :one
-- Pick the tariff that applies to a given (location, day). Falls back to
-- the tenant default (location_id IS NULL) if no per-location tariff
-- covers the day. Sorted so a location-specific row beats the default.
SELECT * FROM energy_tariffs
WHERE tenant_id = $1
  AND effective_from <= sqlc.arg('day')
  AND (effective_to IS NULL OR effective_to >= sqlc.arg('day'))
  AND (location_id = sqlc.narg('location_id') OR location_id IS NULL)
ORDER BY (location_id IS NOT NULL) DESC, effective_from DESC
LIMIT 1;

-- ---------------------------------------------------------------------------
-- Daily aggregates.
-- ---------------------------------------------------------------------------

-- name: AggregateAssetDayKwh :exec
-- Computes the rollup for ONE asset on ONE day from the metrics hypertable
-- and upserts the result. Idempotent: re-running for the same day
-- overwrites the row.
--
-- The integral approximation uses 1-hour TimescaleDB buckets so a missing
-- sample just contributes zero rather than projecting forward forever.
-- This is conservative; if you want to fill gaps, do it before calling.
INSERT INTO energy_daily_kwh (tenant_id, asset_id, day, kwh_total, kw_peak, kw_avg, sample_count)
SELECT
    sqlc.arg('tenant_id')::uuid AS tenant_id,
    sqlc.arg('asset_id')::uuid  AS asset_id,
    sqlc.arg('day')::date       AS day,
    -- kwh_total: sum of (avg_kw_per_hour * 1h) ≈ sum(avg_kw_per_hour)
    COALESCE(SUM(bucket.avg_kw), 0)::numeric(12,4) AS kwh_total,
    COALESCE(MAX(bucket.peak_kw), 0)::numeric(10,4) AS kw_peak,
    COALESCE(AVG(bucket.avg_kw), 0)::numeric(10,4)  AS kw_avg,
    COALESCE(SUM(bucket.n)::integer, 0)              AS sample_count
FROM (
    SELECT
        date_trunc('hour', m.time) AS hour,
        AVG(m.value) AS avg_kw,
        MAX(m.value) AS peak_kw,
        COUNT(*)     AS n
    FROM metrics m
    WHERE m.tenant_id = sqlc.arg('tenant_id')::uuid
      AND m.asset_id  = sqlc.arg('asset_id')::uuid
      AND m.name      = 'power_kw'
      AND m.time     >= sqlc.arg('day')::date
      AND m.time     <  (sqlc.arg('day')::date + INTERVAL '1 day')
    GROUP BY hour
) bucket
ON CONFLICT (tenant_id, asset_id, day) DO UPDATE SET
    kwh_total    = EXCLUDED.kwh_total,
    kw_peak      = EXCLUDED.kw_peak,
    kw_avg       = EXCLUDED.kw_avg,
    sample_count = EXCLUDED.sample_count,
    computed_at  = now();

-- name: ListDailyKwhForTenant :many
-- The "daily timeline" query. Used by the bill UI; restricted by date
-- range to keep the payload bounded.
SELECT d.tenant_id, d.asset_id, d.day, d.kwh_total, d.kw_peak, d.kw_avg, d.sample_count, d.computed_at,
       a.asset_tag, a.name AS asset_name, a.location_id
FROM energy_daily_kwh d
JOIN assets a ON a.id = d.asset_id
WHERE d.tenant_id = $1
  AND d.day >= sqlc.arg('day_from')
  AND d.day <= sqlc.arg('day_to')
ORDER BY d.day DESC, a.name;

-- name: ListDailyKwhForAsset :many
-- Per-asset history view, capped at the supplied window.
SELECT * FROM energy_daily_kwh
WHERE tenant_id = $1 AND asset_id = $2
  AND day >= sqlc.arg('day_from')
  AND day <= sqlc.arg('day_to')
ORDER BY day DESC;

-- name: SumDailyKwhForTenant :one
-- Used by the bill endpoint — total kWh + cost can be computed in two
-- queries (this for kWh, the tariff resolver for cost) without dragging
-- the per-row payload to Go.
SELECT
    COALESCE(SUM(kwh_total), 0)::numeric(14,4) AS total_kwh,
    COALESCE(SUM(kw_peak), 0)::numeric(12,4)   AS sum_peak_kw,
    COUNT(*)::int                              AS row_count
FROM energy_daily_kwh
WHERE tenant_id = $1
  AND day >= sqlc.arg('day_from')
  AND day <= sqlc.arg('day_to');

-- name: ListAssetsWithKwhInRange :many
-- The bill calculator iterates assets so it can apply each asset's
-- location-specific tariff. Returning (asset_id, location_id, kwh) per
-- asset over the window keeps that loop bounded by asset count, not
-- day-asset row count.
SELECT
    d.asset_id,
    a.location_id,
    SUM(d.kwh_total)::numeric(14,4) AS asset_kwh
FROM energy_daily_kwh d
JOIN assets a ON a.id = d.asset_id
WHERE d.tenant_id = $1
  AND d.day >= sqlc.arg('day_from')
  AND d.day <= sqlc.arg('day_to')
GROUP BY d.asset_id, a.location_id
ORDER BY asset_kwh DESC;
