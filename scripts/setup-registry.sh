#!/usr/bin/env bash
set -euo pipefail

# Setup a simple Docker Registry on the current machine.
# Usage: ./scripts/setup-registry.sh [port] [data_dir]

PORT="${1:-5000}"
DATA_DIR="${2:-/data/registry}"

echo "=== Setting up Docker Registry ==="
echo "Port: ${PORT}"
echo "Data: ${DATA_DIR}"

# Check Docker
if ! command -v docker &>/dev/null; then
    echo "ERROR: Docker is not installed."
    exit 1
fi

# Create data directory
mkdir -p "$DATA_DIR"

# Check if already running
if docker ps --format '{{.Names}}' | grep -q '^registry$'; then
    echo "Registry is already running."
    docker ps --filter name=registry --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    exit 0
fi

# Remove stopped container if exists
docker rm -f registry 2>/dev/null || true

# Start registry
docker run -d \
    --name registry \
    --restart always \
    -p "${PORT}:5000" \
    -v "${DATA_DIR}:/var/lib/registry" \
    registry:2

echo ""
echo "=== Registry started ==="

# Get machine IP
IP=$(hostname -I | awk '{print $1}')

echo "URL: http://${IP}:${PORT}"
echo "Verify: curl http://${IP}:${PORT}/v2/_catalog"
echo ""
echo "IMPORTANT: On all Docker machines that need to push/pull, add to /etc/docker/daemon.json:"
echo ""
echo "  {\"insecure-registries\": [\"${IP}:${PORT}\"]}"
echo ""
echo "Then restart Docker: systemctl restart docker"
echo ""
echo "To push CMDB images:"
echo "  ./scripts/push-to-registry.sh ${IP}:${PORT}/cmdb v1.2.0"
