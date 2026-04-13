# v1.2 Phase A: Edge Sync Phase 2 — Design Spec

> Date: 2026-04-13
> Status: Draft
> Prereqs: v1.1.0, Migration 000027, RFC `docs/design/edge-offline-sync-rfc.md` (Approved)
> Scope: Work order dual-dimension state machine, work order/alert sync, conflict resolution UI, backend integration tests

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| status backward compat | DeriveStatus (computed from dual dimensions) | Single source of truth, zero consumer breakage |
| Conflict UI style | Existing platform card/list + modal | No new UI dependencies |
| Implementation order | Inside-out (statemachine → sync → API → frontend) | Each layer tested before the next |
| Test scope | Backend integration tests only | Frontend tests deferred to Phase E (Playwright) |
| Apply strategy | Per-entity apply functions (not generic) | Work orders, alerts, alert_rules all have different strategies |

---

## Section 1: Work Order Dual-Dimension State Machine

### 1.1 Prereq: sqlc Schema Update

First step before any business logic.

1. Update `cmdb-core/db/queries/work_orders.sql`:
   - All `RETURNING` clauses add `execution_status, governance_status`
   - New query: `UpdateExecutionStatus` — `SET execution_status=$2, status=$3 WHERE execution_status=$4`
   - New query: `UpdateGovernanceStatus` — `SET governance_status=$2, status=$3 WHERE governance_status=$4`
   - Modify `StampWorkOrderApproval` — also `SET governance_status='approved'` + derived status
2. Run `sqlc generate` — `WorkOrder` struct gains `ExecutionStatus`, `GovernanceStatus` fields
3. API response automatically includes new fields via json tags

### 1.2 State Definitions

```
ExecutionStatus (Edge controlled):
  pending → working → done

GovernanceStatus (Central controlled):
  submitted → approved | rejected
  approved → verified
  rejected → submitted (resubmit)
```

### 1.3 DeriveStatus

Pure function, only produces the existing 6 status values. No new values introduced.

```go
func DeriveStatus(exec, gov string) (string, error)
```

Tolerates dirty backfill data — maps `gov="in_progress"` or `gov="completed"` to `"approved"` internally.

Mapping table:

| execution | governance | → status | Notes |
|-----------|-----------|----------|-------|
| pending | submitted | submitted | Initial |
| pending | approved | approved | Approved, not started |
| pending | rejected | rejected | Rejected |
| working | submitted | in_progress | Work started, approval pending |
| working | approved | in_progress | Normal: working + approved |
| working | rejected | rejected | Anomaly |
| done | submitted | completed | Work done, approval pending |
| done | approved | completed | Done + approved, awaiting verification |
| done | rejected | rejected | **Anomaly** — emit `SubjectOrderAnomaly` event |
| done | verified | verified | Terminal |
| pending | verified | verified | Terminal takes priority |
| working | verified | verified | Terminal takes priority |

Priority rules: `verified` > `rejected` > execution mapping.

Invalid string values return error.

### 1.4 Dual-Entry Architecture

```
┌──────────────────────┬──────────────────────────┐
│ Old API (local ops)  │ Sync layer (Edge↔Central)│
│ Transition()         │ TransitionExecution()    │
│                      │ TransitionGovernance()   │
├──────────────────────┴──────────────────────────┤
│           DeriveStatus() + log + event           │
└──────────────────────────────────────────────────┘
```

**Entry 1 — Old `Transition()` (enforces linear order):**

- Preserves `ValidateTransition` check (submitted→approved→in_progress→...)
- After passing linear validation, delegates by target status:
  - `approved` / `rejected` / `verified` → `TransitionGovernance`
  - `in_progress` → `TransitionExecution(ExecWorking)`
  - `completed` → `TransitionExecution(ExecDone)`
- Local operations and frontend manual operations use this path
- **No status jumps** — UX identical to pre-v1.2

**Entry 2 — New methods (allow independent operation):**

```go
func (s *Service) TransitionExecution(ctx, tenantID, id, operatorID, newExec) (*WorkOrder, error)
func (s *Service) TransitionGovernance(ctx, tenantID, id, operatorID, roles, newGov, comment) (*WorkOrder, error)
```

- Validates only own dimension's transitions, ignores the other
- Sync layer and conflict resolution UI use this path
- Status jumps are expected behavior (offline scenario)
- `TransitionGovernance` still runs `validateApproval` permission checks

**Shared post-steps for both paths:**

1. Update dimension column (dimension-level optimistic lock)
2. `DeriveStatus()` to compute status
3. Update status column
4. Write work_order_log (action: `transition`)
5. If `execution=done + governance=rejected` → publish `SubjectOrderAnomaly`
6. `incrementSyncVersion`

### 1.5 Tests

