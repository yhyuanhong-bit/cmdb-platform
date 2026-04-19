-- Migration 000046: link approved discovered_assets back to the asset row
-- they produced.
--
-- Rationale: POST /discovery/{id}/approve is the handler that promotes a
-- staging row in `discovered_assets` into a canonical row in `assets`. Before
-- this migration the handler only flipped `discovered_assets.status` to
-- 'approved' without inserting into `assets`, so "approved" was a lie. The
-- new handler creates the asset inside a transaction, and we need a column
-- to record WHICH asset it created so that a retry (e.g. duplicate request
-- from an impatient UI or a post-commit network blip) is idempotent: the
-- second approve sees a non-null approved_asset_id, looks up the existing
-- asset, and returns it without double-creating.
--
-- Nullable on purpose: only rows where status='approved' will have a value;
-- 'pending', 'conflict', 'ignored', 'matched' keep NULL.
--
-- ON DELETE: we do NOT cascade. If an asset is hard-deleted the FK blocks
-- the delete; operationally assets are soft-deleted (deleted_at IS NOT NULL)
-- so this is not expected to fire. If you ever need to purge, null out the
-- pointer on discovered_assets first.

ALTER TABLE discovered_assets
    ADD COLUMN IF NOT EXISTS approved_asset_id UUID REFERENCES assets(id);

-- Partial index so the idempotency lookup (WHERE approved_asset_id = $1) is
-- index-only, and un-approved rows don't bloat the index.
CREATE INDEX IF NOT EXISTS idx_discovered_assets_approved_asset_id
    ON discovered_assets (approved_asset_id)
    WHERE approved_asset_id IS NOT NULL;
