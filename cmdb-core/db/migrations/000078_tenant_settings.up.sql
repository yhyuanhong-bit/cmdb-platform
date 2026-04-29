-- 000078_tenant_settings.up.sql
--
-- W3.2-backend: per-tenant configurable settings store.
--
-- Replaces the hardcoded asset-lifespan map in
-- internal/api/impl_prediction_upgrades.go (lines 87-92) with a
-- tenant-scoped configuration row that drives both the Asset Health
-- Score (RUL endpoint) and the upgrade-recommendation logic.
--
-- Design notes:
--   * One row per tenant (PRIMARY KEY = tenant_id). Updates are an
--     UPSERT on the JSONB column rather than INSERT-then-UPDATE so
--     we can write the row before the tenant has ever opened the
--     settings UI.
--   * The `settings` column is JSONB on purpose — every future
--     settings knob (alert thresholds, notification prefs, retention
--     policy, ...) lives under its own top-level key in the same row
--     instead of triggering a fresh ALTER TABLE every time. This is
--     YAGNI applied to the schema, not to the data: we add columns
--     when they earn their keep (frequent indexed lookups, FK
--     constraints, NOT NULL guarantees), and we leave the rest in
--     JSONB.
--   * `updated_by` is nullable so system-driven writes (e.g. a future
--     migration that backfills defaults) can record themselves
--     without inventing a synthetic user. ON DELETE SET NULL keeps
--     the audit trail intact when a user is removed.
--   * No data is seeded here. The Go layer falls back to canonical
--     defaults (server=5, network=7, storage=5, power=10) whenever a
--     tenant has no row yet, so the migration is non-blocking and
--     doesn't have to know about every existing tenant.

CREATE TABLE IF NOT EXISTS tenant_settings (
    tenant_id  UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    settings   JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

COMMENT ON TABLE tenant_settings IS
    'Per-tenant configuration. Single row per tenant; settings JSONB '
    'holds nested config blobs (e.g. asset_lifespan_config). Falls '
    'back to code defaults when a tenant has no row.';

COMMENT ON COLUMN tenant_settings.settings IS
    'JSONB blob of all tenant settings. Top-level keys are namespaced '
    'by feature (asset_lifespan_config, ...). Schema validation '
    'happens in the application layer, not the DB.';
