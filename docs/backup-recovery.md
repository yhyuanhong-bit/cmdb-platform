# Backup & Recovery Procedures

This document defines backup schedules, procedures, and disaster recovery playbooks
for all CMDB Platform components. Follow these procedures to meet the Recovery Time
Objectives (RTOs) and Recovery Point Objectives (RPOs) defined at the end of this document.

---

## What to Back Up

| Component | Data Stored | Backup Method | Frequency |
|-----------|------------|---------------|-----------|
| PostgreSQL | Assets, work orders, alerts, users, sync state, TimescaleDB metrics | `pg_dump` custom format | Daily |
| Redis | Session tokens, permission cache, Celery task queue | RDB snapshot (`dump.rdb`) | Hourly |
| NATS JetStream | Sync queue messages, stream metadata, consumer state | Directory backup of `/data/jetstream` | Daily |
| Application config | `.env`, `docker-compose.override.yml`, nginx/NATS configs | Version control | On change |
| TLS certificates | Certificate and private key | Secure offsite copy | On renewal |

---

## PostgreSQL Backup

PostgreSQL is the system of record (SSOT) for all CMDB data. It must be backed up
reliably. Losing this data without a backup constitutes a full disaster scenario.

### Automated Daily Backup Script

Save this script to `/usr/local/bin/cmdb-backup-postgres.sh` on the host and make it executable.

```bash
#!/bin/bash
set -euo pipefail

BACKUP_DIR=/backups/postgres
RETENTION_DAYS=30
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/cmdb_${DATE}.dump"

# DATABASE_URL must be set in environment or passed explicitly
: "${DATABASE_URL:?DATABASE_URL is required}"

mkdir -p "$BACKUP_DIR"

echo "[$(date -u +%FT%TZ)] Starting PostgreSQL backup → $BACKUP_FILE"

pg_dump \
  --format=custom \
  --compress=9 \
  --no-password \
  "$DATABASE_URL" \
  > "$BACKUP_FILE"

echo "[$(date -u +%FT%TZ)] Backup complete. Size: $(du -sh "$BACKUP_FILE" | cut -f1)"

# Verify the dump is readable
pg_restore --list "$BACKUP_FILE" > /dev/null \
  && echo "[$(date -u +%FT%TZ)] Backup verified OK" \
  || { echo "[$(date -u +%FT%TZ)] ERROR: Backup verification failed"; exit 1; }

# Purge backups older than retention window
find "$BACKUP_DIR" -name "*.dump" -mtime +"$RETENTION_DAYS" -delete
echo "[$(date -u +%FT%TZ)] Purged backups older than ${RETENTION_DAYS} days"
```

### Schedule via cron

```bash
# Run daily at 02:00 UTC
0 2 * * * DATABASE_URL="postgres://cmdb:PASSWORD@localhost:5432/cmdb" /usr/local/bin/cmdb-backup-postgres.sh >> /var/log/cmdb-backup.log 2>&1
```

### Running Backup from Inside Docker

If PostgreSQL is only accessible inside the Docker network:

```bash
docker compose exec postgres pg_dump \
  -U cmdb \
  --format=custom \
  --compress=9 \
  cmdb \
  > /backups/postgres/cmdb_$(date +%Y%m%d_%H%M%S).dump
```

### Restore from Backup

```bash
# Full restore (drops and recreates all objects)
pg_restore \
  --dbname="$DATABASE_URL" \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges \
  /backups/postgres/cmdb_20260413_020000.dump

# Restore specific table only
pg_restore \
  --dbname="$DATABASE_URL" \
  --table=assets \
  /backups/postgres/cmdb_20260413_020000.dump
```

After restoring, run any pending migrations to bring the schema up to date:

```bash
docker compose exec cmdb-core /app/cmdb-core migrate
```

### TimescaleDB Considerations

