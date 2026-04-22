-- D9-P0 from review-2026-04-21-v2: add a consumer-side feedback loop
-- for data quality. Until now the quality scanner wrote *into* assets
-- but there was no inbound path — downstream systems (incident tools,
-- dashboards, humans looking at a stale record) had no way to say
-- "this asset's data is wrong". The scanner scored CIs against static
-- rules but never heard back from anybody consuming them.
--
-- quality_flags is that inbound channel: one row per reported issue
-- against one asset. The scanner reads the last-24h flag count for
-- each asset during evaluation and applies an accuracy penalty, so a
-- CI that callers actively distrust drops in score even if all of its
-- own fields look schema-clean.
--
-- Severity is a fixed vocabulary so the penalty table can be constant.
-- Status starts at 'open' and moves to 'acknowledged' / 'resolved' /
-- 'rejected' through separate transitions; we keep the historical row
-- around either way so auto-WO attribution can still find it.

BEGIN;

CREATE TABLE quality_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    asset_id        UUID         NOT NULL REFERENCES assets(id) ON DELETE CASCADE,

    -- Who filed it. reporter_type is required so we can weight system
    -- vs. user reports differently if needed; reporter_id is nullable
    -- because external/downstream systems may not have a CMDB user.
    reporter_type   VARCHAR(32)  NOT NULL CHECK (reporter_type IN ('user', 'system', 'external', 'downstream')),
    reporter_id     UUID,

    severity        VARCHAR(16)  NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    category        VARCHAR(64)  NOT NULL,
    message         TEXT         NOT NULL,

    -- Triage state. 'open' until someone picks it up; 'rejected'
    -- preserves the report without counting toward penalties.
    status          VARCHAR(16)  NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'acknowledged', 'resolved', 'rejected')),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID,
    resolution_note TEXT,

    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Primary read path during scan: "how many penalizable flags does
-- asset X have in the last 24h". The partial predicate keeps the
-- index small and skips rejected reports which do not count.
CREATE INDEX idx_quality_flags_asset_recent
    ON quality_flags (tenant_id, asset_id, created_at DESC)
    WHERE status IN ('open', 'acknowledged');

-- Triage list: "show me all open flags sorted by severity". Drives
-- the operator dashboard that is not yet built but will land next.
CREATE INDEX idx_quality_flags_open
    ON quality_flags (tenant_id, severity, created_at DESC)
    WHERE status = 'open';

-- Catch-all composite for the per-asset history view and the auto-WO
-- worker's "has this asset been flagged recently" probe.
CREATE INDEX idx_quality_flags_tenant_created
    ON quality_flags (tenant_id, created_at DESC);

COMMIT;
