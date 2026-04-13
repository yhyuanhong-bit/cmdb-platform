package monitoring

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides alert monitoring operations.
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
}

// NewService creates a new monitoring Service.
func NewService(queries *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: queries, bus: bus}
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
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]interface{}{"rule_id": rule.ID, "tenant_id": rule.TenantID})
		s.bus.Publish(ctx, eventbus.Event{Subject: eventbus.SubjectAlertRuleCreated, TenantID: rule.TenantID.String(), Payload: payload})
	}
	return &rule, nil
}

// UpdateRule updates an existing alert rule.
func (s *Service) UpdateRule(ctx context.Context, params dbgen.UpdateAlertRuleParams) (*dbgen.AlertRule, error) {
	rule, err := s.queries.UpdateAlertRule(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update alert rule: %w", err)
	}
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]interface{}{"rule_id": rule.ID, "tenant_id": rule.TenantID})
		s.bus.Publish(ctx, eventbus.Event{Subject: eventbus.SubjectAlertRuleUpdated, TenantID: rule.TenantID.String(), Payload: payload})
	}
	return &rule, nil
}

// ListIncidents returns a paginated list of incidents.
func (s *Service) ListIncidents(ctx context.Context, tenantID uuid.UUID, status, severity *string, limit, offset int32) ([]dbgen.Incident, int64, error) {
	listParams := dbgen.ListIncidentsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}
	countParams := dbgen.CountIncidentsParams{
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

	incidents, err := s.queries.ListIncidents(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents: %w", err)
	}
	total, err := s.queries.CountIncidents(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count incidents: %w", err)
	}
	return incidents, total, nil
}

// GetIncident returns a single incident by ID, scoped to the given tenant.
func (s *Service) GetIncident(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Incident, error) {
	incident, err := s.queries.GetIncident(ctx, dbgen.GetIncidentParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return &incident, nil
}

// CreateIncident creates a new incident.
func (s *Service) CreateIncident(ctx context.Context, params dbgen.CreateIncidentParams) (*dbgen.Incident, error) {
	incident, err := s.queries.CreateIncident(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	return &incident, nil
}

// UpdateIncident updates an existing incident.
func (s *Service) UpdateIncident(ctx context.Context, params dbgen.UpdateIncidentParams) (*dbgen.Incident, error) {
	incident, err := s.queries.UpdateIncident(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update incident: %w", err)
	}
	return &incident, nil
}

// Acknowledge marks a firing alert as acknowledged, scoped to the given tenant.
func (s *Service) Acknowledge(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.AlertEvent, error) {
	alert, err := s.queries.AcknowledgeAlert(ctx, dbgen.AcknowledgeAlertParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("acknowledge alert: %w", err)
	}
	return &alert, nil
}

// Resolve marks a firing or acknowledged alert as resolved, scoped to the given tenant.
func (s *Service) Resolve(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.AlertEvent, error) {
	alert, err := s.queries.ResolveAlert(ctx, dbgen.ResolveAlertParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("resolve alert: %w", err)
	}
	return &alert, nil
}
