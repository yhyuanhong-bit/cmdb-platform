-- Wave 5.2: Problem management.
--
-- A Problem is the underlying root cause of one-or-more Incidents (ITIL
-- definition). The two entities have separate lifecycles: an incident is
-- "the user can't log in right now"; the problem is "the auth service has
-- a memory leak that fires every Tuesday under load". Multiple incidents
-- can map to one problem, and an incident may link to several problems
-- when the failure spans subsystems — hence the M:N link table.
--
-- The lifecycle here parallels Wave 5.1 incidents:
--   open → investigating → known_error → resolved → closed
-- 'known_error' is ITIL's name for "we know what's wrong and have a
-- workaround, but the permanent fix is not yet shipped". It's optional —
-- a problem may go investigating → resolved directly when fix lands fast.

CREATE TABLE problems (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID         NOT NULL REFERENCES tenants(id),
    title         VARCHAR(255) NOT NULL,
    description   TEXT,
    status        VARCHAR(20)  NOT NULL DEFAULT 'open',
    priority      VARCHAR(8),
    severity      VARCHAR(20)  NOT NULL DEFAULT 'medium',
    root_cause    TEXT,
    workaround    TEXT,
    resolution    TEXT,
    assignee_user_id UUID REFERENCES users(id),
    created_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ,
    resolved_by   UUID REFERENCES users(id),
    closed_at     TIMESTAMPTZ,

    CONSTRAINT chk_problems_status
        CHECK (status IN ('open', 'investigating', 'known_error', 'resolved', 'closed')),
    CONSTRAINT chk_problems_priority
        CHECK (priority IS NULL OR priority IN ('p1', 'p2', 'p3', 'p4')),
    CONSTRAINT chk_problems_severity
        CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info', 'warning'))
);

CREATE INDEX idx_problems_tenant_status ON problems(tenant_id, status);
CREATE INDEX idx_problems_tenant_priority ON problems(tenant_id, priority) WHERE priority IS NOT NULL;
CREATE INDEX idx_problems_assignee ON problems(tenant_id, assignee_user_id) WHERE assignee_user_id IS NOT NULL;
CREATE INDEX idx_problems_tenant_created ON problems(tenant_id, created_at DESC);

-- updated_at auto-stamp. Same pattern as incidents (000065).
CREATE OR REPLACE FUNCTION trg_problems_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER problems_set_updated_at
    BEFORE UPDATE ON problems
    FOR EACH ROW EXECUTE FUNCTION trg_problems_set_updated_at();

-- ---------------------------------------------------------------------------
-- incident_problem_links: M:N between incidents and problems.
-- ---------------------------------------------------------------------------
-- An incident can be associated with multiple problems (e.g. login failure
-- caused by both a leaky auth service AND a flaky load balancer); a problem
-- typically covers many incidents over time. The PK is (incident_id,
-- problem_id) so duplicate links are rejected by the index, not the app.
--
-- Tenant guard is in the table so cross-tenant linkage is impossible at
-- the schema level even if a buggy handler tried.
CREATE TABLE incident_problem_links (
    incident_id  UUID        NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    problem_id   UUID        NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    tenant_id    UUID        NOT NULL REFERENCES tenants(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by   UUID REFERENCES users(id),
    PRIMARY KEY (incident_id, problem_id)
);

CREATE INDEX idx_incident_problem_links_problem ON incident_problem_links(problem_id);
CREATE INDEX idx_incident_problem_links_incident ON incident_problem_links(incident_id);
CREATE INDEX idx_incident_problem_links_tenant ON incident_problem_links(tenant_id);

-- ---------------------------------------------------------------------------
-- problem_comments: timeline mirroring incident_comments (000065).
-- ---------------------------------------------------------------------------
CREATE TABLE problem_comments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id),
    problem_id   UUID        NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    author_id    UUID        REFERENCES users(id),
    kind         VARCHAR(16) NOT NULL DEFAULT 'human',
    body         TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_problem_comments_kind
        CHECK (kind IN ('human', 'system'))
);

CREATE INDEX idx_problem_comments_problem ON problem_comments(problem_id, created_at);
CREATE INDEX idx_problem_comments_tenant ON problem_comments(tenant_id);
