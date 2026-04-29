package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// parseJSONAttributes unmarshals JSONB bytes into a map[string]string,
// extracting only string-typed values.
func parseJSONAttributes(data []byte) map[string]string {
	result := map[string]string{}
	if len(data) == 0 {
		return result
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return result
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// GetAssetRUL handles GET /prediction/rul/:id
// Calculates the Remaining Useful Life for an asset based on warranty and expected lifespan.
func (s *APIServer) GetAssetRUL(c *gin.Context, id IdPath) {
	assetID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	var (
		name      string
		assetType string
		attrBytes []byte
		createdAt time.Time
	)
	sc := database.Scope(s.pool, tenantID)
	err := sc.QueryRow(c.Request.Context(), `
		SELECT name, type, attributes, created_at
		FROM assets
		WHERE id = $2 AND tenant_id = $1
	`, assetID).Scan(&name, &assetType, &attrBytes, &createdAt)
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}

	attrs := parseJSONAttributes(attrBytes)

	now := time.Now().UTC()

	// Parse purchase_date
	purchaseDate := attrs["purchase_date"]
	var ageDays int64
	if purchaseDate != "" {
		if pd, err := time.Parse("2006-01-02", purchaseDate); err == nil {
			ageDays = int64(now.Sub(pd).Hours() / 24)
		}
	} else {
		ageDays = int64(now.Sub(createdAt).Hours() / 24)
		purchaseDate = createdAt.Format("2006-01-02")
	}

	// Parse warranty_expiry
	warrantyExpiry := attrs["warranty_expiry"]
	var warrantyRemainingDays int64
	if warrantyExpiry != "" {
		if we, err := time.Parse("2006-01-02", warrantyExpiry); err == nil {
			warrantyRemainingDays = int64(we.Sub(now).Hours() / 24)
		}
	}

	// W3.2-backend: lifespan is now a per-tenant setting. The settings
	// service falls back to the same canonical defaults this map used
	// to hardcode (server=5, network=7, storage=5, power=10) for any
	// tenant that has not customised the value, so behaviour is
	// preserved by default. Substring matching against assetType is
	// done inside cfg.GetForType, mirroring the previous map's
	// strings.Contains lookup so "rack_server" still picks the server
	// value.
	expectedLifespanYears := int64(5)
	if s.settingsSvc != nil {
		if cfg, lifespanErr := s.settingsSvc.GetAssetLifespan(c.Request.Context(), tenantID); lifespanErr == nil {
			expectedLifespanYears = cfg.GetForType(assetType)
		}
	}
	lifespanDays := expectedLifespanYears * 365
	lifespanRemainingDays := lifespanDays - ageDays

	// RUL = min(warranty_remaining, lifespan_remaining)
	rulDays := warrantyRemainingDays
	if lifespanRemainingDays < rulDays {
		rulDays = lifespanRemainingDays
	}

	// Determine status
	rulStatus := "expired"
	switch {
	case rulDays > 365:
		rulStatus = "healthy"
	case rulDays >= 90:
		rulStatus = "warning"
	case rulDays > 0:
		rulStatus = "critical"
	}

	response.OK(c, gin.H{
		"asset_id":                assetID,
		"asset_name":              name,
		"purchase_date":           purchaseDate,
		"warranty_expiry":         warrantyExpiry,
		"expected_lifespan_years": expectedLifespanYears,
		"age_days":                ageDays,
		"rul_days":                rulDays,
		"rul_status":              rulStatus,
		"warranty_remaining_days": warrantyRemainingDays,
	})
}

// GetFailureDistribution handles GET /prediction/failure-distribution
// Returns failure category distribution based on alerts and work orders from the last 90 days.
func (s *APIServer) GetFailureDistribution(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	// Classify a text string into a failure category.
	classify := func(text string) string {
		t := strings.ToLower(text)
		switch {
		case strings.Contains(t, "temperature") || strings.Contains(t, "temp") || strings.Contains(t, "thermal"):
			return "Thermal"
		case strings.Contains(t, "power") || strings.Contains(t, "voltage") || strings.Contains(t, "electrical") || strings.Contains(t, "pdu") || strings.Contains(t, "ups"):
			return "Electrical"
		case strings.Contains(t, "disk") || strings.Contains(t, "fan") || strings.Contains(t, "vibration") || strings.Contains(t, "hardware"):
			return "Mechanical"
		case strings.Contains(t, "cpu") || strings.Contains(t, "memory") || strings.Contains(t, "software") || strings.Contains(t, "firmware"):
			return "Software"
		default:
			return "Other"
		}
	}

	counts := map[string]int64{
		"Thermal":    0,
		"Electrical": 0,
		"Mechanical": 0,
		"Software":   0,
		"Other":      0,
	}

	sc := database.Scope(s.pool, tenantID)

	// Query alert_events for last 90 days
	alertRows, err := sc.Query(c.Request.Context(), `
		SELECT message
		FROM alert_events
		WHERE tenant_id = $1
		  AND fired_at > now() - INTERVAL '90 days'
	`)
	if err != nil {
		response.InternalError(c, "failed to query alert events")
		return
	}
	defer alertRows.Close()

	for alertRows.Next() {
		var message string
		if scanErr := alertRows.Scan(&message); scanErr != nil {
			continue
		}
		cat := classify(message)
		counts[cat]++
	}
	alertRows.Close()

	// Query work_orders for last 90 days
	woRows, err := sc.Query(c.Request.Context(), `
		SELECT type, title
		FROM work_orders
		WHERE tenant_id = $1
		  AND created_at > now() - INTERVAL '90 days'
	`)
	if err != nil {
		response.InternalError(c, "failed to query work orders")
		return
	}
	defer woRows.Close()

	for woRows.Next() {
		var woType, woTitle string
		if err := woRows.Scan(&woType, &woTitle); err != nil {
			continue
		}
		var cat string
		switch strings.ToLower(woType) {
		case "replacement":
			cat = "Mechanical"
		case "upgrade":
			cat = "Software"
		default:
			cat = classify(woTitle)
		}
		counts[cat]++
	}
	woRows.Close()

	var total int64
	for _, v := range counts {
		total += v
	}

	type distItem struct {
		Category   string  `json:"category"`
		Count      int64   `json:"count"`
		Percentage float64 `json:"percentage"`
	}
	categories := []string{"Thermal", "Electrical", "Mechanical", "Software", "Other"}
	distribution := make([]distItem, 0, len(categories))
	for _, cat := range categories {
		cnt := counts[cat]
		pct := 0.0
		if total > 0 {
			pct = float64(cnt) / float64(total) * 100
		}
		distribution = append(distribution, distItem{
			Category:   cat,
			Count:      cnt,
			Percentage: pct,
		})
	}

	response.OK(c, gin.H{
		"distribution": distribution,
		"total":        total,
		"period_days":  90,
	})
}

// GetAssetUpgradeRecommendations handles GET /assets/:id/upgrade-recommendations
// Returns upgrade recommendations for an asset based on metric thresholds.
// Enhancements: EOL/warranty filter, P95 check, BIA priority boost, cost estimate, warranty warning.
func (s *APIServer) GetAssetUpgradeRecommendations(c *gin.Context, id IdPath) {
	assetID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	// Get asset type, attributes, lifecycle fields, BIA level, and model
	var assetType string
	var assetModel sql.NullString
	var attrBytes []byte
	var biaLevel string
	var eolDate sql.NullTime
	var warrantyEnd sql.NullTime

	sc := database.Scope(s.pool, tenantID)
	err := sc.QueryRow(c.Request.Context(), `
		SELECT type, attributes, COALESCE(bia_level, 'normal'), eol_date, warranty_end, model
		FROM assets WHERE id = $2 AND tenant_id = $1
	`, assetID).Scan(&assetType, &attrBytes, &biaLevel, &eolDate, &warrantyEnd, &assetModel)
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	attrs := parseJSONAttributes(attrBytes)

	// 2a. Filter: don't recommend upgrades for assets approaching end-of-life (within 12 months)
	if eolDate.Valid && eolDate.Time.Before(time.Now().AddDate(0, 12, 0)) {
		response.OK(c, gin.H{
			"recommendations":  []struct{}{},
			"skip_reason":      "Asset approaching end-of-life — upgrade not recommended",
			"eol_date":         eolDate.Time.Format("2006-01-02"),
			"warranty_warning": "",
			"bia_level":        biaLevel,
		})
		return
	}

	// Load upgrade rules for this asset_type and tenant
	ruleRows, err := sc.Query(c.Request.Context(), `
		SELECT id, category, metric_name, threshold, duration_days, priority, recommendation
		FROM upgrade_rules
		WHERE tenant_id = $1 AND asset_type = $2 AND enabled = true
	`, assetType)
	if err != nil {
		response.InternalError(c, "failed to query upgrade rules")
		return
	}
	defer ruleRows.Close()

	type recommendation struct {
		ID             string   `json:"id"`
		Category       string   `json:"category"`
		Priority       string   `json:"priority"`
		CurrentSpec    string   `json:"current_spec"`
		Recommendation string   `json:"recommendation"`
		MetricName     string   `json:"metric_name"`
		AvgValue       float64  `json:"avg_value"`
		P95Value       float64  `json:"p95_value"`
		Threshold      float64  `json:"threshold"`
		DurationDays   int      `json:"duration_days"`
		CostEstimate   *float64 `json:"cost_estimate"`
		Alternatives   []string `json:"alternatives"`
	}

	type ruleRow struct {
		id             uuid.UUID
		category       string
		metricName     string
		threshold      float64
		durationDays   int
		priority       string
		recommendation string
	}

	var rules []ruleRow
	for ruleRows.Next() {
		var r ruleRow
		if err := ruleRows.Scan(&r.id, &r.category, &r.metricName, &r.threshold, &r.durationDays, &r.priority, &r.recommendation); err != nil {
			continue
		}
		rules = append(rules, r)
	}
	ruleRows.Close()

	// 2d. Cost estimates by category
	costEstimates := map[string]float64{
		"cpu":     5000,
		"memory":  2000,
		"storage": 3000,
		"network": 8000,
	}

	var recommendations []recommendation
	for _, r := range rules {
		// Query average metric value over duration_days
		var avgValue float64
		err := sc.QueryRow(c.Request.Context(), `
			SELECT COALESCE(avg(value), 0)
			FROM metrics
			WHERE tenant_id = $1
			  AND asset_id = $2
			  AND name = $3
			  AND time > now() - ($4 || ' days')::interval
		`, assetID, r.metricName, r.durationDays).Scan(&avgValue)
		if err != nil {
			continue
		}

		// 2b. P95 check alongside average
		var p95Value float64
		sc.QueryRow(c.Request.Context(), `
			SELECT COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY value), 0)
			FROM metrics
			WHERE tenant_id = $1 AND asset_id = $2 AND name = $3 AND time > now() - ($4 || ' days')::interval
		`, assetID, r.metricName, r.durationDays).Scan(&p95Value)

		// Trigger if avg > threshold OR p95 > threshold * 1.1
		triggered := avgValue > r.threshold || p95Value > r.threshold*1.1
		if triggered {
			currentSpec := attrs[r.category]
			if currentSpec == "" {
				currentSpec = attrs[r.metricName]
			}
			var costEstimate *float64
			if cost, ok := costEstimates[r.category]; ok {
				c2 := cost
				costEstimate = &c2
			}
			recommendations = append(recommendations, recommendation{
				ID:             r.id.String(),
				Category:       r.category,
				Priority:       r.priority,
				CurrentSpec:    currentSpec,
				Recommendation: r.recommendation,
				MetricName:     r.metricName,
				AvgValue:       avgValue,
				P95Value:       p95Value,
				Threshold:      r.threshold,
				DurationDays:   r.durationDays,
				CostEstimate:   costEstimate,
			})
		}
	}

	// 2c. BIA-based priority boost
	if biaLevel == "critical" || biaLevel == "important" {
		priorityUp := map[string]string{
			"medium": "high",
			"low":    "medium",
		}
		for i := range recommendations {
			if upgraded, ok := priorityUp[recommendations[i].Priority]; ok {
				recommendations[i].Priority = upgraded
			}
			recommendations[i].Recommendation += " [BIA: " + biaLevel + " — prioritized]"
		}
	}

	// Item 7: Composite load score — weighted combination of CPU, RAM, disk metrics
	var cpuAvg, memAvg, diskAvg float64
	if err := sc.QueryRow(c.Request.Context(),
		"SELECT COALESCE(avg(value), 0) FROM metrics WHERE tenant_id = $1 AND asset_id = $2 AND name = 'cpu_usage' AND time > now() - interval '7 days'",
		assetID).Scan(&cpuAvg); err != nil {
		zap.L().Error("prediction: failed to query cpu_usage", zap.Error(err))
	}
	if err := sc.QueryRow(c.Request.Context(),
		"SELECT COALESCE(avg(value), 0) FROM metrics WHERE tenant_id = $1 AND asset_id = $2 AND name = 'memory_usage' AND time > now() - interval '7 days'",
		assetID).Scan(&memAvg); err != nil {
		zap.L().Error("prediction: failed to query memory_usage", zap.Error(err))
	}
	if err := sc.QueryRow(c.Request.Context(),
		"SELECT COALESCE(avg(value), 0) FROM metrics WHERE tenant_id = $1 AND asset_id = $2 AND name = 'disk_usage' AND time > now() - interval '7 days'",
		assetID).Scan(&diskAvg); err != nil {
		zap.L().Error("prediction: failed to query disk_usage", zap.Error(err))
	}

	loadScore := cpuAvg*0.4 + memAvg*0.35 + diskAvg*0.25
	if loadScore > 75 && cpuAvg > 0 {
		recommendations = append(recommendations, recommendation{
			ID:             "load-score-composite",
			Category:       "overall",
			Priority:       "high",
			CurrentSpec:    fmt.Sprintf("CPU: %.0f%%, RAM: %.0f%%, Disk: %.0f%%", cpuAvg, memAvg, diskAvg),
			Recommendation: fmt.Sprintf("Composite load score %.0f%% exceeds 75%%. Consider migrating workloads, scaling horizontally, or upgrading multiple components.", loadScore),
			MetricName:     "composite_load",
			AvgValue:       loadScore,
			Threshold:      75,
			DurationDays:   7,
		})
	}

	// Item 8: Peer comparison — annotate recommendations where this asset is an outlier vs same-model peers
	if assetModel.Valid && assetModel.String != "" {
		for i := range recommendations {
			var peerAvg float64
			var peerCount int
			if err := sc.QueryRow(c.Request.Context(),
				`SELECT COALESCE(avg(m.value), 0), COUNT(DISTINCT m.asset_id)
				 FROM metrics m JOIN assets a ON m.asset_id = a.id
				 WHERE a.tenant_id = $1 AND a.model = $2 AND a.id != $3
				   AND m.name = $4 AND m.time > now() - interval '7 days'
				   AND a.deleted_at IS NULL`,
				assetModel.String, assetID, recommendations[i].MetricName,
			).Scan(&peerAvg, &peerCount); err != nil {
				zap.L().Error("prediction: failed to query peer metrics", zap.String("metric", recommendations[i].MetricName), zap.Error(err))
			}

			if peerCount >= 3 && peerAvg > 0 {
				ratio := recommendations[i].AvgValue / peerAvg
				if ratio > 1.5 {
					recommendations[i].Recommendation += fmt.Sprintf(
						" [Outlier: this asset %.0f%% vs peer avg %.0f%% across %d similar %s — investigate application issues before hardware upgrade]",
						recommendations[i].AvgValue, peerAvg, peerCount, assetModel.String,
					)
				}
			}
		}
	}

	// Item 9: Alternative recommendations per category
	alternativesMap := map[string][]string{
		"cpu": {
			"Migrate CPU-intensive workloads to less loaded servers",
			"Optimize application code or database queries",
			"Add a server for horizontal load balancing",
		},
		"memory": {
			"Identify and fix memory leaks in applications",
			"Reduce cache sizes or implement eviction policies",
			"Migrate memory-intensive services to dedicated nodes",
		},
		"storage": {
			"Archive or delete old data and logs",
			"Implement data compression",
			"Migrate cold data to cheaper storage tier",
		},
		"network": {
			"Implement QoS and traffic shaping",
			"Optimize application protocols (compression, batching)",
			"Add redundant network links for load distribution",
		},
		"overall": {
			"Redistribute workloads across the fleet",
			"Review application architecture for optimization opportunities",
			"Plan horizontal scaling with additional servers",
		},
	}
	for i := range recommendations {
		if alts, ok := alternativesMap[recommendations[i].Category]; ok {
			recommendations[i].Alternatives = alts
		}
	}

	// Sort by priority: critical > high > medium > low
	priorityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	for i := 0; i < len(recommendations); i++ {
		for j := i + 1; j < len(recommendations); j++ {
			pi := priorityOrder[recommendations[i].Priority]
			pj := priorityOrder[recommendations[j].Priority]
			if pj < pi {
				recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
			}
		}
	}

	if recommendations == nil {
		recommendations = []recommendation{}
	}

	// 2e. Warranty warning
	var warrantyWarning string
	if warrantyEnd.Valid && warrantyEnd.Time.Before(time.Now().AddDate(0, 6, 0)) {
		warrantyWarning = fmt.Sprintf("Warning: asset warranty expires %s — consider ROI before upgrading", warrantyEnd.Time.Format("2006-01-02"))
	}

	response.OK(c, gin.H{
		"recommendations":  recommendations,
		"warranty_warning": warrantyWarning,
		"bia_level":        biaLevel,
	})
}

// AcceptUpgradeRecommendation handles POST /assets/:id/upgrade-recommendations/:category/accept
// Creates a work order for the accepted upgrade recommendation.
func (s *APIServer) AcceptUpgradeRecommendation(c *gin.Context, id IdPath, category string) {
	assetUUID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)

	sc := database.Scope(s.pool, tenantID)

	// Get the matching rule for this asset + category
	var ruleRecommendation string
	var assetType string
	err := sc.QueryRow(c.Request.Context(), `
		SELECT ur.recommendation, a.type
		FROM upgrade_rules ur
		JOIN assets a ON a.type = ur.asset_type AND a.tenant_id = ur.tenant_id
		WHERE a.id = $2 AND ur.category = $3 AND ur.tenant_id = $1 AND ur.enabled = true
		LIMIT 1
	`, assetUUID, category).Scan(&ruleRecommendation, &assetType)
	if err != nil {
		response.NotFound(c, "no matching upgrade rule found")
		return
	}

	// Create work order via service layer
	title := fmt.Sprintf("Upgrade %s %s: %s", assetType, category, ruleRecommendation)

	order, err := s.maintenanceSvc.Create(c.Request.Context(), tenantID, userID, maintenance.CreateOrderRequest{
		Title:    title,
		Type:     "upgrade",
		Priority: "medium",
		AssetID:  &assetUUID,
	})
	if err != nil {
		response.InternalError(c, "failed to create work order")
		return
	}

	response.Created(c, gin.H{
		"work_order_id": order.ID.String(),
		"code":          order.Code,
	})
}

