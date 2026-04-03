package topology

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides topology operations on locations and racks.
type Service struct {
	queries *dbgen.Queries
}

// NewService creates a new topology service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// ListRootLocations returns top-level locations for a tenant.
func (s *Service) ListRootLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error) {
	return s.queries.ListRootLocations(ctx, tenantID)
}

// GetLocation returns a single location by ID.
func (s *Service) GetLocation(ctx context.Context, id uuid.UUID) (dbgen.Location, error) {
	return s.queries.GetLocation(ctx, id)
}

// ListChildren returns direct children of a location.
func (s *Service) ListChildren(ctx context.Context, parentID uuid.UUID) ([]dbgen.Location, error) {
	return s.queries.ListChildren(ctx, pgtype.UUID{Bytes: parentID, Valid: true})
}

// ListAncestors returns all ancestor locations along the path.
func (s *Service) ListAncestors(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error) {
	return s.queries.ListAncestors(ctx, dbgen.ListAncestorsParams{
		TenantID: tenantID,
		Column2:  path,
	})
}

// GetLocationStats computes aggregate statistics for a location.
func (s *Service) GetLocationStats(ctx context.Context, locationID uuid.UUID) (LocationStats, error) {
	loc, err := s.queries.GetLocation(ctx, locationID)
	if err != nil {
		return LocationStats{}, err
	}

	// Count assets at this location.
	totalAssets, err := s.queries.CountAssets(ctx, dbgen.CountAssetsParams{
		TenantID:   loc.TenantID,
		LocationID: pgtype.UUID{Bytes: locationID, Valid: true},
	})
	if err != nil {
		return LocationStats{}, err
	}

	// Count racks at this location.
	racks, err := s.queries.ListRacksByLocation(ctx, locationID)
	if err != nil {
		return LocationStats{}, err
	}

	// Count firing alerts (critical severity) for this tenant.
	criticalAlerts, err := s.queries.CountAlerts(ctx, dbgen.CountAlertsParams{
		TenantID: loc.TenantID,
		Status:   pgtype.Text{String: "firing", Valid: true},
		Severity: pgtype.Text{String: "critical", Valid: true},
	})
	if err != nil {
		return LocationStats{}, err
	}

	// Compute average rack occupancy.
	var avgOccupancy float64
	if len(racks) > 0 {
		var totalPct float64
		for _, r := range racks {
			occ, err := s.queries.GetRackOccupancy(ctx, r.ID)
			if err != nil {
				continue
			}
			if occ.TotalU > 0 {
				totalPct += float64(occ.UsedU) / float64(occ.TotalU)
			}
		}
		avgOccupancy = totalPct / float64(len(racks))
	}

	return LocationStats{
		TotalAssets:    totalAssets,
		TotalRacks:     int64(len(racks)),
		CriticalAlerts: criticalAlerts,
		AvgOccupancy:   avgOccupancy,
	}, nil
}

// ListRacksByLocation returns all racks at a location.
func (s *Service) ListRacksByLocation(ctx context.Context, locationID uuid.UUID) ([]dbgen.Rack, error) {
	return s.queries.ListRacksByLocation(ctx, locationID)
}

// GetRack returns a single rack by ID.
func (s *Service) GetRack(ctx context.Context, id uuid.UUID) (dbgen.Rack, error) {
	return s.queries.GetRack(ctx, id)
}

// ListAssetsByRack returns all assets mounted in a rack.
func (s *Service) ListAssetsByRack(ctx context.Context, rackID uuid.UUID) ([]dbgen.Asset, error) {
	return s.queries.ListAssetsByRack(ctx, pgtype.UUID{Bytes: rackID, Valid: true})
}
