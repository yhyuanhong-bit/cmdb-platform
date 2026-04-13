# v1.2 Phase B: Edge Sync Phase 3 — Inventory & Audit Sync Design Spec

> Date: 2026-04-13
> Status: Draft
> Prereqs: Phase A complete (agent apply, sync service, conflict UI)
> Scope: inventory_tasks/items sync, audit_events append-only sync, monitoring dashboard expansion

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| inventory_tasks conflict | Version-gated full UPSERT (not Manual) | Edge does execution, Central does planning — fields rarely overlap; sync_version gate is sufficient |
| inventory_items direction | Edge→Central one-way | Scan results only produced by Edge |
| audit_events sync_version | Not added — use created_at | Append-only by design, never updated; adding sync_version would require trigger modification |
| Monitoring dashboard | Expand existing Sync Status tab | No new page; time-series history deferred to Phase D (Prometheus/Grafana) |
| Prometheus metrics | Counters only (applied/skipped/failed) | Gauges and histograms are Phase D scope |

---

## Section 1: Sync Logic

### 1.1 Sync Scope

| Entity | Direction | Strategy | sync_version |
|--------|-----------|----------|-------------|
| inventory_tasks | Bidirectional | Version-gated full UPSERT | Exists (migration 000027) |
| inventory_items | Edge→Central one-way | Edge Wins | Needs migration |
| audit_events | Edge→Central one-way | Append-only (created_at incremental) | Not needed |

### 1.2 Migration 000031

```sql
ALTER TABLE inventory_items ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_inventory_items_sync_version ON inventory_items(task_id, sync_version);
```

No changes to audit_events (no sync_version, no trigger modification).

### 1.3 Eventbus Subjects

New subjects:

```go
SubjectInventoryItemCreated = "inventory.item_created"
SubjectInventoryItemUpdated = "inventory.item_updated"
```

`SubjectAuditRecorded` already exists — no new subject needed for audit_events.

Inventory Service needs bus injection (same pattern as Phase A monitoring service). The only method that modifies inventory_items is `ScanItem()` — add `bus.Publish` with `SubjectInventoryItemUpdated` there. No need for new CRUD methods.

### 1.4 inventory_tasks Apply (Version-Gated UPSERT)

```go
func (a *Agent) applyInventoryTask(ctx context.Context, env SyncEnvelope) error {
    // Parse payload into field values
    // UPSERT with version gate:
    INSERT INTO inventory_tasks (...) VALUES (...)
    ON CONFLICT (id) DO UPDATE SET
      name = EXCLUDED.name,
      status = EXCLUDED.status,
      assigned_to = EXCLUDED.assigned_to,
      planned_date = EXCLUDED.planned_date,
      completed_at = EXCLUDED.completed_at,
      ...
      sync_version = EXCLUDED.sync_version
    WHERE inventory_tasks.sync_version < EXCLUDED.sync_version
}
```

Idempotent: `sync_version < remote` gate prevents duplicate processing.

### 1.5 inventory_items Apply (Edge→Central One-Way)

```go
func (a *Agent) applyInventoryItem(ctx context.Context, env SyncEnvelope) error {
    INSERT INTO inventory_items (...) VALUES (...)
    ON CONFLICT (id) DO UPDATE SET
      actual = EXCLUDED.actual,
      status = EXCLUDED.status,
      scanned_at = EXCLUDED.scanned_at,
      scanned_by = EXCLUDED.scanned_by,
      sync_version = EXCLUDED.sync_version
    WHERE inventory_items.sync_version < EXCLUDED.sync_version
}
```

Central does not modify inventory_items, so one-way sync with no conflict.

### 1.6 audit_events Apply (Append-Only)

```go
func (a *Agent) applyAuditEvent(ctx context.Context, env SyncEnvelope) error {
    INSERT INTO audit_events (id, tenant_id, action, module, target_type, target_id,
      operator_id, diff, source, created_at)
    VALUES (...)
    ON CONFLICT (id) DO NOTHING
    -- Already exists → skip (idempotent)
}
```

