package bia

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

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
type Service struct {
	queries *dbgen.Queries
}

// NewService creates a new BIA Service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
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

// CreateDependency inserts a new dependency and returns it.
func (s *Service) CreateDependency(ctx context.Context, params dbgen.CreateBIADependencyParams) (*dbgen.BiaDependency, error) {
	d, err := s.queries.CreateBIADependency(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create bia dependency: %w", err)
	}
	return &d, nil
}

// DeleteDependency removes a dependency by ID, scoped to the given tenant.
func (s *Service) DeleteDependency(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteBIADependency(ctx, dbgen.DeleteBIADependencyParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete bia dependency: %w", err)
	}
	return nil
}

// PropagateBIALevel propagates the MAX(tier) from BIA assessments to linked assets.
func (s *Service) PropagateBIALevel(ctx context.Context, assessmentID uuid.UUID) error {
	if err := s.queries.PropagateBIALevelByAssessment(ctx, assessmentID); err != nil {
		return fmt.Errorf("propagate BIA level: %w", err)
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
