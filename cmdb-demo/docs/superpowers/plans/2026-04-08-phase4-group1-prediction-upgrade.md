# Phase 4 Group 1: Prediction Enhancement + Upgrade Recommendations

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add RUL calculation, failure distribution analysis, and rule-based upgrade recommendation engine with 6 Go endpoints and 2 frontend page integrations.

**Architecture:** New Go handler file with raw SQL on pgxpool. RUL computed from asset attributes. Failure distribution from alert message keyword classification + work order types. Upgrade recommendations computed on-demand by comparing metrics averages against threshold rules.

**Tech Stack:** Go/Gin, pgxpool raw SQL, React/TypeScript, TanStack React Query.

**Spec:** `docs/superpowers/specs/2026-04-07-phase4-group1-prediction-upgrade-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/db/migrations/000019_phase4_group1.up.sql` | upgrade_rules table + seed data |
| `cmdb-core/db/migrations/000019_phase4_group1.down.sql` | Rollback |
| `cmdb-core/internal/api/phase4_prediction_endpoints.go` | 6 Go handlers (RUL, failure dist, recommendations, accept, rules CRUD) |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-core/cmd/server/main.go` | Register 6 new routes |
| `cmdb-demo/src/lib/api/prediction.ts` | Add getRUL, getFailureDistribution |
| `cmdb-demo/src/lib/api/assets.ts` | Add getUpgradeRecommendations, acceptUpgradeRecommendation |
| `cmdb-demo/src/hooks/usePrediction.ts` | Add useAssetRUL, useFailureDistribution |
| `cmdb-demo/src/hooks/useAssets.ts` | Add useUpgradeRecommendations, useAcceptUpgradeRecommendation |
| `cmdb-demo/src/pages/PredictiveHub.tsx` | Replace FAILURE_DIST, FALLBACK_ASSETS, INSIGHTS_STATS |
| `cmdb-demo/src/pages/ComponentUpgradeRecommendations.tsx` | Replace initialCards + add asset selector |

---

## Task 1: Database Migration

**Files:**
- Create: `cmdb-core/db/migrations/000019_phase4_group1.up.sql`
- Create: `cmdb-core/db/migrations/000019_phase4_group1.down.sql`

- [ ] **Step 1: Write the up migration**

Create `cmdb-core/db/migrations/000019_phase4_group1.up.sql` with the exact SQL from the spec Section 2 (CREATE TABLE upgrade_rules + seed INSERT).

- [ ] **Step 2: Write the down migration**

Create `cmdb-core/db/migrations/000019_phase4_group1.down.sql`:
```sql
DROP TABLE IF EXISTS upgrade_rules;
```

- [ ] **Step 3: Run the migration**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -f cmdb-core/db/migrations/000019_phase4_group1.up.sql
```

- [ ] **Step 4: Verify**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "SELECT asset_type, category, metric_name, threshold, priority FROM upgrade_rules"
```

Expected: 4 rows (server/cpu, server/memory, server/storage, network/network).

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/db/migrations/000019_phase4_group1.up.sql cmdb-core/db/migrations/000019_phase4_group1.down.sql
git commit -m "feat: add upgrade_rules table with seed data (Phase 4 Group 1)"
```

---

## Task 2: Go Endpoints — RUL + Failure Distribution + Upgrade Rules CRUD

**Files:**
- Create: `cmdb-core/internal/api/phase4_prediction_endpoints.go`

- [ ] **Step 1: Create the endpoint file with all 6 handlers**

Create `cmdb-core/internal/api/phase4_prediction_endpoints.go`:

