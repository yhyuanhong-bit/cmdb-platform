-- rack_network_connections: per-rack network port/connection records.
-- Tenant isolation: rack_network_connections carries its own tenant_id
-- column and the parent racks table also carries tenant_id. Every
-- query below joins (or filters) on both rack_id + racks.tenant_id so
-- a forged rack_id cannot reach another tenant's rows.

-- name: ListRackNetworkConnections :many
-- The JOIN on racks.tenant_id is what actually enforces tenancy —
-- rnc.tenant_id alone is not load-bearing here, but the join filter
-- on the rack preserves the "Fix #11" invariant from the pre-sqlc
-- code verbatim.
SELECT
    rnc.id,
    rnc.source_port,
    COALESCE(a.name, rnc.external_device, '') AS device,
    rnc.connected_asset_id,
    COALESCE(rnc.external_device, '')         AS external_device,
    COALESCE(rnc.speed, '')                   AS speed,
    COALESCE(rnc.status, '')                  AS status,
    COALESCE(rnc.vlans, '{}')::int[]          AS vlans,
    COALESCE(rnc.connection_type, '')         AS connection_type
FROM rack_network_connections rnc
JOIN racks r ON rnc.rack_id = r.id AND r.tenant_id = $2 AND r.deleted_at IS NULL
LEFT JOIN assets a ON rnc.connected_asset_id = a.id
WHERE rnc.rack_id = $1
ORDER BY rnc.source_port;

-- name: CreateRackNetworkConnection :exec
-- Insert assumes the handler has already validated tenant ownership of
-- rack_id. tenant_id is persisted for index locality and FK coherence.
INSERT INTO rack_network_connections
    (id, rack_id, tenant_id, source_port, connected_asset_id, external_device,
     speed, status, vlans, connection_type)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: DeleteRackNetworkConnection :execrows
-- "Fix #10" invariant preserved: the connection is only deletable if
-- its rack_id belongs to a rack owned by the caller's tenant.
DELETE FROM rack_network_connections rnc
WHERE rnc.id = $1
  AND rnc.rack_id = $2
  AND rnc.rack_id IN (SELECT r.id FROM racks r WHERE r.tenant_id = $3 AND r.deleted_at IS NULL);
