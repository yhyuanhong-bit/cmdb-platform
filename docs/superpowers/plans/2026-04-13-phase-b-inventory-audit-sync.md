# v1.2 Phase B: Inventory & Audit Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the Edge sync system to inventory_tasks, inventory_items, and audit_events, plus expand the Sync Status tab with summary cards, version gap chart, and error list.

**Architecture:** Inside-out — migration first, then eventbus/subscriptions, then agent apply functions, then API enhancements, then Prometheus metrics, then frontend dashboard. Phase A infrastructure (agent dispatch, sync service, conflict UI) is already in place; Phase B extends it to new entity types.

**Tech Stack:** Go 1.22+, PostgreSQL, sqlc, NATS JetStream (eventbus), Prometheus (promauto), React 18, TanStack Query v5, Recharts, Tailwind CSS v4

**Design Spec:** `docs/superpowers/specs/2026-04-13-phase-b-inventory-audit-sync-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/db/migrations/000031_inventory_items_sync.up.sql` | inventory_items sync_version column + index |
| `cmdb-core/db/migrations/000031_inventory_items_sync.down.sql` | Rollback |
| `cmdb-core/internal/domain/sync/agent_inventory_test.go` | Tests for inventory/audit apply functions |

### Modified Files

| File | Change |
|------|--------|
| `cmdb-core/internal/eventbus/subjects.go` | +2 inventory_item subjects |
| `cmdb-core/internal/domain/inventory/service.go` | Inject bus + emit event in ScanItem |
| `cmdb-core/cmd/server/main.go` | Pass bus to inventory.NewService + bump expectedMigration to 31 + register SyncStats route |
| `cmdb-core/internal/domain/sync/service.go` | +3 subscriptions (inventory_items ×2, audit_events ×1) |
| `cmdb-core/internal/domain/sync/agent.go` | +3 apply functions in dispatch switch + update updateSyncState |
| `cmdb-core/internal/api/sync_endpoints.go` | audit_events special case + allowedTables + SyncStats endpoint |
| `cmdb-core/internal/platform/telemetry/metrics.go` | +4 Prometheus counters |
| `cmdb-demo/src/lib/api/sync.ts` | SyncStats type + getSyncStats |
| `cmdb-demo/src/hooks/useSync.ts` | useSyncStats hook |
| `cmdb-demo/src/pages/SyncManagement.tsx` | Summary cards + bar chart + error list in SyncStatusTab |

---

## Task 1: Migration 000031 — inventory_items sync_version

**Files:**
- Create: `cmdb-core/db/migrations/000031_inventory_items_sync.up.sql`
- Create: `cmdb-core/db/migrations/000031_inventory_items_sync.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- 000031_inventory_items_sync.up.sql

ALTER TABLE inventory_items ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_inventory_items_sync_version ON inventory_items(task_id, sync_version);
```

- [ ] **Step 2: Write the down migration**

```sql
-- 000031_inventory_items_sync.down.sql

DROP INDEX IF EXISTS idx_inventory_items_sync_version;
ALTER TABLE inventory_items DROP COLUMN IF EXISTS sync_version;
```

- [ ] **Step 3: Run migration on database**

```bash
psql "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" -f cmdb-core/db/migrations/000031_inventory_items_sync.up.sql
```

Then update schema_migrations:
```bash
psql "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" -c "INSERT INTO schema_migrations (version, dirty) VALUES (31, false) ON CONFLICT (version) DO NOTHING"
```

- [ ] **Step 4: Run sqlc generate**

```bash
cd /cmdb-platform/cmdb-core && sqlc generate
```

Verify: `grep -n 'SyncVersion' internal/dbgen/models.go | grep -i item` should show SyncVersion on InventoryItem struct.

- [ ] **Step 5: Bump expectedMigration in main.go**

In `cmdb-core/cmd/server/main.go`, change:
```go
const expectedMigration = 30
```
to:
```go
const expectedMigration = 31
```

