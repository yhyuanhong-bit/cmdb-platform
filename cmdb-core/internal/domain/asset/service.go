package asset

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

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
}

// NewService creates a new asset Service.
func NewService(queries *dbgen.Queries, bus eventbus.Bus) *Service {
	return &Service{queries: queries, bus: bus}
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

// GetByID returns a single asset by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.Asset, error) {
	asset, err := s.queries.GetAsset(ctx, id)
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
	return &a, nil
}

// Update modifies an existing asset and returns it.
func (s *Service) Update(ctx context.Context, params dbgen.UpdateAssetParams) (*dbgen.Asset, error) {
	a, err := s.queries.UpdateAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}
	return &a, nil
}

// Delete removes an asset by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteAsset(ctx, id); err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	return nil
}
