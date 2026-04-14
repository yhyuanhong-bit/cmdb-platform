package api

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// metricSummary holds fleet-wide aggregate stats for a single metric name.
type metricSummary struct {
	Name       string    `json:"name"`
	Label      string    `json:"label"`
	AvgValue   *float64  `json:"avg_value"`
	MinValue   *float64  `json:"min_value"`
	MaxValue   *float64  `json:"max_value"`
	P95Value   *float64  `json:"p95_value"`
	DataPoints int       `json:"data_points"`
	Unit       string    `json:"unit"`
	Sparkline  []float64 `json:"sparkline"` // last 7 days daily avg
}

// GetFleetMetricsSummary returns fleet-wide averages for key metrics.
// GET /api/v1/fleet-metrics
func (s *APIServer) GetFleetMetricsSummary(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()

	metricDefs := []struct {
		name  string
		label string
		unit  string
	}{
		{"cpu_usage", "CPU Usage", "%"},
		{"memory_usage", "Memory Usage", "%"},
		{"disk_usage", "Disk Usage", "%"},
		{"temperature", "Temperature", "°C"},
		{"power_kw", "Power Draw", "kW"},
		{"network_in_bytes", "Network In", "MB/s"},
		{"network_out_bytes", "Network Out", "MB/s"},
	}

	results := make([]metricSummary, 0, len(metricDefs))

	for _, md := range metricDefs {
		ms := metricSummary{
			Name:      md.name,
			Label:     md.label,
			Unit:      md.unit,
			Sparkline: []float64{},
		}

		// Aggregate stats from the last 24 hours.
		var avg, min, max, p95 *float64
		var count int
		err := s.pool.QueryRow(ctx,
			`SELECT avg(value), min(value), max(value),
			        percentile_cont(0.95) WITHIN GROUP (ORDER BY value),
			        count(*)
			 FROM metrics
			 WHERE tenant_id = $1 AND name = $2 AND time > now() - interval '24 hours'`,
			tenantID, md.name).Scan(&avg, &min, &max, &p95, &count)

		if err == nil && count > 0 {
			ms.AvgValue = avg
			ms.MinValue = min
			ms.MaxValue = max
			ms.P95Value = p95
			ms.DataPoints = count
		}

		// 7-day sparkline: one daily average per day.
		sparkRows, err := s.pool.Query(ctx,
			`SELECT date_trunc('day', time) AS d, avg(value)
			 FROM metrics
			 WHERE tenant_id = $1 AND name = $2 AND time > now() - interval '7 days'
			 GROUP BY d ORDER BY d`,
			tenantID, md.name)
		if err == nil {
			for sparkRows.Next() {
				var t time.Time
				var v float64
				if sparkRows.Scan(&t, &v) == nil {
					ms.Sparkline = append(ms.Sparkline, math.Round(v*100)/100)
				}
			}
			sparkRows.Close()
		}

		results = append(results, ms)
	}

	response.OK(c, results)
}
