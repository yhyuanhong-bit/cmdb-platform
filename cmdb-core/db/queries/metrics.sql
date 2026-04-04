-- name: QueryMetricsByAsset :many
SELECT time, name, value
FROM metrics
WHERE asset_id = $1
  AND name = $2
  AND time > $3
ORDER BY time DESC
LIMIT 500;