```go
package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --- RUL ---

var lifespanYears = map[string]int{
	"server": 5, "network": 7, "storage": 5, "power": 10,
}

// GetAssetRUL handles GET /prediction/rul/:id
func (s *APIServer) GetAssetRUL(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}

	var name, assetType string
	var attributes []byte
	var createdAt time.Time
	err = s.pool.QueryRow(c.Request.Context(), `
		SELECT name, type, attributes, created_at FROM assets WHERE id = $1
	`, assetID).Scan(&name, &assetType, &attributes, &createdAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}

	// Parse attributes for dates
	attrMap := parseJSONAttributes(attributes)
	purchaseDate := attrMap["purchase_date"]
	warrantyExpiry := attrMap["warranty_expiry"]

	// Calculate age
	var purchaseTime time.Time
	if purchaseDate != "" {
		purchaseTime, _ = time.Parse("2006-01-02", purchaseDate)
	}
	if purchaseTime.IsZero() {
		purchaseTime = createdAt
	}

	now := time.Now()
	ageDays := int(now.Sub(purchaseTime).Hours() / 24)

	// Warranty remaining
	warrantyDays := -1
	if warrantyExpiry != "" {
		wt, err := time.Parse("2006-01-02", warrantyExpiry)
		if err == nil {
			warrantyDays = int(wt.Sub(now).Hours() / 24)
		}
	}

	// Lifespan remaining
	years := lifespanYears[assetType]
	if years == 0 {
		years = 5
	}
	lifespanDays := years*365 - ageDays

	// RUL = min of warranty and lifespan remaining
	rulDays := lifespanDays
	if warrantyDays >= 0 && warrantyDays < rulDays {
		rulDays = warrantyDays
	}

	status := "healthy"
	if rulDays <= 0 {
		status = "expired"
	} else if rulDays < 90 {
		status = "critical"
	} else if rulDays < 365 {
		status = "warning"
	}

	c.JSON(http.StatusOK, gin.H{
		"asset_id":               assetID.String(),
		"asset_name":             name,
		"purchase_date":          purchaseDate,
		"warranty_expiry":        warrantyExpiry,
		"expected_lifespan_years": years,
		"age_days":               ageDays,
		"rul_days":               rulDays,
		"rul_status":             status,
		"warranty_remaining_days": warrantyDays,
	})
}

// --- Failure Distribution ---

func classifyMessage(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "temperature") || strings.Contains(lower, "temp") || strings.Contains(lower, "thermal"):
		return "Thermal"
	case strings.Contains(lower, "power") || strings.Contains(lower, "voltage") || strings.Contains(lower, "electrical") || strings.Contains(lower, "pdu") || strings.Contains(lower, "ups"):
		return "Electrical"
	case strings.Contains(lower, "disk") || strings.Contains(lower, "fan") || strings.Contains(lower, "vibration") || strings.Contains(lower, "hardware"):
		return "Mechanical"
	case strings.Contains(lower, "cpu") || strings.Contains(lower, "memory") || strings.Contains(lower, "software") || strings.Contains(lower, "firmware"):
		return "Software"
	default:
		return "Other"
	}
}

// GetFailureDistribution handles GET /prediction/failure-distribution
func (s *APIServer) GetFailureDistribution(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	periodDays := 90

	counts := map[string]int{"Thermal": 0, "Electrical": 0, "Mechanical": 0, "Software": 0, "Other": 0}

	// Arm 1: alert events (by message keyword)
	alertRows, err := s.pool.Query(c.Request.Context(), `
		SELECT COALESCE(ae.message, '') FROM alert_events ae
		WHERE ae.tenant_id = $1 AND ae.fired_at > now() - ($2 || ' days')::interval
	`, tenantID, strconv.Itoa(periodDays))
	if err == nil {
		defer alertRows.Close()
		for alertRows.Next() {
			var msg string
			if alertRows.Scan(&msg) == nil {
				counts[classifyMessage(msg)]++
			}
		}
	}

	// Arm 2: work orders (by type + title keyword)
	woRows, err := s.pool.Query(c.Request.Context(), `
		SELECT type, COALESCE(title, '') FROM work_orders
		WHERE tenant_id = $1 AND created_at > now() - ($2 || ' days')::interval
	`, tenantID, strconv.Itoa(periodDays))
	if err == nil {
		defer woRows.Close()
		for woRows.Next() {
			var woType, title string
			if woRows.Scan(&woType, &title) == nil {
				switch woType {
				case "replacement":
					counts["Mechanical"]++
				case "upgrade":
					counts["Software"]++
				default:
					counts[classifyMessage(title)]++
				}
			}
		}
	}

	total := 0
	for _, v := range counts {
		total += v
	}

	var dist []gin.H
	for _, cat := range []string{"Thermal", "Electrical", "Mechanical", "Software", "Other"} {
		cnt := counts[cat]
		if cnt == 0 && cat == "Other" {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(cnt)/float64(total)*1000) / 10
		}
		dist = append(dist, gin.H{"category": cat, "count": cnt, "percentage": pct})
	}
	if dist == nil {
		dist = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{
		"distribution": dist,
		"total":        total,
		"period_days":  periodDays,
	})
}

// --- Upgrade Recommendations ---

// GetAssetUpgradeRecommendations handles GET /assets/:id/upgrade-recommendations
func (s *APIServer) GetAssetUpgradeRecommendations(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}
	tenantID := tenantIDFromContext(c)

	// Get asset type + attributes
	var assetType string
	var attributes []byte
	err = s.pool.QueryRow(c.Request.Context(), `
		SELECT type, attributes FROM assets WHERE id = $1 AND tenant_id = $2
	`, assetID, tenantID).Scan(&assetType, &attributes)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}
	attrMap := parseJSONAttributes(attributes)

	// Load upgrade rules for this asset type
	ruleRows, err := s.pool.Query(c.Request.Context(), `
		SELECT id, category, metric_name, threshold, duration_days, priority, recommendation
		FROM upgrade_rules
		WHERE tenant_id = $1 AND asset_type = $2 AND enabled = true
		ORDER BY priority DESC
	`, tenantID, assetType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query rules failed"})
		return
	}
	defer ruleRows.Close()

	var recommendations []gin.H
	priorityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

	for ruleRows.Next() {
		var ruleID uuid.UUID
		var category, metricName, priority, recommendation string
		var threshold float64
		var durationDays int
		if ruleRows.Scan(&ruleID, &category, &metricName, &threshold, &durationDays, &priority, &recommendation) != nil {
			continue
		}

		// Check metric average over duration
		var avgValue *float64
		s.pool.QueryRow(c.Request.Context(), `
			SELECT avg(value) FROM metrics
			WHERE asset_id = $1 AND name = $2 AND time > now() - ($3 || ' days')::interval
		`, assetID, metricName, strconv.Itoa(durationDays)).Scan(&avgValue)

		if avgValue != nil && *avgValue > threshold {
			currentSpec := attrMap[category]
			if currentSpec == "" {
				currentSpec = "-"
			}
			recommendations = append(recommendations, gin.H{
				"id":            uuid.New().String(),
				"category":      category,
				"priority":      priority,
				"current_spec":  currentSpec,
				"recommendation": recommendation,
				"metric_name":   metricName,
				"avg_value":     math.Round(*avgValue*10) / 10,
				"threshold":     threshold,
				"duration_days": durationDays,
				"cost_estimate": nil,
			})
		}
	}

	// Sort by priority
	if len(recommendations) > 1 {
		for i := 0; i < len(recommendations)-1; i++ {
			for j := i + 1; j < len(recommendations); j++ {
				pi := priorityOrder[recommendations[i]["priority"].(string)]
				pj := priorityOrder[recommendations[j]["priority"].(string)]
				if pj < pi {
					recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
				}
			}
		}
	}
	if recommendations == nil {
		recommendations = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{"recommendations": recommendations})
}

// AcceptUpgradeRecommendation handles POST /assets/:id/upgrade-recommendations/:category/accept
func (s *APIServer) AcceptUpgradeRecommendation(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}
	category := c.Param("category")
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)

	var req struct {
		ScheduledStart *string `json:"scheduled_start"`
	}
	c.ShouldBindJSON(&req)

	// Get recommendation text from rules
	var recommendation string
	s.pool.QueryRow(c.Request.Context(), `
		SELECT recommendation FROM upgrade_rules
		WHERE tenant_id = $1 AND category = $2 AND enabled = true LIMIT 1
	`, tenantID, category).Scan(&recommendation)
	if recommendation == "" {
		recommendation = "Upgrade " + category
	}

	// Generate work order code
	year := time.Now().Year()
	prefix := fmt.Sprintf("WO-%d-", year)
	var lastCode string
	s.pool.QueryRow(c.Request.Context(), `
		SELECT code FROM work_orders WHERE code LIKE $1 || '%' ORDER BY code DESC LIMIT 1
	`, prefix).Scan(&lastCode)

	nextNum := 1
	if lastCode != "" {
		parts := strings.Split(lastCode, "-")
		if len(parts) == 3 {
			if n, err := strconv.Atoi(parts[2]); err == nil {
				nextNum = n + 1
			}
		}
	}
	code := fmt.Sprintf("WO-%d-%04d", year, nextNum)

	// Create work order
	woID := uuid.New()
	title := fmt.Sprintf("[Upgrade] %s - %s", strings.ToUpper(category), recommendation)

	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO work_orders (id, tenant_id, code, title, type, status, priority, asset_id, requestor_id, description)
		VALUES ($1, $2, $3, $4, 'upgrade', 'draft', 'medium', $5, $6, $7)
	`, woID, tenantID, code, title, assetID, userID, recommendation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create work order"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"work_order_id": woID.String(), "code": code})
}

