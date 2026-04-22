package api

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/google/uuid"
)

// assetService is the narrow interface the api package depends on for
// asset CRUD. It matches the subset of *asset.Service that handlers use
// so they can be unit-tested with a mock.
type assetService interface {
	List(ctx context.Context, p asset.ListParams) ([]dbgen.Asset, int64, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Asset, error)
	Create(ctx context.Context, params dbgen.CreateAssetParams) (*dbgen.Asset, error)
	Update(ctx context.Context, params dbgen.UpdateAssetParams) (*dbgen.Asset, error)
	FindBySerialOrTag(ctx context.Context, tenantID uuid.UUID, serial, tag string) (*dbgen.Asset, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	GetStateAt(ctx context.Context, tenantID, assetID uuid.UUID, atTime time.Time) (dbgen.AssetSnapshot, error)
	ListSnapshots(ctx context.Context, tenantID, assetID uuid.UUID, limit int32) ([]dbgen.AssetSnapshot, error)
}
