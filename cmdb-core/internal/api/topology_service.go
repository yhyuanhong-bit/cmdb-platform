package api

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/google/uuid"
)

// topologyService is the narrow interface the api package depends on for
// location/rack/dependency operations. It mirrors the subset of
// *topology.Service that handlers actually call so they can be
// unit-tested with a mock.
type topologyService interface {
	// Location reads
	ListRootLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error)
	ListAllLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error)
	GetLocation(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Location, error)
	GetBySlug(ctx context.Context, tenantID uuid.UUID, slug, level string) (*dbgen.Location, error)
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]dbgen.Location, error)
	ListAncestors(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error)
	ListDescendants(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error)
	GetLocationStats(ctx context.Context, tenantID, locationID uuid.UUID) (topology.LocationStats, error)

	// Location writes
	CreateLocation(ctx context.Context, params dbgen.CreateLocationParams) (*dbgen.Location, error)
	UpdateLocation(ctx context.Context, params dbgen.UpdateLocationParams) (*dbgen.Location, error)
	PreflightDeleteLocation(ctx context.Context, tenantID, id uuid.UUID) (*topology.LocationDeleteInfo, error)
	DeleteLocation(ctx context.Context, tenantID, id uuid.UUID, recursive bool) error

	// Rack reads
	ListRacksByLocation(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.Rack, error)
	GetRack(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Rack, error)
	ListAssetsByRack(ctx context.Context, rackID uuid.UUID) ([]dbgen.Asset, error)
	GetRackOccupancy(ctx context.Context, rackID uuid.UUID) (dbgen.GetRackOccupancyRow, error)
	GetRackOccupanciesByLocation(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.GetRackOccupanciesByLocationRow, error)

	// Rack writes
	CreateRack(ctx context.Context, params dbgen.CreateRackParams) (*dbgen.Rack, error)
	UpdateRack(ctx context.Context, params dbgen.UpdateRackParams) (*dbgen.Rack, error)
	DeleteRack(ctx context.Context, tenantID, id uuid.UUID) error

	// Rack slots
	ListRackSlots(ctx context.Context, rackID uuid.UUID) ([]dbgen.ListRackSlotsRow, error)
	CheckSlotConflict(ctx context.Context, rackID uuid.UUID, side string, startU, endU int32) (int64, error)
	CreateRackSlot(ctx context.Context, params dbgen.CreateRackSlotParams) (*dbgen.RackSlot, error)
	DeleteRackSlot(ctx context.Context, tenantID, slotID uuid.UUID) error

	// Dependencies / impact analysis
	GetImpactPath(ctx context.Context, tenantID, rootAssetID uuid.UUID, maxDepth int, direction topology.ImpactDirection) ([]topology.ImpactEdge, error)
	GetImpactPathAt(ctx context.Context, tenantID, rootAssetID uuid.UUID, maxDepth int, direction topology.ImpactDirection, atTime *time.Time) ([]topology.ImpactEdge, error)
	CreateDependency(ctx context.Context, p topology.CreateDependencyParams) error
}
