package api

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// dashboardQueryTimeout caps how long a single dashboard endpoint can hold
// the request goroutine. Dashboard tiles are interactive — slow queries
// degrade UX worse than empty tiles, so we fail fast and let the UI retry.
const dashboardQueryTimeout = 5 * time.Second

// GetDashboardAssetsTrend returns the asset count series for the requested
// period. Series points are buckets at the granularity implied by the
// period (daily for 7d/30d, weekly for 90d).
//
// Implementation reads from audit_events filtered to asset module to count
// creates and soft-deletes per bucket, then computes the running total
// against the current asset count. This avoids snapshotting and keeps the
// query bounded to the audit retention window.
//
// (GET /dashboard/assets-trend)
func (s *APIServer) GetDashboardAssetsTrend(c *gin.Context, params GetDashboardAssetsTrendParams) {
	tenantID := tenantIDFromContext(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), dashboardQueryTimeout)
	defer cancel()

	bucket, days := bucketForPeriod(string(params.Period))
	if days == 0 {
		response.Err(c, 400, "INVALID_PERIOD", "period must be one of: 7d, 30d, 90d")
		return
	}

	sc := database.Scope(s.pool, tenantID)

	// 1. Current active asset count — anchor for the running-total walk.
	var currentTotal int
	if err := sc.QueryRow(ctx, `
		SELECT count(*) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL
	`).Scan(&currentTotal); err != nil {
		response.InternalError(c, "failed to count assets")
		return
	}

	// 2. Per-bucket create/delete counts from audit_events. Module name is
	//    'asset' for asset CRUD; action='create' / 'delete' tracks lifecycle.
	rows, err := sc.Query(ctx, fmt.Sprintf(`
		SELECT
			date_trunc('%s', created_at) AS bucket,
			count(*) FILTER (WHERE action = 'create') AS created,
			count(*) FILTER (WHERE action = 'delete') AS deleted
		FROM audit_events
		WHERE tenant_id = $1
		  AND module = 'asset'
		  AND created_at >= now() - interval '%d days'
		GROUP BY bucket
		ORDER BY bucket ASC
	`, bucket, days))
	if err != nil {
		response.InternalError(c, "failed to query audit events")
		return
	}
	defer rows.Close()

	type point struct {
		Bucket  time.Time `json:"bucket"`
		Count   int       `json:"count"`
		Created int       `json:"created"`
		Deleted int       `json:"deleted"`
	}
	var changes []point
	for rows.Next() {
		var p point
		if err := rows.Scan(&p.Bucket, &p.Created, &p.Deleted); err != nil {
			response.InternalError(c, "failed to scan trend row")
			return
		}
		changes = append(changes, p)
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "trend row iteration failed")
		return
	}

	// 3. Walk backwards from currentTotal to compute running total per bucket.
	//    Each bucket's count = (next bucket's count) - created + deleted.
	for i := len(changes) - 1; i >= 0; i-- {
		if i == len(changes)-1 {
			changes[i].Count = currentTotal
		} else {
			changes[i].Count = changes[i+1].Count - changes[i+1].Created + changes[i+1].Deleted
			if changes[i].Count < 0 {
				changes[i].Count = 0 // defensive: audit may not perfectly reconcile
			}
		}
	}
	if changes == nil {
		changes = []point{}
	}

	response.OK(c, gin.H{
		"period": string(params.Period),
		"points": changes,
	})
}

// bucketForPeriod returns the date_trunc bucket name and the lookback day
// count for a period parameter. Unknown periods return ("", 0).
func bucketForPeriod(period string) (string, int) {
	switch period {
	case "7d":
		return "day", 7
	case "30d":
		return "day", 30
	case "90d":
		return "week", 90
	default:
		return "", 0
	}
}

// GetDashboardRackHeatmap returns one row per rack with its grid position,
// occupancy, and aggregated power data for the heatmap widget.
//
// Status is derived from occupancy_pct:
//   - critical when >= 90% (no room for additions)
//   - warning  when >= 75% (approaching capacity)
//   - healthy  otherwise
//
// (GET /dashboard/rack-heatmap)
func (s *APIServer) GetDashboardRackHeatmap(c *gin.Context, params GetDashboardRackHeatmapParams) {
	tenantID := tenantIDFromContext(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), dashboardQueryTimeout)
	defer cancel()

	sc := database.Scope(s.pool, tenantID)

	// Allow optional location filter. The Scope wrapper auto-prepends
	// tenantID as $1, so location_id (when present) becomes $2.
	var locationFilter string
	args := []any{}
	if params.LocationId != nil {
		locationFilter = "AND r.location_id = $2"
		args = append(args, uuid.UUID(*params.LocationId))
	}

	// rack_slots stores spans (start_u..end_u inclusive). Used U per rack is
	// the sum of (end_u - start_u + 1). rack_slots has no tenant_id column
	// — isolation flows through the rack FK, which the outer WHERE enforces.
	query := fmt.Sprintf(`
		SELECT
			r.id,
			r.name,
			r.location_id,
			r.row_label,
			r.total_u,
			COALESCE(occ.u_used, 0)::int AS u_used,
			r.power_capacity_kw
		FROM racks r
		LEFT JOIN (
			SELECT rs.rack_id, sum(rs.end_u - rs.start_u + 1) AS u_used
			FROM rack_slots rs
			JOIN racks rk ON rk.id = rs.rack_id
			WHERE rk.tenant_id = $1
			GROUP BY rs.rack_id
		) occ ON occ.rack_id = r.id
		WHERE r.tenant_id = $1
		  AND r.deleted_at IS NULL
		  %s
		ORDER BY r.row_label NULLS LAST, r.name
	`, locationFilter)

	rows, err := sc.Query(ctx, query, args...)
	if err != nil {
		response.InternalError(c, "failed to query racks")
		return
	}
	defer rows.Close()

	type cell struct {
		RackID          string   `json:"rack_id"`
		RackName        string   `json:"rack_name"`
		LocationID      string   `json:"location_id"`
		RowLabel        *string  `json:"row_label"`
		UTotal          int      `json:"u_total"`
		UUsed           int      `json:"u_used"`
		OccupancyPct    float64  `json:"occupancy_pct"`
		PowerCapacityKW *float64 `json:"power_capacity_kw"`
		Status          string   `json:"status"`
	}

	cells := []cell{}
	for rows.Next() {
		var cl cell
		var rackID, locationID uuid.UUID
		var rowLabel *string
		var powerKW *float64
		if err := rows.Scan(&rackID, &cl.RackName, &locationID, &rowLabel, &cl.UTotal, &cl.UUsed, &powerKW); err != nil {
			response.InternalError(c, "failed to scan rack row")
			return
		}
		cl.RackID = rackID.String()
		cl.LocationID = locationID.String()
		cl.RowLabel = rowLabel
		cl.PowerCapacityKW = powerKW
		if cl.UTotal > 0 {
			cl.OccupancyPct = float64(cl.UUsed) / float64(cl.UTotal) * 100
		}
		cl.Status = rackStatus(cl.OccupancyPct)
		cells = append(cells, cl)
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "rack row iteration failed")
		return
	}

	response.OK(c, cells)
}

// rackStatus maps occupancy to a 3-band status. Thresholds match the legend
// the frontend renders so the API and UI agree without an extra lookup.
func rackStatus(occupancyPct float64) string {
	switch {
	case occupancyPct >= 90:
		return "critical"
	case occupancyPct >= 75:
		return "warning"
	default:
		return "healthy"
	}
}
