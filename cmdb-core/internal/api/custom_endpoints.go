package api

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
)

// GetRackStats handles GET /racks/stats
// Returns total rack count, total U capacity, used U slots, and occupancy percentage.
func (s *APIServer) GetRackStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	row := s.pool.QueryRow(c.Request.Context(), `
		SELECT
			count(DISTINCT r.id)         AS total_racks,
			COALESCE(sum(r.total_u), 0)  AS total_u,
			count(rs.id)                 AS used_slots
		FROM racks r
		LEFT JOIN rack_slots rs ON rs.rack_id = r.id
		WHERE r.tenant_id = $1
	`, tenantID)

	var totalRacks, totalU, usedSlots int64
	if err := row.Scan(&totalRacks, &totalU, &usedSlots); err != nil {
		c.JSON(500, gin.H{"error": "failed to query rack stats"})
		return
	}

	occupancyPct := 0.0
	if totalU > 0 {
		occupancyPct = float64(usedSlots) / float64(totalU) * 100
	}

	c.JSON(200, gin.H{
		"total_racks":   totalRacks,
		"total_u":       totalU,
		"used_u":        usedSlots,
		"occupancy_pct": math.Round(occupancyPct*100) / 100,
	})
}

// GetAssetLifecycleStats handles GET /assets/lifecycle-stats
// Returns asset counts grouped by status.
func (s *APIServer) GetAssetLifecycleStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT status, count(*) AS cnt
		FROM assets
		WHERE tenant_id = $1
		GROUP BY status
	`, tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to query lifecycle stats"})
		return
	}
	defer rows.Close()

	stats := map[string]int64{}
	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			continue
		}
		stats[status] = cnt
	}

	c.JSON(200, gin.H{"by_status": stats})
}

// alertTrendPoint represents one hourly bucket in the alerts trend response.
type alertTrendPoint struct {
	Hour     time.Time `json:"hour"`
	Critical int64     `json:"critical"`
	Warning  int64     `json:"warning"`
	Info     int64     `json:"info"`
}

// GetAlertsTrend handles GET /monitoring/alerts/trend?hours=24
// Returns per-hour alert counts broken down by severity.
func (s *APIServer) GetAlertsTrend(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	hours := c.DefaultQuery("hours", "24")

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			date_trunc('hour', fired_at)                            AS hour,
			count(*) FILTER (WHERE severity = 'critical')           AS critical,
			count(*) FILTER (WHERE severity = 'warning')            AS warning,
			count(*) FILTER (WHERE severity = 'info')               AS info
		FROM alert_events
		WHERE tenant_id = $1
		  AND fired_at > now() - ($2 || ' hours')::interval
		GROUP BY hour
		ORDER BY hour
	`, tenantID, hours)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to query alerts trend"})
		return
	}
	defer rows.Close()

	trend := []alertTrendPoint{}
	for rows.Next() {
		var p alertTrendPoint
		if err := rows.Scan(&p.Hour, &p.Critical, &p.Warning, &p.Info); err != nil {
			continue
		}
		trend = append(trend, p)
	}

	c.JSON(200, gin.H{"trend": trend})
}

// rackMaintenanceRecord holds a single work-order row for rack maintenance history.
type rackMaintenanceRecord struct {
	ID             string  `json:"id"`
	Code           string  `json:"code"`
	Title          string  `json:"title"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	Priority       string  `json:"priority"`
	ScheduledStart *string `json:"scheduled_start"`
	ActualStart    *string `json:"actual_start"`
	ActualEnd      *string `json:"actual_end"`
	CreatedAt      string  `json:"created_at"`
}

// GetRackMaintenance handles GET /racks/:id/maintenance
// Returns the last 20 work orders for assets installed in the given rack.
func (s *APIServer) GetRackMaintenance(c *gin.Context) {
	rackID := c.Param("id")
	if rackID == "" {
		c.JSON(400, gin.H{"error": "missing rack id"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			wo.id, wo.code, wo.title, wo.type, wo.status, wo.priority,
			wo.scheduled_start, wo.actual_start, wo.actual_end, wo.created_at
		FROM work_orders wo
		JOIN assets a ON wo.asset_id = a.id
		WHERE a.rack_id = $1
		ORDER BY wo.created_at DESC
		LIMIT 20
	`, rackID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to query rack maintenance"})
		return
	}
	defer rows.Close()

	records := []rackMaintenanceRecord{}
	for rows.Next() {
		var r rackMaintenanceRecord
		var scheduledStart, actualStart, actualEnd *time.Time
		var createdAt time.Time

		if err := rows.Scan(
			&r.ID, &r.Code, &r.Title, &r.Type, &r.Status, &r.Priority,
			&scheduledStart, &actualStart, &actualEnd, &createdAt,
		); err != nil {
			continue
		}

		r.CreatedAt = createdAt.Format(time.RFC3339)
		if scheduledStart != nil {
			s := scheduledStart.Format(time.RFC3339)
			r.ScheduledStart = &s
		}
		if actualStart != nil {
			s := actualStart.Format(time.RFC3339)
			r.ActualStart = &s
		}
		if actualEnd != nil {
			s := actualEnd.Format(time.RFC3339)
			r.ActualEnd = &s
		}

		records = append(records, r)
	}

	c.JSON(200, gin.H{"maintenance": records})
}
