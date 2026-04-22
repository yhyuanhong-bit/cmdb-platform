-- D9-P1 (review-2026-04-21-v2): store the access-weight at scan time
-- so the dashboard aggregation is a pure NUMERIC weighted average and
-- the value we reported yesterday can be audited next quarter.
--
-- Why store and not compute: dashboard reads are hot (Grafana-style
-- polling every 30s). Joining quality_scores → assets and computing
-- 1 + ln(1 + access_count_24h) on every call would put access_count_24h
-- on the hot query path — and that column is written on every GET/LIST,
-- i.e. the single most-updated field in the asset table. Weighted
-- averages over stable numeric columns are far cheaper.
--
-- Formula: access_weight = 1 + ln(1 + access_count_24h), capped at 10.
-- Intuition:
--   count=0   → 1.0  (cold asset, counted once)
--   count=10  → 3.4  (a handful of operators)
--   count=100 → 5.6  (busy)
--   count=1e4 → 10.0 (capped)
-- The log damps the tail so one super-hot asset can't dominate the avg.
--
-- Default 1.0 means existing rows (pre-migration) continue to count as
-- cold assets until the next scan pass rewrites them with the real
-- weight. The constraint keeps weights in a predictable range so a bad
-- insert can't silently shift the dashboard.

BEGIN;

ALTER TABLE quality_scores
    ADD COLUMN IF NOT EXISTS access_weight NUMERIC(6,3) NOT NULL DEFAULT 1.0;

ALTER TABLE quality_scores
    DROP CONSTRAINT IF EXISTS quality_scores_access_weight_range;
ALTER TABLE quality_scores
    ADD CONSTRAINT quality_scores_access_weight_range
    CHECK (access_weight >= 0 AND access_weight <= 10);

COMMIT;
