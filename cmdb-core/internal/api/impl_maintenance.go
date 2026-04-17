package api

import (
	"fmt"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

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
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			response.Err(c, 409, "DUPLICATE", "A work order with this code already exists")
			return
		}
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
	if operatorID == uuid.Nil {
		response.Err(c, 401, "INVALID_TOKEN", "invalid user identity")
		return
	}
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
		zap.L().Warn("transition rejected", zap.String("order_id", uuid.UUID(id).String()), zap.Error(err))
		response.BadRequest(c, "transition not allowed")
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
		p := strings.ToLower(*req.Priority)
		validPriorities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
		if !validPriorities[p] {
			response.BadRequest(c, fmt.Sprintf("invalid priority %q; must be critical, high, medium, or low", *req.Priority))
			return
		}
		params.Priority = pgtype.Text{String: p, Valid: true}
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

	order, err := s.maintenanceSvc.Update(c.Request.Context(), tenantID, params)
	if err != nil {
		if strings.Contains(err.Error(), "cannot modify") {
			response.Forbidden(c, err.Error())
			return
		}
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
	s.recordAudit(c, "order.deleted", "maintenance", "work_order", uuid.UUID(id), nil)
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
