-- Wave 6.2: Energy Phase 2 — PUE per location + asset anomalies.
--
-- Phase 1 (000069) gave us per-asset daily kWh. Phase 2 adds:
--
--   1. energy_location_daily — per-location, per-day rollup that splits
--      kWh into IT (servers/network/storage) vs non-IT (cooling, UPS,
--      lighting, …). PUE = total / IT, computable on read so we never
--      store a derived value that disagrees with the inputs.
--
--   2. energy_anomalies — asset-day rows flagged by the rule-based
--      detector. Simple baseline (7-day median) × threshold; persisted
--      as a row so the UI can surface a backlog and operators can mark
--      reviewed without re-running the detector.
--
-- The PUE definition above is deliberately the "Category 1" one (just
-- power totals), not the more sophisticated "Category 3" with metered
-- breakdown. We don't have meter-level wiring yet; a single tenant
-- might have a row per location even when only some power is actually
-- attributed. Operators see the limitation in the UI when sample_count
-- is low.

-- ---------------------------------------------------------------------------
-- energy_location_daily — per-location PUE rollup.
-- ---------------------------------------------------------------------------
CREATE TABLE energy_location_daily (
    tenant_id     UUID        NOT NULL REFERENCES tenants(id),
    location_id   UUID        NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    day           DATE        NOT NULL,
    it_kwh        NUMERIC(14,4) NOT NULL DEFAULT 0,
    non_it_kwh    NUMERIC(14,4) NOT NULL DEFAULT 0,
    total_kwh     NUMERIC(14,4) NOT NULL DEFAULT 0,
    -- Asset-count denominators help the UI tell "10 servers, 0 cooling"
    -- from "10 servers, 5 cooling, low PUE" — both have the same kWh
    -- ratio but the operator should treat them very differently.
    it_asset_count     INTEGER NOT NULL DEFAULT 0,
    non_it_asset_count INTEGER NOT NULL DEFAULT 0,
    computed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, location_id, day),

    CONSTRAINT chk_location_daily_kwh_consistency
        CHECK (total_kwh >= 0 AND it_kwh >= 0 AND non_it_kwh >= 0)
);

CREATE INDEX idx_energy_location_daily_tenant_day
    ON energy_location_daily(tenant_id, day DESC);

-- ---------------------------------------------------------------------------
-- energy_anomalies — flagged asset-days for operator review.
-- ---------------------------------------------------------------------------
-- kind:
--   'high' → daily kWh ≥ baseline_median × high_threshold
--   'low'  → daily kWh ≤ baseline_median × low_threshold AND > 0
-- (zero-kWh days are not anomalies — they could be planned downtime; the
--  lack of a power_kw sample is a separate signal owned by the alerts
--  pipeline.)
--
-- status flow:
--   open    → operator hasn't looked
--   ack     → operator confirmed (reviewed but accepted as expected)
--   resolved → operator confirmed and the underlying issue is fixed
--
-- The PRIMARY KEY on (tenant_id, asset_id, day) makes the detector's
-- INSERT … ON CONFLICT idempotent: re-running for the same day overwrites
-- the score columns but preserves status + reviewed_by so an operator's
-- ack survives a re-run.
CREATE TABLE energy_anomalies (
    tenant_id        UUID        NOT NULL REFERENCES tenants(id),
    asset_id         UUID        NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    day              DATE        NOT NULL,
    kind             VARCHAR(8)  NOT NULL,
    baseline_median  NUMERIC(12,4) NOT NULL,
    observed_kwh     NUMERIC(12,4) NOT NULL,
    score            NUMERIC(8,4) NOT NULL,
    -- score = observed / baseline. >= 1 for 'high', between 0 and 1 for
    -- 'low'. Stored so the UI can sort by severity.
    status           VARCHAR(16) NOT NULL DEFAULT 'open',
    detected_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_by      UUID REFERENCES users(id),
    reviewed_at      TIMESTAMPTZ,
    note             TEXT,
    PRIMARY KEY (tenant_id, asset_id, day),

    CONSTRAINT chk_energy_anomaly_kind
        CHECK (kind IN ('high', 'low')),
    CONSTRAINT chk_energy_anomaly_status
        CHECK (status IN ('open', 'ack', 'resolved'))
);

-- Open-anomalies dashboard query: tenant + status filter.
CREATE INDEX idx_energy_anomalies_open
    ON energy_anomalies(tenant_id, status, detected_at DESC)
    WHERE status = 'open';

CREATE INDEX idx_energy_anomalies_tenant_day
    ON energy_anomalies(tenant_id, day DESC);
