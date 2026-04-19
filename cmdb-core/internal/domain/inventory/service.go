package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides inventory task operations.
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
}

// NewService creates a new inventory Service.
func NewService(queries *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: queries, bus: bus}
}

// List returns a paginated list of inventory tasks and the total count.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, scopeLocationID *uuid.UUID, limit, offset int32) ([]dbgen.InventoryTask, int64, error) {
	params := dbgen.ListInventoryTasksParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}
	if scopeLocationID != nil {
		params.ScopeLocationID = pgtype.UUID{Bytes: *scopeLocationID, Valid: true}
	}
	tasks, err := s.queries.ListInventoryTasks(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory tasks: %w", err)
	}

	countParams := dbgen.CountInventoryTasksParams{
		TenantID: tenantID,
	}
	if scopeLocationID != nil {
		countParams.ScopeLocationID = pgtype.UUID{Bytes: *scopeLocationID, Valid: true}
	}
	total, err := s.queries.CountInventoryTasks(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count inventory tasks: %w", err)
	}

	return tasks, total, nil
}

// GetByID returns a single inventory task by its ID, scoped to the given tenant.
func (s *Service) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.InventoryTask, error) {
	task, err := s.queries.GetInventoryTask(ctx, dbgen.GetInventoryTaskParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get inventory task: %w", err)
	}
	return &task, nil
}

// ListItems returns a paginated list of items for a given inventory task and the total count.
func (s *Service) ListItems(ctx context.Context, taskID uuid.UUID, limit, offset int32) ([]dbgen.InventoryItem, int64, error) {
	items, err := s.queries.ListInventoryItems(ctx, dbgen.ListInventoryItemsParams{
		TaskID: taskID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory items: %w", err)
	}
	total, err := s.queries.CountInventoryItems(ctx, taskID)
	if err != nil {
		return nil, 0, fmt.Errorf("count inventory items: %w", err)
	}
	return items, total, nil
}

// Create creates a new inventory task.
func (s *Service) Create(ctx context.Context, params dbgen.CreateInventoryTaskParams) (*dbgen.InventoryTask, error) {
	task, err := s.queries.CreateInventoryTask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create inventory task: %w", err)
	}
	return &task, nil
}

// Complete marks an inventory task as completed. The update is scoped to the
// given tenant — a cross-tenant caller gets pgx.ErrNoRows, which callers
// surface as 404 to avoid leaking "exists in another tenant".
func (s *Service) Complete(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.InventoryTask, error) {
	task, err := s.queries.CompleteInventoryTask(ctx, dbgen.CompleteInventoryTaskParams{
		ID:       id,
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("complete inventory task: %w", err)
	}
	return &task, nil
}

// ScanItem records a scan result for an inventory item.
func (s *Service) ScanItem(ctx context.Context, params dbgen.ScanInventoryItemParams) (*dbgen.InventoryItem, error) {
	item, err := s.queries.ScanInventoryItem(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("scan inventory item: %w", err)
	}
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]interface{}{"item_id": item.ID, "task_id": item.TaskID})
		s.bus.Publish(ctx, eventbus.Event{Subject: eventbus.SubjectInventoryItemUpdated, Payload: payload})
	}
	return &item, nil
}

// GetSummary returns scan progress counts for an inventory task.
func (s *Service) GetSummary(ctx context.Context, taskID uuid.UUID) (*dbgen.GetInventorySummaryRow, error) {
	summary, err := s.queries.GetInventorySummary(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get inventory summary: %w", err)
	}
	return &summary, nil
}

// Update applies partial updates to an inventory task. Only planned/in_progress tasks can be updated.
func (s *Service) Update(ctx context.Context, tenantID, taskID uuid.UUID, name *string, plannedDate *string, assignedTo *uuid.UUID) (*dbgen.InventoryTask, error) {
	task, err := s.queries.GetInventoryTask(ctx, dbgen.GetInventoryTaskParams{ID: taskID, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("inventory task not found: %w", err)
	}
	if task.Status == "completed" {
		return nil, fmt.Errorf("cannot update completed task")
	}

	params := dbgen.UpdateInventoryTaskParams{
		ID:       taskID,
		TenantID: tenantID,
	}
	if name != nil {
		params.Name = pgtype.Text{String: *name, Valid: true}
	}
	if plannedDate != nil {
		params.PlannedDate = pgtype.Date{Time: func() time.Time { t, _ := time.Parse("2006-01-02", *plannedDate); return t }(), Valid: true}
	}
	if assignedTo != nil {
		params.AssignedTo = pgtype.UUID{Bytes: *assignedTo, Valid: true}
	}

	updated, err := s.queries.UpdateInventoryTask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update inventory task: %w", err)
	}
	return &updated, nil
}

// Delete soft-deletes an inventory task. Only planned tasks can be deleted.
func (s *Service) Delete(ctx context.Context, tenantID, taskID uuid.UUID) error {
	task, err := s.queries.GetInventoryTask(ctx, dbgen.GetInventoryTaskParams{ID: taskID, TenantID: tenantID})
	if err != nil {
		return fmt.Errorf("inventory task not found: %w", err)
	}
	if task.Status != "planned" {
		return fmt.Errorf("cannot delete task in '%s' status; only planned tasks can be deleted", task.Status)
	}

	return s.queries.SoftDeleteInventoryTask(ctx, dbgen.SoftDeleteInventoryTaskParams{
		ID:       taskID,
		TenantID: tenantID,
	})
}
