# Edge Offline Sync Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundational sync infrastructure — schema, version tracking, sync API endpoints, and the Sync Service/Agent — so Edge nodes can track changes and eventually sync with Central.

**Architecture:** Domain services increment `sync_version` on every write. A new `sync` package subscribes to domain events, wraps them as `SyncEnvelope`, and publishes to a dedicated NATS stream `CMDB_SYNC`. API endpoints expose incremental change queries and snapshot streaming. Work orders gain `execution_status` + `governance_status` columns for conflict-free dual-dimension sync.

**Tech Stack:** Go 1.25, PostgreSQL 17 (migrations), NATS JetStream, pgxpool, Gin

**RFC:** `/cmdb-platform/docs/design/edge-offline-sync-rfc.md`

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `db/migrations/000027_sync_system.up.sql` | Schema: sync_version columns, sync_state, sync_conflicts tables, work order dual-status |
| `db/migrations/000027_sync_system.down.sql` | Rollback migration |
| `internal/domain/sync/envelope.go` | SyncEnvelope struct + checksum helper |
| `internal/domain/sync/layers.go` | Layer dependency definitions for ordered sync |
| `internal/domain/sync/service.go` | SyncService: event subscription, envelope creation, reconciliation |
| `internal/domain/sync/agent.go` | SyncAgent: initial snapshot, incremental apply, layer-ordered processing |
| `internal/api/sync_endpoints.go` | 5 HTTP endpoints for sync API |

### Modified Files
| File | Change |
|------|--------|
| `internal/config/config.go` | Add SyncEnabled, EdgeNodeID, SyncSnapshotBatchSize, envOrDefaultInt |
| `internal/eventbus/subjects.go` | Add sync.* subject constants |
| `internal/eventbus/nats.go` | Add CMDB_SYNC stream creation |
| `internal/domain/asset/service.go` | Add pool field, increment sync_version after Create/Update/Delete |
| `internal/domain/topology/service.go` | Increment sync_version after location/rack Create/Update/Delete (pool already exists) |
| `internal/domain/maintenance/service.go` | Add pool field, increment sync_version after Create/Transition |
| `internal/api/impl.go` | Add syncSvc field to APIServer |
| `cmd/server/main.go` | Initialize SyncService, register sync routes, pass pool to asset/maintenance services |

---

## Task 1: Migration — sync_version, sync_state, sync_conflicts, work order dual-status

**Files:**
- Create: `cmdb-core/db/migrations/000027_sync_system.up.sql`
- Create: `cmdb-core/db/migrations/000027_sync_system.down.sql`

- [ ] **Step 1: Write UP migration**

```sql
-- 000027_sync_system.up.sql

-- 1. Add sync_version to syncable tables
ALTER TABLE assets ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE locations ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE racks ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE inventory_tasks ADD COLUMN IF NOT EXISTS sync_version BIGINT NOT NULL DEFAULT 0;

-- Indexes for incremental sync queries
CREATE INDEX IF NOT EXISTS idx_assets_sync_version ON assets(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_locations_sync_version ON locations(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_racks_sync_version ON racks(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_work_orders_sync_version ON work_orders(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_alert_events_sync_version ON alert_events(tenant_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_inventory_tasks_sync_version ON inventory_tasks(tenant_id, sync_version);

-- 2. Work order dual-dimension status (keep original status column)
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS execution_status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS governance_status VARCHAR(20) NOT NULL DEFAULT 'submitted';

-- Backfill governance_status from existing status
UPDATE work_orders SET governance_status = status WHERE governance_status = 'submitted' AND status != 'submitted';
-- Backfill execution_status from existing status
UPDATE work_orders SET execution_status = CASE
    WHEN status IN ('in_progress') THEN 'working'
    WHEN status IN ('completed', 'verified') THEN 'done'
    ELSE 'pending'
END;

-- 3. Sync state tracking
CREATE TABLE IF NOT EXISTS sync_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id VARCHAR(100) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    entity_type VARCHAR(50) NOT NULL,
    last_sync_version BIGINT DEFAULT 0,
    last_sync_at TIMESTAMPTZ,
    status VARCHAR(20) DEFAULT 'active',
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(node_id, entity_type)
);

-- 4. Sync conflicts
CREATE TABLE IF NOT EXISTS sync_conflicts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    local_version BIGINT NOT NULL,
    remote_version BIGINT NOT NULL,
    local_diff JSONB NOT NULL,
    remote_diff JSONB NOT NULL,
    resolution VARCHAR(20) DEFAULT 'pending',
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_pending ON sync_conflicts(tenant_id, resolution) WHERE resolution = 'pending';
```

