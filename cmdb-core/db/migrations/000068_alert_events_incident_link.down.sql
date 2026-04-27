DROP INDEX IF EXISTS idx_alert_events_incident;
ALTER TABLE alert_events DROP COLUMN IF EXISTS incident_id;
