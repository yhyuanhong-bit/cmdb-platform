-- Wave 8.1: Metrics-pipeline source registry.
--
-- The platform already writes metric samples to the TimescaleDB
-- hypertable from several paths:
--   - the alert-rule evaluator's threshold computations
--   - the ingestion-engine (Python) for SNMP / IPMI / discovery
--   - manual one-off pushes during incidents
--
-- What it doesn't have is a first-class "is my data actually flowing?"
-- view. An operator who notices a flat dashboard has no quick way to
-- ask "which agent stopped reporting?" — they have to query the raw
-- hypertable for last-seen timestamps and reason backwards.
--
-- This migration adds metric_sources: one row per logical pusher of
-- metrics (a particular SNMP collector job, a specific IPMI agent, a
-- manual import script), with the cadence the operator expects from
-- it. A heartbeat call from the agent (or the ingestion endpoint that
-- routes its data) updates last_heartbeat_at + last_sample_count, and
-- a stale detector flags sources whose last_heartbeat is older than
-- 2× expected_interval.
--
-- This is the simplest building block for "data plane health": the
-- quality flagging + alert routing on top of these heartbeats can
-- live in a follow-up wave.

CREATE TABLE metric_sources (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                 UUID         NOT NULL REFERENCES tenants(id),
    name                      VARCHAR(120) NOT NULL,
    kind                      VARCHAR(24)  NOT NULL,
    -- kind classifies the puller / pusher type:
    --   snmp     — SNMP poller (typical for switches, PDUs, UPS)
    --   ipmi     — IPMI / Redfish agent (servers' BMCs)
    --   agent    — generic in-OS agent (telegraf, node-exporter, etc.)
    --   pipeline — the ingestion-engine itself
    --   manual   — operator-driven imports
    expected_interval_seconds INT          NOT NULL,
    -- The cadence the operator expects from this source. Stale
    -- detection treats now() - last_heartbeat_at > 2 × interval as
    -- stale; the 2× factor is tolerant of network jitter and short
    -- agent restarts without flapping the alert.
    status                    VARCHAR(16)  NOT NULL DEFAULT 'active',
    -- 'active' sources participate in stale detection.  'disabled'
    -- sources stay registered (so historic metrics retain their
    -- source name in audit) but are excluded from freshness checks.
    last_heartbeat_at         TIMESTAMPTZ,
    last_sample_count         BIGINT       NOT NULL DEFAULT 0,
    -- Cumulative lifetime counter — useful for "this agent has
    -- contributed N samples" without scanning the metrics
    -- hypertable. Bumped by the heartbeat with the per-call sample
    -- count.
    notes                     TEXT,
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_metric_sources_kind
        CHECK (kind IN ('snmp', 'ipmi', 'agent', 'pipeline', 'manual')),
    CONSTRAINT chk_metric_sources_status
        CHECK (status IN ('active', 'disabled')),
    CONSTRAINT chk_metric_sources_interval_positive
        CHECK (expected_interval_seconds > 0),
    -- (tenant_id, name) is unique so an operator typing "switch-a3"
    -- twice gets a clear constraint error rather than silently
    -- creating a duplicate registry row.
    CONSTRAINT uq_metric_sources_tenant_name UNIQUE (tenant_id, name)
);

-- "Is the data plane healthy?" dashboard query: tenant + status
-- filter, ordered by oldest heartbeat first.
CREATE INDEX idx_metric_sources_freshness
    ON metric_sources(tenant_id, last_heartbeat_at NULLS FIRST)
    WHERE status = 'active';

-- updated_at trigger, same pattern as other entities.
CREATE OR REPLACE FUNCTION trg_metric_sources_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER metric_sources_set_updated_at
    BEFORE UPDATE ON metric_sources
    FOR EACH ROW EXECUTE FUNCTION trg_metric_sources_set_updated_at();
