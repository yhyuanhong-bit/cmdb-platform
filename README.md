# CMDB Platform

> Industrial-grade Configuration Management Database with Edge offline sync, BIA modeling, predictive AI, and SNMP location detection.

---

## Quick Start (Docker Compose)

**Prerequisites:** Docker 24+ and Docker Compose v2

```bash
git clone https://github.com/yhyuanhong-bit/cmdb-platform.git
cd cmdb-platform

# Copy and edit environment config
cp cmdb-core/deploy/.env.example cmdb-core/deploy/.env
vim cmdb-core/deploy/.env   # Set JWT_SECRET, DB_PASS, REDIS_PASS

# Start all services
docker compose -f cmdb-core/deploy/docker-compose.yml up -d

# Wait ~30 seconds for startup + auto-migration + seed
```

Open `http://localhost` and login:
- **Username:** `admin`
- **Password:** `admin123`

> Change the admin password after first login.

---

## Quick Start (Development)

**Prerequisites:** Go 1.22+, Node.js 20+, PostgreSQL 15+, Redis 7+, NATS 2.10+

### Backend

```bash
cd cmdb-core

# Database setup
createdb cmdb
psql cmdb -f db/migrations/*.up.sql   # or let auto-migration handle it

# Seed data (optional, auto-runs on first startup if DB is empty)
psql cmdb -f db/seed/seed.sql

# Start
export DATABASE_URL="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
export DEPLOY_MODE=edge   # allows default JWT secret for dev
export TENANT_ID=a0000000-0000-0000-0000-000000000001
go run ./cmd/server/
```

Backend runs on `http://localhost:8080`

### Frontend

```bash
cd cmdb-demo
npm install --legacy-peer-deps
npm run dev
```

Frontend runs on `http://localhost:5175`

---

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Frontend   │────▶│  cmdb-core  │────▶│ PostgreSQL   │
│  React/Vite  │     │   Go/Gin    │     │ TimescaleDB  │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
                    ┌──────┼──────┐
                    │      │      │
               ┌────▼──┐ ┌▼────┐ ┌▼────────┐
               │ Redis  │ │NATS │ │Ingestion │
               │ Cache  │ │ JS  │ │ Engine   │
               └────────┘ └─────┘ └──────────┘
```

| Component | Tech | Port |
|-----------|------|------|
| Frontend | React 18, Vite, Tailwind, Recharts | 80 (nginx) / 5175 (dev) |
| Backend | Go, Gin, sqlc, pgx | 8080 |
| Database | PostgreSQL 17 + TimescaleDB | 5432 |
| Cache | Redis 7.4 | 6379 |
| Message Bus | NATS JetStream | 4222, 7422 (leafnode) |
| Ingestion | Python, FastAPI | 8000 |

---

## Features

### Core CMDB
- Asset management (CRUD, import/export, lifecycle tracking)
- Location hierarchy (territory → region → city → campus)
- Rack management (U-slot visualization, power monitoring)
- Interactive Leaflet map with real-time health indicators

### Operations
- Work order management with dual-dimension state machine
- High-speed inventory with barcode/Excel import
- Alert monitoring with acknowledge/resolve workflow
- Sensor configuration and alert rule management

### Edge Sync (v1.2)
- Edge nodes with bidirectional sync and central failover
- Bidirectional sync for assets, work orders, inventory, alerts
- Conflict resolution UI
- Sync envelope retention: 14 days (NATS JetStream MaxAge)

> **Scope note**: Edge nodes require connectivity to central for writes. The
> `SyncGateMiddleware` returns HTTP 503 with `Retry-After` while initial sync
> is in progress. True offline-write buffering is not currently implemented.
> See [docs/ROADMAP.md](docs/ROADMAP.md).

### Intelligence
- BIA (Business Impact Analysis) modeling
- Asset health scoring + LLM-assisted root cause analysis (beta)
- Data quality scoring (4 dimensions)
- SNMP location detection

> **About "AI" in this platform**: Health scoring is rule-based
> (`min(warranty_remaining, lifespan_remaining)` + alert frequency). Root
> cause analysis routes incident context to a configurable LLM provider
> (OpenAI / Claude / Dify / custom HTTP endpoint). Native ML failure
> prediction is on the roadmap pending sufficient operational telemetry
> data — see [docs/ROADMAP.md](docs/ROADMAP.md).

### Observability
- Prometheus metrics pull from adapters
- Sync monitoring dashboard
- Audit trail (append-only, immutable)
- 15 Playwright E2E tests

---

## Deployment

### Docker Compose (recommended)

See [Production Deployment Guide](docs/production-deployment-guide.md)

### Offline / Air-Gapped

```bash
# On internet-connected machine: build offline package
./scripts/build-offline-package.sh v1.2.1

