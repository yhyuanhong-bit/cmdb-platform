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
