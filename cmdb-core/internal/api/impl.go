package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/audit"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
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
}

// NewAPIServer constructs an APIServer with all required domain services.
func NewAPIServer(
	pool *pgxpool.Pool,
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
) *APIServer {
	return &APIServer{
		pool:           pool,
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
		Username: req.Username,
		Password: req.Password,
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
	response.Created(c, toAPIAsset(*created))
}

// GetAsset returns a single asset by ID.
// (GET /assets/{id})
func (s *APIServer) GetAsset(c *gin.Context, id IdPath) {
	a, err := s.assetSvc.GetByID(c.Request.Context(), uuid.UUID(id))
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
	s.recordAudit(c, "asset.updated", "asset", "asset", updated.ID, diff)
	response.OK(c, toAPIAsset(*updated))
}

// DeleteAsset deletes an asset.
// (DELETE /assets/{id})
func (s *APIServer) DeleteAsset(c *gin.Context, id IdPath) {
	err := s.assetSvc.Delete(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	s.recordAudit(c, "asset.deleted", "asset", "asset", uuid.UUID(id), nil)
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
	loc, err := s.topologySvc.GetLocation(c.Request.Context(), uuid.UUID(id))
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

	loc, err := s.topologySvc.GetLocation(c.Request.Context(), uuid.UUID(id))
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
	stats, err := s.topologySvc.GetLocationStats(c.Request.Context(), uuid.UUID(id))
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

// ---------------------------------------------------------------------------
// Rack endpoints
// ---------------------------------------------------------------------------

// GetRack returns a single rack by ID.
// (GET /racks/{id})
func (s *APIServer) GetRack(c *gin.Context, id IdPath) {
	rack, err := s.topologySvc.GetRack(c.Request.Context(), uuid.UUID(id))
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

// ---------------------------------------------------------------------------
// Maintenance endpoints
// ---------------------------------------------------------------------------

// ListWorkOrders returns a paginated list of work orders.
// (GET /maintenance/orders)
func (s *APIServer) ListWorkOrders(c *gin.Context, params ListWorkOrdersParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	orders, total, err := s.maintenanceSvc.List(c.Request.Context(), tenantID, params.Status, limit, offset)
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
	response.Created(c, toAPIWorkOrder(*order))
}

// GetWorkOrder returns a single work order by ID.
// (GET /maintenance/orders/{id})
func (s *APIServer) GetWorkOrder(c *gin.Context, id IdPath) {
	order, err := s.maintenanceSvc.GetByID(c.Request.Context(), uuid.UUID(id))
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

	order, err := s.maintenanceSvc.Transition(c.Request.Context(), uuid.UUID(id), operatorID, maintenance.TransitionRequest{
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
	response.OK(c, toAPIWorkOrder(*order))
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
	alert, err := s.monitoringSvc.Acknowledge(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "alert not found")
		return
	}
	s.recordAudit(c, "alert.acknowledged", "monitoring", "alert", alert.ID, map[string]any{
		"status": alert.Status,
	})
	response.OK(c, toAPIAlertEvent(*alert))
}

// ResolveAlert resolves an alert event.
// (POST /monitoring/alerts/{id}/resolve)
func (s *APIServer) ResolveAlert(c *gin.Context, id IdPath) {
	alert, err := s.monitoringSvc.Resolve(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "alert not found")
		return
	}
	s.recordAudit(c, "alert.resolved", "monitoring", "alert", alert.ID, map[string]any{
		"status": alert.Status,
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
	response.Created(c, toAPIAlertRule(*rule))
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
	response.Created(c, toAPIIncident(*incident))
}

// GetIncident returns a single incident.
// (GET /monitoring/incidents/{id})
func (s *APIServer) GetIncident(c *gin.Context, id IdPath) {
	incident, err := s.monitoringSvc.GetIncident(c.Request.Context(), uuid.UUID(id))
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
	response.OK(c, toAPIIncident(*updated))
}

// QueryMetrics queries time-series metric data for an asset.
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

	since := time.Now().Add(-cutoff)

	rows, err := s.pool.Query(c.Request.Context(),
		`SELECT time, name, value
		 FROM metrics
		 WHERE asset_id = $1
		   AND name = $2
		   AND time > $3
		 ORDER BY time DESC
		 LIMIT 500`,
		assetID, metricName, since,
	)
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

	tasks, total, err := s.inventorySvc.List(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list inventory tasks")
		return
	}
	response.OKList(c, convertSlice(tasks, toAPIInventoryTask), page, pageSize, int(total))
}

// GetInventoryTask returns a single inventory task by ID.
// (GET /inventory/tasks/{id})
func (s *APIServer) GetInventoryTask(c *gin.Context, id IdPath) {
	task, err := s.inventorySvc.GetByID(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	response.OK(c, toAPIInventoryTask(*task))
}

// ListInventoryItems returns all items in an inventory task.
// (GET /inventory/tasks/{id}/items)
func (s *APIServer) ListInventoryItems(c *gin.Context, id IdPath) {
	items, err := s.inventorySvc.ListItems(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list inventory items")
		return
	}
	response.OK(c, convertSlice(items, toAPIInventoryItem))
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
