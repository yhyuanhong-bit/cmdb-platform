# CMDB Core Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go backend core with database schema, authentication, 9 business modules exposing 28 P0 REST endpoints, and NATS event bus — enough to replace all frontend mock data with real API calls.

**Architecture:** Modular monolith in Go using Gin HTTP framework, sqlc for type-safe DB access, Wire for dependency injection, NATS JetStream for async events. Single PostgreSQL 17 + TimescaleDB instance. Redis for cache/sessions. All modules share one process but communicate through interfaces and events, not direct calls.

**Tech Stack:** Go 1.23+, Gin, sqlc, golang-migrate, Wire, zap, NATS, PostgreSQL 17 + TimescaleDB + ltree, Redis 7, Docker Compose, casbin

**Spec Reference:** `docs/superpowers/specs/2026-04-03-cmdb-backend-techstack-design.md`

**This plan covers:** Spec Section 2 (Tech Stack), Section 3 (Data Model), Section 5 (Module Structure), and Phase 1 of Section 9.

**Out of scope (later plans):** Python ingestion engine, MCP Server, AI adapters, cross-IDC federation, WebSocket push, Excel import.

---

## File Structure

```
cmdb-core/
├── cmd/server/main.go                          # Entry point: starts HTTP + NATS
├── internal/
│   ├── config/config.go                        # ENV + YAML config struct
│   ├── middleware/
│   │   ├── auth.go                             # JWT verification middleware
│   │   ├── tenant.go                           # RLS tenant context setter
│   │   ├── requestid.go                        # X-Request-Id injection
│   │   ├── recovery.go                         # Panic recovery + error response
│   │   └── cors.go                             # CORS configuration
│   ├── domain/
│   │   ├── identity/
│   │   │   ├── model.go                        # User, Role, Tenant, Department structs
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # sqlc-backed implementation
│   │   │   ├── auth_service.go                 # Login, JWT issue, refresh
│   │   │   ├── service.go                      # User/role CRUD
│   │   │   └── handler.go                      # HTTP handlers
│   │   ├── topology/
│   │   │   ├── model.go                        # Location, Rack, RackSlot
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # ltree queries
│   │   │   ├── service.go                      # Hierarchy traversal + stats
│   │   │   └── handler.go                      # HTTP handlers
│   │   ├── asset/
│   │   │   ├── model.go                        # Asset struct + status enum
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # sqlc impl with flexible filters
│   │   │   ├── service.go                      # Business logic
│   │   │   ├── handler.go                      # HTTP handlers
│   │   │   └── events.go                       # Domain event definitions
│   │   ├── maintenance/
│   │   │   ├── model.go                        # WorkOrder, WorkOrderLog
│   │   │   ├── statemachine.go                 # Status transition rules
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # sqlc impl
│   │   │   ├── service.go                      # Transition logic + log recording
│   │   │   └── handler.go                      # HTTP handlers
│   │   ├── monitoring/
│   │   │   ├── model.go                        # AlertRule, AlertEvent, Incident
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # sqlc impl
│   │   │   ├── service.go                      # Alert lifecycle
│   │   │   └── handler.go                      # HTTP handlers
│   │   ├── inventory/
│   │   │   ├── model.go                        # InventoryTask, InventoryItem
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # sqlc impl
│   │   │   ├── service.go                      # Task lifecycle
│   │   │   └── handler.go                      # HTTP handlers
│   │   ├── audit/
│   │   │   ├── model.go                        # AuditEvent
│   │   │   ├── repository.go                   # Interface
│   │   │   ├── repository_pg.go                # Append-only impl
│   │   │   ├── service.go                      # Query service
│   │   │   ├── handler.go                      # HTTP handlers
│   │   │   └── collector.go                    # Async batch writer from event bus
│   │   └── dashboard/
│   │       ├── service.go                      # Cross-module aggregation
│   │       └── handler.go                      # GET /dashboard/stats
│   ├── eventbus/
│   │   ├── bus.go                              # Publish/Subscribe interface
│   │   ├── nats.go                             # NATS JetStream implementation
│   │   └── subjects.go                         # Event subject constants
│   └── platform/
│       ├── database/
│       │   ├── postgres.go                     # PG pool + RLS helpers
│       │   └── migrate.go                      # golang-migrate runner
│       ├── cache/
│       │   └── redis.go                        # Redis client wrapper
│       └── response/
│           └── response.go                     # Unified API response helpers
├── db/
│   ├── migrations/
│   │   ├── 000001_init_extensions.up.sql
│   │   ├── 000001_init_extensions.down.sql
│   │   ├── 000002_tenants_and_identity.up.sql
│   │   ├── 000002_tenants_and_identity.down.sql
│   │   ├── 000003_locations.up.sql
│   │   ├── 000003_locations.down.sql
│   │   ├── 000004_assets_and_racks.up.sql
│   │   ├── 000004_assets_and_racks.down.sql
│   │   ├── 000005_maintenance.up.sql
│   │   ├── 000005_maintenance.down.sql
│   │   ├── 000006_monitoring.up.sql
│   │   ├── 000006_monitoring.down.sql
│   │   ├── 000007_inventory.up.sql
│   │   ├── 000007_inventory.down.sql
│   │   ├── 000008_audit.up.sql
│   │   ├── 000008_audit.down.sql
│   │   ├── 000009_timescaledb_metrics.up.sql
│   │   └── 000009_timescaledb_metrics.down.sql
│   └── queries/
│       ├── tenants.sql
│       ├── users.sql
│       ├── roles.sql
│       ├── locations.sql
│       ├── assets.sql
│       ├── racks.sql
│       ├── work_orders.sql
│       ├── alert_events.sql
│       ├── inventory_tasks.sql
│       └── audit_events.sql
├── db/seed/
│   └── seed.sql                                # Dev seed data
├── deploy/
│   ├── docker-compose.yml                      # PG + Redis + NATS + cmdb-core
│   └── .env.example
├── sqlc.yaml
├── wire.go                                     # Wire provider set
├── wire_gen.go                                 # Wire generated (auto)
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

---

## Task 1: Project Scaffold + Docker Compose + Makefile

**Files:**
- Create: `cmdb-core/go.mod`
- Create: `cmdb-core/Makefile`
- Create: `cmdb-core/deploy/docker-compose.yml`
- Create: `cmdb-core/deploy/.env.example`
- Create: `cmdb-core/cmd/server/main.go`
- Create: `cmdb-core/internal/config/config.go`
- Create: `cmdb-core/Dockerfile`

- [ ] **Step 1: Initialize Go module**

```bash
cd /cmdb-platform
mkdir -p cmdb-core/cmd/server cmdb-core/internal/config cmdb-core/deploy
cd cmdb-core
go mod init github.com/cmdb-platform/cmdb-core
```

- [ ] **Step 2: Create config struct**

Create `cmdb-core/internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	DatabaseURL string
	RedisURL    string
	NatsURL     string
	JWTSecret   string
	DeployMode  string // "central" or "edge"
	TenantID    string // required in edge mode
	LogLevel    string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	cfg := &Config{
		Port:        port,
		DatabaseURL: getEnv("DATABASE_URL", "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),
		NatsURL:     getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		DeployMode:  getEnv("DEPLOY_MODE", "central"),
		TenantID:    getEnv("TENANT_ID", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}
	if cfg.DeployMode == "edge" && cfg.TenantID == "" {
		return nil, fmt.Errorf("TENANT_ID is required in edge deploy mode")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Create minimal main.go**

Create `cmdb-core/cmd/server/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cmdb-platform/cmdb-core/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("cmdb-core starting on %s (mode=%s)", addr, cfg.DeployMode)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] **Step 4: Create Docker Compose**

Create `cmdb-core/deploy/.env.example`:

```env
DB_PASS=cmdb_secret
JWT_SECRET=change-me-in-production
DEPLOY_MODE=central
CORE_REPLICAS=1
```

Create `cmdb-core/deploy/docker-compose.yml`:

```yaml
services:
  postgres:
    image: timescale/timescaledb:latest-pg17
    environment:
      POSTGRES_DB: cmdb
      POSTGRES_USER: cmdb
      POSTGRES_PASSWORD: ${DB_PASS:-cmdb_secret}
    ports:
      - "5432:5432"
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U cmdb"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7.4-alpine
    command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 3

  nats:
    image: nats:2.10-alpine
    command: ["--jetstream", "--store_dir=/data"]
    ports:
      - "4222:4222"
      - "8222:8222"
    volumes:
      - nats_data:/data

volumes:
  pg_data:
  nats_data:
```

- [ ] **Step 5: Create Makefile**

Create `cmdb-core/Makefile`:

```makefile
.PHONY: dev infra migrate seed test build

# Start infrastructure (PG + Redis + NATS)
infra:
	cd deploy && docker compose up -d

# Stop infrastructure
infra-down:
	cd deploy && docker compose down

# Run migrations
migrate:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
		-database "$(DATABASE_URL)" -path db/migrations up

# Run migrations down
migrate-down:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
		-database "$(DATABASE_URL)" -path db/migrations down 1

# Generate sqlc code
sqlc:
	sqlc generate

# Generate Wire DI
wire:
	wire ./...

# Run dev server
dev:
	go run ./cmd/server

# Run tests
test:
	go test ./... -v -count=1

# Build binary
build:
	CGO_ENABLED=0 go build -o bin/cmdb-core ./cmd/server

# Seed dev data
seed:
	psql "$(DATABASE_URL)" -f db/seed/seed.sql
```

- [ ] **Step 6: Create Dockerfile**

Create `cmdb-core/Dockerfile`:

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cmdb-core ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /cmdb-core /usr/local/bin/cmdb-core
COPY db/migrations /migrations
EXPOSE 8080
CMD ["cmdb-core"]
```

- [ ] **Step 7: Install dependencies and verify build**

```bash
cd /cmdb-platform/cmdb-core
go mod tidy
go build ./cmd/server
```

Expected: Build succeeds with no errors.

- [ ] **Step 8: Commit**

```bash
git add cmdb-core/
git commit -m "feat: scaffold cmdb-core project with config, docker-compose, makefile"
```

---

## Task 2: Database Migrations (All 20+ Tables)

**Files:**
- Create: `cmdb-core/db/migrations/000001_init_extensions.up.sql`
- Create: `cmdb-core/db/migrations/000001_init_extensions.down.sql`
- Create: `cmdb-core/db/migrations/000002_tenants_and_identity.up.sql`
- Create: `cmdb-core/db/migrations/000002_tenants_and_identity.down.sql`
- Create: `cmdb-core/db/migrations/000003_locations.up.sql`
- Create: `cmdb-core/db/migrations/000003_locations.down.sql`
- Create: `cmdb-core/db/migrations/000004_assets_and_racks.up.sql`
- Create: `cmdb-core/db/migrations/000004_assets_and_racks.down.sql`
- Create: `cmdb-core/db/migrations/000005_maintenance.up.sql`
- Create: `cmdb-core/db/migrations/000005_maintenance.down.sql`
- Create: `cmdb-core/db/migrations/000006_monitoring.up.sql`
- Create: `cmdb-core/db/migrations/000006_monitoring.down.sql`
- Create: `cmdb-core/db/migrations/000007_inventory.up.sql`
- Create: `cmdb-core/db/migrations/000007_inventory.down.sql`
- Create: `cmdb-core/db/migrations/000008_audit.up.sql`
- Create: `cmdb-core/db/migrations/000008_audit.down.sql`
- Create: `cmdb-core/db/migrations/000009_timescaledb_metrics.up.sql`
- Create: `cmdb-core/db/migrations/000009_timescaledb_metrics.down.sql`

- [ ] **Step 1: Create migration 000001 — extensions**

Create `cmdb-core/db/migrations/000001_init_extensions.up.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;
```

Create `cmdb-core/db/migrations/000001_init_extensions.down.sql`:

```sql
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS "uuid-ossp";
DROP EXTENSION IF EXISTS ltree;
```

- [ ] **Step 2: Create migration 000002 — tenants and identity**

Create `cmdb-core/db/migrations/000002_tenants_and_identity.up.sql`:

```sql
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(50) UNIQUE NOT NULL,
    status      VARCHAR(20) DEFAULT 'active',
    settings    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE departments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(50) NOT NULL,
    permissions JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id),
    dept_id      UUID REFERENCES departments(id),
    username     VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    email        VARCHAR(200),
    phone        VARCHAR(30),
    password_hash VARCHAR(200) NOT NULL,
    status       VARCHAR(20) DEFAULT 'active',
    source       VARCHAR(20) DEFAULT 'local',
    created_at   TIMESTAMPTZ DEFAULT now(),
    updated_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID REFERENCES tenants(id),
    name        VARCHAR(50) NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '{}',
    is_system   BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_users_tenant ON users (tenant_id);
CREATE INDEX idx_departments_tenant ON departments (tenant_id);
```

Create `cmdb-core/db/migrations/000002_tenants_and_identity.down.sql`:

```sql
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS tenants;
```

- [ ] **Step 3: Create migration 000003 — locations**

Create `cmdb-core/db/migrations/000003_locations.up.sql`:

```sql
CREATE TABLE locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    name_en     VARCHAR(100),
    slug        VARCHAR(50) NOT NULL,
    level       VARCHAR(20) NOT NULL,
    parent_id   UUID REFERENCES locations(id),
    path        LTREE NOT NULL,
    status      VARCHAR(20) DEFAULT 'active',
    metadata    JSONB DEFAULT '{}',
    sort_order  INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_locations_path ON locations USING GIST (path);
CREATE INDEX idx_locations_parent ON locations (parent_id);
CREATE INDEX idx_locations_tenant ON locations (tenant_id);
CREATE INDEX idx_locations_slug ON locations (tenant_id, slug);
```

Create `cmdb-core/db/migrations/000003_locations.down.sql`:

```sql
DROP TABLE IF EXISTS locations;
```

- [ ] **Step 4: Create migration 000004 — assets and racks**

Create `cmdb-core/db/migrations/000004_assets_and_racks.up.sql`:

```sql
CREATE TABLE racks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    location_id       UUID NOT NULL REFERENCES locations(id),
    name              VARCHAR(50) NOT NULL,
    row_label         VARCHAR(10),
    total_u           INT NOT NULL DEFAULT 42,
    power_capacity_kw NUMERIC(8,2),
    status            VARCHAR(20) DEFAULT 'active',
    tags              TEXT[] DEFAULT '{}',
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_racks_tenant ON racks (tenant_id);
CREATE INDEX idx_racks_location ON racks (location_id);

CREATE TABLE assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    asset_tag       VARCHAR(50) UNIQUE NOT NULL,
    property_number VARCHAR(50),
    control_number  VARCHAR(50),
    name            VARCHAR(200) NOT NULL,
    type            VARCHAR(50) NOT NULL,
    sub_type        VARCHAR(50),
    status          VARCHAR(30) NOT NULL DEFAULT 'inventoried',
    bia_level       VARCHAR(20) DEFAULT 'normal',
    location_id     UUID REFERENCES locations(id),
    rack_id         UUID REFERENCES racks(id),
    vendor          VARCHAR(100),
    model           VARCHAR(100),
    serial_number   VARCHAR(100),
    attributes      JSONB DEFAULT '{}',
    tags            TEXT[] DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_assets_tenant ON assets (tenant_id);
CREATE INDEX idx_assets_location ON assets (location_id);
CREATE INDEX idx_assets_rack ON assets (rack_id);
CREATE INDEX idx_assets_serial ON assets (serial_number);
CREATE INDEX idx_assets_type ON assets (tenant_id, type, sub_type);
CREATE INDEX idx_assets_status ON assets (tenant_id, status);
CREATE INDEX idx_assets_tags ON assets USING GIN (tags);
CREATE INDEX idx_assets_attrs ON assets USING GIN (attributes);

CREATE TABLE rack_slots (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rack_id  UUID NOT NULL REFERENCES racks(id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES assets(id),
    start_u  INT NOT NULL,
    end_u    INT NOT NULL,
    side     VARCHAR(5) DEFAULT 'front',
    UNIQUE (rack_id, start_u, side),
    CHECK (end_u >= start_u)
);
```

Create `cmdb-core/db/migrations/000004_assets_and_racks.down.sql`:

```sql
DROP TABLE IF EXISTS rack_slots;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS racks;
```

- [ ] **Step 5: Create migration 000005 — maintenance**

Create `cmdb-core/db/migrations/000005_maintenance.up.sql`:

```sql
CREATE TABLE work_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(30) UNIQUE NOT NULL,
    title           VARCHAR(200) NOT NULL,
    type            VARCHAR(30) NOT NULL,
    status          VARCHAR(30) NOT NULL DEFAULT 'draft',
    priority        VARCHAR(20) DEFAULT 'medium',
    location_id     UUID REFERENCES locations(id),
    asset_id        UUID REFERENCES assets(id),
    requestor_id    UUID REFERENCES users(id),
    assignee_id     UUID REFERENCES users(id),
    description     TEXT,
    reason          TEXT,
    prediction_id   UUID,
    scheduled_start TIMESTAMPTZ,
    scheduled_end   TIMESTAMPTZ,
    actual_start    TIMESTAMPTZ,
    actual_end      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_work_orders_tenant ON work_orders (tenant_id);
CREATE INDEX idx_work_orders_status ON work_orders (tenant_id, status);
CREATE INDEX idx_work_orders_asset ON work_orders (asset_id);

CREATE TABLE work_order_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    action      VARCHAR(30) NOT NULL,
    from_status VARCHAR(30),
    to_status   VARCHAR(30),
    operator_id UUID REFERENCES users(id),
    comment     TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);
```

Create `cmdb-core/db/migrations/000005_maintenance.down.sql`:

```sql
DROP TABLE IF EXISTS work_order_logs;
DROP TABLE IF EXISTS work_orders;
```

- [ ] **Step 6: Create migration 000006 — monitoring**

Create `cmdb-core/db/migrations/000006_monitoring.up.sql`:

```sql
CREATE TABLE alert_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    condition   JSONB NOT NULL,
    severity    VARCHAR(20) NOT NULL,
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE alert_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    rule_id       UUID REFERENCES alert_rules(id),
    asset_id      UUID REFERENCES assets(id),
    status        VARCHAR(20) NOT NULL DEFAULT 'firing',
    severity      VARCHAR(20) NOT NULL,
    message       TEXT NOT NULL,
    trigger_value NUMERIC(12,4),
    fired_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    acked_at      TIMESTAMPTZ,
    resolved_at   TIMESTAMPTZ
);

CREATE INDEX idx_alerts_tenant_status ON alert_events (tenant_id, status);
CREATE INDEX idx_alerts_asset ON alert_events (asset_id);
CREATE INDEX idx_alerts_fired ON alert_events (tenant_id, fired_at DESC);

CREATE TABLE incidents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    title       VARCHAR(200) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    severity    VARCHAR(20) NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);
```

Create `cmdb-core/db/migrations/000006_monitoring.down.sql`:

```sql
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS alert_events;
DROP TABLE IF EXISTS alert_rules;
```

- [ ] **Step 7: Create migration 000007 — inventory**

Create `cmdb-core/db/migrations/000007_inventory.up.sql`:

```sql
CREATE TABLE inventory_tasks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    code              VARCHAR(30) UNIQUE NOT NULL,
    name              VARCHAR(200) NOT NULL,
    scope_location_id UUID NOT NULL REFERENCES locations(id),
    status            VARCHAR(20) DEFAULT 'planned',
    method            VARCHAR(20) NOT NULL,
    planned_date      DATE NOT NULL,
    completed_date    DATE,
    assigned_to       UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_inventory_tasks_tenant ON inventory_tasks (tenant_id);

CREATE TABLE inventory_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES inventory_tasks(id) ON DELETE CASCADE,
    asset_id    UUID REFERENCES assets(id),
    rack_id     UUID REFERENCES racks(id),
    expected    JSONB NOT NULL,
    actual      JSONB,
    status      VARCHAR(20) DEFAULT 'pending',
    scanned_at  TIMESTAMPTZ,
    scanned_by  UUID REFERENCES users(id)
);
```

Create `cmdb-core/db/migrations/000007_inventory.down.sql`:

```sql
DROP TABLE IF EXISTS inventory_items;
DROP TABLE IF EXISTS inventory_tasks;
```

- [ ] **Step 8: Create migration 000008 — audit**

Create `cmdb-core/db/migrations/000008_audit.up.sql`:

```sql
CREATE TABLE audit_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    action      VARCHAR(50) NOT NULL,
    module      VARCHAR(30) NOT NULL,
    target_type VARCHAR(30) NOT NULL,
    target_id   UUID NOT NULL,
    operator_id UUID NOT NULL REFERENCES users(id),
    diff        JSONB DEFAULT '{}',
    source      VARCHAR(20) DEFAULT 'web',
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_audit_tenant ON audit_events (tenant_id, created_at DESC);
CREATE INDEX idx_audit_target ON audit_events (target_type, target_id);
CREATE INDEX idx_audit_operator ON audit_events (operator_id);
```

Create `cmdb-core/db/migrations/000008_audit.down.sql`:

```sql
DROP TABLE IF EXISTS audit_events;
```

- [ ] **Step 9: Create migration 000009 — TimescaleDB metrics**

Create `cmdb-core/db/migrations/000009_timescaledb_metrics.up.sql`:

```sql
CREATE TABLE metrics (
    time      TIMESTAMPTZ NOT NULL,
    asset_id  UUID NOT NULL,
    tenant_id UUID NOT NULL,
    name      VARCHAR(100) NOT NULL,
    value     DOUBLE PRECISION NOT NULL,
    labels    JSONB DEFAULT '{}'
);

SELECT create_hypertable('metrics', 'time');

CREATE INDEX idx_metrics_asset_name ON metrics (asset_id, name, time DESC);
CREATE INDEX idx_metrics_tenant ON metrics (tenant_id, time DESC);

SELECT add_retention_policy('metrics', INTERVAL '30 days');

CREATE MATERIALIZED VIEW metrics_5min
WITH (timescaledb.continuous) AS
SELECT time_bucket('5 minutes', time) AS bucket,
       asset_id, tenant_id, name,
       avg(value) AS avg_val,
       max(value) AS max_val,
       min(value) AS min_val
FROM metrics
GROUP BY bucket, asset_id, tenant_id, name
WITH NO DATA;

SELECT add_continuous_aggregate_policy('metrics_5min',
    start_offset => INTERVAL '1 hour',
    end_offset   => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');

SELECT add_retention_policy('metrics_5min', INTERVAL '180 days');

CREATE MATERIALIZED VIEW metrics_1hour
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 hour', time) AS bucket,
       asset_id, tenant_id, name,
       avg(value) AS avg_val,
       max(value) AS max_val,
       min(value) AS min_val
FROM metrics
GROUP BY bucket, asset_id, tenant_id, name
WITH NO DATA;

SELECT add_continuous_aggregate_policy('metrics_1hour',
    start_offset => INTERVAL '3 hours',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

SELECT add_retention_policy('metrics_1hour', INTERVAL '730 days');
```

Create `cmdb-core/db/migrations/000009_timescaledb_metrics.down.sql`:

```sql
DROP MATERIALIZED VIEW IF EXISTS metrics_1hour;
DROP MATERIALIZED VIEW IF EXISTS metrics_5min;
DROP TABLE IF EXISTS metrics;
```

- [ ] **Step 10: Start infra and run migrations**

```bash
cd /cmdb-platform/cmdb-core
make infra
sleep 5
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" make migrate
```

Expected: All 9 migrations apply successfully.

- [ ] **Step 11: Verify tables exist**

```bash
psql "postgres://cmdb:cmdb_secret@localhost:5432/cmdb" -c "\dt"
```

Expected: All tables listed (tenants, departments, users, roles, user_roles, locations, racks, assets, rack_slots, work_orders, work_order_logs, alert_rules, alert_events, incidents, inventory_tasks, inventory_items, audit_events, metrics).

- [ ] **Step 12: Commit**

```bash
git add cmdb-core/db/
git commit -m "feat: add database migrations for all 20 tables including TimescaleDB"
```

---

## Task 3: sqlc Queries + Generated Code

**Files:**
- Create: `cmdb-core/sqlc.yaml`
- Create: `cmdb-core/db/queries/tenants.sql`
- Create: `cmdb-core/db/queries/users.sql`
- Create: `cmdb-core/db/queries/roles.sql`
- Create: `cmdb-core/db/queries/locations.sql`
- Create: `cmdb-core/db/queries/assets.sql`
- Create: `cmdb-core/db/queries/racks.sql`
- Create: `cmdb-core/db/queries/work_orders.sql`
- Create: `cmdb-core/db/queries/alert_events.sql`
- Create: `cmdb-core/db/queries/inventory_tasks.sql`
- Create: `cmdb-core/db/queries/audit_events.sql`

- [ ] **Step 1: Create sqlc config**

Create `cmdb-core/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries"
    schema: "db/migrations"
    gen:
      go:
        package: "dbgen"
        out: "internal/dbgen"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "ltree"
            go_type: "string"
          - db_type: "jsonb"
            go_type:
              import: "encoding/json"
              type: "RawMessage"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "text[]"
            go_type: "[]string"
```

- [ ] **Step 2: Create tenants queries**

Create `cmdb-core/db/queries/tenants.sql`:

```sql
-- name: GetTenant :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: ListTenants :many
SELECT * FROM tenants ORDER BY name;
```

- [ ] **Step 3: Create users queries**

Create `cmdb-core/db/queries/users.sql`:

```sql
-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: ListUsers :many
SELECT * FROM users
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUsers :one
SELECT count(*) FROM users WHERE tenant_id = $1;

-- name: CreateUser :one
INSERT INTO users (tenant_id, dept_id, username, display_name, email, phone, password_hash, status, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
    display_name = COALESCE(sqlc.narg('display_name'), display_name),
    email = COALESCE(sqlc.narg('email'), email),
    phone = COALESCE(sqlc.narg('phone'), phone),
    dept_id = COALESCE(sqlc.narg('dept_id'), dept_id),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE id = $1
RETURNING *;
```

- [ ] **Step 4: Create roles queries**

Create `cmdb-core/db/queries/roles.sql`:

```sql
-- name: ListRoles :many
SELECT * FROM roles
WHERE tenant_id = $1 OR tenant_id IS NULL
ORDER BY is_system DESC, name;

-- name: CreateRole :one
INSERT INTO roles (tenant_id, name, description, permissions, is_system)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1 AND is_system = false;

-- name: ListUserRoles :many
SELECT r.* FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1;

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;
```

- [ ] **Step 5: Create locations queries**

Create `cmdb-core/db/queries/locations.sql`:

```sql
-- name: ListRootLocations :many
SELECT * FROM locations
WHERE tenant_id = $1 AND parent_id IS NULL
ORDER BY sort_order, name;

-- name: GetLocation :one
SELECT * FROM locations WHERE id = $1;

-- name: ListChildren :many
SELECT * FROM locations
WHERE parent_id = $1
ORDER BY sort_order, name;

-- name: ListDescendants :many
SELECT * FROM locations
WHERE tenant_id = $1 AND path <@ $2::ltree
ORDER BY path;

-- name: ListAncestors :many
SELECT * FROM locations
WHERE tenant_id = $1 AND path @> $2::ltree
ORDER BY path;

-- name: CreateLocation :one
INSERT INTO locations (tenant_id, name, name_en, slug, level, parent_id, path, status, metadata, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7::ltree, $8, $9, $10)
RETURNING *;

-- name: UpdateLocation :one
UPDATE locations SET
    name = COALESCE(sqlc.narg('name'), name),
    name_en = COALESCE(sqlc.narg('name_en'), name_en),
    status = COALESCE(sqlc.narg('status'), status),
    metadata = COALESCE(sqlc.narg('metadata'), metadata),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteLocation :exec
DELETE FROM locations WHERE id = $1;
```

- [ ] **Step 6: Create assets queries**

Create `cmdb-core/db/queries/assets.sql`:

```sql
-- name: ListAssets :many
SELECT * FROM assets
WHERE tenant_id = $1
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('location_id')::uuid IS NULL OR location_id = sqlc.narg('location_id'))
  AND (sqlc.narg('rack_id')::uuid IS NULL OR rack_id = sqlc.narg('rack_id'))
  AND (sqlc.narg('serial_number')::varchar IS NULL OR serial_number = sqlc.narg('serial_number'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAssets :one
SELECT count(*) FROM assets
WHERE tenant_id = $1
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('location_id')::uuid IS NULL OR location_id = sqlc.narg('location_id'));

-- name: GetAsset :one
SELECT * FROM assets WHERE id = $1;

-- name: GetAssetByTag :one
SELECT * FROM assets WHERE asset_tag = $1;

-- name: CreateAsset :one
INSERT INTO assets (tenant_id, asset_tag, property_number, control_number, name, type, sub_type, status, bia_level, location_id, rack_id, vendor, model, serial_number, attributes, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: UpdateAsset :one
UPDATE assets SET
    name = COALESCE(sqlc.narg('name'), name),
    type = COALESCE(sqlc.narg('type'), type),
    sub_type = COALESCE(sqlc.narg('sub_type'), sub_type),
    status = COALESCE(sqlc.narg('status'), status),
    bia_level = COALESCE(sqlc.narg('bia_level'), bia_level),
    location_id = COALESCE(sqlc.narg('location_id'), location_id),
    rack_id = COALESCE(sqlc.narg('rack_id'), rack_id),
    vendor = COALESCE(sqlc.narg('vendor'), vendor),
    model = COALESCE(sqlc.narg('model'), model),
    serial_number = COALESCE(sqlc.narg('serial_number'), serial_number),
    attributes = COALESCE(sqlc.narg('attributes'), attributes),
    tags = COALESCE(sqlc.narg('tags'), tags),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = $1;
```

- [ ] **Step 7: Create racks queries**

Create `cmdb-core/db/queries/racks.sql`:

```sql
-- name: GetRack :one
SELECT * FROM racks WHERE id = $1;

-- name: ListRacksByLocation :many
SELECT * FROM racks
WHERE location_id = $1
ORDER BY name;

-- name: CreateRack :one
INSERT INTO racks (tenant_id, location_id, name, row_label, total_u, power_capacity_kw, status, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateRack :one
UPDATE racks SET
    name = COALESCE(sqlc.narg('name'), name),
    row_label = COALESCE(sqlc.narg('row_label'), row_label),
    total_u = COALESCE(sqlc.narg('total_u'), total_u),
    power_capacity_kw = COALESCE(sqlc.narg('power_capacity_kw'), power_capacity_kw),
    status = COALESCE(sqlc.narg('status'), status),
    tags = COALESCE(sqlc.narg('tags'), tags)
WHERE id = $1
RETURNING *;

-- name: DeleteRack :exec
DELETE FROM racks WHERE id = $1;

-- name: ListAssetsByRack :many
SELECT a.* FROM assets a
WHERE a.rack_id = $1
ORDER BY a.name;

-- name: GetRackOccupancy :one
SELECT r.total_u,
       COALESCE(SUM(rs.end_u - rs.start_u + 1), 0)::int AS used_u
FROM racks r
LEFT JOIN rack_slots rs ON rs.rack_id = r.id
WHERE r.id = $1
GROUP BY r.id;
```

- [ ] **Step 8: Create work_orders queries**

Create `cmdb-core/db/queries/work_orders.sql`:

```sql
-- name: ListWorkOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWorkOrders :one
SELECT count(*) FROM work_orders
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'));

-- name: GetWorkOrder :one
SELECT * FROM work_orders WHERE id = $1;

-- name: CreateWorkOrder :one
INSERT INTO work_orders (tenant_id, code, title, type, status, priority, location_id, asset_id, requestor_id, assignee_id, description, reason, prediction_id, scheduled_start, scheduled_end)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: UpdateWorkOrderStatus :one
UPDATE work_orders SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateWorkOrderLog :one
INSERT INTO work_order_logs (order_id, action, from_status, to_status, operator_id, comment)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListWorkOrderLogs :many
SELECT * FROM work_order_logs
WHERE order_id = $1
ORDER BY created_at;
```

- [ ] **Step 9: Create alert_events queries**

Create `cmdb-core/db/queries/alert_events.sql`:

```sql
-- name: ListAlerts :many
SELECT * FROM alert_events
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'))
ORDER BY fired_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAlerts :one
SELECT count(*) FROM alert_events
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'));

-- name: AcknowledgeAlert :one
UPDATE alert_events SET status = 'acknowledged', acked_at = now()
WHERE id = $1 AND status = 'firing'
RETURNING *;

-- name: ResolveAlert :one
UPDATE alert_events SET status = 'resolved', resolved_at = now()
WHERE id = $1 AND status IN ('firing', 'acknowledged')
RETURNING *;
```

- [ ] **Step 10: Create inventory_tasks queries**

Create `cmdb-core/db/queries/inventory_tasks.sql`:

```sql
-- name: ListInventoryTasks :many
SELECT * FROM inventory_tasks
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountInventoryTasks :one
SELECT count(*) FROM inventory_tasks WHERE tenant_id = $1;

-- name: GetInventoryTask :one
SELECT * FROM inventory_tasks WHERE id = $1;

-- name: ListInventoryItems :many
SELECT * FROM inventory_items
WHERE task_id = $1
ORDER BY status, scanned_at;
```

- [ ] **Step 11: Create audit_events queries**

Create `cmdb-core/db/queries/audit_events.sql`:

```sql
-- name: QueryAuditEvents :many
SELECT * FROM audit_events
WHERE tenant_id = $1
  AND (sqlc.narg('module')::varchar IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('target_type')::varchar IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('target_id')::uuid IS NULL OR target_id = sqlc.narg('target_id'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAuditEvents :one
SELECT count(*) FROM audit_events
WHERE tenant_id = $1
  AND (sqlc.narg('module')::varchar IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('target_type')::varchar IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('target_id')::uuid IS NULL OR target_id = sqlc.narg('target_id'));

-- name: CreateAuditEvent :one
INSERT INTO audit_events (tenant_id, action, module, target_type, target_id, operator_id, diff, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;
```

- [ ] **Step 12: Install sqlc and generate code**

```bash
cd /cmdb-platform/cmdb-core
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go get github.com/google/uuid
go get github.com/jackc/pgx/v5
sqlc generate
```

Expected: `internal/dbgen/` directory created with generated Go files, no errors.

- [ ] **Step 13: Verify generated code compiles**

```bash
cd /cmdb-platform/cmdb-core
go mod tidy
go build ./internal/dbgen/...
```

Expected: Build succeeds.

- [ ] **Step 14: Commit**

```bash
git add cmdb-core/sqlc.yaml cmdb-core/db/queries/ cmdb-core/internal/dbgen/
git commit -m "feat: add sqlc queries and generate type-safe DB access code"
```

---

## Task 4: Platform Layer (Database, Redis, Response Helpers)

**Files:**
- Create: `cmdb-core/internal/platform/database/postgres.go`
- Create: `cmdb-core/internal/platform/cache/redis.go`
- Create: `cmdb-core/internal/platform/response/response.go`

- [ ] **Step 1: Create PostgreSQL connection + RLS helper**

Create `cmdb-core/internal/platform/database/postgres.go`:

```go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 50
	cfg.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// SetTenantContext sets the RLS tenant context for the current transaction.
// Must be called within a transaction (pgx.Tx).
func SetTenantContext(ctx context.Context, tx interface{ Exec(ctx context.Context, sql string, args ...any) (any, error) }, tenantID string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantID))
	return err
}
```

- [ ] **Step 2: Create Redis wrapper**

Create `cmdb-core/internal/platform/cache/redis.go`:

```go
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

// Common cache operations

func Set(ctx context.Context, client *redis.Client, key string, value any, ttl time.Duration) error {
	return client.Set(ctx, key, value, ttl).Err()
}

func Get(ctx context.Context, client *redis.Client, key string) (string, error) {
	return client.Get(ctx, key).Result()
}

func Del(ctx context.Context, client *redis.Client, keys ...string) error {
	return client.Del(ctx, keys...).Err()
}
```

- [ ] **Step 3: Create unified API response helpers**

Create `cmdb-core/internal/platform/response/response.go`:

```go
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Meta struct {
	RequestID string `json:"request_id"`
}

type SingleResponse struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type ListResponse struct {
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
	Meta       Meta       `json:"meta"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

func requestID(c *gin.Context) string {
	if id := c.GetString("request_id"); id != "" {
		return id
	}
	return uuid.New().String()
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, SingleResponse{
		Data: data,
		Meta: Meta{RequestID: requestID(c)},
	})
}

func OKList(c *gin.Context, data any, page, pageSize int, total int64) {
	totalPages := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		totalPages++
	}
	c.JSON(http.StatusOK, ListResponse{
		Data: data,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
		Meta: Meta{RequestID: requestID(c)},
	})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, SingleResponse{
		Data: data,
		Meta: Meta{RequestID: requestID(c)},
	})
}