// GetUpgradeRules handles GET /upgrade-rules
// Returns all upgrade rules for the current tenant.
func (s *APIServer) GetUpgradeRules(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	sc := database.Scope(s.pool, tenantID)

	rows, err := sc.Query(c.Request.Context(), `
		SELECT id, asset_type, category, metric_name, threshold, duration_days, priority, recommendation, enabled
		FROM upgrade_rules
		WHERE tenant_id = $1
		ORDER BY asset_type, category
	`)
	if err != nil {
		response.InternalError(c, "failed to query upgrade rules")
		return
	}
	defer rows.Close()

	type upgradeRule struct {
		ID             string  `json:"id"`
		AssetType      string  `json:"asset_type"`
		Category       string  `json:"category"`
		MetricName     string  `json:"metric_name"`
		Threshold      float64 `json:"threshold"`
		DurationDays   int     `json:"duration_days"`
		Priority       string  `json:"priority"`
		Recommendation string  `json:"recommendation"`
		Enabled        bool    `json:"enabled"`
	}

	rules := []upgradeRule{}
	for rows.Next() {
		var r upgradeRule
		if err := rows.Scan(&r.ID, &r.AssetType, &r.Category, &r.MetricName, &r.Threshold, &r.DurationDays, &r.Priority, &r.Recommendation, &r.Enabled); err != nil {
			continue
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "error reading upgrade rules")
		return
	}

	response.OK(c, gin.H{"rules": rules})
}

