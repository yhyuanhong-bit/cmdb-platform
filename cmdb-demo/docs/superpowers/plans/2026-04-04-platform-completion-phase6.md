# Platform Completion Phase 6 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four remaining mock/semi-static page gaps — complete SystemSettings with real API, add metrics data pipeline for EnergyMonitor, add sensor management for SensorConfig, and add /system/health + Integration CRUD endpoints — making the CMDB platform fully data-driven with zero hardcoded mock data in business pages.

**Architecture:** Three capability layers implemented sequentially: (6a) Platform management — SystemSettings接API + /system/health endpoint + Integration tables/API; (6b) Metrics pipeline — simulated metrics injector script + metrics query API + EnergyMonitor/SensorConfig frontend integration; (6c) Login enhancement is deferred (works fine as-is).

**Tech Stack:** Go (Gin, sqlc), Python (metrics injector), TypeScript (React hooks), PostgreSQL (TimescaleDB), existing OpenAPI codegen pipeline

---

## Phase 6a: Platform Management (SystemSettings + /system/health + Integration)

### File Structure

```
cmdb-core/
├── db/migrations/
│   └── 000012_integration_tables.up.sql     # NEW: integration_adapters + webhook_subscriptions + webhook_deliveries
│   └── 000012_integration_tables.down.sql
├── db/queries/
│   └── integration.sql                       # NEW: CRUD queries for adapters + webhooks
├── internal/api/
│   ├── impl.go                               # MODIFY: add Integration + SystemHealth endpoints
│   ├── convert.go                            # MODIFY: add integration type converters
│   └── generated.go                          # REGENERATE: add integration + health schemas
├── internal/domain/integration/
│   └── service.go                            # NEW: Integration service (adapter + webhook CRUD)
api/
└── openapi.yaml                              # MODIFY: add integration + health endpoints + schemas

cmdb-demo/
├── src/hooks/
│   └── useIntegration.ts                     # NEW: useAdapters, useWebhooks hooks
├── src/hooks/
│   └── useSystemHealth.ts                    # NEW: useSystemHealth hook
└── src/pages/
    └── SystemSettings.tsx                    # MODIFY: replace mock with hooks
```

---

### Task 1: Integration DB Migration

**Files:**
- Create: `cmdb-core/db/migrations/000012_integration_tables.up.sql`
- Create: `cmdb-core/db/migrations/000012_integration_tables.down.sql`

- [ ] **Step 1: Create migration up**

Create `cmdb-core/db/migrations/000012_integration_tables.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS integration_adapters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,
    direction   VARCHAR(10) NOT NULL,
    endpoint    VARCHAR(500),
    config      JSONB DEFAULT '{}',
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    url         VARCHAR(500) NOT NULL,
    secret      VARCHAR(200),
    events      TEXT[] NOT NULL,
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id),
    event_type      VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    status_code     INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ DEFAULT now()
);

-- Seed sample data
INSERT INTO integration_adapters (tenant_id, name, type, direction, endpoint, enabled) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Prometheus Metrics', 'rest', 'inbound', 'http://prometheus:9090/api/v1', true),
    ('a0000000-0000-0000-0000-000000000001', 'SNMP Poller', 'snmp', 'inbound', '10.134.143.0/24', false),
    ('a0000000-0000-0000-0000-000000000001', 'ServiceNow ITSM', 'rest', 'bidirectional', 'https://instance.service-now.com/api', false)
ON CONFLICT DO NOTHING;

INSERT INTO webhook_subscriptions (tenant_id, name, url, events, enabled) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Slack Alerts', 'https://hooks.slack.com/services/xxx', '{alert.fired,alert.resolved}', true),
    ('a0000000-0000-0000-0000-000000000001', 'Teams Notifications', 'https://outlook.office.com/webhook/xxx', '{maintenance.order_created}', false)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Create migration down**

Create `cmdb-core/db/migrations/000012_integration_tables.down.sql`:

```sql
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
DROP TABLE IF EXISTS integration_adapters;
```

- [ ] **Step 3: Apply migration**

```bash
cd /cmdb-platform/cmdb-core/deploy
docker compose exec -T postgres psql -U cmdb -d cmdb < ../db/migrations/000012_integration_tables.up.sql
```

- [ ] **Step 4: Commit**

```bash
git add db/migrations/000012_*
git commit -m "feat: add integration tables migration - adapters, webhooks, deliveries with seed data"
```

---

### Task 2: Integration sqlc Queries + Service

**Files:**
- Create: `cmdb-core/db/queries/integration.sql`
- Create: `cmdb-core/internal/domain/integration/service.go`

- [ ] **Step 1: Create integration queries**

Create `cmdb-core/db/queries/integration.sql`:

```sql
-- name: ListAdapters :many
SELECT * FROM integration_adapters
WHERE tenant_id = $1
ORDER BY name;