func Err(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message},
		Meta:  Meta{RequestID: requestID(c)},
	})
}

func BadRequest(c *gin.Context, message string) {
	Err(c, http.StatusBadRequest, "BAD_REQUEST", message)
}

func NotFound(c *gin.Context, message string) {
	Err(c, http.StatusNotFound, "NOT_FOUND", message)
}

func Unauthorized(c *gin.Context, message string) {
	Err(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(c *gin.Context, message string) {
	Err(c, http.StatusForbidden, "FORBIDDEN", message)
}

func InternalError(c *gin.Context, message string) {
	Err(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// ParsePagination extracts page and page_size from query params with defaults.
func ParsePagination(c *gin.Context) (page, pageSize, offset int) {
	page = 1
	pageSize = 20
	if p := c.Query("page"); p != "" {
		if v := atoi(p); v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v := atoi(ps); v > 0 && v <= 100 {
			pageSize = v
		}
	}
	offset = (page - 1) * pageSize
	return
}

func atoi(s string) int {
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		v = v*10 + int(c-'0')
	}
	return v
}
```

- [ ] **Step 4: Install Gin and Redis deps**

```bash
cd /cmdb-platform/cmdb-core
go get github.com/gin-gonic/gin
go get github.com/redis/go-redis/v9
go mod tidy
go build ./internal/platform/...
```

Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/platform/
git commit -m "feat: add platform layer - database pool, redis client, response helpers"
```

---

## Task 5: Middleware (Auth JWT, Tenant RLS, RequestID, Recovery, CORS)

**Files:**
- Create: `cmdb-core/internal/middleware/auth.go`
- Create: `cmdb-core/internal/middleware/tenant.go`
- Create: `cmdb-core/internal/middleware/requestid.go`
- Create: `cmdb-core/internal/middleware/recovery.go`
- Create: `cmdb-core/internal/middleware/cors.go`

- [ ] **Step 1: Create JWT auth middleware**

Create `cmdb-core/internal/middleware/auth.go`:

```go
package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

type JWTClaims struct {
	UserID    string `json:"sub"`
	Username  string `json:"username"`
	TenantID  string `json:"tenant_id"`
	DeptID    string `json:"dept_id,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c, "missing or invalid authorization header")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := validateJWT(token, secret)
		if err != nil {
			response.Err(c, 401, "INVALID_TOKEN", err.Error())
			c.Abort()
			return
		}

		if claims.ExpiresAt < time.Now().Unix() {
			response.Err(c, 401, "INVALID_TOKEN", "token expired")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("tenant_id", claims.TenantID)
		c.Set("dept_id", claims.DeptID)
		c.Next()
	}
}

func validateJWT(token, secret string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode claims
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	return &claims, nil
}

// GenerateJWT creates a signed JWT token. Used by auth_service.
func GenerateJWT(claims JWTClaims, secret string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header + "." + payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return header + "." + payload + "." + signature, nil
}
```

Note: add `"fmt"` to the import block (it's used by validateJWT error messages).

- [ ] **Step 2: Create tenant RLS middleware**

Create `cmdb-core/internal/middleware/tenant.go`:

```go
package middleware

import (
	"github.com/gin-gonic/gin"
)

// TenantContext extracts tenant_id from JWT claims and sets it in the gin context.
// The actual RLS SET LOCAL happens at the repository/transaction level, not here.
// This middleware just ensures tenant_id is available for all handlers.
func TenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			// Public endpoints (login, healthz) won't have tenant_id
			c.Next()
			return
		}
		// tenant_id is already set by Auth middleware
		c.Next()
	}
}
```

- [ ] **Step 3: Create request ID middleware**

Create `cmdb-core/internal/middleware/requestid.go`:

```go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("request_id", id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}
```

- [ ] **Step 4: Create recovery middleware**

Create `cmdb-core/internal/middleware/recovery.go`:

```go
package middleware

