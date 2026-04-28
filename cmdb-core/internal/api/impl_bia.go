package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/bia"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// BIA endpoints
// ---------------------------------------------------------------------------

// ListBIAAssessments returns a paginated list of BIA assessments.
// (GET /bia/assessments)
func (s *APIServer) ListBIAAssessments(c *gin.Context, params ListBIAAssessmentsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	assessments, total, err := s.biaSvc.ListAssessments(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list BIA assessments")
		return
	}

	response.OKList(c, convertSlice(assessments, toAPIBIAAssessment), page, pageSize, int(total))
}

// CreateBIAAssessment creates a new BIA assessment.
// (POST /bia/assessments)
func (s *APIServer) CreateBIAAssessment(c *gin.Context) {
	var req CreateBIAAssessmentJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	params := dbgen.CreateBIAAssessmentParams{
		TenantID:   tenantID,
		SystemName: req.SystemName,
		SystemCode: req.SystemCode,
		BiaScore:   int32(req.BiaScore),
		Tier:       req.Tier,
	}
	if req.Owner != nil {
		params.Owner = pgtype.Text{String: *req.Owner, Valid: true}
	}
	if req.RtoHours != nil {
		params.RtoHours = float32ToNumeric(*req.RtoHours)
	}
	if req.RpoMinutes != nil {
		params.RpoMinutes = float32ToNumeric(*req.RpoMinutes)
	}
	if req.MtpdHours != nil {
		params.MtpdHours = float32ToNumeric(*req.MtpdHours)
	}
	if req.DataCompliance != nil {
		params.DataCompliance = pgtype.Bool{Bool: *req.DataCompliance, Valid: true}
	}
	if req.AssetCompliance != nil {
		params.AssetCompliance = pgtype.Bool{Bool: *req.AssetCompliance, Valid: true}
	}
	if req.AuditCompliance != nil {
		params.AuditCompliance = pgtype.Bool{Bool: *req.AuditCompliance, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.AssessedBy != nil {
		u := uuid.UUID(*req.AssessedBy)
		params.AssessedBy = pgtype.UUID{Bytes: u, Valid: true}
	}

	created, err := s.biaSvc.CreateAssessment(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create BIA assessment")
		return
	}
	s.recordAudit(c, "bia.assessment.created", "bia", "bia_assessment", created.ID, map[string]any{
		"system_name": created.SystemName,
		"system_code": created.SystemCode,
		"tier":        created.Tier,
	})
	response.Created(c, toAPIBIAAssessment(*created))
}

// GetBIAAssessment returns a single BIA assessment by ID.
// (GET /bia/assessments/{id})
func (s *APIServer) GetBIAAssessment(c *gin.Context, id IdPath) {
	a, err := s.biaSvc.GetAssessment(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "BIA assessment not found")
		return
	}
	response.OK(c, toAPIBIAAssessment(*a))
}

// UpdateBIAAssessment updates an existing BIA assessment.
// (PUT /bia/assessments/{id})
func (s *APIServer) UpdateBIAAssessment(c *gin.Context, id IdPath) {
	var req UpdateBIAAssessmentJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateBIAAssessmentParams{
		ID:       uuid.UUID(id),
		TenantID: tenantIDFromContext(c),
	}
	diff := map[string]any{}
	if req.SystemName != nil {
		params.SystemName = pgtype.Text{String: *req.SystemName, Valid: true}
		diff["system_name"] = *req.SystemName
	}
	if req.Owner != nil {
		params.Owner = pgtype.Text{String: *req.Owner, Valid: true}
		diff["owner"] = *req.Owner
	}
	if req.BiaScore != nil {
		params.BiaScore = pgtype.Int4{Int32: int32(*req.BiaScore), Valid: true}
		diff["bia_score"] = *req.BiaScore
	}
	if req.Tier != nil {
		params.Tier = pgtype.Text{String: *req.Tier, Valid: true}
		diff["tier"] = *req.Tier
	}
	if req.RtoHours != nil {
		params.RtoHours = float32ToNumeric(*req.RtoHours)
		diff["rto_hours"] = *req.RtoHours
	}
	if req.RpoMinutes != nil {
		params.RpoMinutes = float32ToNumeric(*req.RpoMinutes)
		diff["rpo_minutes"] = *req.RpoMinutes
	}
	if req.MtpdHours != nil {
		params.MtpdHours = float32ToNumeric(*req.MtpdHours)
		diff["mtpd_hours"] = *req.MtpdHours
	}
	if req.DataCompliance != nil {
		params.DataCompliance = pgtype.Bool{Bool: *req.DataCompliance, Valid: true}
		diff["data_compliance"] = *req.DataCompliance
	}
	if req.AssetCompliance != nil {
		params.AssetCompliance = pgtype.Bool{Bool: *req.AssetCompliance, Valid: true}
		diff["asset_compliance"] = *req.AssetCompliance
	}
	if req.AuditCompliance != nil {
		params.AuditCompliance = pgtype.Bool{Bool: *req.AuditCompliance, Valid: true}
		diff["audit_compliance"] = *req.AuditCompliance
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
		diff["description"] = *req.Description
	}

	updated, err := s.biaSvc.UpdateAssessment(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "BIA assessment not found")
		} else {
			response.InternalError(c, "failed to update BIA assessment")
		}
		return
	}
	s.recordAudit(c, "bia.assessment.updated", "bia", "bia_assessment", updated.ID, diff)

	// Propagate BIA level to linked assets if tier changed
	if req.Tier != nil {
		if err := s.biaSvc.PropagateBIALevel(c.Request.Context(), tenantIDFromContext(c), updated.ID); err != nil {
			zap.L().Error("BIA propagation error", zap.Error(err))
		}
	}

	response.OK(c, toAPIBIAAssessment(*updated))
}

