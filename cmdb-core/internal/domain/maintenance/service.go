package maintenance

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
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
	return s.createWith(ctx, s.queries, tenantID, requestorID, req, true)
}

// CreateTx is the same as Create but runs inside the caller's transaction.
// Used by the auto-workorder scans (Phase 2.15) so the WO insert and the
// work_order_dedup insert commit or roll back atomically — otherwise a
// crash between the two leaves either an orphan WO or a ghost dedup row.
//
// The caller owns the tx lifecycle (Begin / Commit / Rollback). This
// helper only runs the two inserts using qtx := queries.WithTx(tx).
//
// incrementSyncVersion is deliberately SKIPPED in the tx path: it uses
// s.pool.Exec which would run outside the tx (and deadlock against the
// uncommitted work_orders row). The sync_version bump is done by the
// caller AFTER the tx commits, via BumpSyncVersionAfterCreate below.
func (s *Service) CreateTx(ctx context.Context, tx pgx.Tx, tenantID, requestorID uuid.UUID, req CreateOrderRequest) (*dbgen.WorkOrder, error) {
	qtx := s.queries.WithTx(tx)
	return s.createWith(ctx, qtx, tenantID, requestorID, req, false)
}

// BumpSyncVersionAfterCreate runs the post-commit sync_version bump that
// CreateTx intentionally skipped. Callers that use CreateTx should invoke
// this once their tx commits successfully. A failure here is non-fatal —
// sync_version only drives edge-to-central replication and is reconciled
// by the next real update.
func (s *Service) BumpSyncVersionAfterCreate(ctx context.Context, orderID, tenantID uuid.UUID) {
	s.incrementSyncVersion(ctx, "work_orders", orderID, tenantID)
}