import (
	"log"
	"net/http"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered: %v", r)
				response.Err(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
				c.Abort()
			}
		}()
		c.Next()
	}
}
```

- [ ] **Step 5: Create CORS middleware**

Create `cmdb-core/internal/middleware/cors.go`:

```go
package middleware

import (
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 6: Verify middleware compiles**

```bash
cd /cmdb-platform/cmdb-core
go mod tidy
go build ./internal/middleware/...
```

Expected: Build succeeds.

- [ ] **Step 7: Commit**

```bash
git add cmdb-core/internal/middleware/
git commit -m "feat: add middleware - JWT auth, tenant context, request ID, recovery, CORS"
```

---

## Task 6: Event Bus (NATS JetStream)

**Files:**
- Create: `cmdb-core/internal/eventbus/bus.go`
- Create: `cmdb-core/internal/eventbus/nats.go`
- Create: `cmdb-core/internal/eventbus/subjects.go`

- [ ] **Step 1: Create event bus interface**

Create `cmdb-core/internal/eventbus/bus.go`:

```go
package eventbus

import "context"

type Event struct {
	Subject  string
	TenantID string
	Payload  []byte
}

type Handler func(ctx context.Context, event Event) error

type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(subject string, handler Handler) error
	Close() error
}
```

- [ ] **Step 2: Create NATS JetStream implementation**

Create `cmdb-core/internal/eventbus/nats.go`:

```go
package eventbus

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NATSBus struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	subs   []*nats.Subscription
}

func NewNATSBus(url string) (*NATSBus, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream init: %w", err)
	}

	// Ensure stream exists for CMDB events
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "CMDB",
		Subjects:  []string{"asset.>", "alert.>", "maintenance.>", "import.>", "audit.>", "prediction.>", "config.>"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create stream: %w", err)
	}

	return &NATSBus{conn: nc, js: js}, nil
}

func (b *NATSBus) Publish(ctx context.Context, event Event) error {
	subject := event.Subject
	if event.TenantID != "" {
		subject = event.Subject + "." + event.TenantID
	}
	_, err := b.js.Publish(ctx, subject, event.Payload)
	return err
}

func (b *NATSBus) Subscribe(subject string, handler Handler) error {
	sub, err := b.conn.Subscribe(subject, func(msg *nats.Msg) {
		event := Event{
			Subject: msg.Subject,
			Payload: msg.Data,
		}
		if err := handler(context.Background(), event); err != nil {
			log.Printf("event handler error [%s]: %v", msg.Subject, err)
		}
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)
	return nil
}

func (b *NATSBus) Close() error {
	for _, sub := range b.subs {
		sub.Unsubscribe()
	}
	b.conn.Close()
	return nil
}
```

- [ ] **Step 3: Create event subject constants**

Create `cmdb-core/internal/eventbus/subjects.go`:

```go
package eventbus

const (
	SubjectAssetCreated       = "asset.created"
	SubjectAssetUpdated       = "asset.updated"
	SubjectAssetStatusChanged = "asset.status_changed"
	SubjectAssetDeleted       = "asset.deleted"

	SubjectRackOccupancyChanged = "rack.occupancy_changed"

	SubjectOrderCreated      = "maintenance.order_created"
	SubjectOrderTransitioned = "maintenance.order_transitioned"

	SubjectAlertFired    = "alert.fired"
	SubjectAlertResolved = "alert.resolved"

	SubjectImportCompleted = "import.completed"
	SubjectConflictCreated = "import.conflict_created"

	SubjectPredictionCreated = "prediction.created"

	SubjectAuditRecorded = "audit.recorded"
)
```

- [ ] **Step 4: Install NATS deps and verify build**

```bash
cd /cmdb-platform/cmdb-core
go get github.com/nats-io/nats.go
go get github.com/nats-io/nats.go/jetstream
go mod tidy
go build ./internal/eventbus/...
```

Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/eventbus/
git commit -m "feat: add NATS JetStream event bus with CMDB stream and subject constants"
```

---

## Task 7: Identity Module (Auth + Users/Roles CRUD)

**Files:**
- Create: `cmdb-core/internal/domain/identity/model.go`
- Create: `cmdb-core/internal/domain/identity/auth_service.go`
- Create: `cmdb-core/internal/domain/identity/service.go`
- Create: `cmdb-core/internal/domain/identity/handler.go`

This task covers 3 auth endpoints (POST /auth/login, POST /auth/refresh, GET /auth/me) and basic user/role listing.

- [ ] **Step 1: Create identity models**

Create `cmdb-core/internal/domain/identity/model.go`:

```go
package identity

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	DeptID      *uuid.UUID `json:"dept_id,omitempty"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email,omitempty"`
	Phone       string    `json:"phone,omitempty"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Role struct {
	ID          uuid.UUID       `json:"id"`
	TenantID    *uuid.UUID      `json:"tenant_id,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Permissions json.RawMessage `json:"permissions"`
	IsSystem    bool            `json:"is_system"`
	CreatedAt   time.Time       `json:"created_at"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type CurrentUser struct {
	ID          uuid.UUID              `json:"id"`
	Username    string                 `json:"username"`
	DisplayName string                 `json:"display_name"`
	Email       string                 `json:"email"`
	Permissions map[string][]string    `json:"permissions"`
}
```

Note: add `"encoding/json"` to imports for `json.RawMessage`.

- [ ] **Step 2: Create auth service**

Create `cmdb-core/internal/domain/identity/auth_service.go`:

```go
package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	queries   *dbgen.Queries
	redis     *redis.Client
	jwtSecret string
}

func NewAuthService(q *dbgen.Queries, r *redis.Client, secret string) *AuthService {
	return &AuthService{queries: q, redis: r, jwtSecret: secret}
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := s.queries.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if user.Status != "active" {
		return nil, fmt.Errorf("account is disabled")
	}

	return s.issueTokens(ctx, user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	// Validate refresh token exists in Redis (single-use)
	key := "refresh:" + refreshToken
	userID, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	// Delete used refresh token (single-use rotation)
	s.redis.Del(ctx, key)

	user, err := s.queries.GetUser(ctx, parseUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	return s.issueTokens(ctx, user)
}

func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*CurrentUser, error) {
	user, err := s.queries.GetUser(ctx, parseUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	roles, err := s.queries.ListUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	permissions := make(map[string][]string)
	for _, role := range roles {
		// Merge role permissions into user permissions
		var rolePerm map[string][]string
		if err := json.Unmarshal(role.Permissions, &rolePerm); err == nil {
			for k, v := range rolePerm {
				permissions[k] = append(permissions[k], v...)
			}
		}
	}

	return &CurrentUser{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       stringVal(user.Email),
		Permissions: permissions,
	}, nil
}

func (s *AuthService) issueTokens(ctx context.Context, user dbgen.User) (*TokenResponse, error) {
	expiresIn := 900 // 15 minutes
	claims := middleware.JWTClaims{
		UserID:    user.ID.String(),
		Username:  user.Username,
		TenantID:  user.TenantID.String(),
		ExpiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second).Unix(),
	}
	if user.DeptID != nil {
		claims.DeptID = user.DeptID.String()
	}

	accessToken, err := middleware.GenerateJWT(claims, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	// Generate refresh token and store in Redis (7 days)
	refreshToken := generateSecureToken()
	s.redis.Set(ctx, "refresh:"+refreshToken, user.ID.String(), 7*24*time.Hour)

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

func generateSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

Note: add `"crypto/rand"`, `"encoding/base64"`, `"encoding/json"` to imports, and `"github.com/google/uuid"`.

- [ ] **Step 3: Create identity service (user/role listing)**

Create `cmdb-core/internal/domain/identity/service.go`:

```go
package identity

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

func (s *Service) ListUsers(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]dbgen.User, int64, error) {
	users, err := s.queries.ListUsers(ctx, dbgen.ListUsersParams{
		TenantID: tenantID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := s.queries.CountUsers(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	return users, count, nil
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*dbgen.User, error) {
	user, err := s.queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Role, error) {
	return s.queries.ListRoles(ctx, &tenantID)
}
```

- [ ] **Step 4: Create identity HTTP handlers**

Create `cmdb-core/internal/domain/identity/handler.go`:

```go
package identity

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	authSvc *AuthService
	svc     *Service
}

func NewHandler(authSvc *AuthService, svc *Service) *Handler {
	return &Handler{authSvc: authSvc, svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Public endpoints (no auth)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)

	// Protected endpoints
	auth := r.Group("")
	auth.Use(authMiddleware)
	auth.GET("/auth/me", h.Me)
	auth.GET("/users", h.ListUsers)
	auth.GET("/users/:id", h.GetUser)
	auth.GET("/roles", h.ListRoles)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tokens, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, tokens)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tokens, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, tokens)
}

