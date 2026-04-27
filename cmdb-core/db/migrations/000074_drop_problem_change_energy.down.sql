-- 000074_drop_problem_change_energy.down.sql
--
-- Restoring problem / change / energy-billing tables means re-running the
-- original up migrations 000066 / 000067 / 000069 / 000070. Re-creating
-- them inline here would duplicate ~400 lines of schema; in practice if
-- you need to roll back you should:
--
--   migrate -path db/migrations -database "$DATABASE_URL" goto 70
--
-- which downgrades to the state before 000071. This file is intentionally
-- a no-op so `migrate down 1` from 74 → 73 doesn't crash, but it does
-- NOT restore the dropped tables. RBAC grants are also left as-is.

SELECT 1;
