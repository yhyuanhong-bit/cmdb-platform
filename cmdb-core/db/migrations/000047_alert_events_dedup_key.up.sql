-- Migration 000046: add dedup_key to alert_events for evaluator idempotency.
--
-- Phase 2.1 (REMEDIATION-ROADMAP.md): the in-process alert evaluator scans
-- alert_rules every 60s and emits alert_events rows on threshold breach.
-- Without a dedup key the same rule+asset+hour would insert N rows per tick
-- (60 per hour). We define dedup_key = "<rule_id>:<asset_id>:<YYYY-MM-DDTHH>"
-- and enforce uniqueness via idx alert_events_dedup_unique. The evaluator
-- INSERTs with ON CONFLICT (dedup_key) DO UPDATE SET trigger_value = EXCLUDED.trigger_value,
-- updated_at = now(), collapsing repeated firings within the same hour into
-- one row whose trigger_value reflects the latest tick.
--
-- We also introduce updated_at so repeated-tick updates carry a monotonic
-- timestamp distinct from fired_at (which stays pinned to first firing).
-- Older rows are backfilled so the NOT NULL constraint + unique index can
-- both be created atomically.

ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS dedup_key TEXT;
ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;

-- Backfill dedup_key for existing rows. Rows with a NULL asset_id use the
-- literal 'none' placeholder so the grouping is still valid. Rows with a
-- NULL rule_id should not exist in practice (the evaluator always attaches
-- a rule), but to be safe we fall back to 'none' for that as well.
UPDATE alert_events
   SET dedup_key = COALESCE(rule_id::text, 'none') || ':'
                || COALESCE(asset_id::text, 'none') || ':'
                || to_char(fired_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24')
 WHERE dedup_key IS NULL;

UPDATE alert_events SET updated_at = fired_at WHERE updated_at IS NULL;

ALTER TABLE alert_events ALTER COLUMN dedup_key SET NOT NULL;
ALTER TABLE alert_events ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE alert_events ALTER COLUMN updated_at SET DEFAULT now();

CREATE UNIQUE INDEX IF NOT EXISTS alert_events_dedup_unique
    ON alert_events (dedup_key);
