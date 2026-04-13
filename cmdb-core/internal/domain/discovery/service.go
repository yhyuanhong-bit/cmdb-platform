package discovery

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *dbgen.Queries
}

func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// Queries returns the underlying queries for direct access (e.g. auto-match).
func (s *Service) Queries() *dbgen.Queries {
	return s.queries
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status *string, limit, offset int32) ([]dbgen.DiscoveredAsset, int64, error) {
	params := dbgen.ListDiscoveredAssetsParams{TenantID: tenantID, Limit: limit, Offset: offset}
	countParams := dbgen.CountDiscoveredAssetsParams{TenantID: tenantID}
	if status != nil {
		params.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}
	items, err := s.queries.ListDiscoveredAssets(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list discovered: %w", err)
	}
	total, err := s.queries.CountDiscoveredAssets(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count discovered: %w", err)
	}
	return items, total, nil
}

func (s *Service) Ingest(ctx context.Context, params dbgen.CreateDiscoveredAssetParams) (*dbgen.DiscoveredAsset, error) {
	item, err := s.queries.CreateDiscoveredAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create discovered: %w", err)
	}
	return &item, nil
}

func (s *Service) Approve(ctx context.Context, id, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error) {
	item, err := s.queries.ApproveDiscoveredAsset(ctx, dbgen.ApproveDiscoveredAssetParams{ID: id, ReviewedBy: pgtype.UUID{Bytes: reviewerID, Valid: true}})
	if err != nil {
		return nil, fmt.Errorf("approve discovered: %w", err)
	}
	return &item, nil
}

func (s *Service) Ignore(ctx context.Context, id, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error) {
	item, err := s.queries.IgnoreDiscoveredAsset(ctx, dbgen.IgnoreDiscoveredAssetParams{ID: id, ReviewedBy: pgtype.UUID{Bytes: reviewerID, Valid: true}})
	if err != nil {
		return nil, fmt.Errorf("ignore discovered: %w", err)
	}
	return &item, nil
}

func (s *Service) GetStats(ctx context.Context, tenantID uuid.UUID) (*dbgen.GetDiscoveryStatsRow, error) {
	row, err := s.queries.GetDiscoveryStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get discovery stats: %w", err)
	}
	return &row, nil
}
