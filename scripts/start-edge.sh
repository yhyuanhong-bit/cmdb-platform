#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR/../cmdb-core/deploy"

# Require TENANT_ID
if [ -z "${TENANT_ID:-}" ]; then
  echo "ERROR: TENANT_ID environment variable is required for edge deployments."
  echo "Usage: TENANT_ID=my-edge-site ./scripts/start-edge.sh"
  exit 1
fi

echo "=== CMDB Platform - Edge Mode ==="
echo "  Tenant: $TENANT_ID"
echo ""

export DEPLOY_MODE=edge
export TENANT_ID

# Create .env from .env.example if missing
if [ ! -f "$DEPLOY_DIR/.env" ]; then
  echo "Creating .env from .env.example..."
  cp "$DEPLOY_DIR/.env.example" "$DEPLOY_DIR/.env"
fi

cd "$DEPLOY_DIR"

# Start storage first
echo "Starting storage services (postgres, redis, nats)..."
docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d postgres redis nats
echo "Waiting for storage to be ready..."
sleep 5

# Start all edge services
echo "Starting edge services..."
docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d

echo ""
echo "=== Edge Stack URLs ==="
echo "  Frontend:  http://localhost:80"
echo "  API:       http://localhost:8080"
echo "  Ingestion: http://localhost:8081"
echo ""
echo "Observability stack is disabled in edge mode."
echo "Use 'docker compose logs -f' to follow logs."