func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := h.authSvc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, user)
}

func (h *Handler) ListUsers(c *gin.Context) {
	tenantID := parseUUID(c.GetString("tenant_id"))
	page, pageSize, offset := response.ParsePagination(c)

	users, total, err := h.svc.ListUsers(c.Request.Context(), tenantID, pageSize, offset)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}
	response.OKList(c, users, page, pageSize, total)
}

func (h *Handler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	user, err := h.svc.GetUser(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, user)
}

func (h *Handler) ListRoles(c *gin.Context) {
	tenantID := parseUUID(c.GetString("tenant_id"))
	roles, err := h.svc.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list roles")
		return
	}
	response.OK(c, roles)
}
```

- [ ] **Step 5: Install bcrypt dep and verify build**

```bash
cd /cmdb-platform/cmdb-core
go get golang.org/x/crypto/bcrypt
go mod tidy
go build ./internal/domain/identity/...
```

Expected: Build succeeds.

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/internal/domain/identity/
git commit -m "feat: add identity module - auth login/refresh/me + user/role listing"
```

---

## Task 8: Topology Module (Locations + Racks — 10 endpoints)

**Files:**
- Create: `cmdb-core/internal/domain/topology/model.go`
- Create: `cmdb-core/internal/domain/topology/service.go`
- Create: `cmdb-core/internal/domain/topology/handler.go`