// --- Upgrade Rules CRUD ---

// GetUpgradeRules handles GET /upgrade-rules
func (s *APIServer) GetUpgradeRules(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT id, asset_type, category, metric_name, threshold, duration_days, priority, recommendation, enabled, created_at
		FROM upgrade_rules WHERE tenant_id = $1 ORDER BY asset_type, category
	`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var rules []gin.H
	for rows.Next() {
		var id uuid.UUID
		var assetType, category, metricName, priority, recommendation string
		var threshold float64
		var durationDays int
		var enabled bool
		var createdAt time.Time
		if rows.Scan(&id, &assetType, &category, &metricName, &threshold, &durationDays, &priority, &recommendation, &enabled, &createdAt) != nil {
			continue
		}
		rules = append(rules, gin.H{
			"id": id.String(), "asset_type": assetType, "category": category,
			"metric_name": metricName, "threshold": threshold, "duration_days": durationDays,
			"priority": priority, "recommendation": recommendation, "enabled": enabled,
		})
	}
	if rules == nil {
		rules = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// CreateUpgradeRule handles POST /upgrade-rules
func (s *APIServer) CreateUpgradeRule(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	var req struct {
		AssetType      string  `json:"asset_type"`
		Category       string  `json:"category"`
		MetricName     string  `json:"metric_name"`
		Threshold      float64 `json:"threshold"`
		DurationDays   int     `json:"duration_days"`
		Priority       string  `json:"priority"`
		Recommendation string  `json:"recommendation"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AssetType == "" || req.Category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_type, category, metric_name, threshold, recommendation required"})
		return
	}
	if req.DurationDays == 0 {
		req.DurationDays = 7
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}

	id := uuid.New()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO upgrade_rules (id, tenant_id, asset_type, category, metric_name, threshold, duration_days, priority, recommendation)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, tenantID, req.AssetType, req.Category, req.MetricName, req.Threshold, req.DurationDays, req.Priority, req.Recommendation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

