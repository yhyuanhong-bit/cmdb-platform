-- Phase 2.15: explicit dedup table for auto-generated work orders.
--
-- Replaces the prior `description LIKE '%IP:xxx%' / '%Serial:xxx%'` probe
-- used in checkShadowITForTenant and checkDuplicateSerialsForTenant to
-- skip candidates that already produced a WO. That probe was brittle:
--
--  * It seq-scanned work_orders every scan tick (no index on description).
--  * It matched substrings, so the IP "10.0.0.5" false-matched on any
--    description that happened to embed "10.0.0.5" elsewhere (e.g. the
--    dup-serial WO's asset list). Cross-kind collisions were possible.
--  * Unicode / whitespace variations in description formatting silently
--    broke dedup across releases — the key was free-form prose, not a
--    structured identifier.
--  * It had no cross-tenant guarantee on its own; only the WO.tenant_id
--    filter enforced isolation.
--
-- This table makes the dedup contract explicit and indexed, with a
-- compound primary key doing double duty as the uniqueness constraint
-- and the lookup index.

CREATE TABLE IF NOT EXISTS work_order_dedup (
    tenant_id     UUID NOT NULL,
    work_order_id UUID NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    dedup_kind    TEXT NOT NULL,   -- e.g. 'shadow_it', 'duplicate_serial'
    dedup_key     TEXT NOT NULL,   -- e.g. '10.0.0.5' or 'SN-ABC123'
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, dedup_kind, dedup_key)
);

-- Reverse lookup for the ON DELETE CASCADE path and for ops queries that
-- need "which dedup entries point at this WO?".
CREATE INDEX IF NOT EXISTS idx_work_order_dedup_work_order
    ON work_order_dedup (work_order_id);

-- Backfill from historical WOs so current-state dedup stays consistent
-- across the code swap. We parse the IP / serial out of the description
-- prose the previous code wrote. Anything that doesn't match the pattern
-- is skipped (NULL key) — those WOs stay ambiguous but the new code path
-- will simply create a second WO on re-scan, which is strictly better
-- than the prior silent miss-match.
--
-- ON CONFLICT DO NOTHING guards against a re-run of the migration (e.g.
-- in a staging replay) and against duplicate descriptions generating the
-- same key twice.

-- shadow_it: description format is "... (IP: 10.0.0.5) ..."
INSERT INTO work_order_dedup (tenant_id, work_order_id, dedup_kind, dedup_key)
SELECT tenant_id,
       id,
       'shadow_it',
       substring(description from 'IP: ([^ )]+)')
  FROM work_orders
 WHERE type = 'shadow_it_registration'
   AND description IS NOT NULL
   AND substring(description from 'IP: ([^ )]+)') IS NOT NULL
ON CONFLICT (tenant_id, dedup_kind, dedup_key) DO NOTHING;

-- duplicate_serial: description format is "Serial number 'SN-ABC123' appears on ..."
-- The prior code emitted "Serial number '%s' ...", so the quote style is
-- the stable delimiter. Fallback to "Serial: X" for any alternate phrasing
-- that might have slipped in historically.
INSERT INTO work_order_dedup (tenant_id, work_order_id, dedup_kind, dedup_key)
SELECT tenant_id,
       id,
       'duplicate_serial',
       COALESCE(
           substring(description from 'Serial number ''([^'']+)'''),
           substring(description from 'Serial: ([^ )]+)')
       )
  FROM work_orders
 WHERE type = 'dedup_audit'
   AND description IS NOT NULL
   AND COALESCE(
           substring(description from 'Serial number ''([^'']+)'''),
           substring(description from 'Serial: ([^ )]+)')
       ) IS NOT NULL
ON CONFLICT (tenant_id, dedup_kind, dedup_key) DO NOTHING;
