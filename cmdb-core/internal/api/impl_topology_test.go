package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// mockTopologyService — implements topologyService for handler tests.
// ---------------------------------------------------------------------------

type mockTopologyService struct {
	listRootLocationsFn            func(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error)
	listAllLocationsFn             func(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error)
	getLocationFn                  func(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Location, error)
	getBySlugFn                    func(ctx context.Context, tenantID uuid.UUID, slug, level string) (*dbgen.Location, error)
	listChildrenFn                 func(ctx context.Context, parentID uuid.UUID) ([]dbgen.Location, error)
	listAncestorsFn                func(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error)
	listDescendantsFn              func(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error)
	getLocationStatsFn             func(ctx context.Context, tenantID, locationID uuid.UUID) (topology.LocationStats, error)
	createLocationFn               func(ctx context.Context, params dbgen.CreateLocationParams) (*dbgen.Location, error)
	updateLocationFn               func(ctx context.Context, params dbgen.UpdateLocationParams) (*dbgen.Location, error)
	preflightDeleteLocationFn      func(ctx context.Context, tenantID, id uuid.UUID) (*topology.LocationDeleteInfo, error)
	deleteLocationFn               func(ctx context.Context, tenantID, id uuid.UUID, recursive bool) error
	listRacksByLocationFn          func(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.Rack, error)
	getRackFn                      func(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Rack, error)
	listAssetsByRackFn             func(ctx context.Context, rackID uuid.UUID) ([]dbgen.Asset, error)
	getRackOccupancyFn             func(ctx context.Context, rackID uuid.UUID) (dbgen.GetRackOccupancyRow, error)
	getRackOccupanciesByLocationFn func(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.GetRackOccupanciesByLocationRow, error)
	createRackFn                   func(ctx context.Context, params dbgen.CreateRackParams) (*dbgen.Rack, error)
	updateRackFn                   func(ctx context.Context, params dbgen.UpdateRackParams) (*dbgen.Rack, error)
	deleteRackFn                   func(ctx context.Context, tenantID, id uuid.UUID) error
	listRackSlotsFn                func(ctx context.Context, rackID uuid.UUID) ([]dbgen.ListRackSlotsRow, error)
	checkSlotConflictFn            func(ctx context.Context, rackID uuid.UUID, side string, startU, endU int32) (int64, error)
	createRackSlotFn               func(ctx context.Context, params dbgen.CreateRackSlotParams) (*dbgen.RackSlot, error)
	deleteRackSlotFn               func(ctx context.Context, tenantID, slotID uuid.UUID) error
	getImpactPathFn                func(ctx context.Context, tenantID, rootAssetID uuid.UUID, maxDepth int, direction topology.ImpactDirection) ([]topology.ImpactEdge, error)
	getImpactPathAtFn              func(ctx context.Context, tenantID, rootAssetID uuid.UUID, maxDepth int, direction topology.ImpactDirection, atTime *time.Time) ([]topology.ImpactEdge, error)
	createDependencyFn             func(ctx context.Context, p topology.CreateDependencyParams) error
}

