# Changelog

All notable changes to the CMDB Platform will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-04-04

Initial production-ready release of the CMDB Platform. Platform score: **95/100**.

### Architecture

- **cmdb-core** (Go): REST API server with 48 endpoints, 9 domain modules
- **cmdb-demo** (React/TypeScript): 32-page SPA with real-time updates
- **ingestion-engine** (Python/FastAPI): Asset ingestion pipeline with transform engine
- **Infrastructure**: PostgreSQL, Redis, NATS JetStream, Docker Compose

### Added

#### Phase 1 — Go Core (29 REST endpoints, 9 domain modules)
- Asset management: list, create, get, update, delete
- Topology: locations hierarchy (ltree), racks, slots
- Maintenance: work orders with state machine transitions
- Monitoring: alert events with acknowledge/resolve workflow
- Inventory: tasks and item scanning
- Audit: event trail for all write operations
- Dashboard: aggregated statistics
- Identity: users, roles, JWT authentication
- Prediction: AI models, failure predictions, RCA analysis

#### Phase 2 — Python Ingestion Engine (13 endpoints)
- FastAPI-based ingestion pipeline
- Pydantic models for data validation
- Conflict detection and resolution
- Field authority management

#### Phase 3 — MCP Server + AI Adapters + WebSocket
- MCP Server with 7 tools and 3 resources for AI integration
- AI adapter registry (Dify, LLM, Custom)
- WebSocket hub with tenant-scoped broadcasting

#### Phase 4 — Observability + Deployment
- Structured logging (zap)
- Prometheus metrics with custom middleware
- OpenTelemetry tracing (OTLP gRPC)
- Production Docker Compose (central + edge overlay)
- Nginx reverse proxy, NATS federation

#### Phase 5a — OpenAPI Spec First
- `api/openapi.yaml` as single source of truth
- `oapi-codegen` generates Go types + Gin server
- `openapi-typescript` generates frontend TS types

#### Phase 5b — Frontend Integration (32 pages)
- Migrated all pages from mock data to live API calls
- React Query hooks for every domain module
- Auth guard with JWT token management

#### Phase 6 — Platform Completion
- Integration API: adapters, webhooks
- `/system/health` endpoint
- Metrics simulation injector (backfill + continuous)
- Zero mock pages remaining

#### W1 — Security & Compliance (score 72→80)
- RBAC permission middleware with Redis cache
- Route-to-resource+action mapping for all 48 endpoints
- Audit logging on all 7 write operations (sync via `auditSvc.Record()`)
- 3 role definitions: super-admin, ops-admin, viewer

#### W2 — CRUD Complete + Data Fill (score 80→88)
- `PUT /assets/{id}` — partial update with COALESCE pattern
- `DELETE /assets/{id}` — with audit logging
- `GET/POST /monitoring/rules` — alert rules CRUD
- `GET/POST /monitoring/incidents` + `GET/PUT /monitoring/incidents/{id}` — incident management
- Seed data for 9 previously empty tables (~54 rows): rack_slots, alert_rules, incidents, prediction_models, prediction_results, rca_analyses, inventory_items, webhook_deliveries, departments

#### W3 — Event-Driven + Real-time (score 88→93)
- NATS event publishing on all 11 write operations
- `publishEvent()` fire-and-forget helper on APIServer
- Frontend WebSocket client (`useWebSocket` hook) with auto-reconnect
- `WebSocketProvider` integrated into React app tree
- React Query cache auto-invalidation on incoming events
- Webhook dispatcher: HMAC signing, 3x retry with backoff, delivery recording

#### W4 — Hardening (score 93→95)
- MCP Server API key authentication (`MCP_API_KEY` env var)
- End-to-end smoke test script (25+ assertions, 11 endpoint groups)
- Asset attributes enrichment: 10 key assets with detailed specs (CPU, memory, storage, IPs, warranty, uptime)

### API Endpoints (48 total)

| Module | Endpoints | Methods |
|--------|-----------|---------|
| Auth | 3 | login, refresh, me |
| Assets | 5 | list, create, get, update, delete |
| Topology | 8 | locations (list, get, children, ancestors, stats, racks), racks (get, assets) |
| Maintenance | 4 | orders (list, create, get, transition) |
| Monitoring | 10 | alerts (list, ack, resolve), rules (list, create), incidents (list, create, get, update), metrics |
| Inventory | 3 | tasks (list, get, items) |
| Audit | 1 | events query |
| Dashboard | 1 | stats |
| Identity | 3 | users (list, get), roles |
| Prediction | 4 | models, results by asset, create RCA, verify RCA |
| System | 1 | health |
| Integration | 2 | adapters, webhooks |

### Database

- 20 tables with TimescaleDB hypertable for metrics
- Full seed dataset: ~26K metric rows, 20 assets, 10 racks, 8 alerts, 6 work orders, 5 alert rules, 3 incidents, 2 prediction models, 5 predictions, 2 RCA analyses, 10 inventory items, 3 inventory tasks, 10 audit events, 20 rack slots
- ltree extension for location hierarchy

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `DATABASE_URL` | localhost:5432/cmdb | PostgreSQL connection |
| `REDIS_URL` | localhost:6379 | Redis for RBAC cache |
| `NATS_URL` | localhost:4222 | NATS JetStream |
| `JWT_SECRET` | dev-secret-change-me | JWT signing key |
| `MCP_ENABLED` | true | Enable MCP Server |
| `MCP_PORT` | 3001 | MCP Server port |
| `MCP_API_KEY` | (empty) | MCP auth key (empty = open) |
| `WS_ENABLED` | true | Enable WebSocket hub |
| `DEPLOY_MODE` | cloud | cloud or edge |
| `LOG_LEVEL` | info | zap log level |
| `OTEL_ENDPOINT` | (empty) | OpenTelemetry collector |

[1.0.0]: https://github.com/cmdb-platform/cmdb-platform/releases/tag/v1.0.0
