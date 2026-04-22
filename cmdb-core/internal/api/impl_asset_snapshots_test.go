package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func stubSnapshot(id, assetID, tenantID uuid.UUID, at time.Time, name, status string) dbgen.AssetSnapshot {
	return dbgen.AssetSnapshot{
		ID:         id,
		AssetID:    assetID,
		TenantID:   tenantID,
		ValidAt:    at,
		Name:       name,
		AssetTag:   "TAG-" + name,
		Status:     status,
		BiaLevel:   "B",
		Attributes: json.RawMessage(`{}`),
	}
}

func TestGetAssetStateAt_ReturnsSnapshot(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	snapID := uuid.New()

	var captured struct {
		tenantID uuid.UUID
		assetID  uuid.UUID
		at       time.Time
	}
	svc := &mockAssetService{
		getStateAtFn: func(_ context.Context, tid, aid uuid.UUID, t time.Time) (dbgen.AssetSnapshot, error) {
			captured.tenantID, captured.assetID, captured.at = tid, aid, t
			return stubSnapshot(snapID, aid, tid, t, "server-01", "operational"), nil
		},
	}
	s := newAssetsTestServer(svc)

	rec := runHandler(t,
		func(c *gin.Context) { s.GetAssetStateAt(c, IdPath(assetID), GetAssetStateAtParams{At: at}) },
		http.MethodGet, "/assets/"+assetID.String()+"/state-at?at="+at.Format(time.RFC3339),
		nil, nil, map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if captured.tenantID != tenantID || captured.assetID != assetID || !captured.at.Equal(at) {
		t.Errorf("service args not propagated: %+v", captured)
	}

	var body struct {
		Data AssetSnapshot `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Data.Id != snapID {
		t.Errorf("snapshot id mismatch: got %v, want %v", body.Data.Id, snapID)
	}
	if body.Data.Status != "operational" {
		t.Errorf("status mismatch: got %q", body.Data.Status)
	}
}

func TestGetAssetStateAt_NotFoundMapsToHTTP404(t *testing.T) {
	svc := &mockAssetService{
		getStateAtFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time) (dbgen.AssetSnapshot, error) {
			return dbgen.AssetSnapshot{}, asset.ErrSnapshotNotFound
		},
	}
	s := newAssetsTestServer(svc)

	rec := runHandler(t,
		func(c *gin.Context) {
			s.GetAssetStateAt(c, IdPath(uuid.New()),
				GetAssetStateAtParams{At: time.Now()})
		},
		http.MethodGet, "/assets/x/state-at", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestListAssetSnapshots_RespectsLimit(t *testing.T) {
	assetID := uuid.New()
	tenantID := uuid.New()
	var gotLimit int32
	svc := &mockAssetService{
		listSnapshotsFn: func(_ context.Context, _, _ uuid.UUID, limit int32) ([]dbgen.AssetSnapshot, error) {
			gotLimit = limit
			return []dbgen.AssetSnapshot{
				stubSnapshot(uuid.New(), assetID, tenantID, time.Now(), "srv", "operational"),
			}, nil
		},
	}
	s := newAssetsTestServer(svc)

	limit := 5
	rec := runHandler(t,
		func(c *gin.Context) {
			s.ListAssetSnapshots(c, IdPath(assetID), ListAssetSnapshotsParams{Limit: &limit})
		},
		http.MethodGet, "/assets/x/snapshots?limit=5", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gotLimit != 5 {
		t.Errorf("limit not propagated: got %d, want 5", gotLimit)
	}
}

func TestDiffAssetState_RejectsInvertedRange(t *testing.T) {
	s := newAssetsTestServer(&mockAssetService{})
	now := time.Now()
	rec := runHandler(t,
		func(c *gin.Context) {
			s.DiffAssetState(c, IdPath(uuid.New()),
				DiffAssetStateParams{From: now, To: now.Add(-time.Hour)})
		},
		http.MethodGet, "/assets/x/diff", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestDiffAssetState_NotFoundMapsToHTTP404(t *testing.T) {
	svc := &mockAssetService{
		diffStateAtFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time) (*asset.DiffResult, error) {
			return nil, asset.ErrSnapshotNotFound
		},
	}
	s := newAssetsTestServer(svc)

	from := time.Now().Add(-time.Hour)
	to := time.Now()
	rec := runHandler(t,
		func(c *gin.Context) {
			s.DiffAssetState(c, IdPath(uuid.New()),
				DiffAssetStateParams{From: from, To: to})
		},
		http.MethodGet, "/assets/x/diff", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestDiffAssetState_ReturnsChangesAndAnchors(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	fromAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	toAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	svc := &mockAssetService{
		diffStateAtFn: func(_ context.Context, _, _ uuid.UUID, f, tt time.Time) (*asset.DiffResult, error) {
			return &asset.DiffResult{
				From: dbgen.AssetSnapshot{ValidAt: f, Name: "old", Attributes: json.RawMessage(`{"cpu":"intel"}`)},
				To:   dbgen.AssetSnapshot{ValidAt: tt, Name: "new", Attributes: json.RawMessage(`{"cpu":"amd"}`)},
				Changes: []asset.FieldChange{
					{Field: "name", From: "old", To: "new"},
					{Field: "attributes", From: []byte(`{"cpu":"intel"}`), To: []byte(`{"cpu":"amd"}`)},
				},
			}, nil
		},
	}
	s := newAssetsTestServer(svc)

	rec := runHandler(t,
		func(c *gin.Context) {
			s.DiffAssetState(c, IdPath(assetID),
				DiffAssetStateParams{From: fromAt, To: toAt})
		},
		http.MethodGet, "/assets/x/diff", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data AssetDiff `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Data.AssetId != assetID {
		t.Errorf("asset id = %v, want %v", body.Data.AssetId, assetID)
	}
	if !body.Data.FromAt.Equal(fromAt) || !body.Data.ToAt.Equal(toAt) {
		t.Errorf("anchors not propagated: %+v", body.Data)
	}
	if len(body.Data.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(body.Data.Changes))
	}
	// attributes change must be a decoded JSON object, not a base64 blob.
	attr := body.Data.Changes[1]
	fromMap, ok := attr.From.(map[string]any)
	if !ok {
		t.Fatalf("attributes.from should be a JSON object, got %T: %v", attr.From, attr.From)
	}
	if fromMap["cpu"] != "intel" {
		t.Errorf("from.cpu = %v, want intel", fromMap["cpu"])
	}
}

func TestListAssetSnapshots_DefaultLimitIs100(t *testing.T) {
	var gotLimit int32
	svc := &mockAssetService{
		listSnapshotsFn: func(_ context.Context, _, _ uuid.UUID, limit int32) ([]dbgen.AssetSnapshot, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	s := newAssetsTestServer(svc)

	rec := runHandler(t,
		func(c *gin.Context) {
			s.ListAssetSnapshots(c, IdPath(uuid.New()), ListAssetSnapshotsParams{Limit: nil})
		},
		http.MethodGet, "/assets/x/snapshots", nil, nil,
		map[string]string{"tenant_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gotLimit != 100 {
		t.Errorf("default limit should be 100, got %d", gotLimit)
	}
}