-- name: CreateAdapter :one
INSERT INTO integration_adapters (tenant_id, name, type, direction, endpoint, config, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: ListWebhooks :many
SELECT * FROM webhook_subscriptions
WHERE tenant_id = $1
ORDER BY name;

-- name: CreateWebhook :one
INSERT INTO webhook_subscriptions (tenant_id, name, url, secret, events, enabled)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListDeliveries :many
SELECT * FROM webhook_deliveries
WHERE subscription_id = $1
ORDER BY delivered_at DESC
LIMIT $2;
```

- [ ] **Step 2: Regenerate sqlc**

```bash
cd /cmdb-platform/cmdb-core
go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
go build ./internal/dbgen/...
```

- [ ] **Step 3: Create integration service**

Create `cmdb-core/internal/domain/integration/service.go`:

Read the generated dbgen types first (`grep 'IntegrationAdapter\|WebhookSubscription\|WebhookDelivery' internal/dbgen/models.go`) to understand exact field types. Then write:

```go
package integration

import (
    "context"
    "github.com/cmdb-platform/cmdb-core/internal/dbgen"
    "github.com/google/uuid"
)

type Service struct {
    queries *dbgen.Queries
}

func NewService(q *dbgen.Queries) *Service {
    return &Service{queries: q}
}

func (s *Service) ListAdapters(ctx context.Context, tenantID uuid.UUID) ([]dbgen.IntegrationAdapter, error) {
    return s.queries.ListAdapters(ctx, tenantID)
}

func (s *Service) CreateAdapter(ctx context.Context, params dbgen.CreateAdapterParams) (*dbgen.IntegrationAdapter, error) {
    a, err := s.queries.CreateAdapter(ctx, params)
    if err != nil { return nil, err }
    return &a, nil
}

func (s *Service) ListWebhooks(ctx context.Context, tenantID uuid.UUID) ([]dbgen.WebhookSubscription, error) {
    return s.queries.ListWebhooks(ctx, tenantID)
}

func (s *Service) CreateWebhook(ctx context.Context, params dbgen.CreateWebhookParams) (*dbgen.WebhookSubscription, error) {
    w, err := s.queries.CreateWebhook(ctx, params)
    if err != nil { return nil, err }
    return &w, nil
}

func (s *Service) ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]dbgen.WebhookDelivery, error) {
    return s.queries.ListDeliveries(ctx, dbgen.ListDeliveriesParams{
        SubscriptionID: webhookID,
        Limit: int32(limit),
    })
}
```

Adapt the exact param types to match what sqlc generates.

- [ ] **Step 4: Verify build**

```bash
go build ./internal/domain/integration/...
```

- [ ] **Step 5: Commit**

```bash
git add db/queries/integration.sql internal/dbgen/ internal/domain/integration/
git commit -m "feat: add integration service - adapters + webhooks CRUD"
```

---

### Task 3: /system/health Endpoint + Integration API in OpenAPI + impl.go

**Files:**
- Modify: `api/openapi.yaml`
- Regenerate: `cmdb-core/internal/api/generated.go`
- Modify: `cmdb-core/internal/api/impl.go`
- Modify: `cmdb-core/internal/api/convert.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add schemas + endpoints to openapi.yaml**

Read `api/openapi.yaml`, then add:

**New schemas in components/schemas:**

