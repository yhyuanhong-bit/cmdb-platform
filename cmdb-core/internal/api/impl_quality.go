package api

import (
	"encoding/json"
	"errors"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
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
	scores, err := s.qualitySvc.GetAssetHistory(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to get asset quality history")
		return
	}
	response.OK(c, convertSlice(scores, toAPIQualityScoreFromHistory))
}

// FlagQualityIssue records a consumer-side report that an asset has
// bad CMDB data. The scanner applies an accuracy penalty on its next
// pass until the flag is triaged.
// (POST /quality/flag-issue)
func (s *APIServer) FlagQualityIssue(c *gin.Context) {
	var req FlagQualityIssueJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if req.Message == "" || req.Category == "" {
		response.BadRequest(c, "message and category are required")
		return
	}

	tenantID := tenantIDFromContext(c)
	params := quality.FlagIssueParams{
		TenantID:     tenantID,
		AssetID:      uuid.UUID(req.AssetId),
		ReporterType: string(req.ReporterType),
		Severity:     string(req.Severity),
		Category:     req.Category,
		Message:      req.Message,
	}
	if req.ReporterId != nil {
		rid := uuid.UUID(*req.ReporterId)
		params.ReporterID = &rid
	}

	flag, err := s.qualitySvc.FlagIssue(c.Request.Context(), params)
	if err != nil {
		// W6.3: cross-tenant asset_id surfaces as ErrAssetNotInTenant; map
		// to 404 (not 403) so the existence of the foreign asset is not
		// leaked as an information oracle.
		if errors.Is(err, quality.ErrAssetNotInTenant) {
			response.NotFound(c, "asset not found")
			return
		}
		response.InternalError(c, "failed to create quality flag")
		return
	}
	s.recordAudit(c, "quality.flag_issued", "quality", "quality_flag", flag.ID, map[string]any{
		"asset_id":      flag.AssetID,
		"severity":      flag.Severity,
		"category":      flag.Category,
		"reporter_type": flag.ReporterType,
	})
	response.Created(c, toAPIQualityFlag(*flag))
}

// ListOpenQualityFlags returns the open flag triage list.
// (GET /quality/flags)
func (s *APIServer) ListOpenQualityFlags(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.qualitySvc.ListOpenFlags(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list open quality flags")
		return
	}
	response.OK(c, convertSlice(rows, toAPIQualityFlagListItem))
}

// ResolveQualityFlag transitions an open flag to acknowledged,
// resolved, or rejected.
// (POST /quality/flags/{id}/resolve)
func (s *APIServer) ResolveQualityFlag(c *gin.Context, id IdPath) {
	var req ResolveQualityFlagJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	var resolvedBy *uuid.UUID
	if uidStr, ok := c.Get("user_id"); ok {
		if uidS, ok2 := uidStr.(string); ok2 {
			if u, perr := uuid.Parse(uidS); perr == nil {
				resolvedBy = &u
			}
		}
	}

	note := ""
	if req.ResolutionNote != nil {
		note = *req.ResolutionNote
	}

	flag, err := s.qualitySvc.ResolveFlag(c.Request.Context(), tenantID, uuid.UUID(id), string(req.Status), resolvedBy, note)
	if err != nil {
		response.NotFound(c, "quality flag not found")
		return
	}
	s.recordAudit(c, "quality.flag_resolved", "quality", "quality_flag", flag.ID, map[string]any{
		"status": flag.Status,
	})
	response.OK(c, toAPIQualityFlag(*flag))
}
