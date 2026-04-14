#!/usr/bin/env bash
set -euo pipefail

# Update CMDB Platform on a deployed machine.
# Pulls latest images and restarts with zero downtime.

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
REGISTRY_OVERRIDE=""

# Check for registry override
if [ -f "docker-compose.registry.yml" ]; then
    REGISTRY_OVERRIDE="-f docker-compose.registry.yml"
fi

echo "=== Updating CMDB Platform ==="

echo "[1/3] Pulling latest images..."
docker compose -f "$COMPOSE_FILE" $REGISTRY_OVERRIDE pull

echo "[2/3] Restarting services..."
docker compose -f "$COMPOSE_FILE" $REGISTRY_OVERRIDE up -d

echo "[3/3] Checking health..."
sleep 5
if curl -sf http://localhost:${API_PORT:-8080}/readyz > /dev/null 2>&1; then
    echo "Health check: OK"
else
    echo "WARNING: Health check failed. Check logs: docker compose logs cmdb-core"
fi

echo ""
echo "=== Update complete ==="
docker compose -f "$COMPOSE_FILE" $REGISTRY_OVERRIDE ps
