-- Remove the 90-day retention policy and restore original 30-day policy
SELECT remove_retention_policy('metrics', if_exists => true);
SELECT add_retention_policy('metrics', INTERVAL '30 days');

-- Remove compression policy
SELECT remove_compression_policy('metrics', if_exists => true);

-- Decompress all compressed chunks before disabling compression
SELECT decompress_chunk(c, true)
FROM show_chunks('metrics') c
WHERE is_compressed;

-- Disable compression on the hypertable
ALTER TABLE metrics SET (timescaledb.compress = false);
