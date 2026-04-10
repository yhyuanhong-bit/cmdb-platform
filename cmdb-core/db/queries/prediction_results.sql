-- name: ListPredictionsByAsset :many
SELECT * FROM prediction_results
WHERE asset_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListPredictionsByTenant :many
SELECT * FROM prediction_results
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPredictionsByTenant :one
SELECT count(*) FROM prediction_results
WHERE tenant_id = $1;

-- name: CreatePredictionResult :one
INSERT INTO prediction_results (
    tenant_id, model_id, asset_id, prediction_type,
    result, severity, recommended_action, expires_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8
) RETURNING *;

-- name: CreateRCA :one
INSERT INTO rca_analyses (
    tenant_id, incident_id, model_id, reasoning,
    conclusion_asset_id, confidence
) VALUES (
    $1, $2, $3, $4,
    $5, $6
) RETURNING *;

-- name: GetRCA :one
SELECT * FROM rca_analyses WHERE id = $1 AND tenant_id = $2;

-- name: VerifyRCA :one
UPDATE rca_analyses SET
    human_verified = true,
    verified_by = $2
WHERE id = $1
RETURNING *;
