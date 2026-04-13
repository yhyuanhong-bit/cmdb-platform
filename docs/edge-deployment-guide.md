# Edge Node Deployment Guide

This guide covers deploying a CMDB Edge Node — a local instance that syncs with Central
and serves the full CMDB UI while operating independently of WAN connectivity.

---

## Prerequisites

- Docker 24+ and Docker Compose v2
- Network access to Central NATS server (port 7422 — leafnode protocol)
- Assigned `TENANT_ID` and `EDGE_NODE_ID` from your Central administrator
- PostgreSQL client (`psql`) if you intend to run chaos tests or manual diagnostics

---

## Quick Start (5 minutes)

### Step 1 — Clone the repository

```bash
git clone https://github.com/your-org/cmdb-platform.git
cd cmdb-platform
```

### Step 2 — Create your environment file

```bash
cp .env.edge.example .env
```

### Step 3 — Configure the required variables

Open `.env` and set the following minimum values:

```bash
DEPLOY_MODE=edge
TENANT_ID=<uuid-from-central-admin>
EDGE_NODE_ID=<unique-node-name>          # e.g. "edge-taipei-01"
CENTRAL_NATS_URL=nats://central.example.com:7422
```

Leave all other variables at their defaults unless your environment requires overrides.

### Step 4 — Start the Edge stack

```bash
./scripts/start-edge.sh
```

This command starts the local PostgreSQL, NATS (leafnode), Redis, and cmdb-core containers.
The first boot triggers an automatic snapshot download from Central.

### Step 5 — Verify readiness

```bash
curl -s http://localhost:8080/readyz
# Expected: {"status":"ok"}
```

During initial sync (typically 20–60 seconds), the API returns:

```json
{"error": {"code": "SYNC_IN_PROGRESS", "message": "Edge node is performing initial sync. Please wait."}}
```

with HTTP 503 and a `Retry-After: 30` header. The frontend overlay handles this automatically.

---

## How Initial Sync Works

When cmdb-core boots in Edge mode with an empty local database, the following sequence runs:

1. `sync/agent.go` starts and reads `sync_state` — finds no rows.
2. Agent sends a `SNAPSHOT_REQUEST` message to Central via NATS leafnode.
3. Central streams all entity snapshots in batches of `SYNC_SNAPSHOT_BATCH_SIZE` records.
4. Edge applies each batch via upsert, updating `sync_state.last_sync_version` per entity type.
5. Once all entity types reach parity, `InitialSyncDone` is set to `true`.
6. The `SyncGateMiddleware` stops blocking — all API endpoints become available.

Subsequent boots with an existing `sync_state` skip the snapshot and begin incremental sync immediately.

---

## Configuration Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DEPLOY_MODE` | yes | `cloud` | Must be set to `edge` for Edge mode |
| `TENANT_ID` | yes (edge) | — | Tenant UUID assigned by Central admin |
| `EDGE_NODE_ID` | yes (edge) | — | Unique identifier for this Edge node |
| `NATS_URL` | yes | `nats://localhost:4222` | Local NATS broker address |
| `CENTRAL_NATS_URL` | yes (edge) | — | Central NATS leafnode address (port 7422) |
| `DATABASE_URL` | yes | `postgres://cmdb:changeme@localhost:5432/cmdb` | Local PostgreSQL connection string |
| `REDIS_URL` | yes | `redis://localhost:6379` | Local Redis address |
| `SYNC_SNAPSHOT_BATCH_SIZE` | no | `100` | Records per batch during initial snapshot |

---

## Architecture Overview

```
                    Central Site
  ┌──────────────────────────────────────────┐
  │  cmdb-core (cloud)                       │
  │  ┌─────────────┐   ┌──────────────────┐  │
  │  │  PostgreSQL  │   │  NATS (port 4222) │  │
  │  │  (primary)  │   │  + Leafnode :7422 │  │
  │  └─────────────┘   └────────┬─────────┘  │
  └───────────────────────────┼─────────────┘
                               │  NATS leafnode
                               │  (WAN, port 7422)
  ┌───────────────────────────┼─────────────┐
  │          Edge Site        │             │
  │  ┌─────────────┐   ┌──────┴──────────┐  │
  │  │  PostgreSQL  │   │  NATS (leafnode) │  │
  │  │  (local)    │   │  port 4222      │  │
  │  └──────┬──────┘   └─────────────────┘  │
  │         │                               │
  │  ┌──────┴──────────────────────────┐    │
  │  │  cmdb-core (edge)               │    │
  │  │  SyncGateMiddleware + Agent     │    │
  │  └─────────────────────────────────┘    │
  │                                         │
  │  ┌─────────────────────────────────┐    │
  │  │  cmdb-demo (frontend)           │    │
  │  │  SyncingOverlay (503 handler)   │    │
  │  └─────────────────────────────────┘    │
  └─────────────────────────────────────────┘
```

---

## Sync Entity Types

