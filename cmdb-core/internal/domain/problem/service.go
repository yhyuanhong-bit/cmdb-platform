// Package problem implements the ITIL Problem entity and its lifecycle.
//
// A Problem is the underlying root cause of one or more Incidents. Where an
// Incident is "the user can't log in right now," the Problem is "the auth
// service has a memory leak." The two have separate lifecycles: an
// incident is closed when the symptom goes away; a problem is closed only
// after a permanent fix lands and the workaround is no longer needed.
//
// The state machine here mirrors monitoring.Service's incident lifecycle
// (Wave 5.1) so a future ITSM dashboard can render both with the same UI
// vocabulary:
//
//	open → investigating → known_error (optional) → resolved → closed
//
// 'known_error' is ITIL's name for "we know the cause and have a documented
// workaround, but the permanent fix hasn't shipped." Skipping it is fine
// when the fix lands fast.
package problem

import (
	"context"
	"errors"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidStateTransition is returned when a lifecycle call tries to flip
// the status from a state the WHERE-status guard doesn't allow. The
// handler maps this to 409.
var ErrInvalidStateTransition = errors.New("problem: invalid state transition")

// ErrNotFound is returned when the row isn't visible to the caller's
// tenant (which we surface as 404 to avoid leaking row existence).
var ErrNotFound = errors.New("problem: not found")

// Service bundles the problem queries and the pool needed for tx-scoped
// lifecycle helpers. The pool may be nil for read-only callers; the
// transactional helpers reject a nil pool fast.
type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// CreateParams is the inbound shape for a new problem. priority/severity
// validation lives at the DB CHECK level; we just pass through.
type CreateParams struct {
	TenantID    uuid.UUID
	Title       string
	Description string
	Priority    string // p1..p4 or empty
	Severity    string // critical/high/medium/low/info/warning
	Workaround  string
	AssigneeID  uuid.UUID // uuid.Nil for unassigned
	CreatedBy   uuid.UUID
}

func (s *Service) Create(ctx context.Context, p CreateParams) (*dbgen.Problem, error) {
	if p.Title == "" {
		return nil, errors.New("problem: title required")
	}
	severity := p.Severity
	if severity == "" {
		severity = "medium"
	}
	row, err := s.queries.CreateProblem(ctx, dbgen.CreateProblemParams{
		TenantID:       p.TenantID,
		Title:          p.Title,
		Description:    pgtype.Text{String: p.Description, Valid: p.Description != ""},
		Status:         "open",
		Priority:       pgtype.Text{String: p.Priority, Valid: p.Priority != ""},
		Severity:       severity,
		Workaround:     pgtype.Text{String: p.Workaround, Valid: p.Workaround != ""},
		AssigneeUserID: pgtype.UUID{Bytes: p.AssigneeID, Valid: p.AssigneeID != uuid.Nil},
		CreatedBy:      pgtype.UUID{Bytes: p.CreatedBy, Valid: p.CreatedBy != uuid.Nil},
	})
	if err != nil {
		return nil, fmt.Errorf("create problem: %w", err)
	}
	return &row, nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Problem, error) {
	row, err := s.queries.GetProblem(ctx, dbgen.GetProblemParams{ID: id, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get problem: %w", err)
	}
	return &row, nil
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status, priority *string, limit, offset int32) ([]dbgen.Problem, int64, error) {
	listParams := dbgen.ListProblemsParams{TenantID: tenantID, Limit: limit, Offset: offset}
	countParams := dbgen.CountProblemsParams{TenantID: tenantID}
	if status != nil && *status != "" {
		listParams.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}
	if priority != nil && *priority != "" {
		listParams.Priority = pgtype.Text{String: *priority, Valid: true}
		countParams.Priority = pgtype.Text{String: *priority, Valid: true}
	}
	rows, err := s.queries.ListProblems(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list problems: %w", err)
	}
	total, err := s.queries.CountProblems(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count problems: %w", err)
	}
	return rows, total, nil
}

// UpdateParams covers the partial-update fields. nil = leave unchanged.
type UpdateParams struct {
	TenantID    uuid.UUID
	ID          uuid.UUID
	Title       *string
	Description *string
	Severity    *string
	Priority    *string
	Workaround  *string
	RootCause   *string
	Resolution  *string
	AssigneeID  *uuid.UUID
}

func (s *Service) Update(ctx context.Context, p UpdateParams) (*dbgen.Problem, error) {
	params := dbgen.UpdateProblemParams{ID: p.ID, TenantID: p.TenantID}
	if p.Title != nil {
		params.Title = pgtype.Text{String: *p.Title, Valid: true}
	}
	if p.Description != nil {
		params.Description = pgtype.Text{String: *p.Description, Valid: true}
	}
	if p.Severity != nil {
		params.Severity = pgtype.Text{String: *p.Severity, Valid: true}
	}
	if p.Priority != nil {
		params.Priority = pgtype.Text{String: *p.Priority, Valid: true}
	}
	if p.Workaround != nil {
		params.Workaround = pgtype.Text{String: *p.Workaround, Valid: true}
	}
	if p.RootCause != nil {
		params.RootCause = pgtype.Text{String: *p.RootCause, Valid: true}
	}
	if p.Resolution != nil {
		params.Resolution = pgtype.Text{String: *p.Resolution, Valid: true}
	}
	if p.AssigneeID != nil {
		params.AssigneeUserID = pgtype.UUID{Bytes: *p.AssigneeID, Valid: *p.AssigneeID != uuid.Nil}
	}
	row, err := s.queries.UpdateProblem(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update problem: %w", err)
	}
	return &row, nil
}

// ---------------------------------------------------------------------------
// Lifecycle helpers — each wraps UPDATE + system-comment INSERT in a tx
// so the timeline is consistent with the row state. Same pattern as
// monitoring.Service incident lifecycle.
// ---------------------------------------------------------------------------

func (s *Service) withTx(ctx context.Context, fn func(qtx *dbgen.Queries) error) error {
	if s.pool == nil {
		return errors.New("problem service: pool is required for this operation")
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

func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidStateTransition
	}
	return err
}

func systemCommentBody(action, note string) string {
	if note == "" {
		return action
	}
	return action + "\n" + note
}

func (s *Service) StartInvestigation(ctx context.Context, tenantID, id, userID uuid.UUID, note string) (*dbgen.Problem, error) {
	var out dbgen.Problem
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.StartInvestigatingProblem(ctx, dbgen.StartInvestigatingProblemParams{
			ID: id, TenantID: tenantID,
		})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateProblemComment(ctx, dbgen.CreateProblemCommentParams{
			TenantID:  tenantID,
			ProblemID: id,
			AuthorID:  pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:      "system",
			Body:      systemCommentBody("investigation started", note),
		})
		if err != nil {
			return fmt.Errorf("write start-investigation comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkKnownError requires a workaround — that's the whole point of this
// ITIL state. Empty workaround returns an error, not a transition.
func (s *Service) MarkKnownError(ctx context.Context, tenantID, id, userID uuid.UUID, workaround, note string) (*dbgen.Problem, error) {
	if workaround == "" {
		return nil, errors.New("problem: workaround is required to mark as known_error")
	}
	var out dbgen.Problem
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.MarkProblemKnownError(ctx, dbgen.MarkProblemKnownErrorParams{
			ID: id, TenantID: tenantID,
			Workaround: pgtype.Text{String: workaround, Valid: true},
		})
		if err != nil {
			return mapNoRows(err)
		}
		body := "marked as known error\nworkaround: " + workaround
		if note != "" {
			body = body + "\n" + note
		}
		_, err = qtx.CreateProblemComment(ctx, dbgen.CreateProblemCommentParams{
			TenantID:  tenantID,
			ProblemID: id,
			AuthorID:  pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:      "system",
			Body:      body,
		})
		if err != nil {
			return fmt.Errorf("write known-error comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) Resolve(ctx context.Context, tenantID, id, userID uuid.UUID, rootCause, resolution, note string) (*dbgen.Problem, error) {
	var out dbgen.Problem
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		params := dbgen.ResolveProblemParams{
			ID: id, TenantID: tenantID,
			UserID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
		}
		if rootCause != "" {
			params.RootCause = pgtype.Text{String: rootCause, Valid: true}
		}
		if resolution != "" {
			params.Resolution = pgtype.Text{String: resolution, Valid: true}
		}
		updated, err := qtx.ResolveProblem(ctx, params)
		if err != nil {
			return mapNoRows(err)
		}
		body := "resolved"
		if rootCause != "" {
			body += "\nroot cause: " + rootCause
		}
		if resolution != "" {
			body += "\nresolution: " + resolution
		}
		if note != "" {
			body += "\n" + note
		}
		_, err = qtx.CreateProblemComment(ctx, dbgen.CreateProblemCommentParams{
			TenantID:  tenantID,
			ProblemID: id,
			AuthorID:  pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:      "system",
			Body:      body,
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

func (s *Service) Close(ctx context.Context, tenantID, id, userID uuid.UUID) (*dbgen.Problem, error) {
	var out dbgen.Problem
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.CloseProblem(ctx, dbgen.CloseProblemParams{ID: id, TenantID: tenantID})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateProblemComment(ctx, dbgen.CreateProblemCommentParams{
			TenantID:  tenantID,
			ProblemID: id,
			AuthorID:  pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:      "system",
			Body:      "closed",
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

func (s *Service) Reopen(ctx context.Context, tenantID, id, userID uuid.UUID, reason string) (*dbgen.Problem, error) {
	var out dbgen.Problem
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := qtx.ReopenProblem(ctx, dbgen.ReopenProblemParams{ID: id, TenantID: tenantID})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateProblemComment(ctx, dbgen.CreateProblemCommentParams{
			TenantID:  tenantID,
			ProblemID: id,
			AuthorID:  pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:      "system",
			Body:      systemCommentBody("reopened", reason),
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

// ---------------------------------------------------------------------------
// Linkage with incidents.
// ---------------------------------------------------------------------------

// LinkIncident attaches an incident to a problem. Idempotent — a repeat
// link is silently a no-op (matches ON CONFLICT DO NOTHING). Both rows
// must belong to the caller's tenant; the queries enforce that.
func (s *Service) LinkIncident(ctx context.Context, tenantID, incidentID, problemID, createdBy uuid.UUID) error {
	// Defence in depth: confirm both rows are visible to this tenant before
	// inserting the link. Stops a forged incident_id or problem_id from
	// landing a row that quietly references a foreign tenant's incident.
	if _, err := s.Get(ctx, tenantID, problemID); err != nil {
		return err
	}
	if _, err := s.queries.GetIncident(ctx, dbgen.GetIncidentParams{ID: incidentID, TenantID: tenantID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("verify incident: %w", err)
	}
	return s.queries.LinkIncidentToProblem(ctx, dbgen.LinkIncidentToProblemParams{
		IncidentID: incidentID,
		ProblemID:  problemID,
		TenantID:   tenantID,
		CreatedBy:  pgtype.UUID{Bytes: createdBy, Valid: createdBy != uuid.Nil},
	})
}

func (s *Service) UnlinkIncident(ctx context.Context, tenantID, incidentID, problemID uuid.UUID) error {
	return s.queries.UnlinkIncidentFromProblem(ctx, dbgen.UnlinkIncidentFromProblemParams{
		IncidentID: incidentID,
		ProblemID:  problemID,
		TenantID:   tenantID,
	})
}

func (s *Service) ListIncidentsForProblem(ctx context.Context, tenantID, problemID uuid.UUID) ([]dbgen.Incident, error) {
	return s.queries.ListIncidentsForProblem(ctx, dbgen.ListIncidentsForProblemParams{
		TenantID: tenantID, ProblemID: problemID,
	})
}

func (s *Service) ListProblemsForIncident(ctx context.Context, tenantID, incidentID uuid.UUID) ([]dbgen.Problem, error) {
	return s.queries.ListProblemsForIncident(ctx, dbgen.ListProblemsForIncidentParams{
		TenantID: tenantID, IncidentID: incidentID,
	})
}

// ---------------------------------------------------------------------------
// Comments.
// ---------------------------------------------------------------------------

func (s *Service) ListComments(ctx context.Context, tenantID, problemID uuid.UUID) ([]dbgen.ListProblemCommentsRow, error) {
	return s.queries.ListProblemComments(ctx, dbgen.ListProblemCommentsParams{
		TenantID: tenantID, ProblemID: problemID,
	})
}

func (s *Service) AddComment(ctx context.Context, tenantID, problemID, authorID uuid.UUID, kind, body string) (*dbgen.ProblemComment, error) {
	row, err := s.queries.CreateProblemComment(ctx, dbgen.CreateProblemCommentParams{
		TenantID:  tenantID,
		ProblemID: problemID,
		AuthorID:  pgtype.UUID{Bytes: authorID, Valid: authorID != uuid.Nil},
		Kind:      kind,
		Body:      body,
	})
	if err != nil {
		return nil, fmt.Errorf("create problem comment: %w", err)
	}
	return &row, nil
}
