# Production Deployment Guide

This guide covers deploying CMDB Platform in Central (cloud) mode for production environments.
For Edge Node deployment, see [edge-deployment-guide.md](./edge-deployment-guide.md).

---

## Prerequisites

### Hardware Requirements

| Tier | CPU | RAM | Disk |
|------|-----|-----|------|
| Central (minimum) | 4 vCPU | 8 GB | 100 GB SSD |
| Central (recommended) | 8 vCPU | 16 GB | 500 GB SSD |
| Edge Node (minimum) | 2 vCPU | 2 GB | 20 GB |
| Edge Node (recommended) | 4 vCPU | 4 GB | 40 GB |

### Software Requirements

- Docker 24+ and Docker Compose v2
- Domain name with DNS A record pointing to the server
- TLS certificate (Let's Encrypt or organizational CA)
- Git (to clone the repository)

### Bundled Infrastructure

The following services are bundled in `docker-compose.yml` and do not require separate installation:

- PostgreSQL 17 with TimescaleDB (via `timescale/timescaledb:latest-pg17`)
- Redis 7.4
- NATS 2.10
- OpenTelemetry Collector, Prometheus, Loki, Jaeger, Grafana, Promtail

---

## Quick Start (Central)

### Step 1 — Clone the repository

```bash
git clone https://github.com/your-org/cmdb-platform.git
cd cmdb-platform/cmdb-core/deploy
```

### Step 2 — Create your environment file

```bash
cp .env.example .env
```

### Step 3 — Set all required environment variables

Open `.env` and set secure values for every variable. At minimum:

```bash
# Generate a strong JWT secret
JWT_SECRET=$(openssl rand -hex 32)

# Set a strong database password
DB_PASS=$(openssl rand -hex 16)

# Set deployment mode
DEPLOY_MODE=central

# Set Grafana admin password
GRAFANA_PASS=$(openssl rand -hex 12)

# Scale workers as needed
CORE_REPLICAS=2
WORKER_REPLICAS=4
```

The application refuses to start in `cloud` mode if `JWT_SECRET` or `DATABASE_URL` contain
the default `changeme` value. This is enforced at startup in `config.go`.

### Step 4 — Start the stack

```bash
docker compose up -d
```

This starts all services: cmdb-core, ingestion-engine, ingestion-worker, postgres, redis, nats,
nginx, otel-collector, jaeger, prometheus, loki, promtail, and grafana.

### Step 5 — Run database migrations

```bash
docker compose exec cmdb-core /app/cmdb-core migrate
```

If the migrate subcommand is unavailable, apply migrations via the API endpoint:

```bash
curl -X POST http://localhost:8080/api/v1/admin/migrate \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Step 6 — Seed initial data

```bash
docker compose exec cmdb-core /app/cmdb-core seed
```

This creates the default admin account and required reference data. Change the admin password
immediately after first login.

### Step 7 — Verify health

```bash
# Liveness probe
curl -s http://localhost:8080/healthz
# Expected: {"status":"ok"}

# Readiness probe (confirms DB + Redis connected)
curl -s http://localhost:8080/readyz
# Expected: {"status":"ok"}

# Ingestion engine
curl -s http://localhost:8081/healthz
# Expected: {"status":"ok"}
```

---

## Environment Variables Reference

All variables are read at startup by `cmdb-core/internal/config/config.go`.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PORT` | no | `8080` | HTTP server listen port |
| `DATABASE_URL` | yes | `postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable` | PostgreSQL connection string. Must not contain `changeme` in cloud mode. |
| `REDIS_URL` | yes | `redis://localhost:6379/0` | Redis connection string |
| `NATS_URL` | yes | `nats://localhost:4222` | NATS broker address |
| `JWT_SECRET` | yes | `dev-secret-change-me` | JWT signing secret. Must be changed from default in cloud mode. Recommend 64+ hex chars. |
| `DEPLOY_MODE` | yes | `cloud` | Deployment mode: `cloud` (Central) or `edge`. Cloud mode enforces credential validation. |
| `TENANT_ID` | edge only | — | Tenant UUID. Required when `DEPLOY_MODE=edge`. |
| `EDGE_NODE_ID` | edge only | — | Unique identifier for this Edge node (e.g. `edge-taipei-01`). |
| `SYNC_ENABLED` | no | `true` | Enable Edge sync subsystem. Set to `false` on Central to disable outbound sync. |
| `SYNC_SNAPSHOT_BATCH_SIZE` | no | `500` | Records per batch during initial Edge snapshot. Tune for network bandwidth. |
| `MCP_ENABLED` | no | `true` | Enable the Model Context Protocol server (AI integration). |
| `MCP_PORT` | no | `3001` | MCP server listen port. |
| `MCP_API_KEY` | no | — | API key for authenticating MCP clients. Strongly recommended in production. |
| `WS_ENABLED` | no | `true` | Enable WebSocket support for real-time updates. Disabled on Edge by default. |
| `OTEL_ENDPOINT` | no | — | OpenTelemetry Collector gRPC endpoint (e.g. `otel-collector:4317`). |
| `LOG_LEVEL` | no | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |
| `CARBON_EMISSION_FACTOR` | no | `0.0005` | kg CO₂ per watt-hour for carbon footprint calculations. |

### Ingestion Engine Variables

These apply to the `ingestion-engine` and `ingestion-worker` services:

| Variable | Description |
|----------|-------------|
| `INGESTION_DATABASE_URL` | PostgreSQL connection string for the ingestion engine |
| `INGESTION_REDIS_URL` | Redis connection string (db 0) |
| `INGESTION_CELERY_BROKER_URL` | Celery broker URL (Redis db 1 by default) |
| `INGESTION_NATS_URL` | NATS broker address |
| `INGESTION_OTEL_ENDPOINT` | OpenTelemetry Collector endpoint |

### Docker Compose Scale Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CORE_REPLICAS` | `1` | Number of cmdb-core replicas |
| `WORKER_REPLICAS` | `2` | Number of ingestion-worker Celery replicas |
| `DB_PASS` | `changeme` | PostgreSQL password (used in DATABASE_URL construction) |
| `GRAFANA_PASS` | `admin` | Grafana admin password |

---

## TLS / HTTPS Setup

The bundled Nginx container handles TLS termination. Update the nginx config to add HTTPS:

### Step 1 — Mount your certificate

Add certificate volumes to the nginx service in your `docker-compose.override.yml`:

```yaml
services:
  nginx:
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - /etc/letsencrypt:/etc/letsencrypt:ro
      - ../../cmdb-demo/dist:/usr/share/nginx/html:ro
```

### Step 2 — Update nginx.conf for HTTPS

```nginx
server {
    listen 80;
    server_name cmdb.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name cmdb.example.com;

    ssl_certificate     /etc/letsencrypt/live/cmdb.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cmdb.example.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    location /api/ {
        proxy_pass http://cmdb-core:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://cmdb-core:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;
    }
}
```

---

## Database Setup

### Connection Pooling

The bundled PostgreSQL is configured with `max_connections=200`. For production workloads
with many replicas, consider adding PgBouncer in front of PostgreSQL:

```yaml
# Add to docker-compose.override.yml
services:
  pgbouncer:
    image: pgbouncer/pgbouncer:1.23.1
    environment:
      DATABASES_HOST: postgres
      DATABASES_PORT: 5432
      DATABASES_USER: cmdb
      DATABASES_PASSWORD: ${DB_PASS}
      DATABASES_DBNAME: cmdb
      PGBOUNCER_POOL_MODE: transaction
      PGBOUNCER_MAX_CLIENT_CONN: 1000
      PGBOUNCER_DEFAULT_POOL_SIZE: 25
    ports:
      - "6432:5432"
```

Then update `DATABASE_URL` to point at PgBouncer (`postgres:6432` → `pgbouncer:5432`).

### TimescaleDB Hypertables

Metrics tables are already converted to TimescaleDB hypertables by the migrations. Verify:

```sql
SELECT hypertable_name, num_chunks
FROM timescaledb_information.hypertables;
```

### Retention Policies

Set data retention to control disk usage. Adjust intervals to your compliance requirements:

```sql
-- 30-day retention for raw metrics
SELECT add_retention_policy('asset_metrics', INTERVAL '30 days');

-- 180-day retention for hourly aggregates
SELECT add_retention_policy('asset_metrics_hourly', INTERVAL '180 days');

-- 730-day retention for daily aggregates
SELECT add_retention_policy('asset_metrics_daily', INTERVAL '730 days');
```

---

## Scaling

### Horizontal Scaling (Replicas)

Scale cmdb-core and ingestion-worker via environment variables:

```bash
# In .env
CORE_REPLICAS=4
WORKER_REPLICAS=8
```

Then apply:

```bash
docker compose up -d --scale cmdb-core=4 --scale ingestion-worker=8
```

Note: When running multiple cmdb-core replicas, ensure a load balancer (Nginx upstream or
external LB) distributes traffic across instances. WebSocket sessions are sticky by default
via Nginx `ip_hash`.

### Vertical Scaling (Resource Limits)

Add resource limits in `docker-compose.override.yml`:

```yaml
services:
  cmdb-core:
    deploy:
      resources:
        limits:
          cpus: "4"
          memory: 4G
        reservations:
          cpus: "1"
          memory: 1G
  postgres:
    deploy:
      resources:
        limits:
          cpus: "4"
          memory: 8G
```

---

## Monitoring

### Health Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/healthz` | GET | Liveness — confirms the process is running |
| `/readyz` | GET | Readiness — confirms DB and Redis are reachable |
| `/metrics` | GET | Prometheus metrics scrape endpoint |

### Prometheus Metrics

Prometheus scrapes `/metrics` on port 8080. Key metrics to watch:

| Metric | Alert Threshold |
|--------|----------------|
| `cmdb_api_request_duration_seconds` | p99 > 2s |
| `cmdb_db_query_duration_seconds` | p99 > 500ms |
| `cmdb_sync_lag_seconds` | > 300s (5 min) |
| `cmdb_ingestion_queue_depth` | > 10,000 |
| `process_resident_memory_bytes` | > 80% of limit |

### Grafana Dashboards

Access Grafana at `http://your-server:3000` (default credentials: `admin` / value of `GRAFANA_PASS`).

Pre-provisioned dashboards are located in `cmdb-core/deploy/grafana/provisioning/dashboards/`.
Datasources (Prometheus, Loki, Jaeger) are auto-configured at startup.

### Distributed Tracing

Jaeger UI is available at `http://your-server:16686`. Traces are collected via OpenTelemetry
and exported to Jaeger through the otel-collector. The `OTEL_ENDPOINT` variable must be set
to `otel-collector:4317` for the application to emit traces.

### Log Aggregation

Promtail ships Docker container logs to Loki. Query logs in Grafana using LogQL:

```logql
{container="cmdb-core"} |= "ERROR"
{container="ingestion-worker"} | json | level="error"
```

---

## Backup Strategy

See [backup-recovery.md](./backup-recovery.md) for the full backup and disaster recovery procedures.

Summary:

| Component | Method | Frequency |
|-----------|--------|-----------|
| PostgreSQL | `pg_dump` custom format | Daily |
| Redis | RDB snapshot (`dump.rdb`) | Hourly |
| NATS JetStream | Directory backup of `/data/jetstream` | Daily |
| Config (`.env`, overrides) | Version control | On change |

---

## Upgrade Process

### Standard Rolling Upgrade

```bash
cd cmdb-platform/cmdb-core/deploy

# 1. Pull updated images
docker compose pull

# 2. Apply new database migrations before restarting
docker compose exec cmdb-core /app/cmdb-core migrate

# 3. Restart services with zero-downtime rolling update
docker compose up -d --no-deps cmdb-core ingestion-engine ingestion-worker

# 4. Verify health after restart
curl -s http://localhost:8080/readyz
```

### Verifying a Successful Upgrade

```bash
# Confirm expected version is running
docker compose exec cmdb-core /app/cmdb-core version

# Check logs for startup errors
docker compose logs cmdb-core --since=5m | grep -E "ERROR|FATAL|started"

# Confirm metrics endpoint is responding
curl -s http://localhost:8080/metrics | grep cmdb_
```

### Rollback

If the new version is unhealthy, roll back by specifying the previous image tag in
`docker-compose.override.yml` and running `docker compose up -d`.

---

## Post-Deployment Security Checklist

See [security-hardening-checklist.md](./security-hardening-checklist.md) for the full checklist.
Critical items to verify before going live:

- [ ] `JWT_SECRET` is a 64+ character random string (not the default)
- [ ] `DB_PASS` is set to a unique, strong password (not `changeme`)
- [ ] Default admin password changed after first login
- [ ] HTTPS enabled — no plain HTTP exposed externally
- [ ] Firewall rules restrict database (5432), Redis (6379), and NATS (4222) to internal network only
- [ ] `MCP_API_KEY` set if MCP server is enabled
