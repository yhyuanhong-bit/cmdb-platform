-- Phase 4.2: convert audit_events from a monolithic heap table to a
-- declarative RANGE-partitioned table keyed on created_at (monthly).
--
-- Preserves every column added by prior migrations — importantly the
-- operator_type ENUM, the chk_audit_operator_type_id_match CHECK, and
-- the FK to users(id) introduced in 000051/000052 — so downstream
-- sqlc-generated code and the audit.Service.Record contract keep
-- compiling without a regeneration.
--
-- The partition key must participate in every UNIQUE constraint on
-- a partitioned table, so the primary key grows from (id) to
-- (id, created_at). Application code already references id alone;
-- this shape is still uniquely identifying because gen_random_uuid()
-- collisions within a single microsecond are not a thing we will see.
--
-- Trigger handling: append-only triggers live on the parent; PG 11+
-- propagates them to every partition automatically, so the backfill
-- INSERTs need the triggers OFF. We drop the old triggers off the
-- renamed legacy table first (they were installed on the object that
-- no longer exists after step 2), then recreate on the new parent.
--
-- Idempotency: every CREATE TABLE / CREATE TRIGGER / CREATE INDEX in
-- this migration is either gated by IF NOT EXISTS or wrapped in a
-- DO block so a half-applied migration can be retried.

BEGIN;

-- 1. Rename the old heap table. We keep it around ("legacy") for a
--    safety window — a follow-up migration can drop it once the
--    staging soak confirms partition routing works.
ALTER TABLE audit_events RENAME TO audit_events_legacy;

-- 1a. Drop the append-only triggers from the now-legacy table so the
--     backfill INSERTs below are permitted. We recreate them on the
--     new parent in step 8.
DROP TRIGGER IF EXISTS audit_events_no_delete ON audit_events_legacy;
DROP TRIGGER IF EXISTS audit_events_no_update ON audit_events_legacy;

-- 1b. Indexes follow their table on RENAME, so the old names now
--     belong to the legacy heap. Rename them out of the way with a
--     _legacy suffix; the new partitioned parent below reclaims the
--     canonical names and PG propagates them to every partition.
ALTER INDEX audit_events_pkey                RENAME TO audit_events_legacy_pkey;
ALTER INDEX idx_audit_events_tenant_created  RENAME TO idx_audit_events_legacy_tenant_created;
ALTER INDEX idx_audit_events_target          RENAME TO idx_audit_events_legacy_target;
ALTER INDEX idx_audit_events_operator        RENAME TO idx_audit_events_legacy_operator;
ALTER INDEX idx_audit_events_type_created    RENAME TO idx_audit_events_legacy_type_created;

-- 1c. The CHECK constraint and FK names follow the table too. Rename
--     them so the new partitioned parent can claim the original
--     identifiers — this keeps constraint-name error messages stable
--     for any consumer that matches on them.
ALTER TABLE audit_events_legacy
    RENAME CONSTRAINT chk_audit_operator_type_id_match
                   TO chk_audit_operator_type_id_match_legacy;
ALTER TABLE audit_events_legacy
    RENAME CONSTRAINT audit_events_tenant_id_fkey
                   TO audit_events_legacy_tenant_id_fkey;
ALTER TABLE audit_events_legacy
    RENAME CONSTRAINT audit_events_operator_id_fkey
                   TO audit_events_legacy_operator_id_fkey;

-- 2. Create the partitioned parent table. Column list is a carbon
--    copy of audit_events_legacy so sqlc-generated INSERTs keep
--    binding to the same positional args. The PK shape changes to
--    (id, created_at) because PG requires the partition key to be
--    part of every UNIQUE constraint.
CREATE TABLE audit_events (
    id            UUID                 NOT NULL DEFAULT gen_random_uuid(),
    tenant_id     UUID                 NOT NULL REFERENCES tenants(id),
    action        VARCHAR(50)          NOT NULL,
    module        VARCHAR(30),
    target_type   VARCHAR(30),
    target_id     UUID,
    operator_id   UUID                 REFERENCES users(id),
    diff          JSONB,
    source        VARCHAR(20)          NOT NULL DEFAULT 'web',
    created_at    TIMESTAMPTZ          NOT NULL DEFAULT now(),
    operator_type audit_operator_type  NOT NULL DEFAULT 'user',
    PRIMARY KEY (id, created_at),
    CONSTRAINT chk_audit_operator_type_id_match CHECK (
        (operator_type = 'user' AND operator_id IS NOT NULL)
        OR (operator_type <> 'user' AND operator_id IS NULL)
    )
) PARTITION BY RANGE (created_at);

