CREATE TABLE metrics (
    time      TIMESTAMPTZ      NOT NULL,
    asset_id  UUID,
    tenant_id UUID,
    name      VARCHAR(100)     NOT NULL,
    value     DOUBLE PRECISION,
    labels    JSONB
);

SELECT create_hypertable('metrics', 'time');

CREATE INDEX idx_metrics_asset_name_time ON metrics(asset_id, name, time DESC);
CREATE INDEX idx_metrics_tenant_time ON metrics(tenant_id, time DESC);

SELECT add_retention_policy('metrics', INTERVAL '30 days');

-- 5-minute continuous aggregate
CREATE MATERIALIZED VIEW metrics_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    asset_id,
    tenant_id,
    name,
    avg(value)  AS avg_value,
    max(value)  AS max_value,
    min(value)  AS min_value
FROM metrics
GROUP BY bucket, asset_id, tenant_id, name
WITH NO DATA;

SELECT add_continuous_aggregate_policy('metrics_5min',
    start_offset  => INTERVAL '1 hour',
    end_offset    => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes'
);

SELECT add_retention_policy('metrics_5min', INTERVAL '180 days');

-- 1-hour continuous aggregate
CREATE MATERIALIZED VIEW metrics_1hour
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    asset_id,
    tenant_id,
    name,
    avg(value)  AS avg_value,
    max(value)  AS max_value,
    min(value)  AS min_value
FROM metrics
GROUP BY bucket, asset_id, tenant_id, name
WITH NO DATA;

SELECT add_continuous_aggregate_policy('metrics_1hour',
    start_offset  => INTERVAL '2 hours',
    end_offset    => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour'
);

SELECT add_retention_policy('metrics_1hour', INTERVAL '730 days');
