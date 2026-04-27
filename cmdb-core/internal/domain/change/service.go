// Package change implements the ITIL Change Management entity, the CAB
// approval flow, and the M:N linkage with assets / services / problems.
//
// Lifecycle (status field):
//
//	draft → submitted → approved | rejected
//	approved → in_progress → succeeded | failed | rolled_back
//
// Three change types are supported:
//
//	standard  — pre-approved low-risk procedures. Submit auto-transitions
//	            straight to `approved` (skips CAB review). Used for
//	            patching to known-good versions, restarting a service,
//	            etc. The audit trail flags the auto-approval so reviewers
//	            can audit what got skipped.
//	normal    — needs CAB approval. Multiple voters cast `approve` /
//	            `reject` / `abstain` rows; the change auto-resolves once
//	            approve count reaches approval_threshold (and no reject
//	            exists), or auto-rejects on a single reject.
//	emergency — bypasses the CAB queue but is flagged for retroactive
//	            review. Same submit→approved auto-flow as standard, but
//	            the type field marks it for reporting.
//
// The lifecycle and approval logic are kept inside this package so
// callers (handlers, schedulers, integration tests) get a single,
// consistent view. The handler layer translates HTTP status codes from
// the domain errors below.
package change

import (
	"context"
	"errors"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrInvalidStateTransition surfaces when a lifecycle WHERE-status
	// guard hits zero rows. The handler maps this to 409.
	ErrInvalidStateTransition = errors.New("change: invalid state transition")

	// ErrNotFound — row isn't visible to the caller's tenant. Mapped
	// to 404 (don't leak existence).
	ErrNotFound = errors.New("change: not found")
)

type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// CreateParams is the inbound shape for a new change. CHECK constraints
// at the DB layer enforce type/risk/status enums; we just pass through.
type CreateParams struct {
	TenantID          uuid.UUID
	Title             string
	Description       string
	Type              string // standard / normal / emergency
	Risk              string // low / medium / high / critical
	ApprovalThreshold int32
	RequestedBy       uuid.UUID
	AssigneeID        uuid.UUID
	PlannedStart      *pgtype.Timestamptz
	PlannedEnd        *pgtype.Timestamptz
	RollbackPlan      string
	ImpactSummary     string
}