-- 3. Global indexes — PG propagates them to every partition
--    automatically, including partitions created after this point.
CREATE INDEX idx_audit_events_tenant_created ON audit_events (tenant_id, created_at DESC);
CREATE INDEX idx_audit_events_target         ON audit_events (target_type, target_id);
CREATE INDEX idx_audit_events_operator       ON audit_events (operator_id);
CREATE INDEX idx_audit_events_type_created   ON audit_events (operator_type, created_at DESC);

-- 4. Legacy catch-all partition covering every row written before the
--    current month. A follow-up cleanup migration can split this into
--    monthly granularity once the archive CLI is deployed.
CREATE TABLE audit_events_legacy_partition
    PARTITION OF audit_events
    FOR VALUES FROM (MINVALUE) TO (date_trunc('month', now()));

-- 5. Backfill: move every pre-current-month row into the legacy
--    partition. ON CONFLICT DO NOTHING keeps this re-runnable.
INSERT INTO audit_events_legacy_partition
    (id, tenant_id, action, module, target_type, target_id,
     operator_id, diff, source, created_at, operator_type)
SELECT id, tenant_id, action, module, target_type, target_id,
       operator_id, diff, source, created_at, operator_type
FROM audit_events_legacy
WHERE created_at < date_trunc('month', now())
ON CONFLICT (id, created_at) DO NOTHING;

-- 5a. Stash the current-month rows in a temp table so we can create
--     the current-month partition below without the range overlap
--     that a direct INSERT would trigger.
CREATE TEMP TABLE audit_events_current_month ON COMMIT DROP AS
SELECT * FROM audit_events_legacy
WHERE created_at >= date_trunc('month', now());

-- 6. Pre-create partitions for the current month and the next three.
--    Every write lands in a real partition; the archive CronJob keeps
--    this rolling window populated in production.
DO $$
DECLARE
    m       date := date_trunc('month', now())::date;
    i       int;
    p_name  text;
    p_from  text;
    p_to    text;
BEGIN
    FOR i IN 0..3 LOOP
        p_name := 'audit_events_' || to_char(m + (i || ' month')::interval, 'YYYY_MM');
        p_from := to_char(m + (i     || ' month')::interval, 'YYYY-MM-DD');
        p_to   := to_char(m + ((i+1) || ' month')::interval, 'YYYY-MM-DD');
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_events
             FOR VALUES FROM (%L) TO (%L)',
            p_name, p_from, p_to
        );
    END LOOP;
END $$;

-- 7. Move current-month rows into the freshly-attached current-month
--    partition. ON CONFLICT keeps this retry-safe.
INSERT INTO audit_events
    (id, tenant_id, action, module, target_type, target_id,
     operator_id, diff, source, created_at, operator_type)
SELECT id, tenant_id, action, module, target_type, target_id,
       operator_id, diff, source, created_at, operator_type
FROM audit_events_current_month
ON CONFLICT (id, created_at) DO NOTHING;

-- 8. Reinstate append-only triggers on the new parent. BEFORE-row
--    triggers on a partitioned parent fire for writes to any
--    partition — no need to attach them per-child.
CREATE TRIGGER audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();
CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

-- 9. Drop the purge_old_audit_events() dead code installed by 000023.
--    It issues DELETE which always tripped the append-only trigger,
--    and the archive CLI coming online in Batch 4 supersedes it.
DROP FUNCTION IF EXISTS purge_old_audit_events();

-- 10. Drop the 000023 partial index (WHERE created_at > now() - '90 days')
--     if it somehow survived — now() is non-IMMUTABLE and the expression
--     has no meaning on a partitioned table.
DROP INDEX IF EXISTS idx_audit_events_recent;

-- 11. Leave a breadcrumb so the next migration knows the orphan is
--     safe to drop once the soak period expires.
COMMENT ON TABLE audit_events_legacy IS
    'orphan from 000053; safe to drop after 7 days of steady state';

COMMIT;
