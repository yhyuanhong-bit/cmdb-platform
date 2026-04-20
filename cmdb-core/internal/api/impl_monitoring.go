package api

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Monitoring endpoints
// ---------------------------------------------------------------------------

// ListAlerts returns a paginated list of alert events.
// (GET /monitoring/alerts)
func (s *APIServer) ListAlerts(c *gin.Context, params ListAlertsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	var assetID *uuid.UUID
	if params.AssetId != nil {
		u := uuid.UUID(*params.AssetId)
		assetID = &u
	}

	alerts, total, err := s.monitoringSvc.ListAlerts(c.Request.Context(), tenantID, params.Status, params.Severity, assetID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list alerts")
		return
	}
	response.OKList(c, convertSlice(alerts, toAPIAlertEvent), page, pageSize, int(total))
}

// AcknowledgeAlert acknowledges an alert event.
// (POST /monitoring/alerts/{id}/ack)
func (s *APIServer) AcknowledgeAlert(c *gin.Context, id IdPath) {
	alert, err := s.monitoringSvc.Acknowledge(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "alert not found")
		return
	}
	s.recordAudit(c, "alert.acknowledged", "monitoring", "alert", alert.ID, map[string]any{
		"status": alert.Status,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectAlertResolved, tenantIDFromContext(c).String(), map[string]any{
		"alert_id": alert.ID.String(), "status": "acknowledged",
	})
	response.OK(c, toAPIAlertEvent(*alert))
}

// ResolveAlert resolves an alert event.
// (POST /monitoring/alerts/{id}/resolve)
func (s *APIServer) ResolveAlert(c *gin.Context, id IdPath) {
	alert, err := s.monitoringSvc.Resolve(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "alert not found")
		return
	}
	s.recordAudit(c, "alert.resolved", "monitoring", "alert", alert.ID, map[string]any{
		"status": alert.Status,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectAlertResolved, tenantIDFromContext(c).String(), map[string]any{
		"alert_id": alert.ID.String(), "status": "resolved",
	})
	response.OK(c, toAPIAlertEvent(*alert))
}

// ListAlertRules returns a paginated list of alert rules.
// (GET /monitoring/rules)
func (s *APIServer) ListAlertRules(c *gin.Context, params ListAlertRulesParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	rules, total, err := s.monitoringSvc.ListRules(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list alert rules")
		return
	}
	response.OKList(c, convertSlice(rules, toAPIAlertRule), page, pageSize, int(total))
}

// CreateAlertRule creates a new alert rule.
// (POST /monitoring/rules)
func (s *APIServer) CreateAlertRule(c *gin.Context) {
	if s.cfg.EdgeNodeID != "" {
		response.Forbidden(c, "alert rules are managed by Central, read-only on Edge")
		return
	}

	var req CreateAlertRuleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var conditionJSON json.RawMessage
	if req.Condition != nil {
		conditionJSON, _ = json.Marshal(req.Condition)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	params := dbgen.CreateAlertRuleParams{
		TenantID:   tenantID,
		Name:       req.Name,
		MetricName: req.MetricName,
		Condition:  conditionJSON,
		Severity:   req.Severity,
		Enabled:    enabled,
	}

	rule, err := s.monitoringSvc.CreateRule(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			response.Err(c, 409, "DUPLICATE", "An alert rule with this name already exists")
			return
		}
		response.InternalError(c, "failed to create alert rule")
		return
	}
	s.recordAudit(c, "alert_rule.created", "monitoring", "alert_rule", rule.ID, map[string]any{
		"name":     rule.Name,
		"severity": rule.Severity,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectAlertFired, tenantIDFromContext(c).String(), map[string]any{
		"rule_id": rule.ID.String(), "name": rule.Name,
	})
	response.Created(c, toAPIAlertRule(*rule))
}

// UpdateAlertRule updates an existing alert rule.
// (PUT /monitoring/rules/{id})
func (s *APIServer) UpdateAlertRule(c *gin.Context, id IdPath) {
	if s.cfg.EdgeNodeID != "" {
		response.Forbidden(c, "alert rules are managed by Central, read-only on Edge")
		return
	}

	var req UpdateAlertRuleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateAlertRuleParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.MetricName != nil {
		params.MetricName = pgtype.Text{String: *req.MetricName, Valid: true}
	}
	if req.Condition != nil {
		b, _ := json.Marshal(req.Condition)
		params.Condition = b
	}
	if req.Severity != nil {
		params.Severity = pgtype.Text{String: *req.Severity, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}

	updated, err := s.monitoringSvc.UpdateRule(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "alert rule not found")
		} else {
			response.InternalError(c, "failed to update alert rule")
		}
		return
	}
	s.recordAudit(c, "alert_rule.updated", "monitoring", "alert_rule", updated.ID, map[string]any{
		"name": updated.Name,
	})
	response.OK(c, toAPIAlertRule(*updated))
}

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