func (s *Service) Create(ctx context.Context, p CreateParams) (*dbgen.Change, error) {
	if p.Title == "" {
		return nil, errors.New("change: title required")
	}
	typ := p.Type
	if typ == "" {
		typ = "normal"
	}
	risk := p.Risk
	if risk == "" {
		risk = "medium"
	}
	threshold := p.ApprovalThreshold
	// Standard changes auto-approve on submit, so the threshold is 0
	// (no votes required). Other types default to 1 unless caller set
	// something explicit.
	if typ == "standard" {
		threshold = 0
	} else if threshold <= 0 {
		threshold = 1
	}

	params := dbgen.CreateChangeParams{
		TenantID:          p.TenantID,
		Title:             p.Title,
		Description:       pgtype.Text{String: p.Description, Valid: p.Description != ""},
		Type:              typ,
		Risk:              risk,
		Status:            "draft",
		ApprovalThreshold: threshold,
		RequestedBy:       pgtype.UUID{Bytes: p.RequestedBy, Valid: p.RequestedBy != uuid.Nil},
		AssigneeUserID:    pgtype.UUID{Bytes: p.AssigneeID, Valid: p.AssigneeID != uuid.Nil},
		RollbackPlan:      pgtype.Text{String: p.RollbackPlan, Valid: p.RollbackPlan != ""},
		ImpactSummary:     pgtype.Text{String: p.ImpactSummary, Valid: p.ImpactSummary != ""},
	}
	if p.PlannedStart != nil {
		params.PlannedStart = *p.PlannedStart
	}
	if p.PlannedEnd != nil {
		params.PlannedEnd = *p.PlannedEnd
	}

	row, err := s.queries.CreateChange(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create change: %w", err)
	}
	return &row, nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Change, error) {
	row, err := s.queries.GetChange(ctx, dbgen.GetChangeParams{ID: id, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get change: %w", err)
	}
	return &row, nil
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status, typ, risk *string, limit, offset int32) ([]dbgen.Change, int64, error) {
	listParams := dbgen.ListChangesParams{TenantID: tenantID, Limit: limit, Offset: offset}
	countParams := dbgen.CountChangesParams{TenantID: tenantID}
	if status != nil && *status != "" {
		listParams.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}
	if typ != nil && *typ != "" {
		listParams.Type = pgtype.Text{String: *typ, Valid: true}
		countParams.Type = pgtype.Text{String: *typ, Valid: true}
	}
	if risk != nil && *risk != "" {
		listParams.Risk = pgtype.Text{String: *risk, Valid: true}
		countParams.Risk = pgtype.Text{String: *risk, Valid: true}
	}
	rows, err := s.queries.ListChanges(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list changes: %w", err)
	}
	total, err := s.queries.CountChanges(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count changes: %w", err)
	}
	return rows, total, nil
}

// UpdateParams covers partial-update fields. Status routes through the
// transition methods, not Update.
type UpdateParams struct {
	TenantID          uuid.UUID
	ID                uuid.UUID
	Title             *string
	Description       *string
	Type              *string
	Risk              *string
	ApprovalThreshold *int32
	AssigneeID        *uuid.UUID
	PlannedStart      *pgtype.Timestamptz
	PlannedEnd        *pgtype.Timestamptz
	RollbackPlan      *string
	ImpactSummary     *string
}

func (s *Service) Update(ctx context.Context, p UpdateParams) (*dbgen.Change, error) {
	params := dbgen.UpdateChangeParams{ID: p.ID, TenantID: p.TenantID}
	if p.Title != nil {
		params.Title = pgtype.Text{String: *p.Title, Valid: true}
	}
	if p.Description != nil {
		params.Description = pgtype.Text{String: *p.Description, Valid: true}
	}
	if p.Type != nil {
		params.Type = pgtype.Text{String: *p.Type, Valid: true}
	}
	if p.Risk != nil {
		params.Risk = pgtype.Text{String: *p.Risk, Valid: true}
	}
	if p.ApprovalThreshold != nil {
		params.ApprovalThreshold = pgtype.Int4{Int32: *p.ApprovalThreshold, Valid: true}
	}
	if p.AssigneeID != nil {
		params.AssigneeUserID = pgtype.UUID{Bytes: *p.AssigneeID, Valid: *p.AssigneeID != uuid.Nil}
	}
	if p.PlannedStart != nil {
		params.PlannedStart = *p.PlannedStart
	}
	if p.PlannedEnd != nil {
		params.PlannedEnd = *p.PlannedEnd
	}
	if p.RollbackPlan != nil {
		params.RollbackPlan = pgtype.Text{String: *p.RollbackPlan, Valid: true}
	}
	if p.ImpactSummary != nil {
		params.ImpactSummary = pgtype.Text{String: *p.ImpactSummary, Valid: true}
	}
	row, err := s.queries.UpdateChange(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update change: %w", err)
	}
	return &row, nil
}

// ---------------------------------------------------------------------------
// Lifecycle.
// ---------------------------------------------------------------------------

func (s *Service) withTx(ctx context.Context, fn func(qtx *dbgen.Queries) error) error {
	if s.pool == nil {
		return errors.New("change service: pool is required for this operation")
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

// Submit transitions a change from draft → submitted. For standard and
// emergency changes (which skip CAB), it then auto-flips to `approved`
// in the same tx, so the operator sees one consistent state change
// rather than a flicker.
func (s *Service) Submit(ctx context.Context, tenantID, id, userID uuid.UUID) (*dbgen.Change, error) {
	var out dbgen.Change
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		submitted, err := qtx.SubmitChange(ctx, dbgen.SubmitChangeParams{
			ID: id, TenantID: tenantID,
		})
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
			TenantID: tenantID, ChangeID: id,
			AuthorID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:     "system",
			Body:     "submitted",
		})
		if err != nil {
			return fmt.Errorf("write submit comment: %w", err)
		}

		// Standard / emergency: auto-approve. Threshold for standard is
		// already 0; emergency may have a non-zero threshold but the
		// type's purpose is to bypass review, so we honour the type
		// over the threshold. Reasoning lives at the call site, not in
		// the SQL.
		if submitted.Type == "standard" || submitted.Type == "emergency" {
			approved, err := qtx.ApproveChangeAuto(ctx, dbgen.ApproveChangeAutoParams{
				ID: id, TenantID: tenantID,
			})
			if err != nil {
				return fmt.Errorf("auto-approve %s change: %w", submitted.Type, err)
			}
			_, err = qtx.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
				TenantID: tenantID, ChangeID: id,
				AuthorID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
				Kind:     "system",
				Body:     "auto-approved (" + submitted.Type + " change)",
			})
			if err != nil {
				return fmt.Errorf("write auto-approve comment: %w", err)
			}
			out = approved
			return nil
		}

		out = submitted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CastVote records a CAB voter's decision. After the vote is upserted,
