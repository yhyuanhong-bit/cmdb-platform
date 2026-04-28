-- name: UpsertPredictiveRefresh :exec
-- Idempotent insert. Re-running the rule engine overwrites score / reason
-- / target_date / detected_at but PRESERVES status / reviewed_by /
-- reviewed_at / note — operator's ack must survive a re-run.
INSERT INTO predictive_refresh_recommendations (
    tenant_id, asset_id, kind, risk_score, reason, recommended_action, target_date
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id, asset_id, kind) DO UPDATE SET
    risk_score         = EXCLUDED.risk_score,
    reason             = EXCLUDED.reason,
    recommended_action = EXCLUDED.recommended_action,
    target_date        = EXCLUDED.target_date,
    detected_at        = now()
;

-- name: ListPredictiveRefresh :many
-- Joins asset basics so the UI can render names + tags without a second
-- roundtrip. Sorted by score DESC so the most urgent rows surface first;
-- ties broken by target_date ASC (sooner deadline first).
SELECT
    r.tenant_id, r.asset_id, r.kind, r.risk_score, r.reason,
    r.recommended_action, r.target_date, r.status, r.detected_at,
    r.reviewed_by, r.reviewed_at, r.note,
    a.asset_tag, a.name AS asset_name, a.location_id, a.type AS asset_type,
    a.purchase_date, a.warranty_end, a.eol_date
FROM predictive_refresh_recommendations r
JOIN assets a ON a.id = r.asset_id AND a.tenant_id = r.tenant_id AND a.deleted_at IS NULL
WHERE r.tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR r.status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::varchar   IS NULL OR r.kind   = sqlc.narg('kind'))
ORDER BY r.risk_score DESC, r.target_date ASC NULLS LAST, r.detected_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPredictiveRefresh :one
SELECT count(*) FROM predictive_refresh_recommendations r
JOIN assets a ON a.id = r.asset_id AND a.tenant_id = r.tenant_id AND a.deleted_at IS NULL
WHERE r.tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR r.status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::varchar   IS NULL OR r.kind   = sqlc.narg('kind'));

-- name: TransitionPredictiveRefresh :one
UPDATE predictive_refresh_recommendations SET
    status      = sqlc.arg('status'),
    reviewed_by = sqlc.arg('reviewed_by'),
    reviewed_at = now(),
    note        = COALESCE(sqlc.narg('note'), note)
WHERE tenant_id = sqlc.arg('tenant_id')
  AND asset_id  = sqlc.arg('asset_id')
  AND kind      = sqlc.arg('kind')
RETURNING *;

-- name: DeleteStalePredictiveRefresh :exec
-- The rule engine's compaction step. After ScanAndUpsert writes the
-- current set of (asset, kind) rows, anything still flagged 'open' that
-- wasn't touched in this run is no longer applicable (warranty got
-- renewed, asset retired, EOL date pushed out). Sweep those rows so the
-- UI doesn't show stale recommendations the operator can't act on.
--
-- Acked / resolved rows are kept for audit even when they no longer
-- match — the operator already made a decision and the trail matters.
DELETE FROM predictive_refresh_recommendations
WHERE tenant_id = sqlc.arg('tenant_id')
  AND status = 'open'
  AND detected_at < sqlc.arg('cutoff');

-- name: ListAssetsForPredictiveScan :many
-- The rule engine iterates assets that have at least one of the
-- predictive inputs populated. Returning the full asset row keeps the
-- engine in Go (rules express better there than in SQL) and avoids
-- a second per-asset roundtrip.
SELECT
    a.id, a.tenant_id, a.asset_tag, a.name, a.type,
    a.purchase_date, a.warranty_end, a.eol_date, a.expected_lifespan_months
FROM assets a
WHERE a.tenant_id = $1
  AND a.deleted_at IS NULL
  AND (a.purchase_date IS NOT NULL
    OR a.warranty_end  IS NOT NULL
    OR a.eol_date      IS NOT NULL);

-- name: ListTenantsWithLifecycleAssets :many
-- Scheduler enumerates tenants that have at least one asset with a
-- lifecycle field set. Empty tenants are skipped.
SELECT DISTINCT a.tenant_id
FROM assets a
WHERE a.deleted_at IS NULL
  AND (a.purchase_date IS NOT NULL
    OR a.warranty_end  IS NOT NULL
    OR a.eol_date      IS NOT NULL);

-- name: AggregatePredictiveRefreshByMonth :many
-- Capex backlog roll-up. Groups open recommendations by the month of
-- target_date, returning total count + per-kind counts so the dashboard
-- can render the stacked bar chart server-side instead of paging the
-- whole list and bucketing in JS. Rows with NULL target_date are
-- excluded — they have no month to bucket into and aren't actionable
-- for capex planning anyway.
--
-- Range parameters are optional (NULL = no bound). When set they're
-- compared against the *month boundary* (date_trunc('month', ...)) so
-- callers can pass a YYYY-MM-01 anchor and reason about inclusive
-- months without fencepost math.
SELECT
    date_trunc('month', r.target_date)::date AS month,
    count(*)::bigint                                     AS count,
    count(*) FILTER (WHERE r.kind = 'warranty_expiring')::bigint AS warranty_expiring,
    count(*) FILTER (WHERE r.kind = 'warranty_expired')::bigint  AS warranty_expired,
    count(*) FILTER (WHERE r.kind = 'eol_approaching')::bigint   AS eol_approaching,
    count(*) FILTER (WHERE r.kind = 'eol_passed')::bigint        AS eol_passed,
    count(*) FILTER (WHERE r.kind = 'aged_out')::bigint          AS aged_out
FROM predictive_refresh_recommendations r
JOIN assets a ON a.id = r.asset_id AND a.tenant_id = r.tenant_id AND a.deleted_at IS NULL
WHERE r.tenant_id = $1
  AND r.status = 'open'
  AND r.target_date IS NOT NULL
  AND (sqlc.narg('from_month')::date IS NULL OR date_trunc('month', r.target_date) >= date_trunc('month', sqlc.narg('from_month')::date))
  AND (sqlc.narg('to_month')::date   IS NULL OR date_trunc('month', r.target_date) <= date_trunc('month', sqlc.narg('to_month')::date))
GROUP BY date_trunc('month', r.target_date)
ORDER BY date_trunc('month', r.target_date) ASC;
