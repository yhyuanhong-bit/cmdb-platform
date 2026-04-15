package topology

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Service provides topology operations on locations and racks.
type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

// NewService creates a new topology service.
func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// ListRootLocations returns top-level locations for a tenant.
func (s *Service) ListRootLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error) {
	return s.queries.ListRootLocations(ctx, tenantID)
}

// ListAllLocations returns all locations for a tenant (flat list ordered by path).
func (s *Service) ListAllLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error) {
	return s.queries.ListAllLocations(ctx, tenantID)
}

// GetLocation returns a single location by ID, scoped to the given tenant.
func (s *Service) GetLocation(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Location, error) {
	return s.queries.GetLocation(ctx, dbgen.GetLocationParams{ID: id, TenantID: tenantID})
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
func (s *Service) GetLocationStats(ctx context.Context, tenantID, locationID uuid.UUID) (LocationStats, error) {
	loc, err := s.queries.GetLocation(ctx, dbgen.GetLocationParams{ID: locationID, TenantID: tenantID})
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

	// Compute average rack occupancy: used U positions / total U capacity.
	var avgOccupancy float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(
			CASE WHEN r.total_u > 0
			THEN (SELECT COUNT(*) FROM rack_slots rs WHERE rs.rack_id = r.id)::float / r.total_u * 100
			ELSE 0 END
		), 0)
		FROM racks r
		JOIN locations l ON r.location_id = l.id
		WHERE r.tenant_id = $1
		  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree
	`, loc.TenantID, locationID).Scan(&avgOccupancy)
	if err != nil {
		return LocationStats{}, fmt.Errorf("computing rack occupancy: %w", err)
	}

	return LocationStats{
		TotalAssets:    totalAssets,
		TotalRacks:     totalRacks,
		CriticalAlerts: criticalAlerts,
		AvgOccupancy:   avgOccupancy,
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

// ListRacksByLocation returns all racks at a location, scoped by tenant via ltree.
func (s *Service) ListRacksByLocation(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.Rack, error) {
	return s.queries.ListRacksByLocation(ctx, dbgen.ListRacksByLocationParams{
		TenantID: tenantID,
		ID:       locationID,
	})
}

// GetRack returns a single rack by ID, scoped to the given tenant.
func (s *Service) GetRack(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Rack, error) {
	return s.queries.GetRack(ctx, dbgen.GetRackParams{ID: id, TenantID: tenantID})
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
	s.incrementSyncVersion(ctx, "locations", loc.ID)
	return &loc, nil
}

// UpdateLocation updates an existing location.
func (s *Service) UpdateLocation(ctx context.Context, params dbgen.UpdateLocationParams) (*dbgen.Location, error) {
	loc, err := s.queries.UpdateLocation(ctx, params)
	if err != nil {
		return nil, err
	}
	s.incrementSyncVersion(ctx, "locations", loc.ID)
	return &loc, nil
}

// DeleteLocation removes a location by ID, scoped to the given tenant.
func (s *Service) DeleteLocation(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteLocation(ctx, dbgen.DeleteLocationParams{ID: id, TenantID: tenantID}); err != nil {
		return err
	}
	s.incrementSyncVersion(ctx, "locations", id)
	return nil
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
	s.incrementSyncVersion(ctx, "racks", rack.ID)
	return &rack, nil
}

// UpdateRack updates an existing rack.
func (s *Service) UpdateRack(ctx context.Context, params dbgen.UpdateRackParams) (*dbgen.Rack, error) {
	rack, err := s.queries.UpdateRack(ctx, params)
	if err != nil {
		return nil, err
	}
	s.incrementSyncVersion(ctx, "racks", rack.ID)
	return &rack, nil
}

// DeleteRack removes a rack by ID, scoped to the given tenant.
func (s *Service) DeleteRack(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteRack(ctx, dbgen.DeleteRackParams{ID: id, TenantID: tenantID}); err != nil {
		return err
	}
	s.incrementSyncVersion(ctx, "racks", id)
	return nil
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
// Also increments rack sync_version (#6) and sets assets.rack_id (#20).
func (s *Service) CreateRackSlot(ctx context.Context, params dbgen.CreateRackSlotParams) (*dbgen.RackSlot, error) {
	slot, err := s.queries.CreateRackSlot(ctx, params)
	if err != nil {
		return nil, err
	}
	// Fix #6: increment sync_version on the parent rack
	s.incrementSyncVersion(ctx, "racks", params.RackID)
	// Fix #20: synchronize assets.rack_id with rack_slots
	if s.pool != nil {
		if _, err := s.pool.Exec(ctx, "UPDATE assets SET rack_id = $1, updated_at = now() WHERE id = $2 AND (rack_id IS NULL OR rack_id != $1)", pgtype.UUID{Bytes: params.RackID, Valid: true}, params.AssetID); err != nil {
			zap.L().Error("topology: failed to sync asset rack_id on slot create", zap.Error(err))
		}
	}
	return &slot, nil
}

// DeleteRackSlot removes a rack slot assignment by ID, scoped to the given tenant.
// Also increments rack sync_version (#6) and clears assets.rack_id if no other slots link it (#20).
func (s *Service) DeleteRackSlot(ctx context.Context, tenantID, slotID uuid.UUID) error {
	// Capture slot info before deleting so we can update the asset's rack_id
	var rackID, assetID uuid.UUID
	if s.pool != nil {
		if err := s.pool.QueryRow(ctx, "SELECT rack_id, asset_id FROM rack_slots WHERE id = $1", slotID).Scan(&rackID, &assetID); err != nil {
			zap.L().Error("topology: failed to read slot before delete", zap.Error(err))
		}
	}

	if err := s.queries.DeleteRackSlot(ctx, dbgen.DeleteRackSlotParams{ID: slotID, TenantID: tenantID}); err != nil {
		return err
	}

	// Fix #6: increment sync_version on the parent rack
	if rackID != uuid.Nil {
		s.incrementSyncVersion(ctx, "racks", rackID)
	}

	// Fix #20: clear assets.rack_id if no other rack_slots link this asset to this rack
	if s.pool != nil && assetID != uuid.Nil && rackID != uuid.Nil {
		var remaining int64
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM rack_slots WHERE rack_id = $1 AND asset_id = $2", rackID, assetID).Scan(&remaining); err != nil {
			zap.L().Error("topology: failed to count remaining slots", zap.Error(err))
		}
		if remaining == 0 {
			if _, err := s.pool.Exec(ctx, "UPDATE assets SET rack_id = NULL, updated_at = now() WHERE id = $1 AND rack_id = $2", assetID, pgtype.UUID{Bytes: rackID, Valid: true}); err != nil {
				zap.L().Error("topology: failed to clear asset rack_id on slot delete", zap.Error(err))
			}
		}
	}
	return nil
}

// GetRackOccupancy returns the total_u and used_u for a single rack.
func (s *Service) GetRackOccupancy(ctx context.Context, rackID uuid.UUID) (dbgen.GetRackOccupancyRow, error) {
	return s.queries.GetRackOccupancy(ctx, rackID)
}

// GetRackOccupanciesByLocation returns used_u for all racks under a location (batch, avoids N+1).
func (s *Service) GetRackOccupanciesByLocation(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.GetRackOccupanciesByLocationRow, error) {
	return s.queries.GetRackOccupanciesByLocation(ctx, dbgen.GetRackOccupanciesByLocationParams{
		TenantID: tenantID,
		ID:       locationID,
	})
}

func (s *Service) incrementSyncVersion(ctx context.Context, table string, id uuid.UUID) {
	if s.pool == nil {
		return
	}
	if _, err := s.pool.Exec(ctx, fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", table), id); err != nil {
		zap.L().Error("topology: failed to increment sync_version", zap.String("table", table), zap.Error(err))
	}
}
