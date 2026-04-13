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