No sync_version. No UPDATE (append-only trigger would block it anyway).

**sync_state handling:** For audit_events, `last_sync_version` stores Unix timestamp (epoch of last synced `created_at`). Pull query uses `WHERE created_at > to_timestamp($last_sync_version)`.

### 1.7 SyncGetChanges for audit_events

The existing `SyncGetChanges` endpoint uses `sync_version > $since`. For audit_events (no sync_version), add a special case:

```go
if entityType == "audit_events" {
    query = "SELECT row_to_json(t) AS data, EXTRACT(EPOCH FROM t.created_at)::bigint AS sync_version FROM audit_events t WHERE t.tenant_id = $1 AND t.created_at > to_timestamp($2) ORDER BY t.created_at LIMIT $3"
}
```

This maps `created_at` to the `sync_version` field in the response, keeping the API contract consistent.

### 1.8 Agent updateSyncState

Add `"inventory_items"` to the hardcoded table list in `agent.go` `updateSyncState()` (uses `MAX(sync_version)` like other tables).

Add `"audit_events"` separately with special handling — no sync_version column, so use:
```sql
SELECT COALESCE(MAX(EXTRACT(EPOCH FROM created_at))::bigint, 0) FROM audit_events WHERE tenant_id = $1
```
instead of `MAX(sync_version)`.

### 1.9 Sync Subscriptions

Add to `sync/service.go` `RegisterSubscribers()`:

```go
{eventbus.SubjectInventoryItemCreated, "inventory_items", "create"},
{eventbus.SubjectInventoryItemUpdated, "inventory_items", "update"},
{eventbus.SubjectAuditRecorded, "audit_events", "create"},
```

inventory_tasks already has SubjectInventoryTaskCreated and SubjectInventoryTaskCompleted subscribed.

### 1.10 Tests

| Test | Content |
|------|---------|
| applyInventoryTask — version gate accept | sync_version < remote → UPSERT |
| applyInventoryTask — version gate reject | sync_version >= remote → skip |
| applyInventoryItem — Edge push | UPSERT succeeds |
| applyInventoryItem — duplicate skip | Same version → no change |
| applyAuditEvent — new event | INSERT succeeds |
| applyAuditEvent — duplicate event | ON CONFLICT DO NOTHING |

---

## Section 2: Monitoring Dashboard Expansion

### 2.1 Layout

Expand existing Sync Status tab in `/system/sync`:

```
Sync Status Tab
├── Summary cards (3 cards, new)
├── Version gap bar chart (recharts, new)
├── Node detail table (existing, keep)
└── Error list (new)
```

### 2.2 Summary Cards

Three cards at top:

| Card | Value | Source |
|------|-------|--------|
| Nodes | Total registered node count | `GET /sync/state` → count unique node_ids |
| Sync Health | X OK / Y Lag / Z Error | `GET /sync/state` → group by status |
| Pending | N conflicts / M errors | `GET /sync/conflicts` count + error count from state |

All computed frontend-side from existing APIs. No new endpoint needed.

### 2.3 Version Gap Bar Chart

Recharts `BarChart`. Each entity_type gets a bar per node. Height = `max_version - last_sync_version`.

**New API endpoint:** `GET /api/v1/sync/stats`

```go
// Response
{
  "data": [
    {
      "entity_type": "assets",
      "max_version": 142,
      "nodes": [
        { "node_id": "edge-taipei", "last_sync_version": 140, "gap": 2 }
      ]
    }
  ]
}
```

Implementation: For each syncable table, query `MAX(sync_version)`, then join with sync_state to get per-node gaps.

For audit_events (no sync_version): use `MAX(EXTRACT(EPOCH FROM created_at))` as max_version.

### 2.4 Error List

Cards showing sync_state entries with status='error':

