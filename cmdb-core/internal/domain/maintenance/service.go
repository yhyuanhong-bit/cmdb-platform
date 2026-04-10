package maintenance

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides work order domain operations.
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
}

// NewService creates a new maintenance Service.
func NewService(queries *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: queries, bus: bus}
}

// List returns a paginated list of work orders and the total count.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status *string, locationID *uuid.UUID, limit, offset int32) ([]dbgen.WorkOrder, int64, error) {
	listParams := dbgen.ListWorkOrdersParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}
	countParams := dbgen.CountWorkOrdersParams{
		TenantID: tenantID,
	}

	if status != nil {
		listParams.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}

	if locationID != nil {
		listParams.LocationID = pgtype.UUID{Bytes: *locationID, Valid: true}
		countParams.LocationID = pgtype.UUID{Bytes: *locationID, Valid: true}
	}

	orders, err := s.queries.ListWorkOrders(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list work orders: %w", err)
	}

	total, err := s.queries.CountWorkOrders(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count work orders: %w", err)
	}

	return orders, total, nil
}

// GetByID returns a single work order by its ID, scoped to the given tenant.
func (s *Service) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}
	return &order, nil
}

// generateCode produces a work order code like "WO-2026-A1B2C3D4".
func generateCode() string {
	year := time.Now().Year()
	b := make([]byte, 4)
	if _, err := crand.Read(b); err != nil {
		// fallback
		return fmt.Sprintf("WO-%d-%08X", year, time.Now().UnixNano()&0xFFFFFFFF)
	}
	return fmt.Sprintf("WO-%d-%08X", year, binary.BigEndian.Uint32(b))
}

// Create creates a new work order in submitted status.
func (s *Service) Create(ctx context.Context, tenantID, requestorID uuid.UUID, req CreateOrderRequest) (*dbgen.WorkOrder, error) {
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}
	priority = strings.ToLower(priority)

	params := dbgen.CreateWorkOrderParams{
		TenantID:    tenantID,
		Code:        generateCode(),
		Title:       req.Title,
		Type:        req.Type,
		Status:      StatusSubmitted,
		Priority:    priority,
		RequestorID: pgtype.UUID{Bytes: requestorID, Valid: true},
	}

	if req.LocationID != nil {
		params.LocationID = pgtype.UUID{Bytes: *req.LocationID, Valid: true}
	}
	if req.AssetID != nil {
		params.AssetID = pgtype.UUID{Bytes: *req.AssetID, Valid: true}
	}
	if req.AssigneeID != nil {
		params.AssigneeID = pgtype.UUID{Bytes: *req.AssigneeID, Valid: true}
	}
	if req.Description != "" {
		params.Description = pgtype.Text{String: req.Description, Valid: true}
	}
	if req.Reason != "" {
		params.Reason = pgtype.Text{String: req.Reason, Valid: true}
	}
	if req.ScheduledStart != nil {
		params.ScheduledStart = pgtype.Timestamptz{Time: *req.ScheduledStart, Valid: true}
	}
	if req.ScheduledEnd != nil {
		params.ScheduledEnd = pgtype.Timestamptz{Time: *req.ScheduledEnd, Valid: true}
	}

	order, err := s.queries.CreateWorkOrder(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create work order: %w", err)
	}

	// Create initial log entry
	_, _ = s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
		OrderID:    order.ID,
		Action:     "created",
		ToStatus:   pgtype.Text{String: StatusSubmitted, Valid: true},
		OperatorID: pgtype.UUID{Bytes: requestorID, Valid: true},
	})

	return &order, nil
}

// Transition moves a work order from one status to another after validation.
func (s *Service) Transition(ctx context.Context, tenantID, id, operatorID uuid.UUID, operatorRoles []string, req TransitionRequest) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}

	if err := ValidateTransition(order.Status, req.Status); err != nil {
		return nil, err
	}

	// Check approval permissions
	if RequiresApproval(req.Status) {
		if err := validateApproval(operatorID, order.RequestorID, operatorRoles); err != nil {
			return nil, err
		}
		if req.Comment == "" {
			return nil, fmt.Errorf("approval/rejection requires a comment")
		}
	}

	var updated dbgen.WorkOrder

	if req.Status == StatusApproved {
		// Use StampWorkOrderApproval to set approved_at, approved_by, sla_deadline in one shot
		now := time.Now()
		deadline := SLADeadline(order.Priority, now)
		updated, err = s.queries.StampWorkOrderApproval(ctx, dbgen.StampWorkOrderApprovalParams{
			ID:          id,
			ApprovedBy:  pgtype.UUID{Bytes: operatorID, Valid: true},
			SlaDeadline: pgtype.Timestamptz{Time: deadline, Valid: true},
			TenantID:    tenantID,
		})
	} else {
		updated, err = s.queries.UpdateWorkOrderStatus(ctx, dbgen.UpdateWorkOrderStatusParams{
			ID:       id,
			Status:   req.Status,
			TenantID: tenantID,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("update work order status: %w", err)
	}

	logParams := dbgen.CreateWorkOrderLogParams{
		OrderID:    id,
		Action:     "transition",
		FromStatus: pgtype.Text{String: order.Status, Valid: true},
		ToStatus:   pgtype.Text{String: req.Status, Valid: true},
		OperatorID: pgtype.UUID{Bytes: operatorID, Valid: true},
	}
	if req.Comment != "" {
		logParams.Comment = pgtype.Text{String: req.Comment, Valid: true}
	}
	_, _ = s.queries.CreateWorkOrderLog(ctx, logParams)

	return &updated, nil
}

// validateApproval checks that the operator has approval permissions and is not self-approving.
func validateApproval(operatorID uuid.UUID, requestorID pgtype.UUID, operatorRoles []string) error {
	// System operations (uuid.Nil) bypass approval checks
	if operatorID == uuid.Nil {
		return nil
	}

	// Check role
	hasApprovalRole := false
	for _, role := range operatorRoles {
		if role == "super-admin" || role == "ops-admin" {
			hasApprovalRole = true
			break
		}
	}
	if !hasApprovalRole {
		return fmt.Errorf("insufficient permissions: only ops-admin or super-admin can approve/reject work orders")
	}

	// Block self-approval
	if requestorID.Valid && operatorID == requestorID.Bytes {
		return fmt.Errorf("self-approval is not allowed: the creator cannot approve their own work order")
	}

	return nil
}

// Update applies partial updates to a work order.
func (s *Service) Update(ctx context.Context, params dbgen.UpdateWorkOrderParams) (*dbgen.WorkOrder, error) {
	order, err := s.queries.UpdateWorkOrder(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update work order: %w", err)
	}
	return &order, nil
}

// Delete soft-deletes a work order. Only draft/rejected orders can be deleted.
func (s *Service) Delete(ctx context.Context, tenantID, orderID uuid.UUID) error {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: orderID, TenantID: tenantID})
	if err != nil {
		return fmt.Errorf("work order not found: %w", err)
	}

	if order.Status != StatusSubmitted && order.Status != StatusRejected {
		return fmt.Errorf("cannot delete work order in '%s' status; only submitted or rejected orders can be deleted", order.Status)
	}

	return s.queries.SoftDeleteWorkOrder(ctx, dbgen.SoftDeleteWorkOrderParams{
		ID:       orderID,
		TenantID: tenantID,
	})
}

// ListLogs returns all log entries for a work order.
func (s *Service) ListLogs(ctx context.Context, orderID uuid.UUID) ([]dbgen.WorkOrderLog, error) {
	logs, err := s.queries.ListWorkOrderLogs(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("list work order logs: %w", err)
	}
	return logs, nil
}