| Test | Count | Content |
|------|-------|---------|
| DeriveStatus | 12+1 | All combinations + invalid input error |
| TransitionExecution | 6 | Valid transitions 3 + invalid 3 |
| TransitionGovernance | 8 | Valid 4 + invalid 2 + permissions 2 |
| Old Transition compat | 3 | Key paths delegate correctly |
| Anomaly notification | 2 | done+rejected fires / others don't |

---

## Section 2: Sync Logic

### 2.1 Sync Scope

| Entity | Direction | Strategy | Prereq Status |
|--------|-----------|----------|---------------|
| work_orders | Bidirectional | Zero conflict (dual dimension) | sync_version ✅ events ✅ |
| alert_events | Bidirectional | LWW (compare updated_at) | sync_version ✅ events ✅ |
| alert_rules | Central→Edge one-way | Central Wins | sync_version ❌ events ❌ bus not injected ❌ |

### 2.2 Prereq: alert_rules Infrastructure

**Migration 000028** (combined with governance backfill tolerance):

```sql
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_alert_rules_sync_version ON alert_rules(tenant_id, sync_version);
```

**eventbus — 3 new subjects:**

```go
SubjectAlertRuleCreated = "alert_rule.created"
SubjectAlertRuleUpdated = "alert_rule.updated"
SubjectAlertRuleDeleted = "alert_rule.deleted"
```

**monitoring.Service — inject eventbus.Bus:**

- Change constructor: `func NewService(queries *dbgen.Queries, bus eventbus.Bus) *Service`
- Add `bus.Publish` to Create/Update/Delete methods (one line each)
- Update `cmd/server/main.go` to pass bus parameter

**sync/service.go — add 3 alert_rule subscriptions to RegisterSubscribers.**

**sync_endpoints.go — add `"alert_rules": true` to allowedTables.**

**sqlc regenerate — AlertRule struct gains SyncVersion field.**

### 2.3 Agent Apply Architecture

Current `agent.go` `handleIncomingEnvelope` only updates sync_state, does not write entity data. Rewrite to:

```go
func (a *Agent) handleIncomingEnvelope(ctx, event) error {
    env := parseEnvelope(event)
    if env.Source == a.nodeID { return nil }
    if !env.VerifyChecksum() { return nil }

    switch env.EntityType {
    case "work_orders":
        err = a.applyWorkOrder(ctx, env)
    case "alert_events":
        err = a.applyAlertEvent(ctx, env)
    case "alert_rules":
        err = a.applyAlertRule(ctx, env)
    default:
        err = a.applyGeneric(ctx, env) // assets/locations/racks — simple UPSERT
    }

    if err == nil {
        // Update sync_state (moved to after successful apply)
    }
    return err
}
```

Per-entity functions because strategies differ entirely. Generic UPSERT for tables without special logic.

**Idempotency:** All UPDATE queries use `WHERE sync_version < $remote_version`. CREATE uses `INSERT ... ON CONFLICT (id) DO UPDATE`.

### 2.4 Work Order Apply

Zero conflict — only apply the other side's dimension.

```go
func (a *Agent) applyWorkOrder(ctx, env) error {
    // Parse payload for execution_status, governance_status
    // Determine source: env.Source == "central" → from Central
    if fromCentral {
        // Edge receives Central envelope → only update governance_status
        UPDATE work_orders SET
            governance_status = $gov,
            status = CASE ... END,  -- inline DeriveStatus in SQL
            sync_version = $remote_version
        WHERE id = $id AND sync_version < $remote_version
    } else {
        // Central receives Edge envelope → only update execution_status
        UPDATE work_orders SET
            execution_status = $exec,
            status = CASE ... END,
            sync_version = $remote_version
        WHERE id = $id AND sync_version < $remote_version
    }
    // Check anomaly (done + rejected) → emit event
}
```

DeriveStatus inlined in SQL to avoid read-then-write round trip. Same priority rules as Go function.

**Source detection:**

```go
func isFromCentral(env SyncEnvelope) bool {
    return env.Source == "central"
}
```

Central's nodeID is already `"central"` (sync/service.go L28-29). Edge uses `cfg.EdgeNodeID`.

### 2.5 Alert Event Apply (LWW)

```go
func (a *Agent) applyAlertEvent(ctx, env) error {
    INSERT INTO alert_events (...) VALUES (...)
    ON CONFLICT (id) DO UPDATE SET
        status = EXCLUDED.status,
        resolved_at = EXCLUDED.resolved_at,
        ...
        sync_version = $remote_version
    WHERE alert_events.updated_at < EXCLUDED.updated_at
    -- LWW: only overwrite when remote is newer
}
```

Edge cases:
- Both sides acknowledge same alert simultaneously → LWW picks one, result is "acknowledged" either way
- New alert (not in local DB) → ON CONFLICT doesn't fire, straight INSERT

