# CMDB Platform — Roadmap

> **Last updated**: 2026-04-22
> **Cadence**: roadmap reviewed at end of each milestone (every 8 weeks)
> **Source**: detailed plan in [`plans/2026-04-22-business-remediation-roadmap.md`](plans/2026-04-22-business-remediation-roadmap.md)

This roadmap is the public source of truth for what's shipped, what's in
flight, and what's deliberately out of scope. We strive for **name-claim
parity**: if a feature is in the README/marketing, it works as described.

---

## What ships today (v1.2)

✅ Asset inventory: 800+ CIs, multi-tenant, audit-tracked
✅ Physical hierarchy: locations, racks, U-slot positioning, 3D viz
✅ Work order management with dual-dimension state machine
✅ Discovery via SNMP / SSH / IPMI (manual reconciliation)
✅ BIA assessments (RTO / RPO / tier classification)
✅ Sync framework: edge ↔ central with HMAC-signed envelopes
✅ Webhook integration (outbound)
✅ LLM-routed root cause analysis (OpenAI / Claude / Dify)
✅ Multi-tenant isolation enforced at compile time (custom tenantlint)
✅ Audit trail (immutable, monthly partitioned)
✅ Prometheus / OpenTelemetry observability + SLO definitions

---

## In flight (Q3 2026 — 8 weeks)

### Milestone M1: Service-centric model + data integrity

| Wave | Item | Status |
|---|---|---|
| 0 | Public roadmap + decision log + spec template | 🟢 In progress |
| 0 | README accuracy pass | 🟢 In progress |
| 1 | OpenAPI gate (CI block on spec drift) | 🟡 Planned |
| 2 | **Business Service entity** + service ↔ asset N:M | 🟡 Planned |
| 3 | **Discovery review gate** — no more silent auto-merge | 🟡 Planned |
| 4 | Status field CHECK constraints + cross-page nav links | 🟡 Planned |

**M1 outcome**: BIA can drive decisions; discovery data is auditable; UI is internally coherent.

---

## Planned (Q4 2026 — 8 weeks)

### Milestone M2: External integration

| Wave | Item |
|---|---|
| 5 | **ServiceNow** outbound adapter (work order ↔ incident sync) |
| 5 | **Jira** outbound adapter |
| 5 | Generic webhook DLQ + delivery audit |
| 6 | Service-centric incident aggregation (3 alerts on same service → 1 incident) |

**M2 outcome**: CMDB becomes a real system of record interoperating with ITSM tools.

---

## Planned (Q1 2027 — 8 weeks)

### Milestone M3: Governance + organization

| Wave | Item |
|---|---|
| 7 | **Teams** entity + notification routing to owning team |
| 7 | **CAB approval gate** for high-risk changes (decommission, prod service mutations) |

**M3 outcome**: Compliance frameworks (ISO 27001, SOC2 change control) supported.

---

## Planned (Q2 2027 — 6 weeks)

### Milestone M4: Power monitoring (Energy module)

| Wave | Item |
|---|---|
| 8 | PDU SNMP collectors: APC, Schneider, Eaton |
| 8 | UPS SNMP collectors: APC SmartUPS, Eaton 9PX |
| 8 | `power_events` table + threshold alerts |
| 8 | Real data wiring: peak power, UPS autonomy, rack heatmap |
| 9 | PUE (Power Usage Effectiveness) calculation |
| 9 | Statistical capacity forecasting (Prophet — not ML) |
| 9 | Multi-vendor expansion: Vertiv, CyberPower, ABB |
| 9 | Carbon footprint reporting |
| 9 | TimescaleDB continuous aggregates for long-period trending |

**M4 outcome**: Energy module backed by real data; PUE / carbon reports generation possible for compliance use cases (ISO 50001, Energy Star).

---

## Planned (Q3 2027 — 6 weeks)

### Milestone M5: Operational telemetry foundation

| Wave | Item |
|---|---|
| 10 | Metrics pipeline expansion (target: 100% asset coverage, currently ~2.5%) |
| 10 | Health Score computation refresh (replace static RUL with usage-aware) |
| 10 | LLM-RCA improvement: vector DB + RAG on historical incidents |

**M5 outcome**: Sufficient operational data accumulating to enable real ML training in 6+ months.

---

## Future (data-dependent, 2028+)

These features require operational data that doesn't yet exist. They will
ship when the data foundation (M5) has accumulated enough history.

| Item | Data prerequisite |
|---|---|
| ML failure prediction (XGBoost on metrics + alert history) | 6+ months metrics on 50%+ of assets, 50+ labeled failure incidents |
| Anomaly detection (per-asset baseline) | 30+ days continuous metrics per asset |
| Capacity prediction with uncertainty intervals | 6+ months power/space history |
| Causal inference for incident root cause | 100+ resolved incidents with confirmed root cause labels |

We will not ship these as "AI features" until they are real ML trained on real data. Until then, the platform uses rule-based scoring + LLM-routed analysis, clearly labeled as such.

---

## Explicitly out of scope

These have been considered and **deliberately deferred or cut**:

| Item | Decision | Why |
|---|---|---|
| **Edge offline writes** (true offline-first with local buffer) | Cut | 4 weeks effort + high risk (split-brain on multi-edge concurrent writes); current sync gate behavior covers brief outages; no current customer requires true offline |
| **Custom CI class hierarchy** (per-tenant CI type definitions) | Deferred to 2028+ | JSONB attributes serve current needs; class hierarchy adds complexity without immediate user demand |
| **Federated search** across multiple central CMDBs | Out of scope | Not aligned with hub-spoke product positioning |
| **Real-time multi-user collaborative editing** | Out of scope | Not a CMDB primary use case |

---

## How to influence this roadmap

- **For customers**: file a request via support; we batch-review monthly
- **For internal teams**: open a discussion in `docs/decisions/` with rationale
- **For breaking changes to roadmap**: requires new decision log + sign-off

---

## Related documents

- [Business fit review (2026-04-22)](reviews/2026-04-22-business-fit-review.md) — what's missing from a CMDB perspective
- [Detailed remediation roadmap](plans/2026-04-22-business-remediation-roadmap.md) — engineering task breakdown
- [Day 0 decision log](decisions/2026-04-22-day-0.md) — the decisions backing this roadmap
- [SLO definition](slo-definition.md)
- [Database schema](DATABASE_SCHEMA.md)
