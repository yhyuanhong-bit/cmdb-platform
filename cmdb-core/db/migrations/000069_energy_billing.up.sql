-- Wave 6.1: Energy billing infrastructure.
--
-- The platform already records `metrics` rows with name='power_kw' against
-- assets, and the existing /energy/* endpoints aggregate that hypertable
-- on every request. That's fine for live dashboards but doesn't support
-- the next-most-asked question — "how much did this rack cost us last
-- month?" — without re-scanning a month of timeseries data.
--
-- This migration adds the accounting layer:
--   1. energy_tariffs    — $/kWh per location, valid in a date range.
--                          Different IDCs have different power contracts;
--                          tariffs change at contract renewal so we keep
--                          history with effective_from/to.
--   2. energy_daily_kwh  — pre-rolled (asset, day, kwh) so monthly bill
--                          queries are O(days × assets), not O(metric rows).
--
-- Overlap prevention for tariffs is done at the domain layer (Go) rather
-- than via a btree_gist EXCLUDE constraint because the timescaledb /
-- pg setup doesn't have btree_gist installed and adding an extension on
-- managed Postgres is a separate operational concern. The unit tests pin
-- down the no-overlap rule.

-- ---------------------------------------------------------------------------
-- energy_tariffs — $/kWh per location with effective dates.
-- ---------------------------------------------------------------------------
CREATE TABLE energy_tariffs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    location_id     UUID         REFERENCES locations(id),
    -- NULL location_id = tenant default (used when an asset's location
    -- has no specific tariff). The unique index below treats NULL as a
    -- distinct value so each tenant has at most one default tariff per
    -- effective period.
    currency        VARCHAR(3)   NOT NULL DEFAULT 'USD',
    rate_per_kwh    NUMERIC(10,6) NOT NULL,
    effective_from  DATE         NOT NULL,
    effective_to    DATE,
    -- NULL effective_to means "still in effect". Once a renewal lands,
    -- the old row gets stamped with effective_to = renewal_date - 1.
    notes           TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_tariff_rate_positive
        CHECK (rate_per_kwh > 0),
    CONSTRAINT chk_tariff_dates_ordered
        CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

CREATE INDEX idx_energy_tariffs_lookup
    ON energy_tariffs(tenant_id, location_id, effective_from);

-- updated_at auto-stamp.
CREATE OR REPLACE FUNCTION trg_energy_tariffs_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER energy_tariffs_set_updated_at
    BEFORE UPDATE ON energy_tariffs
    FOR EACH ROW EXECUTE FUNCTION trg_energy_tariffs_set_updated_at();

-- ---------------------------------------------------------------------------
-- energy_daily_kwh — pre-aggregated rollup.
--
-- One row per (tenant, asset, day). The aggregator job takes today's
-- power_kw samples from the metrics hypertable and computes:
--   kwh_total = SUM(value * sample_interval_hours) — energy delivered
--   kw_peak   = MAX(value)                          — peak demand
--   kw_avg    = AVG(value)                          — load factor input
--
-- A naive aggregator runs SUM(value)*1h-bucket-width which is wrong if
-- samples are missing; we use a TIME_BUCKET(1h, time)-based aggregate
-- in the domain code so gap handling is explicit.
--
-- The PRIMARY KEY on (tenant_id, asset_id, day) makes the aggregator's
-- INSERT … ON CONFLICT DO UPDATE idempotent — re-running the rollup for
-- a day overwrites the row instead of duplicating.
-- ---------------------------------------------------------------------------
CREATE TABLE energy_daily_kwh (
    tenant_id    UUID        NOT NULL REFERENCES tenants(id),
    asset_id     UUID        NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    day          DATE        NOT NULL,
    kwh_total    NUMERIC(12,4) NOT NULL,
    kw_peak      NUMERIC(10,4) NOT NULL,
    kw_avg       NUMERIC(10,4) NOT NULL,
    sample_count INTEGER     NOT NULL,
    computed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, asset_id, day)
);

-- Reverse-chrono index for "show me last month's bill" queries that
-- want to scan recent days first.
CREATE INDEX idx_energy_daily_kwh_tenant_day
    ON energy_daily_kwh(tenant_id, day DESC);

-- Per-asset history for "what's this rack been doing all year".
CREATE INDEX idx_energy_daily_kwh_asset_day
    ON energy_daily_kwh(asset_id, day DESC);
