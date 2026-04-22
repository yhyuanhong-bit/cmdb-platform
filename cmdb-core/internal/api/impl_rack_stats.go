package api

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// GetRackStats handles GET /racks/stats
// Returns total rack count, total U capacity, used U slots, and occupancy percentage.
func (s *APIServer) GetRackStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	sc := database.Scope(s.pool, tenantID)

	row := sc.QueryRow(c.Request.Context(), `
		SELECT
			count(DISTINCT r.id)         AS total_racks,
			COALESCE(sum(r.total_u), 0)  AS total_u,
			count(rs.id)                 AS used_slots
		FROM racks r
		LEFT JOIN rack_slots rs ON rs.rack_id = r.id
		WHERE r.tenant_id = $1
	`)

	var totalRacks, totalU, usedSlots int64
	if err := row.Scan(&totalRacks, &totalU, &usedSlots); err != nil {
		response.InternalError(c, "failed to query rack stats")
		return
	}

	occupancyPct := 0.0
	if totalU > 0 {
		occupancyPct = float64(usedSlots) / float64(totalU) * 100
	}

	response.OK(c, gin.H{
		"total_racks":   totalRacks,
		"total_u":       totalU,
		"used_u":        usedSlots,
		"occupancy_pct": math.Round(occupancyPct*100) / 100,
	})
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
func (s *APIServer) GetRackMaintenance(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rackID := uuid.UUID(id)
	sc := database.Scope(s.pool, tenantID)

	rows, err := sc.Query(c.Request.Context(), `
		SELECT
			wo.id, wo.code, wo.title, wo.type, wo.status, wo.priority,
			wo.scheduled_start, wo.actual_start, wo.actual_end, wo.created_at
		FROM work_orders wo
		JOIN assets a ON wo.asset_id = a.id
		WHERE a.rack_id = $2 AND a.tenant_id = $1 AND wo.tenant_id = $1
		ORDER BY wo.created_at DESC
		LIMIT 20
	`, rackID)
	if err != nil {
		response.InternalError(c, "failed to query rack maintenance")
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

	response.OK(c, gin.H{"maintenance": records})
}
