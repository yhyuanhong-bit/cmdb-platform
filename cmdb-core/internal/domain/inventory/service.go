package inventory

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

// Service provides inventory task operations.
type Service struct {
	queries *dbgen.Queries
}

// NewService creates a new inventory Service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// List returns a paginated list of inventory tasks and the total count.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.InventoryTask, int64, error) {
	tasks, err := s.queries.ListInventoryTasks(ctx, dbgen.ListInventoryTasksParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory tasks: %w", err)
	}

	total, err := s.queries.CountInventoryTasks(ctx, tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("count inventory tasks: %w", err)
	}

	return tasks, total, nil
}

// GetByID returns a single inventory task by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.InventoryTask, error) {
	task, err := s.queries.GetInventoryTask(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get inventory task: %w", err)
	}
	return &task, nil
}

// ListItems returns all items for a given inventory task.
func (s *Service) ListItems(ctx context.Context, taskID uuid.UUID) ([]dbgen.InventoryItem, error) {
	items, err := s.queries.ListInventoryItems(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list inventory items: %w", err)
	}
	return items, nil
}

// Create creates a new inventory task.
func (s *Service) Create(ctx context.Context, params dbgen.CreateInventoryTaskParams) (*dbgen.InventoryTask, error) {
	task, err := s.queries.CreateInventoryTask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create inventory task: %w", err)
	}
	return &task, nil
}

// Complete marks an inventory task as completed.
func (s *Service) Complete(ctx context.Context, id uuid.UUID) (*dbgen.InventoryTask, error) {
	task, err := s.queries.CompleteInventoryTask(ctx, id)
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
