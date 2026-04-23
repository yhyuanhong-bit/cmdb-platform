-- ============================================================================
-- Cutover: normalize alert_rules.condition to RuleCondition spec
-- Related: docs/decisions/2026-04-22-day-0.md (Wave 0, item #5)
-- Created: 2026-04-22
-- ============================================================================
--
-- WHAT THIS DOES
--   Converts non-conforming condition JSONB rows to the RuleCondition shape
--   the evaluator expects:
--     { operator, threshold, window_seconds, aggregation, consecutive_triggers }
--
--   Old (broken) shape:
--     { "op": ">", "threshold": 85 }
--
--   New (conforming) shape:
--     { "operator": ">", "threshold": 85,
--       "window_seconds": 300, "aggregation": "avg",
--       "consecutive_triggers": 2 }
--
-- WHY
--   Server startup logs were spamming
--     "alert evaluator: skipping rule with malformed condition ... invalid operator """
--   for 8 seed rules. The evaluator silently skipped them, meaning ALL 5
--   default tenant alert rules + 3 sync-test rules were inert. Production
--   monitoring was effectively off for any rule using this format.
--
-- DEFAULTS CHOSEN
--   window_seconds: 300 (5 min) for warning, 60 (1 min) for critical
--   aggregation:    avg for warning, max for critical
--   consecutive_triggers: 2 for warning (debounce), 1 for critical (fast)
--
-- IDEMPOTENT
--   Re-running this is safe: rows already containing "operator" key are
--   skipped. Only rows with the legacy "op" key get rewritten.
--
-- ROLLBACK
--   BEGIN;
--     UPDATE alert_rules SET condition = jsonb_build_object(
--       'op', condition->>'operator',
--       'threshold', (condition->>'threshold')::numeric)
--     WHERE condition ? 'operator' AND id IN (
--       '40000000-0000-0000-0000-000000000001',
--       ... -- 8 specific IDs from the snapshot below
--     );
--   COMMIT;
-- ============================================================================

BEGIN;

-- 1. Snapshot before — operators can verify which rows are about to change.
SELECT id, name, condition
FROM alert_rules
WHERE NOT (condition ? 'operator')
ORDER BY created_at;

-- 2. Defensive assertion: confirm we're touching only the expected rows.
DO $$
DECLARE
    bad_count INT;
BEGIN
    SELECT count(*) INTO bad_count FROM alert_rules WHERE NOT (condition ? 'operator');
    IF bad_count = 0 THEN
        RAISE EXCEPTION 'no rows to migrate — already cut over, exiting cleanly';
    END IF;
    RAISE NOTICE 'migrating % alert_rule rows', bad_count;
END
$$;

-- 3. Migrate. Severity-driven defaults so debounce matches old expectations.
UPDATE alert_rules
SET condition = jsonb_build_object(
    'operator',
        COALESCE(condition->>'op', condition->>'operator', '>'),
    'threshold',
        COALESCE((condition->>'threshold')::numeric, 0),
    'window_seconds',
        CASE severity
            WHEN 'critical' THEN 60
            WHEN 'warning'  THEN 300
            ELSE 300
        END,
    'aggregation',
        CASE severity
            WHEN 'critical' THEN 'max'
            ELSE 'avg'
        END,
    'consecutive_triggers',
        CASE severity
            WHEN 'critical' THEN 1
            ELSE 2
        END
)
WHERE NOT (condition ? 'operator');

-- 4. Verify — every rule should now parse cleanly.
SELECT
    id, name, severity,
    condition->>'operator'        AS operator,
    condition->>'threshold'       AS threshold,
    condition->>'window_seconds'  AS window_seconds,
    condition->>'aggregation'     AS aggregation,
    condition->>'consecutive_triggers' AS consecutive_triggers
FROM alert_rules
ORDER BY tenant_id, name;

-- 5. Confirm no malformed rows remain.
DO $$
DECLARE
    bad_count INT;
BEGIN
    SELECT count(*) INTO bad_count
    FROM alert_rules
    WHERE NOT (condition ? 'operator')
       OR condition->>'operator' = ''
       OR NOT (condition->'operator' ? 'string' OR condition->>'operator' IN ('>','<','>=','<=','==','!='));
    IF bad_count > 0 THEN
        RAISE EXCEPTION 'still % malformed rows after migration — abort', bad_count;
    END IF;
END
$$;

COMMIT;

-- ============================================================================
-- After this completes, restart cmdb-core (or wait for the next evaluator
-- tick) and confirm logs no longer contain "invalid operator". Alert rules
-- will become active for the first time since seed data shipped.
-- ============================================================================
