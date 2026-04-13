#!/usr/bin/env bash
# run-server.sh — Run cmdb-core server with auto-restart
# Usage: ./scripts/run-server.sh [--daemon]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="${ROOT_DIR}/logs/server.log"
PID_FILE="${ROOT_DIR}/logs/server.pid"
MAX_RESTARTS=10
RESTART_DELAY=3

mkdir -p "$(dirname "$LOG_FILE")"

cleanup() {
    if [ -f "$PID_FILE" ]; then
        kill "$(cat "$PID_FILE")" 2>/dev/null || true
        rm -f "$PID_FILE"
    fi
}
trap cleanup EXIT

restart_count=0

while [ $restart_count -lt $MAX_RESTARTS ]; do
    echo "[$(date)] Starting cmdb-core server (attempt $((restart_count + 1))/$MAX_RESTARTS)..." | tee -a "$LOG_FILE"

    cd "$ROOT_DIR"
    go run ./cmd/server >> "$LOG_FILE" 2>&1 &
    SERVER_PID=$!
    echo $SERVER_PID > "$PID_FILE"

    wait $SERVER_PID || true
    EXIT_CODE=$?

    if [ $EXIT_CODE -eq 0 ]; then
        echo "[$(date)] Server exited gracefully." | tee -a "$LOG_FILE"
        break
    fi

    restart_count=$((restart_count + 1))
    echo "[$(date)] Server crashed (exit code $EXIT_CODE). Restarting in ${RESTART_DELAY}s..." | tee -a "$LOG_FILE"
    sleep $RESTART_DELAY
done

if [ $restart_count -ge $MAX_RESTARTS ]; then
    echo "[$(date)] Max restarts ($MAX_RESTARTS) reached. Giving up." | tee -a "$LOG_FILE"
    exit 1
fi