```
┌─ edge-taipei / inventory_tasks ──── 2h ago ──┐
│ sync lag 3h12m, 45 versions behind            │
└───────────────────────────────────────────────┘
```

Filtered from existing `GET /sync/state` response — no new API.

### 2.5 Prometheus Metrics

Add to `cmdb-core/internal/platform/telemetry/metrics.go`:

```go
var (
    SyncEnvelopeApplied = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "cmdb_sync_envelope_applied_total", Help: "Successfully applied sync envelopes"},
        []string{"entity_type"},
    )
    SyncEnvelopeSkipped = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "cmdb_sync_envelope_skipped_total", Help: "Skipped sync envelopes (version gate)"},
        []string{"entity_type"},
    )
    SyncEnvelopeFailed = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "cmdb_sync_envelope_failed_total", Help: "Failed sync envelope applications"},
        []string{"entity_type"},
    )
    SyncReconciliationRuns = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "cmdb_sync_reconciliation_runs_total", Help: "Reconciliation job executions"},
    )
)
```

Instrument in `agent.go`: increment on apply success/skip/fail. Instrument in `service.go`: increment reconciliation counter.

### 2.6 New API

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/sync/stats` | Per entity_type max_version + per-node gaps |

Add `"sync"` resource already in RBAC resourceMap (done in Phase A).

### 2.7 Frontend Changes

| File | Change |
|------|--------|
| `src/pages/SyncManagement.tsx` | Add SummaryCards, VersionGapChart, ErrorList components inside SyncStatusTab |
| `src/lib/api/sync.ts` | Add `SyncStats` type + `getSyncStats()` method |
| `src/hooks/useSync.ts` | Add `useSyncStats()` hook |

No new files — extend existing.

### 2.8 Not Doing

| Item | Reason |
|------|--------|
| sync_metrics table | Time-series in Prometheus, not PG |
| History trend charts | Phase D (Prometheus + Grafana) |
| WebSocket push | 30s polling sufficient |
| New dashboard page | Expand existing tab |
| Gauges/histograms | Phase D scope — counters only for now |

---

## Files Changed Summary

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/db/migrations/000031_inventory_items_sync.up.sql` | inventory_items sync_version |
| `cmdb-core/db/migrations/000031_inventory_items_sync.down.sql` | Rollback |
| `cmdb-core/internal/domain/sync/agent_inventory_test.go` | Apply function tests |

### Modified Files

| File | Change |
|------|--------|
| `cmdb-core/internal/eventbus/subjects.go` | +2 inventory_item subjects |
| `cmdb-core/internal/domain/inventory/service.go` | Inject bus, emit events on item CRUD |
| `cmdb-core/cmd/server/main.go` | Pass bus to inventory.NewService, bump expectedMigration to 31 |
| `cmdb-core/internal/domain/sync/service.go` | +3 subscriptions (inventory_items, audit_events) |
| `cmdb-core/internal/domain/sync/agent.go` | +3 apply functions, update dispatch switch, update updateSyncState |
| `cmdb-core/internal/api/sync_endpoints.go` | audit_events special case in SyncGetChanges + new SyncStats endpoint + add inventory_items and audit_events to allowedTables in SyncGetChanges and SyncSnapshot |
| `cmdb-core/internal/platform/telemetry/metrics.go` | +4 Prometheus counters |
| `cmdb-demo/src/pages/SyncManagement.tsx` | Summary cards + bar chart + error list |
| `cmdb-demo/src/lib/api/sync.ts` | SyncStats type + getSyncStats |
| `cmdb-demo/src/hooks/useSync.ts` | useSyncStats hook |

---

## Acceptance Criteria (from milestone plan)

- [ ] Edge completes inventory task → Central sees task result and scan records
- [ ] Edge audit events → Central audit_events table synced
- [ ] Sync Status tab shows summary cards, version gap chart, and error list
- [ ] Prometheus counters (cmdb_sync_envelope_*) incrementing on sync activity
- [ ] All new apply functions idempotent (duplicate envelopes handled gracefully)
