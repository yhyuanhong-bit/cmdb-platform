package api

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

const energyQueryTimeout = 10 * time.Second

// GetEnergyBreakdown returns average power consumption grouped by asset type category
// for the last hour.
// GET /energy/breakdown
func (s *APIServer) GetEnergyBreakdown(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), energyQueryTimeout)
	defer cancel()

	rows, err := s.pool.Query(ctx, `
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
		JOIN assets a ON m.asset_id = a.id AND a.deleted_at IS NULL
		WHERE m.tenant_id = $1
		  AND m.name = 'power_kw'
		  AND m.time > now() - interval '1 hour'
		GROUP BY category
	`, tenantID)
	if err != nil {
		response.InternalError(c, err.Error())
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
			response.InternalError(c, err.Error())
			return
		}
		categories = append(categories, row)
		totalKW += row.AvgKW
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, err.Error())
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

	response.OK(c, gin.H{
		"categories": categories,
		"total_kw":   math.Round(totalKW*100) / 100,
	})
}

// GetEnergySummary returns PUE, total power, peak demand, and carbon footprint estimate.
// GET /energy/summary
func (s *APIServer) GetEnergySummary(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), energyQueryTimeout)
	defer cancel()

	// Latest PUE. ErrNoRows is expected (no metric yet) — default to 1.0.
	// Any other error is a DB failure: abort rather than return misleading zeros.
	var pue float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(value, 1.0) FROM metrics
		WHERE tenant_id = $1 AND name = 'pue'
		ORDER BY time DESC LIMIT 1
	`, tenantID).Scan(&pue); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			zap.L().Error("energy: failed to query PUE", zap.Error(err))
			response.InternalError(c, "failed to query energy summary")
			return
		}
	}
	if pue == 0 {
		pue = 1.0
	}

	// Total power (last hour avg). Aggregates always return one row, so any
	// error here indicates a real DB failure.
	var totalKW float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(avg(value), 0) FROM metrics
		WHERE tenant_id = $1 AND name = 'power_kw'
		  AND time > now() - interval '1 hour'
	`, tenantID).Scan(&totalKW); err != nil {
		zap.L().Error("energy: failed to query total power", zap.Error(err))
		response.InternalError(c, "failed to query energy summary")
		return
	}

	// Peak demand (max in last 30 days)
	var peakKW float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(max(value), 0) FROM metrics
		WHERE tenant_id = $1 AND name = 'power_kw'
		  AND time > now() - interval '30 days'
	`, tenantID).Scan(&peakKW); err != nil {
		zap.L().Error("energy: failed to query peak demand", zap.Error(err))
		response.InternalError(c, "failed to query energy summary")
		return
	}

	// Carbon footprint estimate: kW * 24h * 30days * emission_factor (tCO2/kWh)
	carbonMT := totalKW * 24 * 30 * s.cfg.CarbonEmissionFactor // monthly estimate in metric tonnes

	response.OK(c, gin.H{
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
	hoursStr := c.DefaultQuery("hours", "24")
	hoursVal := 24
	if h, err := strconv.Atoi(hoursStr); err == nil && h >= 1 && h <= 168 {
		hoursVal = h
	}
	hours := strconv.Itoa(hoursVal)
	ctx, cancel := context.WithTimeout(c.Request.Context(), energyQueryTimeout)
	defer cancel()

	rows, err := s.pool.Query(ctx, `
		SELECT date_trunc('hour', time) as hour,
		       COALESCE(sum(value), 0) as total_kw
		FROM metrics
		WHERE tenant_id = $1 AND name = 'power_kw'
		  AND time > now() - ($2 || ' hours')::interval
		GROUP BY hour
		ORDER BY hour
	`, tenantID, hours)
	if err != nil {
		response.InternalError(c, err.Error())
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
			response.InternalError(c, err.Error())
			return
		}
		p.TotalKW = math.Round(p.TotalKW*100) / 100
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	if points == nil {
		points = []trendPoint{}
	}

	response.OK(c, gin.H{"trend": points})
}
