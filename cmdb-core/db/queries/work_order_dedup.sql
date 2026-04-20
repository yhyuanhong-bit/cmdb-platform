-- Phase 2.15: work_order_dedup queries backing the auto-WO dedup
-- contract that replaces the old `description LIKE` hack.

-- name: CheckWorkOrderDedup :one
-- Probe: does a dedup entry already exist for (tenant, kind, key)?
-- Used in the per-tenant scan to skip a candidate BEFORE starting a
-- transaction. The authoritative uniqueness check is the ON CONFLICT
-- on InsertWorkOrderDedup — this probe is only a fast pre-filter.
SELECT EXISTS (
    SELECT 1 FROM work_order_dedup
    WHERE tenant_id  = $1
      AND dedup_kind = $2
      AND dedup_key  = $3
) AS exists;

-- name: InsertWorkOrderDedup :execrows
-- Race-safe insert. Returns 0 rows if (tenant, kind, key) already exists
-- — the caller rolls back the surrounding transaction so the orphan WO
-- is never persisted. ON CONFLICT DO NOTHING means a lost race just
-- produces a no-op, never an error.
INSERT INTO work_order_dedup (
    tenant_id, work_order_id, dedup_kind, dedup_key
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (tenant_id, dedup_kind, dedup_key) DO NOTHING;