- [ ] **Step 6: Verify build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add cmdb-core/db/migrations/000031_inventory_items_sync.up.sql cmdb-core/db/migrations/000031_inventory_items_sync.down.sql cmdb-core/internal/dbgen/ cmdb-core/cmd/server/main.go
git commit -m "feat(sync): add migration 000031 — inventory_items sync_version"
```

---

## Task 2: Eventbus Subjects + Inventory Bus Injection

**Files:**
- Modify: `cmdb-core/internal/eventbus/subjects.go`
- Modify: `cmdb-core/internal/domain/inventory/service.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add inventory_item event subjects**

In `cmdb-core/internal/eventbus/subjects.go`, add inside the const block (after the existing inventory subjects):

```go
	SubjectInventoryItemCreated = "inventory.item_created"
	SubjectInventoryItemUpdated = "inventory.item_updated"
```

- [ ] **Step 2: Inject eventbus.Bus into inventory.Service**

In `cmdb-core/internal/domain/inventory/service.go`, change the struct and constructor:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
}

func NewService(queries *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: queries, bus: bus}
}
```

- [ ] **Step 3: Add bus.Publish to ScanItem**

In `ScanItem()`, after `s.queries.ScanInventoryItem` succeeds, add:

```go
func (s *Service) ScanItem(ctx context.Context, params dbgen.ScanInventoryItemParams) (*dbgen.InventoryItem, error) {
	item, err := s.queries.ScanInventoryItem(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("scan inventory item: %w", err)
	}
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]interface{}{"item_id": item.ID, "task_id": item.TaskID, "tenant_id": params.ID})
		s.bus.Publish(ctx, eventbus.Event{Subject: eventbus.SubjectInventoryItemUpdated, Payload: payload})
	}
	return &item, nil
}
```

Note: Check the ScanInventoryItemParams struct for the correct field that contains tenant_id. It may be accessible via item.TaskID → lookup, or directly in params. Use what's available.

- [ ] **Step 4: Update main.go to pass bus**

In `cmdb-core/cmd/server/main.go`, change:
```go
inventorySvc := inventory.NewService(queries)
```
to:
```go
inventorySvc := inventory.NewService(queries, bus)
```

- [ ] **Step 5: Verify build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/internal/eventbus/subjects.go cmdb-core/internal/domain/inventory/service.go cmdb-core/cmd/server/main.go
git commit -m "feat(sync): add inventory_item event subjects + inject eventbus into inventory service"
```

---

## Task 3: Sync Subscriptions + allowedTables

**Files:**
- Modify: `cmdb-core/internal/domain/sync/service.go`
- Modify: `cmdb-core/internal/api/sync_endpoints.go`

- [ ] **Step 1: Add subscriptions for inventory_items and audit_events**

In `cmdb-core/internal/domain/sync/service.go` `RegisterSubscribers()`, add 3 entries to the `subjects` slice:

```go
		{eventbus.SubjectInventoryItemCreated, "inventory_items", "create"},
		{eventbus.SubjectInventoryItemUpdated, "inventory_items", "update"},
		{eventbus.SubjectAuditRecorded, "audit_events", "create"},
```

- [ ] **Step 2: Add inventory_items and audit_events to allowedTables**

In `cmdb-core/internal/api/sync_endpoints.go`, add to BOTH allowedTables maps (in `SyncGetChanges` and `SyncSnapshot`):

```go
	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
		"alert_rules": true, "inventory_items": true, "audit_events": true,
	}
```

- [ ] **Step 3: Add audit_events special case in SyncGetChanges**

In `SyncGetChanges`, update the special-case condition for tables without `deleted_at` to also include `inventory_items`:

```go
	// Tables without deleted_at column
	if entityType == "alert_rules" || entityType == "alert_events" || entityType == "inventory_items" {
		query = fmt.Sprintf(
			"SELECT row_to_json(t) AS data, t.sync_version FROM %s t WHERE t.tenant_id = $1 AND t.sync_version > $2 ORDER BY t.sync_version LIMIT $3",
			entityType)
	}
```

