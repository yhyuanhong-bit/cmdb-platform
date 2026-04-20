-- ============================================================================
-- Cutover: rename ingestion-engine migration 000011 -> 000052
-- Related: /cmdb-platform/docs/reports/phase4/4.5-migration-number-registry.md
-- Created:  2026-04-20
-- ============================================================================
--
-- TODO: apply only after DBA sign-off.
--
-- WHAT THIS DOES
--   Updates the ingestion-engine's `schema_migrations` row that currently
--   records version=11 (for the file that was named
--   `000011_bmc_field_authorities.*.sql`) so that it matches the new file
--   name `000052_bmc_field_authorities.*.sql`. The .sql contents are
--   unchanged; only the file name / version integer changes.
--
-- WHY
--   Both `cmdb-core` and `ingestion-engine` had a migration numbered
--   000011 (cmdb-core: prediction_tables; ingestion-engine:
--   bmc_field_authorities). They currently live in two logically
--   separate `schema_migrations` tables (different services, different
--   connection pools), so the collision has not triggered a production
--   incident. But it violates the "same prefix = same meaning"
--   invariant that Phase 4.5 is introducing. Renaming to 000052 (the
--   next free slot per docs/MIGRATIONS.md) eliminates the collision at
--   the filename and registry layer.
--
-- APPLY THIS ONLY IF
--   The old `000011_bmc_field_authorities` migration was ALREADY APPLIED
--   to the database pointed at by the ingestion-engine service in this
--   environment. If it was NOT yet applied (fresh/preview env), skip
--   this file — the renamed file will run on the next `migrate up` with
--   no DB fixup required.
--
-- HOW TO CHECK
--   SELECT version, dirty FROM schema_migrations ORDER BY version;
--   If you see `version = 11` AND no `version = 52`, proceed.
--   If you see `version = 52` already, do nothing — someone ran it.
--   If you see `dirty = true`, STOP and investigate before touching.
--
-- ROLLBACK
--   BEGIN;
--     UPDATE schema_migrations SET version = 11 WHERE version = 52;
--   COMMIT;
--   (Plus `git revert` the rename commit.)
--
-- SAFETY
--   This script only updates metadata. It does NOT touch
--   `asset_field_authorities` row data. There is no risk of losing
--   authority records. The migration file content is byte-identical
--   before and after rename.
-- ============================================================================

BEGIN;

-- 1. Snapshot current state for the operator's benefit.
SELECT version, dirty FROM schema_migrations ORDER BY version;

-- 2. Defensive assertion: refuse to proceed if the database is dirty.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM schema_migrations WHERE dirty) THEN
        RAISE EXCEPTION 'schema_migrations has a dirty row — resolve before renaming';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 11) THEN
        RAISE EXCEPTION 'no row with version=11 found — this DB never ran the old 000011; skip cutover';
    END IF;
    IF EXISTS (SELECT 1 FROM schema_migrations WHERE version = 52) THEN
        RAISE EXCEPTION 'version=52 already exists — rename already applied, nothing to do';
    END IF;
END
$$;

-- 3. Flip the version integer.
UPDATE schema_migrations SET version = 52 WHERE version = 11;

-- 4. Re-read so the operator can confirm.
SELECT version, dirty FROM schema_migrations ORDER BY version;

COMMIT;

-- ============================================================================
-- After this completes, redeploy the ingestion-engine binary built from the
-- commit that contains the renamed file. `golang-migrate` will see
-- version=52 in both the table and on disk and do nothing — which is
-- the desired outcome.
-- ============================================================================
