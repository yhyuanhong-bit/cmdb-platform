package api

import (
	"encoding/json"

	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// Prediction endpoints
// ---------------------------------------------------------------------------

// ListPredictionModels returns all prediction models.
// (GET /prediction/models)
func (s *APIServer) ListPredictionModels(c *gin.Context) {
	models, err := s.predictionSvc.ListModels(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to list prediction models")
		return
	}
	response.OK(c, convertSlice(models, toAPIPredictionModel))
}

// ListPredictionsByAsset returns prediction results for an asset.
// (GET /prediction/results/ci/{ciId})
func (s *APIServer) ListPredictionsByAsset(c *gin.Context, ciId openapi_types.UUID) {
	results, err := s.predictionSvc.ListByAsset(c.Request.Context(), uuid.UUID(ciId), 50)
	if err != nil {
		response.InternalError(c, "failed to list predictions")
		return
	}
	response.OK(c, convertSlice(results, toAPIPredictionResult))
}

// CreateRCA triggers a root-cause analysis.
// (POST /prediction/rca)
func (s *APIServer) CreateRCA(c *gin.Context) {
	var req CreateRCAJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	modelName := ""
	if req.ModelName != nil {
		modelName = *req.ModelName
	}

	var contextStr string
	if req.Context != nil {
		b, _ := json.Marshal(*req.Context)
		contextStr = string(b)
	}

	rca, err := s.predictionSvc.CreateRCA(c.Request.Context(), tenantID, prediction.CreateRCARequest{
		IncidentID: uuid.UUID(req.IncidentId),
		ModelName:  modelName,
		Context:    contextStr,
	})
	if err != nil {
		response.InternalError(c, "failed to create RCA")
		return
	}
	s.recordAudit(c, "rca.created", "prediction", "rca", rca.ID, map[string]any{
		"incident_id": rca.IncidentID,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectPredictionCreated, tenantID.String(), map[string]any{
		"rca_id": rca.ID.String(), "incident_id": rca.IncidentID.String(),
	})
	response.Created(c, toAPIRCAAnalysis(*rca))
}

// VerifyRCA marks an RCA as human-verified.
// (POST /prediction/rca/{id}/verify)
func (s *APIServer) VerifyRCA(c *gin.Context, id IdPath) {
	var req VerifyRCAJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	rca, err := s.predictionSvc.VerifyRCA(c.Request.Context(), uuid.UUID(id), uuid.UUID(req.VerifiedBy))
	if err != nil {
		response.NotFound(c, "RCA not found")
		return
	}
	s.recordAudit(c, "rca.verified", "prediction", "rca", rca.ID, map[string]any{
		"verified_by": uuid.UUID(req.VerifiedBy),
	})
	response.OK(c, toAPIRCAAnalysis(*rca))
}
