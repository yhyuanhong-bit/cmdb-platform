-- Wave 5.3: ITIL Change Management.
--
-- A Change is a planned modification to one or more configuration items
-- that needs approval before execution. Three flavours per ITIL:
--
--   standard  — pre-approved, low-risk, well-known procedure (e.g. patch
--               an OS to a known-good version). Skips CAB review and goes
--               submitted → approved automatically. The audit trail still
--               records the submit so we can show "this came in pre-
--               approved" rather than "no review happened."
--   normal    — needs CAB review. Requires N approvers (approval_threshold
--               on the row, default 1) before the change auto-transitions
--               to `approved`. A single reject blocks immediately.
--   emergency — bypasses CAB queue but the audit trail flags it as
--               emergency for retroactive review.
--
-- Lifecycle:
--   draft → submitted → approved | rejected
--   approved → in_progress → succeeded | failed | rolled_back
--
-- The two halves are kept distinct because the approval phase and the
-- execution phase have different actors (CAB voters vs implementers) and
-- different SLAs.

CREATE TABLE changes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID         NOT NULL REFERENCES tenants(id),
    title               VARCHAR(255) NOT NULL,
    description         TEXT,
    type                VARCHAR(16)  NOT NULL DEFAULT 'normal',
    risk                VARCHAR(16)  NOT NULL DEFAULT 'medium',
    status              VARCHAR(20)  NOT NULL DEFAULT 'draft',
    -- Approval threshold: how many `approve` votes are needed before
    -- the change auto-transitions to `approved`. Standard changes have
    -- threshold=0 (auto-approved on submit). Normal default is 1; CAB
    -- can require more for high-risk by setting it on create/update.
    approval_threshold  INT          NOT NULL DEFAULT 1,
    requested_by        UUID         REFERENCES users(id),
    assignee_user_id    UUID         REFERENCES users(id),
    planned_start       TIMESTAMPTZ,
    planned_end         TIMESTAMPTZ,
    actual_start        TIMESTAMPTZ,
    actual_end          TIMESTAMPTZ,
    rollback_plan       TEXT,
    impact_summary      TEXT,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    submitted_at        TIMESTAMPTZ,
    approved_at         TIMESTAMPTZ,
    rejected_at         TIMESTAMPTZ,

    CONSTRAINT chk_changes_type
        CHECK (type IN ('standard', 'normal', 'emergency')),
    CONSTRAINT chk_changes_risk
        CHECK (risk IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT chk_changes_status
        CHECK (status IN ('draft', 'submitted', 'approved', 'rejected',
                          'in_progress', 'succeeded', 'failed', 'rolled_back')),
    CONSTRAINT chk_changes_threshold
        CHECK (approval_threshold >= 0)
);

CREATE INDEX idx_changes_tenant_status   ON changes(tenant_id, status);
CREATE INDEX idx_changes_tenant_risk     ON changes(tenant_id, risk);
CREATE INDEX idx_changes_tenant_type     ON changes(tenant_id, type);
CREATE INDEX idx_changes_assignee        ON changes(tenant_id, assignee_user_id) WHERE assignee_user_id IS NOT NULL;
CREATE INDEX idx_changes_tenant_created  ON changes(tenant_id, created_at DESC);
CREATE INDEX idx_changes_planned_start   ON changes(tenant_id, planned_start) WHERE planned_start IS NOT NULL;

CREATE OR REPLACE FUNCTION trg_changes_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER changes_set_updated_at
    BEFORE UPDATE ON changes
    FOR EACH ROW EXECUTE FUNCTION trg_changes_set_updated_at();

-- ---------------------------------------------------------------------------
-- change_approvals — one row per CAB voter's decision.
--
-- A change becomes `approved` once the count of `approve` rows reaches
-- changes.approval_threshold AND no `reject` rows exist. A single `reject`
-- transitions the change to `rejected` immediately. `abstain` rows count
-- toward neither side but are recorded for the audit trail.
--
-- Each voter votes at most once per change — UNIQUE on (change_id, voter_id).
-- Changing your mind requires updating the existing row, not inserting a
-- new one (so the audit trail is one decision per voter).
-- ---------------------------------------------------------------------------
CREATE TABLE change_approvals (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    change_id   UUID        NOT NULL REFERENCES changes(id) ON DELETE CASCADE,
    voter_id    UUID        NOT NULL REFERENCES users(id),
    vote        VARCHAR(8)  NOT NULL,
    note        TEXT,
    voted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (change_id, voter_id),
    CONSTRAINT chk_change_approvals_vote
        CHECK (vote IN ('approve', 'reject', 'abstain'))
);

CREATE INDEX idx_change_approvals_change ON change_approvals(change_id);
CREATE INDEX idx_change_approvals_tenant ON change_approvals(tenant_id);

-- ---------------------------------------------------------------------------
-- M:N linkage tables. Same pattern as incident_problem_links: PK on the
-- pair so re-linking is idempotent at the index level, tenant_id column
-- so the schema enforces tenant scope even if a buggy handler tried.
-- ---------------------------------------------------------------------------
CREATE TABLE change_assets (
    change_id   UUID        NOT NULL REFERENCES changes(id) ON DELETE CASCADE,
    asset_id    UUID        NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (change_id, asset_id)
);
CREATE INDEX idx_change_assets_asset  ON change_assets(asset_id);
CREATE INDEX idx_change_assets_tenant ON change_assets(tenant_id);

CREATE TABLE change_services (
    change_id   UUID        NOT NULL REFERENCES changes(id) ON DELETE CASCADE,
    service_id  UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (change_id, service_id)
);
CREATE INDEX idx_change_services_service ON change_services(service_id);
CREATE INDEX idx_change_services_tenant  ON change_services(tenant_id);

CREATE TABLE change_problems (
    change_id   UUID        NOT NULL REFERENCES changes(id) ON DELETE CASCADE,
    problem_id  UUID        NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (change_id, problem_id)
);
CREATE INDEX idx_change_problems_problem ON change_problems(problem_id);
CREATE INDEX idx_change_problems_tenant  ON change_problems(tenant_id);

-- ---------------------------------------------------------------------------
-- change_comments — timeline mirroring incident/problem comments.
-- ---------------------------------------------------------------------------
CREATE TABLE change_comments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    change_id   UUID        NOT NULL REFERENCES changes(id) ON DELETE CASCADE,
    author_id   UUID        REFERENCES users(id),
    kind        VARCHAR(16) NOT NULL DEFAULT 'human',
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_change_comments_kind
        CHECK (kind IN ('human', 'system'))
);
CREATE INDEX idx_change_comments_change ON change_comments(change_id, created_at);
CREATE INDEX idx_change_comments_tenant ON change_comments(tenant_id);