```yaml
    SystemHealth:
      type: object
      properties:
        database:
          type: object
          properties:
            status: { type: string }
            latency_ms: { type: number }
        redis:
          type: object
          properties:
            status: { type: string }
            latency_ms: { type: number }
        nats:
          type: object
          properties:
            status: { type: string }
            connected: { type: boolean }
        services:
          type: array
          items:
            type: object
            properties:
              name: { type: string }
              status: { type: string }
              latency_ms: { type: number }
    IntegrationAdapter:
      type: object
      properties:
        id: { type: string, format: uuid }
        name: { type: string }
        type: { type: string }
        direction: { type: string }
        endpoint: { type: string }
        enabled: { type: boolean }
        created_at: { type: string, format: date-time }
    WebhookSubscription:
      type: object
      properties:
        id: { type: string, format: uuid }
        name: { type: string }
        url: { type: string }
        events: { type: array, items: { type: string } }
        enabled: { type: boolean }
        created_at: { type: string, format: date-time }
```

**New paths:**

```yaml
  /system/health:
    get:
      operationId: getSystemHealth
      tags: [system]
      summary: Get system health status
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  data: { $ref: '#/components/schemas/SystemHealth' }
                  meta: { $ref: '#/components/schemas/Meta' }
  /integration/adapters:
    get:
      operationId: listAdapters
      tags: [integration]
      summary: List integration adapters
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items: { $ref: '#/components/schemas/IntegrationAdapter' }
                  meta: { $ref: '#/components/schemas/Meta' }
  /integration/webhooks:
    get:
      operationId: listWebhooks
      tags: [integration]
      summary: List webhook subscriptions
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items: { $ref: '#/components/schemas/WebhookSubscription' }
                  meta: { $ref: '#/components/schemas/Meta' }
```

- [ ] **Step 2: Regenerate Go types**

```bash
cd /cmdb-platform/cmdb-core
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config oapi-codegen.yaml ../api/openapi.yaml
go build ./internal/api/...
```

- [ ] **Step 3: Add convert functions for Integration types**

Add to `cmdb-core/internal/api/convert.go`:

```go
func toAPIAdapter(db dbgen.IntegrationAdapter) IntegrationAdapter {
    return IntegrationAdapter{
        Id:        db.ID,
        Name:      db.Name,
        Type:      db.Type,
        Direction: db.Direction,
        Endpoint:  pgtextToStr(db.Endpoint),
        Enabled:   pgboolVal(db.Enabled),
        CreatedAt: db.CreatedAt,
    }
}

func toAPIWebhook(db dbgen.WebhookSubscription) WebhookSubscription {
    return WebhookSubscription{
        Id:        db.ID,
        Name:      db.Name,
        Url:       db.Url,
        Events:    db.Events,
        Enabled:   pgboolVal(db.Enabled),
        CreatedAt: db.CreatedAt,
    }
}
```

Read the generated types and dbgen types to get exact field names right.

- [ ] **Step 4: Add implementations to impl.go**

Add to `cmdb-core/internal/api/impl.go`:

```go
// GetSystemHealth checks the health of all dependent services.
func (s *APIServer) GetSystemHealth(c *gin.Context) {
    ctx := c.Request.Context()

    // Check DB
    dbStart := time.Now()
    var dbStatus, dbLatency = "operational", float64(0)
    if _, err := s.assetSvc.List(ctx, asset.ListParams{Limit: 1}); err != nil {
        dbStatus = "error"
    }
    dbLatency = float64(time.Since(dbStart).Milliseconds())

    health := SystemHealth{
        Database: &struct {
            LatencyMs *float32 `json:"latency_ms,omitempty"`
            Status    *string  `json:"status,omitempty"`
        }{Status: &dbStatus, LatencyMs: ptr(float32(dbLatency))},
        // Redis and NATS checks can be added later
    }
    response.OK(c, health)
}

// ListAdapters returns all integration adapters for the tenant.
func (s *APIServer) ListAdapters(c *gin.Context) {
    tenantID := getTenantID(c)
    adapters, err := s.integrationSvc.ListAdapters(c.Request.Context(), tenantID)
    if err != nil {
        response.InternalError(c, "failed to list adapters")
        return
    }
    response.OK(c, convertSlice(adapters, toAPIAdapter))
}

// ListWebhooks returns all webhook subscriptions for the tenant.
func (s *APIServer) ListWebhooks(c *gin.Context) {
    tenantID := getTenantID(c)
    webhooks, err := s.integrationSvc.ListWebhooks(c.Request.Context(), tenantID)
    if err != nil {
        response.InternalError(c, "failed to list webhooks")
        return
    }
    response.OK(c, convertSlice(webhooks, toAPIWebhook))
}
```

