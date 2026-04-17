package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockAssetService implements assetService for handler tests.
type mockAssetService struct {
	listFn              func(ctx context.Context, p asset.ListParams) ([]dbgen.Asset, int64, error)
	getByIDFn           func(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Asset, error)
	createFn            func(ctx context.Context, params dbgen.CreateAssetParams) (*dbgen.Asset, error)
	updateFn            func(ctx context.Context, params dbgen.UpdateAssetParams) (*dbgen.Asset, error)
	findBySerialOrTagFn func(ctx context.Context, tenantID uuid.UUID, serial, tag string) (*dbgen.Asset, error)
	deleteFn            func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *mockAssetService) List(ctx context.Context, p asset.ListParams) ([]dbgen.Asset, int64, error) {
	return m.listFn(ctx, p)
}
func (m *mockAssetService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.Asset, error) {
	return m.getByIDFn(ctx, tenantID, id)
}
func (m *mockAssetService) Create(ctx context.Context, params dbgen.CreateAssetParams) (*dbgen.Asset, error) {
	return m.createFn(ctx, params)
}
func (m *mockAssetService) Update(ctx context.Context, params dbgen.UpdateAssetParams) (*dbgen.Asset, error) {
	return m.updateFn(ctx, params)
}
func (m *mockAssetService) FindBySerialOrTag(ctx context.Context, tenantID uuid.UUID, serial, tag string) (*dbgen.Asset, error) {
	return m.findBySerialOrTagFn(ctx, tenantID, serial, tag)
}
func (m *mockAssetService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return m.deleteFn(ctx, tenantID, id)
}

func newAssetsTestServer(svc *mockAssetService) *APIServer {
	// qualitySvc, auditSvc, eventBus left nil. Handlers tolerate all three.
	return &APIServer{assetSvc: svc}
}

// stubAsset returns a minimal valid dbgen.Asset for tests.
func stubAsset(id, tenantID uuid.UUID, tag string) *dbgen.Asset {
	return &dbgen.Asset{
		ID:       id,
		TenantID: tenantID,
		AssetTag: tag,
		Name:     "test-" + tag,
		Type:     "server",
		Status:   "active",
		BiaLevel: "B",
	}
}

// ---------------------------------------------------------------------------
// ListAssets
// ---------------------------------------------------------------------------

func TestListAssets_Success(t *testing.T) {
	tenantID := uuid.New()
	a1 := stubAsset(uuid.New(), tenantID, "TAG-001")
	a2 := stubAsset(uuid.New(), tenantID, "TAG-002")

	var captured asset.ListParams
	svc := &mockAssetService{
		listFn: func(_ context.Context, p asset.ListParams) ([]dbgen.Asset, int64, error) {
			captured = p
			return []dbgen.Asset{*a1, *a2}, 2, nil
		},
	}
	s := newAssetsTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.ListAssets(c, ListAssetsParams{}) },
		http.MethodGet, "/assets", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if captured.TenantID != tenantID {
		t.Errorf("tenant_id not propagated: got %v, want %v", captured.TenantID, tenantID)
	}
	if captured.Limit != 20 || captured.Offset != 0 {
		t.Errorf("default pagination wrong: limit=%d offset=%d", captured.Limit, captured.Offset)
	}
}

func TestListAssets_ServiceError(t *testing.T) {
	svc := &mockAssetService{
		listFn: func(context.Context, asset.ListParams) ([]dbgen.Asset, int64, error) {
			return nil, 0, errors.New("db down")
		},
	}
	s := newAssetsTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.ListAssets(c, ListAssetsParams{}) },
		http.MethodGet, "/assets", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestListAssets_PaginationClampsAt100(t *testing.T) {
	var captured asset.ListParams
	svc := &mockAssetService{
		listFn: func(_ context.Context, p asset.ListParams) ([]dbgen.Asset, int64, error) {
			captured = p
			return nil, 0, nil
		},
	}
	s := newAssetsTestServer(svc)
	bigPageSize := 9999
	rec := runHandler(t, func(c *gin.Context) { s.ListAssets(c, ListAssetsParams{PageSize: &bigPageSize}) },
		http.MethodGet, "/assets", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if captured.Limit != 100 {
		t.Errorf("page_size=%d was not clamped to 100, got limit=%d", bigPageSize, captured.Limit)
	}
}

// ---------------------------------------------------------------------------
// GetAsset
// ---------------------------------------------------------------------------

func TestGetAsset_Success(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	svc := &mockAssetService{
		getByIDFn: func(_ context.Context, tid, id uuid.UUID) (*dbgen.Asset, error) {
			if tid != tenantID {
				t.Errorf("tenant not propagated: %v vs %v", tid, tenantID)
			}
			if id != assetID {
				t.Errorf("id not propagated: %v vs %v", id, assetID)
			}
			return stubAsset(assetID, tenantID, "TAG-G"), nil
		},
	}
	s := newAssetsTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetAsset(c, IdPath(assetID)) },
		http.MethodGet, "/assets/"+assetID.String(), nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	svc := &mockAssetService{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*dbgen.Asset, error) {
			return nil, errors.New("no rows")
		},
	}
	s := newAssetsTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.GetAsset(c, IdPath(uuid.New())) },
		http.MethodGet, "/assets/x", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// CreateAsset
// ---------------------------------------------------------------------------

