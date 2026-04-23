package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/service"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Service handlers. The API server delegates to domain/service which owns
// all validation + event publishing; these handlers translate between
// HTTP payloads and the domain contract only.

// ListServices — GET /services
func (s *APIServer) ListServices(c *gin.Context, params ListServicesParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize := paginationFrom(params.Page, params.PageSize)

	var tier, status, ownerTeam *string
	if params.Tier != nil {
		v := string(*params.Tier)
		tier = &v
	}
	if params.Status != nil {
		v := string(*params.Status)
		status = &v
	}
	if params.OwnerTeam != nil {
		ownerTeam = params.OwnerTeam
	}

	items, total, err := s.serviceSvc.List(c.Request.Context(), tenantID, tier, status, ownerTeam, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       convertSlice(items, toAPIService),
		"pagination": paginationMeta(page, pageSize, int(total)),
		"meta":       metaFrom(c),
	})
}

// CreateService — POST /services
func (s *APIServer) CreateService(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)

	var body CreateServiceRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	p := service.CreateParams{
		TenantID:  tenantID,
		Code:      body.Code,
		Name:      body.Name,
		CreatedBy: userID,
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.Tier != nil {
		p.Tier = string(*body.Tier)
	}
	if body.OwnerTeam != nil {
		p.OwnerTeam = *body.OwnerTeam
	}
	if body.BiaAssessmentId != nil {
		id := uuid.UUID(*body.BiaAssessmentId)
		p.BIAAssessmentID = &id
	}
	if body.Tags != nil {
		p.Tags = *body.Tags
	}

	created, err := s.serviceSvc.Create(c.Request.Context(), p)
	switch {
	case errors.Is(err, service.ErrInvalidCode):
		response.BadRequest(c, err.Error())
		return
	case errors.Is(err, service.ErrInvalidTier):
		response.BadRequest(c, err.Error())
		return
	case errors.Is(err, service.ErrDuplicateCode):
		response.Err(c, http.StatusConflict, "DUPLICATE_SERVICE_CODE", err.Error())
		return
	case err != nil:
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": toAPIService(created), "meta": metaFrom(c)})
}

// GetService — GET /services/{id}
func (s *APIServer) GetService(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	svc, err := s.serviceSvc.GetByID(c.Request.Context(), tenantID, uuid.UUID(id))
	if errors.Is(err, service.ErrNotFound) {
		response.NotFound(c, "service not found")
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, toAPIService(svc))
}

// UpdateService — PUT /services/{id}
func (s *APIServer) UpdateService(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	var body UpdateServiceRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	p := service.UpdateParams{TenantID: tenantID, ID: uuid.UUID(id)}
	if body.Name != nil {
		p.Name = body.Name
	}
	if body.Description != nil {
		p.Description = body.Description
	}
	if body.Tier != nil {
		v := string(*body.Tier)
		p.Tier = &v
	}
	if body.OwnerTeam != nil {
		p.OwnerTeam = body.OwnerTeam
	}
	if body.BiaAssessmentId != nil {
		bid := uuid.UUID(*body.BiaAssessmentId)
		p.BIAAssessmentID = &bid
	}
	if body.Status != nil {
		v := string(*body.Status)
		p.Status = &v
	}
	if body.Tags != nil {
		tags := *body.Tags
		p.Tags = &tags
	}

	updated, err := s.serviceSvc.Update(c.Request.Context(), p)
	switch {
	case errors.Is(err, service.ErrNotFound):
		response.NotFound(c, "service not found")
		return
	case errors.Is(err, service.ErrInvalidTier), errors.Is(err, service.ErrInvalidStatus):
		response.BadRequest(c, err.Error())
		return
	case err != nil:
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, toAPIService(updated))
}

// DeleteService — DELETE /services/{id}
func (s *APIServer) DeleteService(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	if err := s.serviceSvc.Delete(c.Request.Context(), tenantID, uuid.UUID(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ListServiceAssets — GET /services/{id}/assets
func (s *APIServer) ListServiceAssets(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.serviceSvc.ListAssets(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	out := make([]ServiceAssetMember, 0, len(rows))
	for _, r := range rows {
		out = append(out, ServiceAssetMember{
			ServiceId:   openapi_types.UUID(r.ServiceID),
			AssetId:     openapi_types.UUID(r.AssetID),
			AssetTag:    stringPtr(r.AssetTag),
			AssetName:   stringPtr(r.AssetName),
			AssetStatus: stringPtr(r.AssetStatus),
			AssetType:   stringPtr(r.AssetType),
			Role:        ServiceAssetMemberRole(r.Role),
			IsCritical:  r.IsCritical,
			CreatedAt:   r.CreatedAt,
		})
	}
	response.OK(c, out)
}

// AddServiceAsset — POST /services/{id}/assets
func (s *APIServer) AddServiceAsset(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)

	var body AddServiceAssetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	assetID := uuid.UUID(body.AssetId)
	role := string(service.RoleComponent)
	if body.Role != nil {
		role = string(*body.Role)
	}
	isCritical := false
	if body.IsCritical != nil {
		isCritical = *body.IsCritical
	}

	sa, err := s.serviceSvc.AddAsset(c.Request.Context(), tenantID, uuid.UUID(id), assetID, role, isCritical, userID)
	switch {
	case errors.Is(err, service.ErrInvalidRole):
		response.BadRequest(c, err.Error())
		return
	case errors.Is(err, service.ErrNotFound):
		response.NotFound(c, "service not found")
		return
	case errors.Is(err, service.ErrAssetNotInTenant):
		// Return 404 (not 403) to avoid leaking existence of assets in
		// other tenants to UUID-probe attacks.
		response.NotFound(c, "asset not found")
		return
	case err != nil:
		if strings.Contains(err.Error(), "foreign key") {
			response.NotFound(c, "service or asset not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": ServiceAssetMember{
			ServiceId:  openapi_types.UUID(sa.ServiceID),
			AssetId:    openapi_types.UUID(sa.AssetID),
			Role:       ServiceAssetMemberRole(sa.Role),
			IsCritical: sa.IsCritical,
			CreatedAt:  sa.CreatedAt,
		},
		"meta": metaFrom(c),
	})
}

// RemoveServiceAsset — DELETE /services/{id}/assets/{assetId}
func (s *APIServer) RemoveServiceAsset(c *gin.Context, id IdPath, assetId openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	if err := s.serviceSvc.RemoveAsset(c.Request.Context(), tenantID, uuid.UUID(id), uuid.UUID(assetId)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// GetServiceHealth — GET /services/{id}/health
func (s *APIServer) GetServiceHealth(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	svcID := uuid.UUID(id)
	hs, total, bad, err := s.serviceSvc.Health(c.Request.Context(), tenantID, svcID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, ServiceHealth{
		ServiceId:         openapi_types.UUID(svcID),
		Status:            ServiceHealthStatus(hs),
		CriticalTotal:     int(total),
		CriticalUnhealthy: int(bad),
	})
}

// ListServicesForAsset — GET /assets/{id}/services (reverse lookup)
func (s *APIServer) ListServicesForAsset(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.serviceSvc.ServicesForAsset(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	out := make([]AssetServiceMembership, 0, len(rows))
	for _, r := range rows {
		out = append(out, AssetServiceMembership{
			Id:         openapi_types.UUID(r.ID),
			Code:       r.Code,
			Name:       r.Name,
			Tier:       AssetServiceMembershipTier(r.Tier),
			Status:     AssetServiceMembershipStatus(r.Status),
			Role:       r.Role,
			IsCritical: r.IsCritical,
		})
	}
	response.OK(c, out)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// stringPtr returns &s for non-empty strings, nil for empty — matches
// the openapi generated nullable-string shape.
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// paginationFrom extracts page / page_size from optional pointers with
// the project-standard defaults (page=1, size=50, cap 500).
func paginationFrom(page, pageSize *int) (int, int) {
	p := 1
	if page != nil && *page > 0 {
		p = *page
	}
	ps := 50
	if pageSize != nil && *pageSize > 0 {
		ps = *pageSize
	}
	if ps > 500 {
		ps = 500
	}
	return p, ps
}

// paginationMeta builds the standard pagination envelope.
func paginationMeta(page, pageSize, total int) gin.H {
	return gin.H{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	}
}

// metaFrom returns the standard meta object carrying request_id.
func metaFrom(c *gin.Context) gin.H {
	rid := c.GetString("request_id")
	if rid == "" {
		rid = c.Request.Header.Get("X-Request-Id")
	}
	return gin.H{"request_id": rid}
}

// toAPIService converts a dbgen.Service into the OpenAPI shape.
func toAPIService(s dbgen.Service) Service {
	out := Service{
		Id:          openapi_types.UUID(s.ID),
		TenantId:    openapi_types.UUID(s.TenantID),
		Code:        s.Code,
		Name:        s.Name,
		Tier:        ServiceTier(s.Tier),
		Status:      ServiceStatus(s.Status),
		Tags:        s.Tags,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		SyncVersion: s.SyncVersion,
	}
	if s.Description.Valid {
		v := s.Description.String
		out.Description = &v
	}
	if s.OwnerTeam.Valid {
		v := s.OwnerTeam.String
		out.OwnerTeam = &v
	}
	if s.BiaAssessmentID.Valid {
		v := openapi_types.UUID(s.BiaAssessmentID.Bytes)
		out.BiaAssessmentId = &v
	}
	if s.CreatedBy.Valid {
		v := openapi_types.UUID(s.CreatedBy.Bytes)
		out.CreatedBy = &v
	}
	return out
}
