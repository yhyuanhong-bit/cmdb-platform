#!/usr/bin/env bash
set -euo pipefail

# Build offline installation package for air-gapped environments.
# Outputs: cmdb-platform-offline-<version>.tar.gz
#
# Usage: ./scripts/build-offline-package.sh [version]
# Example: ./scripts/build-offline-package.sh v1.2.0

VERSION="${1:-latest}"
OUTPUT_DIR="cmdb-platform-offline"
COMPOSE_FILE="cmdb-core/deploy/docker-compose.yml"

echo "=== Building CMDB Platform Offline Package (${VERSION}) ==="

# 1. Build images
echo "[1/5] Building images..."
docker compose -f "$COMPOSE_FILE" build

# 2. Pull dependency images
echo "[2/5] Pulling dependency images..."
docker compose -f "$COMPOSE_FILE" pull --ignore-buildable

# 3. List all images
echo "[3/5] Collecting image list..."
IMAGES=$(docker compose -f "$COMPOSE_FILE" config --images | sort -u)
echo "Images to package:"
echo "$IMAGES" | sed 's/^/  /'

# 4. Save images to tar
echo "[4/5] Saving images to tar (this may take a few minutes)..."
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
docker save $IMAGES | gzip > "$OUTPUT_DIR/images.tar.gz"

# 5. Copy deployment files
echo "[5/5] Copying deployment files..."
cp "$COMPOSE_FILE" "$OUTPUT_DIR/docker-compose.yml"
cp cmdb-core/deploy/docker-compose.edge.yml "$OUTPUT_DIR/docker-compose.edge.yml" 2>/dev/null || true
cp cmdb-core/deploy/.env.example "$OUTPUT_DIR/.env.example"
cp cmdb-core/deploy/nats/nats-central.conf "$OUTPUT_DIR/nats-central.conf" 2>/dev/null || true
cp cmdb-core/deploy/nats/nats-edge.conf "$OUTPUT_DIR/nats-edge.conf" 2>/dev/null || true

# Create install script
cat > "$OUTPUT_DIR/install.sh" << 'INSTALL_EOF'
#!/usr/bin/env bash
set -euo pipefail

echo "=== CMDB Platform Offline Installer ==="

# Check Docker
if ! command -v docker &>/dev/null; then
    echo "ERROR: Docker is not installed. Install Docker first."
    exit 1
fi

if ! docker compose version &>/dev/null; then
    echo "ERROR: Docker Compose v2 is not installed."
    exit 1
fi

# Load images
echo "[1/3] Loading Docker images (this may take a few minutes)..."
docker load < images.tar.gz

# Setup config
if [ ! -f .env ]; then
    echo "[2/3] Creating .env from template..."
    cp .env.example .env
    echo "IMPORTANT: Edit .env and set secure passwords before starting!"
    echo "  vim .env"
else
    echo "[2/3] .env already exists, skipping..."
fi

# Start
echo "[3/3] Starting services..."
docker compose up -d

echo ""
echo "=== Installation complete ==="
echo "Frontend: http://$(hostname -I | awk '{print $1}'):${FRONTEND_PORT:-80}"
echo "API:      http://$(hostname -I | awk '{print $1}'):${API_PORT:-8080}"
echo ""
echo "Default login: admin / admin123"
echo "IMPORTANT: Change the admin password after first login!"
INSTALL_EOF
chmod +x "$OUTPUT_DIR/install.sh"

# Create uninstall script
cat > "$OUTPUT_DIR/uninstall.sh" << 'UNINSTALL_EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "Stopping and removing CMDB Platform..."
docker compose down -v
echo "Done. Docker images remain (run 'docker image prune' to clean)."
UNINSTALL_EOF
chmod +x "$OUTPUT_DIR/uninstall.sh"

# Package everything
echo ""
echo "=== Packaging ==="
tar czf "cmdb-platform-offline-${VERSION}.tar.gz" "$OUTPUT_DIR"
SIZE=$(du -sh "cmdb-platform-offline-${VERSION}.tar.gz" | cut -f1)
echo "Package: cmdb-platform-offline-${VERSION}.tar.gz ($SIZE)"
echo ""
echo "To deploy on target machine:"
echo "  1. Copy cmdb-platform-offline-${VERSION}.tar.gz to target"
echo "  2. tar xzf cmdb-platform-offline-${VERSION}.tar.gz"
echo "  3. cd $OUTPUT_DIR"
echo "  4. vim .env  # set passwords"
echo "  5. ./install.sh"

# Cleanup
rm -rf "$OUTPUT_DIR"
