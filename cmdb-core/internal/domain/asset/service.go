package asset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ErrSnapshotNotFound is returned by GetStateAt when the asset has no
// snapshot at or before the requested time — distinct from an unknown
// asset so the API layer can map it to a 404 with a targeted message
// ("this asset did not exist yet") rather than a generic not-found.
var ErrSnapshotNotFound = errors.New("no asset snapshot at or before requested time")

// ListParams holds the filtering and pagination parameters for listing assets.
type ListParams struct {
	TenantID     uuid.UUID
	Type         *string
	Status       *string
	LocationID   *uuid.UUID
	RackID       *uuid.UUID
	SerialNumber *string
	Search       *string
	Limit        int32
	Offset       int32
}

// Service provides asset domain operations.
type Service struct {
	queries *dbgen.Queries
	bus     eventbus.Bus
	pool    *pgxpool.Pool
}

// NewService creates a new asset Service.
func NewService(queries *dbgen.Queries, bus eventbus.Bus, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, bus: bus, pool: pool}
}

// List returns a paginated, filtered list of assets and the total count.
func (s *Service) List(ctx context.Context, p ListParams) ([]dbgen.Asset, int64, error) {
	listParams := dbgen.ListAssetsParams{
		TenantID: p.TenantID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	}
	countParams := dbgen.CountAssetsParams{
		TenantID: p.TenantID,
	}

	if p.Type != nil {
		listParams.Type = pgtype.Text{String: *p.Type, Valid: true}
		countParams.Type = pgtype.Text{String: *p.Type, Valid: true}
	}
	if p.Status != nil {
		listParams.Status = pgtype.Text{String: *p.Status, Valid: true}
		countParams.Status = pgtype.Text{String: *p.Status, Valid: true}
	}
	if p.LocationID != nil {
		listParams.LocationID = pgtype.UUID{Bytes: *p.LocationID, Valid: true}
		countParams.LocationID = pgtype.UUID{Bytes: *p.LocationID, Valid: true}
	}
	if p.RackID != nil {
		listParams.RackID = pgtype.UUID{Bytes: *p.RackID, Valid: true}
		countParams.RackID = pgtype.UUID{Bytes: *p.RackID, Valid: true}
	}
	if p.SerialNumber != nil {
		listParams.SerialNumber = pgtype.Text{String: *p.SerialNumber, Valid: true}
		countParams.SerialNumber = pgtype.Text{String: *p.SerialNumber, Valid: true}
	}
	if p.Search != nil {
		listParams.Search = pgtype.Text{String: *p.Search, Valid: true}
		countParams.Search = pgtype.Text{String: *p.Search, Valid: true}
	}

	assets, err := s.queries.ListAssets(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list assets: %w", err)
	}

	total, err := s.queries.CountAssets(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}

	return assets, total, nil
}

// GetByID returns a single asset by its ID, scoped to the given tenant.
func (s *Service) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Asset, error) {
	asset, err := s.queries.GetAsset(ctx, dbgen.GetAssetParams{ID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}
	return &asset, nil
}

// Create inserts a new asset and returns it.
func (s *Service) Create(ctx context.Context, params dbgen.CreateAssetParams) (*dbgen.Asset, error) {
	a, err := s.queries.CreateAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}
	s.incrementSyncVersion(ctx, "assets", a.ID)
	return &a, nil
}

// Update modifies an existing asset and returns it.
func (s *Service) Update(ctx context.Context, params dbgen.UpdateAssetParams) (*dbgen.Asset, error) {
	a, err := s.queries.UpdateAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}
	s.incrementSyncVersion(ctx, "assets", a.ID)
	return &a, nil
}