TimescaleDB hypertables (metrics tables) are included in `pg_dump` output by default.
The restore process recreates them correctly. Retention policies are schema objects and
are also included in the dump. Verify after restore:

```sql
SELECT hypertable_name, num_chunks
FROM timescaledb_information.hypertables;

SELECT * FROM timescaledb_information.jobs
WHERE proc_name = 'policy_retention';
```

---

## Redis Backup

Redis stores session tokens and the Celery task queue. Data in Redis is transient by design —
loss of Redis data forces users to re-authenticate but does not cause data loss in the database.

### Automatic RDB Snapshots

The bundled Redis is configured with LRU eviction (`allkeys-lru`). For persistent snapshots,
add save directives to the Redis command in `docker-compose.override.yml`:

```yaml
services:
  redis:
    command: >
      redis-server
      --maxmemory 256mb
      --maxmemory-policy allkeys-lru
      --save 3600 1
      --save 300 100
      --save 60 10000
      --dir /data
      --dbfilename dump.rdb
```

This writes `/data/dump.rdb` inside the container, which is persisted to the `redis_data`
Docker volume.

### Copy RDB Snapshot

```bash
# Copy the RDB file from the running container
docker compose exec redis redis-cli BGSAVE
# Wait for background save to complete
docker compose exec redis redis-cli LASTSAVE

# Copy the snapshot file out
docker cp $(docker compose ps -q redis):/data/dump.rdb \
  /backups/redis/dump_$(date +%Y%m%d_%H%M%S).rdb
```

### Restore Redis

```bash
# Stop Redis
docker compose stop redis

# Copy backup into the volume (via a helper container)
docker run --rm \
  -v cmdb_redis_data:/data \
  -v /backups/redis:/backups \
  alpine cp /backups/dump_20260413_020000.rdb /data/dump.rdb

# Restart Redis
docker compose start redis
```

---

## NATS JetStream Backup

NATS JetStream persists sync queue messages to disk. Loss of JetStream data may cause
Edge nodes to miss incremental sync events, but Edge nodes will automatically request
a fresh snapshot from Central on reconnection. NATS data loss is low severity compared
to PostgreSQL.

### Backup JetStream Data Directory

```bash
# The nats_data Docker volume maps to /data inside the container
docker compose stop nats

docker run --rm \
  -v cmdb_nats_data:/data \
  -v /backups/nats:/backups \
  alpine tar -czf /backups/nats_$(date +%Y%m%d_%H%M%S).tar.gz -C /data .

docker compose start nats
```

### Restore JetStream Data

```bash
docker compose stop nats

docker run --rm \
  -v cmdb_nats_data:/data \
  -v /backups/nats:/backups \
  alpine sh -c "rm -rf /data/jetstream && tar -xzf /backups/nats_20260413_020000.tar.gz -C /data"

docker compose start nats
```

Streams that existed in config but not in the restored data are automatically recreated by
NATS on startup. Consumer positions may be reset, but Edge nodes will reconcile via
the sync subsystem.

---

## Application Configuration Backup

`.env`, `docker-compose.override.yml`, and nginx/NATS configuration files should be
version-controlled (in a private repository) or stored in a configuration management system.

```bash
# Never commit .env to a public repository
# Store in a private repo, encrypted secrets store, or vault

# Example: back up configs to a secure location
tar -czf /backups/config/cmdb_config_$(date +%Y%m%d).tar.gz \
  .env \
  docker-compose.override.yml \
  cmdb-core/deploy/nginx/nginx.conf \
  cmdb-core/deploy/nats/nats-central.conf
```

Store this archive in an offsite location (S3, encrypted backup service) separate from
the database backups.

---

## Disaster Recovery Playbooks

### Scenario 1: Single Service Crash

**Symptoms:** One container exits unexpectedly; other services remain healthy.

**Resolution:**

