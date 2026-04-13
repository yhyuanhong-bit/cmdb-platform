# v1.2 Phase C: Edge Sync Phase 4 — Stress Testing, Chaos Testing, Offline UI & Docs

> Date: 2026-04-13
> Status: Draft
> Prereqs: Phase A+B complete (all sync entity types operational)
> Scope: Stress test, chaos test, Edge 503 offline UI, Edge deployment guide

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Test delivery | Scripts + short run in session | Write-only risks broken scripts; 5-min short version validates correctness |
| 503 trigger | Initial snapshot sync only | Daily incremental sync is background — shouldn't block API. Only first-time sync needs full data before serving |
| Documentation | Single integrated file | One doc for deploy + ops + troubleshoot; splitting into 3 would be too thin |
| Test framework | Go integration test + shell script | Go test for stress (DB-level), shell for chaos (Docker network-level) |

---

## C1: Stress Test

### File
`cmdb-core/tests/sync_stress_test.go`

### Design
Go integration test with build tag `//go:build integration` (excluded from `go test ./...`).

Run with: `go test -tags integration ./tests/ -run TestSyncStress -v`

### Logic

```
1. Connect to local PostgreSQL
2. Insert simulated 14-day offline data:
   - N records per entity type (assets, work_orders, alert_events, inventory_tasks, inventory_items)
   - Each with incrementing sync_version
   - Short version: N=20 (100 total across 5 types)
   - Full version: N=140 (700 total, ~50/day for 14 days)
3. For each entity type, call GET /sync/changes?entity_type=X&since_version=0
4. Verify: returned count == inserted count
5. Record: elapsed time, memory usage
6. Assert: total sync time < 30 seconds (full version)
```

### Configuration

```go
const (
    shortModeRecords = 20   // per entity type, for session validation
    fullModeRecords  = 140  // per entity type, for 14-day simulation
)
```

Toggle via `-short` flag: `go test -tags integration ./tests/ -run TestSyncStress -short`

---

## C2: Chaos Test

### File
`scripts/chaos-test.sh`

### Design
Shell script using Docker network disconnect/connect to simulate NATS failures.

Run with: `./scripts/chaos-test.sh --rounds 3` (short) or `./scripts/chaos-test.sh --duration 24h` (full)

### Prerequisites
- Docker Compose stack running (Central + Edge)
- `scripts/start-central.sh` and `scripts/start-edge.sh` already exist

### Logic

```bash
for each round:
    1. docker network disconnect cmdb-net edge-nats
    2. Insert test record in Edge DB (simulate offline write)
    3. Insert test record in Central DB (simulate concurrent write)
    4. sleep random(5, 30) seconds
    5. docker network connect cmdb-net edge-nats
    6. sleep 10 seconds (wait for sync)
    7. Query both DBs: compare sync_state.last_sync_version per entity_type
    8. Assert: versions match (eventual consistency verified)
    9. Log: round number, disconnect duration, sync lag after reconnect
```

### Output
Writes to stdout and `chaos-test-results.log`:
```
Round 1: disconnect 12s, reconnect OK, sync lag 2s, versions match ✓
Round 2: disconnect 27s, reconnect OK, sync lag 3s, versions match ✓
...
SUMMARY: 3/3 rounds passed, max sync lag 3s
```

### Docker environment note
If Docker Compose stack is not running, short version validates script syntax only (dry-run mode with `--dry-run` flag).

---

## C3: Edge Offline UI

### Backend: Sync Gate Middleware

**New file:** `cmdb-core/internal/middleware/sync_gate.go`

```go
func SyncGateMiddleware(initialSyncDone *atomic.Bool, deployMode string) gin.HandlerFunc
```

Logic:
- If `deployMode != "edge"` → pass through (Central never blocks)
- If `initialSyncDone.Load() == true` → pass through (sync complete)
- If request path is `/readyz` or starts with `/api/v1/sync/` → pass through (health + sync endpoints always available)
- Otherwise → return 503 with JSON body:
  ```json
  {"error": {"code": "SYNC_IN_PROGRESS", "message": "Edge node is performing initial sync. Please wait."}}
  ```
  Plus `Retry-After: 30` header

**Agent modification:** `cmdb-core/internal/domain/sync/agent.go`

Add `InitialSyncDone *atomic.Bool` field to Agent struct. In `Start()`:
- Before subscribing to envelopes, check if sync_state has any records
- If sync_state is empty → request snapshot from Central, apply it, THEN set `InitialSyncDone.Store(true)`
- If sync_state has records → set `InitialSyncDone.Store(true)` immediately (not first boot)

**main.go wiring:**
```go
var initialSyncDone atomic.Bool
initialSyncDone.Store(true) // default: don't block (Central mode)

if cfg.DeployMode == "edge" {
    initialSyncDone.Store(false) // Edge: block until sync completes
    agent := sync.NewAgent(pool, bus, cfg)
    agent.InitialSyncDone = &initialSyncDone
    go agent.Start(ctx)
}

// Register middleware before API routes
router.Use(middleware.SyncGateMiddleware(&initialSyncDone, cfg.DeployMode))
```