func (m *mockTopologyService) ListRootLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error) {
	return m.listRootLocationsFn(ctx, tenantID)
}
func (m *mockTopologyService) ListAllLocations(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Location, error) {
	return m.listAllLocationsFn(ctx, tenantID)
}
func (m *mockTopologyService) GetLocation(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Location, error) {
	return m.getLocationFn(ctx, tenantID, id)
}
func (m *mockTopologyService) GetBySlug(ctx context.Context, tenantID uuid.UUID, slug, level string) (*dbgen.Location, error) {
	return m.getBySlugFn(ctx, tenantID, slug, level)
}
func (m *mockTopologyService) ListChildren(ctx context.Context, parentID uuid.UUID) ([]dbgen.Location, error) {
	return m.listChildrenFn(ctx, parentID)
}
func (m *mockTopologyService) ListAncestors(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error) {
	return m.listAncestorsFn(ctx, tenantID, path)
}
func (m *mockTopologyService) ListDescendants(ctx context.Context, tenantID uuid.UUID, path string) ([]dbgen.Location, error) {
	return m.listDescendantsFn(ctx, tenantID, path)
}
func (m *mockTopologyService) GetLocationStats(ctx context.Context, tenantID, locationID uuid.UUID) (topology.LocationStats, error) {
	return m.getLocationStatsFn(ctx, tenantID, locationID)
}
func (m *mockTopologyService) CreateLocation(ctx context.Context, params dbgen.CreateLocationParams) (*dbgen.Location, error) {
	return m.createLocationFn(ctx, params)
}
func (m *mockTopologyService) UpdateLocation(ctx context.Context, params dbgen.UpdateLocationParams) (*dbgen.Location, error) {
	return m.updateLocationFn(ctx, params)
}
func (m *mockTopologyService) PreflightDeleteLocation(ctx context.Context, tenantID, id uuid.UUID) (*topology.LocationDeleteInfo, error) {
	return m.preflightDeleteLocationFn(ctx, tenantID, id)
}
func (m *mockTopologyService) DeleteLocation(ctx context.Context, tenantID, id uuid.UUID, recursive bool) error {
	return m.deleteLocationFn(ctx, tenantID, id, recursive)
}
func (m *mockTopologyService) ListRacksByLocation(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.Rack, error) {
	return m.listRacksByLocationFn(ctx, tenantID, locationID)
}
func (m *mockTopologyService) GetRack(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Rack, error) {
	return m.getRackFn(ctx, tenantID, id)
}
func (m *mockTopologyService) ListAssetsByRack(ctx context.Context, rackID uuid.UUID) ([]dbgen.Asset, error) {
	return m.listAssetsByRackFn(ctx, rackID)
}
func (m *mockTopologyService) GetRackOccupancy(ctx context.Context, rackID uuid.UUID) (dbgen.GetRackOccupancyRow, error) {
	if m.getRackOccupancyFn == nil {
		return dbgen.GetRackOccupancyRow{}, errors.New("not stubbed")
	}
	return m.getRackOccupancyFn(ctx, rackID)
}
func (m *mockTopologyService) GetRackOccupanciesByLocation(ctx context.Context, tenantID, locationID uuid.UUID) ([]dbgen.GetRackOccupanciesByLocationRow, error) {
	if m.getRackOccupanciesByLocationFn == nil {
		return nil, errors.New("not stubbed")
	}
	return m.getRackOccupanciesByLocationFn(ctx, tenantID, locationID)
}
func (m *mockTopologyService) CreateRack(ctx context.Context, params dbgen.CreateRackParams) (*dbgen.Rack, error) {
	return m.createRackFn(ctx, params)
}
func (m *mockTopologyService) UpdateRack(ctx context.Context, params dbgen.UpdateRackParams) (*dbgen.Rack, error) {
	return m.updateRackFn(ctx, params)
}
func (m *mockTopologyService) DeleteRack(ctx context.Context, tenantID, id uuid.UUID) error {
	return m.deleteRackFn(ctx, tenantID, id)
}
func (m *mockTopologyService) ListRackSlots(ctx context.Context, rackID uuid.UUID) ([]dbgen.ListRackSlotsRow, error) {
	return m.listRackSlotsFn(ctx, rackID)
}
func (m *mockTopologyService) CheckSlotConflict(ctx context.Context, rackID uuid.UUID, side string, startU, endU int32) (int64, error) {
	return m.checkSlotConflictFn(ctx, rackID, side, startU, endU)
}
func (m *mockTopologyService) CreateRackSlot(ctx context.Context, params dbgen.CreateRackSlotParams) (*dbgen.RackSlot, error) {
	return m.createRackSlotFn(ctx, params)
}
func (m *mockTopologyService) DeleteRackSlot(ctx context.Context, tenantID, slotID uuid.UUID) error {
	return m.deleteRackSlotFn(ctx, tenantID, slotID)
}
func (m *mockTopologyService) GetImpactPath(ctx context.Context, tenantID, rootAssetID uuid.UUID, maxDepth int, direction topology.ImpactDirection) ([]topology.ImpactEdge, error) {
	return m.getImpactPathFn(ctx, tenantID, rootAssetID, maxDepth, direction)
}
func (m *mockTopologyService) GetImpactPathAt(ctx context.Context, tenantID, rootAssetID uuid.UUID, maxDepth int, direction topology.ImpactDirection, atTime *time.Time) ([]topology.ImpactEdge, error) {
	return m.getImpactPathAtFn(ctx, tenantID, rootAssetID, maxDepth, direction, atTime)
}
func (m *mockTopologyService) CreateDependency(ctx context.Context, p topology.CreateDependencyParams) error {
	return m.createDependencyFn(ctx, p)
}

