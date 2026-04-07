package dashboard

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Stats holds the aggregated dashboard statistics.
type Stats struct {
	TotalAssets    int64 `json:"total_assets"`
	TotalRacks     int64 `json:"total_racks"`
	CriticalAlerts int64 `json:"critical_alerts"`
	ActiveOrders   int64 `json:"active_orders"`
}

// Service provides dashboard aggregation operations.
type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

// NewService creates a new dashboard Service.
func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// GetStats aggregates key metrics for the dashboard.
func (s *Service) GetStats(ctx context.Context, tenantID uuid.UUID) (*Stats, error) {
	totalAssets, err := s.queries.CountAssets(ctx, dbgen.CountAssetsParams{
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("count assets: %w", err)
	}

	criticalAlerts, err := s.queries.CountAlerts(ctx, dbgen.CountAlertsParams{
		TenantID: tenantID,
		Status:   pgtype.Text{String: "firing", Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("count alerts: %w", err)
	}

	activeOrders, err := s.queries.CountWorkOrders(ctx, dbgen.CountWorkOrdersParams{
		TenantID: tenantID,
		Status:   pgtype.Text{String: "in_progress", Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("count work orders: %w", err)
	}

	var totalRacks int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM racks WHERE tenant_id = $1`, tenantID).Scan(&totalRacks); err != nil {
		return nil, fmt.Errorf("count racks: %w", err)
	}

	return &Stats{
		TotalAssets:    totalAssets,
		TotalRacks:     totalRacks,
		CriticalAlerts: criticalAlerts,
		ActiveOrders:   activeOrders,
	}, nil
}
