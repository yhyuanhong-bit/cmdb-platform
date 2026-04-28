-- name: ListBIAAssessments :many
SELECT * FROM bia_assessments
WHERE tenant_id = $1
ORDER BY bia_score DESC
LIMIT $2 OFFSET $3;

-- name: CountBIAAssessments :one
SELECT count(*) FROM bia_assessments WHERE tenant_id = $1;

-- name: GetBIAAssessment :one
SELECT * FROM bia_assessments WHERE id = $1 AND tenant_id = $2;

-- name: CreateBIAAssessment :one
INSERT INTO bia_assessments (
    tenant_id, system_name, system_code, owner, bia_score, tier,
    rto_hours, rpo_minutes, mtpd_hours,
    data_compliance, asset_compliance, audit_compliance,
    description, assessed_by
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
RETURNING *;

-- name: UpdateBIAAssessment :one
UPDATE bia_assessments SET
    system_name      = COALESCE(sqlc.narg('system_name'), system_name),
    owner            = COALESCE(sqlc.narg('owner'), owner),
    bia_score        = COALESCE(sqlc.narg('bia_score'), bia_score),
    tier             = COALESCE(sqlc.narg('tier'), tier),
    rto_hours        = COALESCE(sqlc.narg('rto_hours'), rto_hours),
    rpo_minutes      = COALESCE(sqlc.narg('rpo_minutes'), rpo_minutes),
    mtpd_hours       = COALESCE(sqlc.narg('mtpd_hours'), mtpd_hours),
    data_compliance  = COALESCE(sqlc.narg('data_compliance'), data_compliance),
    asset_compliance = COALESCE(sqlc.narg('asset_compliance'), asset_compliance),
    audit_compliance = COALESCE(sqlc.narg('audit_compliance'), audit_compliance),
    description      = COALESCE(sqlc.narg('description'), description),
    last_assessed    = now(),
    updated_at       = now()
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: DeleteBIAAssessment :exec
DELETE FROM bia_assessments WHERE id = $1 AND tenant_id = $2;

-- name: ListBIAScoringRules :many
SELECT * FROM bia_scoring_rules
WHERE tenant_id = $1
ORDER BY tier_level;

-- name: CreateBIAScoringRule :one
INSERT INTO bia_scoring_rules (tenant_id, tier_name, tier_level, display_name, min_score, max_score, rto_threshold, rpo_threshold, description, color, icon)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING *;

-- name: UpdateBIAScoringRule :one
UPDATE bia_scoring_rules SET
    display_name   = COALESCE(sqlc.narg('display_name'), display_name),
    min_score      = COALESCE(sqlc.narg('min_score'), min_score),
    max_score      = COALESCE(sqlc.narg('max_score'), max_score),
    rto_threshold  = COALESCE(sqlc.narg('rto_threshold'), rto_threshold),
    rpo_threshold  = COALESCE(sqlc.narg('rpo_threshold'), rpo_threshold),
    description    = COALESCE(sqlc.narg('description'), description),
    color          = COALESCE(sqlc.narg('color'), color)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: ListBIADependencies :many
SELECT * FROM bia_dependencies
WHERE assessment_id = $1 AND tenant_id = $2;

-- name: CreateBIADependency :one
INSERT INTO bia_dependencies (tenant_id, assessment_id, asset_id, dependency_type, criticality)
VALUES ($1,$2,$3,$4,$5)
RETURNING *;

-- name: DeleteBIADependency :exec
DELETE FROM bia_dependencies WHERE id = $1 AND tenant_id = $2;

-- name: CountBIAByTier :many
SELECT tier, count(*) as count FROM bia_assessments
WHERE tenant_id = $1
GROUP BY tier;

-- name: GetBIAComplianceStats :one
SELECT
    count(*) as total,
    count(*) FILTER (WHERE data_compliance = true) as data_compliant,
    count(*) FILTER (WHERE asset_compliance = true) as asset_compliant,
    count(*) FILTER (WHERE audit_compliance = true) as audit_compliant
FROM bia_assessments
WHERE tenant_id = $1;

-- name: PropagateBIALevelByAssessment :exec
-- Tenant-scoped. Walks the BIA dependency graph starting from $1=assessment_id
-- (restricted to $2=tenant_id) and rewrites assets.bia_level to the
-- highest-priority tier connected to each asset via any in-tenant assessment.
-- Assets that share at least one dependency row with this assessment are
-- refreshed; orphaned-after-delete assets are handled separately by
-- RecomputeBIALevelForAsset.
UPDATE assets SET
    bia_level = sub.max_tier,
    updated_at = now()
FROM (
    SELECT bd.asset_id,
        CASE
            WHEN 'critical'  = ANY(array_agg(ba.tier)) THEN 'critical'
            WHEN 'important' = ANY(array_agg(ba.tier)) THEN 'important'
            WHEN 'normal'    = ANY(array_agg(ba.tier)) THEN 'normal'
            ELSE 'minor'
        END as max_tier
    FROM bia_dependencies bd
    JOIN bia_assessments ba ON ba.id = bd.assessment_id AND ba.tenant_id = $2
    WHERE bd.tenant_id = $2
      AND bd.asset_id IN (
          SELECT bd2.asset_id
          FROM bia_dependencies bd2
          WHERE bd2.assessment_id = $1
            AND bd2.tenant_id = $2
      )
    GROUP BY bd.asset_id
) sub
WHERE assets.id = sub.asset_id
  AND assets.tenant_id = $2;

-- name: RecomputeBIALevelForAsset :exec
-- Tenant-scoped. Recomputes a single asset's bia_level from its remaining
-- BIA dependencies. If the asset has no remaining dependencies (common after
-- a DELETE), bia_level falls back to 'normal' (the schema default).
UPDATE assets SET
    bia_level = COALESCE(sub.max_tier, 'normal'),
    updated_at = now()
FROM (
    SELECT sqlc.arg('asset_id')::uuid as asset_id,
        CASE
            WHEN 'critical'  = ANY(array_agg(ba.tier)) THEN 'critical'
            WHEN 'important' = ANY(array_agg(ba.tier)) THEN 'important'
            WHEN 'normal'    = ANY(array_agg(ba.tier)) THEN 'normal'
            WHEN array_length(array_agg(ba.tier), 1) IS NOT NULL THEN 'minor'
            ELSE NULL
        END as max_tier
    FROM bia_dependencies bd
    JOIN bia_assessments ba ON ba.id = bd.assessment_id AND ba.tenant_id = sqlc.arg('tenant_id')
    WHERE bd.asset_id = sqlc.arg('asset_id')
      AND bd.tenant_id = sqlc.arg('tenant_id')
) sub
WHERE assets.id = sub.asset_id
  AND assets.tenant_id = sqlc.arg('tenant_id');

-- name: GetBIADependency :one
-- Tenant-scoped lookup used before delete to enforce ownership and to
-- capture asset_id / assessment_id for post-delete propagation.
SELECT * FROM bia_dependencies WHERE id = $1 AND tenant_id = $2;

-- name: GetBIAAssessmentTenant :one
-- Minimal tenant-guard lookup: returns the tenant_id for an assessment id.
-- Used to verify the caller owns the assessment before mutating dependencies.
SELECT tenant_id FROM bia_assessments WHERE id = $1;

-- name: GetImpactedAssessments :many
SELECT ba.* FROM bia_assessments ba
JOIN bia_dependencies bd ON bd.assessment_id = ba.id
WHERE bd.asset_id = $1;