// --- Helpers ---

func parseJSONAttributes(data []byte) map[string]string {
	result := map[string]string{}
	if data == nil {
		return result
	}
	// Simple JSON object parsing for string values
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return result
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}
```

**Note:** Add `"encoding/json"` to the imports at the top for `parseJSONAttributes`.

- [ ] **Step 2: Build to verify**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

Fix any compile errors. The build may fail because routes aren't registered yet — that's OK if the file compiles without syntax errors.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/internal/api/phase4_prediction_endpoints.go
git commit -m "feat: add RUL, failure distribution, upgrade recommendations Go endpoints"
```

---

## Task 3: Register Routes + Build

**Files:**
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add route registrations**

Read `cmdb-core/cmd/server/main.go`. After the existing Phase 3 routes, add:

```go
	// Phase 4 Group 1 routes
	v1.GET("/prediction/rul/:id", apiServer.GetAssetRUL)
	v1.GET("/prediction/failure-distribution", apiServer.GetFailureDistribution)
	v1.GET("/assets/:id/upgrade-recommendations", apiServer.GetAssetUpgradeRecommendations)
	v1.POST("/assets/:id/upgrade-recommendations/:category/accept", apiServer.AcceptUpgradeRecommendation)
	v1.GET("/upgrade-rules", apiServer.GetUpgradeRules)
	v1.POST("/upgrade-rules", apiServer.CreateUpgradeRule)
```

- [ ] **Step 2: Build and verify**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/cmd/server/main.go
git commit -m "feat: register Phase 4 Group 1 routes (RUL, failure dist, upgrade rules)"
```

---

## Task 4: Frontend API Clients + Hooks

**Files:**
- Modify: `cmdb-demo/src/lib/api/prediction.ts`
- Modify: `cmdb-demo/src/lib/api/assets.ts`
- Modify: `cmdb-demo/src/hooks/usePrediction.ts`
- Modify: `cmdb-demo/src/hooks/useAssets.ts`

- [ ] **Step 1: Extend prediction API client**

Read `cmdb-demo/src/lib/api/prediction.ts`. Add to the `predictionApi` object:

```ts
  getRUL: (assetId: string) =>
    apiClient.get(`/prediction/rul/${assetId}`),
  getFailureDistribution: () =>
    apiClient.get('/prediction/failure-distribution'),
