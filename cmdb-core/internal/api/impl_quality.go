package api

import (
	"encoding/json"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Quality — Data Quality Governance Engine
// ---------------------------------------------------------------------------

// ListQualityRules returns all quality rules for the tenant.
// (GET /quality/rules)
func (s *APIServer) ListQualityRules(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rules, err := s.qualitySvc.ListRules(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list quality rules")
		return
	}
	response.OK(c, convertSlice(rules, toAPIQualityRule))
}

// CreateQualityRule creates a new quality rule.
// (POST /quality/rules)
func (s *APIServer) CreateQualityRule(c *gin.Context) {
	var req CreateQualityRuleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var ruleConfig []byte
	if req.RuleConfig != nil {
		ruleConfig, _ = json.Marshal(req.RuleConfig)
	} else {
		ruleConfig = []byte("{}")
	}

	params := dbgen.CreateQualityRuleParams{
		TenantID:   tenantID,
		CiType:     textFromPtr(req.CiType),
		Dimension:  req.Dimension,
		FieldName:  req.FieldName,
		RuleType:   req.RuleType,
		RuleConfig: ruleConfig,
	}
	if req.Weight != nil {
		params.Weight = pgtype.Int4{Int32: int32(*req.Weight), Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	} else {
		params.Enabled = pgtype.Bool{Bool: true, Valid: true}
	}

	rule, err := s.qualitySvc.CreateRule(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create quality rule")
		return
	}
	s.recordAudit(c, "quality_rule.created", "quality", "quality_rule", rule.ID, map[string]any{
		"dimension":  req.Dimension,
		"field_name": req.FieldName,
		"rule_type":  req.RuleType,
	})
	response.Created(c, toAPIQualityRule(*rule))
}

// TriggerQualityScan runs the quality engine across all tenant assets.
// (POST /quality/scan)
func (s *APIServer) TriggerQualityScan(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	scanned, err := s.qualitySvc.ScanAllAssets(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "quality scan failed")
		return
	}
	s.recordAudit(c, "quality.scan_triggered", "quality", "tenant", tenantID, map[string]any{
		"scanned": scanned,
	})
	response.OK(c, gin.H{"scanned": scanned})
}

// GetQualityDashboard returns aggregate quality metrics.
// (GET /quality/dashboard)
func (s *APIServer) GetQualityDashboard(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	dash, err := s.qualitySvc.GetDashboard(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get quality dashboard")
		return
	}
	response.OK(c, toAPIQualityDashboard(*dash))
}

// GetWorstAssets returns the bottom-10 quality assets.
// (GET /quality/worst)
func (s *APIServer) GetWorstAssets(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.qualitySvc.GetWorstAssets(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get worst assets")
		return
	}
	response.OK(c, convertSlice(rows, toAPIQualityScoreFromWorst))
}

// GetAssetQualityHistory returns quality score history for an asset.
// (GET /quality/history/{id})
func (s *APIServer) GetAssetQualityHistory(c *gin.Context, id IdPath) {
	scores, err := s.qualitySvc.GetAssetHistory(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to get asset quality history")
		return
	}
	response.OK(c, convertSlice(scores, toAPIQualityScoreFromHistory))
}
