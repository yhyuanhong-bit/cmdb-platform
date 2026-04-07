package api

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
)

// GetEnergyBreakdown returns average power consumption grouped by asset type category
// for the last hour.
// GET /energy/breakdown
func (s *APIServer) GetEnergyBreakdown(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			CASE a.type
				WHEN 'server'  THEN 'IT Equipment'
				WHEN 'network' THEN 'IT Equipment'
				WHEN 'storage' THEN 'IT Equipment'
				WHEN 'power'   THEN 'UPS/Power'
				ELSE 'Other'
			END as category,
			COALESCE(avg(m.value), 0) as avg_kw
		FROM metrics m
		JOIN assets a ON m.asset_id = a.id
		WHERE m.tenant_id = $1
		  AND m.name = 'power_kw'
		  AND m.time > now() - interval '1 hour'
		GROUP BY category
	`, tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type categoryRow struct {
		Name  string  `json:"name"`
		AvgKW float64 `json:"avg_kw"`
		Pct   float64 `json:"pct"`
	}

	var categories []categoryRow
	var totalKW float64

	for rows.Next() {
		var row categoryRow
		if err := rows.Scan(&row.Name, &row.AvgKW); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		categories = append(categories, row)
		totalKW += row.AvgKW
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Calculate percentages
	for i := range categories {
		if totalKW > 0 {
			categories[i].Pct = math.Round(categories[i].AvgKW/totalKW*10000) / 100
		}
	}

	if categories == nil {
		categories = []categoryRow{}
	}

	c.JSON(200, gin.H{
		"categories": categories,
		"total_kw":   math.Round(totalKW*100) / 100,
	})
}

// GetEnergySummary returns PUE, total power, peak demand, and carbon footprint estimate.
// GET /energy/summary
func (s *APIServer) GetEnergySummary(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Latest PUE
	var pue float64
	_ = s.pool.QueryRow(c.Request.Context(), `
		SELECT COALESCE(value, 1.0) FROM metrics
		WHERE tenant_id = $1 AND name = 'pue'
		ORDER BY time DESC LIMIT 1
	`, tenantID).Scan(&pue)
	if pue == 0 {
		pue = 1.0
	}

	// Total power (last hour avg)
	var totalKW float64
	_ = s.pool.QueryRow(c.Request.Context(), `
		SELECT COALESCE(avg(value), 0) FROM metrics
		WHERE tenant_id = $1 AND name = 'power_kw'
		  AND time > now() - interval '1 hour'
	`, tenantID).Scan(&totalKW)

	// Peak demand (max in last 30 days)
	var peakKW float64
	_ = s.pool.QueryRow(c.Request.Context(), `
		SELECT COALESCE(max(value), 0) FROM metrics
		WHERE tenant_id = $1 AND name = 'power_kw'
		  AND time > now() - interval '30 days'
	`, tenantID).Scan(&peakKW)

	// Carbon footprint estimate: kW * 24h * 30days * emission_factor (tCO2/kWh)
	carbonMT := totalKW * 24 * 30 * 0.0005 // rough monthly estimate in metric tonnes

	c.JSON(200, gin.H{
		"pue":               math.Round(pue*1000) / 1000,
		"total_kw":          math.Round(totalKW*100) / 100,
		"peak_kw":           math.Round(peakKW*100) / 100,
		"carbon_mt_monthly": math.Round(carbonMT*100) / 100,
	})
}

// GetEnergyTrend returns hourly aggregated power consumption over the requested window.
// GET /energy/trend?hours=24&granularity=hourly
func (s *APIServer) GetEnergyTrend(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	hours := c.DefaultQuery("hours", "24")

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT date_trunc('hour', time) as hour,
		       COALESCE(sum(value), 0) as total_kw
		FROM metrics
		WHERE tenant_id = $1 AND name = 'power_kw'
		  AND time > now() - ($2 || ' hours')::interval
		GROUP BY hour
		ORDER BY hour
	`, tenantID, hours)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type trendPoint struct {
		Hour    time.Time `json:"hour"`
		TotalKW float64   `json:"total_kw"`
	}

	var points []trendPoint
	for rows.Next() {
		var p trendPoint
		if err := rows.Scan(&p.Hour, &p.TotalKW); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		p.TotalKW = math.Round(p.TotalKW*100) / 100
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if points == nil {
		points = []trendPoint{}
	}

	c.JSON(200, gin.H{"trend": points})
}