func TestCreateAsset_Success(t *testing.T) {
	tenantID := uuid.New()
	var captured dbgen.CreateAssetParams
	svc := &mockAssetService{
		createFn: func(_ context.Context, p dbgen.CreateAssetParams) (*dbgen.Asset, error) {
			captured = p
			return stubAsset(uuid.New(), p.TenantID, p.AssetTag), nil
		},
	}
	s := newAssetsTestServer(svc)
	body := map[string]any{
		"asset_tag":  "TAG-C-001",
		"name":       "new-asset",
		"type":       "server",
		"status":     "active",
		"bia_level":  "A",
		"model":      "Dell R740",
		"vendor":     "Dell",
		"serial_number": "SN12345",
		"sub_type":   "",
		"attributes": map[string]any{},
		"tags":       []string{},
	}
	rec := runHandler(t, s.CreateAsset,
		http.MethodPost, "/assets", body, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if captured.TenantID != tenantID {
		t.Errorf("tenant_id from context not used: got %v, want %v", captured.TenantID, tenantID)
	}
	if captured.AssetTag != "TAG-C-001" {
		t.Errorf("asset_tag not propagated: got %q", captured.AssetTag)
	}
}

func TestCreateAsset_InvalidBody(t *testing.T) {
	s := newAssetsTestServer(&mockAssetService{})
	rec := runHandler(t, s.CreateAsset,
		http.MethodPost, "/assets", "not-an-object", nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAsset_DuplicateTag(t *testing.T) {
	svc := &mockAssetService{
		createFn: func(context.Context, dbgen.CreateAssetParams) (*dbgen.Asset, error) {
			return nil, errors.New(`ERROR: duplicate key value violates unique constraint "assets_asset_tag_key"`)
		},
	}
	s := newAssetsTestServer(svc)
	body := map[string]any{
		"asset_tag": "DUPE", "name": "x", "type": "server", "status": "active",
		"bia_level": "C", "model": "", "vendor": "", "serial_number": "",
		"sub_type": "", "attributes": map[string]any{}, "tags": []string{},
	}
	rec := runHandler(t, s.CreateAsset,
		http.MethodPost, "/assets", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 on duplicate", rec.Code)
	}
}

func TestCreateAsset_ServiceError(t *testing.T) {
	svc := &mockAssetService{
		createFn: func(context.Context, dbgen.CreateAssetParams) (*dbgen.Asset, error) {
			return nil, errors.New("db down")
		},
	}
	s := newAssetsTestServer(svc)
	body := map[string]any{
		"asset_tag": "T", "name": "x", "type": "server", "status": "active",
		"bia_level": "C", "model": "", "vendor": "", "serial_number": "",
		"sub_type": "", "attributes": map[string]any{}, "tags": []string{},
	}
	rec := runHandler(t, s.CreateAsset,
		http.MethodPost, "/assets", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateAsset
// ---------------------------------------------------------------------------

func TestUpdateAsset_Success(t *testing.T) {
	assetID := uuid.New()
	tenantID := uuid.New()
	newName := "renamed"
	var captured dbgen.UpdateAssetParams
	svc := &mockAssetService{
		updateFn: func(_ context.Context, p dbgen.UpdateAssetParams) (*dbgen.Asset, error) {
			captured = p
			return stubAsset(assetID, tenantID, "TAG-U"), nil
		},
	}
	s := newAssetsTestServer(svc)
	// Do NOT set ip_address — that path hits s.pool which is nil in tests.
	body := map[string]any{"name": newName}
	rec := runHandler(t, func(c *gin.Context) { s.UpdateAsset(c, IdPath(assetID)) },
		http.MethodPut, "/assets/"+assetID.String(), body, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if uuid.UUID(captured.ID) != assetID {
		t.Errorf("id not propagated: %v vs %v", captured.ID, assetID)
	}
	if !captured.Name.Valid || captured.Name.String != newName {
		t.Errorf("name not propagated: %+v", captured.Name)
	}
}

func TestUpdateAsset_InvalidBody(t *testing.T) {
	s := newAssetsTestServer(&mockAssetService{})
	rec := runHandler(t, func(c *gin.Context) { s.UpdateAsset(c, IdPath(uuid.New())) },
		http.MethodPut, "/assets/x", "not-an-object", nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateAsset_NotFound(t *testing.T) {
	svc := &mockAssetService{
		updateFn: func(context.Context, dbgen.UpdateAssetParams) (*dbgen.Asset, error) {
			return nil, errors.New("no rows in result set")
		},
	}
	s := newAssetsTestServer(svc)
	name := "n"
	body := map[string]any{"name": name}
	rec := runHandler(t, func(c *gin.Context) { s.UpdateAsset(c, IdPath(uuid.New())) },
		http.MethodPut, "/assets/x", body, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// DeleteAsset
// ---------------------------------------------------------------------------

func TestDeleteAsset_Success(t *testing.T) {
	assetID := uuid.New()
	tenantID := uuid.New()
	var capturedTenant, capturedID uuid.UUID
	svc := &mockAssetService{
		deleteFn: func(_ context.Context, tid, id uuid.UUID) error {
			capturedTenant = tid
			capturedID = id
			return nil
		},
	}
	s := newAssetsTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteAsset(c, IdPath(assetID)) },
		http.MethodDelete, "/assets/"+assetID.String(), nil, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if capturedTenant != tenantID {
		t.Errorf("tenant not propagated: %v vs %v", capturedTenant, tenantID)
	}
	if capturedID != assetID {
		t.Errorf("id not propagated: %v vs %v", capturedID, assetID)
	}
}

func TestDeleteAsset_NotFound(t *testing.T) {
	svc := &mockAssetService{
		deleteFn: func(context.Context, uuid.UUID, uuid.UUID) error {
			return errors.New("no rows in result set")
		},
	}
	s := newAssetsTestServer(svc)
	rec := runHandler(t, func(c *gin.Context) { s.DeleteAsset(c, IdPath(uuid.New())) },
		http.MethodDelete, "/assets/x", nil, nil,
		map[string]string{"tenant_id": uuid.New().String(), "user_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
