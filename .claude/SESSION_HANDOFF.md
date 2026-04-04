# Session Handoff — CMDB Platform

## Current State (2026-04-04)

**Platform Score: 80/100**

### Completed Phases

| Phase | Content | Status |
|-------|---------|--------|
| Phase 1 | Go core: 29 REST endpoints, 9 domain modules | Done |
| Phase 2 | Python ingestion engine: 13 endpoints, transform pipeline | Done |
| Phase 3 | MCP Server (7 tools), AI adapters (Dify/LLM/Custom), WebSocket | Done |
| Phase 4 | Observability (zap/Prometheus/OTel), Nginx, NATS federation, Docker Compose | Done |
| Phase 5a | OpenAPI Spec First: openapi.yaml → oapi-codegen → generated types + impl.go | Done |
| Phase 5b | Frontend integration: 32 pages mock→API, generated TS types | Done |
| Phase 6 | Platform completion: Integration API, /system/health, metrics pipeline | Done |
| **W1** | **RBAC middleware + audit logging for all write ops** | **Done** |

### What's Next: W2-W4

Roadmap: `docs/platform-completion-roadmap.md`

**W2 (score 80→88): CRUD Complete + Data Fill**
- Asset PUT/DELETE endpoints
- Alert Rules CRUD (2 endpoints)
- Incidents CRUD (4 endpoints)
- 12 empty tables filled with seed data (rack_slots, prediction_models, inventory_items, etc.)

**W3 (score 88→93): Event-Driven + WebSocket**
- Write ops publish NATS events
- Frontend WebSocket client (auto-invalidate React Query cache)
- Webhook dispatcher (HTTP delivery on events)

**W4 (score 93→95): Hardening**
- Smoke test script
- MCP authentication
- AssetDetail attributes enrichment

### Key Architecture

```
api/openapi.yaml              ← Single source of truth (37 endpoints)
  ↓ make generate
cmdb-core/internal/api/
  generated.go                ← oapi-codegen: ServerInterface + types
  impl.go                     ← Hand-written: implements all endpoints
  convert.go                  ← dbgen → API type conversion (14 converters)

cmdb-demo/src/generated/
  api-types.ts                ← openapi-typescript: all TS types
```

### Running Services

```bash
# Infrastructure (Docker)
cd cmdb-core/deploy && docker compose up -d postgres redis nats

# Backend
cd cmdb-core
DATABASE_URL="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" \
REDIS_URL="redis://localhost:6379/0" \
NATS_URL="nats://localhost:4222" \
JWT_SECRET="dev-secret-change-me" \
MCP_ENABLED="false" \
go run ./cmd/server

# Frontend
cd cmdb-demo && npm run dev

# Login: admin / admin123
```

### Key Files for W2

```
cmdb-core/
├── api/openapi.yaml               ← Add new endpoints here
├── internal/api/impl.go           ← Implement new endpoints here
├── internal/api/convert.go        ← Add converters for new types
├── db/queries/*.sql               ← Add sqlc queries
├── db/seed/seed.sql               ← Add seed data for empty tables
└── internal/domain/*/service.go   ← Add service methods

cmdb-demo/
├── src/lib/api/*.ts               ← Frontend API modules (already have dead code for missing endpoints)
├── src/hooks/*.ts                  ← React Query hooks
└── src/generated/api-types.ts     ← Regenerate after openapi.yaml changes
```

### DB Tables Status

Seeded (12): tenants, users, roles, user_roles, locations, racks, assets, alert_events, work_orders, work_order_logs, inventory_tasks, audit_events, integration_adapters, webhook_subscriptions

Empty (12): departments, rack_slots, alert_rules, incidents, prediction_models, prediction_results, rca_analyses, inventory_items, import_conflicts, discovery_tasks, discovery_candidates, import_jobs

Metrics: ~26K rows from inject-metrics.py (cpu_usage, temperature, power_kw, memory_usage, pue)

### Git Branch

Working branch: `feat/cmdb-core-phase1` (all commits here)
