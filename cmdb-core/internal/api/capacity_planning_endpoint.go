package api

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// CapacityForecast represents a single capacity prediction.
type CapacityForecast struct {
	ResourceType     string  `json:"resource_type"`
	ResourceName     string  `json:"resource_name"`
	CurrentUsage     float64 `json:"current_usage"`
	CurrentCapacity  float64 `json:"current_capacity"`
	UsagePercent     float64 `json:"usage_percent"`
	MonthlyGrowth    float64 `json:"monthly_growth"`
	ThresholdPercent float64 `json:"threshold_percent"`
	MonthsUntilFull  *int    `json:"months_until_full"`
	Trend            string  `json:"trend"` // rising, stable, declining
	Severity         string  `json:"severity"` // critical, warning, ok
	Recommendation   string  `json:"recommendation"`
}

// linearRegression computes slope from monthly data points.
// Returns slope (change per month) and R² goodness of fit.
func linearRegression(values []float64) (slope float64, r2 float64) {
	n := float64(len(values))
	if n < 2 {
		return 0, 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	// R²
	var ssRes, ssTot float64
	meanY := sumY / n
	for i, y := range values {
		predicted := slope*float64(i) + intercept
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}

	return slope, r2
}

func capacityTrendLabel(slope float64) string {
	if slope > 1 {
		return "rising"
	} else if slope < -1 {
		return "declining"
	}
	return "stable"
}

func capacitySeverity(usagePct float64, monthsUntil *int) string {
	if usagePct >= 90 || (monthsUntil != nil && *monthsUntil <= 1) {
		return "critical"
	}
	if usagePct >= 75 || (monthsUntil != nil && *monthsUntil <= 3) {
		return "warning"
	}
	return "ok"
}

// GetCapacityPlanning returns capacity forecasts for infrastructure and device metrics.
// GET /api/v1/capacity-planning
func (s *APIServer) GetCapacityPlanning(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()

	var forecasts []CapacityForecast

	// ============================================================
	// 1. Infrastructure Layer (from CMDB data, no external monitoring)
	// ============================================================

	// 1a. Rack U-slot capacity — derive used_u from rack_slots
	var totalU float64
	s.pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(total_u), 0) FROM racks WHERE tenant_id = $1 AND deleted_at IS NULL",
		tenantID).Scan(&totalU)

	var usedU float64
	s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(rs.end_u - rs.start_u + 1), 0)
		 FROM rack_slots rs
		 JOIN racks r ON r.id = rs.rack_id
		 WHERE r.tenant_id = $1 AND r.deleted_at IS NULL`,
		tenantID).Scan(&usedU)

	if totalU > 0 {
		pct := usedU / totalU * 100
		// Get monthly asset-creation rate (new assets per month over last 6 months)
		var monthlyData []float64
		rows, _ := s.pool.Query(ctx,
			`SELECT date_trunc('month', created_at) AS m, COUNT(*)
			 FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL AND created_at > now() - interval '6 months'
			 GROUP BY m ORDER BY m`, tenantID)
		if rows != nil {
			for rows.Next() {
				var t time.Time
				var cnt float64
				rows.Scan(&t, &cnt)
				monthlyData = append(monthlyData, cnt)
			}
			rows.Close()
		}
		slope, _ := linearRegression(monthlyData)
		// Estimate U growth: assume avg 2U per new asset
		uGrowthPerMonth := slope * 2
		var monthsUntil *int
		if uGrowthPerMonth > 0 {
			remaining := totalU - usedU
			m := int(remaining / uGrowthPerMonth)
			if m >= 0 && m < 120 {
				monthsUntil = &m
			}
		}
		forecasts = append(forecasts, CapacityForecast{
			ResourceType:     "infrastructure",
			ResourceName:     "Rack U-Slots",
			CurrentUsage:     usedU,
			CurrentCapacity:  totalU,
			UsagePercent:     math.Round(pct*10) / 10,
			MonthlyGrowth:    math.Round(uGrowthPerMonth*10) / 10,
			ThresholdPercent: 85,
			MonthsUntilFull:  monthsUntil,
			Trend:            capacityTrendLabel(uGrowthPerMonth),
			Severity:         capacitySeverity(pct, monthsUntil),
			Recommendation:   rackRecommendation(pct, monthsUntil),
		})
	}

	// 1b. Power capacity — only total_capacity is available; skip current if no sensor data
	var totalPower float64
	s.pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(power_capacity_kw), 0) FROM racks WHERE tenant_id = $1 AND deleted_at IS NULL",
		tenantID).Scan(&totalPower)

	if totalPower > 0 {
		// No power_current_kw column in racks; report capacity headroom only
		forecasts = append(forecasts, CapacityForecast{
			ResourceType:     "infrastructure",
			ResourceName:     "Power Capacity (kW)",
			CurrentUsage:     0,
			CurrentCapacity:  math.Round(totalPower*100) / 100,
			UsagePercent:     0,
			ThresholdPercent: 80,
			Trend:            "stable",
			Severity:         "ok",
			Recommendation:   "Power capacity configured. Connect power monitoring sensors to track live utilization.",
		})
	}

	// 1c. Asset growth trend
	var totalAssets float64
	s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL",
		tenantID).Scan(&totalAssets)

	var assetMonthly []float64
	rows2, _ := s.pool.Query(ctx,
		`SELECT date_trunc('month', created_at) AS m, COUNT(*)
		 FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL AND created_at > now() - interval '6 months'
		 GROUP BY m ORDER BY m`, tenantID)
	if rows2 != nil {
		for rows2.Next() {
			var t time.Time
			var cnt float64
			rows2.Scan(&t, &cnt)
			assetMonthly = append(assetMonthly, cnt)
		}
		rows2.Close()
	}
	assetSlope, _ := linearRegression(assetMonthly)
	forecasts = append(forecasts, CapacityForecast{
		ResourceType:     "infrastructure",
		ResourceName:     "Total Assets",
		CurrentUsage:     totalAssets,
		CurrentCapacity:  totalAssets,
		UsagePercent:     0,
		MonthlyGrowth:    math.Round(assetSlope*10) / 10,
		ThresholdPercent: 0,
		Trend:            capacityTrendLabel(assetSlope),
		Severity:         "ok",
		Recommendation:   assetGrowthRecommendation(assetSlope),
	})

	// ============================================================
	// 2. Device Layer (from metrics table, needs Prometheus/Zabbix)
	// ============================================================

	metricTypes := []struct {
		name      string
		label     string
		threshold float64
	}{
		{"cpu_usage", "Fleet CPU Usage", 85},
		{"memory_usage", "Fleet Memory Usage", 85},
		{"disk_usage", "Fleet Disk Usage", 90},
	}

	for _, mt := range metricTypes {
		var monthly []float64
		mrows, _ := s.pool.Query(ctx,
			`SELECT date_trunc('month', time) AS m, avg(value)
			 FROM metrics WHERE tenant_id = $1 AND name = $2 AND time > now() - interval '6 months'
			 GROUP BY m ORDER BY m`, tenantID, mt.name)
		if mrows != nil {
			for mrows.Next() {
				var t time.Time
				var avg float64
				mrows.Scan(&t, &avg)
				monthly = append(monthly, avg)
			}
			mrows.Close()
		}

		if len(monthly) < 2 {
			continue
		}

		currentAvg := monthly[len(monthly)-1]
		slope, _ := linearRegression(monthly)
		var monthsUntil *int
		if slope > 0 {
			remaining := mt.threshold - currentAvg
			if remaining > 0 {
				m := int(remaining / slope)
				if m >= 0 && m < 120 {
					monthsUntil = &m
				}
			}
		}

		forecasts = append(forecasts, CapacityForecast{
			ResourceType:     "device",
			ResourceName:     mt.label,
			CurrentUsage:     math.Round(currentAvg*10) / 10,
			CurrentCapacity:  100,
			UsagePercent:     math.Round(currentAvg*10) / 10,
			MonthlyGrowth:    math.Round(slope*100) / 100,
			ThresholdPercent: mt.threshold,
			MonthsUntilFull:  monthsUntil,
			Trend:            capacityTrendLabel(slope),
			Severity:         capacitySeverity(currentAvg, monthsUntil),
			Recommendation:   deviceRecommendation(mt.label, currentAvg, slope, monthsUntil),
		})
	}

	if forecasts == nil {
		forecasts = []CapacityForecast{}
	}

	response.OK(c, forecasts)
}

func rackRecommendation(pct float64, months *int) string {
	if pct >= 90 {
		return "Rack space critically low. Immediate expansion or decommission of unused equipment needed."
	}
	if months != nil && *months <= 3 {
		return "Rack space projected to be full within 3 months. Begin procurement of additional racks."
	}
	if pct >= 75 {
		return "Rack utilization above 75%. Monitor growth and plan expansion for next quarter."
	}
	return "Rack capacity is healthy."
}

func assetGrowthRecommendation(slope float64) string {
	if slope > 10 {
		return "Rapid asset growth detected. Ensure staffing, licensing, and infrastructure keep pace."
	}
	if slope > 5 {
		return "Moderate asset growth. Review capacity planning quarterly."
	}
	return "Asset growth is stable."
}

func deviceRecommendation(label string, current, slope float64, months *int) string {
	if months != nil && *months <= 1 {
		return label + " projected to hit threshold within 1 month. Immediate action required — scale up or redistribute load."
	}
	if months != nil && *months <= 3 {
		return label + " projected to hit threshold within 3 months. Begin capacity expansion planning."
	}
	if slope > 2 {
		return label + " trending upward. Monitor closely and prepare scaling plan."
	}
	return label + " within healthy range."
}
