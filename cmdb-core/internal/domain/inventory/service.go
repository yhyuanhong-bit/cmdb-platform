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
