package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Incidents
// ---------------------------------------------------------------------------

// ListIncidents returns a paginated list of incidents.
// (GET /monitoring/incidents)
func (s *APIServer) ListIncidents(c *gin.Context, params ListIncidentsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	incidents, total, err := s.monitoringSvc.ListIncidents(c.Request.Context(), tenantID, params.Status, params.Severity, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list incidents")
		return
	}
	response.OKList(c, convertSlice(incidents, toAPIIncident), page, pageSize, int(total))
}

// CreateIncident creates a new incident.
// (POST /monitoring/incidents)
func (s *APIServer) CreateIncident(c *gin.Context) {
	var req CreateIncidentJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	status := "open"
	if req.Status != nil {
		status = *req.Status
	}

	params := dbgen.CreateIncidentParams{
		TenantID:  tenantID,
		Title:     req.Title,
		Status:    status,
		Severity:  req.Severity,
		StartedAt: time.Now(),
	}

	incident, err := s.monitoringSvc.CreateIncident(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create incident")
		return
	}
	s.recordAudit(c, "incident.created", "monitoring", "incident", incident.ID, map[string]any{
		"title":    incident.Title,
		"severity": incident.Severity,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectAlertFired, tenantIDFromContext(c).String(), map[string]any{
		"incident_id": incident.ID.String(), "title": incident.Title, "severity": incident.Severity,
	})
	response.Created(c, toAPIIncident(*incident))
}

// ---------------------------------------------------------------------------
// Wave 5.1 lifecycle endpoints.
//
// Each transition lives behind its own endpoint so:
//   1. The state-machine guard is visible at the URL level (nobody can flip
//      a resolved incident back to 'open' by sneaking a status= on PUT).
//   2. The handler routes through monitoringSvc.* helpers which wrap the
//      UPDATE + incident_comments insert in a single transaction so the
//      timeline never drifts from the row state.
//
// All four endpoints return 409 when the source state doesn't match the
// transition's guard (e.g. acknowledge on an already-acknowledged incident).
// ---------------------------------------------------------------------------

// AcknowledgeIncident transitions open → acknowledged.
// (POST /monitoring/incidents/{id}/acknowledge)
func (s *APIServer) AcknowledgeIncident(c *gin.Context, id IdPath) {
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.monitoringSvc.AcknowledgeIncident(c.Request.Context(), tenantID, uuid.UUID(id), userID, req.Note)
	s.writeLifecycleResponse(c, updated, err, "incident.acknowledged")
}

// StartInvestigatingIncident transitions acknowledged → investigating.
// (POST /monitoring/incidents/{id}/start-investigating)
func (s *APIServer) StartInvestigatingIncident(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.monitoringSvc.StartInvestigatingIncident(c.Request.Context(), tenantID, uuid.UUID(id), userID)
	s.writeLifecycleResponse(c, updated, err, "incident.investigating")
}

// ResolveIncident transitions * → resolved (except from closed).
// Accepts an optional root_cause that also gets appended as a system
// comment so the post-mortem has it in the timeline.
// (POST /monitoring/incidents/{id}/resolve)
func (s *APIServer) ResolveIncident(c *gin.Context, id IdPath) {
	var req struct {
		RootCause string `json:"root_cause"`
		Note      string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.monitoringSvc.ResolveIncident(c.Request.Context(), tenantID, uuid.UUID(id), userID, req.RootCause, req.Note)
	s.writeLifecycleResponse(c, updated, err, "incident.resolved")
}

// CloseIncident transitions resolved → closed (post-mortem lock).
// (POST /monitoring/incidents/{id}/close)
func (s *APIServer) CloseIncident(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.monitoringSvc.CloseIncident(c.Request.Context(), tenantID, uuid.UUID(id), userID)
	s.writeLifecycleResponse(c, updated, err, "incident.closed")
}

// ReopenIncident transitions resolved → open.
// (POST /monitoring/incidents/{id}/reopen)
func (s *APIServer) ReopenIncident(c *gin.Context, id IdPath) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.monitoringSvc.ReopenIncident(c.Request.Context(), tenantID, uuid.UUID(id), userID, req.Reason)
	s.writeLifecycleResponse(c, updated, err, "incident.reopened")
}

// ListIncidentComments returns the activity timeline.
// (GET /monitoring/incidents/{id}/comments)
func (s *APIServer) ListIncidentComments(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.monitoringSvc.ListIncidentComments(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list comments")
		return
	}
	out := make([]IncidentComment, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIIncidentComment(r))
	}
	response.OK(c, out)
}