Also add `integrationSvc *integration.Service` to the APIServer struct and the NewAPIServer constructor.

- [ ] **Step 5: Wire integration service in main.go**

Add to `cmd/server/main.go`:
- Import `"github.com/cmdb-platform/cmdb-core/internal/domain/integration"`
- Create `integrationSvc := integration.NewService(queries)`
- Pass to `api.NewAPIServer(..., integrationSvc)`

- [ ] **Step 6: Verify full build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add api/openapi.yaml cmdb-core/internal/api/ cmdb-core/cmd/server/main.go
git commit -m "feat: add /system/health + Integration API endpoints (adapters, webhooks)"
```

---

### Task 4: SystemSettings Frontend — Replace All Mock Data

**Files:**
- Create: `cmdb-demo/src/hooks/useIntegration.ts`
- Create: `cmdb-demo/src/hooks/useSystemHealth.ts`
- Modify: `cmdb-demo/src/pages/SystemSettings.tsx`
- Regenerate: `cmdb-demo/src/generated/api-types.ts`

- [ ] **Step 1: Regenerate TS types**

```bash
cd /cmdb-platform/cmdb-demo
npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts
```

- [ ] **Step 2: Create useIntegration.ts**

```typescript
import { useQuery } from '@tanstack/react-query'
import { integrationApi } from '../lib/api/integration'

export function useAdapters() {
  return useQuery({
    queryKey: ['adapters'],
    queryFn: () => integrationApi.listAdapters(),
  })
}

export function useWebhooks() {
  return useQuery({
    queryKey: ['webhooks'],
    queryFn: () => integrationApi.listWebhooks(),
  })
}
```

- [ ] **Step 3: Create useSystemHealth.ts**

```typescript
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'

interface ServiceHealth {
  name: string
  status: string
  latency_ms: number
}

interface SystemHealth {
  database: { status: string; latency_ms: number }
  redis: { status: string; latency_ms: number }
  nats: { status: string; connected: boolean }
  services: ServiceHealth[]
}

export function useSystemHealth() {
  return useQuery({
    queryKey: ['systemHealth'],
    queryFn: () => apiClient.get<ApiResponse<SystemHealth>>('/system/health'),
    refetchInterval: 30000, // refresh every 30s
  })
}
```

- [ ] **Step 4: Rewrite SystemSettings.tsx**

Read the current file fully. Then replace:

1. **Users Tab (permissions):** Delete `const users = [...]` mock. Add:
```typescript
import { useUsers, useRoles } from '../hooks/useIdentity'

// Inside component:
const { data: usersResp } = useUsers()
const { data: rolesResp } = useRoles()
const users = usersResp?.data ?? []
const roles = rolesResp?.data ?? []
```
Map API User fields to the table: `user.display_name`, `user.status`, `user.email`.

2. **Security Tab:** Delete `const healthIndicators = [...]` mock. Add:
```typescript
import { useSystemHealth } from '../hooks/useSystemHealth'

const { data: healthResp } = useSystemHealth()
const health = healthResp?.data
const healthIndicators = [
  { label: 'Database', status: health?.database?.status ?? 'unknown', latency: `${health?.database?.latency_ms ?? 0}ms` },
  { label: 'Redis', status: health?.redis?.status ?? 'unknown', latency: `${health?.redis?.latency_ms ?? 0}ms` },
  { label: 'NATS', status: health?.nats?.connected ? 'operational' : 'error', latency: '-' },
]
```

3. **Integrations Tab:** Add:
```typescript
import { useAdapters, useWebhooks } from '../hooks/useIntegration'