```

- [ ] **Step 2: Extend assets API client**

Read `cmdb-demo/src/lib/api/assets.ts`. Add to the `assetApi` object:

```ts
  getUpgradeRecommendations: (assetId: string) =>
    apiClient.get(`/assets/${assetId}/upgrade-recommendations`),
  acceptUpgradeRecommendation: (assetId: string, category: string, data?: any) =>
    apiClient.post(`/assets/${assetId}/upgrade-recommendations/${category}/accept`, data || {}),
```

- [ ] **Step 3: Extend prediction hooks**

Read `cmdb-demo/src/hooks/usePrediction.ts`. Add:

```ts
export function useAssetRUL(assetId: string) {
  return useQuery({
    queryKey: ['assetRUL', assetId],
    queryFn: () => predictionApi.getRUL(assetId),
    enabled: !!assetId,
  })
}

export function useFailureDistribution() {
  return useQuery({
    queryKey: ['failureDistribution'],
    queryFn: () => predictionApi.getFailureDistribution(),
  })
}
```

- [ ] **Step 4: Extend asset hooks**

Read `cmdb-demo/src/hooks/useAssets.ts`. Add:

```ts
export function useUpgradeRecommendations(assetId: string) {
  return useQuery({
    queryKey: ['upgradeRecommendations', assetId],
    queryFn: () => assetApi.getUpgradeRecommendations(assetId),
    enabled: !!assetId,
  })
}

export function useAcceptUpgradeRecommendation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ assetId, category, data }: { assetId: string; category: string; data?: any }) =>
      assetApi.acceptUpgradeRecommendation(assetId, category, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['upgradeRecommendations'] })
      qc.invalidateQueries({ queryKey: ['workOrders'] })
    },
  })
}
```

Import `assetApi` if not already imported. Import `useMutation`, `useQueryClient` if not already imported.

- [ ] **Step 5: Commit**

```bash
git add cmdb-demo/src/lib/api/prediction.ts cmdb-demo/src/lib/api/assets.ts \
       cmdb-demo/src/hooks/usePrediction.ts cmdb-demo/src/hooks/useAssets.ts
git commit -m "feat: add Phase 4 Group 1 frontend API clients and hooks"
```

---

## Task 5: Connect PredictiveHub Page

**Files:**
- Modify: `cmdb-demo/src/pages/PredictiveHub.tsx`

- [ ] **Step 1: Read the current file and identify hardcoded data**

Read the full file. Find:
- `FAILURE_DIST` — hardcoded array with Mechanical/Electrical/Thermal/Software percentages
- `FALLBACK_ASSETS` — hardcoded array with asset names, failure dates, RUL days
- `INSIGHTS_STATS` — hardcoded counts (14 critical, 28 major, 42 minor)

- [ ] **Step 2: Replace hardcoded data with API hooks**

1. Import: `import { useFailureDistribution } from '../hooks/usePrediction'`
2. Add query: `const { data: failDistData } = useFailureDistribution()`
3. Replace `FAILURE_DIST` with: `const failureDist = (failDistData as any)?.distribution ?? []`
4. For the failure distribution rendering, map API fields (`category`, `count`, `percentage`) to the existing chart format
5. Replace `INSIGHTS_STATS` with derived values from failure distribution:
```tsx
const insightsStats = [
  { label: 'Critical', value: failureDist.filter((d: any) => d.category === 'Thermal' || d.category === 'Electrical').reduce((s: number, d: any) => s + d.count, 0), status: 'critical' },
  { label: 'Major', value: failureDist.filter((d: any) => d.category === 'Mechanical').reduce((s: number, d: any) => s + d.count, 0), status: 'warning' },
  { label: 'Minor', value: failureDist.filter((d: any) => d.category === 'Software' || d.category === 'Other').reduce((s: number, d: any) => s + d.count, 0), status: 'info' },
]
```
6. For `FALLBACK_ASSETS`: keep using `usePredictionsByAsset()` which already exists. The RUL data enhancement (adding rul_days to each asset card) can use `useAssetRUL()` per asset, or just display the prediction `expires_at` field as a proxy. For simplicity, use the existing predictions and display `severity` as the urgency indicator.
7. Delete the hardcoded constants
8. Keep `AI_MESSAGES` hardcoded (explicitly out of scope per spec)

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/PredictiveHub.tsx
git commit -m "feat: connect PredictiveHub to failure distribution and prediction APIs"
```