### 2.6 Alert Rules Apply (Central Wins)

```go
func (a *Agent) applyAlertRule(ctx, env) error {
    INSERT INTO alert_rules (...) VALUES (...)
    ON CONFLICT (id) DO UPDATE SET
        name = EXCLUDED.name,
        metric_name = EXCLUDED.metric_name,
        condition = EXCLUDED.condition,
        severity = EXCLUDED.severity,
        enabled = EXCLUDED.enabled,
        sync_version = $remote_version
    -- No version check: Central always wins
}
```

**Edge write protection — in API handler:**

```go
if cfg.EdgeNodeID != "" {
    response.Forbidden(c, "alert rules are managed by Central, read-only on Edge")
    return
}
```

Added to alert_rules Create/Update/Delete handlers only. One `if` per handler.

### 2.7 SyncGetChanges Enhancement

Return full row data to reduce round trips (Edge networks are unreliable):

```sql
-- Before
SELECT id, sync_version FROM {table} WHERE sync_version > $since

-- After
SELECT row_to_json(t) AS data, sync_version FROM {table} t
WHERE tenant_id = $1 AND sync_version > $2 ORDER BY sync_version LIMIT $3
```

Consistent with SyncSnapshot's row_to_json format.

### 2.8 SyncResolveConflict Apply Framework

Current endpoint only marks conflict as resolved but doesn't write back to entity. Change to:

```go
func (s *APIServer) SyncResolveConflict(c *gin.Context) {
    // 1. Read conflict row (entity_type, entity_id, local_diff, remote_diff)
    // 2. UPDATE sync_conflicts SET resolution, resolved_by, resolved_at
    // 3. Apply resolution:
    //    - local_wins → no data change (local is already correct)
    //    - remote_wins → update entity with remote_diff values
    // 4. incrementSyncVersion on the entity
}
```

Phase A won't produce manual conflicts, but framework is ready for Phase B inventory_tasks.

### 2.9 Files Changed

| File | Change | Size |
|------|--------|------|
| `000028_sync_phase_a.up.sql` | alert_rules sync_version + index | S |
| `eventbus/subjects.go` | +3 alert_rule subjects | S |
| `monitoring/service.go` | Inject bus + CRUD emit events | M |
| `cmd/server/main.go` | Pass bus to NewMonitoringService | S |
| `sync/service.go` | RegisterSubscribers +3 alert_rule listeners | S |
| `sync/agent.go` | Full apply architecture + 3 apply functions + applyGeneric | **L** |
| `sync_endpoints.go` | SyncGetChanges row_to_json + allowedTables + ResolveConflict apply | M |

### 2.10 Tests

| Test | Content |
|------|---------|
| applyWorkOrder — Edge envelope → Central | Only updates execution_status, governance untouched |
| applyWorkOrder — Central envelope → Edge | Only updates governance_status, execution untouched |
| applyWorkOrder — stale version skip | sync_version <= local → no overwrite |
| applyWorkOrder — anomaly detection | done+rejected → emits event |
| applyAlertEvent — remote newer | Overwrites local |
| applyAlertEvent — local newer | Skips remote |
| applyAlertEvent — new alert INSERT | Not in local DB → insert |
| applyAlertRule — Central push | Unconditional overwrite |
| applyAlertRule — Edge write blocked | API returns 403 |
| SyncResolveConflict — remote_wins | Actually writes back to entity |
| SyncResolveConflict — local_wins | Entity unchanged |

---

## Section 3: Conflict Resolution UI (/system/sync)

### 3.1 Page Positioning

Phase A entities don't produce manual conflicts. This page provides:

1. **Sync status overview** — useful now (per-node sync progress)
2. **Conflict management** — framework for Phase B

Two tabs, same pattern as SystemSettings.tsx (useState + button toggle).

### 3.2 Page Structure

```
/system/sync
├── Tab 1: Sync Status
│   └── Per Edge node × entity type sync progress
│
└── Tab 2: Conflicts
    ├── Filter (entity_type dropdown, frontend-side)
    ├── Conflict list (Tailwind div cards)
    └── Modal: left/right JSON + resolve buttons
```

### 3.3 Tab 1: Sync Status

**Data source:** `GET /api/v1/sync/state`, `useQuery` with `refetchInterval: 30000`

Per Edge node section:

```
┌─ edge-taipei ──────────────────────────────────┐
│ Entity          Version   Last Sync     Status  │
│ assets          142       2m ago        ● OK    │
│ work_orders     87        2m ago        ● OK    │
│ alert_events    203       15m ago       ● Lag   │
│ alert_rules     12        2m ago        ● OK    │
│ inventory_tasks 45        25h ago       ● Error │
└─────────────────────────────────────────────────┘
```

