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
-- Clear plaintext, aggregate per-tenant counts, and emit one audit_events row
-- per affected tenant in a single CTE. audit_events.tenant_id is NOT NULL
-- with an FK to tenants(id), so a cross-tenant summary row is not
-- representable — each tenant gets an event scoped to its own data.
--
-- Adapter config is reset to '{}' (the column default) rather than NULL so
-- JSONB semantics match rows written by code paths that never populated
-- config explicitly. Webhook secret is set to NULL (the column default; no
-- secret = no HMAC signing).
-- ---------------------------------------------------------------------------
WITH
    cleared_adapters AS (
        UPDATE integration_adapters
        SET config = '{}'::jsonb
        WHERE config_encrypted IS NOT NULL
          AND config::text <> '{}'
        RETURNING tenant_id
    ),
    cleared_webhooks AS (
        UPDATE webhook_subscriptions
        SET secret = NULL
        WHERE secret_encrypted IS NOT NULL
          AND secret IS NOT NULL
        RETURNING tenant_id
    ),
    a_counts AS (
        SELECT tenant_id, COUNT(*)::bigint AS n
        FROM cleared_adapters GROUP BY tenant_id
    ),
    w_counts AS (
        SELECT tenant_id, COUNT(*)::bigint AS n
        FROM cleared_webhooks GROUP BY tenant_id
    ),
    per_tenant AS (
        SELECT COALESCE(a.tenant_id, w.tenant_id) AS tenant_id,
               COALESCE(a.n, 0) AS adapters_cleared,
               COALESCE(w.n, 0) AS webhooks_cleared
        FROM a_counts a
        FULL OUTER JOIN w_counts w USING (tenant_id)
    ),
    audit_inserted AS (
        INSERT INTO audit_events (tenant_id, action, module, target_type, diff, source)
        SELECT
            tenant_id,
            'integration_plaintext_cleared',
            'integration',
            'system',
            jsonb_build_object(
                'adapters_cleared', adapters_cleared,
                'webhooks_cleared', webhooks_cleared,
                'script_version',   '1'
            ),
            'admin-script'
        FROM per_tenant
        RETURNING diff
    )
SELECT
    COALESCE(SUM((diff->>'adapters_cleared')::bigint), 0) AS adapters_cleared,
    COALESCE(SUM((diff->>'webhooks_cleared')::bigint), 0) AS webhooks_cleared
FROM audit_inserted
\gset

COMMIT;

-- Report to the operator's stdout.
\echo ''
\echo '=== Cleanup complete ==='
\echo 'adapters_cleared:' :adapters_cleared
\echo 'webhooks_cleared:' :webhooks_cleared
