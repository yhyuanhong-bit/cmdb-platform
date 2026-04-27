-- 000074_drop_problem_change_energy.up.sql
--
-- 2026-04-27 scope reduction: refocusing the platform on CMDB core
-- (assets + presentation) plus a thin RCA panel. Dropping ITIL Problem
-- Management, ITIL Change Management (with CAB voting), and the energy
-- billing / PUE / anomaly subsystem — each was built in earlier waves
-- without real production usage.
--
-- The base energy ingest (power_kw metrics + GetEnergyBreakdown / Summary
-- / Trend endpoints used by the Energy dashboard page) is INTENTIONALLY
-- KEPT — only the billing/tariff/PUE/anomaly tables are dropped.

-- Change Management (Wave 5.3) — drop in FK reverse order.
DROP TABLE IF EXISTS change_problems;
DROP TABLE IF EXISTS change_services;
DROP TABLE IF EXISTS change_assets;
DROP TABLE IF EXISTS change_comments;
DROP TABLE IF EXISTS change_approvals;
DROP TABLE IF EXISTS changes;

-- Problem Management (Wave 5.2) — also reverse FK order.
DROP TABLE IF EXISTS incident_problem_links;
DROP TABLE IF EXISTS problem_comments;
DROP TABLE IF EXISTS problems;

-- Energy billing + PUE + anomalies (Waves 6.1, 6.2).
DROP TABLE IF EXISTS energy_anomalies;
DROP TABLE IF EXISTS energy_location_daily;
DROP TABLE IF EXISTS energy_daily_kwh;
DROP TABLE IF EXISTS energy_tariffs;

-- Strip RBAC permissions for the dropped resources (added in 000067 / 000069 / 000070).
UPDATE roles
   SET permissions = permissions - 'problems' - 'changes' - 'energy_billing' - 'energy_phase2'
 WHERE permissions ?| array['problems','changes','energy_billing','energy_phase2'];