Then add a SEPARATE special case for audit_events (no sync_version at all):

```go
	// audit_events: no sync_version, use created_at for incremental pull
	if entityType == "audit_events" {
		query = fmt.Sprintf(
			"SELECT row_to_json(t) AS data, EXTRACT(EPOCH FROM t.created_at)::bigint AS sync_version FROM audit_events t WHERE t.tenant_id = $1 AND t.created_at > to_timestamp($2::bigint) ORDER BY t.created_at LIMIT $3")
	}
```

Note: The `since_version` parameter for audit_events is an epoch timestamp, not a version number. The query converts it back via `to_timestamp()`.

- [ ] **Step 4: Verify build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/domain/sync/service.go cmdb-core/internal/api/sync_endpoints.go
git commit -m "feat(sync): add inventory/audit subscriptions + allowedTables + audit_events incremental pull"
```

---

## Task 4: Agent Apply Functions — Inventory + Audit

**Files:**
- Modify: `cmdb-core/internal/domain/sync/agent.go`
- Create: `cmdb-core/internal/domain/sync/agent_inventory_test.go`

- [ ] **Step 1: Write tests**

Create `cmdb-core/internal/domain/sync/agent_inventory_test.go`:

```go
package sync

import (
	"encoding/json"
	"testing"
)

func TestApplyInventoryTaskPayloadParse(t *testing.T) {
	// Verify that a typical inventory_task payload can be unmarshalled
	payload := `{"id":"00000000-0000-0000-0000-000000000001","tenant_id":"00000000-0000-0000-0000-000000000002","name":"Q1 Inventory","status":"in_progress","sync_version":5}`
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["name"] != "Q1 Inventory" {
		t.Errorf("name = %v, want Q1 Inventory", m["name"])
	}
	if m["status"] != "in_progress" {
		t.Errorf("status = %v, want in_progress", m["status"])
	}
}

