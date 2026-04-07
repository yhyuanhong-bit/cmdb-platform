# Phase 4 Group 1: Prediction Enhancement + Upgrade Recommendations

**Date:** 2026-04-07
**Scope:** RUL calculation, failure distribution analysis, upgrade recommendation engine
**Depends on:** Phase 1-3 completed (metrics API, asset attributes, prediction service exist)

---

## 1. Overview

Enhance the existing prediction service and build an upgrade recommendation engine:

1. **RUL (Remaining Useful Life)** — Calculate asset remaining lifespan from purchase date + warranty + asset type preset lifespan
2. **Failure Distribution** — Aggregate alert rules + work order types to compute failure category breakdown
3. **Upgrade Recommendations** — Rule-based engine that compares asset metrics vs thresholds to generate recommendations
4. **PredictiveHub + ComponentUpgrade** — Connect both pages to real data

```
Existing (Phase 1-3 built):
  ├── prediction_models table     → 2 models seeded
  ├── prediction_results table    → 5 predictions seeded (disk, memory, fan, etc.)
  ├── rca_analyses table          → 2 RCAs seeded
  ├── metrics hypertable          → cpu_usage, memory_usage, disk_usage, temperature
  ├── alert_rules table           → 5 rules with metric_name
  ├── work_orders table           → 6 orders with type field
  └── assets.attributes JSONB     → cpu, memory, storage, warranty_expiry

New (this phase):
  ├── upgrade_rules table         → threshold rules for recommendations
  ├── GET /prediction/rul/{id}    → RUL calculation
  ├── GET /prediction/failure-distribution → failure type breakdown
  ├── GET /assets/{id}/upgrade-recommendations → per-asset recommendations
  ├── POST /assets/{id}/upgrade-recommendations/{recId}/accept → create work order
  └── Frontend: PredictiveHub + ComponentUpgrade pages connected
```

---

## 2. Database Schema

### Migration: `000019_phase4_group1.up.sql`

```sql
-- Upgrade rules: define thresholds for generating recommendations
CREATE TABLE upgrade_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    asset_type  VARCHAR(50) NOT NULL,              -- server / network / storage
    category    VARCHAR(30) NOT NULL,              -- cpu / memory / storage / network
    metric_name VARCHAR(100) NOT NULL,             -- cpu_usage / memory_usage / disk_usage
    threshold   NUMERIC(10,2) NOT NULL,            -- e.g. 80.00 (percent)
    duration_days INT NOT NULL DEFAULT 7,          -- sustained above threshold for N days
    priority    VARCHAR(20) NOT NULL DEFAULT 'medium', -- low / medium / high / critical
    recommendation TEXT NOT NULL,                  -- "Upgrade to next-gen CPU"
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_upgrade_rules_tenant ON upgrade_rules(tenant_id);

-- Seed default rules
INSERT INTO upgrade_rules (tenant_id, asset_type, category, metric_name, threshold, duration_days, priority, recommendation) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'server', 'cpu', 'cpu_usage', 80.00, 7, 'high', 'Upgrade CPU to next generation processor'),
    ('a0000000-0000-0000-0000-000000000001', 'server', 'memory', 'memory_usage', 85.00, 7, 'high', 'Increase memory capacity'),
    ('a0000000-0000-0000-0000-000000000001', 'server', 'storage', 'disk_usage', 85.00, 7, 'critical', 'Expand storage or migrate to larger drives'),
    ('a0000000-0000-0000-0000-000000000001', 'network', 'network', 'cpu_usage', 70.00, 7, 'medium', 'Upgrade network equipment firmware or hardware')
ON CONFLICT DO NOTHING;
```

### Down migration: `000019_phase4_group1.down.sql`

```sql
DROP TABLE IF EXISTS upgrade_rules;
```

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| No separate `upgrade_recommendations` results table | Recommendations are computed on-demand (not stored), keeping it simple for Phase 1. Can add caching table later if needed. |
| `upgrade_rules` is tenant-scoped | Different tenants may have different thresholds |
| Rules reference `metric_name` | Directly maps to `metrics` table for threshold checking |
| `duration_days` | Prevents false positives from temporary spikes |

---

## 3. API Endpoints (5 new)

