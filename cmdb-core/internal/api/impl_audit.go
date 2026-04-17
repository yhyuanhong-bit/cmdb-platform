package api

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Audit endpoints
// ---------------------------------------------------------------------------

// QueryAuditEvents returns a paginated list of audit events.
// (GET /audit/events)
func (s *APIServer) QueryAuditEvents(c *gin.Context, params QueryAuditEventsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	var targetID *uuid.UUID
	if params.TargetId != nil {
		u := uuid.UUID(*params.TargetId)
		targetID = &u
	}

	events, total, err := s.auditSvc.Query(c.Request.Context(), tenantID, params.Module, params.TargetType, targetID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to query audit events")
		return
	}
	response.OKList(c, convertSlice(events, toAPIAuditEvent), page, pageSize, int(total))
}

// ---------------------------------------------------------------------------
// Dashboard endpoints
// ---------------------------------------------------------------------------

// GetDashboardStats returns aggregated dashboard statistics.
// (GET /dashboard/stats)
func (s *APIServer) GetDashboardStats(c *gin.Context, params GetDashboardStatsParams) {
	tenantID := tenantIDFromContext(c)

	stats, err := s.dashboardSvc.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get dashboard stats")
		return
	}
	response.OK(c, DashboardStats{
		TotalAssets:    int(stats.TotalAssets),
		TotalRacks:     int(stats.TotalRacks),
		CriticalAlerts: int(stats.CriticalAlerts),
		ActiveOrders:   int(stats.ActiveOrders),
	})
}