// we compute the running tally and auto-transition the change if the
// threshold or reject condition is met. All four steps run in one tx.
//
// Rules:
//   - vote must be one of: approve, reject, abstain
//   - the change must be in `submitted` state to accept votes
//   - a single `reject` triggers immediate auto-rejection
//   - approve_count >= approval_threshold (and no reject) triggers
//     auto-approval
//   - re-voting overwrites the previous vote (UNIQUE constraint on
//     (change_id, voter_id)); the new tally is computed against the
//     updated set
func (s *Service) CastVote(ctx context.Context, tenantID, changeID, voterID uuid.UUID, vote, note string) (*dbgen.Change, error) {
	if vote != "approve" && vote != "reject" && vote != "abstain" {
		return nil, fmt.Errorf("invalid vote %q (must be approve/reject/abstain)", vote)
	}
	if voterID == uuid.Nil {
		return nil, errors.New("voter_id required")
	}
	var out dbgen.Change
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		// Pre-check: the change must be in `submitted` state. Done at
		// app level rather than SQL because we need a clear error to
		// distinguish "wrong state" (409) from "no such change" (404).
		row, err := qtx.GetChange(ctx, dbgen.GetChangeParams{ID: changeID, TenantID: tenantID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get change: %w", err)
		}
		if row.Status != "submitted" {
			return ErrInvalidStateTransition
		}

		_, err = qtx.UpsertChangeApproval(ctx, dbgen.UpsertChangeApprovalParams{
			TenantID:  tenantID,
			ChangeID:  changeID,
			VoterID:   voterID,
			Vote:      vote,
			Note:      pgtype.Text{String: note, Valid: note != ""},
		})
		if err != nil {
			return fmt.Errorf("upsert approval: %w", err)
		}
		_, err = qtx.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
			TenantID: tenantID, ChangeID: changeID,
			AuthorID: pgtype.UUID{Bytes: voterID, Valid: true},
			Kind:     "system",
			Body:     "vote: " + vote + commentTail(note),
		})
		if err != nil {
			return fmt.Errorf("write vote comment: %w", err)
		}

		// Tally the running counts. A single reject flips the change
		// straight to `rejected`. Otherwise, if approve_count meets
		// the threshold, auto-approve.
		tally, err := qtx.CountChangeApprovalsByVote(ctx, dbgen.CountChangeApprovalsByVoteParams{
			TenantID: tenantID, ChangeID: changeID,
		})
		if err != nil {
			return fmt.Errorf("count approvals: %w", err)
		}

		if tally.RejectCount > 0 {
			rejected, err := qtx.RejectChangeAuto(ctx, dbgen.RejectChangeAutoParams{
				ID: changeID, TenantID: tenantID,
			})
			if err != nil {
				return fmt.Errorf("auto-reject: %w", err)
			}
			_, err = qtx.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
				TenantID: tenantID, ChangeID: changeID,
				AuthorID: pgtype.UUID{Bytes: voterID, Valid: true},
				Kind:     "system",
				Body:     "auto-rejected (CAB rejection vote received)",
			})
			if err != nil {
				return fmt.Errorf("write auto-reject comment: %w", err)
			}
			out = rejected
			return nil
		}

		if tally.ApproveCount >= row.ApprovalThreshold {
			approved, err := qtx.ApproveChangeAuto(ctx, dbgen.ApproveChangeAutoParams{
				ID: changeID, TenantID: tenantID,
			})
			if err != nil {
				return fmt.Errorf("auto-approve: %w", err)
			}
			_, err = qtx.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
				TenantID: tenantID, ChangeID: changeID,
				AuthorID: pgtype.UUID{Bytes: voterID, Valid: true},
				Kind:     "system",
				Body:     fmt.Sprintf("auto-approved (CAB threshold %d met)", row.ApprovalThreshold),
			})
			if err != nil {
				return fmt.Errorf("write auto-approve comment: %w", err)
			}
			out = approved
			return nil
		}

		// No transition yet — return the un-mutated change so the caller
		// sees the running state.
		out = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) Start(ctx context.Context, tenantID, id, userID uuid.UUID) (*dbgen.Change, error) {
	return s.transition(ctx, tenantID, id, userID, "started", func(qtx *dbgen.Queries) (dbgen.Change, error) {
		return qtx.StartChange(ctx, dbgen.StartChangeParams{ID: id, TenantID: tenantID})
	})
}