### 3.1 GET `/prediction/rul/{assetId}` — Remaining Useful Life

```json
{
  "asset_id": "uuid",
  "asset_name": "SRV-PROD-001",
  "purchase_date": "2023-03-15",
  "warranty_expiry": "2028-03-15",
  "expected_lifespan_years": 5,
  "age_days": 1119,
  "rul_days": 706,
  "rul_status": "healthy",
  "warranty_remaining_days": 706
}
```

**Calculation logic:**
1. Read `assets.attributes->>'warranty_expiry'` — if present, RUL = warranty_expiry - today
2. Read `assets.created_at` as proxy for purchase date (or `attributes->>'purchase_date'`)
3. Default lifespan by asset type: server=5yr, network=7yr, storage=5yr, power=10yr
4. `rul_days` = min(warranty_remaining, lifespan_remaining)
5. `rul_status`: healthy (>365 days), warning (90-365), critical (<90), expired (<=0)

### 3.2 GET `/prediction/failure-distribution` — Failure Category Breakdown

```json
{
  "distribution": [
    { "category": "Thermal", "count": 12, "percentage": 38.7 },
    { "category": "Electrical", "count": 8, "percentage": 25.8 },
    { "category": "Mechanical", "count": 7, "percentage": 22.6 },
    { "category": "Software", "count": 4, "percentage": 12.9 }
  ],
  "total": 31,
  "period_days": 90
}
```

**Calculation logic:**
1. Query `alert_events` JOIN `alert_rules` for last 90 days:
   - metric_name containing 'temperature' or 'temp' → Thermal
   - metric_name containing 'power' or 'voltage' → Electrical
   - metric_name containing 'disk' or 'fan' or 'vibration' → Mechanical
   - metric_name containing 'cpu' or 'memory' or 'software' → Software
2. Query `work_orders` for last 90 days:
   - type 'repair' → count by title keywords (same classification)
   - type 'replacement' → Mechanical
3. Merge counts, calculate percentages

### 3.3 GET `/assets/{assetId}/upgrade-recommendations` — Per-Asset Recommendations

```json
{
  "recommendations": [
    {
      "id": "generated-uuid",
      "category": "cpu",
      "priority": "high",
      "current_spec": "2x Intel Xeon Gold 6348",
      "recommendation": "Upgrade CPU to next generation processor",
      "metric_name": "cpu_usage",
      "avg_value": 87.5,
      "threshold": 80.0,
      "duration_days": 7,
      "cost_estimate": null
    }
  ]
}
```

**Calculation logic:**
1. Get asset type and attributes
2. Load `upgrade_rules` for this asset_type and tenant
3. For each enabled rule:
   - Query `metrics` for avg(value) WHERE asset_id AND metric_name AND time > now() - duration_days
   - If avg > threshold → generate recommendation
4. Enrich with current_spec from `assets.attributes` (e.g., attributes->>'cpu')
5. Return sorted by priority (critical > high > medium > low)

### 3.4 POST `/assets/{assetId}/upgrade-recommendations/{category}/accept` — Accept & Create Work Order

```json
// Request
{ "scheduled_start": "2026-04-15T09:00:00Z" }

// Response
{ "work_order_id": "uuid", "code": "WO-2026-007" }
```

**Logic:**
1. Create a work_order with type="upgrade", asset_id, title from recommendation text
2. Status: "draft"
3. Return the created work order ID

### 3.5 GET/POST `/upgrade-rules` — CRUD for Upgrade Rules

```json
// GET response
{
  "rules": [
    { "id": "uuid", "asset_type": "server", "category": "cpu", "metric_name": "cpu_usage",
      "threshold": 80.0, "duration_days": 7, "priority": "high",
      "recommendation": "Upgrade CPU...", "enabled": true }
  ]
}

// POST request
{ "asset_type": "server", "category": "cpu", "metric_name": "cpu_usage",
  "threshold": 80.0, "duration_days": 7, "priority": "high",
  "recommendation": "Upgrade CPU to next gen" }
```

---

## 4. Go Backend

### New File: `cmdb-core/internal/api/phase4_prediction_endpoints.go`

