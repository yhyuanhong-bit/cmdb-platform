package api

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
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

// GetAssetLifecycleStats handles GET /assets/lifecycle-stats
// Returns aggregated lifecycle/financial stats for the overview page.
func (s *APIServer) GetAssetLifecycleStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()

	// Status-grouped counts (kept for backward compatibility)
	rows, err := s.pool.Query(ctx, `
		SELECT status, count(*) AS cnt
		FROM assets
		WHERE tenant_id = $1 AND deleted_at IS NULL
		GROUP BY status
	`, tenantID)
	if err != nil {
		response.InternalError(c, "failed to query lifecycle stats")
		return
	}
	defer rows.Close()

	byStatus := map[string]int64{}
	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			continue
		}
		byStatus[status] = cnt
	}

	// Aggregated financial & warranty fields. Any DB error here means the
	// totals would be fabricated zeros — abort rather than mislead the caller.
	var totalCost float64
	if err := s.pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(purchase_cost::numeric::float8), 0) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL",
		tenantID).Scan(&totalCost); err != nil {
		zap.L().Error("lifecycle: failed to sum purchase_cost", zap.Error(err))
		response.InternalError(c, "failed to query lifecycle stats")
		return
	}

	var warrantyActiveCount, warrantyExpiredCount, approachingEOL int64
	if err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM assets WHERE tenant_id = $1 AND warranty_end > now() AND deleted_at IS NULL",
		tenantID).Scan(&warrantyActiveCount); err != nil {
		zap.L().Error("lifecycle: failed to count active warranty", zap.Error(err))
		response.InternalError(c, "failed to query lifecycle stats")
		return
	}

	if err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM assets WHERE tenant_id = $1 AND warranty_end IS NOT NULL AND warranty_end <= now() AND deleted_at IS NULL",
		tenantID).Scan(&warrantyExpiredCount); err != nil {
		zap.L().Error("lifecycle: failed to count expired warranty", zap.Error(err))
		response.InternalError(c, "failed to query lifecycle stats")
		return
	}

	if err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM assets WHERE tenant_id = $1 AND eol_date IS NOT NULL AND eol_date <= now() + interval '6 months' AND eol_date > now() AND deleted_at IS NULL",
		tenantID).Scan(&approachingEOL); err != nil {
		zap.L().Error("lifecycle: failed to count approaching EOL", zap.Error(err))
		response.InternalError(c, "failed to query lifecycle stats")
		return
	}

	response.OK(c, gin.H{
		"by_status":              byStatus,
		"total_purchase_cost":    totalCost,
		"warranty_active_count":  warrantyActiveCount,
		"warranty_expired_count": warrantyExpiredCount,
		"approaching_eol_count":  approachingEOL,
	})
}

// lifecycleEvent is a single entry in the asset lifecycle timeline.
type lifecycleEvent struct {
	Type        string     `json:"type"`
	Action      string     `json:"action"`
	FromStatus  string     `json:"from_status,omitempty"`
	ToStatus    string     `json:"to_status,omitempty"`
	Description string     `json:"description,omitempty"`
	OperatorID  *uuid.UUID `json:"operator_id,omitempty"`
	Date        time.Time  `json:"date"`
	Diff        any        `json:"diff,omitempty"`
}