func TestApplyAuditEventPayloadParse(t *testing.T) {
	// Verify that an audit_event payload can be unmarshalled
	payload := `{"id":"00000000-0000-0000-0000-000000000001","tenant_id":"00000000-0000-0000-0000-000000000002","action":"asset.created","module":"assets","target_type":"asset","target_id":"00000000-0000-0000-0000-000000000003","source":"edge-taipei","created_at":"2026-04-13T10:00:00Z"}`
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["action"] != "asset.created" {
		t.Errorf("action = %v, want asset.created", m["action"])
	}
	if m["source"] != "edge-taipei" {
		t.Errorf("source = %v, want edge-taipei", m["source"])
	}
}
```

- [ ] **Step 2: Run tests — verify they pass**

```bash
cd /cmdb-platform/cmdb-core && go test ./internal/domain/sync/ -run 'TestApplyInventory|TestApplyAudit' -v
```

- [ ] **Step 3: Add applyInventoryTask to agent.go**

Add after `applyAlertRule`:

```go
func (a *Agent) applyInventoryTask(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory task payload: %w", err)
	}

	// Version-gated full UPSERT
	_, err := a.pool.Exec(ctx,
		`INSERT INTO inventory_tasks (id, tenant_id, code, name, scope_location_id, status, method, planned_date, completed_date, assigned_to, created_at, sync_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (id) DO UPDATE SET
		   name = EXCLUDED.name,
		   status = EXCLUDED.status,
		   method = EXCLUDED.method,
		   planned_date = EXCLUDED.planned_date,
		   completed_date = EXCLUDED.completed_date,
		   assigned_to = EXCLUDED.assigned_to,
		   sync_version = EXCLUDED.sync_version
		 WHERE inventory_tasks.sync_version < EXCLUDED.sync_version`,
		payload["id"], payload["tenant_id"], payload["code"], payload["name"],
		payload["scope_location_id"], payload["status"], payload["method"],
		payload["planned_date"], payload["completed_date"], payload["assigned_to"],
		payload["created_at"], env.Version)
	return err
}
```

- [ ] **Step 4: Add applyInventoryItem to agent.go**

```go
func (a *Agent) applyInventoryItem(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory item payload: %w", err)
	}

	expectedJSON, _ := json.Marshal(payload["expected"])
	actualJSON, _ := json.Marshal(payload["actual"])

	// Edge→Central one-way, version-gated
	_, err := a.pool.Exec(ctx,
		`INSERT INTO inventory_items (id, task_id, asset_id, rack_id, expected, actual, status, scanned_at, scanned_by, sync_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO UPDATE SET
		   actual = EXCLUDED.actual,
		   status = EXCLUDED.status,
		   scanned_at = EXCLUDED.scanned_at,
		   scanned_by = EXCLUDED.scanned_by,
		   sync_version = EXCLUDED.sync_version
		 WHERE inventory_items.sync_version < EXCLUDED.sync_version`,
		payload["id"], payload["task_id"], payload["asset_id"], payload["rack_id"],
		expectedJSON, actualJSON, payload["status"],
		payload["scanned_at"], payload["scanned_by"], env.Version)
	return err
}
```

- [ ] **Step 5: Add applyAuditEvent to agent.go**

```go
func (a *Agent) applyAuditEvent(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal audit event payload: %w", err)
	}

	diffJSON, _ := json.Marshal(payload["diff"])

	// Append-only: insert or skip if exists
	_, err := a.pool.Exec(ctx,
		`INSERT INTO audit_events (id, tenant_id, action, module, target_type, target_id, operator_id, diff, source, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO NOTHING`,
		payload["id"], payload["tenant_id"], payload["action"], payload["module"],
		payload["target_type"], payload["target_id"], payload["operator_id"],
		diffJSON, payload["source"], payload["created_at"])
	return err
}
```

- [ ] **Step 6: Update dispatch switch in handleIncomingEnvelope**

In `handleIncomingEnvelope`, add to the switch statement before `default`:

```go
	case "inventory_tasks":
		err = a.applyInventoryTask(ctx, env)
	case "inventory_items":
		err = a.applyInventoryItem(ctx, env)
	case "audit_events":
		err = a.applyAuditEvent(ctx, env)
```

The full switch should now be:
```go
	switch env.EntityType {
	case "work_orders":
		err = a.applyWorkOrder(ctx, env)
	case "alert_events":
		err = a.applyAlertEvent(ctx, env)
	case "alert_rules":
		err = a.applyAlertRule(ctx, env)
	case "inventory_tasks":
		err = a.applyInventoryTask(ctx, env)
	case "inventory_items":
		err = a.applyInventoryItem(ctx, env)
	case "audit_events":
		err = a.applyAuditEvent(ctx, env)
	default:
		err = a.applyGeneric(ctx, env)
	}
