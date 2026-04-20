-- asset_location_history: append-only audit of physical relocations
-- detected by the location_detect pipeline. tenant_id is a first-class
-- column. All queries here are tenant-scoped EXCEPT GetLocationHistory,
-- which historically scopes only by asset_id — that is preserved
-- verbatim and annotated cross-tenant: (the asset FK row is tenant-
-- scoped upstream, so callers that have already fetched the asset are
-- implicitly tenant-safe; adding a tenant filter here would be a
-- behavior change for no security gain).

-- name: RecordLocationChange :exec
-- Called by location_detect.Service on every detected asset move.
-- All nullable columns are passed through as pgtype.UUID so callers can
-- record an initial placement (from_rack_id NULL) or a relocation
-- without a work order (work_order_id NULL).
INSERT INTO asset_location_history (tenant_id, asset_id, from_rack_id, to_rack_id, detected_by, work_order_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLocationHistory :many
-- cross-tenant: scoped by asset_id only. Pre-migration parity.
SELECT
    h.id,
    h.from_rack_id,
    fr.name AS from_rack_name,
    h.to_rack_id,
    tr.name AS to_rack_name,
    h.detected_by,
    h.work_order_id,
    h.detected_at
FROM asset_location_history h
LEFT JOIN racks fr ON h.from_rack_id = fr.id
LEFT JOIN racks tr ON h.to_rack_id = tr.id
WHERE h.asset_id = $1
ORDER BY h.detected_at DESC
LIMIT $2;

-- name: CountRelocationsSince24h :one
-- Used by LocationDetectGetSummary (fixed 24h window).
SELECT count(*)
FROM asset_location_history
WHERE tenant_id = $1
  AND detected_at > now() - interval '24 hours';

-- name: CountRelocationsSince :one
-- Used by LocationDetectGetReport (variable window in days).
-- Intervals pass as an int day count rather than pgtype.Interval —
-- the convention established by DeleteOldWebhookDeliveries.
SELECT count(*)
FROM asset_location_history
WHERE tenant_id = $1
  AND detected_at > now() - ($2::int * interval '1 day');

-- name: CountAuthorizedRelocationsSince :one
-- Used by LocationDetectGetReport — count of relocations in the last
-- $2 days that also have a backing work_order_id.
SELECT count(*)
FROM asset_location_history
WHERE tenant_id = $1
  AND detected_at > now() - ($2::int * interval '1 day')
  AND work_order_id IS NOT NULL;

-- name: DetectFrequentRelocations :many
-- Assets that moved 3 or more times in 30 days. Used by the anomaly
-- detector.
SELECT h.asset_id, a.asset_tag, a.name, count(*)::bigint AS move_count
FROM asset_location_history h
JOIN assets a ON h.asset_id = a.id
WHERE h.tenant_id = $1 AND h.detected_at > now() - interval '30 days'
GROUP BY h.asset_id, a.asset_tag, a.name
HAVING count(*) >= 3
ORDER BY count(*) DESC;

-- name: DetectOffHoursRelocations :many
-- Relocations between 22:00 and 06:00 in the last 24h. The HOUR EXTRACT
-- uses server local time — preserving pre-migration behavior.
SELECT h.asset_id, a.asset_tag, a.name, h.detected_at
FROM asset_location_history h
JOIN assets a ON h.asset_id = a.id
WHERE h.tenant_id = $1
  AND h.detected_at > now() - interval '24 hours'
  AND (EXTRACT(HOUR FROM h.detected_at) >= 22 OR EXTRACT(HOUR FROM h.detected_at) < 6)
ORDER BY h.detected_at DESC;

-- name: DetectBulkDisappearance :many
-- Racks where 3 or more devices left in the last hour.
SELECT h.from_rack_id, r.name, count(*)::bigint AS device_count
FROM asset_location_history h
JOIN racks r ON h.from_rack_id = r.id
WHERE h.tenant_id = $1
  AND h.detected_at > now() - interval '1 hour'
  AND h.from_rack_id IS NOT NULL
GROUP BY h.from_rack_id, r.name
HAVING count(*) >= 3;