- [ ] **Step 2: Write DOWN migration**

```sql
-- 000027_sync_system.down.sql
DROP TABLE IF EXISTS sync_conflicts;
DROP TABLE IF EXISTS sync_state;

ALTER TABLE work_orders DROP COLUMN IF EXISTS execution_status;
ALTER TABLE work_orders DROP COLUMN IF EXISTS governance_status;

DROP INDEX IF EXISTS idx_assets_sync_version;
DROP INDEX IF EXISTS idx_locations_sync_version;
DROP INDEX IF EXISTS idx_racks_sync_version;
DROP INDEX IF EXISTS idx_work_orders_sync_version;
DROP INDEX IF EXISTS idx_alert_events_sync_version;
DROP INDEX IF EXISTS idx_inventory_tasks_sync_version;

ALTER TABLE assets DROP COLUMN IF EXISTS sync_version;
ALTER TABLE locations DROP COLUMN IF EXISTS sync_version;
ALTER TABLE racks DROP COLUMN IF EXISTS sync_version;
ALTER TABLE work_orders DROP COLUMN IF EXISTS sync_version;
ALTER TABLE alert_events DROP COLUMN IF EXISTS sync_version;
ALTER TABLE inventory_tasks DROP COLUMN IF EXISTS sync_version;
```

- [ ] **Step 3: Apply migration**

Run: `cd /cmdb-platform/cmdb-core && DATABASE_URL="postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable" go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path db/migrations -database "$DATABASE_URL" up`

Expected: migration 000027 applied successfully

- [ ] **Step 4: Verify**

Run: `psql -h localhost -U cmdb -d cmdb -c "SELECT column_name FROM information_schema.columns WHERE table_name='assets' AND column_name='sync_version'"`

Expected: `sync_version` column exists

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/db/migrations/000027_sync_system.*
git commit -m "feat(sync): add migration 000027 — sync_version, sync_state, sync_conflicts, work order dual-status"
```

---

## Task 2: Config — sync settings

**Files:**
- Modify: `cmdb-core/internal/config/config.go`

- [ ] **Step 1: Write test for envOrDefaultInt**

Create: `cmdb-core/internal/config/config_test.go` — add test (file already exists, append):

```go
func TestEnvOrDefaultInt(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		def      int
		expected int
	}{
		{"uses default when unset", "TEST_INT_UNSET", "", 500, 500},
		{"uses env when set", "TEST_INT_SET", "1000", 500, 1000},
		{"uses default on invalid", "TEST_INT_BAD", "notanumber", 500, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envKey, tt.envVal)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}
			got := envOrDefaultInt(tt.envKey, tt.def)
			if got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /cmdb-platform/cmdb-core && go test ./internal/config/ -run TestEnvOrDefaultInt -v`

Expected: FAIL — `envOrDefaultInt` not defined

- [ ] **Step 3: Add config fields and helper**

In `config.go`, add to Config struct:
```go
SyncEnabled           bool
SyncSnapshotBatchSize int
EdgeNodeID            string
```

Add helper:
```go
func envOrDefaultInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
```

In `Load()`, add after existing config parsing:
```go
cfg.SyncEnabled = envOrDefault("SYNC_ENABLED", "true") == "true"
cfg.SyncSnapshotBatchSize = envOrDefaultInt("SYNC_SNAPSHOT_BATCH_SIZE", 500)
cfg.EdgeNodeID = envOrDefault("EDGE_NODE_ID", "")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /cmdb-platform/cmdb-core && go test ./internal/config/ -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/config/config.go cmdb-core/internal/config/config_test.go
git commit -m "feat(sync): add sync config — SyncEnabled, EdgeNodeID, SyncSnapshotBatchSize"
```

---

## Task 3: SyncEnvelope and Layer definitions

**Files:**
- Create: `cmdb-core/internal/domain/sync/envelope.go`
- Create: `cmdb-core/internal/domain/sync/layers.go`
- Create: `cmdb-core/internal/domain/sync/layers_test.go`

- [ ] **Step 1: Write layers test**

```go
// layers_test.go
package sync

import "testing"

func TestSyncLayersCompleteness(t *testing.T) {
	seen := make(map[string]bool)
	for _, layer := range SyncLayers {
		for _, entity := range layer {
			if seen[entity] {
				t.Errorf("duplicate entity %q in SyncLayers", entity)
			}
			seen[entity] = true
		}
	}
	required := []string{"locations", "assets", "racks", "work_orders", "alert_events", "inventory_tasks"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("required entity %q missing from SyncLayers", r)
		}
	}
}