// DeleteBIAAssessment deletes a BIA assessment.
// (DELETE /bia/assessments/{id})
func (s *APIServer) DeleteBIAAssessment(c *gin.Context, id IdPath) {
	err := s.biaSvc.DeleteAssessment(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "BIA assessment not found")
		return
	}
	s.recordAudit(c, "bia.assessment.deleted", "bia", "bia_assessment", uuid.UUID(id), nil)
	c.Status(204)
}

// ListBIAScoringRules returns all scoring rules for the tenant.
// (GET /bia/rules)
func (s *APIServer) ListBIAScoringRules(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rules, err := s.biaSvc.ListRules(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list BIA scoring rules")
		return
	}
	response.OK(c, convertSlice(rules, toAPIBIAScoringRule))
}

// UpdateBIAScoringRule updates an existing scoring rule.
// (PUT /bia/rules/{id})
func (s *APIServer) UpdateBIAScoringRule(c *gin.Context, id IdPath) {
	var req UpdateBIAScoringRuleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateBIAScoringRuleParams{
		ID:       uuid.UUID(id),
		TenantID: tenantIDFromContext(c),
	}
	diff := map[string]any{}
	if req.DisplayName != nil {
		params.DisplayName = pgtype.Text{String: *req.DisplayName, Valid: true}
		diff["display_name"] = *req.DisplayName
	}
	if req.MinScore != nil {
		params.MinScore = pgtype.Int4{Int32: int32(*req.MinScore), Valid: true}
		diff["min_score"] = *req.MinScore
	}
	if req.MaxScore != nil {
		params.MaxScore = pgtype.Int4{Int32: int32(*req.MaxScore), Valid: true}
		diff["max_score"] = *req.MaxScore
	}
	if req.RtoThreshold != nil {
		params.RtoThreshold = float32ToNumeric(*req.RtoThreshold)
		diff["rto_threshold"] = *req.RtoThreshold
	}
	if req.RpoThreshold != nil {
		params.RpoThreshold = float32ToNumeric(*req.RpoThreshold)
		diff["rpo_threshold"] = *req.RpoThreshold
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
		diff["description"] = *req.Description
	}
	if req.Color != nil {
		params.Color = pgtype.Text{String: *req.Color, Valid: true}
		diff["color"] = *req.Color
	}

	updated, err := s.biaSvc.UpdateRule(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "BIA scoring rule not found")
		} else {
			response.InternalError(c, "failed to update BIA scoring rule")
		}
		return
	}
	s.recordAudit(c, "bia.rule.updated", "bia", "bia_scoring_rule", updated.ID, diff)
	response.OK(c, toAPIBIAScoringRule(*updated))
}

// ListBIADependencies returns dependencies for a BIA assessment.
// (GET /bia/assessments/{id}/dependencies)
func (s *APIServer) ListBIADependencies(c *gin.Context, id IdPath) {
	deps, err := s.biaSvc.ListDependencies(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list BIA dependencies")
		return
	}
	response.OK(c, convertSlice(deps, toAPIBIADependency))
}

