-- Enable compression on the metrics hypertable
-- Segment by tenant_id, asset_id, and name for efficient filtered queries
-- Order by time DESC for time-range scans
ALTER TABLE metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'tenant_id, asset_id, name',
    timescaledb.compress_orderby = 'time DESC'
);

-- Automatically compress chunks older than 7 days
SELECT add_compression_policy('metrics', INTERVAL '7 days');

-- Update retention policy from 30 days to 90 days
-- Remove the existing 30-day policy first, then add 90-day policy
SELECT remove_retention_policy('metrics', if_exists => true);
SELECT add_retention_policy('metrics', INTERVAL '90 days');