- [ ] **Step 1: Create topology models**

Create `cmdb-core/internal/domain/topology/model.go`:

```go
package topology

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Location struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Name      string          `json:"name"`
	NameEn    string          `json:"name_en,omitempty"`
	Slug      string          `json:"slug"`
	Level     string          `json:"level"`
	ParentID  *uuid.UUID      `json:"parent_id,omitempty"`
	Path      string          `json:"path"`
	Status    string          `json:"status"`
	Metadata  json.RawMessage `json:"metadata"`
	SortOrder int             `json:"sort_order"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Rack struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	LocationID      uuid.UUID `json:"location_id"`
	Name            string    `json:"name"`
	RowLabel        string    `json:"row_label,omitempty"`
	TotalU          int       `json:"total_u"`
	UsedU           int       `json:"used_u,omitempty"`
	PowerCapacityKW float64   `json:"power_capacity_kw,omitempty"`
	Status          string    `json:"status"`
	Tags            []string  `json:"tags"`
	CreatedAt       time.Time `json:"created_at"`
}

type LocationStats struct {
	TotalAssets    int64   `json:"total_assets"`
	TotalRacks     int64   `json:"total_racks"`
	CriticalAlerts int64   `json:"critical_alerts"`
	AvgOccupancy   float64 `json:"avg_occupancy"`
}
```

- [ ] **Step 2: Create topology service**

Create `cmdb-core/internal/domain/topology/service.go`:

```go
package topology

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

func (s *Service) ListRootLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error) {
	return s.queries.ListRootLocations(ctx, tenantID)
}