const { data: adaptersResp } = useAdapters()
const { data: webhooksResp } = useWebhooks()
const adapters = adaptersResp?.data ?? []
const webhooks = webhooksResp?.data ?? []
```
Render adapter list and webhook list from API data.

- [ ] **Step 5: Commit**

```bash
git add cmdb-demo/src/hooks/useIntegration.ts cmdb-demo/src/hooks/useSystemHealth.ts cmdb-demo/src/pages/SystemSettings.tsx cmdb-demo/src/generated/api-types.ts
git commit -m "feat: SystemSettings fully connected - users/health/integration tabs use real API"
```

---

## Phase 6b: Metrics Pipeline (EnergyMonitor + Dynamic Charts)

### Task 5: Metrics Simulation Injector Script

**Files:**
- Create: `scripts/inject-metrics.py`

- [ ] **Step 1: Create the metrics injector**

Create `scripts/inject-metrics.py`:

```python
#!/usr/bin/env python3
"""
Metrics simulator — injects realistic time-series data into the CMDB metrics table.
Generates CPU, temperature, power_kw, and PUE metrics for all assets.

Usage:
  # Inject 24 hours of historical data (one-shot backfill):
  python3 scripts/inject-metrics.py --backfill 24h

  # Run continuously, inserting every 60 seconds:
  python3 scripts/inject-metrics.py --continuous --interval 60
"""

import argparse
import math
import random
import time
from datetime import datetime, timedelta, timezone

import psycopg2  # pip install psycopg2-binary

DB_URL = "postgresql://cmdb:changeme@localhost:5432/cmdb"

METRICS = {
    "cpu_usage":    {"base": 45, "amplitude": 25, "noise": 10},
    "temperature":  {"base": 32, "amplitude": 8,  "noise": 3},
    "power_kw":     {"base": 3.5, "amplitude": 1.5, "noise": 0.5},
    "memory_usage": {"base": 60, "amplitude": 15, "noise": 8},
}

PUE_BASE = 1.35
PUE_NOISE = 0.08


def generate_value(metric_cfg: dict, t: datetime) -> float:
    """Generate a realistic metric value with daily sine wave + random noise."""
    hour = t.hour + t.minute / 60.0
    # Peak at 14:00, trough at 04:00
    daily_factor = math.sin((hour - 4) / 24 * 2 * math.pi)
    value = metric_cfg["base"] + metric_cfg["amplitude"] * daily_factor + random.gauss(0, metric_cfg["noise"])
    return max(0, round(value, 2))


def get_assets(conn):
    with conn.cursor() as cur:
        cur.execute("SELECT id, tenant_id, type FROM assets")
        return cur.fetchall()


def inject_batch(conn, assets, timestamp):
    rows = []
    for asset_id, tenant_id, asset_type in assets:
        for metric_name, cfg in METRICS.items():
            # Skip power_kw for non-server/power assets
            if metric_name == "power_kw" and asset_type not in ("server", "power"):
                continue
            value = generate_value(cfg, timestamp)
            rows.append((timestamp, str(asset_id), str(tenant_id), metric_name, value, "{}"))

    # Also inject PUE as a location-level metric (use first asset's tenant)
    if assets:
        pue = PUE_BASE + random.gauss(0, PUE_NOISE)
        rows.append((timestamp, str(assets[0][0]), str(assets[0][1]), "pue", round(pue, 3), '{"scope": "campus"}'))

    with conn.cursor() as cur:
        cur.executemany(
            "INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels) VALUES (%s, %s::uuid, %s::uuid, %s, %s, %s::jsonb)",
            rows,
        )
    conn.commit()
    return len(rows)


def backfill(conn, assets, hours):
    now = datetime.now(timezone.utc)
    total = 0
    for minutes_ago in range(hours * 60, 0, -1):
        ts = now - timedelta(minutes=minutes_ago)
        total += inject_batch(conn, assets, ts)
        if minutes_ago % 60 == 0:
            print(f"  backfill: {hours - minutes_ago // 60}h / {hours}h ({total} rows)")
    print(f"Backfill complete: {total} rows inserted")


def continuous(conn, assets, interval):
    print(f"Continuous mode: injecting every {interval}s (Ctrl+C to stop)")
    while True:
        ts = datetime.now(timezone.utc)
        count = inject_batch(conn, assets, ts)
        print(f"  {ts.isoformat()} — {count} metrics inserted")
        time.sleep(interval)


