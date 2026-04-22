package bia

import (
	"context"
	"errors"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a BIA record cannot be resolved for the given
// tenant. Handlers translate this into a 404.
var ErrNotFound = errors.New("bia: not found")

// BIAStats holds aggregated BIA statistics.
type BIAStats struct {
	Total          int64            `json:"total"`
	ByTier         map[string]int64 `json:"by_tier"`
	AvgCompliance  float64          `json:"avg_compliance"`
	DataCompliant  int64            `json:"data_compliant"`
	AssetCompliant int64            `json:"asset_compliant"`
	AuditCompliant int64            `json:"audit_compliant"`
}

// Service provides BIA domain operations.
//
// pool is optional; when nil, dependency create/delete fall back to a
// non-transactional path (used by older tests). Production construction
// should always pass the pool so propagation is atomic with the write.
type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

// NewService creates a new BIA Service. pool may be nil — see Service docs.
func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// ListAssessments returns a paginated list of BIA assessments and total count.
func (s *Service) ListAssessments(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.BiaAssessment, int64, error) {
	assessments, err := s.queries.ListBIAAssessments(ctx, dbgen.ListBIAAssessmentsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list bia assessments: %w", err)
	}

	total, err := s.queries.CountBIAAssessments(ctx, tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("count bia assessments: %w", err)
	}

	return assessments, total, nil
}

// GetAssessment returns a single BIA assessment by ID, scoped to the given tenant.
func (s *Service) GetAssessment(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.BiaAssessment, error) {
	a, err := s.queries.GetBIAAssessment(ctx, dbgen.GetBIAAssessmentParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get bia assessment: %w", err)
	}
	return &a, nil
}

// CreateAssessment inserts a new BIA assessment and returns it.
func (s *Service) CreateAssessment(ctx context.Context, params dbgen.CreateBIAAssessmentParams) (*dbgen.BiaAssessment, error) {
	a, err := s.queries.CreateBIAAssessment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create bia assessment: %w", err)
	}
	return &a, nil
}

// UpdateAssessment modifies an existing BIA assessment and returns it.
func (s *Service) UpdateAssessment(ctx context.Context, params dbgen.UpdateBIAAssessmentParams) (*dbgen.BiaAssessment, error) {
	a, err := s.queries.UpdateBIAAssessment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update bia assessment: %w", err)
	}
	return &a, nil
}

// DeleteAssessment removes a BIA assessment by ID, scoped to the given tenant.
func (s *Service) DeleteAssessment(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteBIAAssessment(ctx, dbgen.DeleteBIAAssessmentParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete bia assessment: %w", err)
	}
	return nil
}

// ListRules returns all scoring rules for a tenant.
func (s *Service) ListRules(ctx context.Context, tenantID uuid.UUID) ([]dbgen.BiaScoringRule, error) {
	rules, err := s.queries.ListBIAScoringRules(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list bia scoring rules: %w", err)
	}
	return rules, nil
}

// UpdateRule modifies an existing scoring rule and returns it.
func (s *Service) UpdateRule(ctx context.Context, params dbgen.UpdateBIAScoringRuleParams) (*dbgen.BiaScoringRule, error) {
	r, err := s.queries.UpdateBIAScoringRule(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update bia scoring rule: %w", err)
	}
	return &r, nil
}

// ListDependencies returns all dependencies for an assessment.
func (s *Service) ListDependencies(ctx context.Context, assessmentID uuid.UUID) ([]dbgen.BiaDependency, error) {
	deps, err := s.queries.ListBIADependencies(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("list bia dependencies: %w", err)
	}
	return deps, nil
}

