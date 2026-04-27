package api

import (
	"errors"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/change"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// Wave 5.3: Change (ITIL Change Management) handlers + CAB approval.
//
// State machine + auto-approval logic lives in change.Service. Handlers
// translate HTTP concerns: JSON parsing, status code mapping (404 / 409 /
// 400), and audit-event emission.
// ---------------------------------------------------------------------------

// ListChanges — GET /changes
func (s *APIServer) ListChanges(c *gin.Context, params ListChangesParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)
	rows, total, err := s.changeSvc.List(c.Request.Context(), tenantID, params.Status, params.Type, params.Risk, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list changes")
		return
	}
	out := make([]Change, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIChange(r))
	}
	response.OKList(c, out, page, pageSize, int(total))
}

// CreateChange — POST /changes
func (s *APIServer) CreateChange(c *gin.Context) {
	var body CreateChangeJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" {
		response.BadRequest(c, "title is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	p := change.CreateParams{
		TenantID:    tenantID,
		Title:       body.Title,
		RequestedBy: userID,
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.Type != nil {
		p.Type = string(*body.Type)
	}
	if body.Risk != nil {
		p.Risk = string(*body.Risk)
	}
	if body.ApprovalThreshold != nil {
		p.ApprovalThreshold = int32(*body.ApprovalThreshold)
	}
	if body.AssigneeUserId != nil {
		p.AssigneeID = uuid.UUID(*body.AssigneeUserId)
	}
	if body.PlannedStart != nil {
		p.PlannedStart = &pgtype.Timestamptz{Time: *body.PlannedStart, Valid: true}
	}
	if body.PlannedEnd != nil {
		p.PlannedEnd = &pgtype.Timestamptz{Time: *body.PlannedEnd, Valid: true}
	}
	if body.RollbackPlan != nil {
		p.RollbackPlan = *body.RollbackPlan
	}
	if body.ImpactSummary != nil {
		p.ImpactSummary = *body.ImpactSummary
	}

	created, err := s.changeSvc.Create(c.Request.Context(), p)
	if err != nil {
		if strings.Contains(err.Error(), "title required") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to create change")
		return
	}
	s.recordAudit(c, "change.created", "monitoring", "change", created.ID, map[string]any{
		"title": created.Title, "type": created.Type, "risk": created.Risk,
	})
	response.Created(c, toAPIChange(*created))
}

// GetChange — GET /changes/{id}
func (s *APIServer) GetChange(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	row, err := s.changeSvc.Get(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, change.ErrNotFound) {
			response.NotFound(c, "change not found")
			return
		}
		response.InternalError(c, "failed to load change")
		return
	}
	response.OK(c, toAPIChange(*row))
}

// UpdateChange — PUT /changes/{id}
func (s *APIServer) UpdateChange(c *gin.Context, id IdPath) {
	var body UpdateChangeJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tenantID := tenantIDFromContext(c)
	p := change.UpdateParams{TenantID: tenantID, ID: uuid.UUID(id)}
	if body.Title != nil {
		p.Title = body.Title
	}
	if body.Description != nil {
		p.Description = body.Description
	}
	if body.Type != nil {
		p.Type = body.Type
	}
	if body.Risk != nil {
		p.Risk = body.Risk
	}
	if body.ApprovalThreshold != nil {
		v := int32(*body.ApprovalThreshold)
		p.ApprovalThreshold = &v
	}
	if body.AssigneeUserId != nil {
		u := uuid.UUID(*body.AssigneeUserId)
		p.AssigneeID = &u
	}
	if body.PlannedStart != nil {
		p.PlannedStart = &pgtype.Timestamptz{Time: *body.PlannedStart, Valid: true}
	}
	if body.PlannedEnd != nil {
		p.PlannedEnd = &pgtype.Timestamptz{Time: *body.PlannedEnd, Valid: true}
	}
	if body.RollbackPlan != nil {
		p.RollbackPlan = body.RollbackPlan
	}
	if body.ImpactSummary != nil {
		p.ImpactSummary = body.ImpactSummary
	}
	updated, err := s.changeSvc.Update(c.Request.Context(), p)
	if err != nil {
		if errors.Is(err, change.ErrNotFound) {
			response.NotFound(c, "change not found")
			return
		}
		response.InternalError(c, "failed to update change")
		return
	}
	s.recordAudit(c, "change.updated", "monitoring", "change", updated.ID, nil)
	response.OK(c, toAPIChange(*updated))
}

// ---------------------------------------------------------------------------
// Lifecycle endpoints.
// ---------------------------------------------------------------------------

func (s *APIServer) SubmitChange(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.changeSvc.Submit(c.Request.Context(), tenantID, uuid.UUID(id), userID)
	s.writeChangeLifecycle(c, updated, err, "change.submitted")
}

func (s *APIServer) StartChange(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.changeSvc.Start(c.Request.Context(), tenantID, uuid.UUID(id), userID)
	s.writeChangeLifecycle(c, updated, err, "change.started")
}

func (s *APIServer) MarkChangeSucceeded(c *gin.Context, id IdPath) {
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.changeSvc.MarkSucceeded(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Note)
	s.writeChangeLifecycle(c, updated, err, "change.succeeded")
}

func (s *APIServer) MarkChangeFailed(c *gin.Context, id IdPath) {
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.changeSvc.MarkFailed(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Note)
	s.writeChangeLifecycle(c, updated, err, "change.failed")
}

func (s *APIServer) MarkChangeRolledBack(c *gin.Context, id IdPath) {
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.changeSvc.MarkRolledBack(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Note)
	s.writeChangeLifecycle(c, updated, err, "change.rolled_back")
}

