# CMDB-Core Service Level Objectives

Status: **Active** (wave-3 observability deliverable, 2026-04-22)
Scope: `cmdb-core` HTTP API + cross-node sync agent
Owners: Platform / SRE
Reporting window: rolling 30 days (availability, latency) / 7 days (sync)

This document defines the formal SLOs, SLIs, and error budgets for
cmdb-core. The accompanying Prometheus recording rules and burn-rate
alerts live in
[`cmdb-core/deploy/prometheus/rules/slo.yml`](../cmdb-core/deploy/prometheus/rules/slo.yml).

All SLIs reference metrics that are already registered in
[`internal/platform/telemetry/metrics.go`](../cmdb-core/internal/platform/telemetry/metrics.go).
No invented metric names appear below — every formula is grounded in
live cardinality.

---

## 1. Availability SLO

> The API should return a non-5xx response to legitimate requests.

| Field | Value |
| --- | --- |
| SLI | `sum(rate(http_requests_total{status!~"5.."}[W])) / sum(rate(http_requests_total[W]))` |
| Target | **99.9%** over 30 days |
| Error budget | 0.1% = **43m 49.7s** of "bad" requests per 30d |
| Source metric | `http_requests_total{method,path,status}` (counter) |
| Signal boundary | Any HTTP 5xx status code counts as a "bad" event. 4xx is **not** counted — a 401/404 is the caller's fault, not ours. |

### Why 99.9%, not 99.99%?

- The service has a single-region Postgres primary today. A DB
  failover realistically costs 2-5 minutes. One failover per quarter
  already eats >50% of a 99.99% budget; 99.9% leaves headroom for
  one unplanned failover per month plus routine deploys.
- Upstream dependencies (NATS, Redis) are not yet configured for HA
  in the edge profile. Availability math cannot exceed the weakest
  tier in the stack.
- Revisit target after the wave-3 HA work lands.

### Known gaps

- `status` labels are free-form strings (from `strconv.Itoa(c.Writer.Status())`).
  A bug that writes `status=0` from an aborted middleware would be counted
  as "good". Mitigation: the `slo:http_requests:unlabelled_5m` auxiliary
  recording rule tracks unknown status codes so ops can spot drift.
- The `path` label uses `c.FullPath()` which falls back to the raw URL on
  404. High-cardinality 404 floods can inflate total-count denominators.
  Acceptable for now — 404 traffic is a tiny share and still
  legitimately "good" from an availability standpoint.

---

## 2. Latency SLO

> 99% of requests should complete in under 1 second.

| Field | Value |
| --- | --- |
| SLI | `sum(rate(http_request_duration_seconds_bucket{le="1.0"}[W])) / sum(rate(http_request_duration_seconds_count[W]))` |
| Target | **99%** of requests under **1s** over 30 days |
| Error budget | 1% = ~**7h 12m** of "slow" requests per 30d |
| Source metric | `http_request_duration_seconds` (histogram, buckets `[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0]`) |
| Signal boundary | Any request whose observed duration is ≥ 1s is "slow". This is measured from the Gin middleware (`PrometheusMiddleware`) which wraps the entire handler. |

### Why 1s and not 500ms?

- The real 99th-percentile driver is `DashboardGetStatsDuration`, which
  has a per-field timeout budget of ~2.1s **by design** (see
  `metrics.go:282-286`). Setting the SLO threshold at 500ms would
  tag the partial-tolerance fallback as a failure even though the
  dashboard is behaving correctly.
- The histogram has a native `le="1.0"` bucket — we don't have to
  interpolate or estimate. That makes the SLI literally a counter
  division, which makes the burn-rate alerts trivially correct.
