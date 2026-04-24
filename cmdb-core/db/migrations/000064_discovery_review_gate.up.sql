-- ============================================================================
-- 000064: Discovery review gate columns (Wave 3)
-- ============================================================================
--
-- Spec: db/specs/services.md roadmap reference + docs/reviews/2026-04-22-
--       business-fit-review.md #3 (Discovery 审核闸门).
--
-- Adds the metadata the review queue needs:
--
--   match_confidence  — how sure the ingestion pipeline is that this
--                       discovered asset matches `matched_asset_id`.
--                       Range: 0.0 (no confidence) to 1.0 (exact match).
--                       NULL means "not matched yet" (status=pending).
--
--   match_strategy    — which rule the pipeline used to pick
--                       matched_asset_id: 'serial_number' | 'asset_tag'
--                       | 'hostname' | 'ip' | 'manual' | NULL when not
--                       matched. Used by the review UI to tell the user
--                       "we matched this by serial — do you trust that?"
--
--   review_reason     — captured when an operator approves / rejects /
--                       ignores a discovery. Required from the UI so the
--                       audit trail can answer "why did anyone accept
--                       this change?". NULL only on pre-Wave-3 rows.
--
-- Values for existing rows: back-filled to safe defaults. match_confidence
-- is set to 1.0 for rows that already have matched_asset_id (the legacy
-- pipeline auto-merged without measuring confidence; assume certain) and
-- NULL for unmatched rows. match_strategy gets 'legacy' for any row with
-- matched_asset_id to mark "we do not know how this was matched" without
-- erasing the link.

BEGIN;

ALTER TABLE discovered_assets
    ADD COLUMN IF NOT EXISTS match_confidence NUMERIC(3,2),
    ADD COLUMN IF NOT EXISTS match_strategy   VARCHAR(32),
    ADD COLUMN IF NOT EXISTS review_reason    TEXT;

-- Constrain confidence to the [0, 1] range the ingestion pipeline agrees on.
ALTER TABLE discovered_assets
    ADD CONSTRAINT chk_discovered_assets_confidence
    CHECK (match_confidence IS NULL OR (match_confidence >= 0.0 AND match_confidence <= 1.0));

-- Allowlist for match_strategy: 'legacy' is the back-fill sentinel for
-- pre-Wave-3 rows; new rows must declare an explicit strategy so the
-- review UI can render a trust signal per match.
ALTER TABLE discovered_assets
    ADD CONSTRAINT chk_discovered_assets_match_strategy
    CHECK (match_strategy IS NULL OR match_strategy IN (
        'serial_number', 'asset_tag', 'hostname', 'ip', 'manual', 'legacy'
    ));

-- Back-fill for rows that already have a matched_asset_id.
UPDATE discovered_assets
SET match_confidence = 1.0,
    match_strategy   = 'legacy'
WHERE matched_asset_id IS NOT NULL
  AND match_strategy IS NULL;

CREATE INDEX IF NOT EXISTS idx_discovered_assets_pending_age
    ON discovered_assets(tenant_id, discovered_at)
    WHERE status IN ('pending', 'conflict');

CREATE INDEX IF NOT EXISTS idx_discovered_assets_confidence
    ON discovered_assets(tenant_id, match_confidence)
    WHERE match_confidence IS NOT NULL;

COMMIT;