func (s *Service) GetLocation(ctx context.Context, id uuid.UUID) (*dbgen.Location, error) {
	loc, err := s.queries.GetLocation(ctx, id)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func (s *Service) ListChildren(ctx context.Context, id uuid.UUID) ([]dbgen.Location, error) {
	return s.queries.ListChildren(ctx, &id)
}

func (s *Service) ListAncestors(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error) {
	return s.queries.ListAncestors(ctx, dbgen.ListAncestorsParams{
		TenantID: tenantID,
		Path:     path,
	})
}

func (s *Service) GetLocationStats(ctx context.Context, locationID uuid.UUID) (*LocationStats, error) {
	// Count assets under this location
	assetCount, err := s.queries.CountAssets(ctx, dbgen.CountAssetsParams{
		TenantID:   uuid.Nil, // will be set via RLS
		LocationID: &locationID,
	})
	if err != nil {
		return nil, err
	}

	racks, err := s.queries.ListRacksByLocation(ctx, locationID)
	if err != nil {
		return nil, err
	}

	alerts, err := s.queries.CountAlerts(ctx, dbgen.CountAlertsParams{
		TenantID: uuid.Nil,
		Status:   strPtr("firing"),
	})
	if err != nil {
		return nil, err
	}

	return &LocationStats{
		TotalAssets:    assetCount,
		TotalRacks:     int64(len(racks)),
		CriticalAlerts: alerts,
	}, nil
}

func (s *Service) ListRacksByLocation(ctx context.Context, locationID uuid.UUID) ([]dbgen.Rack, error) {
	return s.queries.ListRacksByLocation(ctx, locationID)
}

func (s *Service) GetRack(ctx context.Context, id uuid.UUID) (*dbgen.Rack, error) {
	rack, err := s.queries.GetRack(ctx, id)
	if err != nil {
		return nil, err
	}
	return &rack, nil
}

func (s *Service) ListAssetsByRack(ctx context.Context, rackID uuid.UUID) ([]dbgen.Asset, error) {
	return s.queries.ListAssetsByRack(ctx, &rackID)
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 3: Create topology HTTP handlers**

Create `cmdb-core/internal/domain/topology/handler.go`:

```go
package topology

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/locations", h.ListRootLocations)
	r.GET("/locations/:id", h.GetLocation)
	r.GET("/locations/:id/children", h.ListChildren)
	r.GET("/locations/:id/ancestors", h.ListAncestors)
	r.GET("/locations/:id/stats", h.GetLocationStats)
	r.GET("/locations/:id/racks", h.ListRacksByLocation)
	r.GET("/racks/:id", h.GetRack)
	r.GET("/racks/:id/assets", h.ListAssetsByRack)
}

func (h *Handler) ListRootLocations(c *gin.Context) {
	tenantID := parseTenantID(c)
	locations, err := h.svc.ListRootLocations(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list locations")
		return
	}
	response.OK(c, locations)
}

func (h *Handler) GetLocation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid location id")
		return
	}
	loc, err := h.svc.GetLocation(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	response.OK(c, loc)
}

func (h *Handler) ListChildren(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid location id")
		return
	}
	children, err := h.svc.ListChildren(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list children")
		return
	}
	response.OK(c, children)
}

func (h *Handler) ListAncestors(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid location id")
		return
	}
	loc, err := h.svc.GetLocation(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	tenantID := parseTenantID(c)
	ancestors, err := h.svc.ListAncestors(c.Request.Context(), tenantID, loc.Path)
	if err != nil {
		response.InternalError(c, "failed to list ancestors")
		return
	}
	response.OK(c, ancestors)
}

func (h *Handler) GetLocationStats(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid location id")
		return
	}
	stats, err := h.svc.GetLocationStats(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to get stats")
		return
	}
	response.OK(c, stats)
}

func (h *Handler) ListRacksByLocation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid location id")
		return
	}
	racks, err := h.svc.ListRacksByLocation(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list racks")
		return
	}
	response.OK(c, racks)
}

func (h *Handler) GetRack(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid rack id")
		return
	}
	rack, err := h.svc.GetRack(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	response.OK(c, rack)
}

func (h *Handler) ListAssetsByRack(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid rack id")
		return
	}
	assets, err := h.svc.ListAssetsByRack(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list assets")
		return
	}
	response.OK(c, assets)
}

func parseTenantID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("tenant_id"))
	return id
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./internal/domain/topology/...
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/domain/topology/
git commit -m "feat: add topology module - locations hierarchy + racks + stats (8 endpoints)"
```

---

## Task 9: Asset Module (3 endpoints)

**Files:**
- Create: `cmdb-core/internal/domain/asset/model.go`
- Create: `cmdb-core/internal/domain/asset/service.go`
- Create: `cmdb-core/internal/domain/asset/handler.go`
- Create: `cmdb-core/internal/domain/asset/events.go`

- [ ] **Step 1: Create asset model and events**

Create `cmdb-core/internal/domain/asset/model.go`:

```go
package asset

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Asset struct {
	ID             uuid.UUID       `json:"id"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	AssetTag       string          `json:"asset_tag"`
	PropertyNumber *string         `json:"property_number,omitempty"`
	ControlNumber  *string         `json:"control_number,omitempty"`
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	SubType        *string         `json:"sub_type,omitempty"`
	Status         string          `json:"status"`
	BIALevel       string          `json:"bia_level"`
	LocationID     *uuid.UUID      `json:"location_id,omitempty"`
	RackID         *uuid.UUID      `json:"rack_id,omitempty"`
	Vendor         *string         `json:"vendor,omitempty"`
	Model          *string         `json:"model,omitempty"`
	SerialNumber   *string         `json:"serial_number,omitempty"`
	Attributes     json.RawMessage `json:"attributes"`
	Tags           []string        `json:"tags"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
```

Create `cmdb-core/internal/domain/asset/events.go`:

```go
package asset

import (
	"encoding/json"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
)

type AssetEvent struct {
	AssetID  uuid.UUID `json:"asset_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Action   string    `json:"action"`
}

func NewAssetEvent(assetID, tenantID uuid.UUID, subject string) eventbus.Event {
	payload, _ := json.Marshal(AssetEvent{
		AssetID:  assetID,
		TenantID: tenantID,
		Action:   subject,
	})
	return eventbus.Event{
		Subject:  subject,
		TenantID: tenantID.String(),
		Payload:  payload,
	}
}
```

- [ ] **Step 2: Create asset service**

Create `cmdb-core/internal/domain/asset/service.go`:

```go
package asset

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
)

type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
}

func NewService(q *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: q, bus: bus}
}

type ListParams struct {
	TenantID     uuid.UUID
	Type         *string
	Status       *string
	LocationID   *uuid.UUID
	RackID       *uuid.UUID
	SerialNumber *string
	Limit        int
	Offset       int
}

func (s *Service) List(ctx context.Context, p ListParams) ([]dbgen.Asset, int64, error) {
	assets, err := s.queries.ListAssets(ctx, dbgen.ListAssetsParams{
		TenantID:     p.TenantID,
		Type:         p.Type,
		Status:       p.Status,
		LocationID:   p.LocationID,
		RackID:       p.RackID,
		SerialNumber: p.SerialNumber,
		Limit:        int32(p.Limit),
		Offset:       int32(p.Offset),
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := s.queries.CountAssets(ctx, dbgen.CountAssetsParams{
		TenantID:   p.TenantID,
		Type:       p.Type,
		Status:     p.Status,
		LocationID: p.LocationID,
	})
	if err != nil {
		return nil, 0, err
	}

	return assets, count, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.Asset, error) {
	a, err := s.queries.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) Create(ctx context.Context, params dbgen.CreateAssetParams) (*dbgen.Asset, error) {
	a, err := s.queries.CreateAsset(ctx, params)
	if err != nil {
		return nil, err
	}

	if s.bus != nil {
		s.bus.Publish(ctx, NewAssetEvent(a.ID, a.TenantID, eventbus.SubjectAssetCreated))
	}

	return &a, nil
}
```

- [ ] **Step 3: Create asset HTTP handlers**

Create `cmdb-core/internal/domain/asset/handler.go`:

```go
package asset

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/assets", h.List)
	r.GET("/assets/:id", h.GetByID)
	r.POST("/assets", h.Create)
}

func (h *Handler) List(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	page, pageSize, offset := response.ParsePagination(c)

	params := ListParams{
		TenantID: tenantID,
		Limit:    pageSize,
		Offset:   offset,
	}
	if v := c.Query("type"); v != "" {
		params.Type = &v
	}
	if v := c.Query("status"); v != "" {
		params.Status = &v
	}
	if v := c.Query("serial_number"); v != "" {
		params.SerialNumber = &v
	}
	if v := c.Query("location_id"); v != "" {
		id, _ := uuid.Parse(v)
		params.LocationID = &id
	}
	if v := c.Query("rack_id"); v != "" {
		id, _ := uuid.Parse(v)
		params.RackID = &id
	}

	assets, total, err := h.svc.List(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to list assets")
		return
	}
	response.OKList(c, assets, page, pageSize, total)
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid asset id")
		return
	}

	asset, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	response.OK(c, asset)
}

func (h *Handler) Create(c *gin.Context) {
	// Placeholder for Phase 2 — P1 endpoint
	// For now returns 501
	response.Err(c, 501, "NOT_IMPLEMENTED", "asset creation will be available in Phase 2")
}
```

- [ ] **Step 4: Verify and commit**

```bash
go build ./internal/domain/asset/...
git add cmdb-core/internal/domain/asset/
git commit -m "feat: add asset module - list, get by ID, serial number search (3 endpoints)"
```

---

## Task 10: Maintenance Module (4 endpoints)

**Files:**
- Create: `cmdb-core/internal/domain/maintenance/model.go`
- Create: `cmdb-core/internal/domain/maintenance/statemachine.go`
- Create: `cmdb-core/internal/domain/maintenance/service.go`
- Create: `cmdb-core/internal/domain/maintenance/handler.go`

- [ ] **Step 1: Create maintenance model + state machine**

Create `cmdb-core/internal/domain/maintenance/model.go`:

```go
package maintenance

import (
	"time"

	"github.com/google/uuid"
)

type WorkOrder struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	Code           string     `json:"code"`
	Title          string     `json:"title"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Priority       string     `json:"priority"`
	LocationID     *uuid.UUID `json:"location_id,omitempty"`
	AssetID        *uuid.UUID `json:"asset_id,omitempty"`
	RequestorID    *uuid.UUID `json:"requestor_id,omitempty"`
	AssigneeID     *uuid.UUID `json:"assignee_id,omitempty"`
	Description    *string    `json:"description,omitempty"`
	Reason         *string    `json:"reason,omitempty"`
	ScheduledStart *time.Time `json:"scheduled_start,omitempty"`
	ScheduledEnd   *time.Time `json:"scheduled_end,omitempty"`
	ActualStart    *time.Time `json:"actual_start,omitempty"`
	ActualEnd      *time.Time `json:"actual_end,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type TransitionRequest struct {
	Status  string `json:"status" binding:"required"`
	Comment string `json:"comment"`
}

