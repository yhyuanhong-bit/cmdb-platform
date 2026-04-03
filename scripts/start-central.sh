#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR/../cmdb-core/deploy"

echo "=== CMDB Platform - Central Mode ==="
echo ""

# Create .env from .env.example if missing
if [ ! -f "$DEPLOY_DIR/.env" ]; then
  echo "Creating .env from .env.example..."
  cp "$DEPLOY_DIR/.env.example" "$DEPLOY_DIR/.env"
  echo "  -> Edit deploy/.env to set passwords before production use."
  echo ""
fi

export DEPLOY_MODE=central

# Start storage services first
echo "Starting storage services (postgres, redis, nats)..."
cd "$DEPLOY_DIR"
docker compose up -d postgres redis nats
echo "Waiting for storage to be ready..."
sleep 5

# Start remaining services
echo "Starting all services..."
docker compose up -d

echo ""
echo "=== Central Stack URLs ==="
echo "  Frontend:   http://localhost:80"
echo "  API:        http://localhost:8080"
echo "  MCP:        http://localhost:3001"
echo "  Ingestion:  http://localhost:8081"
echo "  Grafana:    http://localhost:3000  (admin/admin)"
echo "  Jaeger:     http://localhost:16686"
echo "  Prometheus: http://localhost:9090"
echo ""
echo "Use 'docker compose logs -f' to follow logs."
