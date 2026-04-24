BEGIN;

DROP INDEX IF EXISTS idx_discovered_assets_confidence;
DROP INDEX IF EXISTS idx_discovered_assets_pending_age;

ALTER TABLE discovered_assets
    DROP CONSTRAINT IF EXISTS chk_discovered_assets_match_strategy,
    DROP CONSTRAINT IF EXISTS chk_discovered_assets_confidence;

ALTER TABLE discovered_assets
    DROP COLUMN IF EXISTS review_reason,
    DROP COLUMN IF EXISTS match_strategy,
    DROP COLUMN IF EXISTS match_confidence;

COMMIT;
