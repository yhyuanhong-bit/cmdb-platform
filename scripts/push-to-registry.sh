#!/usr/bin/env bash
set -euo pipefail

# Push CMDB Platform images to a private Docker Registry.
#
# Usage: ./scripts/push-to-registry.sh <registry> [version]
# Example: ./scripts/push-to-registry.sh harbor.internal:5000/cmdb v1.2.0

REGISTRY="${1:?Usage: $0 <registry> [version]}"
VERSION="${2:-latest}"
COMPOSE_FILE="cmdb-core/deploy/docker-compose.yml"

# Remove trailing slash
REGISTRY="${REGISTRY%/}"

echo "=== Pushing CMDB Platform to ${REGISTRY} (${VERSION}) ==="

# 1. Build images
echo "[1/3] Building images..."
docker compose -f "$COMPOSE_FILE" build

# 2. Tag and push each built image
echo "[2/3] Tagging and pushing..."

SERVICES=("cmdb-core" "cmdb-frontend" "ingestion-engine" "ingestion-worker")
for SVC in "${SERVICES[@]}"; do
    LOCAL_IMAGE="${SVC}:latest"
    REMOTE_IMAGE="${REGISTRY}/${SVC}:${VERSION}"

    if docker image inspect "$LOCAL_IMAGE" &>/dev/null; then
        echo "  ${LOCAL_IMAGE} → ${REMOTE_IMAGE}"
        docker tag "$LOCAL_IMAGE" "$REMOTE_IMAGE"
        docker push "$REMOTE_IMAGE"

        # Also tag as latest
        if [ "$VERSION" != "latest" ]; then
            docker tag "$LOCAL_IMAGE" "${REGISTRY}/${SVC}:latest"
            docker push "${REGISTRY}/${SVC}:latest"
        fi
    else
        echo "  SKIP: ${LOCAL_IMAGE} not found"
    fi
done

# 3. Generate deployment compose override
echo "[3/3] Generating registry compose override..."
cat > "docker-compose.registry.yml" << EOF
# Auto-generated: use images from private registry
# Usage: docker compose -f docker-compose.yml -f docker-compose.registry.yml up -d
services:
  cmdb-core:
    image: ${REGISTRY}/cmdb-core:${VERSION}
  frontend:
    image: ${REGISTRY}/cmdb-frontend:${VERSION}
  ingestion-engine:
    image: ${REGISTRY}/ingestion-engine:${VERSION}
  ingestion-worker:
    image: ${REGISTRY}/ingestion-worker:${VERSION}
EOF

echo ""
echo "=== Push complete ==="
echo ""
echo "To deploy from registry on target machine:"
echo "  1. Copy docker-compose.yml + docker-compose.registry.yml + .env to target"
echo "  2. docker compose -f docker-compose.yml -f docker-compose.registry.yml up -d"
echo ""
echo "To update on target machine:"
echo "  docker compose -f docker-compose.yml -f docker-compose.registry.yml pull"
echo "  docker compose -f docker-compose.yml -f docker-compose.registry.yml up -d"