func (s *APIServer) writeChangeLifecycle(c *gin.Context, updated *dbgen.Change, err error, action string) {
	if err != nil {
		if errors.Is(err, change.ErrInvalidStateTransition) {
			response.Err(c, 409, "CHANGE_INVALID_TRANSITION", err.Error())
			return
		}
		if errors.Is(err, change.ErrNotFound) {
			response.NotFound(c, "change not found")
			return
		}
		response.InternalError(c, "failed to transition change")
		return
	}
	s.recordAudit(c, action, "monitoring", "change", updated.ID, map[string]any{
		"status": updated.Status,
	})
	response.OK(c, toAPIChange(*updated))
}

// ---------------------------------------------------------------------------
// CAB approvals.
// ---------------------------------------------------------------------------

// ListChangeApprovals — GET /changes/{id}/votes
func (s *APIServer) ListChangeApprovals(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.changeSvc.ListApprovals(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list approvals")
		return
	}
	out := make([]ChangeApproval, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIChangeApproval(r))
	}
	response.OK(c, out)
}

// CastChangeVote — POST /changes/{id}/votes
func (s *APIServer) CastChangeVote(c *gin.Context, id IdPath) {
	var body CastChangeVoteJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "vote is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	if userID == uuid.Nil {
		response.Err(c, 401, "UNAUTHORIZED", "user_id missing from context")
		return
	}
	note := ""
	if body.Note != nil {
		note = *body.Note
	}
	updated, err := s.changeSvc.CastVote(c.Request.Context(), tenantID, uuid.UUID(id), userID, string(body.Vote), note)
	if err != nil {
		if errors.Is(err, change.ErrNotFound) {
			response.NotFound(c, "change not found")
			return
		}
		if errors.Is(err, change.ErrInvalidStateTransition) {
			response.Err(c, 409, "CHANGE_INVALID_TRANSITION",
				"votes are only accepted while the change is in 'submitted' state")
			return
		}
		if strings.Contains(err.Error(), "invalid vote") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to record vote")
		return
	}
	s.recordAudit(c, "change.vote_cast", "monitoring", "change", uuid.UUID(id), map[string]any{
		"vote": body.Vote, "resulting_status": updated.Status,
	})
	response.OK(c, toAPIChange(*updated))
}

// ---------------------------------------------------------------------------
// Comments.
// ---------------------------------------------------------------------------

func (s *APIServer) ListChangeComments(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.changeSvc.ListComments(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list comments")
		return
	}
	out := make([]ChangeComment, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIChangeComment(r))
	}
	response.OK(c, out)
}

func (s *APIServer) CreateChangeComment(c *gin.Context, id IdPath) {
	var body struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Body) == "" {
		response.BadRequest(c, "comment body is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	row, err := s.changeSvc.AddComment(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Body)
	if err != nil {
		response.InternalError(c, "failed to add comment")
		return
	}
	response.Created(c, toAPIChangeCommentFromRecord(*row))
}

// ---------------------------------------------------------------------------
// Linkage handlers.
// ---------------------------------------------------------------------------

func (s *APIServer) LinkChangeAsset(c *gin.Context, id IdPath, assetId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	if err := s.changeSvc.LinkAsset(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(assetId)); err != nil {
		s.writeLinkErr(c, err)
		return
	}
	c.Status(204)
}

func (s *APIServer) UnlinkChangeAsset(c *gin.Context, id IdPath, assetId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	_ = s.changeSvc.UnlinkAsset(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(assetId))
	c.Status(204)
}

func (s *APIServer) ListAssetsForChange(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.changeSvc.ListAssets(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list assets")
		return
	}
	out := make([]Asset, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIAsset(r))
	}
	response.OK(c, out)
}

func (s *APIServer) LinkChangeService(c *gin.Context, id IdPath, serviceId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	if err := s.changeSvc.LinkService(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(serviceId)); err != nil {
		s.writeLinkErr(c, err)
		return
	}
	c.Status(204)
}

func (s *APIServer) UnlinkChangeService(c *gin.Context, id IdPath, serviceId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	_ = s.changeSvc.UnlinkService(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(serviceId))
	c.Status(204)
}

func (s *APIServer) ListServicesForChange(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.changeSvc.ListServices(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list services")
		return
	}
	out := make([]Service, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIService(r))
	}
	response.OK(c, out)
}

func (s *APIServer) LinkChangeProblem(c *gin.Context, id IdPath, problemId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	if err := s.changeSvc.LinkProblem(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(problemId)); err != nil {
		s.writeLinkErr(c, err)
		return
	}
	c.Status(204)
}

func (s *APIServer) UnlinkChangeProblem(c *gin.Context, id IdPath, problemId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	_ = s.changeSvc.UnlinkProblem(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(problemId))
	c.Status(204)
}

func (s *APIServer) ListProblemsForChange(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.changeSvc.ListProblems(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list problems")
		return
	}
	out := make([]Problem, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIProblem(r))
	}
	response.OK(c, out)
}

// ListChangesForProblem — GET /problems/{id}/changes
func (s *APIServer) ListChangesForProblem(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.changeSvc.ListChangesForProblem(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list changes")
		return
	}
	out := make([]Change, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIChange(r))
	}
	response.OK(c, out)
}

func (s *APIServer) writeLinkErr(c *gin.Context, err error) {
	if errors.Is(err, change.ErrNotFound) {
		response.NotFound(c, "change or linked entity not found")
		return
	}
	response.InternalError(c, "failed to link")
}
