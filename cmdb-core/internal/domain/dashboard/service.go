package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Stats holds the aggregated dashboard statistics.
//
// Fields after ActiveOrders are populated by independent best-effort
// queries: each has its own 500ms budget and degrades to zero on
// timeout or query error, so one slow metric cannot fail the whole
// response. See GetStats for the partial-tolerance contract.
type Stats struct {
	TotalAssets       int64   `json:"total_assets"`
	TotalRacks        int64   `json:"total_racks"`
	CriticalAlerts    int64   `json:"critical_alerts"`
	ActiveOrders      int64   `json:"active_orders"`
	PendingWorkOrders int64   `json:"pending_work_orders"`
	EnergyCurrentKW   float64 `json:"energy_current_kw"`
	RackUtilizationPct float64 `json:"rack_utilization_pct"`
	AvgQualityScore   float64 `json:"avg_quality_score"`
}

// statsCacheTTL is the duration dashboard stats are cached in Redis.
const statsCacheTTL = 60 * time.Second

// fieldTimeout bounds each partial-tolerant field query. A single slow
// query (e.g. a cold metrics chunk forcing a seq-scan across millions
// of rows) must not block the whole dashboard response; the field
// degrades to zero and the UI still renders the other tiles.
const fieldTimeout = 500 * time.Millisecond

// Service provides dashboard aggregation operations.
type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
	redis   *redis.Client
}

// NewService creates a new dashboard Service.
func NewService(queries *dbgen.Queries, pool *pgxpool.Pool, rc *redis.Client) *Service {
	return &Service{queries: queries, pool: pool, redis: rc}
}

// GetStats aggregates key metrics for the dashboard.
// Results are cached in Redis for statsCacheTTL to avoid repeated count queries.
func (s *Service) GetStats(ctx context.Context, tenantID uuid.UUID) (*Stats, error) {
	cacheKey := fmt.Sprintf("dashboard:stats:%s", tenantID.String())

	// Try cache first (best-effort — skip on any error).
	if s.redis != nil {
		if val, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
			var cached Stats
			if json.Unmarshal([]byte(val), &cached) == nil {
				return &cached, nil
			}
		}
	}

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

	pendingWorkOrders, err := s.queries.CountPendingWorkOrders(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("count pending work orders: %w", err)
	}

	var totalRacks int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM racks WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&totalRacks); err != nil {
		return nil, fmt.Errorf("count racks: %w", err)
	}

	stats := &Stats{
		TotalAssets:       totalAssets,
		TotalRacks:        totalRacks,
		CriticalAlerts:    criticalAlerts,
		ActiveOrders:      activeOrders,
		PendingWorkOrders: pendingWorkOrders,
		AvgQualityScore:   s.avgQualityScore(ctx, tenantID),
	}

	// Write-through cache (best-effort).
	if s.redis != nil {
		if data, err := json.Marshal(stats); err == nil {
			_ = s.redis.Set(ctx, cacheKey, string(data), statsCacheTTL).Err()
		}
	}

	return stats, nil
}

// avgQualityScore returns the mean of each asset's most recent
// total_score. Zero on any error or timeout — the caller treats this
// as a best-effort tile and cannot fail the whole response.
func (s *Service) avgQualityScore(parent context.Context, tenantID uuid.UUID) float64 {
	ctx, cancel := context.WithTimeout(parent, fieldTimeout)
	defer cancel()
	v, err := s.queries.AvgLatestQualityScore(ctx, tenantID)
	if err != nil {
		return 0
	}
	return v
}