func TestSyncLayerOrder(t *testing.T) {
	indexOf := func(entity string) int {
		for i, layer := range SyncLayers {
			for _, e := range layer {
				if e == entity { return i }
			}
		}
		return -1
	}
	// locations must be before racks
	if indexOf("locations") >= indexOf("racks") {
		t.Error("locations must be in an earlier layer than racks")
	}
	// assets must be before work_orders
	if indexOf("assets") >= indexOf("work_orders") {
		t.Error("assets must be in an earlier layer than work_orders")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /cmdb-platform/cmdb-core && go test ./internal/domain/sync/ -v`

Expected: FAIL — package doesn't exist

- [ ] **Step 3: Write envelope.go**

```go
// envelope.go
package sync

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SyncEnvelope wraps a single entity change for sync transport.
type SyncEnvelope struct {
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	TenantID   string          `json:"tenant_id"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Action     string          `json:"action"`
	Version    int64           `json:"version"`
	Timestamp  time.Time       `json:"timestamp"`
	Diff       json.RawMessage `json:"diff"`
	Checksum   string          `json:"checksum"`
}

// NewEnvelope creates a SyncEnvelope with computed checksum.
func NewEnvelope(source, tenantID, entityType, entityID, action string, version int64, diff json.RawMessage) SyncEnvelope {
	env := SyncEnvelope{
		ID:         uuid.New().String(),
		Source:     source,
		TenantID:   tenantID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Version:    version,
		Timestamp:  time.Now(),
		Diff:       diff,
	}
	env.Checksum = env.computeChecksum()
	return env
}

func (e *SyncEnvelope) computeChecksum() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", e.EntityID, e.Version, string(e.Diff))))
	return fmt.Sprintf("%x", h)
}

// VerifyChecksum returns true if the checksum matches.
func (e *SyncEnvelope) VerifyChecksum() bool {
	return e.Checksum == e.computeChecksum()
}
```

- [ ] **Step 4: Write layers.go**

```go
// layers.go
package sync

// SyncLayers defines entity processing order for dependency-safe sync.
// Each layer depends only on layers with a lower index.
var SyncLayers = [][]string{
	{"locations", "users", "roles", "alert_rules"},                   // Layer 0: no dependencies
	{"racks", "assets"},                                               // Layer 1: depends on locations
	{"rack_slots", "asset_dependencies", "alert_events"},              // Layer 2: depends on racks, assets
	{"work_orders", "inventory_tasks"},                                // Layer 3: depends on assets
	{"work_order_logs", "inventory_items", "inventory_scan_history", "audit_events"}, // Layer 4: depends on work_orders, inventory_tasks
}

// LayerOf returns the layer index for an entity type, or -1 if unknown.
func LayerOf(entityType string) int {
	for i, layer := range SyncLayers {
		for _, e := range layer {
			if e == entityType {
				return i
			}
		}
	}
	return -1
}
```

- [ ] **Step 5: Run tests**

Run: `cd /cmdb-platform/cmdb-core && go test ./internal/domain/sync/ -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/internal/domain/sync/
git commit -m "feat(sync): add SyncEnvelope and Layer definitions"
```

---

## Task 4: Event subjects + NATS stream for sync

**Files:**
- Modify: `cmdb-core/internal/eventbus/subjects.go`
- Modify: `cmdb-core/internal/eventbus/nats.go`

- [ ] **Step 1: Add sync subjects**

In `subjects.go`, add:
```go
// Sync subjects
SubjectSyncEnvelope = "sync.envelope"
```

- [ ] **Step 2: Add CMDB_SYNC stream**

In `nats.go`, after the CMDB stream creation (around line 71), add:
```go
// Sync stream for edge federation
_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
	Name: "CMDB_SYNC",
	Subjects: []string{
		"sync.>",
	},
	Retention: jetstream.WorkQueuePolicy,
	MaxAge:    14 * 24 * time.Hour,
	Storage:   jetstream.FileStorage,
})
if err != nil {
	zap.L().Warn("failed to create CMDB_SYNC stream", zap.Error(err))
}
```

- [ ] **Step 3: Run existing eventbus tests**

Run: `cd /cmdb-platform/cmdb-core && go test ./internal/eventbus/ -v`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/eventbus/subjects.go cmdb-core/internal/eventbus/nats.go
git commit -m "feat(sync): add sync event subjects and CMDB_SYNC NATS stream"
```

---

## Task 5: Domain services — add pool + sync_version increment

**Files:**
- Modify: `cmdb-core/internal/domain/asset/service.go`
- Modify: `cmdb-core/internal/domain/maintenance/service.go`
- Modify: `cmdb-core/internal/domain/topology/service.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add pool to Asset Service**

In `asset/service.go`, change struct (line ~26):
```go
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
	pool    *pgxpool.Pool
}
```

Update constructor:
```go
func NewService(queries *dbgen.Queries, bus eventbus.Bus, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, bus: bus, pool: pool}
}
```

Add helper at bottom of file:
```go
func (s *Service) incrementSyncVersion(ctx context.Context, table string, id uuid.UUID) {
	if s.pool == nil {
		return
	}
	_, _ = s.pool.Exec(ctx, fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", table), id)
}
```

Add import for `"fmt"` if not present.

After each write in Create/Update/Delete, add:
```go
// In Create, after s.queries.CreateAsset:
s.incrementSyncVersion(ctx, "assets", a.ID)

// In Update, after s.queries.UpdateAsset:
s.incrementSyncVersion(ctx, "assets", a.ID)

// In Delete, after s.queries.DeleteAsset:
s.incrementSyncVersion(ctx, "assets", id)
```

- [ ] **Step 2: Add pool to Maintenance Service**

In `maintenance/service.go`, change struct (line ~18):
```go
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
	pool    *pgxpool.Pool
}
```

Update constructor:
```go
func NewService(queries *dbgen.Queries, bus eventbus.Bus, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, bus: bus, pool: pool}
}
```

Add same `incrementSyncVersion` helper. After Create and Transition writes:
```go
// In Create, after s.queries.CreateWorkOrder:
s.incrementSyncVersion(ctx, "work_orders", order.ID)

// In Transition, after status update:
s.incrementSyncVersion(ctx, "work_orders", id)
```

- [ ] **Step 3: Add sync_version to Topology Service**

Topology already has `pool`. Add `incrementSyncVersion` helper and call after each location/rack Create/Update/Delete:
```go
// After CreateLocation:
s.incrementSyncVersion(ctx, "locations", loc.ID)

// After CreateRack:
s.incrementSyncVersion(ctx, "racks", rack.ID)
// (same pattern for Update/Delete)
```

- [ ] **Step 4: Update main.go constructor calls**

Find where services are constructed and add `pool` parameter:
```go
// Asset service — add pool
assetSvc := asset.NewService(queries, bus, pool)

// Maintenance service — add pool
maintenanceSvc := maintenance.NewService(queries, bus, pool)
```

Topology already receives pool.

- [ ] **Step 5: Build and test**

Run: `cd /cmdb-platform/cmdb-core && go build ./... && go test ./... -race -count=1`

Expected: BUILD OK, all tests PASS

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/internal/domain/asset/service.go cmdb-core/internal/domain/topology/service.go cmdb-core/internal/domain/maintenance/service.go cmdb-core/cmd/server/main.go
git commit -m "feat(sync): increment sync_version on all domain writes"
```

---

## Task 6: Sync Service — event subscription + envelope publishing

**Files:**
- Create: `cmdb-core/internal/domain/sync/service.go`

- [ ] **Step 1: Write SyncService**

```go
// service.go
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Service handles sync envelope creation and distribution.
type Service struct {
	pool   *pgxpool.Pool
	bus    eventbus.Bus
	cfg    *config.Config
	nodeID string
}

// NewService creates a SyncService.
func NewService(pool *pgxpool.Pool, bus eventbus.Bus, cfg *config.Config) *Service {
	nodeID := cfg.EdgeNodeID
	if nodeID == "" {
		nodeID = "central"
	}
	return &Service{pool: pool, bus: bus, cfg: cfg, nodeID: nodeID}
}

// RegisterSubscribers subscribes to all domain events and wraps them as SyncEnvelopes.
func (s *Service) RegisterSubscribers() {
	if s.bus == nil {
		return
	}

	subjects := []struct {
		subject    string
		entityType string
		action     string
	}{
		{eventbus.SubjectAssetCreated, "assets", "create"},
		{eventbus.SubjectAssetUpdated, "assets", "update"},
		{eventbus.SubjectAssetDeleted, "assets", "delete"},
		{eventbus.SubjectLocationCreated, "locations", "create"},
		{eventbus.SubjectLocationUpdated, "locations", "update"},
		{eventbus.SubjectLocationDeleted, "locations", "delete"},
		{eventbus.SubjectRackCreated, "racks", "create"},
		{eventbus.SubjectRackUpdated, "racks", "update"},
		{eventbus.SubjectRackDeleted, "racks", "delete"},
		{eventbus.SubjectOrderCreated, "work_orders", "create"},
		{eventbus.SubjectOrderTransitioned, "work_orders", "update"},
		{eventbus.SubjectAlertFired, "alert_events", "create"},
		{eventbus.SubjectAlertResolved, "alert_events", "update"},
		{eventbus.SubjectInventoryTaskCreated, "inventory_tasks", "create"},
		{eventbus.SubjectInventoryTaskCompleted, "inventory_tasks", "update"},
	}

	for _, sub := range subjects {
		sub := sub
		s.bus.Subscribe(sub.subject, func(ctx context.Context, event eventbus.Event) error {
			return s.onDomainEvent(ctx, event, sub.entityType, sub.action)
		})
	}

	zap.L().Info("sync subscribers registered", zap.Int("count", len(subjects)))
}

func (s *Service) onDomainEvent(ctx context.Context, event eventbus.Event, entityType, action string) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil
	}

	// Extract entity ID from payload (convention: first field ending in _id matching entity type)
	entityID := extractEntityID(payload, entityType)
	if entityID == "" {
		return nil
	}

	// Query current sync_version
	var version int64
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT sync_version FROM %s WHERE id = $1", entityType),
		entityID).Scan(&version)
	if err != nil {
		version = 0
	}

	env := NewEnvelope(s.nodeID, event.TenantID, entityType, entityID, action, version, event.Payload)

	// Publish to sync stream
	syncSubject := fmt.Sprintf("sync.%s.%s.%s", event.TenantID, entityType, action)
	data, _ := json.Marshal(env)
	return s.bus.Publish(ctx, eventbus.Event{
		Subject:  syncSubject,
		TenantID: event.TenantID,
		Payload:  data,
	})
}

