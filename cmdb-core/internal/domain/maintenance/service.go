package maintenance

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides work order domain operations.
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
	pool    *pgxpool.Pool
}

// NewService creates a new maintenance Service.
func NewService(queries *dbgen.Queries, bus eventbus.Bus, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, bus: bus, pool: pool}
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

	validPriorities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
	if !validPriorities[priority] {
		return nil, fmt.Errorf("invalid priority %q; must be critical, high, medium, or low", priority)
	}

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
	s.incrementSyncVersion(ctx, "work_orders", order.ID)

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
// It delegates to TransitionGovernance or TransitionExecution based on the target status.
func (s *Service) Transition(ctx context.Context, tenantID, id, operatorID uuid.UUID, operatorRoles []string, req TransitionRequest) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}

	if err := ValidateTransition(order.Status, req.Status); err != nil {
		return nil, err
	}

	switch req.Status {
	case StatusApproved, StatusRejected, StatusVerified:
		return s.TransitionGovernance(ctx, tenantID, id, operatorID, operatorRoles, req.Status, req.Comment)
	case StatusInProgress:
		return s.TransitionExecution(ctx, tenantID, id, operatorID, ExecWorking)
	case StatusCompleted:
		return s.TransitionExecution(ctx, tenantID, id, operatorID, ExecDone)
	default:
		return nil, fmt.Errorf("unsupported transition target %q", req.Status)
	}
}

// TransitionExecution updates execution_status independently.
// Used by sync layer — allows status jumps (e.g., Edge starts work before Central approves).
func (s *Service) TransitionExecution(ctx context.Context, tenantID, id, operatorID uuid.UUID, newExec string) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}

	if err := ValidateExecTransition(order.ExecutionStatus, newExec); err != nil {
		return nil, err
	}

	derivedStatus, err := DeriveStatus(newExec, order.GovernanceStatus)
	if err != nil {
		return nil, fmt.Errorf("derive status: %w", err)
	}

	updated, err := s.queries.UpdateExecutionStatus(ctx, dbgen.UpdateExecutionStatusParams{
		ID:                id,
		ExecutionStatus:   newExec,
		Status:            derivedStatus,
		TenantID:          tenantID,
		ExecutionStatus_2: order.ExecutionStatus, // optimistic lock
	})
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("execution_status has changed concurrently, please retry")
		}
		return nil, fmt.Errorf("update execution_status: %w", err)
	}

	s.incrementSyncVersion(ctx, "work_orders", id)

	_, _ = s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
		OrderID:    id,
		Action:     "transition",
		FromStatus: pgtype.Text{String: order.ExecutionStatus, Valid: true},
		ToStatus:   pgtype.Text{String: newExec, Valid: true},
		OperatorID: pgtype.UUID{Bytes: operatorID, Valid: true},
	})

	// Anomaly detection: done + rejected
	if newExec == ExecDone && updated.GovernanceStatus == GovRejected && s.bus != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"order_id": id, "execution_status": newExec, "governance_status": updated.GovernanceStatus,
		})
		s.bus.Publish(ctx, eventbus.Event{
			Subject: eventbus.SubjectOrderAnomaly, TenantID: tenantID.String(), Payload: payload,
		})
	}

	return &updated, nil
}

// TransitionGovernance updates governance_status independently.
func (s *Service) TransitionGovernance(ctx context.Context, tenantID, id, operatorID uuid.UUID, operatorRoles []string, newGov, comment string) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}

	if err := ValidateGovTransition(order.GovernanceStatus, newGov); err != nil {
		return nil, err
	}

	if newGov == GovApproved || newGov == GovRejected {
		if err := validateApproval(operatorID, order.RequestorID, operatorRoles); err != nil {
			return nil, err
		}
		if comment == "" {
			return nil, fmt.Errorf("approval/rejection requires a comment")
		}
	}

	derivedStatus, err := DeriveStatus(order.ExecutionStatus, newGov)
	if err != nil {
		return nil, fmt.Errorf("derive status: %w", err)
	}

	var updated dbgen.WorkOrder
	if newGov == GovApproved {
		now := time.Now()
		deadline := SLADeadline(order.Priority, now)
		updated, err = s.queries.StampWorkOrderApproval(ctx, dbgen.StampWorkOrderApprovalParams{
			ID:          id,
			ApprovedBy:  pgtype.UUID{Bytes: operatorID, Valid: true},
			SlaDeadline: pgtype.Timestamptz{Time: deadline, Valid: true},
			TenantID:    tenantID,
		})
	} else {
		updated, err = s.queries.UpdateGovernanceStatus(ctx, dbgen.UpdateGovernanceStatusParams{
			ID:                 id,
			GovernanceStatus:   newGov,
			Status:             derivedStatus,
			TenantID:           tenantID,
			GovernanceStatus_2: order.GovernanceStatus, // optimistic lock
		})
	}
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("governance_status has changed concurrently, please retry")
		}
		return nil, fmt.Errorf("update governance_status: %w", err)
	}

	s.incrementSyncVersion(ctx, "work_orders", id)

	logParams := dbgen.CreateWorkOrderLogParams{
		OrderID:    id,
		Action:     "transition",
		FromStatus: pgtype.Text{String: order.GovernanceStatus, Valid: true},
		ToStatus:   pgtype.Text{String: newGov, Valid: true},
		OperatorID: pgtype.UUID{Bytes: operatorID, Valid: true},
	}
	if comment != "" {
		logParams.Comment = pgtype.Text{String: comment, Valid: true}
	}
	_, _ = s.queries.CreateWorkOrderLog(ctx, logParams)

	if updated.ExecutionStatus == ExecDone && newGov == GovRejected && s.bus != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"order_id": id, "execution_status": updated.ExecutionStatus, "governance_status": newGov,
		})
		s.bus.Publish(ctx, eventbus.Event{
			Subject: eventbus.SubjectOrderAnomaly, TenantID: tenantID.String(), Payload: payload,
		})
	}

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
// Only submitted or rejected orders can be edited.
func (s *Service) Update(ctx context.Context, tenantID uuid.UUID, params dbgen.UpdateWorkOrderParams) (*dbgen.WorkOrder, error) {
	// Check current status - only allow edits on submitted or rejected orders
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{
		ID:       params.ID,
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("work order not found: %w", err)
	}
	if order.Status != StatusSubmitted && order.Status != StatusRejected {
		return nil, fmt.Errorf("cannot modify work order in '%s' status; only submitted or rejected orders can be edited", order.Status)
	}

	updated, err := s.queries.UpdateWorkOrder(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update work order: %w", err)
	}
	return &updated, nil
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

func (s *Service) incrementSyncVersion(ctx context.Context, table string, id uuid.UUID) {
	if s.pool == nil {
		return
	}
	_, _ = s.pool.Exec(ctx, fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", table), id)
}
