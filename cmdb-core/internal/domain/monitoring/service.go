package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidStateTransition is returned when an incident lifecycle call
// tries to flip the status from a state the guard doesn't allow (e.g.
// resolve on an already-closed incident). The handler maps this to 409.
var ErrInvalidStateTransition = errors.New("invalid state transition")

// Service provides alert monitoring operations.
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
	pool    *pgxpool.Pool
}

// NewService creates a new monitoring Service.
// The pool may be nil when only the non-transactional read paths are
// exercised (unit tests, old callers); lifecycle helpers that need an
// UPDATE + timeline INSERT in the same tx will reject a nil pool.
func NewService(queries *dbgen.Queries, bus eventbus.Bus, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, bus: bus, pool: pool}
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

// DeleteRule deletes an alert rule by ID.
func (s *Service) DeleteRule(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteAlertRule(ctx, id)
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

// ---------------------------------------------------------------------------
// Wave 5.1: incident lifecycle transitions.
//
// Each transition wraps the UPDATE and a system comment insert in a single
// tx so the timeline never drifts from row state. If the UPDATE's WHERE
// clause hits zero rows (meaning the status wasn't in the allowed source
// state), we translate that into ErrInvalidStateTransition at the domain
// layer — the caller doesn't need to know the SQL contract.
// ---------------------------------------------------------------------------

// withIncidentTx runs fn inside a pgx tx. Rolled back on error; committed
// otherwise. Tests that don't need lifecycle transitions may pass a nil
// pool to NewService, in which case this errors fast.
func (s *Service) withIncidentTx(ctx context.Context, fn func(qtx *dbgen.Queries) error) error {
	if s.pool == nil {
		return errors.New("monitoring service: pool is required for this operation")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)
	if err := fn(qtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// mapNoRows turns the "no rows returned" case from a WHERE-guarded UPDATE
// into ErrInvalidStateTransition. Every lifecycle helper reuses this.
func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidStateTransition
	}
	return err
}

// systemComment builds a deterministic activity-feed string for a transition.
// A free-form note from the caller is appended on a new line so the UI can
// render it verbatim. Empty notes collapse cleanly.
func systemCommentBody(action, note string) string {
	if note == "" {
		return action
	}
	return action + "\n" + note
}

func (s *Service) AcknowledgeIncident(ctx context.Context, tenantID, id, userID uuid.UUID, note string) (*dbgen.Incident, error) {
	var out dbgen.Incident
	err := s.withIncidentTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.AcknowledgeIncident(ctx, dbgen.AcknowledgeIncidentParams{
			ID: id, TenantID: tenantID, UserID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
		})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateIncidentComment(ctx, dbgen.CreateIncidentCommentParams{
			TenantID:    tenantID,
			IncidentID:  id,
			AuthorID:    pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:        "system",
			Body:        systemCommentBody("acknowledged", note),
		})
		if err != nil {
			return fmt.Errorf("write ack comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) StartInvestigatingIncident(ctx context.Context, tenantID, id, userID uuid.UUID) (*dbgen.Incident, error) {
	var out dbgen.Incident
	err := s.withIncidentTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.StartInvestigatingIncident(ctx, dbgen.StartInvestigatingIncidentParams{
			ID: id, TenantID: tenantID,
		})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateIncidentComment(ctx, dbgen.CreateIncidentCommentParams{
			TenantID:   tenantID,
			IncidentID: id,
			AuthorID:   pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:       "system",
			Body:       "investigation started",
		})
		if err != nil {
			return fmt.Errorf("write investigating comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) ResolveIncident(ctx context.Context, tenantID, id, userID uuid.UUID, rootCause, note string) (*dbgen.Incident, error) {
	var out dbgen.Incident
	err := s.withIncidentTx(ctx, func(qtx *dbgen.Queries) error {
		params := dbgen.ResolveIncidentParams{
			ID: id, TenantID: tenantID,
			UserID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
		}
		if rootCause != "" {
			params.RootCause = pgtype.Text{String: rootCause, Valid: true}
		}
		updated, err := qtx.ResolveIncident(ctx, params)
		if err != nil {
			return mapNoRows(err)
		}
		body := "resolved"
		if rootCause != "" {
			body = "resolved\nroot cause: " + rootCause
		}
		if note != "" {
			body = body + "\n" + note
		}
		_, err = qtx.CreateIncidentComment(ctx, dbgen.CreateIncidentCommentParams{
			TenantID:   tenantID,
			IncidentID: id,
			AuthorID:   pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:       "system",
			Body:       body,
		})
		if err != nil {
			return fmt.Errorf("write resolve comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) CloseIncident(ctx context.Context, tenantID, id, userID uuid.UUID) (*dbgen.Incident, error) {
	var out dbgen.Incident
	err := s.withIncidentTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.CloseIncident(ctx, dbgen.CloseIncidentParams{ID: id, TenantID: tenantID})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateIncidentComment(ctx, dbgen.CreateIncidentCommentParams{
			TenantID:   tenantID,
			IncidentID: id,
			AuthorID:   pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:       "system",
			Body:       "closed",
		})
		if err != nil {
			return fmt.Errorf("write close comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) ReopenIncident(ctx context.Context, tenantID, id, userID uuid.UUID, reason string) (*dbgen.Incident, error) {
	var out dbgen.Incident
	err := s.withIncidentTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.ReopenIncident(ctx, dbgen.ReopenIncidentParams{ID: id, TenantID: tenantID})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateIncidentComment(ctx, dbgen.CreateIncidentCommentParams{
			TenantID:   tenantID,
			IncidentID: id,
			AuthorID:   pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:       "system",
			Body:       systemCommentBody("reopened", reason),
		})
		if err != nil {
			return fmt.Errorf("write reopen comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListIncidentComments returns the timeline for an incident.
func (s *Service) ListIncidentComments(ctx context.Context, tenantID, incidentID uuid.UUID) ([]dbgen.ListIncidentCommentsRow, error) {
	return s.queries.ListIncidentComments(ctx, dbgen.ListIncidentCommentsParams{
		TenantID: tenantID, IncidentID: incidentID,
	})
}

// AddIncidentComment appends a human comment. System comments come from the
// lifecycle methods above.
func (s *Service) AddIncidentComment(ctx context.Context, tenantID, incidentID, authorID uuid.UUID, kind, body string) (*dbgen.IncidentComment, error) {
	row, err := s.queries.CreateIncidentComment(ctx, dbgen.CreateIncidentCommentParams{
		TenantID:   tenantID,
		IncidentID: incidentID,
		AuthorID:   pgtype.UUID{Bytes: authorID, Valid: authorID != uuid.Nil},
		Kind:       kind,
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}
	return &row, nil
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