- Revisit after query-optimization work (dashboard field timeouts
  should be hit <0.1% of the time; today they're the headline slow case).

### Why not p99 as a separate SLO?

A classic p99 latency SLO requires `histogram_quantile(0.99, ...)`,
which is **not a ratio** and therefore cannot be consumed by standard
burn-rate alerts. The bucket-ratio form above is the Google SRE
Workbook's preferred formulation (Ch. 4, "The Error Budget Policy")
because it composes cleanly with multi-window multi-burn-rate rules.

### Known gaps

- Write-heavy endpoints (sync apply, alert emit) run out-of-band of
  the HTTP path and are **not** covered by this SLO. They have
  their own SLI below.
- Background CronJobs (audit-archive, webhook retention) are not
  user-facing and are deliberately excluded.

---

## 3. Sync Apply Success SLO

> Cross-node sync envelopes should successfully apply (not reject, not fail).

| Field | Value |
| --- | --- |
| SLI | `sum(rate(cmdb_sync_envelope_applied_total[W])) / (sum(rate(cmdb_sync_envelope_applied_total[W])) + sum(rate(cmdb_sync_envelope_failed_total[W])) + sum(rate(cmdb_sync_envelope_rejected_total[W])))` |
| Target | **99%** over 7 days |
| Error budget | 1% = **1h 40m 48s** of bad envelope outcomes per 7d |
| Source metrics | `cmdb_sync_envelope_applied_total{entity_type}`, `cmdb_sync_envelope_failed_total{entity_type}`, `cmdb_sync_envelope_rejected_total{entity_type,reason}` |
| Signal boundary | An envelope is "bad" if it was rejected (tenant mismatch, bad checksum, bad signature) or failed to apply (DB error). Skipped envelopes (`cmdb_sync_envelope_skipped_total`) are version-gate no-ops and are **intentionally excluded** from both numerator and denominator — they don't represent work that was supposed to happen. |

### Why success-ratio and not freshness?

The original plan called for a **freshness SLI** — "% of sync
envelopes applied within 5 min of publish". That SLI **cannot be
built from the current metric surface**:

- There is no publish-timestamp label on `cmdb_sync_envelope_applied_total`.
- There is no histogram observing publish→apply latency.
- There is no gauge of oldest-unapplied envelope age per node.

This is the single biggest observability gap on the sync path. Until
one of those metrics lands (tracked as `OBS-SYNC-FRESHNESS` in
`docs/reports/`), the success-ratio SLI is the closest lawful proxy.
It catches the operational failure modes that freshness would catch
(stuck agents produce zero applies and growing failures), just on a
longer detection horizon.

### Why 99% / 7d and not 99.9% / 30d?

- Sync traffic volume is low — single-digit envelopes/sec in typical
  multi-tenant load. Rare failures dominate the ratio and a 30d
  window would mask real short-term outages.
- The 7-day window is the same cadence as the `cmdb_sync_reconciliation_runs_total`
  job, which means an operator can diff SLO burn against reconciliation
  behaviour directly.

### Known gaps

- This is a **throughput-success** SLO, not a **latency** SLO. A sync
  agent that applies envelopes correctly but with a 4-hour delay
  looks perfectly healthy on this SLI. Add a freshness SLI when the
  metric exists.
- The `cmdb_sync_envelope_rejected_total{reason="tenant_mismatch"}`
  signal is a security event, not a reliability event. A tenant
  running a buggy publisher shouldn't burn our SLO budget.
  **Accepted caveat** — we keep it in the denominator deliberately
  so that ops notices when tenant_mismatch becomes endemic.

---

## 4. Not Covered

The following surfaces **do not** currently have SLOs and we are not
pretending they do:

| Surface | Reason | Planned |
| --- | --- | --- |
| WebSocket connection stability | `ws_active_connections` is a gauge with no success/failure event stream | Add `ws_connection_terminations_total{reason}` first |
| Webhook delivery | `webhook_circuit_breaker_trips_total` is an aggregate trip counter with no per-delivery success rate | Add `webhook_deliveries_total{outcome}` |
| Adapter pulls | `adapter_pull_attempts_total{outcome}` exists but per-tenant cardinality makes target-setting per-tenant, not service-wide | Needs product decision on per-tenant SLOs |
| Sync freshness | see section 3 | `OBS-SYNC-FRESHNESS` |
| Dashboard p99 latency | Covered by global HTTP latency SLO today; the `cmdb_dashboard_get_stats_duration_seconds` histogram could support a tighter per-endpoint SLO later | Defer until dashboard is the limiting factor |

## 5. Alert Taxonomy

The rules file implements two burn-rate alerts per SLO
(Google SRE Workbook Ch. 5, "Multi-window, Multi-burn-rate Alerts"):

| Severity | Window pair | Burn rate | Budget burned | Action |
| --- | --- | --- | --- | --- |
| `page` | 1h (long) + 5m (short) | 14.4× | 2% of 30d budget in 1h | Wake on-call |
| `ticket` | 6h (long) + 30m (short) | 1× | ~4% of 30d budget in 6h | Open ticket, fix next business day |

The `for: 2m` on the page alert avoids paging on transient scrape
noise; the `for: 15m` on the ticket alert reflects that slow burns
don't need sub-minute reactivity.

## 6. Change Control

- Target changes (e.g. 99.9% → 99.95%) require a PR that updates
  both this doc and `deploy/prometheus/rules/slo.yml`. The two
  MUST stay in lockstep — a target in one place but not the other
  is a deploy-time correctness bug.
- Adding new SLIs requires showing the source metric is registered
  and emitted. Fictional metrics do not earn SLOs.
- Retiring an SLI requires a written note here with the date and
  the replacement SLI, if any.