// CreateBIADependency adds a dependency to a BIA assessment.
// (POST /bia/assessments/{id}/dependencies)
//
// The service layer runs the INSERT and tier propagation inside a single
// transaction, so a propagation failure rolls back the new dependency. If
// the assessment belongs to a different tenant, the service returns
// bia.ErrNotFound and we surface a 404 rather than leaking existence.
func (s *APIServer) CreateBIADependency(c *gin.Context, id IdPath) {
	var req CreateBIADependencyJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	params := dbgen.CreateBIADependencyParams{
		TenantID:       tenantID,
		AssessmentID:   uuid.UUID(id),
		AssetID:        uuid.UUID(req.AssetId),
		DependencyType: req.DependencyType,
	}
	if req.Criticality != nil {
		params.Criticality = pgtype.Text{String: *req.Criticality, Valid: true}
	}

	created, err := s.biaSvc.CreateDependency(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, bia.ErrNotFound) {
			response.NotFound(c, "BIA assessment not found")
			return
		}
		response.InternalError(c, "failed to create BIA dependency")
		return
	}
	s.recordAudit(c, "bia.dependency.created", "bia", "bia_dependency", created.ID, map[string]any{
		"assessment_id":   uuid.UUID(id).String(),
		"asset_id":        uuid.UUID(req.AssetId).String(),
		"dependency_type": req.DependencyType,
	})
	response.Created(c, toAPIBIADependency(*created))
}

// DeleteBIADependency removes a dependency from a BIA assessment.
// (DELETE /bia/assessments/{id}/dependencies/{depId})
//
// The service layer verifies tenant ownership of the dependency, deletes it,
// and recomputes the bia_level of the affected asset — all inside a single
// transaction so a propagation failure rolls back the delete. 404 is
// returned when the dependency does not exist within the caller's tenant.
func (s *APIServer) DeleteBIADependency(c *gin.Context, id IdPath, depId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	assessmentID := uuid.UUID(id)
	dependencyID := uuid.UUID(depId)

	// Tenant-scoped assessment existence check. Prevents callers in tenant A
	// from hitting a dependency id in tenant B through the path.
	if _, err := s.biaSvc.GetAssessment(c.Request.Context(), tenantID, assessmentID); err != nil {
		response.NotFound(c, "BIA assessment not found")
		return
	}

	if err := s.biaSvc.DeleteDependency(c.Request.Context(), tenantID, dependencyID); err != nil {
		if errors.Is(err, bia.ErrNotFound) {
			response.NotFound(c, "BIA dependency not found")
			return
		}
		response.InternalError(c, "failed to delete BIA dependency")
		return
	}
	s.recordAudit(c, "bia.dependency.deleted", "bia", "bia_dependency", dependencyID, map[string]any{
		"assessment_id": assessmentID.String(),
	})
	// AbortWithStatus forces the header to flush immediately — matches the
	// project's Logout handler convention and keeps 204 observable in tests.
	c.AbortWithStatus(http.StatusNoContent)
}

// GetBIAStats returns aggregated BIA statistics.
// (GET /bia/stats)
func (s *APIServer) GetBIAStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	stats, err := s.biaSvc.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get BIA stats")
		return
	}

	total := int(stats.Total)
	avgCompliance := float32(stats.AvgCompliance)
	dataCompliant := int(stats.DataCompliant)
	assetCompliant := int(stats.AssetCompliant)
	auditCompliant := int(stats.AuditCompliant)
	byTier := make(map[string]int)
	for k, v := range stats.ByTier {
		byTier[k] = int(v)
	}

	response.OK(c, BIAStats{
		Total:          &total,
		AvgCompliance:  &avgCompliance,
		DataCompliant:  &dataCompliant,
		AssetCompliant: &assetCompliant,
		AuditCompliant: &auditCompliant,
		ByTier:         &byTier,
	})
}

// ---------------------------------------------------------------------------
// BIA Impact
// ---------------------------------------------------------------------------

// GetBIAImpact returns the BIA assessments impacted by a given asset.
// (GET /bia/impact/{id})
func (s *APIServer) GetBIAImpact(c *gin.Context, id IdPath) {
	assessments, err := s.biaSvc.GetImpactedAssessments(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		// Table may not exist yet (migration not run) — return empty array instead of 500.
		// Audit C HIGH (2026-04-28): swallowing the error masks real failures from
		// operators; log it so problems surface.
		zap.L().Warn("GetBIAImpact: query failed; returning empty list", zap.Error(err))
		response.OK(c, []any{})
		return
	}
	response.OK(c, convertSlice(assessments, toAPIBIAAssessment))
}
