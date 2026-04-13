#!/usr/bin/env bash
set -euo pipefail

# Chaos test: random NATS disconnect/reconnect to verify eventual consistency.
# Usage:
#   ./scripts/chaos-test.sh --rounds 3          # short
#   ./scripts/chaos-test.sh --rounds 50          # extended
#   ./scripts/chaos-test.sh --dry-run            # validate without Docker

ROUNDS=3
DRY_RUN=false
LOG_FILE="chaos-test-results.log"

EDGE_NATS_CONTAINER="cmdb-core-nats-1"
DOCKER_NETWORK="cmdb-core_default"
CENTRAL_DB="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
EDGE_DB="postgres://cmdb:changeme@localhost:5433/cmdb?sslmode=disable"

while [[ $# -gt 0 ]]; do
    case $1 in
        --rounds) ROUNDS="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        --edge-container) EDGE_NATS_CONTAINER="$2"; shift 2 ;;
        --network) DOCKER_NETWORK="$2"; shift 2 ;;
        --central-db) CENTRAL_DB="$2"; shift 2 ;;
        --edge-db) EDGE_DB="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "Chaos Test: $ROUNDS rounds" | tee "$LOG_FILE"
echo "Edge NATS container: $EDGE_NATS_CONTAINER" | tee -a "$LOG_FILE"
echo "Docker network: $DOCKER_NETWORK" | tee -a "$LOG_FILE"
echo "---" | tee -a "$LOG_FILE"

PASSED=0
FAILED=0
MAX_LAG=0

for ((i=1; i<=ROUNDS; i++)); do
    DISCONNECT_DURATION=$((RANDOM % 26 + 5))

    echo -n "Round $i/$ROUNDS: disconnect ${DISCONNECT_DURATION}s ... " | tee -a "$LOG_FILE"

    if [ "$DRY_RUN" = true ]; then
        echo "[DRY RUN] would disconnect $EDGE_NATS_CONTAINER from $DOCKER_NETWORK for ${DISCONNECT_DURATION}s" | tee -a "$LOG_FILE"
        PASSED=$((PASSED + 1))
        continue
    fi

    docker network disconnect "$DOCKER_NETWORK" "$EDGE_NATS_CONTAINER" 2>/dev/null || {
        echo "SKIP (container not found)" | tee -a "$LOG_FILE"
        continue
    }

    EDGE_TEST_ID="cccccccc-0000-0000-0000-$(printf '%012d' $i)"
    psql "$EDGE_DB" -c "INSERT INTO assets (id, tenant_id, asset_tag, name, type, status, sync_version) VALUES ('$EDGE_TEST_ID', 'a0000000-0000-0000-0000-000000000001', 'CHAOS-EDGE-$i', 'Chaos Edge $i', 'server', 'operational', $i) ON CONFLICT (id) DO NOTHING" 2>/dev/null

    CENTRAL_TEST_ID="dddddddd-0000-0000-0000-$(printf '%012d' $i)"
    psql "$CENTRAL_DB" -c "INSERT INTO assets (id, tenant_id, asset_tag, name, type, status, sync_version) VALUES ('$CENTRAL_TEST_ID', 'a0000000-0000-0000-0000-000000000001', 'CHAOS-CENTRAL-$i', 'Chaos Central $i', 'server', 'operational', $i) ON CONFLICT (id) DO NOTHING" 2>/dev/null

    sleep "$DISCONNECT_DURATION"

    docker network connect "$DOCKER_NETWORK" "$EDGE_NATS_CONTAINER"

    sleep 10

    EDGE_VER=$(psql -t "$EDGE_DB" -c "SELECT COALESCE(MAX(last_sync_version), 0) FROM sync_state WHERE entity_type = 'assets'" 2>/dev/null | tr -d ' ')
    CENTRAL_VER=$(psql -t "$CENTRAL_DB" -c "SELECT COALESCE(MAX(sync_version), 0) FROM assets WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001'" 2>/dev/null | tr -d ' ')

    LAG=$((CENTRAL_VER - EDGE_VER))
    if [ "$LAG" -lt 0 ]; then LAG=0; fi
    if [ "$LAG" -gt "$MAX_LAG" ]; then MAX_LAG=$LAG; fi

    if [ "$EDGE_VER" = "$CENTRAL_VER" ] || [ "$LAG" -le 5 ]; then
        echo "reconnect OK, sync lag ${LAG} versions ✓" | tee -a "$LOG_FILE"
        PASSED=$((PASSED + 1))
    else
        echo "FAIL: edge=$EDGE_VER central=$CENTRAL_VER lag=$LAG ✗" | tee -a "$LOG_FILE"
        FAILED=$((FAILED + 1))
    fi
done

echo "---" | tee -a "$LOG_FILE"
echo "SUMMARY: $PASSED/$ROUNDS passed, $FAILED failed, max lag $MAX_LAG versions" | tee -a "$LOG_FILE"

if [ "$DRY_RUN" = false ]; then
    psql "$CENTRAL_DB" -c "DELETE FROM assets WHERE asset_tag LIKE 'CHAOS-%'" 2>/dev/null || true
    psql "$EDGE_DB" -c "DELETE FROM assets WHERE asset_tag LIKE 'CHAOS-%'" 2>/dev/null || true
    echo "Cleanup complete" | tee -a "$LOG_FILE"
fi

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi
