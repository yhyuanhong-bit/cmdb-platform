package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/audit"
	"github.com/cmdb-platform/cmdb-core/internal/domain/bia"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Ensure APIServer implements ServerInterface at compile time.
var _ ServerInterface = (*APIServer)(nil)

// APIServer implements every method of the generated ServerInterface,
// delegating business logic to the domain services.
type APIServer struct {
	pool           *pgxpool.Pool
	eventBus       eventbus.Bus
	authSvc        *identity.AuthService
	identitySvc    *identity.Service
	topologySvc    *topology.Service
	assetSvc       *asset.Service
	maintenanceSvc *maintenance.Service
	monitoringSvc  *monitoring.Service
	inventorySvc   *inventory.Service
	auditSvc       *audit.Service
	dashboardSvc   *dashboard.Service
	predictionSvc  *prediction.Service
	integrationSvc *integration.Service
	biaSvc         *bia.Service
	qualitySvc     *quality.Service
	discoverySvc   *discovery.Service
}

// NewAPIServer constructs an APIServer with all required domain services.
func NewAPIServer(
	pool *pgxpool.Pool,
	bus eventbus.Bus,
	authSvc *identity.AuthService,
	identitySvc *identity.Service,
	topologySvc *topology.Service,
	assetSvc *asset.Service,
	maintenanceSvc *maintenance.Service,
	monitoringSvc *monitoring.Service,
	inventorySvc *inventory.Service,
	auditSvc *audit.Service,
	dashboardSvc *dashboard.Service,
	predictionSvc *prediction.Service,
	integrationSvc *integration.Service,
	biaSvc *bia.Service,
	qualitySvc *quality.Service,
	discoverySvc *discovery.Service,
) *APIServer {
	return &APIServer{
		pool:           pool,
		eventBus:       bus,
		authSvc:        authSvc,
		identitySvc:    identitySvc,
		topologySvc:    topologySvc,
		assetSvc:       assetSvc,
		maintenanceSvc: maintenanceSvc,
		monitoringSvc:  monitoringSvc,
		inventorySvc:   inventorySvc,
		auditSvc:       auditSvc,
		dashboardSvc:   dashboardSvc,
		predictionSvc:  predictionSvc,
		integrationSvc: integrationSvc,
		biaSvc:         biaSvc,
		qualitySvc:     qualitySvc,
		discoverySvc:   discoverySvc,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func tenantIDFromContext(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("tenant_id"))
	return id
}

func userIDFromContext(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("user_id"))
	return id
}

func paginationDefaults(page, pageSize *int) (int, int, int32, int32) {
	p := 1
	ps := 20
	if page != nil && *page > 0 {
		p = *page
	}
	if pageSize != nil && *pageSize > 0 {
		ps = *pageSize
		if ps > 100 {
			ps = 100
		}
	}
	offset := (p - 1) * ps
	return p, ps, int32(ps), int32(offset)
}

func uuidPtrFromOAPI(v *openapi_types.UUID) *uuid.UUID {
	if v == nil {
		return nil
	}
	u := uuid.UUID(*v)
	return &u
}

func textFromPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func pguuidFromPtr(v *uuid.UUID) pgtype.UUID {
	if v == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *v, Valid: true}
}

// recordAudit logs an audit event. Errors are logged but don't fail the request.
func (s *APIServer) recordAudit(c *gin.Context, action, module, targetType string, targetID uuid.UUID, diff map[string]any) {
	tenantID := tenantIDFromContext(c)
	operatorID := userIDFromContext(c)
	if err := s.auditSvc.Record(c.Request.Context(), tenantID, action, module, targetType, targetID, operatorID, diff, "api"); err != nil {
		// Log but don't fail the request
		fmt.Printf("audit record error: %v\n", err)
	}
}

