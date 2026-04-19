package api

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/google/uuid"
)

// discoveryService is the narrow interface the api package depends on for
// discovered-asset handlers. It mirrors the subset of *discovery.Service
// that the handlers call so they can be unit-tested with a mock.
//
// It intentionally excludes Queries() — that is used by IngestDiscoveredAsset
// to do an asset-by-IP auto-match outside of the service contract; callers
// that need it read the field directly from APIServer.
type discoveryService interface {
	List(ctx context.Context, tenantID uuid.UUID, status *string, limit, offset int32) ([]dbgen.DiscoveredAsset, int64, error)
	Ingest(ctx context.Context, params dbgen.CreateDiscoveredAssetParams) (*dbgen.DiscoveredAsset, error)
	ApproveAndCreateAsset(ctx context.Context, discoveredID, tenantID, reviewerID uuid.UUID) (*discovery.ApproveResult, error)
	Ignore(ctx context.Context, id, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error)
	GetStats(ctx context.Context, tenantID uuid.UUID) (*dbgen.GetDiscoveryStatsRow, error)
	Queries() *dbgen.Queries
}
