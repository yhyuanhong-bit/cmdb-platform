-- ============================================================================
-- 000077: Add status CHECK constraint to racks
-- ============================================================================
-- W1.4 follow-up. Migration 000062 added status allowlists for assets,
-- work_orders, discovered_assets, alert_events, and inventory_tasks. The
-- racks table was missed: 000004 created it with status VARCHAR(20) NOT NULL
-- DEFAULT 'active' but never installed a CHECK, so any string value can be
-- written today.
--
-- Production usage (audited 2026-04-28):
--   * test-fixture seed inserts only 'active' and 'maintenance'.
--   * Frontend RackHeader form exposes 'active', 'maintenance',
--     'decommissioned' (lowercase).
--   * Frontend RackManagement filter shows the same three labels in upper
--     case ('ACTIVE' / 'MAINTENANCE' / 'DECOMMISSIONED'); those are display
--     strings only — the API contract is lowercase.
--   * Backend handler (impl_racks.go) passes the raw client string straight
--     into UpdateRack with no validation, so the DB is the only safety net.
--
-- The allowlist below is the canonical lowercase set actually exercised by
-- the product. We intentionally omit speculative values ('reserved',
-- 'planned', 'retired') until a feature lands that needs them, in line with
-- 000062's approach of widening only when production data justifies it.
--
-- incidents.status is NOT touched here: 000065_incidents_lifecycle already
-- installs chk_incidents_status with the 5-state lifecycle
-- ('open','acknowledged','investigating','resolved','closed').
--
-- Idempotent: DROP IF EXISTS then ADD, mirroring the 000062 pattern.
--
-- Pre-flight data normalization: production audit (2026-04-29) found 1 row
-- with status 'ACTIVE' (uppercase) — likely a stray write before the
-- frontend was disciplined to lowercase. We lowercase the value rather
-- than widen the allowlist, since uppercase is display-only per the
-- contract above.

BEGIN;

UPDATE racks SET status = lower(status) WHERE status <> lower(status);

ALTER TABLE racks DROP CONSTRAINT IF EXISTS chk_racks_status;
ALTER TABLE racks ADD CONSTRAINT chk_racks_status
    CHECK (status IN (
        'active',         -- in service (default)
        'maintenance',    -- temporarily offline for service
        'decommissioned'  -- permanently retired but still tracked
    ));

COMMIT;