func (s *Service) MarkSucceeded(ctx context.Context, tenantID, id, userID uuid.UUID, note string) (*dbgen.Change, error) {
	body := "succeeded"
	if note != "" {
		body = body + "\n" + note
	}
	return s.transitionWithBody(ctx, tenantID, id, userID, body, func(qtx *dbgen.Queries) (dbgen.Change, error) {
		return qtx.MarkChangeSucceeded(ctx, dbgen.MarkChangeSucceededParams{ID: id, TenantID: tenantID})
	})
}

func (s *Service) MarkFailed(ctx context.Context, tenantID, id, userID uuid.UUID, note string) (*dbgen.Change, error) {
	body := "failed"
	if note != "" {
		body = body + "\n" + note
	}
	return s.transitionWithBody(ctx, tenantID, id, userID, body, func(qtx *dbgen.Queries) (dbgen.Change, error) {
		return qtx.MarkChangeFailed(ctx, dbgen.MarkChangeFailedParams{ID: id, TenantID: tenantID})
	})
}

func (s *Service) MarkRolledBack(ctx context.Context, tenantID, id, userID uuid.UUID, note string) (*dbgen.Change, error) {
	body := "rolled back"
	if note != "" {
		body = body + "\n" + note
	}
	return s.transitionWithBody(ctx, tenantID, id, userID, body, func(qtx *dbgen.Queries) (dbgen.Change, error) {
		return qtx.MarkChangeRolledBack(ctx, dbgen.MarkChangeRolledBackParams{ID: id, TenantID: tenantID})
	})
}

// transition is a small helper for the simple state flips that just need
// "run UPDATE, write a system comment with this label". transitionWithBody
// is the variant where the caller has built its own multi-line body
// (e.g. with a free-form note). Both share the same tx + error mapping.
func (s *Service) transition(
	ctx context.Context,
	tenantID, id, userID uuid.UUID,
	commentBody string,
	op func(qtx *dbgen.Queries) (dbgen.Change, error),
) (*dbgen.Change, error) {
	return s.transitionWithBody(ctx, tenantID, id, userID, commentBody, op)
}