// createWith holds the shared insert+log logic for Create / CreateTx.
// `bumpSyncVersion` is true for the pool-backed Create (which can run
// the separate UPDATE immediately) and false for CreateTx (where the
// UPDATE must wait for the caller's commit — see BumpSyncVersionAfterCreate).
func (s *Service) createWith(
	ctx context.Context,
	q *dbgen.Queries,
	tenantID, requestorID uuid.UUID,
	req CreateOrderRequest,
	bumpSyncVersion bool,
) (*dbgen.WorkOrder, error) {
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

	order, err := q.CreateWorkOrder(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create work order: %w", err)
	}
	if bumpSyncVersion {
		s.incrementSyncVersion(ctx, "work_orders", order.ID, tenantID)
	}

	// Create initial log entry
	_, _ = q.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
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

	if vErr := ValidateExecTransition(order.ExecutionStatus, newExec); vErr != nil {
		return nil, vErr
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

	s.incrementSyncVersion(ctx, "work_orders", id, tenantID)

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

	if vErr := ValidateGovTransition(order.GovernanceStatus, newGov); vErr != nil {
		return nil, vErr
	}

	if newGov == GovApproved || newGov == GovRejected {
		if aErr := validateApproval(operatorID, order.RequestorID, operatorRoles); aErr != nil {
			return nil, aErr
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

	s.incrementSyncVersion(ctx, "work_orders", id, tenantID)

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

// TransitionEmergencyAtomic approves-and-starts an emergency work order in a
// single SQL UPDATE. This replaces the previous two-step flow
// (Transition(approved) → Transition(in_progress)) which could strand the WO
// half-approved if the process crashed, timed out, or retried between the two
// statements. The half-approved state then tripped SLA scans for a row that
// was never actually in progress.
//
// The underlying UPDATE guards on tenant_id, type='emergency',
// governance_status='submitted', and execution_status='pending', so a second
// concurrent caller (or a stale retry after a successful transition) finds
// zero rows and the database returns pgx.ErrNoRows. That is treated as
// idempotent success — (nil, nil) — because the caller's intent is already
// satisfied.
//
// operatorID may be uuid.Nil to mark a system auto-approval; validateApproval
// is deliberately NOT invoked here because emergency auto-approval is a
// system-authoritative action triggered by the alert event pipeline, not a
// user decision.
func (s *Service) TransitionEmergencyAtomic(ctx context.Context, tenantID, orderID, approverID uuid.UUID) (*dbgen.WorkOrder, error) {
	deadline := SLADeadline("critical", time.Now())
	approverUUID := pgtype.UUID{Bytes: approverID, Valid: true}

	updated, err := s.queries.TransitionEmergencyWorkOrder(ctx, dbgen.TransitionEmergencyWorkOrderParams{
		ID:          orderID,
		TenantID:    tenantID,
		ApprovedBy:  approverUUID,
		SlaDeadline: pgtype.Timestamptz{Time: deadline, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Idempotent: already transitioned, wrong type, cross-tenant, or
			// a concurrent winner beat us. Not an error for the caller.
			return nil, nil
		}
		return nil, fmt.Errorf("atomic emergency transition: %w", err)
	}

	s.incrementSyncVersion(ctx, "work_orders", orderID, tenantID)

	// Best-effort audit trail. Failing to write the log should not roll back
	// a successful state transition — the transition is the source of truth.
	if _, logErr := s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
		OrderID:    orderID,
		Action:     "emergency_auto_approve",
		FromStatus: pgtype.Text{String: StatusSubmitted, Valid: true},
		ToStatus:   pgtype.Text{String: StatusInProgress, Valid: true},
		OperatorID: approverUUID,
		Comment:    pgtype.Text{String: "Auto-approved + started: emergency work order", Valid: true},
	}); logErr != nil {
		zap.L().Warn("maintenance: emergency audit log failed",
			zap.String("order_id", orderID.String()), zap.Error(logErr))
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

// Assign reassigns a work order to a different operator. Reassignment is
// allowed in submitted, rejected, approved, and in_progress — completed
// and verified orders are immutable. Update() rejects approved+ states
// because it edits domain content (title, priority, schedule); reassignment
// is a workflow action that must work on in-flight orders too. Returns
// ErrAssignNotAllowed when the order exists but is in a frozen state, and
// wraps pgx.ErrNoRows otherwise so callers can distinguish 404 from 422.
func (s *Service) Assign(ctx context.Context, tenantID, orderID, assigneeID uuid.UUID) (*dbgen.WorkOrder, error) {
	order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: orderID, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("work order not found: %w", err)
	}

	switch order.Status {
	case StatusSubmitted, StatusRejected, StatusApproved, StatusInProgress:
		// allowed
	default:
		return nil, fmt.Errorf("cannot reassign work order in '%s' status; only submitted, rejected, approved, or in_progress orders can be reassigned", order.Status)
	}

	updated, err := s.queries.AssignWorkOrder(ctx, dbgen.AssignWorkOrderParams{
		ID:         orderID,
		AssigneeID: pgtype.UUID{Bytes: assigneeID, Valid: true},
		TenantID:   tenantID,
	})
	if err != nil {
		// Treated as a state race: GetWorkOrder said allowed, the row
		// guard rejected the UPDATE because a concurrent transition
		// moved the order out of an assignable status.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("work order state changed concurrently, please retry")
		}
		return nil, fmt.Errorf("assign work order: %w", err)
	}

	s.incrementSyncVersion(ctx, "work_orders", orderID, tenantID)

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

func (s *Service) incrementSyncVersion(ctx context.Context, table string, id, tenantID uuid.UUID) {
	if s.pool == nil {
		return
	}
	tableIdent := pgx.Identifier{table}.Sanitize()
	sc := database.Scope(s.pool, tenantID)
	if _, err := sc.Exec(ctx, fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $2 AND tenant_id = $1", tableIdent), id); err != nil {
		zap.L().Error("maintenance: failed to increment sync_version", zap.String("table", table), zap.Error(err))
	}
}