```

- [ ] **Step 7: Update updateSyncState**

In `updateSyncState()`, add `"inventory_items"` to the existing `tables` slice:

```go
tables := []string{"assets", "locations", "racks", "work_orders", "alert_events", "inventory_tasks", "inventory_items"}
```

Then add special handling for `audit_events` AFTER the loop:

```go
	// audit_events: no sync_version, use created_at epoch
	var auditMax int64
	err := a.pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM created_at))::bigint, 0) FROM audit_events WHERE tenant_id = $1",
		a.cfg.TenantID).Scan(&auditMax)
	if err == nil {
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
			 VALUES ($1, $2, 'audit_events', $3, now(), 'active')
			 ON CONFLICT (node_id, entity_type) DO UPDATE SET last_sync_version = $3, last_sync_at = now()`,
			a.nodeID, a.cfg.TenantID, auditMax)
	}
```

- [ ] **Step 8: Verify build and tests**

```bash
cd /cmdb-platform/cmdb-core && go build ./... && go test ./internal/domain/sync/ -v
```

- [ ] **Step 9: Commit**

```bash
git add cmdb-core/internal/domain/sync/agent.go cmdb-core/internal/domain/sync/agent_inventory_test.go
git commit -m "feat(sync): add inventory_tasks/items/audit_events apply functions to sync agent"
```

---

## Task 5: Prometheus Metrics

**Files:**
- Modify: `cmdb-core/internal/platform/telemetry/metrics.go`
- Modify: `cmdb-core/internal/domain/sync/agent.go`
- Modify: `cmdb-core/internal/domain/sync/service.go`

- [ ] **Step 1: Add sync counter metrics**

In `cmdb-core/internal/platform/telemetry/metrics.go`, add after the existing metric vars:

```go
	// Sync metrics
	SyncEnvelopeApplied = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_applied_total",
		Help: "Successfully applied sync envelopes.",
	}, []string{"entity_type"})

	SyncEnvelopeSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_skipped_total",
		Help: "Skipped sync envelopes (version gate or duplicate).",
	}, []string{"entity_type"})

	SyncEnvelopeFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_failed_total",
		Help: "Failed sync envelope applications.",
	}, []string{"entity_type"})

	SyncReconciliationRuns = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cmdb_sync_reconciliation_runs_total",
		Help: "Total reconciliation job executions.",
	})
```

- [ ] **Step 2: Instrument agent.go**

In `handleIncomingEnvelope`, after the apply switch block, replace the error handling:

```go
	if err != nil {
		telemetry.SyncEnvelopeFailed.WithLabelValues(env.EntityType).Inc()
		zap.L().Error("sync agent: apply failed",
			zap.String("entity_type", env.EntityType),
			zap.String("entity_id", env.EntityID),
			zap.Error(err))
		return nil
	}

	telemetry.SyncEnvelopeApplied.WithLabelValues(env.EntityType).Inc()
```

Add `"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"` to agent.go imports.

- [ ] **Step 3: Instrument service.go reconciliation**

In `cmdb-core/internal/domain/sync/service.go`, at the start of `reconcile()`:

```go
func (s *Service) reconcile(ctx context.Context) {
	telemetry.SyncReconciliationRuns.Inc()
	// ... rest of existing code
```

Add the telemetry import if not already present.

- [ ] **Step 4: Verify build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/platform/telemetry/metrics.go cmdb-core/internal/domain/sync/agent.go cmdb-core/internal/domain/sync/service.go
git commit -m "feat(sync): add Prometheus counters for sync envelope apply/skip/fail"
```

---

## Task 6: SyncStats API Endpoint

**Files:**
- Modify: `cmdb-core/internal/api/sync_endpoints.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add SyncStats endpoint**

Add to `cmdb-core/internal/api/sync_endpoints.go`:

```go
// SyncStats returns per-entity-type max versions and per-node sync gaps.
// GET /api/v1/sync/stats
func (s *APIServer) SyncStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()

	syncableTables := []string{"assets", "locations", "racks", "work_orders", "alert_events", "inventory_tasks", "alert_rules", "inventory_items"}

	type nodeGap struct {
		NodeID          string `json:"node_id"`
		LastSyncVersion int64  `json:"last_sync_version"`
		Gap             int64  `json:"gap"`
	}
	type entityStats struct {
		EntityType string    `json:"entity_type"`
		MaxVersion int64     `json:"max_version"`
		Nodes      []nodeGap `json:"nodes"`
	}

	var results []entityStats

	for _, table := range syncableTables {
		var maxVersion int64
		var err error

		if table == "audit_events" {
			err = s.pool.QueryRow(ctx,
				"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM created_at))::bigint, 0) FROM audit_events WHERE tenant_id = $1",
				tenantID).Scan(&maxVersion)
		} else {
			err = s.pool.QueryRow(ctx,
				fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s WHERE tenant_id = $1", table),
				tenantID).Scan(&maxVersion)
		}
		if err != nil {
			continue
		}

		// Get per-node sync state for this entity type
		rows, err := s.pool.Query(ctx,
			"SELECT node_id, last_sync_version FROM sync_state WHERE tenant_id = $1 AND entity_type = $2",
			tenantID, table)
		if err != nil {
			results = append(results, entityStats{EntityType: table, MaxVersion: maxVersion, Nodes: []nodeGap{}})
			continue
		}

		var nodes []nodeGap
		for rows.Next() {
			var ng nodeGap
			if rows.Scan(&ng.NodeID, &ng.LastSyncVersion) == nil {
				ng.Gap = maxVersion - ng.LastSyncVersion
				if ng.Gap < 0 {
					ng.Gap = 0
				}
				nodes = append(nodes, ng)
			}
		}
		rows.Close()
		if nodes == nil {
			nodes = []nodeGap{}
		}

		results = append(results, entityStats{EntityType: table, MaxVersion: maxVersion, Nodes: nodes})
	}

	response.OK(c, results)
}
```

- [ ] **Step 2: Register the route**

In `cmdb-core/cmd/server/main.go`, after the existing sync routes (line ~270), add:

```go
	v1.GET("/sync/stats", apiServer.SyncStats)
```

- [ ] **Step 3: Verify build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/api/sync_endpoints.go cmdb-core/cmd/server/main.go
git commit -m "feat(sync): add GET /sync/stats endpoint for version gap monitoring"
```

---

## Task 7: Frontend — API + Hook for SyncStats

**Files:**
- Modify: `cmdb-demo/src/lib/api/sync.ts`
- Modify: `cmdb-demo/src/hooks/useSync.ts`

- [ ] **Step 1: Add SyncStats type and API method**

In `cmdb-demo/src/lib/api/sync.ts`, add the type and method:

```ts
export interface SyncNodeGap {
  node_id: string
  last_sync_version: number
  gap: number
}

export interface SyncEntityStats {
  entity_type: string
  max_version: number
  nodes: SyncNodeGap[]
}

// Add to syncApi object:
export const syncApi = {
  // ... existing methods ...
  getStats: () =>
    apiClient.get<ApiResponse<SyncEntityStats[]>>('/sync/stats'),
}
```

- [ ] **Step 2: Add useSyncStats hook**

In `cmdb-demo/src/hooks/useSync.ts`, add:

```ts
export function useSyncStats() {
  return useQuery({
    queryKey: ['syncStats'],
    queryFn: () => syncApi.getStats(),
    refetchInterval: 30000,
  })
}
```

Import `syncApi` should already be there. Make sure `SyncEntityStats` is not needed in the import (it's used internally by the hook via type inference).

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/lib/api/sync.ts cmdb-demo/src/hooks/useSync.ts
git commit -m "feat(ui): add SyncStats API client and useSyncStats hook"
```

---

## Task 8: Frontend — Dashboard Expansion

**Files:**
- Modify: `cmdb-demo/src/pages/SyncManagement.tsx`

- [ ] **Step 1: Add recharts import and useSyncStats**

At the top of `SyncManagement.tsx`, add:

```tsx
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import { useSyncStats } from '../hooks/useSync'
```

Update the existing import to include `useSyncConflicts`:
```tsx
import { useSyncState, useSyncConflicts, useResolveConflict, useSyncStats } from '../hooks/useSync'
```

- [ ] **Step 2: Add SummaryCards component**

Add before the `SyncStatusTab` function:

```tsx
function SummaryCards({ states, conflictCount }: { states: any[]; conflictCount: number }) {
  const uniqueNodes = new Set(states.map((s: any) => s.node_id))
  const okCount = states.filter((s: any) => {
    if (s.status === 'error') return false
    const hoursSince = (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000
    return hoursSince <= 1
  }).length
  const lagCount = states.filter((s: any) => {
    if (s.status === 'error') return false
    const hoursSince = (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000
    return hoursSince > 1 && hoursSince <= 24
  }).length
  const errorCount = states.filter((s: any) => {
    if (s.status === 'error') return true
    const hoursSince = (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000
    return hoursSince > 24
  }).length

  return (
    <div className="grid grid-cols-3 gap-4 mb-6">
      <div className="bg-surface-container rounded-lg p-4 text-center">
        <div className="text-2xl font-bold text-on-surface">{uniqueNodes.size}</div>
        <div className="text-xs text-on-surface-variant mt-1">Total Nodes</div>
      </div>
      <div className="bg-surface-container rounded-lg p-4 text-center">
        <div className="text-sm font-semibold text-on-surface">
          <span className="text-emerald-500">{okCount} OK</span>
          {lagCount > 0 && <span className="text-yellow-500 ml-2">{lagCount} Lag</span>}
          {errorCount > 0 && <span className="text-red-500 ml-2">{errorCount} Error</span>}
        </div>
        <div className="text-xs text-on-surface-variant mt-1">Sync Health</div>
      </div>
      <div className="bg-surface-container rounded-lg p-4 text-center">
        <div className="text-sm font-semibold text-on-surface">
          {conflictCount} conflicts · {errorCount} errors
        </div>
        <div className="text-xs text-on-surface-variant mt-1">Pending</div>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Add VersionGapChart component**

```tsx
function VersionGapChart({ stats }: { stats: any[] }) {
  // Flatten: one bar per entity_type showing max gap across nodes
  const chartData = stats
    .map((s: any) => ({
      entity_type: s.entity_type.replace('_', ' '),
      gap: Math.max(0, ...s.nodes.map((n: any) => n.gap), 0),
    }))
    .filter((d: any) => d.gap > 0)

  if (chartData.length === 0) {
    return (
      <div className="bg-surface-container rounded-lg p-4 mb-6 text-center text-on-surface-variant text-sm">
        All nodes are up to date — no version gaps.
      </div>
    )
  }

  return (
    <div className="bg-surface-container rounded-lg p-4 mb-6">
      <h3 className="text-sm font-bold text-on-surface mb-3">Version Gap by Entity Type</h3>
      <ResponsiveContainer width="100%" height={200}>
        <BarChart data={chartData}>
          <XAxis dataKey="entity_type" tick={{ fontSize: 11 }} />
          <YAxis tick={{ fontSize: 11 }} />
          <Tooltip />
          <Bar dataKey="gap" fill="#f59e0b" radius={[4, 4, 0, 0]}>
            {chartData.map((_: any, i: number) => (
              <Cell key={i} fill={chartData[i].gap > 50 ? '#ef4444' : '#f59e0b'} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
```

- [ ] **Step 4: Add ErrorList component**

```tsx
function ErrorList({ states }: { states: any[] }) {
  const errors = states.filter((s: any) => s.status === 'error' || (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000 > 24)

  if (errors.length === 0) return null

  return (
    <div className="mt-6">
      <h3 className="text-sm font-bold text-on-surface mb-3">Sync Errors</h3>
      <div className="space-y-2">
        {errors.map((s: any) => (
          <div key={`${s.node_id}-${s.entity_type}`} className="bg-red-500/10 border border-red-500/20 rounded-lg p-3">
            <div className="text-sm font-semibold text-on-surface">
              {s.node_id} / {s.entity_type}
            </div>
            <div className="text-xs text-on-surface-variant mt-1">
              {s.error_message || `Last sync: ${formatTimeAgo(s.last_sync_at)}`}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Update SyncStatusTab to use new components**

Replace the `SyncStatusTab` function body:

```tsx
function SyncStatusTab() {
  const { data: stateResp, isLoading: stateLoading } = useSyncState()
  const { data: conflictsResp } = useSyncConflicts()
  const { data: statsResp, isLoading: statsLoading } = useSyncStats()
  const states = (stateResp as any)?.data ?? []
  const conflicts = (conflictsResp as any)?.data ?? []
  const stats = (statsResp as any)?.data ?? []

  if (stateLoading) {
    return <div className="text-on-surface-variant">Loading sync state...</div>
  }

  if (states.length === 0) {
    return (
      <div className="bg-surface-container rounded-lg p-8 text-center text-on-surface-variant">
        No sync nodes registered yet.
      </div>
    )
  }

  // Group by node_id for detail table
  const byNode: Record<string, typeof states> = {}
  for (const s of states) {
    if (!byNode[s.node_id]) byNode[s.node_id] = []
    byNode[s.node_id].push(s)
  }

  return (
    <div>
      <SummaryCards states={states} conflictCount={conflicts.length} />

      {!statsLoading && stats.length > 0 && <VersionGapChart stats={stats} />}

      <div className="space-y-4">
        {Object.entries(byNode).map(([nodeId, nodeStates]) => (
          <div key={nodeId} className="bg-surface-container rounded-lg p-4">
            <h3 className="text-sm font-bold text-on-surface mb-3 uppercase tracking-wide">{nodeId}</h3>
            <div className="grid grid-cols-[1fr_80px_100px_60px] gap-2 text-sm">
              <div className="text-on-surface-variant font-semibold">Entity</div>
              <div className="text-on-surface-variant font-semibold">Version</div>
              <div className="text-on-surface-variant font-semibold">Last Sync</div>
              <div className="text-on-surface-variant font-semibold">Status</div>
              {(nodeStates as any[]).map((s: any) => {
                const { color, label } = syncStatusColor(s.status, s.last_sync_at)
                return (
                  <div key={s.entity_type} className="contents">
                    <div className="text-on-surface">{s.entity_type}</div>
                    <div className="text-on-surface">{s.last_sync_version}</div>
                    <div className="text-on-surface-variant">{formatTimeAgo(s.last_sync_at)}</div>
                    <div className="flex items-center gap-1.5">
                      <span className={`inline-block w-2 h-2 rounded-full ${color}`} />
                      <span className="text-on-surface-variant">{label}</span>
                    </div>
                  </div>
                )
              })}
            </div>
            {(nodeStates as any[]).some((s: any) => s.error_message) && (
              <div className="mt-3 text-xs text-red-400">
                {(nodeStates as any[]).filter((s: any) => s.error_message).map((s: any) => (
                  <div key={s.entity_type}>{s.entity_type}: {s.error_message}</div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>

      <ErrorList states={states} />
    </div>
  )
}
```

- [ ] **Step 6: Verify frontend builds**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit
```

- [ ] **Step 7: Start dev server and test**

```bash
cd /cmdb-platform/cmdb-demo && npm run dev
```

Test in browser at `http://localhost:5175/system/sync`:
1. Sync Status tab shows summary cards (Nodes / Sync Health / Pending)
2. Version gap chart shows (or "All nodes up to date" if no gaps)
3. Node detail table still works
4. Error list appears if any errors exist

- [ ] **Step 8: Commit**

```bash
git add cmdb-demo/src/pages/SyncManagement.tsx
git commit -m "feat(ui): expand SyncStatus tab with summary cards, version gap chart, and error list"
```

---

## Task 9: Integration Verification

**Files:** None (verification only)

- [ ] **Step 1: Full backend build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 2: Run all tests**

```bash
cd /cmdb-platform/cmdb-core && go test ./... -v -count=1
```

- [ ] **Step 3: Restart server and test APIs**

```bash
# Rebuild and restart
kill $(lsof -t -i :8080) 2>/dev/null
cd /cmdb-platform/cmdb-core && go build -o cmdb-server ./cmd/server/ && ./cmdb-server &
sleep 2

TOKEN=$(curl -s http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | jq -r '.data.access_token')

# Test new sync stats endpoint
curl -s http://localhost:8080/api/v1/sync/stats -H "Authorization: Bearer $TOKEN" | jq .

# Test inventory_items in SyncGetChanges
curl -s "http://localhost:8080/api/v1/sync/changes?entity_type=inventory_items&since_version=0&limit=5" -H "Authorization: Bearer $TOKEN" | jq .

# Test audit_events in SyncGetChanges (uses created_at epoch)
curl -s "http://localhost:8080/api/v1/sync/changes?entity_type=audit_events&since_version=0&limit=5" -H "Authorization: Bearer $TOKEN" | jq .
```

- [ ] **Step 4: Frontend verification**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit && npm run build
```