func (s *Service) transitionWithBody(
	ctx context.Context,
	tenantID, id, userID uuid.UUID,
	commentBody string,
	op func(qtx *dbgen.Queries) (dbgen.Change, error),
) (*dbgen.Change, error) {
	var out dbgen.Change
	err := s.withTx(ctx, func(qtx *dbgen.Queries) error {
		updated, err := op(qtx)
		if err != nil {
			return mapNoRows(err)
		}
		_, err = qtx.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
			TenantID: tenantID, ChangeID: id,
			AuthorID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
			Kind:     "system",
			Body:     commentBody,
		})
		if err != nil {
			return fmt.Errorf("write transition comment: %w", err)
		}
		out = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func commentTail(note string) string {
	if note == "" {
		return ""
	}
	return "\n" + note
}

// ---------------------------------------------------------------------------
// CAB approvals (read).
// ---------------------------------------------------------------------------

func (s *Service) ListApprovals(ctx context.Context, tenantID, changeID uuid.UUID) ([]dbgen.ListChangeApprovalsRow, error) {
	return s.queries.ListChangeApprovals(ctx, dbgen.ListChangeApprovalsParams{
		TenantID: tenantID, ChangeID: changeID,
	})
}

// ---------------------------------------------------------------------------
// Linkage.
// ---------------------------------------------------------------------------

// LinkAsset attaches an asset to a change. Verifies the asset belongs to
// the caller's tenant before inserting the link (defence in depth on top
// of the tenant_id column on change_assets).
func (s *Service) LinkAsset(ctx context.Context, tenantID, changeID, assetID uuid.UUID) error {
	if _, err := s.Get(ctx, tenantID, changeID); err != nil {
		return err
	}
	// Verify asset visibility through the assets list (we don't have a
	// direct GetAsset that respects tenant + soft-delete in queries.sql,
	// so a count via SELECT is the simplest correct check).
	var n int
	row := database.Scope(s.pool, tenantID).QueryRow(ctx,
		`SELECT count(*) FROM assets WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`,
		assetID,
	)
	if err := row.Scan(&n); err != nil {
		return fmt.Errorf("verify asset: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return s.queries.LinkChangeAsset(ctx, dbgen.LinkChangeAssetParams{
		ChangeID: changeID, AssetID: assetID, TenantID: tenantID,
	})
}

func (s *Service) UnlinkAsset(ctx context.Context, tenantID, changeID, assetID uuid.UUID) error {
	return s.queries.UnlinkChangeAsset(ctx, dbgen.UnlinkChangeAssetParams{
		ChangeID: changeID, AssetID: assetID, TenantID: tenantID,
	})
}

func (s *Service) ListAssets(ctx context.Context, tenantID, changeID uuid.UUID) ([]dbgen.Asset, error) {
	return s.queries.ListAssetsForChange(ctx, dbgen.ListAssetsForChangeParams{
		TenantID: tenantID, ChangeID: changeID,
	})
}

func (s *Service) LinkService(ctx context.Context, tenantID, changeID, serviceID uuid.UUID) error {
	if _, err := s.Get(ctx, tenantID, changeID); err != nil {
		return err
	}
	var n int
	if err := database.Scope(s.pool, tenantID).QueryRow(ctx,
		`SELECT count(*) FROM services WHERE tenant_id = $1 AND id = $2`,
		serviceID,
	).Scan(&n); err != nil {
		return fmt.Errorf("verify service: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return s.queries.LinkChangeService(ctx, dbgen.LinkChangeServiceParams{
		ChangeID: changeID, ServiceID: serviceID, TenantID: tenantID,
	})
}

func (s *Service) UnlinkService(ctx context.Context, tenantID, changeID, serviceID uuid.UUID) error {
	return s.queries.UnlinkChangeService(ctx, dbgen.UnlinkChangeServiceParams{
		ChangeID: changeID, ServiceID: serviceID, TenantID: tenantID,
	})
}

func (s *Service) ListServices(ctx context.Context, tenantID, changeID uuid.UUID) ([]dbgen.Service, error) {
	return s.queries.ListServicesForChange(ctx, dbgen.ListServicesForChangeParams{
		TenantID: tenantID, ChangeID: changeID,
	})
}

func (s *Service) LinkProblem(ctx context.Context, tenantID, changeID, problemID uuid.UUID) error {
	if _, err := s.Get(ctx, tenantID, changeID); err != nil {
		return err
	}
	if _, err := s.queries.GetProblem(ctx, dbgen.GetProblemParams{ID: problemID, TenantID: tenantID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("verify problem: %w", err)
	}
	return s.queries.LinkChangeProblem(ctx, dbgen.LinkChangeProblemParams{
		ChangeID: changeID, ProblemID: problemID, TenantID: tenantID,
	})
}

func (s *Service) UnlinkProblem(ctx context.Context, tenantID, changeID, problemID uuid.UUID) error {
	return s.queries.UnlinkChangeProblem(ctx, dbgen.UnlinkChangeProblemParams{
		ChangeID: changeID, ProblemID: problemID, TenantID: tenantID,
	})
}

func (s *Service) ListProblems(ctx context.Context, tenantID, changeID uuid.UUID) ([]dbgen.Problem, error) {
	return s.queries.ListProblemsForChange(ctx, dbgen.ListProblemsForChangeParams{
		TenantID: tenantID, ChangeID: changeID,
	})
}

func (s *Service) ListChangesForProblem(ctx context.Context, tenantID, problemID uuid.UUID) ([]dbgen.Change, error) {
	return s.queries.ListChangesForProblem(ctx, dbgen.ListChangesForProblemParams{
		TenantID: tenantID, ProblemID: problemID,
	})
}

// ---------------------------------------------------------------------------
// Comments.
// ---------------------------------------------------------------------------

func (s *Service) ListComments(ctx context.Context, tenantID, changeID uuid.UUID) ([]dbgen.ListChangeCommentsRow, error) {
	return s.queries.ListChangeComments(ctx, dbgen.ListChangeCommentsParams{
		TenantID: tenantID, ChangeID: changeID,
	})
}

func (s *Service) AddComment(ctx context.Context, tenantID, changeID, authorID uuid.UUID, body string) (*dbgen.ChangeComment, error) {
	row, err := s.queries.CreateChangeComment(ctx, dbgen.CreateChangeCommentParams{
		TenantID:  tenantID,
		ChangeID:  changeID,
		AuthorID:  pgtype.UUID{Bytes: authorID, Valid: authorID != uuid.Nil},
		Kind:      "human",
		Body:      body,
	})
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}
	return &row, nil
}
