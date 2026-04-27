-- Wave 7.1: Predictive Phase 1 — hardware-refresh recommendations.
--
-- The platform already records purchase_date, warranty_end, eol_date,
-- and expected_lifespan_months per asset (added in 000036). This wave
-- runs a rule engine over those fields and persists one row per
-- (asset, kind) so capex planners get an auditable backlog rather
-- than an ad-hoc query.
--
-- Phase 1 is intentionally rule-based, not ML — the platform doesn't
-- yet have enough historical refresh-vs-failure data to train a model
-- that beats the rules. Per the planning doc the upgrade path is
-- "build the data pipeline, train when data matures." The schema here
-- is shaped so model-driven scores can replace the rule engine without
-- table changes (just a different writer).

CREATE TABLE predictive_refresh_recommendations (
    tenant_id           UUID        NOT NULL REFERENCES tenants(id),
    asset_id            UUID        NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    kind                VARCHAR(24) NOT NULL,
    -- The five rule kinds the Phase 1 engine can produce. Captured in
    -- the CHECK so a buggy writer can't smuggle a typo into the table.
    --   warranty_expiring  — warranty ends within 90 days
    --   warranty_expired   — warranty ended already
    --   eol_approaching    — EOL within 180 days
    --   eol_passed         — EOL date already passed
    --   aged_out           — purchase_date older than expected_lifespan_months
    risk_score          NUMERIC(5,2) NOT NULL,
    -- Score 0-100. Higher = more urgent. The rule engine maps days
    -- remaining (or days past) into this band. Using NUMERIC keeps the
    -- comparator stable when an ML model later writes fractional scores.
    reason              TEXT        NOT NULL,
    recommended_action  TEXT,
    target_date         DATE,
    -- target_date is the operator-actionable deadline: warranty/EOL date
    -- for those kinds, computed-end-of-life for aged_out. Renders as
    -- "by 2026-08-12" in the UI.
    status              VARCHAR(16) NOT NULL DEFAULT 'open',
    detected_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- detected_at refreshes on every UPSERT so the UI can show
    -- "first surfaced X days ago" alongside "score now Y."
    reviewed_by         UUID REFERENCES users(id),
    reviewed_at         TIMESTAMPTZ,
    note                TEXT,

    PRIMARY KEY (tenant_id, asset_id, kind),

    CONSTRAINT chk_predictive_refresh_kind
        CHECK (kind IN ('warranty_expiring', 'warranty_expired',
                        'eol_approaching', 'eol_passed', 'aged_out')),
    CONSTRAINT chk_predictive_refresh_status
        CHECK (status IN ('open', 'ack', 'resolved')),
    CONSTRAINT chk_predictive_refresh_score_range
        CHECK (risk_score >= 0 AND risk_score <= 100)
);

-- The dashboard query: tenant + status + reverse-chrono detection.
CREATE INDEX idx_predictive_refresh_open
    ON predictive_refresh_recommendations(tenant_id, status, risk_score DESC, detected_at DESC)
    WHERE status = 'open';

-- The asset-detail view: "what's pending on this rack?"
CREATE INDEX idx_predictive_refresh_asset
    ON predictive_refresh_recommendations(tenant_id, asset_id);

-- The capex-planning view: order by target_date so the soonest
-- deadlines surface first.
CREATE INDEX idx_predictive_refresh_target
    ON predictive_refresh_recommendations(tenant_id, target_date)
    WHERE target_date IS NOT NULL AND status = 'open';