// CreateIncidentComment appends a human comment.
// (POST /monitoring/incidents/{id}/comments)
func (s *APIServer) CreateIncidentComment(c *gin.Context, id IdPath) {
	var req struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Body) == "" {
		response.BadRequest(c, "comment body is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	row, err := s.monitoringSvc.AddIncidentComment(c.Request.Context(), tenantID, uuid.UUID(id), userID, "human", req.Body)
	if err != nil {
		response.InternalError(c, "failed to add comment")
		return
	}
	response.Created(c, toAPIIncidentCommentFromRecord(*row))
}

// writeLifecycleResponse centralises the success/error/conflict mapping the
// five transition endpoints all share.
func (s *APIServer) writeLifecycleResponse(c *gin.Context, updated *dbgen.Incident, err error, auditAction string) {
	if err != nil {
		if errIsInvalidTransition(err) {
			response.Err(c, 409, "INCIDENT_INVALID_TRANSITION", err.Error())
			return
		}
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "incident not found")
			return
		}
		response.InternalError(c, "failed to transition incident")
		return
	}
	s.recordAudit(c, auditAction, "monitoring", "incident", updated.ID, map[string]any{
		"status": updated.Status,
	})
	response.OK(c, toAPIIncident(*updated))
}

func errIsInvalidTransition(err error) bool {
	return strings.Contains(err.Error(), "invalid state transition")
}

// GetIncident returns a single incident.
// (GET /monitoring/incidents/{id})
func (s *APIServer) GetIncident(c *gin.Context, id IdPath) {
	incident, err := s.monitoringSvc.GetIncident(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "incident not found")
		return
	}
	response.OK(c, toAPIIncident(*incident))
}

// UpdateIncident updates non-status incident fields. Status changes go
// through the dedicated acknowledge/resolve/reopen endpoints so the state
// machine guard has a single home. If a caller still sends `status` or
// `resolved_at`, those fields are silently ignored for backward compat.
// (PUT /monitoring/incidents/{id})
func (s *APIServer) UpdateIncident(c *gin.Context, id IdPath) {
	var req UpdateIncidentJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	params := dbgen.UpdateIncidentParams{
		ID:       uuid.UUID(id),
		TenantID: tenantID,
	}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Severity != nil {
		params.Severity = pgtype.Text{String: *req.Severity, Valid: true}
	}
	// req.Status and req.ResolvedAt intentionally ignored — see Wave 5.1
	// state machine: acknowledge/resolve/reopen are the only status paths.
	_ = req.Status
	_ = req.ResolvedAt

	updated, err := s.monitoringSvc.UpdateIncident(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "incident not found")
		} else {
			response.InternalError(c, "failed to update incident")
		}
		return
	}
	s.recordAudit(c, "incident.updated", "monitoring", "incident", updated.ID, map[string]any{
		"status": updated.Status,
	})
	response.OK(c, toAPIIncident(*updated))
}

// QueryMetrics queries time-series metric data for an asset.
// It selects the optimal TimescaleDB source based on requested time range:
//   - <= 1h  : raw metrics hypertable (full resolution)
//   - <= 24h : metrics_5min continuous aggregate
//   - > 24h  : metrics_1hour continuous aggregate
//
// (GET /monitoring/metrics)
func (s *APIServer) QueryMetrics(c *gin.Context, params QueryMetricsParams) {
	tenantID := tenantIDFromContext(c)
	assetID := uuid.UUID(params.AssetId)
	metricName := params.MetricName
	timeRange := params.TimeRange

	// Parse time_range (e.g., "1h", "6h", "24h", "7d") into a duration.
	cutoff, err := parseTimeRange(timeRange)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid time_range: %s", timeRange))
		return
	}

	// Select the appropriate table/view based on the requested time range.
	// Continuous aggregates use "bucket" and "avg_value" columns instead of
	// "time" and "value".
	var tableName, timeCol, valueCol string
	switch {
	case cutoff > 24*time.Hour:
		tableName = "metrics_1hour"
		timeCol = "bucket"
		valueCol = "avg_value"
	case cutoff > time.Hour:
		tableName = "metrics_5min"
		timeCol = "bucket"
		valueCol = "avg_value"
	default:
		tableName = "metrics"
		timeCol = "time"
		valueCol = "value"
	}

	since := time.Now().Add(-cutoff)

	// Table and column names are selected from our own constants above
	// (not from user input), so fmt.Sprintf is safe here.  Bind parameters
	// are still used for all user-supplied values.
	query := fmt.Sprintf(
		`SELECT %s AS time, name, %s AS value
		 FROM %s
		 WHERE tenant_id = $1
		   AND asset_id = $2
		   AND name = $3
		   AND %s > $4
		 ORDER BY %s DESC
		 LIMIT 500`,
		timeCol, valueCol, tableName, timeCol, timeCol,
	)

	sc := database.Scope(s.pool, tenantID)
	rows, err := sc.Query(c.Request.Context(), query, assetID, metricName, since)
	if err != nil {
		response.InternalError(c, "failed to query metrics")
		return
	}
	defer rows.Close()

	points := make([]MetricPoint, 0, 128)
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.Time, &p.Name, &p.Value); err != nil {
			response.InternalError(c, "failed to scan metric row")
			return
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "error reading metric rows")
		return
	}

	response.OK(c, points)
}

// parseTimeRange converts a string like "6h", "24h", "7d" into a time.Duration.
func parseTimeRange(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("too short")
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	switch unit {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown unit: %c", unit)
	}
}