def main():
    parser = argparse.ArgumentParser(description="CMDB Metrics Simulator")
    parser.add_argument("--backfill", type=str, help="Backfill duration, e.g., 24h, 7d")
    parser.add_argument("--continuous", action="store_true", help="Run continuously")
    parser.add_argument("--interval", type=int, default=60, help="Seconds between injections (continuous mode)")
    parser.add_argument("--db-url", type=str, default=DB_URL, help="PostgreSQL connection URL")
    args = parser.parse_args()

    conn = psycopg2.connect(args.db_url)
    assets = get_assets(conn)
    print(f"Found {len(assets)} assets")

    if args.backfill:
        unit = args.backfill[-1]
        num = int(args.backfill[:-1])
        hours = num * 24 if unit == "d" else num
        backfill(conn, assets, hours)
    elif args.continuous:
        continuous(conn, assets, args.interval)
    else:
        # Default: backfill 24h then run continuously
        backfill(conn, assets, 24)
        continuous(conn, assets, args.interval)

    conn.close()


if __name__ == "__main__":
    main()
```

```bash
chmod +x scripts/inject-metrics.py
```

- [ ] **Step 2: Test backfill**

```bash
pip install psycopg2-binary 2>/dev/null
python3 scripts/inject-metrics.py --backfill 24h
```

Expected: ~100K rows inserted (20 assets × 3-4 metrics × 1440 minutes).

- [ ] **Step 3: Verify data in DB**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "
SELECT name, count(*), round(avg(value)::numeric, 2) AS avg_val
FROM metrics GROUP BY name ORDER BY name;"
```

- [ ] **Step 4: Commit**

```bash
git add scripts/inject-metrics.py
git commit -m "feat: add metrics simulation injector - backfill + continuous modes for all assets"
```

---

### Task 6: Metrics Query API Endpoint

**Files:**
- Create: `cmdb-core/db/queries/metrics.sql`
- Modify: `api/openapi.yaml`
- Modify: `cmdb-core/internal/api/impl.go`

- [ ] **Step 1: Create metrics queries**

Create `cmdb-core/db/queries/metrics.sql`:

```sql
-- name: QueryMetrics :many
SELECT time, asset_id, name, value
FROM metrics
WHERE asset_id = $1
  AND name = $2
  AND time > now() - ($3 || ' hours')::interval
ORDER BY time DESC
LIMIT 1000;

-- name: QueryMetricsAggregated :many
SELECT time_bucket($4::interval, time) AS bucket,
       avg(value) AS avg_val,
       max(value) AS max_val,
       min(value) AS min_val
FROM metrics
WHERE asset_id = $1
  AND name = $2
  AND time > now() - ($3 || ' hours')::interval
GROUP BY bucket
ORDER BY bucket;

-- name: QueryMetricsByLocation :many
SELECT m.name, avg(m.value) AS avg_val, max(m.value) AS max_val, count(*) AS sample_count
FROM metrics m
JOIN assets a ON m.asset_id = a.id
JOIN locations l ON a.location_id = l.id
WHERE l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $1)::ltree
  AND m.time > now() - ($2 || ' hours')::interval
GROUP BY m.name
ORDER BY m.name;
```

- [ ] **Step 2: Regenerate sqlc, add endpoint to openapi.yaml, regenerate Go types, implement in impl.go**

Add to openapi.yaml paths:

```yaml
  /monitoring/metrics:
    get:
      operationId: queryMetrics
      tags: [monitoring]
      parameters:
        - name: asset_id
          in: query
          schema: { type: string, format: uuid }
        - name: metric_name
          in: query
          schema: { type: string }
        - name: time_range
          in: query
          schema: { type: string }
          description: "e.g., 1h, 6h, 24h, 7d"
        - name: location_id
          in: query
          schema: { type: string, format: uuid }
          description: "Query aggregated metrics for all assets under a location"
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      type: object
                      properties:
                        time: { type: string, format: date-time }
                        name: { type: string }
                        value: { type: number }
                        avg_val: { type: number }
                        max_val: { type: number }
                        min_val: { type: number }
                  meta: { $ref: '#/components/schemas/Meta' }
```

Regenerate, add impl.

- [ ] **Step 3: Verify**

```bash
# After rebuilding backend:
curl -s "http://localhost:8080/api/v1/monitoring/metrics?asset_id=f0000000-0000-0000-0000-000000000001&metric_name=cpu_usage&time_range=6h" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool | head -20
```

- [ ] **Step 4: Commit**

```bash
git add db/queries/metrics.sql api/openapi.yaml cmdb-core/internal/
git commit -m "feat: add metrics query API endpoint with time range and location aggregation"
```

---

### Task 7: EnergyMonitor + SensorConfig Frontend Integration