type CreateOrderRequest struct {
	Title          string     `json:"title" binding:"required"`
	Type           string     `json:"type" binding:"required"`
	Priority       string     `json:"priority"`
	LocationID     *uuid.UUID `json:"location_id"`
	AssetID        *uuid.UUID `json:"asset_id"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	Description    string     `json:"description"`
	Reason         string     `json:"reason"`
	ScheduledStart *time.Time `json:"scheduled_start"`
	ScheduledEnd   *time.Time `json:"scheduled_end"`
}
```

Create `cmdb-core/internal/domain/maintenance/statemachine.go`:

```go
package maintenance

import "fmt"

// Valid transitions: draft->pending->approved->in_progress->completed->closed
//                    draft->pending->rejected
var validTransitions = map[string][]string{
	"draft":       {"pending"},
	"pending":     {"approved", "rejected"},
	"approved":    {"in_progress"},
	"in_progress": {"completed"},
	"completed":   {"closed"},
}

func ValidateTransition(from, to string) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("no transitions from status %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("cannot transition from %q to %q", from, to)
}
```

- [ ] **Step 2: Create maintenance service**

Create `cmdb-core/internal/domain/maintenance/service.go`:

```go
package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
)

type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
}

func NewService(q *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: q, bus: bus}
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status *string, limit, offset int) ([]dbgen.WorkOrder, int64, error) {
	orders, err := s.queries.ListWorkOrders(ctx, dbgen.ListWorkOrdersParams{
		TenantID: tenantID,
		Status:   status,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := s.queries.CountWorkOrders(ctx, dbgen.CountWorkOrdersParams{
		TenantID: tenantID,
		Status:   status,
	})
	if err != nil {
		return nil, 0, err
	}
	return orders, count, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.WorkOrder, error) {
	o, err := s.queries.GetWorkOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Service) Create(ctx context.Context, tenantID, requestorID uuid.UUID, req CreateOrderRequest) (*dbgen.WorkOrder, error) {
	code := fmt.Sprintf("WO-%s-%04d", time.Now().Format("2006"), time.Now().UnixMilli()%10000)
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	order, err := s.queries.CreateWorkOrder(ctx, dbgen.CreateWorkOrderParams{
		TenantID:       tenantID,
		Code:           code,
		Title:          req.Title,
		Type:           req.Type,
		Status:         "draft",
		Priority:       priority,
		LocationID:     req.LocationID,
		AssetID:        req.AssetID,
		RequestorID:    &requestorID,
		AssigneeID:     req.AssigneeID,
		Description:    &req.Description,
		Reason:         &req.Reason,
		ScheduledStart: req.ScheduledStart,
		ScheduledEnd:   req.ScheduledEnd,
	})
	if err != nil {
		return nil, err
	}

	s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
		OrderID:    order.ID,
		Action:     "created",
		ToStatus:   &order.Status,
		OperatorID: &requestorID,
	})

	return &order, nil
}

func (s *Service) Transition(ctx context.Context, id uuid.UUID, operatorID uuid.UUID, req TransitionRequest) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("work order not found")
	}

	if err := ValidateTransition(order.Status, req.Status); err != nil {
		return nil, err
	}

	fromStatus := order.Status
	updated, err := s.queries.UpdateWorkOrderStatus(ctx, dbgen.UpdateWorkOrderStatusParams{
		ID:     id,
		Status: req.Status,
	})
	if err != nil {
		return nil, err
	}

	s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
		OrderID:    id,
		Action:     "transitioned",
		FromStatus: &fromStatus,
		ToStatus:   &req.Status,
		OperatorID: &operatorID,
		Comment:    &req.Comment,
	})

	return &updated, nil
}
```

- [ ] **Step 3: Create maintenance handlers**

Create `cmdb-core/internal/domain/maintenance/handler.go`:

```go
package maintenance

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/maintenance/orders", h.List)
	r.GET("/maintenance/orders/:id", h.GetByID)
	r.POST("/maintenance/orders", h.Create)
	r.POST("/maintenance/orders/:id/transition", h.Transition)
}

func (h *Handler) List(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	page, pageSize, offset := response.ParsePagination(c)

	var status *string
	if v := c.Query("status"); v != "" {
		status = &v
	}

	orders, total, err := h.svc.List(c.Request.Context(), tenantID, status, pageSize, offset)
	if err != nil {
		response.InternalError(c, "failed to list work orders")
		return
	}
	response.OKList(c, orders, page, pageSize, total)
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid order id")
		return
	}
	order, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "work order not found")
		return
	}
	response.OK(c, order)
}

func (h *Handler) Create(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	userID, _ := uuid.Parse(c.GetString("user_id"))

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	order, err := h.svc.Create(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, order)
}

func (h *Handler) Transition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid order id")
		return
	}
	userID, _ := uuid.Parse(c.GetString("user_id"))

	var req TransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	order, err := h.svc.Transition(c.Request.Context(), id, userID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, order)
}
```

- [ ] **Step 4: Verify and commit**

```bash
go build ./internal/domain/maintenance/...
git add cmdb-core/internal/domain/maintenance/
git commit -m "feat: add maintenance module - work orders CRUD + state machine (4 endpoints)"
```

---

## Task 11: Monitoring Module (3 endpoints)

**Files:**
- Create: `cmdb-core/internal/domain/monitoring/model.go`
- Create: `cmdb-core/internal/domain/monitoring/service.go`
- Create: `cmdb-core/internal/domain/monitoring/handler.go`

- [ ] **Step 1: Create monitoring model, service, handler**

Create `cmdb-core/internal/domain/monitoring/model.go`:

```go
package monitoring

import (
	"time"

	"github.com/google/uuid"
)

type AlertEvent struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	RuleID       *uuid.UUID `json:"rule_id,omitempty"`
	AssetID      *uuid.UUID `json:"asset_id,omitempty"`
	Status       string     `json:"status"`
	Severity     string     `json:"severity"`
	Message      string     `json:"message"`
	TriggerValue *float64   `json:"trigger_value,omitempty"`
	FiredAt      time.Time  `json:"fired_at"`
	AckedAt      *time.Time `json:"acked_at,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}
```

Create `cmdb-core/internal/domain/monitoring/service.go`:

```go
package monitoring

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

func (s *Service) ListAlerts(ctx context.Context, tenantID uuid.UUID, status, severity *string, assetID *uuid.UUID, limit, offset int) ([]dbgen.AlertEvent, int64, error) {
	alerts, err := s.queries.ListAlerts(ctx, dbgen.ListAlertsParams{
		TenantID: tenantID,
		Status:   status,
		Severity: severity,
		AssetID:  assetID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := s.queries.CountAlerts(ctx, dbgen.CountAlertsParams{
		TenantID: tenantID,
		Status:   status,
	})
	if err != nil {
		return nil, 0, err
	}
	return alerts, count, nil
}

func (s *Service) Acknowledge(ctx context.Context, id uuid.UUID) (*dbgen.AlertEvent, error) {
	a, err := s.queries.AcknowledgeAlert(ctx, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) Resolve(ctx context.Context, id uuid.UUID) (*dbgen.AlertEvent, error) {
	a, err := s.queries.ResolveAlert(ctx, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
```

Create `cmdb-core/internal/domain/monitoring/handler.go`:

```go
package monitoring

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/monitoring/alerts", h.ListAlerts)
	r.POST("/monitoring/alerts/:id/ack", h.Acknowledge)
	r.POST("/monitoring/alerts/:id/resolve", h.Resolve)
}

func (h *Handler) ListAlerts(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	page, pageSize, offset := response.ParsePagination(c)

	var status, severity *string
	var assetID *uuid.UUID
	if v := c.Query("status"); v != "" {
		status = &v
	}
	if v := c.Query("severity"); v != "" {
		severity = &v
	}
	if v := c.Query("asset_id"); v != "" {
		id, _ := uuid.Parse(v)
		assetID = &id
	}

	alerts, total, err := h.svc.ListAlerts(c.Request.Context(), tenantID, status, severity, assetID, pageSize, offset)
	if err != nil {
		response.InternalError(c, "failed to list alerts")
		return
	}
	response.OKList(c, alerts, page, pageSize, total)
}

func (h *Handler) Acknowledge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid alert id")
		return
	}
	alert, err := h.svc.Acknowledge(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, alert)
}

func (h *Handler) Resolve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid alert id")
		return
	}
	alert, err := h.svc.Resolve(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, alert)
}
```

- [ ] **Step 2: Verify and commit**

```bash
go build ./internal/domain/monitoring/...
git add cmdb-core/internal/domain/monitoring/
git commit -m "feat: add monitoring module - list alerts, acknowledge, resolve (3 endpoints)"
```

---

## Task 12: Inventory + Audit + Dashboard Modules (5 endpoints)

**Files:**
- Create: `cmdb-core/internal/domain/inventory/model.go`
- Create: `cmdb-core/internal/domain/inventory/service.go`
- Create: `cmdb-core/internal/domain/inventory/handler.go`
- Create: `cmdb-core/internal/domain/audit/model.go`
- Create: `cmdb-core/internal/domain/audit/service.go`
- Create: `cmdb-core/internal/domain/audit/handler.go`
- Create: `cmdb-core/internal/domain/dashboard/service.go`
- Create: `cmdb-core/internal/domain/dashboard/handler.go`

- [ ] **Step 1: Create inventory module**

Create `cmdb-core/internal/domain/inventory/model.go`:

```go
package inventory

type InventoryTask struct{}  // Uses dbgen types directly
```

Create `cmdb-core/internal/domain/inventory/service.go`:

```go
package inventory

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

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]dbgen.InventoryTask, int64, error) {
	tasks, err := s.queries.ListInventoryTasks(ctx, dbgen.ListInventoryTasksParams{
		TenantID: tenantID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := s.queries.CountInventoryTasks(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	return tasks, count, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.InventoryTask, error) {
	t, err := s.queries.GetInventoryTask(ctx, id)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) ListItems(ctx context.Context, taskID uuid.UUID) ([]dbgen.InventoryItem, error) {
	return s.queries.ListInventoryItems(ctx, taskID)
}
```

Create `cmdb-core/internal/domain/inventory/handler.go`:

```go
package inventory

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/inventory/tasks", h.List)
	r.GET("/inventory/tasks/:id", h.GetByID)
	r.GET("/inventory/tasks/:id/items", h.ListItems)
}

func (h *Handler) List(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	page, pageSize, offset := response.ParsePagination(c)
	tasks, total, err := h.svc.List(c.Request.Context(), tenantID, pageSize, offset)
	if err != nil {
		response.InternalError(c, "failed to list tasks")
		return
	}
	response.OKList(c, tasks, page, pageSize, total)
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}
	task, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "task not found")
		return
	}
	response.OK(c, task)
}

func (h *Handler) ListItems(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}
	items, err := h.svc.ListItems(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list items")
		return
	}
	response.OK(c, items)
}
```

- [ ] **Step 2: Create audit module**

Create `cmdb-core/internal/domain/audit/model.go`:

```go
package audit

type AuditEvent struct{} // Uses dbgen types directly
```

Create `cmdb-core/internal/domain/audit/service.go`:

```go
package audit

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

func (s *Service) Query(ctx context.Context, tenantID uuid.UUID, module, targetType *string, targetID *uuid.UUID, limit, offset int) ([]dbgen.AuditEvent, int64, error) {
	events, err := s.queries.QueryAuditEvents(ctx, dbgen.QueryAuditEventsParams{
		TenantID:   tenantID,
		Module:     module,
		TargetType: targetType,
		TargetID:   targetID,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := s.queries.CountAuditEvents(ctx, dbgen.CountAuditEventsParams{
		TenantID:   tenantID,
		Module:     module,
		TargetType: targetType,
		TargetID:   targetID,
	})
	if err != nil {
		return nil, 0, err
	}
	return events, count, nil
}
```

Create `cmdb-core/internal/domain/audit/handler.go`:

```go
package audit

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/audit/events", h.Query)
}

func (h *Handler) Query(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	page, pageSize, offset := response.ParsePagination(c)

	var module, targetType *string
	var targetID *uuid.UUID
	if v := c.Query("module"); v != "" {
		module = &v
	}
	if v := c.Query("target_type"); v != "" {
		targetType = &v
	}
	if v := c.Query("target_id"); v != "" {
		id, _ := uuid.Parse(v)
		targetID = &id
	}

	events, total, err := h.svc.Query(c.Request.Context(), tenantID, module, targetType, targetID, pageSize, offset)
	if err != nil {
		response.InternalError(c, "failed to query audit events")
		return
	}
	response.OKList(c, events, page, pageSize, total)
}
```

- [ ] **Step 3: Create dashboard module**

Create `cmdb-core/internal/domain/dashboard/service.go`:

```go
package dashboard

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

type Stats struct {
	TotalAssets    int64 `json:"total_assets"`
	TotalRacks     int64 `json:"total_racks"`
	CriticalAlerts int64 `json:"critical_alerts"`
	ActiveOrders   int64 `json:"active_orders"`
}

type Service struct {
	queries *dbgen.Queries
}

func NewService(q *dbgen.Queries) *Service {
	return &Service{queries: q}
}

func (s *Service) GetStats(ctx context.Context, tenantID uuid.UUID) (*Stats, error) {
	assetCount, err := s.queries.CountAssets(ctx, dbgen.CountAssetsParams{TenantID: tenantID})
	if err != nil {
		return nil, err
	}

	firingStatus := "firing"
	alertCount, err := s.queries.CountAlerts(ctx, dbgen.CountAlertsParams{
		TenantID: tenantID,
		Status:   &firingStatus,
	})
	if err != nil {
		return nil, err
	}

	inProgressStatus := "in_progress"
	orderCount, err := s.queries.CountWorkOrders(ctx, dbgen.CountWorkOrdersParams{
		TenantID: tenantID,
		Status:   &inProgressStatus,
	})
	if err != nil {
		return nil, err
	}

	return &Stats{
		TotalAssets:    assetCount,
		CriticalAlerts: alertCount,
		ActiveOrders:   orderCount,
	}, nil
}
```

Create `cmdb-core/internal/domain/dashboard/handler.go`:

```go
package dashboard

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/dashboard/stats", h.GetStats)
}

