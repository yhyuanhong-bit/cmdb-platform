// Package metricsource implements the Wave 8.1 source registry — one
// entry per logical pusher of metrics (an SNMP collector, an IPMI
// agent, a manual import script). The registry powers the "is my data
// actually flowing?" view: a freshness check that flags any source
// whose last_heartbeat_at is older than 2× expected_interval.
//
// The domain stays narrow on purpose. Quality flagging on top of
// heartbeats (stuck values, gap detection) belongs in a follow-up
// wave on the metric data itself, not on the source registry.
package metricsource

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

var (
	// ErrNotFound — row not visible to caller's tenant.
	ErrNotFound = errors.New("metric source: not found")
	// ErrDuplicateName — (tenant, name) collision; the registry
	// rejects duplicates so a typo doesn't silently create a second
	// row that gets out of sync.
	ErrDuplicateName = errors.New("metric source: a source with this name already exists in the tenant")
	// ErrValidation — invalid kind, status, or interval.
	ErrValidation = errors.New("metric source: validation failed")
)

type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

type CreateParams struct {
	TenantID                uuid.UUID
	Name                    string
	Kind                    string // snmp / ipmi / agent / pipeline / manual
	ExpectedIntervalSeconds int32
	Status                  string // active (default) / disabled
	Notes                   string
}

func (s *Service) Create(ctx context.Context, p CreateParams) (*dbgen.MetricSource, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("%w: name required", ErrValidation)
	}
	if p.ExpectedIntervalSeconds <= 0 {
		return nil, fmt.Errorf("%w: expected_interval_seconds must be > 0", ErrValidation)
	}
	if p.Kind == "" {
		return nil, fmt.Errorf("%w: kind required", ErrValidation)
	}
	status := p.Status
	if status == "" {
		status = "active"
	}
	row, err := s.queries.CreateMetricSource(ctx, dbgen.CreateMetricSourceParams{
		TenantID:                p.TenantID,
		Name:                    p.Name,
		Kind:                    p.Kind,
		ExpectedIntervalSeconds: p.ExpectedIntervalSeconds,
		Status:                  status,
		Notes:                   pgtype.Text{String: p.Notes, Valid: p.Notes != ""},
	})
	if err != nil {
		// Postgres unique violation (23505) for the (tenant, name)
		// constraint. We don't import pq here — string-match the
		// constraint name, which is stable and explicit in the
		// migration.
		if isUniqueViolation(err, "uq_metric_sources_tenant_name") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("create metric source: %w", err)
	}
	return &row, nil
}

type UpdateParams struct {
	TenantID                uuid.UUID
	ID                      uuid.UUID
	Name                    *string
	Kind                    *string
	ExpectedIntervalSeconds *int32
	Status                  *string
	Notes                   *string
}

func (s *Service) Update(ctx context.Context, p UpdateParams) (*dbgen.MetricSource, error) {
	params := dbgen.UpdateMetricSourceParams{ID: p.ID, TenantID: p.TenantID}
	if p.Name != nil {
		if *p.Name == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		params.Name = pgtype.Text{String: *p.Name, Valid: true}
	}
	if p.Kind != nil {
		params.Kind = pgtype.Text{String: *p.Kind, Valid: true}
	}
	if p.ExpectedIntervalSeconds != nil {
		if *p.ExpectedIntervalSeconds <= 0 {
			return nil, fmt.Errorf("%w: expected_interval_seconds must be > 0", ErrValidation)
		}
		params.ExpectedIntervalSeconds = pgtype.Int4{Int32: *p.ExpectedIntervalSeconds, Valid: true}
	}
	if p.Status != nil {
		params.Status = pgtype.Text{String: *p.Status, Valid: true}
	}
	if p.Notes != nil {
		params.Notes = pgtype.Text{String: *p.Notes, Valid: true}
	}
	row, err := s.queries.UpdateMetricSource(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err, "uq_metric_sources_tenant_name") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update metric source: %w", err)
	}
	return &row, nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.MetricSource, error) {
	row, err := s.queries.GetMetricSource(ctx, dbgen.GetMetricSourceParams{ID: id, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get metric source: %w", err)
	}
	return &row, nil
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status, kind *string) ([]dbgen.MetricSource, error) {
	params := dbgen.ListMetricSourcesParams{TenantID: tenantID}
	if status != nil && *status != "" {
		params.Status = pgtype.Text{String: *status, Valid: true}
	}
	if kind != nil && *kind != "" {
		params.Kind = pgtype.Text{String: *kind, Valid: true}
	}
	return s.queries.ListMetricSources(ctx, params)
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteMetricSource(ctx, dbgen.DeleteMetricSourceParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete metric source: %w", err)
	}
	return nil
}

// Heartbeat is the entry point external agents (or the ingestion
// endpoint relaying their data) call to bump last_heartbeat_at and
// add to the lifetime sample counter. sampleDelta is the count of
// metric rows this call wrote — pass 0 for a "I'm still alive" ping
// that doesn't carry data.
func (s *Service) Heartbeat(ctx context.Context, tenantID, id uuid.UUID, sampleDelta int64) (*dbgen.MetricSource, error) {
	row, err := s.queries.HeartbeatMetricSource(ctx, dbgen.HeartbeatMetricSourceParams{
		ID:          id,
		TenantID:    tenantID,
		SampleDelta: sampleDelta,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("heartbeat: %w", err)
	}
	return &row, nil
}

// StaleSource is one row of the freshness report. SecondsSinceHeartbeat
// is null when the source has never heartbeated.
type StaleSource = dbgen.ListStaleMetricSourcesRow

// ListStale returns sources whose last_heartbeat_at is older than 2×
// their expected_interval (or have never heartbeated). Disabled
// sources are excluded.
func (s *Service) ListStale(ctx context.Context, tenantID uuid.UUID) ([]StaleSource, error) {
	return s.queries.ListStaleMetricSources(ctx, tenantID)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// isUniqueViolation does a string match on the error against the named
// constraint. Avoids dragging in pq just for an error code check.
func isUniqueViolation(err error, constraintName string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAll(msg, "duplicate key", constraintName) ||
		containsAll(msg, "unique constraint", constraintName)
}

func containsAll(haystack string, needles ...string) bool {
	for _, n := range needles {
		if !contains(haystack, n) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	// Naive; the substrings we look for are short and the error messages
	// are short. strings.Contains would be the obvious move, but keeping
	// this package import-light avoids making it depend on an extra
	// stdlib in callers that vendor.
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
