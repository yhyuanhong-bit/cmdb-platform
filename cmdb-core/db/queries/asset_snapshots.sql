-- name: GetAssetStateAt :one
-- D10-P0: point-in-time query. Returns the most-recent snapshot whose
-- valid_at is <= the caller's target time, scoped to the asset and
-- tenant. The (asset_id, valid_at DESC) index makes this a single
-- partition+index lookup regardless of how many snapshots exist.
-- Returns sql.ErrNoRows when the asset has no snapshot at or before
-- the query time — the service layer maps this to a 404.
SELECT *
FROM asset_snapshots
WHERE asset_id = $1
  AND tenant_id = $2
  AND valid_at <= $3
ORDER BY valid_at DESC
LIMIT 1;

-- name: ListAssetSnapshots :many
-- All snapshots for an asset, newest first. Feeds the "asset history"
-- UI and lets an operator walk every state change without needing to
-- replay the audit diff chain. Bounded by $3 because a heavily-edited
-- asset can produce thousands of rows; the default call site passes
-- 100.
SELECT *
FROM asset_snapshots
WHERE asset_id = $1
  AND tenant_id = $2
ORDER BY valid_at DESC
LIMIT $3;

-- name: CountAssetSnapshots :one
-- Health-check query used by the admin diagnostics endpoint to confirm
-- the snapshot trigger is firing. A stuck trigger shows up as a count
-- that stops growing relative to audit_events.
SELECT COUNT(*) FROM asset_snapshots
WHERE tenant_id = $1;