---

## Task 6: Connect ComponentUpgradeRecommendations Page

**Files:**
- Modify: `cmdb-demo/src/pages/ComponentUpgradeRecommendations.tsx`

- [ ] **Step 1: Read the current file**

Find `initialCards` — hardcoded array of 3-4 upgrade recommendation cards.

- [ ] **Step 2: Replace with API data + add asset selector**

1. Import: `import { useUpgradeRecommendations, useAcceptUpgradeRecommendation } from '../hooks/useAssets'` and `import { useAssets } from '../hooks/useAssets'`
2. Add asset selector state: `const [selectedAssetId, setSelectedAssetId] = useState('')`
3. Fetch assets for dropdown: `const { data: assetsData } = useAssets({ type: 'server' })`
4. Fetch recommendations: `const { data: recData } = useUpgradeRecommendations(selectedAssetId)`
5. Accept mutation: `const acceptMutation = useAcceptUpgradeRecommendation()`
6. Map API recommendations to the card format:
```tsx
const cards = ((recData as any)?.recommendations ?? []).map((r: any) => ({
  id: r.id,
  category: r.category.toUpperCase(),
  filterKey: r.category.toUpperCase(),
  title: r.recommendation,
  rcmLevel: `RCM ${r.priority.toUpperCase()}`,
  current: r.current_spec,
  recommended: r.recommendation,
  metric: r.metric_name,
  metricValue: `${r.avg_value}% avg (threshold: ${r.threshold}%)`,
  costPerNode: r.cost_estimate ?? '-',
  selected: false,
}))
```
7. Add asset selector dropdown at the top of the page:
```tsx
<select value={selectedAssetId} onChange={e => setSelectedAssetId(e.target.value)}
  className="bg-[#0d1117] text-white p-2 rounded border border-gray-700">
  <option value="">Select asset to analyze</option>
  {((assetsData as any)?.data ?? []).map((a: any) => (
    <option key={a.id} value={a.id}>{a.name} ({a.type})</option>
  ))}
</select>
```
8. Wire "Request Upgrade" button:
```tsx
onClick={() => acceptMutation.mutate({ assetId: selectedAssetId, category: card.category.toLowerCase() })}
```
9. Show success message when work order created
10. Delete `initialCards` hardcoded constant
11. If no asset selected, show "Select an asset to view upgrade recommendations"

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/ComponentUpgradeRecommendations.tsx
git commit -m "feat: connect ComponentUpgradeRecommendations to real API with asset selector"
```

---

## Task 7: Build Verification + Smoke Test

- [ ] **Step 1: Go build**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

- [ ] **Step 2: TypeScript check**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit 2>&1 | grep -E "PredictiveHub|ComponentUpgrade|usePrediction|useAssets|prediction.ts|assets.ts" | head -10
```

Expected: No errors in changed files.

- [ ] **Step 3: Restart and test endpoints**

```bash
# Restart cmdb-core
kill $(lsof -t -i:8080) 2>/dev/null; sleep 1
cd /cmdb-platform/cmdb-core && DATABASE_URL="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" NATS_URL="nats://localhost:4222" REDIS_URL="redis://localhost:6379/0" JWT_SECRET="changeme" nohup ./server > /tmp/cmdb-core.log 2>&1 &
sleep 3

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('access_token',''))")

echo "=== RUL ==="
curl -s "http://localhost:8080/api/v1/prediction/rul/f0000000-0000-0000-0000-000000000001" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo "=== Failure Distribution ==="
curl -s "http://localhost:8080/api/v1/prediction/failure-distribution" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo "=== Upgrade Rules ==="
curl -s "http://localhost:8080/api/v1/upgrade-rules" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo "=== Upgrade Recommendations ==="
curl -s "http://localhost:8080/api/v1/assets/f0000000-0000-0000-0000-000000000001/upgrade-recommendations" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