```bash
# Identify the failed container
docker compose ps

# Inspect its exit reason
docker compose logs <service-name> --tail=50

# Restart the service
docker compose restart <service-name>

# Verify it recovers
docker compose ps <service-name>
curl -s http://localhost:8080/healthz
```

All services are configured with `restart: unless-stopped`. Transient crashes are handled
automatically. Investigate root cause if a service restarts more than 3 times in an hour.

---

### Scenario 2: Central PostgreSQL Lost

**Symptoms:** cmdb-core fails to start or returns 500 errors; logs show database connection failures.

**Impact:** All API operations unavailable. Edge nodes enter offline mode and continue serving
cached data locally. No data is written during the outage.

**Resolution:**

```bash
# 1. Stop dependent services to prevent split-brain writes
docker compose stop cmdb-core ingestion-engine ingestion-worker

# 2. Verify PostgreSQL is down
docker compose ps postgres

# 3. If the volume is intact, restart PostgreSQL
docker compose start postgres
docker compose logs postgres --tail=20

# 4. If the volume is corrupt or lost, restore from backup
docker compose stop postgres

# Restore the volume from backup dump
docker compose run --rm -e "DATABASE_URL=$DATABASE_URL" postgres \
  pg_restore --dbname="$DATABASE_URL" --clean --if-exists /backups/cmdb_latest.dump

# 5. Start all services
docker compose start postgres cmdb-core ingestion-engine ingestion-worker

# 6. Run pending migrations
docker compose exec cmdb-core /app/cmdb-core migrate

# 7. Verify health
curl -s http://localhost:8080/readyz

# 8. Edge nodes will auto-resync — no manual action required
# Monitor sync progress:
docker compose logs cmdb-core | grep -i "sync"
```

**Edge node behavior during Central outage:** Edge nodes serve read traffic from local
PostgreSQL. Write operations are blocked. Once Central is restored and sync resumes,
any pending sync events are replayed via NATS JetStream.

---

### Scenario 3: Edge Node Lost

**Symptoms:** An Edge node is unresponsive or its data is corrupt.

**Impact:** Users at the affected edge site lose access. Central and other Edge nodes
are unaffected.

**Resolution:**

```bash
# Option A: Restart existing Edge node
docker compose restart

# Option B: Re-initialize Edge node from scratch
# See edge-deployment-guide.md → Re-initialize Edge (Nuclear Option)

# The Edge node requests a fresh snapshot from Central on boot with empty sync_state
# Initial sync completes in 20–60 seconds depending on data volume
curl -s http://edge-host:8080/readyz
# Wait for {"status":"ok"}
```

All Edge data is derived from Central. There is no unique data on an Edge node that
requires backing up. The Edge node is disposable and self-healing.

---

### Scenario 4: NATS JetStream Data Lost

**Symptoms:** Sync stops advancing; Edge nodes show stale `last_sync_at`; NATS logs
show stream or consumer errors.

**Impact:** Incremental sync events may be lost. Edge nodes will not receive recent changes
until the issue is resolved.

**Resolution:**

```bash
# 1. Restart NATS — streams are recreated from configuration
docker compose restart nats
docker compose logs nats --tail=30

# 2. Edge nodes reconnect automatically via NATS leafnode
# Monitor reconnection:
docker compose logs cmdb-core | grep -i "nats\|leafnode\|reconnect"

# 3. The sync reconciliation service detects version gaps within 5 minutes
# and triggers targeted re-sync for affected entity types

# 4. Verify sync is advancing:
docker compose exec postgres psql -U cmdb -c \
  "SELECT entity_type, last_sync_version, last_sync_at FROM sync_state ORDER BY last_sync_at DESC;"

# 5. If sync does not advance within 10 minutes, trigger a manual re-sync:
curl -X POST http://edge-host:8080/api/v1/sync/request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"entity_type": "assets", "since_version": 0}'
```

---

### Scenario 5: Complete Disaster (Host Lost)

**Symptoms:** The entire Central host is lost — hardware failure, accidental deletion,
or catastrophic event.