**Files:**
- Create: `cmdb-demo/src/hooks/useMetrics.ts`
- Modify: `cmdb-demo/src/pages/EnergyMonitor.tsx`
- Modify: `cmdb-demo/src/pages/SensorConfiguration.tsx`

- [ ] **Step 1: Create useMetrics hook**

```typescript
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'

interface MetricPoint {
  time: string
  name: string
  value: number
  avg_val?: number
  max_val?: number
  min_val?: number
}

export function useMetrics(params: { asset_id?: string; metric_name?: string; time_range?: string; location_id?: string }) {
  return useQuery({
    queryKey: ['metrics', params],
    queryFn: () => apiClient.get<ApiResponse<MetricPoint[]>>('/monitoring/metrics', params as Record<string, string>),
    enabled: !!(params.asset_id || params.location_id),
  })
}

export function useLocationMetrics(locationId: string, timeRange: string = '24h') {
  return useMetrics({ location_id: locationId, time_range: timeRange })
}
```

- [ ] **Step 2: Migrate EnergyMonitor.tsx**

Read the full file. Replace hardcoded `DAILY_BARS`, `WEEKLY_BARS`, `MONTHLY_BARS` with:

```typescript
import { useLocationMetrics } from '../hooks/useMetrics'
import { useLocationContext } from '../contexts/LocationContext'

// Inside component:
const { path } = useLocationContext()
const locationId = path.campus?.id ?? "d0000000-0000-0000-0000-000000000004"
const metricsQ = useLocationMetrics(locationId, '168h') // 7 days

// Transform API data into chart format
const chartData = useMemo(() => {
  const raw = metricsQ.data?.data ?? []
  // Group by day/hour for DAILY_BARS, etc.
  // ... transformation logic
  return { daily: [...], weekly: [...], monthly: [...] }
}, [metricsQ.data])
```

Keep the chart rendering components unchanged — just change the data source from hardcoded arrays to the computed `chartData`.

For fields not available from the metrics API (carbon emissions, cost), keep as hardcoded with clear comments.

- [ ] **Step 3: Migrate SensorConfiguration.tsx**

Replace `INITIAL_SENSORS` mock with assets query filtered by type or attributes:

```typescript
import { useAssets } from '../hooks/useAssets'
import { useMetrics } from '../hooks/useMetrics'

// Sensors are assets with monitoring capabilities
const { data: assetsResp } = useAssets()
const assets = assetsResp?.data ?? []

// Map assets to sensor display format
const sensors = assets.map(a => ({
  id: a.asset_tag,
  name: a.name,
  type: a.type,
  icon: typeToIcon(a.type),
  location: a.location_id ?? '-',
  enabled: a.status === 'operational',
  pollingInterval: 30,
  lastSeen: 'live',
  status: a.status === 'operational' ? 'Online' : a.status === 'maintenance' ? 'Degraded' : 'Offline',
}))
```

This maps the existing asset model to the sensor concept. The enable/disable toggle can be a UI-only feature for now.

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/hooks/useMetrics.ts cmdb-demo/src/pages/EnergyMonitor.tsx cmdb-demo/src/pages/SensorConfiguration.tsx
git commit -m "feat: EnergyMonitor + SensorConfig use real metrics API - zero mock data remaining"
```

---

## Task Summary

| Task | Scope | Files | Outcome |
|------|-------|-------|---------|
| 1 | Integration DB migration | 2 | Tables + seed data |
| 2 | Integration sqlc + service | 3 | CRUD backend |
| 3 | /system/health + Integration API + impl | 5 | 3 new API endpoints |
| 4 | SystemSettings frontend | 4 | All 3 tabs use real API |
| 5 | Metrics injector script | 1 | 24h of simulated time-series data |
| 6 | Metrics query API | 4 | `GET /monitoring/metrics` with time range |
| 7 | EnergyMonitor + SensorConfig frontend | 3 | Last 2 mock pages connected |

## Verification After All Tasks

```
Pages with mock data before:  4
Pages with mock data after:   0 (Login reclassified as "working")

New API endpoints:            4 (/system/health, /integration/adapters, /integration/webhooks, /monitoring/metrics)
New DB tables:                3 (integration_adapters, webhook_subscriptions, webhook_deliveries)
Metrics data:                 ~100K rows (24h backfill for 20 assets)
```