### Frontend: Syncing Overlay

**New file:** `cmdb-demo/src/components/SyncingOverlay.tsx`

A full-screen overlay component that shows when API returns 503 with code `SYNC_IN_PROGRESS`.

**Modify:** `cmdb-demo/src/lib/api/client.ts`

In the `request()` method, after checking for 401, add 503 handling:

```ts
if (res.status === 503 && json?.error?.code === 'SYNC_IN_PROGRESS') {
    // Dispatch a custom event that SyncingOverlay listens to
    window.dispatchEvent(new CustomEvent('sync-in-progress'))
    throw new ApiRequestError('SYNC_IN_PROGRESS', json.error.message, 503)
}
```

**SyncingOverlay behavior:**
- Listens to `sync-in-progress` custom event
- Shows full-screen overlay with spinner + "Initial sync in progress..."
- Every 5 seconds, calls `GET /readyz` to check if sync is done
- When readyz returns 200, dismisses overlay and reloads page

**Mount in App.tsx:** `<SyncingOverlay />` at root level, outside routes.

---

## C4: Edge Deployment Guide

### File
`docs/edge-deployment-guide.md`

### Structure (~250 lines)

```markdown
# Edge Node Deployment Guide

## Prerequisites
- Docker 24+ and Docker Compose v2
- Network access to Central NATS server (port 7422)
- Assigned TENANT_ID and EDGE_NODE_ID from Central admin

## Quick Start (5 minutes)
1. Clone repository
2. Copy .env.edge.example → .env
3. Set TENANT_ID, EDGE_NODE_ID, CENTRAL_NATS_URL
4. Run: ./scripts/start-edge.sh
5. Verify: curl http://localhost:8080/readyz

## Configuration Reference
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| DEPLOY_MODE | yes | cloud | Set to "edge" |
| TENANT_ID | yes (edge) | — | Tenant UUID from Central |
| EDGE_NODE_ID | yes (edge) | — | Unique node identifier |
| NATS_URL | yes | nats://localhost:4222 | Local NATS address |
| CENTRAL_NATS_URL | yes (edge) | — | Central NATS leafnode address |
| DATABASE_URL | yes | postgres://... | Local PostgreSQL |
| REDIS_URL | yes | redis://localhost:6379 | Local Redis |

## Initial Sync
- First boot triggers automatic snapshot download from Central
- API returns 503 until sync completes (~30 seconds for typical dataset)
- Monitor progress: curl http://localhost:8080/api/v1/sync/state

## Operations Checklist
### Daily
- Check /system/sync page on Central for node health
- Verify sync_state.last_sync_at is recent

### Weekly
- Review error logs: docker compose logs cmdb-core | grep ERROR
- Check disk usage: docker system df

### Monthly
- Update container images
- Review sync conflict history

## Troubleshooting

### NATS Connection Failed
Symptom: "NATS not available" in logs
Fix: Verify CENTRAL_NATS_URL is reachable, check firewall rules for port 7422

### Sync Stuck
Symptom: sync_state.last_sync_at not updating
Fix: Check NATS leafnode status, restart edge NATS: docker compose restart nats

### Data Inconsistency
Symptom: Central and Edge show different data
Fix: Check sync_conflicts table, trigger manual reconciliation via API

### Re-initialize Edge
Nuclear option: drop sync_state rows for this node, restart cmdb-core
Edge will re-request full snapshot from Central
```

---

## Files Changed Summary

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/tests/sync_stress_test.go` | Stress test (Go integration test) |
| `scripts/chaos-test.sh` | Chaos test (Docker network disconnect) |
| `cmdb-core/internal/middleware/sync_gate.go` | 503 middleware for Edge initial sync |
| `cmdb-demo/src/components/SyncingOverlay.tsx` | Full-screen sync overlay |
| `docs/edge-deployment-guide.md` | Integrated deployment + ops + troubleshooting guide |

### Modified Files

| File | Change |
|------|--------|
| `cmdb-core/internal/domain/sync/agent.go` | Add InitialSyncDone atomic.Bool + set on sync complete |
| `cmdb-core/cmd/server/main.go` | Wire initialSyncDone + register SyncGateMiddleware |
| `cmdb-demo/src/lib/api/client.ts` | 503 SYNC_IN_PROGRESS handling |
| `cmdb-demo/src/App.tsx` | Mount SyncingOverlay |

---

## Acceptance Criteria (from milestone plan)

- [ ] Stress test: 14-day offline data syncs in < 30 seconds (short version passes in session)
- [ ] Chaos test: random disconnect/reconnect script runs 3 rounds, all pass
- [ ] Edge initial sync → API returns 503 → frontend shows overlay → sync completes → overlay dismisses
- [ ] Edge deployment guide enables new operator to deploy Edge independently