func newTopologyTestServer(svc *mockTopologyService) *APIServer {
	return &APIServer{topologySvc: svc}
}

func stubLocation(id, tenantID uuid.UUID, name, slug string) dbgen.Location {
	return dbgen.Location{
		ID:       id,
		TenantID: tenantID,
		Name:     name,
		Slug:     slug,
		Level:    "room",
		Status:   "active",
		Path:     pgtype.Text{String: slug, Valid: true},
	}
}

func stubRack(id, tenantID, locationID uuid.UUID, name string, totalU int32) dbgen.Rack {
	return dbgen.Rack{
		ID:         id,
		TenantID:   tenantID,
		LocationID: locationID,
		Name:       name,
		TotalU:     totalU,
		Status:     "active",
	}
}

// ---------------------------------------------------------------------------
// Location handlers
// ---------------------------------------------------------------------------

func TestListLocations_BySlug_Success(t *testing.T) {
	tenantID := uuid.New()
	locID := uuid.New()
	slug, level := "sfo-1", "datacenter"
	svc := &mockTopologyService{
		getBySlugFn: func(_ context.Context, tid uuid.UUID, s, l string) (*dbgen.Location, error) {
			if tid != tenantID || s != slug || l != level {
				t.Errorf("args not propagated: tid=%v slug=%q level=%q", tid, s, l)
			}
			loc := stubLocation(locID, tenantID, "SFO DC 1", slug)
			return &loc, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) {
		s.ListLocations(c, ListLocationsParams{Slug: &slug, Level: &level})
	}, http.MethodGet, "/locations?slug="+slug+"&level="+level, nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestListLocations_BySlug_NotFound(t *testing.T) {
	slug, level := "missing", "room"
	svc := &mockTopologyService{
		getBySlugFn: func(context.Context, uuid.UUID, string, string) (*dbgen.Location, error) {
			return nil, errors.New("no rows")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) {
		s.ListLocations(c, ListLocationsParams{Slug: &slug, Level: &level})
	}, http.MethodGet, "/locations", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListLocations_Roots_Success(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockTopologyService{
		listRootLocationsFn: func(_ context.Context, tid uuid.UUID) ([]dbgen.Location, error) {
			if tid != tenantID {
				t.Errorf("tenant not propagated: %v vs %v", tid, tenantID)
			}
			return []dbgen.Location{stubLocation(uuid.New(), tenantID, "root", "root")}, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.ListLocations(c, ListLocationsParams{}) },
		http.MethodGet, "/locations", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestListLocations_Roots_ServiceError(t *testing.T) {
	svc := &mockTopologyService{
		listRootLocationsFn: func(context.Context, uuid.UUID) ([]dbgen.Location, error) {
			return nil, errors.New("db down")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.ListLocations(c, ListLocationsParams{}) },
		http.MethodGet, "/locations", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestGetLocation_Success(t *testing.T) {
	tenantID := uuid.New()
	locID := uuid.New()
	svc := &mockTopologyService{
		getLocationFn: func(_ context.Context, tid, id uuid.UUID) (dbgen.Location, error) {
			if tid != tenantID || id != locID {
				t.Errorf("args not propagated: tid=%v id=%v", tid, id)
			}
			return stubLocation(locID, tenantID, "site", "site"), nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetLocation(c, IdPath(locID)) },
		http.MethodGet, "/locations/"+locID.String(), nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestGetLocation_NotFound(t *testing.T) {
	svc := &mockTopologyService{
		getLocationFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Location, error) {
			return dbgen.Location{}, errors.New("no rows")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetLocation(c, IdPath(uuid.New())) },
		http.MethodGet, "/locations/x", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListLocationChildren_Success(t *testing.T) {
	parentID := uuid.New()
	tenantID := uuid.New()
	var capturedParent uuid.UUID
	svc := &mockTopologyService{
		listChildrenFn: func(_ context.Context, pid uuid.UUID) ([]dbgen.Location, error) {
			capturedParent = pid
			return []dbgen.Location{stubLocation(uuid.New(), tenantID, "child", "child")}, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.ListLocationChildren(c, IdPath(parentID)) },
		http.MethodGet, "/locations/"+parentID.String()+"/children", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if capturedParent != parentID {
		t.Errorf("parent id not propagated: %v vs %v", capturedParent, parentID)
	}
}

func TestGetLocationStats_Success(t *testing.T) {
	locID := uuid.New()
	svc := &mockTopologyService{
		getLocationStatsFn: func(context.Context, uuid.UUID, uuid.UUID) (topology.LocationStats, error) {
			return topology.LocationStats{
				TotalAssets:    3,
				TotalRacks:     2,
				CriticalAlerts: 1,
				AvgOccupancy:   0.5,
			}, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetLocationStats(c, IdPath(locID)) },
		http.MethodGet, "/locations/x/stats", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestGetLocationStats_ServiceError(t *testing.T) {
	svc := &mockTopologyService{
		getLocationStatsFn: func(context.Context, uuid.UUID, uuid.UUID) (topology.LocationStats, error) {
			return topology.LocationStats{}, errors.New("db down")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetLocationStats(c, IdPath(uuid.New())) },
		http.MethodGet, "/locations/x/stats", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCreateLocation_InvalidBody(t *testing.T) {
	s := newTopologyTestServer(&mockTopologyService{})
	rec := runHandler(t, s.CreateLocation,
		http.MethodPost, "/locations", "not-json", nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateLocation_DuplicateSlug(t *testing.T) {
	svc := &mockTopologyService{
		createLocationFn: func(context.Context, dbgen.CreateLocationParams) (*dbgen.Location, error) {
			return nil, errors.New(`ERROR: duplicate key value violates unique constraint "locations_slug_key"`)
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"name": "Room 3", "slug": "r3", "level": "room", "status": "active",
	}
	rec := runHandler(t, s.CreateLocation,
		http.MethodPost, "/locations", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 on duplicate slug", rec.Code)
	}
}

func TestDeleteLocation_PreflightMode(t *testing.T) {
	svc := &mockTopologyService{
		preflightDeleteLocationFn: func(context.Context, uuid.UUID, uuid.UUID) (*topology.LocationDeleteInfo, error) {
			return &topology.LocationDeleteInfo{ChildLocations: 1, Racks: 2, Assets: 3}, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteLocation(c, IdPath(uuid.New())) },
		http.MethodDelete, "/locations/x?preflight=true", nil, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for preflight", rec.Code)
	}
}

func TestDeleteLocation_HasDependencies_Returns409(t *testing.T) {
	svc := &mockTopologyService{
		deleteLocationFn: func(context.Context, uuid.UUID, uuid.UUID, bool) error {
			return errors.New("location has 2 children, 0 racks, 0 assets — use recursive=true to force delete")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteLocation(c, IdPath(uuid.New())) },
		http.MethodDelete, "/locations/x", nil, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for HAS_DEPENDENCIES", rec.Code)
	}
}

func TestDeleteLocation_Success(t *testing.T) {
	var capturedRecursive bool
	svc := &mockTopologyService{
		deleteLocationFn: func(_ context.Context, _, _ uuid.UUID, recursive bool) error {
			capturedRecursive = recursive
			return nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteLocation(c, IdPath(uuid.New())) },
		http.MethodDelete, "/locations/x?recursive=true", nil, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if !capturedRecursive {
		t.Errorf("recursive=true not propagated to service")
	}
}

// ---------------------------------------------------------------------------
// Rack handlers
// ---------------------------------------------------------------------------

func TestCreateRack_InvalidTotalU(t *testing.T) {
	s := newTopologyTestServer(&mockTopologyService{})
	zeroU := 0
	body := map[string]any{
		"location_id": uuid.New(),
		"name":        "R1",
		"status":      "active",
		"total_u":     zeroU,
	}
	rec := runHandler(t, s.CreateRack,
		http.MethodPost, "/racks", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for total_u <= 0", rec.Code)
	}
}

func TestCreateRack_Success(t *testing.T) {
	tenantID := uuid.New()
	locID := uuid.New()
	var captured dbgen.CreateRackParams
	svc := &mockTopologyService{
		createRackFn: func(_ context.Context, p dbgen.CreateRackParams) (*dbgen.Rack, error) {
			captured = p
			r := stubRack(uuid.New(), p.TenantID, p.LocationID, p.Name, p.TotalU)
			return &r, nil
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"location_id": locID,
		"name":        "R-A",
		"status":      "active",
		"total_u":     42,
	}
	rec := runHandler(t, s.CreateRack,
		http.MethodPost, "/racks", body, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if captured.TenantID != tenantID {
		t.Errorf("tenant_id not propagated: %v vs %v", captured.TenantID, tenantID)
	}
	if captured.TotalU != 42 {
		t.Errorf("total_u not propagated: %d", captured.TotalU)
	}
}

func TestCreateRack_Duplicate(t *testing.T) {
	svc := &mockTopologyService{
		createRackFn: func(context.Context, dbgen.CreateRackParams) (*dbgen.Rack, error) {
			return nil, errors.New(`duplicate key value violates unique constraint "racks_location_name"`)
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"location_id": uuid.New(),
		"name":        "dup",
		"status":      "active",
		"total_u":     10,
	}
	rec := runHandler(t, s.CreateRack,
		http.MethodPost, "/racks", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestGetRack_NotFound(t *testing.T) {
	svc := &mockTopologyService{
		getRackFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Rack, error) {
			return dbgen.Rack{}, errors.New("no rows")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetRack(c, IdPath(uuid.New())) },
		http.MethodGet, "/racks/x", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGetRack_Success(t *testing.T) {
	rackID := uuid.New()
	tenantID := uuid.New()
	svc := &mockTopologyService{
		getRackFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Rack, error) {
			return stubRack(rackID, tenantID, uuid.New(), "R1", 42), nil
		},
		// getRackOccupancyFn left nil → handler uses 0 on error; that's OK.
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetRack(c, IdPath(rackID)) },
		http.MethodGet, "/racks/"+rackID.String(), nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRack_InvalidLocationID(t *testing.T) {
	s := newTopologyTestServer(&mockTopologyService{})
	body := map[string]any{"location_id": "not-a-uuid"}
	rec := runHandler(t, func(c *gin.Context) { s.UpdateRack(c, IdPath(uuid.New())) },
		http.MethodPut, "/racks/x", body, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateRack_NotFound(t *testing.T) {
	svc := &mockTopologyService{
		updateRackFn: func(context.Context, dbgen.UpdateRackParams) (*dbgen.Rack, error) {
			return nil, errors.New("no rows in result set")
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{"name": "x"}
	rec := runHandler(t, func(c *gin.Context) { s.UpdateRack(c, IdPath(uuid.New())) },
		http.MethodPut, "/racks/x", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteRack_Success(t *testing.T) {
	rackID := uuid.New()
	tenantID := uuid.New()
	var capturedTenant, capturedID uuid.UUID
	svc := &mockTopologyService{
		deleteRackFn: func(_ context.Context, tid, id uuid.UUID) error {
			capturedTenant = tid
			capturedID = id
			return nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteRack(c, IdPath(rackID)) },
		http.MethodDelete, "/racks/"+rackID.String(), nil, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if capturedTenant != tenantID || capturedID != rackID {
		t.Errorf("ids not propagated: tenant=%v id=%v", capturedTenant, capturedID)
	}
}

func TestDeleteRack_NotFound(t *testing.T) {
	svc := &mockTopologyService{
		deleteRackFn: func(context.Context, uuid.UUID, uuid.UUID) error {
			return errors.New("no rows")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteRack(c, IdPath(uuid.New())) },
		http.MethodDelete, "/racks/x", nil, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Rack slot handlers
// ---------------------------------------------------------------------------

func TestListRackSlots_Success(t *testing.T) {
	svc := &mockTopologyService{
		listRackSlotsFn: func(context.Context, uuid.UUID) ([]dbgen.ListRackSlotsRow, error) {
			return []dbgen.ListRackSlotsRow{}, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.ListRackSlots(c, IdPath(uuid.New())) },
		http.MethodGet, "/racks/x/slots", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCreateRackSlot_RackNotFound(t *testing.T) {
	svc := &mockTopologyService{
		getRackFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Rack, error) {
			return dbgen.Rack{}, errors.New("no rows")
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"asset_id": uuid.New(),
		"start_u":  1,
		"end_u":    2,
	}
	rec := runHandler(t, func(c *gin.Context) { s.CreateRackSlot(c, IdPath(uuid.New())) },
		http.MethodPost, "/racks/x/slots", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when rack missing", rec.Code)
	}
}

func TestCreateRackSlot_OutOfRange(t *testing.T) {
	rackID := uuid.New()
	svc := &mockTopologyService{
		getRackFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Rack, error) {
			return stubRack(rackID, uuid.New(), uuid.New(), "R", 10), nil
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"asset_id": uuid.New(),
		"start_u":  1,
		"end_u":    99, // > rack's total_u=10
	}
	rec := runHandler(t, func(c *gin.Context) { s.CreateRackSlot(c, IdPath(rackID)) },
		http.MethodPost, "/racks/"+rackID.String()+"/slots", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for out-of-range U", rec.Code)
	}
}

func TestCreateRackSlot_StartExceedsEnd(t *testing.T) {
	rackID := uuid.New()
	svc := &mockTopologyService{
		getRackFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Rack, error) {
			return stubRack(rackID, uuid.New(), uuid.New(), "R", 42), nil
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"asset_id": uuid.New(),
		"start_u":  10,
		"end_u":    5,
	}
	rec := runHandler(t, func(c *gin.Context) { s.CreateRackSlot(c, IdPath(rackID)) },
		http.MethodPost, "/racks/x/slots", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for start > end", rec.Code)
	}
}

func TestCreateRackSlot_Conflict(t *testing.T) {
	rackID := uuid.New()
	svc := &mockTopologyService{
		getRackFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.Rack, error) {
			return stubRack(rackID, uuid.New(), uuid.New(), "R", 42), nil
		},
		checkSlotConflictFn: func(context.Context, uuid.UUID, string, int32, int32) (int64, error) {
			return 1, nil
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"asset_id": uuid.New(),
		"start_u":  1,
		"end_u":    4,
	}
	rec := runHandler(t, func(c *gin.Context) { s.CreateRackSlot(c, IdPath(rackID)) },
		http.MethodPost, "/racks/"+rackID.String()+"/slots", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for slot conflict", rec.Code)
	}
}

func TestDeleteRackSlot_NotFound(t *testing.T) {
	svc := &mockTopologyService{
		deleteRackSlotFn: func(context.Context, uuid.UUID, uuid.UUID) error {
			return errors.New("no rows")
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) {
		s.DeleteRackSlot(c, IdPath(uuid.New()), openapi_types.UUID(uuid.New()))
	}, http.MethodDelete, "/racks/x/slots/y", nil, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Dependency handlers
// ---------------------------------------------------------------------------

func TestCreateAssetDependency_InvalidBody(t *testing.T) {
	s := newTopologyTestServer(&mockTopologyService{})
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", "not-an-object", nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAssetDependency_MissingIDs(t *testing.T) {
	s := newTopologyTestServer(&mockTopologyService{})
	body := map[string]any{
		"source_asset_id": uuid.Nil,
		"target_asset_id": uuid.Nil,
	}
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", body, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for nil UUIDs", rec.Code)
	}
}

func TestCreateAssetDependency_InvalidCategory(t *testing.T) {
	s := newTopologyTestServer(&mockTopologyService{})
	bogus := DependencyCategory("bogus")
	body := map[string]any{
		"source_asset_id":     uuid.New(),
		"target_asset_id":     uuid.New(),
		"dependency_category": bogus,
	}
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", body, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid category", rec.Code)
	}
}

func TestCreateAssetDependency_SelfDependency(t *testing.T) {
	svc := &mockTopologyService{
		createDependencyFn: func(context.Context, topology.CreateDependencyParams) error {
			return topology.ErrSelfDependency
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"source_asset_id": uuid.New(),
		"target_asset_id": uuid.New(),
	}
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for self-dependency", rec.Code)
	}
}

func TestCreateAssetDependency_Cycle(t *testing.T) {
	svc := &mockTopologyService{
		createDependencyFn: func(context.Context, topology.CreateDependencyParams) error {
			return topology.ErrCycleDetected
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"source_asset_id": uuid.New(),
		"target_asset_id": uuid.New(),
	}
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for cycle", rec.Code)
	}
}

func TestCreateAssetDependency_AlreadyExists(t *testing.T) {
	svc := &mockTopologyService{
		createDependencyFn: func(context.Context, topology.CreateDependencyParams) error {
			return topology.ErrDependencyExists
		},
	}
	s := newTopologyTestServer(svc)
	body := map[string]any{
		"source_asset_id": uuid.New(),
		"target_asset_id": uuid.New(),
	}
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for existing dependency", rec.Code)
	}
}

func TestCreateAssetDependency_Success(t *testing.T) {
	var captured topology.CreateDependencyParams
	svc := &mockTopologyService{
		createDependencyFn: func(_ context.Context, p topology.CreateDependencyParams) error {
			captured = p
			return nil
		},
	}
	s := newTopologyTestServer(svc)
	src, tgt := uuid.New(), uuid.New()
	body := map[string]any{
		"source_asset_id": src,
		"target_asset_id": tgt,
		"dependency_type": "connects_to",
	}
	rec := runHandler(t, s.CreateAssetDependency,
		http.MethodPost, "/topology/dependencies", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if captured.SourceAssetID != src || captured.TargetAssetID != tgt {
		t.Errorf("ids not propagated: got src=%v tgt=%v", captured.SourceAssetID, captured.TargetAssetID)
	}
	if captured.DependencyType != "connects_to" {
		t.Errorf("dependency_type not propagated: %q", captured.DependencyType)
	}
	if captured.Category != "dependency" {
		t.Errorf("default category not applied: %q", captured.Category)
	}
}

func TestGetTopologyImpact_InvalidDepth(t *testing.T) {
	svc := &mockTopologyService{
		getImpactPathAtFn: func(context.Context, uuid.UUID, uuid.UUID, int, topology.ImpactDirection, *time.Time) ([]topology.ImpactEdge, error) {
			return nil, errors.New("max_depth must be between 1 and 10")
		},
	}
	s := newTopologyTestServer(svc)
	bigDepth := 999
	rec := runHandler(t, func(c *gin.Context) {
		s.GetTopologyImpact(c, GetTopologyImpactParams{
			RootAssetId: openapi_types.UUID(uuid.New()),
			MaxDepth:    &bigDepth,
		})
	}, http.MethodGet, "/topology/impact", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for bad max_depth", rec.Code)
	}
}

func TestGetTopologyImpact_InvalidDirection(t *testing.T) {
	svc := &mockTopologyService{
		getImpactPathAtFn: func(context.Context, uuid.UUID, uuid.UUID, int, topology.ImpactDirection, *time.Time) ([]topology.ImpactEdge, error) {
			return nil, errors.New("direction must be downstream, upstream, or both")
		},
	}
	s := newTopologyTestServer(svc)
	bogus := GetTopologyImpactParamsDirection("sideways")
	rec := runHandler(t, func(c *gin.Context) {
		s.GetTopologyImpact(c, GetTopologyImpactParams{
			RootAssetId: openapi_types.UUID(uuid.New()),
			Direction:   &bogus,
		})
	}, http.MethodGet, "/topology/impact", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for bad direction", rec.Code)
	}
}

func TestGetTopologyImpact_Success(t *testing.T) {
	rootID := uuid.New()
	edge := topology.ImpactEdge{
		ID:                 uuid.New(),
		SourceAssetID:      rootID,
		SourceAssetName:    "root",
		TargetAssetID:      uuid.New(),
		TargetAssetName:    "child",
		DependencyType:     "depends_on",
		DependencyCategory: "dependency",
		Depth:              1,
		Path:               []uuid.UUID{rootID},
		Direction:          topology.ImpactDirectionDownstream,
	}
	var capturedDepth int
	var capturedDir topology.ImpactDirection
	svc := &mockTopologyService{
		getImpactPathAtFn: func(_ context.Context, _, _ uuid.UUID, depth int, dir topology.ImpactDirection, _ *time.Time) ([]topology.ImpactEdge, error) {
			capturedDepth = depth
			capturedDir = dir
			return []topology.ImpactEdge{edge}, nil
		},
	}
	s := newTopologyTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) {
		s.GetTopologyImpact(c, GetTopologyImpactParams{
			RootAssetId: openapi_types.UUID(rootID),
		})
	}, http.MethodGet, "/topology/impact", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if capturedDepth != defaultImpactMaxDepth {
		t.Errorf("default depth not applied: got %d, want %d", capturedDepth, defaultImpactMaxDepth)
	}
	if capturedDir != topology.ImpactDirectionDownstream {
		t.Errorf("default direction not applied: got %q", capturedDir)
	}
}
