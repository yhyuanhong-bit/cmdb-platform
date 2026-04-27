-- Wave 5.4: Direct alert_events ↔ incidents linkage.
--
-- Pre-5.4 the relationship between alert_events and incidents was implicit
-- and temporal — see queries/incidents.sql ListAlertEventsByIncident, which
-- joined alerts to an incident's started_at..resolved_at window. That works
-- as a "what was happening around this incident" RCA helper but doesn't
-- support the inverse: "which incident does this firing alert belong to?".
-- Operators want a hard pointer so the timeline is unambiguous.
--
-- This migration adds a nullable FK from alert_events to incidents and a
-- partial index for the lookup the bridge logic does on every emit (find
-- an open incident on the same asset). The column is nullable because:
--   1. Backfilling all historic alerts to incidents would be guesswork.
--   2. Low-severity alerts should never spawn an incident.
--   3. Alerts without asset_id can't be tied to a specific affected_asset_id.

ALTER TABLE alert_events
    ADD COLUMN IF NOT EXISTS incident_id UUID REFERENCES incidents(id) ON DELETE SET NULL;

-- Partial index — most alert_events rows will never link to an incident,
-- so a full b-tree wastes space. The "WHERE incident_id IS NOT NULL"
-- predicate keeps the index small while still serving the reverse-lookup
-- (which alerts belong to incident X) in O(log n).
CREATE INDEX IF NOT EXISTS idx_alert_events_incident
    ON alert_events(incident_id)
    WHERE incident_id IS NOT NULL;