5 handlers:
- `GetAssetRUL` — RUL calculation from attributes + created_at
- `GetFailureDistribution` — UNION query on alert_events + work_orders with keyword classification
- `GetAssetUpgradeRecommendations` — Load rules, check metrics, generate recommendations
- `AcceptUpgradeRecommendation` — Create work order from recommendation
- `GetUpgradeRules` / `CreateUpgradeRule` — CRUD for rules

### Route Registration

```go
v1.GET("/prediction/rul/:id", apiServer.GetAssetRUL)
v1.GET("/prediction/failure-distribution", apiServer.GetFailureDistribution)
v1.GET("/assets/:id/upgrade-recommendations", apiServer.GetAssetUpgradeRecommendations)
v1.POST("/assets/:id/upgrade-recommendations/:category/accept", apiServer.AcceptUpgradeRecommendation)
v1.GET("/upgrade-rules", apiServer.GetUpgradeRules)
v1.POST("/upgrade-rules", apiServer.CreateUpgradeRule)
```

---

## 5. Frontend Changes

### New API Methods

**`src/lib/api/prediction.ts`** — extend:
```ts
  getRUL: (assetId: string) => apiClient.get(`/prediction/rul/${assetId}`),
  getFailureDistribution: () => apiClient.get('/prediction/failure-distribution'),
```

**`src/lib/api/assets.ts`** — extend:
```ts
  getUpgradeRecommendations: (assetId: string) =>
    apiClient.get(`/assets/${assetId}/upgrade-recommendations`),
  acceptUpgradeRecommendation: (assetId: string, category: string, data: any) =>
    apiClient.post(`/assets/${assetId}/upgrade-recommendations/${category}/accept`, data),
```

### New Hooks

**`src/hooks/usePrediction.ts`** — extend:
```ts
  useAssetRUL(assetId: string)
  useFailureDistribution()
```

**`src/hooks/useAssets.ts`** — extend:
```ts
  useUpgradeRecommendations(assetId: string)
  useAcceptUpgradeRecommendation()
```

### Page Changes

**PredictiveHub** (`src/pages/PredictiveHub.tsx`)

Replace:
- `FAILURE_DIST` hardcoded → `useFailureDistribution()` API
- `FALLBACK_ASSETS` hardcoded → `usePredictionsByAsset()` + `useAssetRUL()` for each asset
- `INSIGHTS_STATS` hardcoded → derive from failure distribution totals
- Keep `AI_MESSAGES` hardcoded for now (AI conversation needs Phase 4 Group 2 or separate work)

**ComponentUpgradeRecommendations** (`src/pages/ComponentUpgradeRecommendations.tsx`)

Replace:
- `initialCards` hardcoded → `useUpgradeRecommendations(assetId)` for selected asset
- Add asset selector dropdown at top (select which asset to analyze)
- Wire "Request Upgrade" button → `useAcceptUpgradeRecommendation().mutate()` → creates work order
- Wire "Schedule Maintenance" button → navigate to `/maintenance/add` with pre-filled asset

---

## 6. File Structure

### New Files

| File | Content |
|------|---------|
| `cmdb-core/db/migrations/000019_phase4_group1.up.sql` | upgrade_rules table + seed |
| `cmdb-core/db/migrations/000019_phase4_group1.down.sql` | Rollback |
| `cmdb-core/internal/api/phase4_prediction_endpoints.go` | 6 handlers |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-core/cmd/server/main.go` | Register 6 new routes |
| `cmdb-demo/src/lib/api/prediction.ts` | Add getRUL, getFailureDistribution |
| `cmdb-demo/src/lib/api/assets.ts` | Add getUpgradeRecommendations, acceptUpgradeRecommendation |
| `cmdb-demo/src/hooks/usePrediction.ts` | Add useAssetRUL, useFailureDistribution |
| `cmdb-demo/src/hooks/useAssets.ts` | Add useUpgradeRecommendations, useAcceptUpgradeRecommendation |
| `cmdb-demo/src/pages/PredictiveHub.tsx` | Replace FAILURE_DIST, FALLBACK_ASSETS, INSIGHTS_STATS |
| `cmdb-demo/src/pages/ComponentUpgradeRecommendations.tsx` | Replace initialCards + add asset selector + wire accept |