# Copy to target, then:
tar xzf cmdb-platform-offline-v1.2.1.tar.gz
cd cmdb-platform-offline
vim .env
./install.sh
```

See [Backup & Recovery](docs/backup-recovery.md)

### Private Registry

```bash
# Setup registry
./scripts/setup-registry.sh

# Push images
./scripts/push-to-registry.sh registry.internal:5000/cmdb v1.2.1

# On target machines
docker compose -f docker-compose.yml -f docker-compose.registry.yml up -d
```

See [Private Registry Guide](docs/private-registry-guide.md)

### Edge Node

See [Edge Deployment Guide](docs/edge-deployment-guide.md)

---

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEPLOY_MODE` | yes | `cloud` | `cloud` (production) or `edge` |
| `JWT_SECRET` | yes (cloud) | `dev-secret-change-me` | 64+ char random string |
| `DATABASE_URL` | yes | `postgres://cmdb:changeme@...` | PostgreSQL connection |
| `REDIS_URL` | yes | `redis://localhost:6379/0` | Redis connection |
| `NATS_URL` | yes | `nats://localhost:4222` | NATS connection |
| `TENANT_ID` | edge only | — | Tenant UUID |
| `EDGE_NODE_ID` | edge only | — | Unique node ID |
| `SYNC_ENABLED` | no | `true` | Enable Edge sync |

Full reference: [.env.example](cmdb-core/deploy/.env.example)

---

## Documentation

| Document | Description |
|----------|-------------|
| [Production Deployment Guide](docs/production-deployment-guide.md) | Setup, TLS, scaling, monitoring |
| [Security Hardening Checklist](docs/security-hardening-checklist.md) | Pre-production security checklist |
| [Backup & Recovery](docs/backup-recovery.md) | Backup scripts, disaster recovery |
| [Edge Deployment Guide](docs/edge-deployment-guide.md) | Edge node setup and operations |
| [Private Registry Guide](docs/private-registry-guide.md) | Docker Registry for internal deployment |
| [Edge Sync RFC](docs/design/edge-offline-sync-rfc.md) | Architecture design document |
| [Page Audit Report](docs/page-audit-report.md) | Page-by-page functionality assessment |
| [CHANGELOG](CHANGELOG.md) | Version history |

---

## Scripts

| Script | Usage |
|--------|-------|
| `scripts/start-central.sh` | Start Central stack (Docker Compose) |
| `scripts/start-edge.sh` | Start Edge stack |
| `scripts/build-offline-package.sh` | Build offline installer |
| `scripts/push-to-registry.sh` | Push images to private registry |
| `scripts/setup-registry.sh` | Setup Docker Registry |
| `scripts/update.sh` | Update deployed instance |
| `scripts/chaos-test.sh` | Run chaos test (NATS disconnect) |

---

## Testing

```bash
# Backend unit tests
cd cmdb-core && go test ./... -count=1

# Stress test (needs running server)
TEST_API_TOKEN=<token> go test -tags integration ./tests/ -run TestSyncStress -short -v

# Chaos test (needs Docker Compose stack)
./scripts/chaos-test.sh --dry-run --rounds 3

# E2E tests (needs running frontend + backend)
cd cmdb-demo && npx playwright test
```

---

## License

Proprietary. All rights reserved.
