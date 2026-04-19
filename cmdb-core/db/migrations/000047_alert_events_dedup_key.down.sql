-- Down migration for 000046: drop the dedup index and columns.
DROP INDEX IF EXISTS alert_events_dedup_unique;
ALTER TABLE alert_events DROP COLUMN IF EXISTS dedup_key;
ALTER TABLE alert_events DROP COLUMN IF EXISTS updated_at;