// CreateUpgradeRule handles POST /upgrade-rules
// Creates a new upgrade rule for the current tenant.
func (s *APIServer) CreateUpgradeRule(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	sc := database.Scope(s.pool, tenantID)

	var body struct {
		AssetType      string  `json:"asset_type" binding:"required"`
		Category       string  `json:"category" binding:"required"`
		MetricName     string  `json:"metric_name"`
		Threshold      float64 `json:"threshold"`
		DurationDays   int     `json:"duration_days"`
		Priority       string  `json:"priority"`
		Recommendation string  `json:"recommendation"`
		Enabled        *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Apply defaults
	if body.DurationDays == 0 {
		body.DurationDays = 7
	}
	if body.Priority == "" {
		body.Priority = "medium"
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	newID := uuid.New()
	_, err := sc.Exec(c.Request.Context(), `
		INSERT INTO upgrade_rules (id, tenant_id, asset_type, category, metric_name, threshold, duration_days, priority, recommendation, enabled, created_at, updated_at)
		VALUES ($2, $1, $3, $4, $5, $6, $7, $8, $9, $10, now(), now())
	`, newID, body.AssetType, body.Category, body.MetricName, body.Threshold, body.DurationDays, body.Priority, body.Recommendation, enabled)
	if err != nil {
		response.InternalError(c, "failed to create upgrade rule")
		return
	}

	s.recordAudit(c, "upgrade_rule.created", "prediction", "upgrade_rule", newID, map[string]any{
		"asset_type": body.AssetType,
		"category":   body.Category,
		"priority":   body.Priority,
	})
	response.Created(c, gin.H{"id": newID.String()})
}

// UpdateUpgradeRule handles PUT /upgrade-rules/{id}
// Updates an existing upgrade rule (threshold, duration, priority, recommendation, enabled).
func (s *APIServer) UpdateUpgradeRule(c *gin.Context, id IdPath) {
	ruleID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)
	var body UpdateUpgradeRuleJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	var priority *string
	if body.Priority != nil {
		p := string(*body.Priority)
		priority = &p
	}
	sc := database.Scope(s.pool, tenantID)
	tag, err := sc.Exec(c.Request.Context(), `
		UPDATE upgrade_rules SET
		  threshold      = COALESCE($3, threshold),
		  duration_days  = COALESCE($4, duration_days),
		  priority       = COALESCE($5, priority),
		  recommendation = COALESCE($6, recommendation),
		  enabled        = COALESCE($7, enabled),
		  updated_at     = now()
		WHERE id = $2 AND tenant_id = $1
	`, ruleID, body.Threshold, body.DurationDays, priority, body.Recommendation, body.Enabled)
	if err != nil {
		response.InternalError(c, "failed to update upgrade rule")
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "upgrade rule not found")
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// DeleteUpgradeRule handles DELETE /upgrade-rules/{id}
// Deletes an upgrade rule.
func (s *APIServer) DeleteUpgradeRule(c *gin.Context, id IdPath) {
	ruleID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)
	sc := database.Scope(s.pool, tenantID)
	tag, err := sc.Exec(c.Request.Context(), "DELETE FROM upgrade_rules WHERE id = $2 AND tenant_id = $1", ruleID)
	if err != nil {
		zap.L().Error("failed to delete upgrade rule", zap.String("id", ruleID.String()), zap.Error(err))
		response.InternalError(c, "failed to delete rule")
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "upgrade rule not found")
		return
	}
	c.Status(204)
}
