-- D2-P2 / D10-P1 (review-2026-04-21-v2): extend asset_dependencies
-- with a validity interval so topology queries can be time-filtered.
--
-- Why: the review specifically called out that relationships have no
-- temporal semantics — "asset_dependencies 无 valid_from / valid_to".
-- Combined with 000056 (asset_snapshots), this commit closes the
-- historical-query story for the entire relationship graph.
--
-- valid_from: inclusive lower bound. Backfills to created_at for all
--   existing rows so point-in-time queries at created_at <= t return
--   them.
-- valid_to: exclusive upper bound. NULL means "still in effect" — the
--   normal case. Setting valid_to soft-closes an edge; deletions go
--   through that path so the topology history stays queryable.
--
-- The old UNIQUE(source_asset_id, target_asset_id, dependency_type)
-- constraint prevented re-creating an edge that was previously deleted,
-- which breaks soft-closure. Replace it with a partial unique index
-- scoped to open edges (valid_to IS NULL) so:
--   1. You cannot have two simultaneously-open edges of the same shape
--   2. A closed edge does not block re-creating the same relationship
--   3. Historical edges accumulate instead of being overwritten

BEGIN;

-- 1. Add the interval columns. DEFAULT now() on valid_from makes the
--    migration re-entrant for any row inserted while it runs.
ALTER TABLE asset_dependencies
    ADD COLUMN IF NOT EXISTS valid_from TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS valid_to   TIMESTAMPTZ NULL;

-- 2. Backfill: an existing edge has been "in effect" since its
--    created_at. Without this, a query at "one day ago" would return
--    an empty graph even though edges existed then.
UPDATE asset_dependencies
SET valid_from = created_at
WHERE valid_from > created_at;

-- 3. Swap the hard UNIQUE for a partial unique on open edges. Two
--    identical edges with disjoint validity intervals are legal (it's
--    the whole point of a history table), but two simultaneously-open
--    ones are not.
ALTER TABLE asset_dependencies
    DROP CONSTRAINT IF EXISTS asset_dependencies_source_asset_id_target_asset_id_dependen_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_asset_deps_open
    ON asset_dependencies (source_asset_id, target_asset_id, dependency_type)
    WHERE valid_to IS NULL;

-- 4. Lookup index for the "at this time" query shape:
--    WHERE tenant_id=? AND valid_from<=? AND (valid_to IS NULL OR valid_to>?)
CREATE INDEX IF NOT EXISTS idx_asset_deps_validity
    ON asset_dependencies (tenant_id, valid_from, valid_to);

COMMIT;
