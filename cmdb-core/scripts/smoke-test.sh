#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${API_URL:-http://localhost:8080/api/v1}"
PASS=0
FAIL=0
TOTAL=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

assert_ok() {
  local test_name="$1"
  local status="$2"
  TOTAL=$((TOTAL + 1))
  if [ "$status" -ge 200 ] && [ "$status" -lt 300 ]; then
    echo -e "  ${GREEN}✓${NC} $test_name (HTTP $status)"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $test_name (HTTP $status)"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_empty() {
  local test_name="$1"
  local value="$2"
  TOTAL=$((TOTAL + 1))
  if [ -n "$value" ] && [ "$value" != "null" ]; then
    echo -e "  ${GREEN}✓${NC} $test_name"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $test_name (empty or null)"
    FAIL=$((FAIL + 1))
  fi
}

assert_gte() {
  local test_name="$1"
  local actual="$2"
  local expected="$3"
  TOTAL=$((TOTAL + 1))
  if [ "$actual" -ge "$expected" ] 2>/dev/null; then
    echo -e "  ${GREEN}✓${NC} $test_name ($actual >= $expected)"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $test_name ($actual < $expected)"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== CMDB Platform Smoke Test ==="
echo "Target: $BASE_URL"
echo ""

# 1. Login
echo "1. Authentication"
LOGIN_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
LOGIN_STATUS=$(echo "$LOGIN_RESP" | tail -1)
LOGIN_BODY=$(echo "$LOGIN_RESP" | sed '$d')
assert_ok "POST /auth/login" "$LOGIN_STATUS"

TOKEN=$(echo "$LOGIN_BODY" | jq -r '.data.access_token // empty')
assert_not_empty "Access token received" "$TOKEN"

AUTH="Authorization: Bearer $TOKEN"

# 2. Get current user
echo ""
echo "2. Current User"
ME_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/auth/me" -H "$AUTH")
ME_STATUS=$(echo "$ME_RESP" | tail -1)
assert_ok "GET /auth/me" "$ME_STATUS"

# 3. List assets
echo ""
echo "3. Assets"
ASSETS_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/assets" -H "$AUTH")
ASSETS_STATUS=$(echo "$ASSETS_RESP" | tail -1)
ASSETS_BODY=$(echo "$ASSETS_RESP" | sed '$d')
assert_ok "GET /assets" "$ASSETS_STATUS"
ASSET_COUNT=$(echo "$ASSETS_BODY" | jq '.pagination.total // 0')
assert_gte "Asset count" "$ASSET_COUNT" 20

# Get first asset ID for later tests
ASSET_ID=$(echo "$ASSETS_BODY" | jq -r '.data[0].id // empty')

# 4. Get single asset
if [ -n "$ASSET_ID" ]; then
  ASSET_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/assets/$ASSET_ID" -H "$AUTH")
  ASSET_STATUS=$(echo "$ASSET_RESP" | tail -1)
  assert_ok "GET /assets/{id}" "$ASSET_STATUS"
fi

# 5. Dashboard stats
echo ""
echo "4. Dashboard"
DASH_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/dashboard/stats" -H "$AUTH")
DASH_STATUS=$(echo "$DASH_RESP" | tail -1)
assert_ok "GET /dashboard/stats" "$DASH_STATUS"

# 6. Work orders
echo ""
echo "5. Maintenance"
WO_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/maintenance/orders" -H "$AUTH")
WO_STATUS=$(echo "$WO_RESP" | tail -1)
WO_BODY=$(echo "$WO_RESP" | sed '$d')
assert_ok "GET /maintenance/orders" "$WO_STATUS"

# Create a work order
CREATE_WO=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/maintenance/orders" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"title":"Smoke Test Order","type":"inspection","priority":"low"}')
CREATE_WO_STATUS=$(echo "$CREATE_WO" | tail -1)
CREATE_WO_BODY=$(echo "$CREATE_WO" | sed '$d')
assert_ok "POST /maintenance/orders" "$CREATE_WO_STATUS"
ORDER_ID=$(echo "$CREATE_WO_BODY" | jq -r '.data.id // empty')
assert_not_empty "Work order ID" "$ORDER_ID"

# Transition work order
if [ -n "$ORDER_ID" ]; then
  TRANS_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/maintenance/orders/$ORDER_ID/transition" \
    -H "$AUTH" -H "Content-Type: application/json" \
    -d '{"status":"pending","comment":"Smoke test transition"}')
  TRANS_STATUS=$(echo "$TRANS_RESP" | tail -1)
  assert_ok "POST /maintenance/orders/{id}/transition" "$TRANS_STATUS"
fi

# 7. Monitoring
echo ""
echo "6. Monitoring"
ALERTS_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/monitoring/alerts" -H "$AUTH")
ALERTS_STATUS=$(echo "$ALERTS_RESP" | tail -1)
ALERTS_BODY=$(echo "$ALERTS_RESP" | sed '$d')
assert_ok "GET /monitoring/alerts" "$ALERTS_STATUS"
ALERT_COUNT=$(echo "$ALERTS_BODY" | jq '.pagination.total // 0')
assert_gte "Alert count" "$ALERT_COUNT" 1

RULES_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/monitoring/rules" -H "$AUTH")
RULES_STATUS=$(echo "$RULES_RESP" | tail -1)
assert_ok "GET /monitoring/rules" "$RULES_STATUS"

INC_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/monitoring/incidents" -H "$AUTH")
INC_STATUS=$(echo "$INC_RESP" | tail -1)
assert_ok "GET /monitoring/incidents" "$INC_STATUS"

# 8. Metrics (use first asset)
if [ -n "$ASSET_ID" ]; then
  METRICS_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/monitoring/metrics?asset_id=$ASSET_ID&metric_name=cpu_usage&time_range=24h" -H "$AUTH")
  METRICS_STATUS=$(echo "$METRICS_RESP" | tail -1)
  assert_ok "GET /monitoring/metrics" "$METRICS_STATUS"
fi

# 9. Audit events
echo ""
echo "7. Audit"
if [ -n "$ORDER_ID" ]; then
  AUDIT_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/audit/events?target_id=$ORDER_ID" -H "$AUTH")
  AUDIT_STATUS=$(echo "$AUDIT_RESP" | tail -1)
  AUDIT_BODY=$(echo "$AUDIT_RESP" | sed '$d')
  assert_ok "GET /audit/events (by target_id)" "$AUDIT_STATUS"
  AUDIT_COUNT=$(echo "$AUDIT_BODY" | jq '.data | length // 0')
  assert_gte "Audit events for order" "$AUDIT_COUNT" 1
fi

# 10. System health
echo ""
echo "8. System"
HEALTH_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/system/health" -H "$AUTH")
HEALTH_STATUS=$(echo "$HEALTH_RESP" | tail -1)
HEALTH_BODY=$(echo "$HEALTH_RESP" | sed '$d')
assert_ok "GET /system/health" "$HEALTH_STATUS"
DB_STATUS=$(echo "$HEALTH_BODY" | jq -r '.data.database.status // empty')
assert_not_empty "Database status" "$DB_STATUS"

# 11. Inventory
echo ""
echo "9. Inventory"
INV_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/inventory/tasks" -H "$AUTH")
INV_STATUS=$(echo "$INV_RESP" | tail -1)
assert_ok "GET /inventory/tasks" "$INV_STATUS"

# 12. Prediction
echo ""
echo "10. Prediction"
PRED_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/prediction/models" -H "$AUTH")
PRED_STATUS=$(echo "$PRED_RESP" | tail -1)
assert_ok "GET /prediction/models" "$PRED_STATUS"

# 13. Integration
echo ""
echo "11. Integration"
ADAPT_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/integration/adapters" -H "$AUTH")
ADAPT_STATUS=$(echo "$ADAPT_RESP" | tail -1)
assert_ok "GET /integration/adapters" "$ADAPT_STATUS"

WH_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/integration/webhooks" -H "$AUTH")
WH_STATUS=$(echo "$WH_RESP" | tail -1)
assert_ok "GET /integration/webhooks" "$WH_STATUS"

# Summary
echo ""
echo "========================================="
echo "Results: $PASS passed, $FAIL failed (of $TOTAL tests)"
if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}SMOKE TEST FAILED${NC}"
  exit 1
else
  echo -e "${GREEN}ALL SMOKE TESTS PASSED${NC}"
  exit 0
fi
