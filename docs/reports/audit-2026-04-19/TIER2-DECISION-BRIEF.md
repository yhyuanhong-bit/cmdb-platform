# Tier 2 Stakeholder Decision Brief — Phase 4.2 / 4.7 / 4.8

> **Scope:** three items from the Phase 4 remediation roadmap that cannot proceed without product / security / compliance sign-off. Target: single 60-minute review meeting yields binding decisions for all three.
>
> **Date:** 2026-04-20
> **Facilitator:** backend-architect
> **Required attendees:** Product, Architecture, Security, Compliance, SRE/DBA, Frontend Lead
> **Source reports (read before meeting):**
> - `docs/reports/phase4/4.2-audit-monthly-partition-and-archival.md`
> - `docs/reports/phase4/4.7-dashboard-expansion-and-invalidation.md`
> - `docs/reports/phase4/4.8-operator-id-fk-design-spike.md`

---

## Executive summary (3-min read)

| # | Topic | Recommendation | Primary risk if deferred | Implementation cost |
|---|-------|----------------|-------------------------|---------------------|
| D1 | **4.8** `operator_id` FK violations | **Option C + mini-A2 hybrid** (ENUM discriminator + NULL operator_id on audit_events; per-tenant `system` user for work_orders/inventory) | Silent audit writes fail FK → audit gap → compliance exposure. Phase 3.1 already exposed the error, so future any swallow regression is visible to ops. | 8.5 person-days (≈ 2 weeks, 1 senior Go) |
| D2 | **4.2** audit retention period | **12 mo hot + 7 yr cold**, with per-tenant override column | `audit_events` grows unbounded; ~10 M rows/yr at 100 tenants; `VACUUM FULL` needs maintenance window; PITR bloat | 4-6 person-days (partition + archival CLI + cron) |
| D3 | **4.7** dashboard KPI semantics (3 fields) | (a) `power.current_w` canonical; (b) avg quality = mean of per-asset latest score; (c) rack utilization = slot-based (current SQL) | Diverging definitions across UI cards → "42% vs 43.8%" bug. Blocks 4.7 cache-invalidation work. | 3 person-days (inside 4.7's 2-week estimate) |

**Default path if no decision:** none of these items ships; 4.2 and 4.7 stay in Tier 3 quarterly backlog, 4.8 remains a latent FK-violation bug until Phase 3.1 swallow-cleanup completes — at which point audit write failures become user-visible 500s.

---

## D1 — Phase 4.8: `operator_id = uuid.Nil` FK violations

### Background (2 min)

Six tables reference `users(id)` via FK on "who did this" columns (`audit_events.operator_id`, `work_orders.requestor_id/assignee_id`, `work_order_logs.operator_id`, `inventory_tasks.assigned_to`, `inventory_items.scanned_by`). Today 17+ call sites pass `uuid.Nil` (the all-zero UUID) for system-initiated writes — Postgres rejects every one of them on FK check. The errors are currently silent because the calling workflow loops swallow them (see Phase 3.1). Once Phase 3.1 finishes, they will surface as 500s.

Phase 1.3 (migration 000050) made `users.username` unique per-tenant, so a **global** sentinel with `id = uuid.Nil` can no longer live cleanly in the schema — it would have to pick one tenant to own it, re-introducing the cross-tenant user row the Phase 1 work just eliminated.

### Option matrix

| Option | What changes | Schema impact | Code change blast radius | Auditability | Phase 1 tenant-isolation clean? |
|--------|--------------|---------------|--------------------------|--------------|-------------------------------|
| **A1** Global sentinel (`uuid.Nil` user) | Insert one "system" user with `id = uuid.Nil` in an arbitrary tenant | Pollutes the owner tenant | Zero at call-sites | "System" shows as one hard-coded name | **No** — leaks across tenants |
| **A2** Per-tenant sentinel (one `source='system'` user per tenant) | Migration 000052 seeds one row per tenant; tenant.created trigger seeds future tenants | `users.source` column already exists (`local`/`ldap`/…); add `'system'`; login guard rejects `source='system'` | Zero at call-sites (just swap `uuid.Nil` for cached `systemUserID[tenantID]`) | Good — audit row JOIN renders `System` with tenant context | Yes |
| **B** FK `DEFERRABLE INITIALLY DEFERRED` | Keep `uuid.Nil` writes; defer FK check | `ALTER TABLE … DEFERRABLE` on 6 FKs | Zero | No — FK is checked at commit, still fails | N/A, not a real fix |
| **C** ✅ ENUM discriminator + NULL operator_id | Add `operator_type audit_operator_type NOT NULL` ENUM (`user`/`system`/`integration`/`sync`/`anonymous`); allow `operator_id IS NULL` for non-user types; CHECK constraint binds the pair | New column + ENUM type + CHECK; no per-tenant seeding | Re-sign `audit.Service.Record` to take `operator_type`; 17 call-sites updated | Best — structured discriminator is queryable, reportable | Yes |

### Recommendation: **C for `audit_events`; mini-A2 for work_orders / inventory_tasks / inventory_items**

**Why hybrid:**
- `audit_events` is the compliance-critical table and is append-only — the ENUM type is worth the 17 call-site re-signing because queries like "show all system-initiated deletes" become `WHERE operator_type='system'` instead of a `JOIN users ON users.source='system'`.
- `work_orders.requestor_id` / `assignee_id` / inventory `assigned_to` etc. are **operational** columns, not audit — users want to see a human name in the UI, not a "(system)" badge. Seeding a per-tenant `source='system'` user (mini-A2) lets the existing UI JOIN render "System" without schema churn, and keeps work-order assignment queries (`WHERE assignee_id = $1`) working unchanged.

**Combined implementation cost:** 8.5 person-days (see report § 10). If stakeholders insist on uniformity (one option across all 6 tables), pure C is +1 day, pure A2 is -0.5 day.

### Questions requiring sign-off

| Q | Default recommendation | Who owns the call | Sign-off |
|---|----------------------|------------------|----------|
| **Q1.1** Adopt C (audit) + mini-A2 (work_orders/inventory) hybrid? | Yes | Architecture + Product | ☐ ☐ |
| **Q1.2** Initial ENUM values: `user / system / integration / sync / anonymous`. Add `ai_agent`, `cli`, `webhook` now or defer? | Defer `ai_agent`; fold `webhook` into `integration`; fold `cli` into `system`. Revisit when first AI agent ships. | Product | ☐ |
| **Q1.3** Work_orders.requestor_id — keep NULL-semantics UI ("System" label) or strictly require a human? | Allow NULL + UI "System" label — matches mini-A2 seeded user exactly | Product + Frontend | ☐ ☐ |
| **Q1.4** Add SLO `audit_write_failures_total{reason,module}` alongside this work? | Yes — pair with Phase 3.1 swallow cleanup; cost ≈ 0.25 person-days | SRE + Architecture | ☐ ☐ |
| **Q1.5** DBA to run baseline count `SELECT count(*) FROM audit_events WHERE operator_id = '00000000-0000-0000-0000-000000000000'::uuid` on staging + prod before migration? | Yes | DBA | ☐ |
| **Q1.6** Security sign-off that login rejects `source='system'` with generic `"authentication failed"` (no existence leak) | Yes | Security | ☐ |

**Deferred to implementation PR review (no meeting time needed):** cache TTL for `systemUserID` lookup (report § 12 P2-8), append-only trigger disable-window risk (§ 12 P2-6).

---

## D2 — Phase 4.2: audit retention period

### Background (2 min)

`audit_events` has no retention today. `purge_old_audit_events()` was shipped in migration 000023 but is **dead code** — migration 000028 added an append-only trigger that rejects its `DELETE`. At current sizing (~300 events/day/tenant, ~2.5 KB/row including JSONB diff), 100 tenants yield ~40 GB/yr in the hot path. Phase 4.2 proposes month-RANGE declarative partitioning + monthly cron that DETACHes + exports to S3 Parquet + DROPs expired partitions.

### Decision required: retention window(s)

The partitioning mechanism is not controversial; the **policy** is. Three commonly-requested compliance horizons:

| Policy | Hot (Postgres) | Cold (S3 Parquet, Object-Lock compliance) | Who asks for it |
|--------|----------------|-------------------------------------------|-----------------|
| **P-min** | 6 months | 6 months total (no cold) | China 等保 Level 2 (baseline) |
| **P-rec** ✅ | **12 months** | **7 years** cold (S3 Object Lock compliance mode) | SOX-adjacent customers, most DC operators |
| **P-max** | 24 months | 10 years cold | Healthcare / regulated finance customers |

### Recommendation: **P-rec (12 mo hot + 7 yr cold) with per-tenant override column**

**Why:**
- 12 mo hot = two quarters of "easy UI drill-down" + full year of trend analysis, keeps DB under ~40 GB/tenant at 100 tenants.
- 7 yr cold = SOX/GDPR superset; S3 DEEP_ARCHIVE at $0.00099/GB-mo means 40 GB × 84 mo ≈ $3/yr/tenant. Trivial.
- Per-tenant override (`tenants.audit_retention_months`, nullable, default uses policy) lets enterprise customers ask for 10 yr without a code change. CronJob takes `MAX(cutoff)` so no tenant loses data early.

### Questions requiring sign-off

| Q | Default recommendation | Who owns the call | Sign-off |
|---|----------------------|------------------|----------|
| **Q2.1** Hot retention = 12 months? | Yes | Product + Compliance | ☐ ☐ |
| **Q2.2** Cold retention = 7 years in S3? | Yes | Compliance + Security | ☐ ☐ |
| **Q2.3** Add `tenants.audit_retention_months` nullable column for per-tenant override? | Yes | Product | ☐ |
| **Q2.4** Cold storage format: Parquet (column store, 60% smaller, needs `parquet-go` dep) vs CSV.gz (simpler) | Parquet | Architecture | ☐ |
| **Q2.5** Per-tenant vs flat S3 prefix layout (`audit/YYYY/MM/tenant=<uuid>/`)? | Per-tenant, with automatic consolidation for tenants < 1000 rows/month | Compliance + SRE | ☐ ☐ |
| **Q2.6** Delete the dead `purge_old_audit_events()` function as part of migration 000051? | Yes | DBA | ☐ |

**Deferred to implementation PR review:** sqlc regen diff verification (report § 7), `audit_events_legacy` 7-day observation window reminder location (runbook vs CI).

---

## D3 — Phase 4.7: dashboard KPI semantics (3 new fields)

### Background (1 min)

Phase 4.7 adds 4 new fields to `GET /api/v1/dashboard/stats` (`EnergyCurrentKW`, `RackUtilizationPct`, `PendingWorkOrders`, `AvgQualityScore`) and replaces the 60 s TTL with domain-event-driven Redis DEL. The **mechanism** is settled (service-level aggregation + `workflows.bus` subscribers). Three **semantics** questions need stakeholder alignment so the SQL doesn't get rewritten after launch.

### The three semantics questions

**S1 — `EnergyCurrentKW`: canonical metric name**

The `metrics` table mixes Prometheus (`node_*`), Zabbix (custom keys), and custom REST keys. The SQL needs one canonical name to sum.

| Candidate | Source | Unit |
|-----------|--------|------|
| `power.current_w` ✅ | Prometheus `node_power_current_watts` + custom REST | W (scaled to kW in app) |
| `power.draw_watts` | Zabbix legacy key | W |
| `energy.current` | Old custom schema | kW (mixed) |

**Recommendation:** `power.current_w`. Integration team ports Zabbix + legacy custom keys to this name in a separate data-migration PR (not blocking).

**S2 — `AvgQualityScore`: aggregation window & method**

| Candidate | Behavior | Robustness |
|-----------|----------|------------|
| **Mean of per-asset latest score** ✅ | One row per asset, take newest score, average | Simple, matches existing quality UI |
| Mean over last 24 h of scores | Rolling window, smoother | Harder to reproduce in audit |
| Median of per-asset latest score | Outlier-robust | Surprising in small-tenant demos |

**Recommendation:** mean of per-asset latest. If outliers become a real issue, switch to trimmed-mean (drop top/bottom 5%) in a follow-up without changing field name.

**S3 — `RackUtilizationPct`: weighting**

| Candidate | Formula | Note |
|-----------|---------|------|
| **Slot-based** ✅ | `SUM(occupied_slots) / SUM(total_slots)` | Current SQL; a 2U asset correctly counts as 2 |
| U-weighted | `SUM(slot_u) / SUM(rack.total_u)` | Requires `slot_u` column on every slot; matches datacenter-ops convention |
| Count-based | `COUNT(occupied) / COUNT(total)` | Wrong for multi-U assets (under-reports density) |

**Recommendation:** slot-based. Every slot already stores its size, so this already does what U-weighted would do — the two formulas converge when all slots are 1U, which is the sensor/asset model we ship.

### Questions requiring sign-off

| Q | Default recommendation | Who owns the call | Sign-off |
|---|----------------------|------------------|----------|
| **Q3.1** Canonical energy metric name = `power.current_w`? | Yes | Product + Integration | ☐ ☐ |
| **Q3.2** `AvgQualityScore` = mean of per-asset latest score? | Yes | Quality team | ☐ |
| **Q3.3** `RackUtilizationPct` = slot-based (current SQL)? | Yes | Product | ☐ |
| **Q3.4** Frontend `staleTime` for `useDashboardStats` lowered from 5 min → 10 s, `refetchOnWindowFocus: true`? | Yes | Frontend Lead | ☐ |
| **Q3.5** Add `meta.partial: true` envelope flag when any field degrades to 0 on error? | No — defer. Revisit if users complain. | Product + Frontend | ☐ ☐ |

**Deferred to implementation PR review:** event-subject granularity (over- vs under-invalidate, report § 12 #4), Redis failure chaos test (§ 12 #9).

---

## Meeting agenda (60 min)

| Time | Item | Output |
|------|------|--------|
| 0-5 | Context + brief walkthrough | Shared understanding of why all 3 need a decision |
| 5-25 | **D1 (4.8)** — walk Q1.1 → Q1.6 | Signed answers, hybrid C + mini-A2 locked in or alternative chosen |
| 25-40 | **D2 (4.2)** — walk Q2.1 → Q2.6 | Retention policy + cold format locked in |
| 40-55 | **D3 (4.7)** — walk Q3.1 → Q3.5 | KPI semantics locked in |
| 55-60 | Action items, implementation owners, target dates | Action list attached below |

---

## Sign-off ledger

| Decision | Owner | Signed | Date | Notes |
|----------|-------|--------|------|-------|
| D1 Q1.1–Q1.6 | Architecture + Product + Security + SRE | ☐ | | |
| D2 Q2.1–Q2.6 | Product + Compliance + Security + DBA | ☐ | | |
| D3 Q3.1–Q3.5 | Product + Quality + Frontend + Integration | ☐ | | |

---

## Appendix — implementation unblock path

Once this brief is signed:

1. **D1 (4.8)** — 2 person-weeks. Blocks **audit write reliability**. Recommended sequence:
   - Migration 000053 (add ENUM + `operator_type` column + CHECK on `audit_events`)
   - Migration 000054 (per-tenant `system` user seeded via trigger; union with mini-A2 for work_orders / inventory)
   - `audit.Service.Record` re-signed to require `operator_type` + `operator_id nullable`
   - 17 call-sites migrated, batched by module (workflows / sync / maintenance / inventory)
   - Phase 3.1 swallow-cleanup pass revisited to ensure audit write errors now surface
2. **D2 (4.2)** — 1 person-week. Blocks **audit table growth**.
   - Migration 000055 (partitioned `audit_events` parent, backfill from legacy)
   - `cmd/audit-archive` CLI + k8s CronJob + S3 bucket provisioning
   - Runbook for partition observation + legacy table cleanup
3. **D3 (4.7)** — 2 person-weeks (pre-existing estimate). Blocks nothing but unifies dashboard KPIs.
   - OpenAPI + sqlc stubs for 4 new fields
   - `dashboard.InvalidationSubscriber` + `workflows.bus` wiring
   - Frontend hook update + `staleTime` tweak

All three can proceed in parallel (different files, different migrations numbers, different subsystems) once signed.

---

**Document status:** ready for review
**Next action:** schedule 60-minute decision meeting; distribute this brief 48 h in advance so attendees can read the three source reports.
