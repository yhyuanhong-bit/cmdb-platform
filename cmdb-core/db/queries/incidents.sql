-- name: ListIncidents :many
SELECT * FROM incidents
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'))
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: CountIncidents :one
SELECT count(*) FROM incidents
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'));

-- name: GetIncident :one
SELECT * FROM incidents WHERE id = $1 AND tenant_id = $2;

-- name: CreateIncident :one
INSERT INTO incidents (tenant_id, title, status, severity, started_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateIncident :one
UPDATE incidents SET
    title       = COALESCE(sqlc.narg('title'), title),
    status      = COALESCE(sqlc.narg('status'), status),
    severity    = COALESCE(sqlc.narg('severity'), severity),
    resolved_at = COALESCE(sqlc.narg('resolved_at'), resolved_at)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListAlertEventsByIncident :many
-- Returns alert_events considered "related" to an incident for RCA context.
-- There is no alert_events.incident_id FK, so the link is derived by:
--   1. Same tenant (defence-in-depth; tenant_id is passed explicitly).
--   2. The alert fired during the incident's active window: between the
--      incident's started_at and COALESCE(resolved_at, now()).
-- This is the most conservative temporal association we can make without a
-- dedicated link table. Results are ordered newest-first and capped at 100
-- rows to bound the payload handed to the AI provider.
SELECT ae.id, ae.tenant_id, ae.rule_id, ae.asset_id, ae.status,
       ae.severity, ae.message, ae.trigger_value, ae.fired_at,
       ae.acked_at, ae.resolved_at, ae.sync_version
FROM alert_events ae
JOIN incidents i
  ON i.id = @incident_id::uuid
 AND i.tenant_id = @tenant_id::uuid
WHERE ae.tenant_id = @tenant_id::uuid
  AND ae.fired_at >= i.started_at
  AND ae.fired_at <= COALESCE(i.resolved_at, now())
ORDER BY ae.fired_at DESC
LIMIT 100;

-- name: ListAssetsForIncident :many
-- Returns DISTINCT assets referenced by alert_events related to the incident
-- (same temporal window as ListAlertEventsByIncident). Soft-deleted assets
-- are excluded. Tenant scope is enforced on BOTH alert_events and assets
-- and on the incidents row (defence-in-depth).
--
-- Explicit column list mirrors dbgen.Asset so sqlc returns []Asset rather
-- than a bespoke row type. When the assets table gains new columns (e.g.
-- 000058 added access_count_24h and last_accessed_at; 000060 added
-- owner_team), add them here too or the row shape diverges and callers
-- break to compile.
SELECT DISTINCT a.id, a.tenant_id, a.asset_tag, a.property_number,
       a.control_number, a.name, a.type, a.sub_type, a.status, a.bia_level,
       a.location_id, a.rack_id, a.vendor, a.model, a.serial_number,
       a.attributes, a.tags, a.created_at, a.updated_at, a.ip_address,
       a.deleted_at, a.sync_version, a.bmc_ip, a.bmc_type, a.bmc_firmware,
       a.purchase_date, a.purchase_cost, a.warranty_start, a.warranty_end,
       a.warranty_vendor, a.warranty_contract, a.expected_lifespan_months,
       a.eol_date, a.access_count_24h, a.last_accessed_at, a.owner_team
FROM assets a
JOIN alert_events ae ON ae.asset_id = a.id
JOIN incidents i
  ON i.id = @incident_id::uuid
 AND i.tenant_id = @tenant_id::uuid
WHERE a.tenant_id = @tenant_id::uuid
  AND ae.tenant_id = @tenant_id::uuid
  AND a.deleted_at IS NULL
  AND ae.fired_at >= i.started_at
  AND ae.fired_at <= COALESCE(i.resolved_at, now())
ORDER BY a.name
LIMIT 100;