**Impact:** All API access unavailable. Edge nodes enter offline mode. No data is written.

**Resolution:**

```bash
# 1. Provision a new host meeting the hardware requirements in the deployment guide

# 2. Install Docker and Docker Compose
apt-get update && apt-get install -y docker.io docker-compose-plugin

# 3. Clone the repository
git clone https://github.com/your-org/cmdb-platform.git
cd cmdb-platform/cmdb-core/deploy

# 4. Restore application configuration
# Copy .env and docker-compose.override.yml from your config backup

# 5. Start infrastructure services first
docker compose up -d postgres redis nats

# 6. Restore PostgreSQL from the latest daily backup
cat /backups/postgres/cmdb_latest.dump | docker compose exec -T postgres \
  pg_restore -U cmdb -d cmdb --clean --if-exists --no-owner

# 7. Start the full stack
docker compose up -d

# 8. Run pending migrations
docker compose exec cmdb-core /app/cmdb-core migrate

# 9. Restore TLS certificates and update DNS if the IP address changed

# 10. Verify health
curl -s https://cmdb.example.com/healthz
curl -s https://cmdb.example.com/readyz

# 11. Edge nodes will detect the Central NATS server is back and reconnect automatically
# Update CENTRAL_NATS_URL on Edge nodes if the IP or hostname changed
```

---

## Recovery Time Objectives

| Scenario | RTO | RPO | Notes |
|----------|-----|-----|-------|
| Single service restart | < 1 minute | 0 | Automatic via `restart: unless-stopped` |
| PostgreSQL restart (volume intact) | < 5 minutes | 0 | No data loss |
| PostgreSQL restore from backup | < 15 minutes | < 24 hours | Depends on backup age and data volume |
| Edge node re-initialization | < 5 minutes | 0 | Central is SSOT; Edge is fully self-healing |
| NATS data lost | < 10 minutes | < 5 minutes | Reconciliation detects and fills gaps |
| Full disaster recovery | < 1 hour | < 24 hours | Depends on backup retrieval time |

---

## Backup Testing

Untested backups are not backups. Schedule the following tests:

### Monthly: Restore Verification

```bash
# 1. Provision a test environment (separate host or namespace)
# 2. Restore the latest PostgreSQL backup
pg_restore \
  --dbname="postgres://cmdb:PASSWORD@test-host:5432/cmdb_test" \
  --clean --if-exists \
  /backups/postgres/cmdb_latest.dump

# 3. Verify data integrity
psql "postgres://cmdb:PASSWORD@test-host:5432/cmdb_test" << 'SQL'
  SELECT 'assets'      AS tbl, COUNT(*) FROM assets
  UNION ALL
  SELECT 'work_orders',         COUNT(*) FROM work_orders
  UNION ALL
  SELECT 'users',               COUNT(*) FROM users
  UNION ALL
  SELECT 'maintenance_tasks',   COUNT(*) FROM maintenance_tasks;
SQL

# 4. Compare row counts against known baseline
# Document the test date and row counts in your runbook
```

### Quarterly: Full Disaster Recovery Drill

Provision a complete new environment, restore from backup, and verify that the platform
reaches a healthy state end-to-end including Edge node reconnection. Document completion
time against the RTO targets above.

---

## Offsite Storage

Store backup archives in a geographically separate location from the primary host:

- Object storage: AWS S3, Google Cloud Storage, Cloudflare R2, or equivalent
- Retention in object storage: 90 days minimum; 1 year for compliance use cases
- Encryption: Enable server-side encryption on the bucket; consider client-side encryption
  for sensitive environments

Example: sync backups to S3 after each run:

```bash
aws s3 cp /backups/postgres/cmdb_${DATE}.dump \
  s3://your-org-cmdb-backups/postgres/cmdb_${DATE}.dump \
  --storage-class STANDARD_IA
```
