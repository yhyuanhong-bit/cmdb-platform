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

// GetLocationStats computes aggregate statistics for a location
// including ALL descendant locations (recursive via ltree).
func (s *Service) GetLocationStats(ctx context.Context, locationID uuid.UUID) (LocationStats, error) {
	loc, err := s.queries.GetLocation(ctx, locationID)
	if err != nil {
		return LocationStats{}, err
	}

	// Count assets under this location and ALL descendants.
	totalAssets, err := s.queries.CountAssetsUnderLocation(ctx, dbgen.CountAssetsUnderLocationParams{
		TenantID: loc.TenantID,
		ID:       locationID,
	})
	if err != nil {
		return LocationStats{}, err
	}

	// Count racks under this location and ALL descendants.
	totalRacks, err := s.queries.CountRacksUnderLocation(ctx, dbgen.CountRacksUnderLocationParams{
		TenantID: loc.TenantID,
		ID:       locationID,
	})
	if err != nil {
		return LocationStats{}, err
	}

	// Count firing alerts for assets under this location tree.
	criticalAlerts, err := s.queries.CountAlertsUnderLocation(ctx, dbgen.CountAlertsUnderLocationParams{
		TenantID: loc.TenantID,
		ID:       locationID,
	})
	if err != nil {
		return LocationStats{}, err
	}

	return LocationStats{
		TotalAssets:    totalAssets,
		TotalRacks:     totalRacks,
		CriticalAlerts: criticalAlerts,
		AvgOccupancy:   0, // TODO: compute when rack_slots data is populated
	}, nil
}

// GetBySlug looks up a location by its slug and level for a given tenant.
func (s *Service) GetBySlug(ctx context.Context, tenantID uuid.UUID, slug, level string) (*dbgen.Location, error) {
	loc, err := s.queries.GetLocationBySlug(ctx, dbgen.GetLocationBySlugParams{
		TenantID: tenantID,
		Slug:     slug,
		Level:    level,
	})
	if err != nil {
		return nil, err
	}
	return &loc, nil
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

// CreateLocation inserts a new location.
func (s *Service) CreateLocation(ctx context.Context, params dbgen.CreateLocationParams) (*dbgen.Location, error) {
	loc, err := s.queries.CreateLocation(ctx, params)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

// UpdateLocation updates an existing location.
func (s *Service) UpdateLocation(ctx context.Context, params dbgen.UpdateLocationParams) (*dbgen.Location, error) {
	loc, err := s.queries.UpdateLocation(ctx, params)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

// DeleteLocation removes a location by ID.
func (s *Service) DeleteLocation(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteLocation(ctx, id)
}

// ListDescendants returns all descendant locations under a path.
func (s *Service) ListDescendants(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error) {
	return s.queries.ListDescendants(ctx, dbgen.ListDescendantsParams{
		TenantID: tenantID,
		Column2:  path,
	})
}

// CreateRack inserts a new rack.
func (s *Service) CreateRack(ctx context.Context, params dbgen.CreateRackParams) (*dbgen.Rack, error) {
	rack, err := s.queries.CreateRack(ctx, params)
	if err != nil {
		return nil, err
	}
	return &rack, nil
}

// UpdateRack updates an existing rack.
func (s *Service) UpdateRack(ctx context.Context, params dbgen.UpdateRackParams) (*dbgen.Rack, error) {
	rack, err := s.queries.UpdateRack(ctx, params)
	if err != nil {
		return nil, err
	}
	return &rack, nil
}

// DeleteRack removes a rack by ID.
func (s *Service) DeleteRack(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteRack(ctx, id)
}

// ListRackSlots returns all slot assignments for a rack.
func (s *Service) ListRackSlots(ctx context.Context, rackID uuid.UUID) ([]dbgen.ListRackSlotsRow, error) {
	return s.queries.ListRackSlots(ctx, rackID)
}

// CheckSlotConflict checks if a U-position range conflicts with existing slots.
func (s *Service) CheckSlotConflict(ctx context.Context, rackID uuid.UUID, side string, startU, endU int32) (int64, error) {
	return s.queries.CheckSlotConflict(ctx, dbgen.CheckSlotConflictParams{
		RackID: rackID,
		Side:   side,
		EndU:   startU,
		StartU: endU,
	})
}

// CreateRackSlot inserts a new rack slot assignment.
func (s *Service) CreateRackSlot(ctx context.Context, params dbgen.CreateRackSlotParams) (*dbgen.RackSlot, error) {
	slot, err := s.queries.CreateRackSlot(ctx, params)
	if err != nil {
		return nil, err
	}
	return &slot, nil
}

// DeleteRackSlot removes a rack slot assignment by ID.
func (s *Service) DeleteRackSlot(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteRackSlot(ctx, id)
}
