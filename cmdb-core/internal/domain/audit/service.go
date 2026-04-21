package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// OperatorType classifies *who* produced an audit event. The value maps 1:1
// to the Postgres audit_operator_type ENUM introduced in migration 000051
// and enforced by chk_audit_operator_type_id_match: operator_id must be
// set iff operator_type=OperatorTypeUser.
type OperatorType string

const (
	OperatorTypeUser        OperatorType = "user"
	OperatorTypeSystem      OperatorType = "system"
	OperatorTypeIntegration OperatorType = "integration"
	OperatorTypeSync        OperatorType = "sync"
	OperatorTypeAnonymous   OperatorType = "anonymous"
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

// Record creates a new audit event entry for a write operation.
//
// operatorType must match the DB ENUM (see OperatorType constants).
// operatorID is the authenticated user's UUID for OperatorTypeUser and
// must be nil for every other operator type — the CHECK constraint on
// audit_events rejects mismatches, and we validate client-side here so
// callers get a clear error instead of a cryptic Postgres 23514.
func (s *Service) Record(
	ctx context.Context,
	tenantID uuid.UUID,
	action, module, targetType string,
	targetID uuid.UUID,
	operatorType OperatorType,
	operatorID *uuid.UUID,
	diff map[string]any,
	source string,
) error {
	opID := pgtype.UUID{}
	if operatorType == OperatorTypeUser {
		if operatorID == nil {
			return fmt.Errorf("audit.Record: operator_type=user requires operator_id")
		}
		opID = pgtype.UUID{Bytes: *operatorID, Valid: true}
	} else if operatorID != nil {
		return fmt.Errorf("audit.Record: operator_type=%q must not carry operator_id", operatorType)
	}

	diffJSON, _ := json.Marshal(diff)

	_, err := s.queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		TenantID:     tenantID,
		Action:       action,
		Module:       pgtype.Text{String: module, Valid: true},
		TargetType:   pgtype.Text{String: targetType, Valid: true},
		TargetID:     pgtype.UUID{Bytes: targetID, Valid: true},
		OperatorType: dbgen.AuditOperatorType(operatorType),
		OperatorID:   opID,
		Diff:         diffJSON,
		Source:       source,
	})
	return err
}