| Entity Type | Sync Direction | Strategy |
|---|---|---|
| `assets` | Central → Edge | Snapshot + incremental |
| `work_orders` | Central → Edge | Snapshot + incremental |
| `alert_events` | Central → Edge | Incremental only |
| `inventory_tasks` | Central → Edge | Snapshot + incremental |
| `inventory_items` | Central → Edge | Snapshot + incremental |
| `maintenance_tasks` | Central → Edge | Snapshot + incremental |
| `change_requests` | Central → Edge | Snapshot + incremental |
| `tenants` | Central → Edge | Snapshot only |
| `users` | Central → Edge | Snapshot only |

All entity types use `sync_version` (monotonically increasing integer) as the cursor.
Conflict resolution uses last-write-wins based on `sync_version`.

---

## Operations Checklist

### Daily

- [ ] Confirm Edge node appears healthy in Central: `/system/sync` page
- [ ] Check `sync_state.last_sync_at` is within the last 5 minutes:

  ```sql
  SELECT entity_type, last_sync_version, last_sync_at
  FROM sync_state
  ORDER BY last_sync_at DESC;
  ```

- [ ] Check for sync errors:

  ```bash
  docker compose logs cmdb-core --since=24h | grep -i error
  ```

### Weekly

- [ ] Review full error log for recurring patterns:

  ```bash
  docker compose logs cmdb-core | grep ERROR | sort | uniq -c | sort -rn | head -20
  ```

- [ ] Check disk usage:

  ```bash
  docker system df
  ```

- [ ] Review sync conflict history:

  ```sql
  SELECT entity_type, COUNT(*) AS conflicts
  FROM sync_conflicts
  WHERE created_at > NOW() - INTERVAL '7 days'
  GROUP BY entity_type;
  ```

### Monthly

- [ ] Pull and apply updated container images:

  ```bash
  docker compose pull && docker compose up -d
  ```

- [ ] Review and rotate credentials if your security policy requires it
- [ ] Archive or purge resolved sync conflicts older than 90 days

---

## Troubleshooting

### NATS Connection Failed

**Symptom:** Logs show `NATS not available` or `failed to connect to leafnode`

**Diagnosis:**

```bash
docker compose logs nats | tail -50
```

**Fix:**

1. Verify `CENTRAL_NATS_URL` is correct and the hostname resolves.
2. Test connectivity from the Edge host:

   ```bash
   nc -zv central.example.com 7422
   ```

3. Check firewall rules — port 7422 (TCP) must be open outbound from Edge to Central.
4. Restart the local NATS container:

   ```bash
   docker compose restart nats
   ```

---

### Sync Stuck

**Symptom:** `sync_state.last_sync_at` is not updating; frontend shows stale data.

**Diagnosis:**

```bash
# Check NATS leafnode status
docker compose exec nats nats server report jetstream
```

```sql
-- Check last activity per entity type
SELECT entity_type, last_sync_version, last_sync_at,
       NOW() - last_sync_at AS age
FROM sync_state
ORDER BY age DESC;
```

**Fix:**

1. Restart the cmdb-core container — the agent will reconnect and resume:

   ```bash
   docker compose restart cmdb-core
   ```

2. If sync resumes but version does not advance, verify Central is publishing changes:

   ```bash
   # On Central host
   docker compose logs cmdb-core | grep "published sync envelope"
   ```

---

### Data Inconsistency

**Symptom:** Central and Edge show different records for the same entity.

**Diagnosis:**

```sql
-- On Edge: check for unresolved conflicts
SELECT * FROM sync_conflicts
WHERE resolved_at IS NULL
ORDER BY created_at DESC
LIMIT 20;
```

**Fix:**

1. Review conflicts and resolve manually if needed (last-write-wins is automatic, but can be overridden).
2. Trigger a targeted re-sync for the affected entity type via the API:

   ```bash
   curl -X POST http://localhost:8080/api/v1/sync/request \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"entity_type": "assets", "since_version": 0}'
   ```

---

### Re-initialize Edge (Nuclear Option)

Use this only when the Edge state is corrupt and you need a full re-sync from Central.

**Warning:** This discards all local sync progress. The Edge will be offline during re-sync (~30–60 seconds).

```bash
# 1. Stop cmdb-core (keep NATS and Postgres running)
docker compose stop cmdb-core

# 2. Drop sync state for this node
psql "$DATABASE_URL" -c "DELETE FROM sync_state;"
psql "$DATABASE_URL" -c "TRUNCATE sync_conflicts;"

# 3. Restart — agent detects empty sync_state and requests fresh snapshot
docker compose start cmdb-core

# 4. Watch logs for snapshot completion
docker compose logs -f cmdb-core | grep -E "snapshot|sync_state|InitialSyncDone"
```

---

## Resource Requirements

| Component | Minimum | Recommended |
|---|---|---|
| CPU | 2 vCPU | 4 vCPU |
| RAM | 2 GB | 4 GB |
| Disk (OS + Docker) | 20 GB | 40 GB |
| Disk (PostgreSQL data) | 10 GB | 50 GB |
| Network to Central | 1 Mbps sustained | 10 Mbps |
| PostgreSQL version | 14 | 16 |
| Docker Engine | 24.0 | latest stable |

---

## Support

For Central administrator contact, sync conflict escalation, or EDGE_NODE_ID provisioning,
refer to your organization's internal CMDB runbook or open a ticket in your service desk.