// FindBySerialOrTag finds an asset by serial number or asset tag.
func (s *Service) FindBySerialOrTag(ctx context.Context, tenantID uuid.UUID, serial, tag string) (*dbgen.Asset, error) {
	asset, err := s.queries.FindAssetBySerialOrTag(ctx, dbgen.FindAssetBySerialOrTagParams{
		TenantID:     tenantID,
		SerialNumber: pgtype.Text{String: serial, Valid: serial != ""},
		AssetTag:     tag,
	})
	if err != nil {
		return nil, fmt.Errorf("find asset by serial or tag: %w", err)
	}
	return &asset, nil
}

// Delete removes an asset by ID, scoped to the given tenant.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteAsset(ctx, dbgen.DeleteAssetParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	s.incrementSyncVersion(ctx, "assets", id)
	return nil
}

// GetStateAt returns the most-recent snapshot of the given asset whose
// valid_at is at or before atTime, scoped to the tenant. Drives D10-P0
// point-in-time queries ("what did this asset look like at 2026-03-01?").
//
// Returns ErrSnapshotNotFound when the asset has no snapshot at or
// before atTime — which can happen two ways: (a) the asset was created
// after atTime, (b) the asset existed but predates the 000056 backfill
// and has not been written since. Both cases legitimately have no
// historical state to return.
func (s *Service) GetStateAt(ctx context.Context, tenantID, assetID uuid.UUID, atTime time.Time) (dbgen.AssetSnapshot, error) {
	snap, err := s.queries.GetAssetStateAt(ctx, dbgen.GetAssetStateAtParams{
		AssetID:  assetID,
		TenantID: tenantID,
		ValidAt:  atTime,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.AssetSnapshot{}, ErrSnapshotNotFound
		}
		return dbgen.AssetSnapshot{}, fmt.Errorf("get asset state at %s: %w", atTime.Format(time.RFC3339), err)
	}
	return snap, nil
}

// ListSnapshots returns snapshots for an asset newest-first, capped at
// limit. A limit of 0 uses the default (100) — a heavily-edited asset
// can accumulate thousands of snapshots, and the UI uses this for a
// paged timeline rather than a full dump.
func (s *Service) ListSnapshots(ctx context.Context, tenantID, assetID uuid.UUID, limit int32) ([]dbgen.AssetSnapshot, error) {
	if limit <= 0 {
		limit = 100
	}
	snaps, err := s.queries.ListAssetSnapshots(ctx, dbgen.ListAssetSnapshotsParams{
		AssetID:  assetID,
		TenantID: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list asset snapshots: %w", err)
	}
	return snaps, nil
}

// BumpAccess records a read of one asset against the D9-P1 heat counter.
// Fire-and-forget: callers run this in a goroutine with a detached
// context so a counter failure never poisons the user-visible read.
func (s *Service) BumpAccess(ctx context.Context, tenantID, assetID uuid.UUID) error {
	if err := s.queries.BumpAssetAccess(ctx, dbgen.BumpAssetAccessParams{
		ID:       assetID,
		TenantID: tenantID,
	}); err != nil {
		return fmt.Errorf("bump asset access: %w", err)
	}
	return nil
}

// BumpAccessMany records reads of a page of assets in a single UPDATE.
// A no-op when ids is empty — the UPDATE would touch zero rows anyway,
// and the empty-array branch avoids a wasted round-trip on empty list
// pages (filter with no results).
func (s *Service) BumpAccessMany(ctx context.Context, tenantID uuid.UUID, assetIDs []uuid.UUID) error {
	if len(assetIDs) == 0 {
		return nil
	}
	if err := s.queries.BumpAssetsAccess(ctx, dbgen.BumpAssetsAccessParams{
		Ids:      assetIDs,
		TenantID: tenantID,
	}); err != nil {
		return fmt.Errorf("bump assets access: %w", err)
	}
	return nil
}

func (s *Service) incrementSyncVersion(ctx context.Context, table string, id uuid.UUID) {
	if s.pool == nil {
		return
	}
	if _, err := s.pool.Exec(ctx, fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", table), id); err != nil {
		zap.L().Error("asset: failed to increment sync_version", zap.String("table", table), zap.Error(err))
	}
}
