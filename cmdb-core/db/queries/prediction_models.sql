-- name: ListEnabledModels :many
SELECT * FROM prediction_models
WHERE enabled = true
ORDER BY name;

-- name: ListAllModels :many
SELECT * FROM prediction_models
ORDER BY name;

-- name: GetModel :one
SELECT * FROM prediction_models WHERE id = $1;

-- name: CreateModel :one
INSERT INTO prediction_models (
    name, type, provider, config, enabled
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;
