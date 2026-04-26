package api

import (
	"errors"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/problem"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// Wave 5.2: Problem (ITIL) handlers.
//
// Same shape as Wave 5.1 incident lifecycle: dedicated POST endpoint per
// transition, 409 on illegal source state, single tx for UPDATE + system
// comment so the timeline never drifts. The domain layer does the work;
// these handlers translate HTTP concerns (JSON parsing, status codes).
// ---------------------------------------------------------------------------

// ListProblems — GET /problems
func (s *APIServer) ListProblems(c *gin.Context, params ListProblemsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)
	rows, total, err := s.problemSvc.List(c.Request.Context(), tenantID, params.Status, params.Priority, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list problems")
		return
	}
	out := make([]Problem, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIProblem(r))
	}
	response.OKList(c, out, page, pageSize, int(total))
}

// CreateProblem — POST /problems
func (s *APIServer) CreateProblem(c *gin.Context) {
	var body CreateProblemJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" {
		response.BadRequest(c, "title is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	p := problem.CreateParams{
		TenantID:  tenantID,
		Title:     body.Title,
		CreatedBy: userID,
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.Severity != nil {
		p.Severity = *body.Severity
	}
	if body.Priority != nil {
		p.Priority = *body.Priority
	}
	if body.Workaround != nil {
		p.Workaround = *body.Workaround
	}
	if body.AssigneeUserId != nil {
		p.AssigneeID = uuid.UUID(*body.AssigneeUserId)
	}

	created, err := s.problemSvc.Create(c.Request.Context(), p)
	if err != nil {
		// CHECK constraints surface as opaque errors; map known patterns.
		if strings.Contains(err.Error(), "title required") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to create problem")
		return
	}
	s.recordAudit(c, "problem.created", "monitoring", "problem", created.ID, map[string]any{
		"title": created.Title,
	})
	response.Created(c, toAPIProblem(*created))
}

// GetProblem — GET /problems/{id}
func (s *APIServer) GetProblem(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	p, err := s.problemSvc.Get(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, problem.ErrNotFound) {
			response.NotFound(c, "problem not found")
			return
		}
		response.InternalError(c, "failed to load problem")
		return
	}
	response.OK(c, toAPIProblem(*p))
}

// UpdateProblem — PUT /problems/{id}
func (s *APIServer) UpdateProblem(c *gin.Context, id IdPath) {
	var body UpdateProblemJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tenantID := tenantIDFromContext(c)
	p := problem.UpdateParams{TenantID: tenantID, ID: uuid.UUID(id)}
	if body.Title != nil {
		p.Title = body.Title
	}
	if body.Description != nil {
		p.Description = body.Description
	}
	if body.Severity != nil {
		p.Severity = body.Severity
	}
	if body.Priority != nil {
		p.Priority = body.Priority
	}
	if body.Workaround != nil {
		p.Workaround = body.Workaround
	}
	if body.RootCause != nil {
		p.RootCause = body.RootCause
	}
	if body.Resolution != nil {
		p.Resolution = body.Resolution
	}
	if body.AssigneeUserId != nil {
		u := uuid.UUID(*body.AssigneeUserId)
		p.AssigneeID = &u
	}
	updated, err := s.problemSvc.Update(c.Request.Context(), p)
	if err != nil {
		if errors.Is(err, problem.ErrNotFound) {
			response.NotFound(c, "problem not found")
			return
		}
		response.InternalError(c, "failed to update problem")
		return
	}
	s.recordAudit(c, "problem.updated", "monitoring", "problem", updated.ID, nil)
	response.OK(c, toAPIProblem(*updated))
}

// ---------------------------------------------------------------------------
// Lifecycle endpoints. The writeProblemLifecycleResponse helper centralises
// the 409/404 mapping so each transition stays a one-liner.
// ---------------------------------------------------------------------------

func (s *APIServer) StartInvestigatingProblem(c *gin.Context, id IdPath) {
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.problemSvc.StartInvestigation(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Note)
	s.writeProblemLifecycleResponse(c, updated, err, "problem.investigating")
}

