-- ============================================================================
-- 000062: Status field CHECK constraints
-- ============================================================================
-- Locks the set of allowed values for every mutable status column we care
-- about in a CMDB. Before this migration, status fields were raw VARCHAR —
-- typos, stale values from old code paths, or scripts that bypassed the
-- Go layer could all pollute the table. That's how we accumulated things
-- like alert_rules.condition.op vs .operator (see cutover
-- 2026-04-22-alert-rule-condition-format.sql).
--
-- Allowlist derivation — each constraint covers:
--   (a) every value currently present in production data (audited
--       2026-04-23 against running DB);
--   (b) canonical values the Go code / statemachine references; and
--   (c) near-term values planned by subsequent roadmap waves.
-- Values outside the allowlist are rejected at INSERT/UPDATE time.
--
-- Drift not resolved here — this migration intentionally accepts
-- synonymous values (e.g. assets.status has both 'active' and
-- 'operational'). Normalization to a single canonical value is a
-- separate Wave-3+ project with user-facing implications. This
-- migration just prevents NEW drift.

BEGIN;

-- ---------------------------------------------------------------------------
-- assets.status
-- ---------------------------------------------------------------------------
-- In use (2026-04-23): active, deployed, inventoried, maintenance,
-- operational, retired.
-- Canonical (future): planned, in_stock, deployed, operational,
-- maintenance, retired, disposed. 'active' is a synonym for operational
-- kept for compatibility.
ALTER TABLE assets
    ADD CONSTRAINT chk_assets_status
    CHECK (status IN (
        'planned',       -- procured but not yet received
        'in_stock',      -- received, in warehouse
        'inventoried',   -- physically located, pre-deployment
        'deployed',      -- placed in rack but not yet serving traffic
        'active',        -- serving traffic (synonym for operational)
        'operational',   -- serving traffic (canonical)
        'maintenance',   -- temporarily offline for maintenance
        'retired',       -- removed from service but still tracked
        'disposed'       -- physically destroyed / decommissioned
    ));

-- ---------------------------------------------------------------------------
-- work_orders.status (governance + execution state machine collapsed view)
-- ---------------------------------------------------------------------------
-- In use (2026-04-23): approved, completed, draft, in_progress, rejected,
-- submitted, verified.
-- Canonical (see internal/domain/maintenance/statemachine.go): submitted,
-- approved, rejected, in_progress, completed, verified. 'draft' is used
-- for pre-submission authoring. 'cancelled' added for product need.
ALTER TABLE work_orders
    ADD CONSTRAINT chk_work_orders_status
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
-- discovered_assets.status (Discovery review gate, Wave 3)
-- ---------------------------------------------------------------------------
-- In use (2026-04-23): approved, ignored, pending.
-- Wave 3 adds: conflict, matched (auto-matched before review).
ALTER TABLE discovered_assets
    ADD CONSTRAINT chk_discovered_assets_status
    CHECK (status IN (
        'pending',       -- awaiting review
        'matched',       -- auto-matched to existing CI (Wave 3)
        'conflict',      -- attribute collision with existing CI (Wave 3)
        'approved',      -- reviewed + merged into assets
        'ignored'        -- reviewed + rejected
    ));

-- ---------------------------------------------------------------------------
-- alert_events.status
-- ---------------------------------------------------------------------------
-- In use (2026-04-23): acknowledged, firing, resolved.
-- 'silenced' added for standard alert workflow support.
ALTER TABLE alert_events
    ADD CONSTRAINT chk_alert_events_status
    CHECK (status IN (
        'firing',        -- condition currently met
        'acknowledged',  -- operator saw it but condition still active
        'silenced',      -- suppressed during maintenance window
        'resolved'       -- condition cleared
    ));

-- ---------------------------------------------------------------------------
-- inventory_tasks.status
-- ---------------------------------------------------------------------------
-- In use (2026-04-23): completed, planned.
-- in_progress added for the common intermediate state; cancelled for abort.
ALTER TABLE inventory_tasks
    ADD CONSTRAINT chk_inventory_tasks_status
    CHECK (status IN (
        'planned',       -- scheduled but not started
        'in_progress',   -- scan actively underway
        'completed',     -- scan finished, ready for reconciliation
        'cancelled'      -- aborted
    ));

COMMIT;
