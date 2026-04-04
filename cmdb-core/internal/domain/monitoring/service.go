package monitoring

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides alert monitoring operations.
type Service struct {
	queries *dbgen.Queries
}

// NewService creates a new monitoring Service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// ListAlerts returns a paginated, filtered list of alerts and the total count.
func (s *Service) ListAlerts(ctx context.Context, tenantID uuid.UUID, status, severity *string, assetID *uuid.UUID, limit, offset int32) ([]dbgen.AlertEvent, int64, error) {
	listParams := dbgen.ListAlertsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}
	countParams := dbgen.CountAlertsParams{
		TenantID: tenantID,
	}

	if status != nil {
		listParams.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}
	if severity != nil {
		listParams.Severity = pgtype.Text{String: *severity, Valid: true}
		countParams.Severity = pgtype.Text{String: *severity, Valid: true}
	}
	if assetID != nil {
		listParams.AssetID = pgtype.UUID{Bytes: *assetID, Valid: true}
		countParams.AssetID = pgtype.UUID{Bytes: *assetID, Valid: true}
	}

	alerts, err := s.queries.ListAlerts(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list alerts: %w", err)
	}

	total, err := s.queries.CountAlerts(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count alerts: %w", err)
	}

	return alerts, total, nil
}

// ListRules returns a paginated list of alert rules.
func (s *Service) ListRules(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.AlertRule, int64, error) {
	rules, err := s.queries.ListAlertRules(ctx, dbgen.ListAlertRulesParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list alert rules: %w", err)
	}
	total, err := s.queries.CountAlertRules(ctx, tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("count alert rules: %w", err)
	}
	return rules, total, nil
}

// CreateRule creates a new alert rule.
func (s *Service) CreateRule(ctx context.Context, params dbgen.CreateAlertRuleParams) (*dbgen.AlertRule, error) {
	rule, err := s.queries.CreateAlertRule(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}
	return &rule, nil
}

// Acknowledge marks a firing alert as acknowledged.
func (s *Service) Acknowledge(ctx context.Context, id uuid.UUID) (*dbgen.AlertEvent, error) {
	alert, err := s.queries.AcknowledgeAlert(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("acknowledge alert: %w", err)
	}
	return &alert, nil
}

// Resolve marks a firing or acknowledged alert as resolved.
func (s *Service) Resolve(ctx context.Context, id uuid.UUID) (*dbgen.AlertEvent, error) {
	alert, err := s.queries.ResolveAlert(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("resolve alert: %w", err)
	}
	return &alert, nil
}
