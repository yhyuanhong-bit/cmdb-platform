-- ============================================================================
-- clear_integration_plaintext.sql
--
-- Purpose: Clear plaintext integration secrets once dual-write + backfill has
--          fully populated the encrypted columns. Runs as a single transaction
--          with a guard that aborts if any row still lacks ciphertext.
--
-- This is NOT an auto-applied migration. It lives under db/scripts/ deliberately
-- so main.go's migration loop never picks it up. Run it manually AFTER:
--   1. Migration 000038 has been applied (both columns exist).
--   2. CMDB_SECRET_KEY has been stable for enough time that every new write
--      has produced a ciphertext (at least one full dual-write release cycle).
--   3. You have backfilled any historical rows (see
--      docs/integration-encryption-deployment.md section 7).
--   4. You have a fresh pg_dump backup.
--
-- Idempotent: rerunning after plaintext is already cleared is a no-op.
--
-- Usage:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
--     -f cmdb-core/db/scripts/cleanup/clear_integration_plaintext.sql
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Pre-flight: refuse to proceed if any row would lose data.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    unencrypted_adapters INT;
    unencrypted_webhooks INT;
BEGIN
    SELECT COUNT(*) INTO unencrypted_adapters
    FROM integration_adapters
    WHERE config IS NOT NULL
      AND config::text <> '{}'
      AND config_encrypted IS NULL;

    IF unencrypted_adapters > 0 THEN
        RAISE EXCEPTION
            'Aborting: % integration_adapters row(s) have plaintext config but no ciphertext. Run the encryption backfill task before clearing plaintext.',
            unencrypted_adapters;
    END IF;

    SELECT COUNT(*) INTO unencrypted_webhooks
    FROM webhook_subscriptions
    WHERE secret IS NOT NULL
      AND secret <> ''
      AND secret_encrypted IS NULL;

    IF unencrypted_webhooks > 0 THEN
        RAISE EXCEPTION
            'Aborting: % webhook_subscriptions row(s) have plaintext secret but no ciphertext. Run the encryption backfill task before clearing plaintext.',
            unencrypted_webhooks;
    END IF;

    RAISE NOTICE 'Pre-flight passed: all non-empty plaintext rows have matching ciphertext.';
END $$;

-- ---------------------------------------------------------------------------
-- Clear adapter config plaintext. Reset to default '{}' rather than NULL so
-- the column semantics (JSONB, DEFAULT '{}') stay consistent with writes from
-- older code paths that never populate it explicitly.
-- ---------------------------------------------------------------------------
WITH cleared AS (
    UPDATE integration_adapters
    SET config = '{}'::jsonb
    WHERE config_encrypted IS NOT NULL
      AND config::text <> '{}'
    RETURNING id
)
SELECT COUNT(*) AS adapters_cleared FROM cleared
\gset

-- ---------------------------------------------------------------------------
-- Clear webhook secret plaintext. NULL here is fine because the column is
-- nullable and the default is NULL (no secret = no HMAC signing).
-- ---------------------------------------------------------------------------
WITH cleared AS (
    UPDATE webhook_subscriptions
    SET secret = NULL
    WHERE secret_encrypted IS NOT NULL
      AND secret IS NOT NULL
    RETURNING id
)
SELECT COUNT(*) AS webhooks_cleared FROM cleared
\gset

-- ---------------------------------------------------------------------------
-- Record the cleanup for audit. Uses a dedicated action so operators can grep
-- for it later. tenant_id is NULL because this is a system-wide operation;
-- audit_events.tenant_id is nullable via 000031_audit_system_events.up.sql
-- (confirm before running — if your deployment has a stricter schema, comment
-- out this INSERT).
-- ---------------------------------------------------------------------------
INSERT INTO audit_events (tenant_id, operator_id, action, module, target_type, diff, source)
VALUES (
    NULL,
    NULL,
    'integration_plaintext_cleared',
    'integration',
    'system',
    jsonb_build_object(
        'adapters_cleared', :adapters_cleared,
        'webhooks_cleared', :webhooks_cleared,
        'script_version',   '1'
    ),
    'admin-script'
);

COMMIT;

-- Report to the operator's stdout.
\echo ''
\echo '=== Cleanup complete ==='
\echo 'adapters_cleared:' :adapters_cleared
\echo 'webhooks_cleared:' :webhooks_cleared