// publishEvent publishes a domain event to the event bus. Errors are logged but don't fail the request.
func (s *APIServer) publishEvent(ctx context.Context, subject, tenantID string, payload any) {
	if s.eventBus == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("event marshal error: %v\n", err)
		return
	}
	if err := s.eventBus.Publish(ctx, eventbus.Event{
		Subject:  subject,
		TenantID: tenantID,
		Payload:  data,
	}); err != nil {
		fmt.Printf("event publish error: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// Auth endpoints
// ---------------------------------------------------------------------------

// Login authenticates a user and returns a token pair.
// (POST /auth/login)
func (s *APIServer) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tokens, err := s.authSvc.Login(c.Request.Context(), identity.LoginRequest{
		Username:  req.Username,
		Password:  req.Password,
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// RefreshToken issues a new token pair using a refresh token.
// (POST /auth/refresh)
func (s *APIServer) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tokens, err := s.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// GetCurrentUser returns the authenticated user with merged permissions.
// (GET /auth/me)
func (s *APIServer) GetCurrentUser(c *gin.Context) {
	userID := c.GetString("user_id")
	cu, err := s.authSvc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		response.Unauthorized(c, "failed to get current user")
		return
	}
	response.OK(c, CurrentUser{
		Id:          cu.ID,
		Username:    cu.Username,
		DisplayName: cu.DisplayName,
		Email:       cu.Email,
		Permissions: cu.Permissions,
	})
}

// ---------------------------------------------------------------------------
// Asset endpoints
// ---------------------------------------------------------------------------

// ListAssets returns a paginated, filtered list of assets.
// (GET /assets)
func (s *APIServer) ListAssets(c *gin.Context, params ListAssetsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	assets, total, err := s.assetSvc.List(c.Request.Context(), asset.ListParams{
		TenantID:     tenantID,
		Type:         params.Type,
		Status:       params.Status,
		LocationID:   uuidPtrFromOAPI(params.LocationId),
		RackID:       uuidPtrFromOAPI(params.RackId),
		SerialNumber: params.SerialNumber,
		Search:       params.Search,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		response.InternalError(c, "failed to list assets")
		return
	}

	response.OKList(c, convertSlice(assets, toAPIAsset), page, pageSize, int(total))
}

// CreateAsset creates a new asset.
// (POST /assets)
func (s *APIServer) CreateAsset(c *gin.Context) {
	var req CreateAssetJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var attrsJSON json.RawMessage
	if req.Attributes != nil {
		attrsJSON, _ = json.Marshal(req.Attributes)
	}

	params := dbgen.CreateAssetParams{
		TenantID:       tenantID,
		AssetTag:       req.AssetTag,
		PropertyNumber: textFromPtr(req.PropertyNumber),
		ControlNumber:  textFromPtr(req.ControlNumber),
		Name:           req.Name,
		Type:           req.Type,
		SubType:        pgtype.Text{String: req.SubType, Valid: req.SubType != ""},
		Status:         req.Status,
		BiaLevel:       req.BiaLevel,
		RackID:         pguuidFromPtr(uuidPtrFromOAPI(req.RackId)),
		Vendor:         pgtype.Text{String: req.Vendor, Valid: req.Vendor != ""},
		Model:          pgtype.Text{String: req.Model, Valid: req.Model != ""},
		SerialNumber:   pgtype.Text{String: req.SerialNumber, Valid: req.SerialNumber != ""},
		Attributes:     attrsJSON,
		Tags:           req.Tags,
	}

	created, err := s.assetSvc.Create(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create asset")
		return
	}
	s.recordAudit(c, "asset.created", "asset", "asset", created.ID, map[string]any{
		"asset_tag": created.AssetTag,
		"name":      created.Name,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectAssetCreated, tenantID.String(), map[string]any{
		"asset_id": created.ID.String(), "tenant_id": tenantID.String(), "asset_tag": created.AssetTag, "type": created.Type,
	})

	// CIType soft validation: warn about missing recommended attributes.
	warnings := ciTypeSoftValidation(req.Type, req.Attributes)
	if len(warnings) > 0 {
		c.JSON(201, gin.H{
			"data": toAPIAsset(*created),
			"meta": gin.H{"warnings": warnings, "request_id": c.GetString("request_id")},
		})
		return
	}
	response.Created(c, toAPIAsset(*created))
}

// GetAsset returns a single asset by ID.
// (GET /assets/{id})
func (s *APIServer) GetAsset(c *gin.Context, id IdPath) {
	a, err := s.assetSvc.GetByID(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	response.OK(c, toAPIAsset(*a))
}

// UpdateAsset updates an existing asset.
// (PUT /assets/{id})
func (s *APIServer) UpdateAsset(c *gin.Context, id IdPath) {
	var req UpdateAssetJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateAssetParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.BiaLevel != nil {
		params.BiaLevel = pgtype.Text{String: *req.BiaLevel, Valid: true}
	}
	if req.LocationId != nil {
		u := uuid.UUID(*req.LocationId)
		params.LocationID = pgtype.UUID{Bytes: u, Valid: true}
	}
	if req.RackId != nil {
		u := uuid.UUID(*req.RackId)
		params.RackID = pgtype.UUID{Bytes: u, Valid: true}
	}
	if req.Vendor != nil {
		params.Vendor = pgtype.Text{String: *req.Vendor, Valid: true}
	}
	if req.Model != nil {
		params.Model = pgtype.Text{String: *req.Model, Valid: true}
	}
	if req.SerialNumber != nil {
		params.SerialNumber = pgtype.Text{String: *req.SerialNumber, Valid: true}
	}
	if req.Attributes != nil {
		b, _ := json.Marshal(req.Attributes)
		params.Attributes = b
	}
	if req.Tags != nil {
		params.Tags = *req.Tags
	}

	updated, err := s.assetSvc.Update(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "asset not found")
		} else {
			response.InternalError(c, "failed to update asset")
		}
		return
	}

	// Supplementary update for ip_address (not in sqlc-generated query)
	if req.IpAddress != nil {
		s.pool.Exec(c.Request.Context(),
			"UPDATE assets SET ip_address = $1 WHERE id = $2",
			*req.IpAddress, uuid.UUID(id),
		)
	}

	diff := map[string]any{}
	if req.Name != nil {
		diff["name"] = *req.Name
	}
	if req.Status != nil {
		diff["status"] = *req.Status
	}
	if req.BiaLevel != nil {
		diff["bia_level"] = *req.BiaLevel
	}
	if req.Vendor != nil {
		diff["vendor"] = *req.Vendor
	}
	if req.Model != nil {
		diff["model"] = *req.Model
	}
	if req.SerialNumber != nil {
		diff["serial_number"] = *req.SerialNumber
	}
	if req.IpAddress != nil {
		diff["ip_address"] = *req.IpAddress
	}
	s.recordAudit(c, "asset.updated", "asset", "asset", updated.ID, diff)
	s.publishEvent(c.Request.Context(), eventbus.SubjectAssetUpdated, tenantIDFromContext(c).String(), map[string]any{
		"asset_id": updated.ID.String(), "tenant_id": tenantIDFromContext(c).String(),
	})

	// ITSM Change Audit: Critical assets auto-create change audit work order
	var changeOrderID *uuid.UUID
	if updated.BiaLevel == "critical" {
		tenantID := tenantIDFromContext(c)
		userID := userIDFromContext(c)
		order, err := s.maintenanceSvc.Create(c.Request.Context(), tenantID, userID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Change Audit: %s (%s)", updated.Name, updated.AssetTag),
			Type:        "change_audit",
			Description: "Critical asset modified. Review required.",
			Priority:    "high",
		})
		if err == nil {
			id := order.ID
			changeOrderID = &id
		}
	}

	// Return asset with optional change_order_id in meta
	if changeOrderID != nil {
		c.JSON(200, gin.H{
			"data": toAPIAsset(*updated),
			"meta": gin.H{"change_order_id": changeOrderID.String(), "request_id": c.GetString("request_id")},
		})
		return
	}
	response.OK(c, toAPIAsset(*updated))
}

// DeleteAsset deletes an asset.
// (DELETE /assets/{id})
func (s *APIServer) DeleteAsset(c *gin.Context, id IdPath) {
	err := s.assetSvc.Delete(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	s.recordAudit(c, "asset.deleted", "asset", "asset", uuid.UUID(id), nil)
	s.publishEvent(c.Request.Context(), eventbus.SubjectAssetDeleted, tenantIDFromContext(c).String(), map[string]any{
		"asset_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	c.Status(204)
}

// ---------------------------------------------------------------------------
// Location endpoints
// ---------------------------------------------------------------------------

// ListLocations returns root locations, or looks up a location by slug+level.
// (GET /locations)
func (s *APIServer) ListLocations(c *gin.Context, params ListLocationsParams) {
	tenantID := tenantIDFromContext(c)

	if params.Slug != nil && params.Level != nil {
		loc, err := s.topologySvc.GetBySlug(c.Request.Context(), tenantID, *params.Slug, *params.Level)
		if err != nil {
			response.NotFound(c, "location not found")
			return
		}
		response.OK(c, toAPILocation(*loc))
		return
	}

	locations, err := s.topologySvc.ListRootLocations(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list locations")
		return
	}
	response.OK(c, convertSlice(locations, toAPILocation))
}

// GetLocation returns a single location by ID.
// (GET /locations/{id})
func (s *APIServer) GetLocation(c *gin.Context, id IdPath) {
	loc, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	response.OK(c, toAPILocation(loc))
}

// ListLocationAncestors returns ancestor locations for a given location.
// (GET /locations/{id}/ancestors)
func (s *APIServer) ListLocationAncestors(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)

	loc, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}

	path := pgtextToStr(loc.Path)
	ancestors, err := s.topologySvc.ListAncestors(c.Request.Context(), tenantID, path)
	if err != nil {
		response.InternalError(c, "failed to list ancestors")
		return
	}
	response.OK(c, convertSlice(ancestors, toAPILocation))
}

// ListLocationChildren returns child locations for a given parent.
// (GET /locations/{id}/children)
func (s *APIServer) ListLocationChildren(c *gin.Context, id IdPath) {
	children, err := s.topologySvc.ListChildren(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list children")
		return
	}
	response.OK(c, convertSlice(children, toAPILocation))
}

// ListLocationRacks returns racks at a location.
// (GET /locations/{id}/racks)
func (s *APIServer) ListLocationRacks(c *gin.Context, id IdPath) {
	racks, err := s.topologySvc.ListRacksByLocation(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list racks")
		return
	}
	response.OK(c, convertSlice(racks, toAPIRack))
}

// GetLocationStats returns aggregate statistics for a location.
// (GET /locations/{id}/stats)
func (s *APIServer) GetLocationStats(c *gin.Context, id IdPath) {
	stats, err := s.topologySvc.GetLocationStats(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to get location stats")
		return
	}
	response.OK(c, LocationStats{
		TotalAssets:    int(stats.TotalAssets),
		TotalRacks:     int(stats.TotalRacks),
		CriticalAlerts: int(stats.CriticalAlerts),
		AvgOccupancy:   float32(stats.AvgOccupancy),
	})
}

// CreateLocation creates a new location.
// (POST /locations)
func (s *APIServer) CreateLocation(c *gin.Context) {
	var req CreateLocationJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var metadataJSON json.RawMessage
	if req.Metadata != nil {
		metadataJSON, _ = json.Marshal(req.Metadata)
	}

	var sortOrder int32
	if req.SortOrder != nil {
		sortOrder = int32(*req.SortOrder)
	}

	// Build ltree path: for root locations use slug, for children use parent.path + "." + slug
	path := req.Slug
	if req.ParentId != nil {
		parent, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(*req.ParentId))
		if err != nil {
			response.BadRequest(c, "parent location not found")
			return
		}
		parentPath := pgtextToStr(parent.Path)
		if parentPath != "" {
			path = parentPath + "." + req.Slug
		}
	}

	params := dbgen.CreateLocationParams{
		TenantID:  tenantID,
		Name:      req.Name,
		NameEn:    textFromPtr(req.NameEn),
		Slug:      req.Slug,
		Level:     req.Level,
		ParentID:  pguuidFromPtr(uuidPtrFromOAPI(req.ParentId)),
		Column7:   path,
		Status:    req.Status,
		Metadata:  metadataJSON,
		SortOrder: sortOrder,
	}

	created, err := s.topologySvc.CreateLocation(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create location")
		return
	}
	s.recordAudit(c, "location.created", "topology", "location", created.ID, map[string]any{
		"name": created.Name, "slug": created.Slug, "level": created.Level,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectLocationCreated, tenantID.String(), map[string]any{
		"location_id": created.ID.String(), "tenant_id": tenantID.String(), "name": created.Name,
	})
	response.Created(c, toAPILocation(*created))
}

// UpdateLocation updates an existing location.
// (PUT /locations/{id})
func (s *APIServer) UpdateLocation(c *gin.Context, id IdPath) {
	var req UpdateLocationJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateLocationParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.NameEn != nil {
		params.NameEn = pgtype.Text{String: *req.NameEn, Valid: true}
	}
	if req.Slug != nil {
		params.Slug = pgtype.Text{String: *req.Slug, Valid: true}
	}
	if req.Level != nil {
		params.Level = pgtype.Text{String: *req.Level, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Metadata != nil {
		b, _ := json.Marshal(req.Metadata)
		params.Metadata = b
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: int32(*req.SortOrder), Valid: true}
	}

	updated, err := s.topologySvc.UpdateLocation(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "location not found")
		} else {
			response.InternalError(c, "failed to update location")
		}
		return
	}
	diff := map[string]any{}
	if req.Name != nil {
		diff["name"] = *req.Name
	}
	if req.Status != nil {
		diff["status"] = *req.Status
	}
	if req.Level != nil {
		diff["level"] = *req.Level
	}
	s.recordAudit(c, "location.updated", "topology", "location", updated.ID, diff)
	s.publishEvent(c.Request.Context(), eventbus.SubjectLocationUpdated, tenantIDFromContext(c).String(), map[string]any{
		"location_id": updated.ID.String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	response.OK(c, toAPILocation(*updated))
}

// DeleteLocation deletes a location.
// (DELETE /locations/{id})
func (s *APIServer) DeleteLocation(c *gin.Context, id IdPath) {
	err := s.topologySvc.DeleteLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	s.recordAudit(c, "location.deleted", "topology", "location", uuid.UUID(id), nil)
	s.publishEvent(c.Request.Context(), eventbus.SubjectLocationDeleted, tenantIDFromContext(c).String(), map[string]any{
		"location_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	c.Status(204)
}

// ListLocationDescendants returns all descendant locations for a given location.
// (GET /locations/{id}/descendants)
func (s *APIServer) ListLocationDescendants(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)

	loc, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}

	path := pgtextToStr(loc.Path)
	descendants, err := s.topologySvc.ListDescendants(c.Request.Context(), tenantID, path)
	if err != nil {
		response.InternalError(c, "failed to list descendants")
		return
	}
	response.OK(c, convertSlice(descendants, toAPILocation))
}

// ---------------------------------------------------------------------------
// Rack endpoints
// ---------------------------------------------------------------------------

// CreateRack creates a new rack.
// (POST /racks)
func (s *APIServer) CreateRack(c *gin.Context) {
	var req CreateRackJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var totalU int32
	if req.TotalU != nil {
		totalU = int32(*req.TotalU)
	}

	var powerKw pgtype.Numeric
	if req.PowerCapacityKw != nil {
		powerKw = float32ToNumeric(*req.PowerCapacityKw)
	}

	var tags []string
	if req.Tags != nil {
		tags = *req.Tags
	}

	params := dbgen.CreateRackParams{
		TenantID:        tenantID,
		LocationID:      uuid.UUID(req.LocationId),
		Name:            req.Name,
		RowLabel:        textFromPtr(req.RowLabel),
		TotalU:          totalU,
		PowerCapacityKw: powerKw,
		Status:          req.Status,
		Tags:            tags,
	}

	created, err := s.topologySvc.CreateRack(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create rack")
		return
	}
	s.recordAudit(c, "rack.created", "topology", "rack", created.ID, map[string]any{
		"name": created.Name, "location_id": created.LocationID.String(),
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackCreated, tenantID.String(), map[string]any{
		"rack_id": created.ID.String(), "tenant_id": tenantID.String(), "name": created.Name,
	})
	response.Created(c, toAPIRack(*created))
}

// GetRack returns a single rack by ID.
// (GET /racks/{id})
func (s *APIServer) GetRack(c *gin.Context, id IdPath) {
	rack, err := s.topologySvc.GetRack(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	response.OK(c, toAPIRack(rack))
}

// ListRackAssets returns all assets in a rack.
// (GET /racks/{id}/assets)
func (s *APIServer) ListRackAssets(c *gin.Context, id IdPath) {
	assets, err := s.topologySvc.ListAssetsByRack(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list rack assets")
		return
	}
	response.OK(c, convertSlice(assets, toAPIAsset))
}

// UpdateRack updates an existing rack.
// (PUT /racks/{id})
func (s *APIServer) UpdateRack(c *gin.Context, id IdPath) {
	var req UpdateRackJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateRackParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.RowLabel != nil {
		params.RowLabel = pgtype.Text{String: *req.RowLabel, Valid: true}
	}
	if req.TotalU != nil {
		params.TotalU = pgtype.Int4{Int32: int32(*req.TotalU), Valid: true}
	}
	if req.PowerCapacityKw != nil {
		params.PowerCapacityKw = float32ToNumeric(*req.PowerCapacityKw)
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Tags != nil {
		params.Tags = *req.Tags
	}

	updated, err := s.topologySvc.UpdateRack(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "rack not found")
		} else {
			response.InternalError(c, "failed to update rack")
		}
		return
	}
	diff := map[string]any{}
	if req.Name != nil {
		diff["name"] = *req.Name
	}
	if req.Status != nil {
		diff["status"] = *req.Status
	}
	s.recordAudit(c, "rack.updated", "topology", "rack", updated.ID, diff)
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackUpdated, tenantIDFromContext(c).String(), map[string]any{
		"rack_id": updated.ID.String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	response.OK(c, toAPIRack(*updated))
}

// DeleteRack deletes a rack.
// (DELETE /racks/{id})
func (s *APIServer) DeleteRack(c *gin.Context, id IdPath) {
	err := s.topologySvc.DeleteRack(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	s.recordAudit(c, "rack.deleted", "topology", "rack", uuid.UUID(id), nil)
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackDeleted, tenantIDFromContext(c).String(), map[string]any{
		"rack_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	c.Status(204)
}

// ---------------------------------------------------------------------------
// Rack Slot endpoints
// ---------------------------------------------------------------------------

// ListRackSlots returns all slot assignments for a rack.
// (GET /racks/{id}/slots)
func (s *APIServer) ListRackSlots(c *gin.Context, id IdPath) {
	slots, err := s.topologySvc.ListRackSlots(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list rack slots")
		return
	}
	response.OK(c, convertSlice(slots, toAPIRackSlot))
}

// CreateRackSlot assigns an asset to a rack slot with conflict detection.
// (POST /racks/{id}/slots)
func (s *APIServer) CreateRackSlot(c *gin.Context, id IdPath) {
	var req CreateRackSlotJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	side := "front"
	if req.Side != nil {
		side = *req.Side
	}

	// Check for U-position conflict
	conflictCount, err := s.topologySvc.CheckSlotConflict(c.Request.Context(), uuid.UUID(id), side, int32(req.StartU), int32(req.EndU))
	if err != nil {
		response.InternalError(c, "failed to check slot conflict")
		return
	}
	if conflictCount > 0 {
		response.BadRequest(c, fmt.Sprintf("U position conflict: U%d-U%d on %s is occupied", req.StartU, req.EndU, side))
		return
	}

	slot, err := s.topologySvc.CreateRackSlot(c.Request.Context(), dbgen.CreateRackSlotParams{
		RackID:  uuid.UUID(id),
		AssetID: uuid.UUID(req.AssetId),
		StartU:  int32(req.StartU),
		EndU:    int32(req.EndU),
		Side:    side,
	})
	if err != nil {
		response.InternalError(c, "failed to create rack slot")
		return
	}

	// Convert the created slot to API format
	apiSlot := RackSlot{
		Id:     &slot.ID,
		RackId: &slot.RackID,
		AssetId: &slot.AssetID,
		StartU: func() *int { v := int(slot.StartU); return &v }(),
		EndU:   func() *int { v := int(slot.EndU); return &v }(),
		Side:   &slot.Side,
	}
	response.Created(c, apiSlot)
}

// DeleteRackSlot removes an asset from a rack slot.
// (DELETE /racks/{id}/slots/{slotId})
func (s *APIServer) DeleteRackSlot(c *gin.Context, id IdPath, slotId openapi_types.UUID) {
	err := s.topologySvc.DeleteRackSlot(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(slotId))
	if err != nil {
		response.NotFound(c, "rack slot not found")
		return
	}
	c.Status(204)
}

// ---------------------------------------------------------------------------
// Maintenance endpoints
// ---------------------------------------------------------------------------

// ListWorkOrders returns a paginated list of work orders.
// (GET /maintenance/orders)
func (s *APIServer) ListWorkOrders(c *gin.Context, params ListWorkOrdersParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	var locationID *uuid.UUID
	if params.LocationId != nil {
		u := uuid.UUID(*params.LocationId)
		locationID = &u
	}
	orders, total, err := s.maintenanceSvc.List(c.Request.Context(), tenantID, params.Status, locationID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list work orders")
		return
	}
	response.OKList(c, convertSlice(orders, toAPIWorkOrder), page, pageSize, int(total))
}

// CreateWorkOrder creates a new work order.
// (POST /maintenance/orders)
func (s *APIServer) CreateWorkOrder(c *gin.Context) {
	var req CreateWorkOrderJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	requestorID := userIDFromContext(c)

	domainReq := maintenance.CreateOrderRequest{
		Title: req.Title,
		Type:  req.Type,
	}
	if req.Priority != nil {
		domainReq.Priority = *req.Priority
	}
	if req.LocationId != nil {
		u := uuid.UUID(*req.LocationId)
		domainReq.LocationID = &u
	}
	if req.AssigneeId != nil {
		u := uuid.UUID(*req.AssigneeId)
		domainReq.AssigneeID = &u
	}
	if req.Description != nil {
		domainReq.Description = *req.Description
	}
	if req.ScheduledStart != nil {
		domainReq.ScheduledStart = req.ScheduledStart
	}
	if req.ScheduledEnd != nil {
		domainReq.ScheduledEnd = req.ScheduledEnd
	}

	order, err := s.maintenanceSvc.Create(c.Request.Context(), tenantID, requestorID, domainReq)
	if err != nil {
		response.InternalError(c, "failed to create work order")
		return
	}
	s.recordAudit(c, "order.created", "maintenance", "work_order", order.ID, map[string]any{
		"code":  order.Code,
		"title": order.Title,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectOrderCreated, tenantID.String(), map[string]any{
		"order_id": order.ID.String(), "tenant_id": tenantID.String(), "code": order.Code, "priority": order.Priority,
	})
	response.Created(c, toAPIWorkOrder(*order))
}

// GetWorkOrder returns a single work order by ID.
// (GET /maintenance/orders/{id})
func (s *APIServer) GetWorkOrder(c *gin.Context, id IdPath) {
	order, err := s.maintenanceSvc.GetByID(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "work order not found")
		return
	}
	response.OK(c, toAPIWorkOrder(*order))
}

// TransitionWorkOrder transitions a work order to a new status.
// (POST /maintenance/orders/{id}/transition)
func (s *APIServer) TransitionWorkOrder(c *gin.Context, id IdPath) {
	var req TransitionWorkOrderJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	operatorID := userIDFromContext(c)
	comment := ""
	if req.Comment != nil {
		comment = *req.Comment
	}

	// Fetch operator role names for approval permission checks
	var roleNames []string
	if maintenance.RequiresApproval(req.Status) {
		roles, roleErr := dbgen.New(s.pool).ListUserRoles(c.Request.Context(), operatorID)
		if roleErr == nil {
			for _, r := range roles {
				roleNames = append(roleNames, r.Name)
			}
		}
	}

	order, err := s.maintenanceSvc.Transition(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id), operatorID, roleNames, maintenance.TransitionRequest{
		Status:  req.Status,
		Comment: comment,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	s.recordAudit(c, "order.transitioned", "maintenance", "work_order", uuid.UUID(id), map[string]any{
		"status":  req.Status,
		"comment": comment,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectOrderTransitioned, tenantIDFromContext(c).String(), map[string]any{
		"order_id": uuid.UUID(id).String(), "status": req.Status, "tenant_id": tenantIDFromContext(c).String(), "type": order.Type, "priority": order.Priority, "asset_id": "",
	})
	response.OK(c, toAPIWorkOrder(*order))
}

// UpdateWorkOrder updates a work order's details.
// (PUT /maintenance/orders/{id})
func (s *APIServer) UpdateWorkOrder(c *gin.Context, id IdPath) {
	var req UpdateWorkOrderJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	params := dbgen.UpdateWorkOrderParams{
		ID:       uuid.UUID(id),
		TenantID: tenantID,
	}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Priority != nil {
		params.Priority = pgtype.Text{String: *req.Priority, Valid: true}
	}
	if req.AssigneeId != nil {
		params.AssigneeID = pgtype.UUID{Bytes: uuid.UUID(*req.AssigneeId), Valid: true}
	}
	if req.ScheduledStart != nil {
		params.ScheduledStart = pgtype.Timestamptz{Time: *req.ScheduledStart, Valid: true}
	}
	if req.ScheduledEnd != nil {
		params.ScheduledEnd = pgtype.Timestamptz{Time: *req.ScheduledEnd, Valid: true}
	}

	order, err := s.maintenanceSvc.Update(c.Request.Context(), params)
	if err != nil {
		response.NotFound(c, "work order not found")
		return
	}
	s.recordAudit(c, "order.updated", "maintenance", "work_order", uuid.UUID(id), map[string]any{
		"title":    req.Title,
		"priority": req.Priority,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectOrderUpdated, tenantIDFromContext(c).String(), map[string]any{
		"order_id": uuid.UUID(id).String(), "action": "updated",
	})
	response.OK(c, toAPIWorkOrder(*order))
}

// DeleteWorkOrder soft-deletes a work order.
// (DELETE /maintenance/orders/{id})
func (s *APIServer) DeleteWorkOrder(c *gin.Context, id openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	err := s.maintenanceSvc.Delete(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "work order not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.Status(204)
}

// ListWorkOrderLogs returns the audit trail for a work order.
// (GET /maintenance/orders/{id}/logs)
func (s *APIServer) ListWorkOrderLogs(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	// Verify the work order belongs to the current tenant before listing logs
	_, err := s.maintenanceSvc.GetByID(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "work order not found")
		return
	}
	logs, err := s.maintenanceSvc.ListLogs(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "work order not found")
		return
	}
	response.OK(c, convertSlice(logs, toAPIWorkOrderLog))
}

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

// UpdateIncident updates an incident.
// (PUT /monitoring/incidents/{id})
func (s *APIServer) UpdateIncident(c *gin.Context, id IdPath) {
	var req UpdateIncidentJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateIncidentParams{
		ID: uuid.UUID(id),
	}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Severity != nil {
		params.Severity = pgtype.Text{String: *req.Severity, Valid: true}
	}
	if req.ResolvedAt != nil {
		params.ResolvedAt = pgtype.Timestamptz{Time: *req.ResolvedAt, Valid: true}
	}

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
	s.publishEvent(c.Request.Context(), eventbus.SubjectAlertResolved, tenantIDFromContext(c).String(), map[string]any{
		"incident_id": updated.ID.String(), "status": updated.Status,
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
		 WHERE asset_id = $1
		   AND name = $2
		   AND %s > $3
		 ORDER BY %s DESC
		 LIMIT 500`,
		timeCol, valueCol, tableName, timeCol, timeCol,
	)

	rows, err := s.pool.Query(c.Request.Context(), query, assetID, metricName, since)
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

// ---------------------------------------------------------------------------
// Inventory endpoints
// ---------------------------------------------------------------------------

// ListInventoryTasks returns a paginated list of inventory tasks.
// (GET /inventory/tasks)
func (s *APIServer) ListInventoryTasks(c *gin.Context, params ListInventoryTasksParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	var scopeLocationID *uuid.UUID
	if params.ScopeLocationId != nil {
		u := uuid.UUID(*params.ScopeLocationId)
		scopeLocationID = &u
	}
	tasks, total, err := s.inventorySvc.List(c.Request.Context(), tenantID, scopeLocationID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list inventory tasks")
		return
	}
	response.OKList(c, convertSlice(tasks, toAPIInventoryTask), page, pageSize, int(total))
}

// GetInventoryTask returns a single inventory task by ID.
// (GET /inventory/tasks/{id})
func (s *APIServer) GetInventoryTask(c *gin.Context, id IdPath) {
	task, err := s.inventorySvc.GetByID(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	response.OK(c, toAPIInventoryTask(*task))
}

// ListInventoryItems returns a paginated list of items in an inventory task.
// (GET /inventory/tasks/{id}/items)
func (s *APIServer) ListInventoryItems(c *gin.Context, id IdPath) {
	var pg, pgs *int
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			pg = &v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			pgs = &v
		}
	}
	page, pageSize, limit, offset := paginationDefaults(pg, pgs)

	items, total, err := s.inventorySvc.ListItems(c.Request.Context(), uuid.UUID(id), limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list inventory items")
		return
	}
	response.OKList(c, convertSlice(items, toAPIInventoryItem), page, pageSize, int(total))
}

// CreateInventoryTask creates a new inventory task.
// (POST /inventory/tasks)
func (s *APIServer) CreateInventoryTask(c *gin.Context) {
	var req CreateInventoryTaskJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	code := fmt.Sprintf("INV-%d-%04d", time.Now().Year(), rand.Intn(10000))

	params := dbgen.CreateInventoryTaskParams{
		TenantID: tenantID,
		Code:     code,
		Name:     req.Name,
		Method:   pgtype.Text{String: req.Method, Valid: true},
	}
	if req.PlannedDate != "" {
		t, err := time.Parse("2006-01-02", req.PlannedDate)
		if err == nil {
			params.PlannedDate = pgtype.Date{Time: t, Valid: true}
		}
	}
	if req.ScopeLocationId != nil {
		params.ScopeLocationID = pgtype.UUID{Bytes: uuid.UUID(*req.ScopeLocationId), Valid: true}
	}
	if req.AssignedTo != nil {
		params.AssignedTo = pgtype.UUID{Bytes: uuid.UUID(*req.AssignedTo), Valid: true}
	}

	task, err := s.inventorySvc.Create(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create inventory task")
		return
	}
	s.recordAudit(c, "task.created", "inventory", "inventory_task", task.ID, map[string]any{
		"code": task.Code,
		"name": task.Name,
	})
	response.Created(c, toAPIInventoryTask(*task))
}

// CompleteInventoryTask marks an inventory task as completed.
// (POST /inventory/tasks/{id}/complete)
func (s *APIServer) CompleteInventoryTask(c *gin.Context, id IdPath) {
	task, err := s.inventorySvc.Complete(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	s.recordAudit(c, "task.completed", "inventory", "inventory_task", uuid.UUID(id), map[string]any{
		"code": task.Code,
	})
	response.OK(c, toAPIInventoryTask(*task))
}

// UpdateInventoryTask updates an inventory task.
// (PUT /inventory/tasks/{id})
func (s *APIServer) UpdateInventoryTask(c *gin.Context, id openapi_types.UUID) {
	var req UpdateInventoryTaskJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	var assignedTo *uuid.UUID
	if req.AssignedTo != nil {
		u := uuid.UUID(*req.AssignedTo)
		assignedTo = &u
	}

	task, err := s.inventorySvc.Update(c.Request.Context(), tenantID, uuid.UUID(id), req.Name, req.PlannedDate, assignedTo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "inventory task not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, toAPIInventoryTask(*task))
}

// DeleteInventoryTask soft-deletes an inventory task.
// (DELETE /inventory/tasks/{id})
func (s *APIServer) DeleteInventoryTask(c *gin.Context, id openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	err := s.inventorySvc.Delete(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "inventory task not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.Status(204)
}

// ScanInventoryItem records a scan result for an inventory item.
// (POST /inventory/tasks/{id}/items/{itemId}/scan)
func (s *APIServer) ScanInventoryItem(c *gin.Context, id IdPath, itemId openapi_types.UUID) {
	var req ScanInventoryItemJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	actualJSON, _ := json.Marshal(req.Actual)
	userID := userIDFromContext(c)

	params := dbgen.ScanInventoryItemParams{
		ID:        uuid.UUID(itemId),
		Actual:    actualJSON,
		Status:    req.Status,
		ScannedBy: pgtype.UUID{Bytes: userID, Valid: true},
	}

	ctx := c.Request.Context()
	item, err := s.inventorySvc.ScanItem(ctx, params)
	if err != nil {
		response.NotFound(c, "inventory item not found")
		return
	}

	// Auto-activate task if still planned
	taskID := uuid.UUID(id)
	s.pool.Exec(ctx,
		"UPDATE inventory_tasks SET status = 'in_progress' WHERE id = $1 AND status = 'planned'",
		taskID)

	s.recordAudit(c, "item.scanned", "inventory", "inventory_item", uuid.UUID(itemId), map[string]any{
		"status": req.Status,
	})
	response.OK(c, toAPIInventoryItem(*item))
}

// ImportInventoryItems accepts a JSON batch of items, matches them against
// existing assets by serial_number/asset_tag, and returns match statistics.
// (POST /inventory/tasks/{id}/import)
func (s *APIServer) ImportInventoryItems(c *gin.Context, id IdPath) {
	var req ImportInventoryItemsJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	taskID := uuid.UUID(id)
	ctx := c.Request.Context()

	stats := map[string]int{"matched": 0, "discrepancy": 0, "not_found": 0, "total": 0}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		response.InternalError(c, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	for _, item := range req.Items {
		stats["total"]++
		tag := ""
		serial := ""
		if item.AssetTag != nil {
			tag = *item.AssetTag
		}
		if item.SerialNumber != nil {
			serial = *item.SerialNumber
		}

		asset, err := s.assetSvc.FindBySerialOrTag(ctx, tenantID, serial, tag)

		// Fallback: try property_number
		if (err != nil || asset == nil) && item.PropertyNumber != nil && *item.PropertyNumber != "" {
			row := s.pool.QueryRow(ctx,
				"SELECT id FROM assets WHERE tenant_id = $1 AND property_number = $2 LIMIT 1",
				tenantID, *item.PropertyNumber)
			var assetID uuid.UUID
			if row.Scan(&assetID) == nil {
				a, e := s.assetSvc.GetByID(ctx, tenantID, assetID)
				if e == nil {
					asset = a
					err = nil
				}
			}
		}

		// Fallback: try control_number
		if (err != nil || asset == nil) && item.ControlNumber != nil && *item.ControlNumber != "" {
			row := s.pool.QueryRow(ctx,
				"SELECT id FROM assets WHERE tenant_id = $1 AND control_number = $2 LIMIT 1",
				tenantID, *item.ControlNumber)
			var assetID uuid.UUID
			if row.Scan(&assetID) == nil {
				a, e := s.assetSvc.GetByID(ctx, tenantID, assetID)
				if e == nil {
					asset = a
					err = nil
				}
			}
		}

		// Build expected JSON
		expectedData := map[string]string{}
		if item.AssetTag != nil {
			expectedData["asset_tag"] = *item.AssetTag
		}
		if item.SerialNumber != nil {
			expectedData["serial_number"] = *item.SerialNumber
		}
		if item.ExpectedLocation != nil {
			expectedData["expected_location"] = *item.ExpectedLocation
		}
		if item.PropertyNumber != nil {
			expectedData["property_number"] = *item.PropertyNumber
		}
		if item.ControlNumber != nil {
			expectedData["control_number"] = *item.ControlNumber
		}
		expectedJSON, _ := json.Marshal(expectedData)

		if err != nil || asset == nil {
			stats["not_found"]++
			// Insert as missing item (no asset_id)
			tx.Exec(ctx,
				"INSERT INTO inventory_items (task_id, expected, status) VALUES ($1, $2, 'missing')",
				taskID, expectedJSON)
			continue
		}

		stats["matched"]++
		// Insert matched item
		tx.Exec(ctx,
			"INSERT INTO inventory_items (task_id, asset_id, rack_id, expected, status) VALUES ($1, $2, $3, $4, 'pending')",
			taskID, asset.ID, asset.RackID, expectedJSON)
	}

	// Auto-transition task: planned → in_progress (inside transaction)
	tx.Exec(ctx,
		"UPDATE inventory_tasks SET status = 'in_progress' WHERE id = $1 AND status = 'planned'",
		taskID)

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, "failed to commit import")
		return
	}

	s.recordAudit(c, "inventory.imported", "inventory", "inventory_task", taskID, map[string]any{
		"matched":   stats["matched"],
		"not_found": stats["not_found"],
		"total":     stats["total"],
	})
	response.OK(c, stats)
}

// GetInventorySummary returns scan progress counts for an inventory task.
// (GET /inventory/tasks/{id}/summary)
func (s *APIServer) GetInventorySummary(c *gin.Context, id IdPath) {
	summary, err := s.inventorySvc.GetSummary(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	response.OK(c, map[string]any{
		"total":       summary.Total,
		"scanned":     summary.Scanned,
		"pending":     summary.Pending,
		"discrepancy": summary.Discrepancy,
	})
}

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

// ---------------------------------------------------------------------------
// Identity endpoints
// ---------------------------------------------------------------------------

// ListUsers returns a paginated list of users.
// (GET /users)
func (s *APIServer) ListUsers(c *gin.Context, params ListUsersParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	users, total, err := s.identitySvc.ListUsers(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}
	response.OKList(c, convertSlice(users, toAPIUser), page, pageSize, int(total))
}

// GetUser returns a single user by ID.
// (GET /users/{id})
func (s *APIServer) GetUser(c *gin.Context, id IdPath) {
	user, err := s.identitySvc.GetUser(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, toAPIUser(*user))
}

// CreateUser creates a new user.
// (POST /users)
func (s *APIServer) CreateUser(c *gin.Context) {
	var req CreateUserJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	status := "active"
	if req.Status != nil {
		status = *req.Status
	}
	source := "local"
	if req.Source != nil {
		source = *req.Source
	}

	params := dbgen.CreateUserParams{
		TenantID:    tenantID,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       pgtype.Text{String: "", Valid: false},
		Status:      status,
		Source:      source,
	}
	if req.Phone != nil {
		params.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}

	user, err := s.identitySvc.CreateUser(c.Request.Context(), params, req.Password)
	if err != nil {
		response.InternalError(c, "failed to create user")
		return
	}
	s.recordAudit(c, "user.created", "identity", "user", user.ID, map[string]any{
		"username": user.Username,
	})
	response.Created(c, toAPIUser(*user))
}

// UpdateUser updates an existing user.
// (PUT /users/{id})
func (s *APIServer) UpdateUser(c *gin.Context, id IdPath) {
	var req UpdateUserJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateUserParams{
		ID: uuid.UUID(id),
	}
	if req.DisplayName != nil {
		params.DisplayName = pgtype.Text{String: *req.DisplayName, Valid: true}
	}
	if req.Email != nil {
		params.Email = pgtype.Text{String: *req.Email, Valid: true}
	}
	if req.Phone != nil {
		params.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}

	user, err := s.identitySvc.UpdateUser(c.Request.Context(), params)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	s.recordAudit(c, "user.updated", "identity", "user", user.ID, map[string]any{
		"username": user.Username,
	})
	response.OK(c, toAPIUser(*user))
}

// ListRoles returns all roles for the tenant.
// (GET /roles)
func (s *APIServer) ListRoles(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	roles, err := s.identitySvc.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list roles")
		return
	}
	response.OK(c, convertSlice(roles, toAPIRole))
}

// CreateRole creates a new custom role.
// (POST /roles)
func (s *APIServer) CreateRole(c *gin.Context) {
	var req CreateRoleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var permJSON json.RawMessage
	if req.Permissions != nil {
		b, _ := json.Marshal(*req.Permissions)
		permJSON = b
	} else {
		permJSON = json.RawMessage(`{}`)
	}

	params := dbgen.CreateRoleParams{
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		Name:        req.Name,
		Permissions: permJSON,
		IsSystem:    false,
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	role, err := s.identitySvc.CreateRole(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create role")
		return
	}
	s.recordAudit(c, "role.created", "identity", "role", role.ID, map[string]any{
		"name": role.Name,
	})
	response.Created(c, toAPIRole(*role))
}

// UpdateRole updates a non-system role.
// (PUT /roles/{id})
func (s *APIServer) UpdateRole(c *gin.Context, id IdPath) {
	var req UpdateRoleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateRoleParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Permissions != nil {
		b, _ := json.Marshal(*req.Permissions)
		params.Permissions = b
	}

	role, err := s.identitySvc.UpdateRole(c.Request.Context(), params)
	if err != nil {
		response.NotFound(c, "role not found or is a system role")
		return
	}
	s.recordAudit(c, "role.updated", "identity", "role", role.ID, map[string]any{
		"name": role.Name,
	})
	response.OK(c, toAPIRole(*role))
}

// DeleteRole deletes a non-system role.
// (DELETE /roles/{id})
func (s *APIServer) DeleteRole(c *gin.Context, id IdPath) {
	err := s.identitySvc.DeleteRole(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "role not found or is a system role")
		return
	}
	s.recordAudit(c, "role.deleted", "identity", "role", uuid.UUID(id), nil)
	c.Status(204)
}

// ---------------------------------------------------------------------------
// Role Assignment + User Deletion endpoints
// ---------------------------------------------------------------------------

// AssignRoleToUser assigns a role to a user.
// (POST /users/{id}/roles)
func (s *APIServer) AssignRoleToUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}
	var req struct {
		RoleID string `json:"role_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "role_id is required")
		return
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "invalid role_id")
		return
	}
	if err := s.identitySvc.AssignRole(c.Request.Context(), userID, roleID); err != nil {
		response.InternalError(c, "failed to assign role")
		return
	}
	s.recordAudit(c, "role.assigned", "identity", "user", userID, map[string]any{
		"role_id": roleID.String(),
	})
	response.OK(c, gin.H{"assigned": true})
}

// RemoveRoleFromUser removes a role from a user.
// (DELETE /users/{id}/roles/{roleId})
func (s *APIServer) RemoveRoleFromUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
		return
	}
	if err := s.identitySvc.RemoveRole(c.Request.Context(), userID, roleID); err != nil {
		response.InternalError(c, "failed to remove role")
		return
	}
	s.recordAudit(c, "role.removed", "identity", "user", userID, map[string]any{
		"role_id": roleID.String(),
	})
	c.Status(204)
}

// ListUserRoles returns roles assigned to a user.
// (GET /users/{id}/roles)
func (s *APIServer) ListUserRoles(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}
	roleIDs, err := s.identitySvc.ListUserRoleIDs(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to list user roles")
		return
	}
	response.OK(c, roleIDs)
}

// DeleteUser soft-deletes (deactivates) a user.
// (DELETE /users/{id})
func (s *APIServer) DeleteUser(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}
	if err := s.identitySvc.Deactivate(c.Request.Context(), tenantID, userID); err != nil {
		response.InternalError(c, "failed to delete user")
		return
	}
	s.recordAudit(c, "user.deleted", "identity", "user", userID, nil)
	c.Status(204)
}

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

// ---------------------------------------------------------------------------
// System endpoints
// ---------------------------------------------------------------------------

// GetSystemHealth returns health status of backend dependencies.
// (GET /system/health)
func (s *APIServer) GetSystemHealth(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database
	dbStatus := "ok"
	dbStart := time.Now()
	var one int
	err := s.pool.QueryRow(ctx, "SELECT 1").Scan(&one)
	dbLatency := float32(time.Since(dbStart).Milliseconds())
	if err != nil {
		dbStatus = "error"
	}

	health := SystemHealth{
		Database: &struct {
			LatencyMs *float32 `json:"latency_ms,omitempty"`
			Status    *string  `json:"status,omitempty"`
		}{
			Status:    &dbStatus,
			LatencyMs: &dbLatency,
		},
	}

	response.OK(c, health)
}

// ---------------------------------------------------------------------------
// Integration endpoints
// ---------------------------------------------------------------------------

// ListAdapters returns all integration adapters for the tenant.
// (GET /integration/adapters)
func (s *APIServer) ListAdapters(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	adapters, err := s.integrationSvc.ListAdapters(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list adapters")
		return
	}
	response.OK(c, convertSlice(adapters, toAPIAdapter))
}

// CreateAdapter creates a new integration adapter.
// (POST /integration/adapters)
func (s *APIServer) CreateAdapter(c *gin.Context) {
	var req CreateAdapterJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var configBytes []byte
	if req.Config != nil {
		configBytes, _ = json.Marshal(*req.Config)
	} else {
		configBytes = []byte(`{}`)
	}

	params := dbgen.CreateAdapterParams{
		TenantID:  tenantID,
		Name:      req.Name,
		Type:      req.Type,
		Direction: req.Direction,
		Config:    configBytes,
	}
	if req.Endpoint != nil {
		params.Endpoint = pgtype.Text{String: *req.Endpoint, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	} else {
		params.Enabled = pgtype.Bool{Bool: true, Valid: true}
	}

	adapter, err := s.integrationSvc.CreateAdapter(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create adapter")
		return
	}
	response.Created(c, toAPIAdapter(adapter))
}

// ListWebhooks returns all webhook subscriptions for the tenant.
// (GET /integration/webhooks)
func (s *APIServer) ListWebhooks(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	webhooks, err := s.integrationSvc.ListWebhooks(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list webhooks")
		return
	}
	response.OK(c, convertSlice(webhooks, toAPIWebhook))
}

// CreateWebhook creates a new webhook subscription.
// (POST /integration/webhooks)
func (s *APIServer) CreateWebhook(c *gin.Context) {
	var req CreateWebhookJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	params := dbgen.CreateWebhookParams{
		TenantID: tenantID,
		Name:     req.Name,
		Url:      req.Url,
		Events:   req.Events,
	}
	if req.Secret != nil {
		params.Secret = pgtype.Text{String: *req.Secret, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	} else {
		params.Enabled = pgtype.Bool{Bool: true, Valid: true}
	}

	webhook, err := s.integrationSvc.CreateWebhook(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create webhook")
		return
	}
	response.Created(c, toAPIWebhook(webhook))
}

// ListWebhookDeliveries returns delivery history for a webhook.
// (GET /integration/webhooks/{id}/deliveries)
func (s *APIServer) ListWebhookDeliveries(c *gin.Context, id IdPath) {
	deliveries, err := s.integrationSvc.ListDeliveries(c.Request.Context(), uuid.UUID(id), 100)
	if err != nil {
		response.InternalError(c, "failed to list deliveries")
		return
	}
	response.OK(c, convertSlice(deliveries, toAPIWebhookDelivery))
}

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
		ID: uuid.UUID(id),
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
		if err := s.biaSvc.PropagateBIALevel(c.Request.Context(), updated.ID); err != nil {
			fmt.Printf("BIA propagation error: %v\n", err)
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
		ID: uuid.UUID(id),
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
	deps, err := s.biaSvc.ListDependencies(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list BIA dependencies")
		return
	}
	response.OK(c, convertSlice(deps, toAPIBIADependency))
}

// CreateBIADependency adds a dependency to a BIA assessment.
// (POST /bia/assessments/{id}/dependencies)
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
	assessments, err := s.biaSvc.GetImpactedAssessments(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		// Table may not exist yet (migration not run) — return empty array instead of 500
		response.OK(c, []any{})
		return
	}
	response.OK(c, convertSlice(assessments, toAPIBIAAssessment))
}

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

// ---------------------------------------------------------------------------
// Discovery (Auto-Discovery Staging Area)
// ---------------------------------------------------------------------------

// ListDiscoveredAssets lists discovered assets with optional status filter.
// (GET /discovery/pending)
func (s *APIServer) ListDiscoveredAssets(c *gin.Context, params ListDiscoveredAssetsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	items, total, err := s.discoverySvc.List(c.Request.Context(), tenantID, params.Status, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list discovered assets")
		return
	}
	response.OKList(c, convertSlice(items, toAPIDiscoveredAsset), page, pageSize, int(total))
}

// IngestDiscoveredAsset ingests a newly discovered asset into the staging area.
// (POST /discovery/ingest)
func (s *APIServer) IngestDiscoveredAsset(c *gin.Context) {
	var req IngestDiscoveredAssetJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var rawDataJSON json.RawMessage
	if req.RawData != nil {
		rawDataJSON, _ = json.Marshal(req.RawData)
	} else {
		rawDataJSON = json.RawMessage("{}")
	}

	params := dbgen.CreateDiscoveredAssetParams{
		TenantID: tenantID,
		Source:   req.Source,
		Hostname: textFromPtr(&req.Hostname),
		RawData:  rawDataJSON,
		Status:   "pending",
	}
	if req.ExternalId != nil {
		params.ExternalID = textFromPtr(req.ExternalId)
	}
	if req.IpAddress != nil {
		params.IpAddress = textFromPtr(req.IpAddress)
	}

	// Auto-match by IP if possible
	if req.IpAddress != nil && *req.IpAddress != "" {
		matched, matchErr := s.discoverySvc.Queries().FindAssetByIP(c.Request.Context(), dbgen.FindAssetByIPParams{
			TenantID:     tenantID,
			IpAddress: pgtype.Text{String: *req.IpAddress, Valid: true},
		})
		if matchErr == nil {
			params.MatchedAssetID = pgtype.UUID{Bytes: matched.ID, Valid: true}
			params.Status = "conflict"
		}
	}

	item, err := s.discoverySvc.Ingest(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to ingest discovered asset")
		return
	}
	response.Created(c, toAPIDiscoveredAsset(*item))
}

// ApproveDiscoveredAsset approves a discovered asset.
// (POST /discovery/{id}/approve)
func (s *APIServer) ApproveDiscoveredAsset(c *gin.Context, id IdPath) {
	reviewerID := userIDFromContext(c)
	item, err := s.discoverySvc.Approve(c.Request.Context(), uuid.UUID(id), reviewerID)
	if err != nil {
		response.InternalError(c, "failed to approve discovered asset")
		return
	}
	response.OK(c, toAPIDiscoveredAsset(*item))
}

// IgnoreDiscoveredAsset ignores a discovered asset.
// (POST /discovery/{id}/ignore)
func (s *APIServer) IgnoreDiscoveredAsset(c *gin.Context, id IdPath) {
	reviewerID := userIDFromContext(c)
	item, err := s.discoverySvc.Ignore(c.Request.Context(), uuid.UUID(id), reviewerID)
	if err != nil {
		response.InternalError(c, "failed to ignore discovered asset")
		return
	}
	response.OK(c, toAPIDiscoveredAsset(*item))
}

// GetDiscoveryStats returns discovery statistics for the last 24 hours.
// (GET /discovery/stats)
func (s *APIServer) GetDiscoveryStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	row, err := s.discoverySvc.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get discovery stats")
		return
	}
	total := int(row.Total)
	pending := int(row.Pending)
	conflict := int(row.Conflict)
	approved := int(row.Approved)
	ignored := int(row.Ignored)
	matched := int(row.Matched)
	response.OK(c, DiscoveryStats{
		Total:    &total,
		Pending:  &pending,
		Conflict: &conflict,
		Approved: &approved,
		Ignored:  &ignored,
		Matched:  &matched,
	})
}

// ---------------------------------------------------------------------------
// CIType soft validation helper
// ---------------------------------------------------------------------------

var assetTypeSchemas = map[string][]string{
	"server":  {"cpu", "memory", "storage", "os"},
	"network": {"ports", "firmware"},
	"storage": {"raw_capacity", "protocol"},
	"power":   {"capacity"},
}

func ciTypeSoftValidation(assetType string, attrs map[string]interface{}) []string {
	schema, ok := assetTypeSchemas[assetType]
	if !ok {
		return nil
	}
	var warnings []string
	if attrs == nil {
		warnings = append(warnings, fmt.Sprintf("type %s recommends attributes: %v", assetType, schema))
	} else {
		for _, field := range schema {
			if _, exists := attrs[field]; !exists {
				warnings = append(warnings, fmt.Sprintf("missing recommended attribute: %s", field))
			}
		}
	}
	return warnings
}
