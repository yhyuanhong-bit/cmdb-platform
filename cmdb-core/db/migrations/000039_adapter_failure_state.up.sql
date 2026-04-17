-- Persist integration adapter failure tracking so service restarts do not
-- reset the counters. Enables exponential backoff gating and avoids the
-- previous in-memory-only adapterFailures map race condition.
ALTER TABLE integration_adapters
    ADD COLUMN IF NOT EXISTS consecutive_failures INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_failure_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_failure_reason  TEXT,
    ADD COLUMN IF NOT EXISTS next_attempt_at      TIMESTAMPTZ;

-- Partial index over enabled inbound adapters that are eligible to be polled
-- right now. Keeps the hot puller query fast even with many tenants.
CREATE INDEX IF NOT EXISTS idx_integration_adapters_next_attempt
    ON integration_adapters (next_attempt_at)
    WHERE enabled = true AND direction = 'inbound';
