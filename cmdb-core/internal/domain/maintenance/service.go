package maintenance

import (
	"context"
	"fmt"
	"math/rand"
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

// GetByID returns a single work order by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}
	return &order, nil
}

// generateCode produces a work order code like "WO-2026-0042".
func generateCode() string {
	year := time.Now().Year()
	seq := rand.Intn(9000) + 1000
	return fmt.Sprintf("WO-%d-%04d", year, seq)
}

// Create creates a new work order in draft status.
func (s *Service) Create(ctx context.Context, tenantID, requestorID uuid.UUID, req CreateOrderRequest) (*dbgen.WorkOrder, error) {
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	params := dbgen.CreateWorkOrderParams{
		TenantID:    tenantID,
		Code:        generateCode(),
		Title:       req.Title,
		Type:        req.Type,
		Status:      "draft",
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
		ToStatus:   pgtype.Text{String: "draft", Valid: true},
		OperatorID: pgtype.UUID{Bytes: requestorID, Valid: true},
	})

	return &order, nil
}

// Transition moves a work order from one status to another after validation.
func (s *Service) Transition(ctx context.Context, id, operatorID uuid.UUID, req TransitionRequest) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}

	if err := ValidateTransition(order.Status, req.Status); err != nil {
		return nil, err
	}

	updated, err := s.queries.UpdateWorkOrderStatus(ctx, dbgen.UpdateWorkOrderStatusParams{
		ID:     id,
		Status: req.Status,
	})
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
	order, err := s.queries.GetWorkOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("work order not found: %w", err)
	}

	if order.Status != "draft" && order.Status != "rejected" {
		return fmt.Errorf("cannot delete work order in '%s' status; only draft or rejected orders can be deleted", order.Status)
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
