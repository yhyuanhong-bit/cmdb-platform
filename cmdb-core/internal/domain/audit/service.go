package audit

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides audit event query operations.
type Service struct {
	queries *dbgen.Queries
}

// NewService creates a new audit Service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// Query returns a paginated, filtered list of audit events and the total count.
func (s *Service) Query(ctx context.Context, tenantID uuid.UUID, module, targetType *string, targetID *uuid.UUID, limit, offset int32) ([]dbgen.AuditEvent, int64, error) {
	queryParams := dbgen.QueryAuditEventsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}
	countParams := dbgen.CountAuditEventsParams{
		TenantID: tenantID,
	}

	if module != nil {
		queryParams.Module = pgtype.Text{String: *module, Valid: true}
		countParams.Module = pgtype.Text{String: *module, Valid: true}
	}
	if targetType != nil {
		queryParams.TargetType = pgtype.Text{String: *targetType, Valid: true}
		countParams.TargetType = pgtype.Text{String: *targetType, Valid: true}
	}
	if targetID != nil {
		queryParams.TargetID = pgtype.UUID{Bytes: *targetID, Valid: true}
		countParams.TargetID = pgtype.UUID{Bytes: *targetID, Valid: true}
	}

	events, err := s.queries.QueryAuditEvents(ctx, queryParams)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit events: %w", err)
	}

	total, err := s.queries.CountAuditEvents(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit events: %w", err)
	}

	return events, total, nil
}
