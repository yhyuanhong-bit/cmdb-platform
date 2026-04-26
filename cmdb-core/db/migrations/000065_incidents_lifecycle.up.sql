-- Wave 5.1: Incident lifecycle hardening.
--
-- Pre-5 the incidents table was a stub: title + status + severity + started_at
-- + resolved_at, with no state-machine enforcement, no assignee, no impact
-- record, and no link to the affected asset/service. That was fine for an
-- alert-to-incident dumping ground but inadequate for an operator actually
-- running an incident response.
--
-- This migration extends the row to carry the fields a real ITSM Incident
-- needs and plants a CHECK constraint for the status state machine. We also
-- add an incident_comments table so the UI can render a timeline instead of
-- surfacing silent status changes.

-- Columns added IF NOT EXISTS so a staging env that ran an earlier hotfix
-- won't trip on re-apply.
ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS description        TEXT,
    ADD COLUMN IF NOT EXISTS priority           VARCHAR(16),
    ADD COLUMN IF NOT EXISTS assignee_user_id   UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS affected_asset_id  UUID REFERENCES assets(id),
    ADD COLUMN IF NOT EXISTS affected_service_id UUID REFERENCES services(id),
    ADD COLUMN IF NOT EXISTS acknowledged_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS acknowledged_by    UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS resolved_by        UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS root_cause         TEXT,
    ADD COLUMN IF NOT EXISTS impact             TEXT,
    ADD COLUMN IF NOT EXISTS updated_at         TIMESTAMPTZ NOT NULL DEFAULT now();

-- Backfill updated_at for existing rows (trigger is set up below, so new
-- rows get it for free; this line matters for the migration step only).
UPDATE incidents SET updated_at = COALESCE(resolved_at, started_at) WHERE updated_at IS NULL;

-- Priority: keep NULL tolerated for legacy rows so the migration is
-- non-breaking; new writes should pick from the CHECK set.
ALTER TABLE incidents
    DROP CONSTRAINT IF EXISTS chk_incidents_priority,
    ADD CONSTRAINT chk_incidents_priority
    CHECK (priority IS NULL OR priority IN ('p1', 'p2', 'p3', 'p4'));

-- Severity: enum tightened — previously it was free-text which let the UI
-- render whatever came out of alert_rules.severity. The allowed set matches
-- the alert_rules.severity vocabulary ('warning' exists there) plus the
-- higher-signal values an operator might pick manually ('critical', 'high',
-- 'medium', 'low', 'info'). CHECK rather than code so bad writes fail fast
-- at the DB.
ALTER TABLE incidents
    DROP CONSTRAINT IF EXISTS chk_incidents_severity,
    ADD CONSTRAINT chk_incidents_severity
    CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info', 'warning'));

-- Status state machine — any write outside this set fails. The domain layer
-- also guards the transitions (you can't go closed → open) but the CHECK
-- is our defence-in-depth.
ALTER TABLE incidents
    DROP CONSTRAINT IF EXISTS chk_incidents_status,
    ADD CONSTRAINT chk_incidents_status
    CHECK (status IN ('open', 'acknowledged', 'investigating', 'resolved', 'closed'));

-- Resolved coherence: if we've set a resolved_at we must also know who
-- closed it (or be migrating legacy data, in which case resolved_by stays
-- NULL — that's intentional, not broken).

-- Indexes used by list/filter UI.
CREATE INDEX IF NOT EXISTS idx_incidents_assignee
    ON incidents(tenant_id, assignee_user_id) WHERE assignee_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_incidents_affected_service
    ON incidents(tenant_id, affected_service_id) WHERE affected_service_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_incidents_affected_asset
    ON incidents(tenant_id, affected_asset_id) WHERE affected_asset_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_incidents_priority
    ON incidents(tenant_id, priority) WHERE priority IS NOT NULL;

-- updated_at trigger so the domain layer doesn't have to remember to stamp
-- it. Postgres has no shorthand so we roll the usual 3-liner.
CREATE OR REPLACE FUNCTION trg_incidents_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS incidents_set_updated_at ON incidents;
CREATE TRIGGER incidents_set_updated_at
    BEFORE UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION trg_incidents_set_updated_at();

-- ---------------------------------------------------------------------------
-- incident_comments: activity timeline.
-- ---------------------------------------------------------------------------
-- Every lifecycle change (acknowledge, resolve, reopen) writes a system
-- comment in the same transaction as the status update, so the timeline is
-- always consistent with the row. Human comments get kind='human'.
CREATE TABLE IF NOT EXISTS incident_comments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id),
    incident_id  UUID        NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    author_id    UUID        REFERENCES users(id),
    kind         VARCHAR(16) NOT NULL DEFAULT 'human',
    body         TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_incident_comments_kind
        CHECK (kind IN ('human', 'system'))
);

CREATE INDEX IF NOT EXISTS idx_incident_comments_incident
    ON incident_comments(incident_id, created_at);
CREATE INDEX IF NOT EXISTS idx_incident_comments_tenant
    ON incident_comments(tenant_id);
