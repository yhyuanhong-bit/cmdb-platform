package api

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

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

	sc := database.Scope(s.pool, tenantID)
	rows, err := sc.Query(c.Request.Context(), `
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
	`, hours)
	if err != nil {
		response.InternalError(c, "failed to query alerts trend")
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

	response.OK(c, gin.H{"trend": trend})
}