func (h *Handler) GetStats(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	stats, err := h.svc.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get dashboard stats")
		return
	}
	response.OK(c, stats)
}
```

- [ ] **Step 4: Verify and commit**

```bash
go build ./internal/domain/inventory/... ./internal/domain/audit/... ./internal/domain/dashboard/...
git add cmdb-core/internal/domain/inventory/ cmdb-core/internal/domain/audit/ cmdb-core/internal/domain/dashboard/
git commit -m "feat: add inventory (3), audit (1), dashboard (1) modules - 5 endpoints total"
```

---

## Task 13: Wire Everything Together in main.go + Seed Data

**Files:**
- Modify: `cmdb-core/cmd/server/main.go`
- Create: `cmdb-core/db/seed/seed.sql`

- [ ] **Step 1: Rewrite main.go to wire all modules**

Replace `cmdb-core/cmd/server/main.go` with:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/audit"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/cache"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	// Database
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	queries := dbgen.New(pool)

	// Redis
	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redisClient.Close()

	// Event bus
	var bus eventbus.Bus
	natsBus, err := eventbus.NewNATSBus(cfg.NatsURL)
	if err != nil {
		log.Printf("WARN: NATS not available, events disabled: %v", err)
	} else {
		bus = natsBus
		defer natsBus.Close()
	}

	// Services
	authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret)
	identitySvc := identity.NewService(queries)
	topologySvc := topology.NewService(queries)
	assetSvc := asset.NewService(queries, bus)
	maintenanceSvc := maintenance.NewService(queries, bus)
	monitoringSvc := monitoring.NewService(queries)
	inventorySvc := inventory.NewService(queries)
	auditSvc := audit.NewService(queries)
	dashboardSvc := dashboard.NewService(queries)

	// Handlers
	identityHandler := identity.NewHandler(authSvc, identitySvc)
	topologyHandler := topology.NewHandler(topologySvc)
	assetHandler := asset.NewHandler(assetSvc)
	maintenanceHandler := maintenance.NewHandler(maintenanceSvc)
	monitoringHandler := monitoring.NewHandler(monitoringSvc)
	inventoryHandler := inventory.NewHandler(inventorySvc)
	auditHandler := audit.NewHandler(auditSvc)
	dashboardHandler := dashboard.NewHandler(dashboardSvc)

	// Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())

	// Health check (no auth)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1
	v1 := r.Group("/api/v1")
	authMW := middleware.Auth(cfg.JWTSecret)

	// Identity (login/refresh are public, rest need auth)
	identityHandler.Register(v1, authMW)

	// All other modules need auth
	protected := v1.Group("")
	protected.Use(authMW)

	topologyHandler.Register(protected)
	assetHandler.Register(protected)
	maintenanceHandler.Register(protected)
	monitoringHandler.Register(protected)
	inventoryHandler.Register(protected)
	auditHandler.Register(protected)
	dashboardHandler.Register(protected)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("cmdb-core started on %s (mode=%s)", addr, cfg.DeployMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
```

- [ ] **Step 2: Create seed data**

Create `cmdb-core/db/seed/seed.sql`:

```sql
-- Seed tenant
INSERT INTO tenants (id, name, slug) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Taipei Campus', 'tw')
ON CONFLICT (slug) DO NOTHING;

-- Seed admin user (password: admin123)
INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source) VALUES
    ('b0000000-0000-0000-0000-000000000001',
     'a0000000-0000-0000-0000-000000000001',
     'admin',
     'System Admin',
     'admin@cmdb.local',
     '$2a$12$LJ3m4ys/ApX6iBHaRfMwWeSS.lCGHXBxWKCy/OPPIa4IhzRo.mJHq',
     'active',
     'local')
ON CONFLICT (username) DO NOTHING;

-- Seed admin role
INSERT INTO roles (id, tenant_id, name, description, permissions, is_system) VALUES
    ('c0000000-0000-0000-0000-000000000001',
     NULL,
     'super-admin',
     'Full system access',
     '{"*": ["*"]}',
     true)
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;

-- Seed location hierarchy
INSERT INTO locations (id, tenant_id, name, name_en, slug, level, parent_id, path, metadata, sort_order) VALUES
    ('d0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', '台灣', 'Taiwan', 'tw', 'country', NULL, 'tw', '{}', 1),
    ('d0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', '北部', 'North', 'north', 'region', 'd0000000-0000-0000-0000-000000000001', 'tw.north', '{}', 1),
    ('d0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', '台北', 'Taipei', 'taipei', 'city', 'd0000000-0000-0000-0000-000000000002', 'tw.north.taipei', '{}', 1),
    ('d0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', '内湖園區', 'Neihu Campus', 'neihu', 'campus', 'd0000000-0000-0000-0000-000000000003', 'tw.north.taipei.neihu', '{"pue": 1.45}', 1)
ON CONFLICT DO NOTHING;

-- Seed racks
INSERT INTO racks (id, tenant_id, location_id, name, row_label, total_u, power_capacity_kw, status) VALUES
    ('e0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-A01', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-A02', 'A', 42, 10.0, 'active'),
    ('e0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004', 'RACK-B01', 'B', 42, 12.0, 'active')
ON CONFLICT DO NOTHING;

-- Seed assets
INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, bia_level, location_id, rack_id, vendor, model, serial_number) VALUES
    ('f0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'SRV-PROD-001', 'Production Server 01', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Dell', 'PowerEdge R750', 'SN-DELL-001'),
    ('f0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'SRV-PROD-002', 'Production Server 02', 'server', 'rack_server', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Dell', 'PowerEdge R750', 'SN-DELL-002'),
    ('f0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'NET-SW-A01', 'Core Switch A01', 'network', 'switch', 'operational', 'critical', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000002', 'Cisco', 'Nexus 9336C-FX2', 'SN-CISCO-001'),
    ('f0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001', 'STG-NAS-001', 'NAS Storage 01', 'storage', 'nas', 'operational', 'important', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000003', 'Synology', 'SA3600', 'SN-SYN-001')
ON CONFLICT (asset_tag) DO NOTHING;

-- Seed alert events
INSERT INTO alert_events (id, tenant_id, asset_id, status, severity, message, fired_at) VALUES
    ('10000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'firing', 'warning', 'CPU usage above 85% for 10 minutes', now() - interval '2 hours'),
    ('10000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003', 'firing', 'critical', 'Interface Eth1/1 down', now() - interval '30 minutes')
ON CONFLICT DO NOTHING;
```

- [ ] **Step 3: Build, seed, and test**

```bash
cd /cmdb-platform/cmdb-core
go mod tidy
go build ./cmd/server
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" make seed
```

Expected: Seed data inserted. Binary builds successfully.

- [ ] **Step 4: Start server and smoke test**

```bash
# Terminal 1: Start server
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" ./bin/cmdb-core

# Terminal 2: Test endpoints
# Health
curl -s http://localhost:8080/healthz | jq .
# Expected: {"status":"ok"}

# Login
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq .
# Expected: {"data":{"access_token":"...","refresh_token":"...","expires_in":900},"meta":{...}}

# Use the access_token for subsequent requests
TOKEN="<paste access_token here>"

# List locations
curl -s http://localhost:8080/api/v1/locations \
  -H "Authorization: Bearer $TOKEN" | jq .

# List assets
curl -s http://localhost:8080/api/v1/assets \
  -H "Authorization: Bearer $TOKEN" | jq .

# Dashboard stats
curl -s http://localhost:8080/api/v1/dashboard/stats \
  -H "Authorization: Bearer $TOKEN" | jq .
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/cmd/server/main.go cmdb-core/db/seed/
git commit -m "feat: wire all 9 modules into main server with seed data - 28 endpoints live"
```

---

## Endpoint Summary

After completing all 13 tasks, the following 28 P0 endpoints are functional:

| # | Method | Path | Module | Task |
|---|--------|------|--------|------|
| 1 | POST | `/api/v1/auth/login` | Identity | 7 |
| 2 | POST | `/api/v1/auth/refresh` | Identity | 7 |
| 3 | GET | `/api/v1/auth/me` | Identity | 7 |
| 4 | GET | `/api/v1/users` | Identity | 7 |
| 5 | GET | `/api/v1/users/:id` | Identity | 7 |
| 6 | GET | `/api/v1/roles` | Identity | 7 |
| 7 | GET | `/api/v1/locations` | Topology | 8 |
| 8 | GET | `/api/v1/locations/:id` | Topology | 8 |
| 9 | GET | `/api/v1/locations/:id/children` | Topology | 8 |
| 10 | GET | `/api/v1/locations/:id/ancestors` | Topology | 8 |
| 11 | GET | `/api/v1/locations/:id/stats` | Topology | 8 |
| 12 | GET | `/api/v1/locations/:id/racks` | Topology | 8 |
| 13 | GET | `/api/v1/racks/:id` | Topology | 8 |
| 14 | GET | `/api/v1/racks/:id/assets` | Topology | 8 |
| 15 | GET | `/api/v1/assets` | Asset | 9 |
| 16 | GET | `/api/v1/assets/:id` | Asset | 9 |
| 17 | POST | `/api/v1/assets` | Asset | 9 |
| 18 | GET | `/api/v1/maintenance/orders` | Maintenance | 10 |
| 19 | GET | `/api/v1/maintenance/orders/:id` | Maintenance | 10 |
| 20 | POST | `/api/v1/maintenance/orders` | Maintenance | 10 |
| 21 | POST | `/api/v1/maintenance/orders/:id/transition` | Maintenance | 10 |
| 22 | GET | `/api/v1/monitoring/alerts` | Monitoring | 11 |
| 23 | POST | `/api/v1/monitoring/alerts/:id/ack` | Monitoring | 11 |
| 24 | POST | `/api/v1/monitoring/alerts/:id/resolve` | Monitoring | 11 |
| 25 | GET | `/api/v1/inventory/tasks` | Inventory | 12 |
| 26 | GET | `/api/v1/inventory/tasks/:id` | Inventory | 12 |
| 27 | GET | `/api/v1/inventory/tasks/:id/items` | Inventory | 12 |
| 28 | GET | `/api/v1/audit/events` | Audit | 12 |
| 29 | GET | `/api/v1/dashboard/stats` | Dashboard | 12 |