// StartReconciliation runs a background task to check sync state every 5 minutes.
func (s *Service) StartReconciliation(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				s.reconcile(ctx)
			}
		}
	}()
	zap.L().Info("sync reconciliation started (5m interval)")
}

func (s *Service) reconcile(ctx context.Context) {
	// Check for stale sync_state entries and log warnings
	rows, err := s.pool.Query(ctx,
		"SELECT node_id, entity_type, last_sync_version, last_sync_at FROM sync_state WHERE status = 'active' AND last_sync_at < now() - interval '1 hour'")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var nodeID, entityType string
		var version int64
		var lastSync time.Time
		if rows.Scan(&nodeID, &entityType, &version, &lastSync) == nil {
			zap.L().Warn("sync lag detected",
				zap.String("node_id", nodeID),
				zap.String("entity_type", entityType),
				zap.Int64("version", version),
				zap.Time("last_sync", lastSync))
		}
	}
}

func extractEntityID(payload map[string]interface{}, entityType string) string {
	// Try common ID field names
	keys := []string{"asset_id", "location_id", "rack_id", "order_id", "alert_id", "task_id", "id"}
	for _, k := range keys {
		if v, ok := payload[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
```

- [ ] **Step 2: Build and verify**

Run: `cd /cmdb-platform/cmdb-core && go build ./...`

Expected: BUILD OK

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/internal/domain/sync/service.go
git commit -m "feat(sync): SyncService with event subscription and reconciliation"
```

---

## Task 7: Sync API endpoints

**Files:**
- Create: `cmdb-core/internal/api/sync_endpoints.go`
- Modify: `cmdb-core/internal/api/impl.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Write sync_endpoints.go**

```go
package api

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// SyncGetChanges returns incremental changes for a given entity type since a version.
// GET /api/v1/sync/changes?entity_type=assets&since_version=0&limit=100
func (s *APIServer) SyncGetChanges(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	entityType := c.Query("entity_type")
	if entityType == "" {
		response.BadRequest(c, "entity_type is required")
		return
	}

	sinceVersion, _ := strconv.ParseInt(c.DefaultQuery("since_version", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit > 1000 {
		limit = 1000
	}

	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
	}
	if !allowedTables[entityType] {
		response.BadRequest(c, "invalid entity_type")
		return
	}

	query := fmt.Sprintf(
		"SELECT id, sync_version FROM %s WHERE tenant_id = $1 AND sync_version > $2 ORDER BY sync_version LIMIT $3",
		entityType)

	rows, err := s.pool.Query(c.Request.Context(), query, tenantID, sinceVersion, limit+1)
	if err != nil {
		response.InternalError(c, "failed to query changes")
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var id uuid.UUID
		var version int64
		if rows.Scan(&id, &version) == nil {
			items = append(items, gin.H{"entity_id": id, "entity_type": entityType, "version": version})
		}
	}
	if items == nil {
		items = []gin.H{}
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var latestVersion int64
	if len(items) > 0 {
		if v, ok := items[len(items)-1]["version"].(int64); ok {
			latestVersion = v
		}
	}

	response.OK(c, gin.H{
		"changes":        items,
		"has_more":       hasMore,
		"latest_version": latestVersion,
	})
}

// SyncGetState returns sync state for all entity types.
// GET /api/v1/sync/state
func (s *APIServer) SyncGetState(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(),
		"SELECT node_id, entity_type, last_sync_version, last_sync_at, status, error_message FROM sync_state WHERE tenant_id = $1 ORDER BY node_id, entity_type",
		tenantID)
	if err != nil {
		response.InternalError(c, "failed to query sync state")
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var nodeID, entityType, status string
		var version int64
		var lastSync, errMsg interface{}
		if rows.Scan(&nodeID, &entityType, &version, &lastSync, &status, &errMsg) == nil {
			items = append(items, gin.H{
				"node_id": nodeID, "entity_type": entityType,
				"last_sync_version": version, "last_sync_at": lastSync,
				"status": status, "error_message": errMsg,
			})
		}
	}
	if items == nil {
		items = []gin.H{}
	}
	response.OK(c, items)
}

// SyncGetConflicts returns pending sync conflicts.
// GET /api/v1/sync/conflicts
func (s *APIServer) SyncGetConflicts(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(),
		"SELECT id, entity_type, entity_id, local_version, remote_version, local_diff, remote_diff, created_at FROM sync_conflicts WHERE tenant_id = $1 AND resolution = 'pending' ORDER BY created_at",
		tenantID)
	if err != nil {
		response.InternalError(c, "failed to query conflicts")
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var id, entityID uuid.UUID
		var entityType string
		var localV, remoteV int64
		var localDiff, remoteDiff, createdAt interface{}
		if rows.Scan(&id, &entityType, &entityID, &localV, &remoteV, &localDiff, &remoteDiff, &createdAt) == nil {
			items = append(items, gin.H{
				"id": id, "entity_type": entityType, "entity_id": entityID,
				"local_version": localV, "remote_version": remoteV,
				"local_diff": localDiff, "remote_diff": remoteDiff,
				"created_at": createdAt,
			})
		}
	}
	if items == nil {
		items = []gin.H{}
	}
	response.OK(c, items)
}

// SyncResolveConflict resolves a sync conflict.
// POST /api/v1/sync/conflicts/:id/resolve
func (s *APIServer) SyncResolveConflict(c *gin.Context) {
	userID := userIDFromContext(c)
	conflictID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid conflict ID")
		return
	}

	var req struct {
		Resolution string `json:"resolution" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "resolution is required (local_wins or remote_wins)")
		return
	}
	if req.Resolution != "local_wins" && req.Resolution != "remote_wins" {
		response.BadRequest(c, "resolution must be local_wins or remote_wins")
		return
	}

	_, err = s.pool.Exec(c.Request.Context(),
		"UPDATE sync_conflicts SET resolution = $1, resolved_by = $2, resolved_at = now() WHERE id = $3",
		req.Resolution, userID, conflictID)
	if err != nil {
		response.InternalError(c, "failed to resolve conflict")
		return
	}

	c.Status(204)
}

// SyncSnapshot streams a full snapshot of an entity type for initial sync.
// GET /api/v1/sync/snapshot?entity_type=assets
func (s *APIServer) SyncSnapshot(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	entityType := c.Query("entity_type")

	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
	}
	if !allowedTables[entityType] {
		response.BadRequest(c, "invalid entity_type")
		return
	}

	query := fmt.Sprintf("SELECT row_to_json(t) FROM %s t WHERE tenant_id = $1 ORDER BY sync_version", entityType)
	rows, err := s.pool.Query(c.Request.Context(), query, tenantID)
	if err != nil {
		response.InternalError(c, "failed to query snapshot")
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "application/x-ndjson")
	c.Status(200)

	for rows.Next() {
		var jsonRow []byte
		if rows.Scan(&jsonRow) == nil {
			c.Writer.Write(jsonRow)
			c.Writer.Write([]byte("\n"))
		}
	}
}
```

- [ ] **Step 2: Add syncSvc to APIServer (impl.go)**

In APIServer struct, add:
```go
syncSvc *sync.Service
```

Update NewAPIServer to accept and store it.

- [ ] **Step 3: Register routes in main.go**

After existing route registration, add:
```go
// Sync endpoints
v1.GET("/sync/changes", apiServer.SyncGetChanges)
v1.GET("/sync/state", apiServer.SyncGetState)
v1.GET("/sync/conflicts", apiServer.SyncGetConflicts)
v1.POST("/sync/conflicts/:id/resolve", apiServer.SyncResolveConflict)
v1.GET("/sync/snapshot", apiServer.SyncSnapshot)
```

Also initialize SyncService and pass to APIServer:
```go
var syncSvc *sync.Service
if cfg.SyncEnabled && bus != nil {
	syncSvc = sync.NewService(pool, bus, cfg)
	syncSvc.RegisterSubscribers()
	syncSvc.StartReconciliation(ctx)
	zap.L().Info("Sync service started")
}
```

- [ ] **Step 4: Build and test**

Run: `cd /cmdb-platform/cmdb-core && go build ./... && go test ./... -race -count=1`

Expected: BUILD OK, all tests PASS

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/api/sync_endpoints.go cmdb-core/internal/api/impl.go cmdb-core/cmd/server/main.go
git commit -m "feat(sync): add 5 sync API endpoints — changes, state, conflicts, resolve, snapshot"
```

---

## Task 8: Sync Agent (Edge mode only)

**Files:**
- Create: `cmdb-core/internal/domain/sync/agent.go`

- [ ] **Step 1: Write SyncAgent**

```go
// agent.go
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Agent runs on Edge nodes to handle initial sync and incremental apply.
type Agent struct {
	pool   *pgxpool.Pool
	bus    eventbus.Bus
	cfg    *config.Config
	nodeID string
}

// NewAgent creates a SyncAgent for Edge nodes.
func NewAgent(pool *pgxpool.Pool, bus eventbus.Bus, cfg *config.Config) *Agent {
	return &Agent{pool: pool, bus: bus, cfg: cfg, nodeID: cfg.EdgeNodeID}
}

// Start runs the sync agent lifecycle.
func (a *Agent) Start(ctx context.Context) {
	// Check if initial sync is needed
	var count int
	err := a.pool.QueryRow(ctx, "SELECT count(*) FROM sync_state WHERE node_id = $1", a.nodeID).Scan(&count)
	if err != nil || count == 0 {
		zap.L().Info("sync agent: no sync state found, initial sync may be needed",
			zap.String("node_id", a.nodeID))
	}

	// Subscribe to incoming sync envelopes from Central
	if a.bus != nil {
		a.bus.Subscribe("sync.>", func(ctx context.Context, event eventbus.Event) error {
			return a.handleIncomingEnvelope(ctx, event)
		})
		zap.L().Info("sync agent: listening for sync envelopes", zap.String("node_id", a.nodeID))
	}

	// Periodic state update
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				a.updateSyncState(ctx)
			}
		}
	}()
}

func (a *Agent) handleIncomingEnvelope(ctx context.Context, event eventbus.Event) error {
	var env SyncEnvelope
	if err := json.Unmarshal(event.Payload, &env); err != nil {
		zap.L().Warn("sync agent: invalid envelope", zap.Error(err))
		return nil
	}

	// Skip our own envelopes
	if env.Source == a.nodeID {
		return nil
	}

	// Verify checksum
	if !env.VerifyChecksum() {
		zap.L().Warn("sync agent: checksum mismatch", zap.String("id", env.ID))
		return nil
	}

	// Check layer order
	layer := LayerOf(env.EntityType)
	if layer < 0 {
		zap.L().Warn("sync agent: unknown entity type", zap.String("type", env.EntityType))
		return nil
	}

	zap.L().Debug("sync agent: received envelope",
		zap.String("entity_type", env.EntityType),
		zap.String("entity_id", env.EntityID),
		zap.String("action", env.Action),
		zap.Int64("version", env.Version))

	// Update sync state
	_, _ = a.pool.Exec(ctx,
		`INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
		 VALUES ($1, $2, $3, $4, now(), 'active')
		 ON CONFLICT (node_id, entity_type) DO UPDATE SET last_sync_version = GREATEST(sync_state.last_sync_version, $4), last_sync_at = now(), status = 'active'`,
		a.nodeID, env.TenantID, env.EntityType, env.Version)

	return nil
}

func (a *Agent) updateSyncState(ctx context.Context) {
	// For each syncable table, record current max sync_version
	tables := []string{"assets", "locations", "racks", "work_orders", "alert_events", "inventory_tasks"}
	for _, table := range tables {
		var maxVersion int64
		err := a.pool.QueryRow(ctx,
			fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s", table)).Scan(&maxVersion)
		if err != nil {
			continue
		}
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
			 VALUES ($1, $2, $3, $4, now(), 'active')
			 ON CONFLICT (node_id, entity_type) DO UPDATE SET last_sync_version = $4, last_sync_at = now()`,
			a.nodeID, a.cfg.TenantID, table, maxVersion)
	}
}
```

- [ ] **Step 2: Wire agent startup in main.go**

In main.go, within the sync initialization block:
```go
if cfg.SyncEnabled && bus != nil {
	syncSvc = sync.NewService(pool, bus, cfg)
	syncSvc.RegisterSubscribers()
	syncSvc.StartReconciliation(ctx)

	if cfg.DeployMode == "edge" && cfg.EdgeNodeID != "" {
		agent := sync.NewAgent(pool, bus, cfg)
		go agent.Start(ctx)
		zap.L().Info("Sync agent started", zap.String("node_id", cfg.EdgeNodeID))
	}
}
```

- [ ] **Step 3: Build and test**

Run: `cd /cmdb-platform/cmdb-core && go build ./... && go test ./... -race -count=1`

Expected: BUILD OK, all tests PASS

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/domain/sync/agent.go cmdb-core/cmd/server/main.go
git commit -m "feat(sync): SyncAgent for Edge nodes — envelope handling and state tracking"
```

---

## Task 9: Integration test — end-to-end sync_version verification

**Files:**
- Create: `cmdb-core/internal/domain/sync/envelope_test.go`

- [ ] **Step 1: Write envelope tests**

```go
package sync

import (
	"encoding/json"
	"testing"
)

func TestNewEnvelope(t *testing.T) {
	diff := json.RawMessage(`{"name":"test"}`)
	env := NewEnvelope("central", "tenant-1", "assets", "asset-1", "create", 1, diff)

	if env.Source != "central" {
		t.Errorf("expected source 'central', got %q", env.Source)
	}
	if env.EntityType != "assets" {
		t.Errorf("expected entity_type 'assets', got %q", env.EntityType)
	}
	if env.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if env.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestEnvelopeVerifyChecksum(t *testing.T) {
	diff := json.RawMessage(`{"status":"active"}`)
	env := NewEnvelope("edge-1", "t1", "assets", "a1", "update", 5, diff)

	if !env.VerifyChecksum() {
		t.Error("checksum should verify for unmodified envelope")
	}

	// Tamper with the payload
	env.Diff = json.RawMessage(`{"status":"hacked"}`)
	if env.VerifyChecksum() {
		t.Error("checksum should fail for tampered envelope")
	}
}

func TestEnvelopeJSON(t *testing.T) {
	diff := json.RawMessage(`{"x":1}`)
	env := NewEnvelope("central", "t1", "racks", "r1", "delete", 10, diff)

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded SyncEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.EntityID != env.EntityID {
		t.Errorf("entity_id mismatch: %q vs %q", decoded.EntityID, env.EntityID)
	}
	if decoded.Checksum != env.Checksum {
		t.Errorf("checksum mismatch")
	}
}

func TestLayerOf(t *testing.T) {
	tests := []struct {
		entity   string
		expected int
	}{
		{"locations", 0},
		{"assets", 1},
		{"racks", 1},
		{"rack_slots", 2},
		{"work_orders", 3},
		{"audit_events", 4},
		{"unknown_table", -1},
	}
	for _, tt := range tests {
		t.Run(tt.entity, func(t *testing.T) {
			got := LayerOf(tt.entity)
			if got != tt.expected {
				t.Errorf("LayerOf(%q) = %d, want %d", tt.entity, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run all tests**

Run: `cd /cmdb-platform/cmdb-core && go test ./... -race -count=1 -v 2>&1 | tail -20`

Expected: All tests PASS including new sync package tests

- [ ] **Step 3: Final build verification**

Run: `cd /cmdb-platform/cmdb-core && go build -o /dev/null ./cmd/server`

Expected: BUILD OK

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/domain/sync/envelope_test.go
git commit -m "test(sync): add envelope, layer, and checksum unit tests"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-04-13-edge-sync-phase1.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