// CreateDependency inserts a new dependency AND atomically propagates the
// assessment's tier to every asset currently linked to that assessment.
//
// Tenant guard: params.TenantID must match the assessment's tenant. If not,
// ErrNotFound is returned (404-equivalent) and nothing is written.
//
// Atomicity: the insert + propagation run in a single pgx transaction so a
// propagation failure rolls back the dependency insert. Inline (not async)
// because most tenants have < 1000 linked assets; if that changes, swap the
// propagation call for an eventbus publish after commit.
func (s *Service) CreateDependency(ctx context.Context, params dbgen.CreateBIADependencyParams) (*dbgen.BiaDependency, error) {
	// Fallback path: no pool wired (unit tests using an in-memory Queries stub).
	// Skip propagation — callers in that mode don't exercise it.
	if s.pool == nil {
		if err := s.verifyAssessmentTenant(ctx, s.queries, params.AssessmentID, params.TenantID); err != nil {
			return nil, err
		}
		d, err := s.queries.CreateBIADependency(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("create bia dependency: %w", err)
		}
		return &d, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	if verr := s.verifyAssessmentTenant(ctx, qtx, params.AssessmentID, params.TenantID); verr != nil {
		return nil, verr
	}

	created, err := qtx.CreateBIADependency(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create bia dependency: %w", err)
	}

	if err := qtx.PropagateBIALevelByAssessment(ctx, dbgen.PropagateBIALevelByAssessmentParams{
		AssessmentID: params.AssessmentID,
		TenantID:     params.TenantID,
	}); err != nil {
		return nil, fmt.Errorf("propagate bia level: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &created, nil
}

// DeleteDependency removes a dependency by ID AND atomically recomputes the
// bia_level of the asset that dependency pointed at. If no other dependencies
// remain for that asset, bia_level falls back to the schema default ('normal').
//
// Tenant guard: the dependency must belong to tenantID. If not, ErrNotFound
// is returned and nothing is written.
func (s *Service) DeleteDependency(ctx context.Context, tenantID, id uuid.UUID) error {
	if s.pool == nil {
		// Fallback — no propagation when pool is absent.
		if err := s.queries.DeleteBIADependency(ctx, dbgen.DeleteBIADependencyParams{ID: id, TenantID: tenantID}); err != nil {
			return fmt.Errorf("delete bia dependency: %w", err)
		}
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	// Load the dependency first so we can both enforce tenant ownership and
	// capture the asset_id for post-delete recomputation.
	dep, err := qtx.GetBIADependency(ctx, dbgen.GetBIADependencyParams{ID: id, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load bia dependency: %w", err)
	}

	if err := qtx.DeleteBIADependency(ctx, dbgen.DeleteBIADependencyParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete bia dependency: %w", err)
	}

	if err := qtx.RecomputeBIALevelForAsset(ctx, dbgen.RecomputeBIALevelForAssetParams{
		AssetID:  dep.AssetID,
		TenantID: tenantID,
	}); err != nil {
		return fmt.Errorf("recompute bia level: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// PropagateBIALevel propagates the MAX(tier) from BIA assessments to linked assets.
// Tenant-scoped: only assessments + dependencies + assets in tenantID are touched.
func (s *Service) PropagateBIALevel(ctx context.Context, tenantID, assessmentID uuid.UUID) error {
	if err := s.queries.PropagateBIALevelByAssessment(ctx, dbgen.PropagateBIALevelByAssessmentParams{
		AssessmentID: assessmentID,
		TenantID:     tenantID,
	}); err != nil {
		return fmt.Errorf("propagate BIA level: %w", err)
	}
	return nil
}

// verifyAssessmentTenant enforces that the given assessment belongs to the
// given tenant. Returns ErrNotFound on mismatch or missing row so the caller
// can translate to 404 without leaking cross-tenant existence.
func (s *Service) verifyAssessmentTenant(ctx context.Context, q *dbgen.Queries, assessmentID, tenantID uuid.UUID) error {
	ownerTenant, err := q.GetBIAAssessmentTenant(ctx, assessmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("verify assessment tenant: %w", err)
	}
	if ownerTenant != tenantID {
		return ErrNotFound
	}
	return nil
}

// GetImpactedAssessments returns BIA assessments that depend on a given asset.
func (s *Service) GetImpactedAssessments(ctx context.Context, assetID uuid.UUID) ([]dbgen.BiaAssessment, error) {
	return s.queries.GetImpactedAssessments(ctx, assetID)
}

// GetStats returns aggregated BIA statistics for a tenant.
func (s *Service) GetStats(ctx context.Context, tenantID uuid.UUID) (*BIAStats, error) {
	tierCounts, err := s.queries.CountBIAByTier(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("count bia by tier: %w", err)
	}

	compliance, err := s.queries.GetBIAComplianceStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get bia compliance stats: %w", err)
	}

	byTier := make(map[string]int64)
	for _, tc := range tierCounts {
		byTier[tc.Tier] = tc.Count
	}

	var avgCompliance float64
	if compliance.Total > 0 {
		totalChecks := compliance.Total * 3 // 3 compliance fields
		passedChecks := compliance.DataCompliant + compliance.AssetCompliant + compliance.AuditCompliant
		avgCompliance = float64(passedChecks) / float64(totalChecks) * 100
	}

	return &BIAStats{
		Total:          compliance.Total,
		ByTier:         byTier,
		AvgCompliance:  avgCompliance,
		DataCompliant:  compliance.DataCompliant,
		AssetCompliant: compliance.AssetCompliant,
		AuditCompliant: compliance.AuditCompliant,
	}, nil
}