Status indicator (inline Tailwind dot + text):
- Green `OK` — last_sync_at < 1h
- Yellow `Lag` — 1h ~ 24h
- Red `Error` — status='error' or > 24h

### 3.4 Tab 2: Conflict Management

**Data source:** `GET /api/v1/sync/conflicts`, full load (no pagination — zero or near-zero conflicts in Phase A)

**Filter:** entity_type dropdown, client-side filter.

**List:** Tailwind div cards, each showing entity_type, entity_id, versions, created_at, and a "View Details" button.

**Batch operations:** Top checkbox select-all + batch resolve buttons. Uses `Promise.all` calling resolve API per item.

**Modal detail** (same pattern as CreateAssetModal — fixed overlay):

```
┌─ Conflict: work_orders / WO-2026-A1B2C3D4 ─── ✕ ─┐
│                                                     │
│  Local (v15)              Remote (v16)              │
│  ┌──────────────┐         ┌──────────────┐          │
│  │ <pre>        │         │ <pre>        │          │
│  │ JSON.stringify│         │ JSON.stringify│          │
│  │ </pre>       │         │ </pre>       │          │
│  └──────────────┘         └──────────────┘          │
│                                                     │
│              [Local Wins]  [Remote Wins]            │
└─────────────────────────────────────────────────────┘
```

- JSON rendered via `JSON.stringify(diff, null, 2)` in `<pre>` blocks
- No field-level diff highlighting in Phase A (add in Phase B when real data exists)
- On resolve: modal closes, `invalidateQueries(['syncConflicts'])`, `toast.success()`

**Empty state:** "No pending conflicts. Sync is running smoothly."

### 3.5 Permission Control

```tsx
const canResolve = usePermission('sync', 'write')
```

- Without write permission: resolve buttons and batch ops hidden, view-only
- Backend RBAC: add `sync` resource with read/write permissions
- seed.sql: super-admin and ops-admin get `sync:write` by default

### 3.6 Files

| File | Responsibility |
|------|---------------|
| `src/lib/api/sync.ts` | API wrapper (3 endpoints) + type definitions |
| `src/hooks/useSync.ts` | React Query hooks (3 hooks) |
| `src/pages/SyncManagement.tsx` | Page with 2 tabs + conflict modal |
| `src/App.tsx` | Lazy import + Route `/system/sync` |
| `src/layouts/MainLayout.tsx` | Sidebar children add sync item |
| `public/locales/en/translation.json` | i18n keys |

**src/lib/api/sync.ts types:**

```ts
export interface SyncState {
  node_id: string
  entity_type: string
  last_sync_version: number
  last_sync_at: string
  status: string
  error_message: string | null
}

export interface SyncConflict {
  id: string
  entity_type: string
  entity_id: string
  local_version: number
  remote_version: number
  local_diff: Record<string, unknown>
  remote_diff: Record<string, unknown>
  created_at: string
}
```

**src/hooks/useSync.ts:**

```ts
export function useSyncState()        // GET /sync/state, refetchInterval: 30000
export function useSyncConflicts()    // GET /sync/conflicts
export function useResolveConflict()  // POST /sync/conflicts/:id/resolve, invalidates syncConflicts
```

### 3.7 Not Doing

| Item | Reason |
|------|--------|
| Pagination | Zero conflicts in Phase A, add in Phase B |
| JSON diff highlighting | Same — `<pre>` is sufficient |
| WebSocket realtime | 30s polling is enough for sync status |
| New UI component library | All Tailwind inline, consistent with existing |
| Component tests | Deferred to Phase E Playwright E2E |

---

## Migration 000028 Summary

Single migration file `000028_sync_phase_a.up.sql` containing:

```sql
-- 1. alert_rules sync support
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_alert_rules_sync_version ON alert_rules(tenant_id, sync_version);

-- 2. RBAC: add sync permissions to ops-admin (super-admin already has "*":"*")
UPDATE roles SET permissions = permissions || '{"sync":["read","write"]}'::jsonb
WHERE name = 'ops-admin';

UPDATE roles SET permissions = permissions || '{"sync":["read"]}'::jsonb
WHERE name = 'viewer';
```

No data migration for governance_status backfill — `DeriveStatus` tolerates dirty values at runtime.

---

## Acceptance Criteria (from milestone plan)

- [ ] Edge offline modifies work order execution_status → after recovery, Central sees it
- [ ] Central approves governance_status → Edge receives update
- [ ] Same work order modified on both sides (different dimensions) → zero conflict auto-merge
- [ ] Conflict UI can list, view, and resolve pending conflicts (framework ready, real data in Phase B)
- [ ] Alert events sync bidirectionally via LWW
- [ ] Alert rules pushed from Central to Edge, Edge is read-only
- [ ] Backend integration tests cover all sync strategies