// GetAssetLifecycle handles GET /assets/:id/lifecycle
// Returns the lifecycle timeline for an asset, combining audit_events with warranty milestones.
func (s *APIServer) GetAssetLifecycle(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	assetID := uuid.UUID(id)
	ctx := c.Request.Context()

	// 1. Get asset basic info (for warranty/EOL dates and summary)
	asset, err := s.assetSvc.GetByID(ctx, tenantID, assetID)
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}

	// 2. Query audit_events for this asset's history
	auditRows, err := s.pool.Query(ctx,
		`SELECT action, diff, operator_id, created_at
		 FROM audit_events
		 WHERE target_type = 'asset' AND target_id = $1
		 ORDER BY created_at ASC`,
		assetID)
	if err != nil {
		response.InternalError(c, "failed to query lifecycle events")
		return
	}
	defer auditRows.Close()

	var events []lifecycleEvent

	// pgUUID matches the wire format of pgtype.UUID for scanning.
	type pgUUID struct {
		Bytes [16]byte
		Valid bool
	}

	for auditRows.Next() {
		var action string
		var diffBytes []byte
		var opID pgUUID
		var createdAt time.Time

		if err := auditRows.Scan(&action, &diffBytes, &opID, &createdAt); err != nil {
			continue
		}

		evt := lifecycleEvent{
			Action: action,
			Date:   createdAt,
		}
		if opID.Valid {
			id := uuid.UUID(opID.Bytes)
			evt.OperatorID = &id
		}

		var diffMap map[string]any
		// The audit diff is opaque JSONB built from trusted internal
		// code paths, but a broken row would previously have produced
		// a silent unlabeled timeline entry. Log the parse failure so
		// a corrupted audit row is visible; leave diffMap nil and let
		// the switch below fall through to its default "Asset event"
		// branch so the timeline still renders something.
		if err := json.Unmarshal(diffBytes, &diffMap); err != nil {
			zap.L().Warn("lifecycle: audit diff parse failed",
				zap.String("action", action), zap.Error(err))
		}

		switch action {
		case "asset.created":
			evt.Type = "created"
			evt.Description = "Asset created"
			if status, ok := diffMap["status"]; ok {
				evt.ToStatus = fmt.Sprintf("%v", status)
			}
		case "asset.updated":
			if statusDiff, ok := diffMap["status"]; ok {
				if m, ok := statusDiff.(map[string]any); ok {
					evt.Type = "status_change"
					evt.FromStatus = fmt.Sprintf("%v", m["old"])
					evt.ToStatus = fmt.Sprintf("%v", m["new"])
					evt.Description = fmt.Sprintf("Status changed: %v → %v", m["old"], m["new"])
				} else {
					evt.Type = "updated"
					evt.Description = "Asset updated"
				}
			} else {
				evt.Type = "updated"
				evt.Description = "Asset updated"
				evt.Diff = diffMap
			}
		case "asset.deleted":
			evt.Type = "deleted"
			evt.Description = "Asset deleted"
		default:
			evt.Type = "other"
			evt.Description = action
		}

		events = append(events, evt)
	}

	// 3. Add warranty milestone events (synthetic, from asset fields)
	if asset.WarrantyStart.Valid {
		events = append(events, lifecycleEvent{
			Type:        "warranty_start",
			Action:      "warranty.started",
			Description: "Warranty period started",
			Date:        asset.WarrantyStart.Time,
		})
	}
	if asset.WarrantyEnd.Valid {
		events = append(events, lifecycleEvent{
			Type:        "warranty_end",
			Action:      "warranty.expires",
			Description: "Warranty expires",
			Date:        asset.WarrantyEnd.Time,
		})
	}
	if asset.EolDate.Valid {
		events = append(events, lifecycleEvent{
			Type:        "eol",
			Action:      "lifecycle.eol",
			Description: "End of life",
			Date:        asset.EolDate.Time,
		})
	}

	// 4. Sort all events by date
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date.Before(events[j].Date)
	})

	// 5. Build summary
	summary := gin.H{
		"asset_id":                 assetID,
		"asset_tag":                asset.AssetTag,
		"name":                     asset.Name,
		"status":                   asset.Status,
		"purchase_date":            nil,
		"purchase_cost":            nil,
		"warranty_start":           nil,
		"warranty_end":             nil,
		"warranty_vendor":          nil,
		"warranty_contract":        nil,
		"eol_date":                 nil,
		"expected_lifespan_months": nil,
	}
	if asset.PurchaseDate.Valid {
		summary["purchase_date"] = asset.PurchaseDate.Time.Format("2006-01-02")
	}
	if asset.PurchaseCost.Valid {
		f, _ := asset.PurchaseCost.Float64Value()
		summary["purchase_cost"] = f.Float64
	}
	if asset.WarrantyStart.Valid {
		summary["warranty_start"] = asset.WarrantyStart.Time.Format("2006-01-02")
	}
	if asset.WarrantyEnd.Valid {
		summary["warranty_end"] = asset.WarrantyEnd.Time.Format("2006-01-02")
	}
	if asset.WarrantyVendor.Valid {
		summary["warranty_vendor"] = asset.WarrantyVendor.String
	}
	if asset.WarrantyContract.Valid {
		summary["warranty_contract"] = asset.WarrantyContract.String
	}
	if asset.EolDate.Valid {
		summary["eol_date"] = asset.EolDate.Time.Format("2006-01-02")
	}
	if asset.ExpectedLifespanMonths.Valid {
		summary["expected_lifespan_months"] = asset.ExpectedLifespanMonths.Int32
	}

	if events == nil {
		events = []lifecycleEvent{}
	}

	response.OK(c, gin.H{
		"summary":  summary,
		"timeline": events,
	})
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

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			wo.id, wo.code, wo.title, wo.type, wo.status, wo.priority,
			wo.scheduled_start, wo.actual_start, wo.actual_end, wo.created_at
		FROM work_orders wo
		JOIN assets a ON wo.asset_id = a.id
		WHERE a.rack_id = $1 AND a.tenant_id = $2 AND wo.tenant_id = $2
		ORDER BY wo.created_at DESC
		LIMIT 20
	`, rackID, tenantID)
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
