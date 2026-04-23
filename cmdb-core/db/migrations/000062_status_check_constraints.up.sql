-- ============================================================================
-- 000062: Extend status field CHECK constraints
-- ============================================================================
-- Migration 000028 first introduced status CHECK constraints on assets,
-- work_orders, alert_events, and inventory_tasks. This migration:
--   1. Widens those allowlists to match values actually seen in production
--      (audited 2026-04-23: assets has 'active'/'deployed'/'inventoried'/
--      'maintenance'/'operational'/'retired'; work_orders has all 7 plus
--      'cancelled' coming in product roadmap).
--   2. Adds a NEW constraint on discovered_assets.status (not covered by 028).
--   3. Adds 'silenced' to alert_events (standard alerting feature).
--   4. Adds 'cancelled' to inventory_tasks.
--
-- Approach: DROP each pre-existing constraint, then ADD the superset. This
-- keeps the migration idempotent when re-applied against a DB that has
-- 028 but not 062, without needing separate up-steps.
--
-- Allowlist rationale:
--   assets: the 028 set already covers the canonical lifecycle. We
--     remove 'procurement' / 'deploying' / 'decommission' / 'offline'
--     (zero production rows observed) and add 'planned' / 'in_stock' to
--     match the new-hardware-received workflow the roadmap assumes.
--   work_orders: add 'cancelled' for abort paths.
--   alert_events: add 'silenced' for maintenance-window suppression.
--   inventory_tasks: add 'cancelled' for abort paths.
--   discovered_assets: NEW — the existing 'pending'/'matched'/'conflict'/
--     'approved'/'ignored' states are all exercised by Wave 3.

BEGIN;

-- ---------------------------------------------------------------------------
-- assets.status — replace 028's constraint with the canonical + roadmap set.
-- ---------------------------------------------------------------------------
ALTER TABLE assets DROP CONSTRAINT IF EXISTS chk_assets_status;
ALTER TABLE assets ADD CONSTRAINT chk_assets_status
    CHECK (status IN (
        'planned',       -- procured but not yet received (roadmap)
        'in_stock',      -- received, in warehouse (roadmap)
        'inventoried',   -- physically located, pre-deployment
        'deployed',      -- placed in rack but not yet serving traffic
        'active',        -- serving traffic (synonym for operational)
        'operational',   -- serving traffic (canonical)
        'maintenance',   -- temporarily offline for maintenance
        'retired',       -- removed from service but still tracked
        'disposed'       -- physically destroyed / decommissioned
    ));

-- ---------------------------------------------------------------------------
-- work_orders.status — add 'cancelled' to 028's set.
-- ---------------------------------------------------------------------------
ALTER TABLE work_orders DROP CONSTRAINT IF EXISTS chk_work_orders_status;
ALTER TABLE work_orders ADD CONSTRAINT chk_work_orders_status
    CHECK (status IN (
        'draft',         -- authoring, not yet submitted
        'submitted',     -- awaiting approval
        'approved',      -- approved, not yet started
        'rejected',      -- rejected in governance
        'in_progress',   -- work actively underway
        'completed',     -- work finished, awaiting verification
        'verified',      -- verified complete
        'cancelled'      -- aborted before completion
    ));

-- ---------------------------------------------------------------------------
-- discovered_assets.status — NEW constraint. Wave 3 uses the 5 states below.
-- ---------------------------------------------------------------------------
ALTER TABLE discovered_assets DROP CONSTRAINT IF EXISTS chk_discovered_assets_status;
ALTER TABLE discovered_assets ADD CONSTRAINT chk_discovered_assets_status
    CHECK (status IN (
        'pending',       -- awaiting review
        'matched',       -- auto-matched to existing CI (Wave 3)
        'conflict',      -- attribute collision with existing CI (Wave 3)
        'approved',      -- reviewed + merged into assets
        'ignored'        -- reviewed + rejected
    ));

-- ---------------------------------------------------------------------------
-- alert_events.status — add 'silenced'.
-- ---------------------------------------------------------------------------
ALTER TABLE alert_events DROP CONSTRAINT IF EXISTS chk_alert_events_status;
ALTER TABLE alert_events ADD CONSTRAINT chk_alert_events_status
    CHECK (status IN (
        'firing',        -- condition currently met
        'acknowledged',  -- operator saw it but condition still active
        'silenced',      -- suppressed during maintenance window
        'resolved'       -- condition cleared
    ));

-- ---------------------------------------------------------------------------
-- inventory_tasks.status — add 'cancelled'.
-- ---------------------------------------------------------------------------
ALTER TABLE inventory_tasks DROP CONSTRAINT IF EXISTS chk_inventory_tasks_status;
ALTER TABLE inventory_tasks ADD CONSTRAINT chk_inventory_tasks_status
    CHECK (status IN (
        'planned',       -- scheduled but not started
        'in_progress',   -- scan actively underway
        'completed',     -- scan finished, ready for reconciliation
        'cancelled'      -- aborted
    ));

COMMIT;