func (s *APIServer) MarkProblemKnownError(c *gin.Context, id IdPath) {
	var body struct {
		Workaround string `json:"workaround" binding:"required"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Workaround) == "" {
		response.BadRequest(c, "workaround is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.problemSvc.MarkKnownError(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Workaround, body.Note)
	if err != nil && strings.Contains(err.Error(), "workaround is required") {
		response.BadRequest(c, err.Error())
		return
	}
	s.writeProblemLifecycleResponse(c, updated, err, "problem.known_error")
}

func (s *APIServer) ResolveProblem(c *gin.Context, id IdPath) {
	var body struct {
		RootCause  string `json:"root_cause"`
		Resolution string `json:"resolution"`
		Note       string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.problemSvc.Resolve(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.RootCause, body.Resolution, body.Note)
	s.writeProblemLifecycleResponse(c, updated, err, "problem.resolved")
}

func (s *APIServer) CloseProblem(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.problemSvc.Close(c.Request.Context(), tenantID, uuid.UUID(id), userID)
	s.writeProblemLifecycleResponse(c, updated, err, "problem.closed")
}

func (s *APIServer) ReopenProblem(c *gin.Context, id IdPath) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	updated, err := s.problemSvc.Reopen(c.Request.Context(), tenantID, uuid.UUID(id), userID, body.Reason)
	s.writeProblemLifecycleResponse(c, updated, err, "problem.reopened")
}

func (s *APIServer) writeProblemLifecycleResponse(c *gin.Context, updated *dbgen.Problem, err error, action string) {
	if err != nil {
		if errors.Is(err, problem.ErrInvalidStateTransition) {
			response.Err(c, 409, "PROBLEM_INVALID_TRANSITION", err.Error())
			return
		}
		if errors.Is(err, problem.ErrNotFound) {
			response.NotFound(c, "problem not found")
			return
		}
		response.InternalError(c, "failed to transition problem")
		return
	}
	s.recordAudit(c, action, "monitoring", "problem", updated.ID, map[string]any{
		"status": updated.Status,
	})
	response.OK(c, toAPIProblem(*updated))
}

// ---------------------------------------------------------------------------
// Comments.
// ---------------------------------------------------------------------------

func (s *APIServer) ListProblemComments(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.problemSvc.ListComments(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list comments")
		return
	}
	out := make([]ProblemComment, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIProblemComment(r))
	}
	response.OK(c, out)
}

func (s *APIServer) CreateProblemComment(c *gin.Context, id IdPath) {
	var body struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Body) == "" {
		response.BadRequest(c, "comment body is required")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	row, err := s.problemSvc.AddComment(c.Request.Context(), tenantID, uuid.UUID(id), userID, "human", body.Body)
	if err != nil {
		response.InternalError(c, "failed to add comment")
		return
	}
	response.Created(c, toAPIProblemCommentFromRecord(*row))
}

// ---------------------------------------------------------------------------
// Linkage with incidents.
// ---------------------------------------------------------------------------

// LinkIncidentToProblem — POST /monitoring/incidents/{id}/problems/{problemId}
func (s *APIServer) LinkIncidentToProblem(c *gin.Context, id IdPath, problemId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	if err := s.problemSvc.LinkIncident(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(problemId), userID); err != nil {
		if errors.Is(err, problem.ErrNotFound) {
			response.NotFound(c, "incident or problem not found")
			return
		}
		response.InternalError(c, "failed to link")
		return
	}
	s.recordAudit(c, "incident.linked_to_problem", "monitoring", "incident", uuid.UUID(id), map[string]any{
		"problem_id": uuid.UUID(problemId).String(),
	})
	c.Status(204)
}

// UnlinkIncidentFromProblem — DELETE /monitoring/incidents/{id}/problems/{problemId}
func (s *APIServer) UnlinkIncidentFromProblem(c *gin.Context, id IdPath, problemId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	if err := s.problemSvc.UnlinkIncident(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(problemId)); err != nil {
		response.InternalError(c, "failed to unlink")
		return
	}
	s.recordAudit(c, "incident.unlinked_from_problem", "monitoring", "incident", uuid.UUID(id), map[string]any{
		"problem_id": uuid.UUID(problemId).String(),
	})
	c.Status(204)
}

// ListIncidentsForProblem — GET /problems/{id}/incidents
func (s *APIServer) ListIncidentsForProblem(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.problemSvc.ListIncidentsForProblem(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list incidents")
		return
	}
	out := make([]Incident, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIIncident(r))
	}
	response.OK(c, out)
}

// ListProblemsForIncident — GET /monitoring/incidents/{id}/problems
func (s *APIServer) ListProblemsForIncident(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.problemSvc.ListProblemsForIncident(c.Request.Context(), tenantID, uuid.UUID(id))
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
