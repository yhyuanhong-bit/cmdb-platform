package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockDiscoveryService implements discoveryService for handler-level tests.
// Only the fields actually exercised by a test need to be populated.
type mockDiscoveryService struct {
	listFn      func(ctx context.Context, tenantID uuid.UUID, status *string, limit, offset int32) ([]dbgen.DiscoveredAsset, int64, error)
	ingestFn    func(ctx context.Context, params dbgen.CreateDiscoveredAssetParams) (*dbgen.DiscoveredAsset, error)
	approveFn   func(ctx context.Context, discoveredID, tenantID, reviewerID uuid.UUID) (*discovery.ApproveResult, error)
	ignoreFn    func(ctx context.Context, id, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error)
	getStatsFn  func(ctx context.Context, tenantID uuid.UUID) (*dbgen.GetDiscoveryStatsRow, error)
	queriesStub *dbgen.Queries
}

func (m *mockDiscoveryService) List(ctx context.Context, tenantID uuid.UUID, status *string, limit, offset int32) ([]dbgen.DiscoveredAsset, int64, error) {
	return m.listFn(ctx, tenantID, status, limit, offset)
}
func (m *mockDiscoveryService) Ingest(ctx context.Context, params dbgen.CreateDiscoveredAssetParams) (*dbgen.DiscoveredAsset, error) {
	return m.ingestFn(ctx, params)
}
func (m *mockDiscoveryService) ApproveAndCreateAsset(ctx context.Context, discoveredID, tenantID, reviewerID uuid.UUID) (*discovery.ApproveResult, error) {
	return m.approveFn(ctx, discoveredID, tenantID, reviewerID)
}
func (m *mockDiscoveryService) Ignore(ctx context.Context, id, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error) {
	return m.ignoreFn(ctx, id, reviewerID)
}
func (m *mockDiscoveryService) GetStats(ctx context.Context, tenantID uuid.UUID) (*dbgen.GetDiscoveryStatsRow, error) {
	return m.getStatsFn(ctx, tenantID)
}
func (m *mockDiscoveryService) Queries() *dbgen.Queries {
	return m.queriesStub
}

// recordingBus captures every event published so tests can assert on subject
// and payload shape without spinning up NATS.
type recordingBus struct {
	events []eventbus.Event
}

func (b *recordingBus) Publish(ctx context.Context, ev eventbus.Event) error {
	b.events = append(b.events, ev)
	return nil
}
func (b *recordingBus) Subscribe(subject string, handler eventbus.Handler) error {
	return nil
}
func (b *recordingBus) Close() error { return nil }

// ---------------------------------------------------------------------------
// ApproveDiscoveredAsset — unit (mocked service)
// ---------------------------------------------------------------------------

// TestApproveDiscoveredAsset_Success_CreatesAssetAndPublishesEvent verifies
// the happy path: the handler forwards (discovered_id, tenant, user) to the
// service and, on a Created=true result, publishes exactly one
// asset.created event with the expected payload shape.
func TestApproveDiscoveredAsset_Success_CreatesAssetAndPublishesEvent(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	discoveredID := uuid.New()
	assetID := uuid.New()

	var capturedDisc, capturedTenant, capturedReviewer uuid.UUID
	svc := &mockDiscoveryService{
		approveFn: func(_ context.Context, d, t, r uuid.UUID) (*discovery.ApproveResult, error) {
			capturedDisc, capturedTenant, capturedReviewer = d, t, r
			return &discovery.ApproveResult{
				Asset: dbgen.Asset{
					ID:       assetID,
					TenantID: tenantID,
					AssetTag: "DSC-ABCDEF01",
					Name:     "host1",
					Type:     "server",
					Status:   "inventoried",
				},
				Discovered: dbgen.DiscoveredAsset{
					ID:       discoveredID,
					TenantID: tenantID,
					Status:   "approved",
					Source:   "test",
				},
				Created: true,
			}, nil
		},
	}
	bus := &recordingBus{}
	s := &APIServer{discoverySvc: svc, eventBus: bus}

	rec := runHandler(t, func(c *gin.Context) { s.ApproveDiscoveredAsset(c, IdPath(discoveredID)) },
		http.MethodPost, "/discovery/"+discoveredID.String()+"/approve", nil, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": userID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	if capturedDisc != discoveredID {
		t.Errorf("discovered_id not propagated: got %v want %v", capturedDisc, discoveredID)
	}
	if capturedTenant != tenantID {
		t.Errorf("tenant_id not propagated: got %v want %v", capturedTenant, tenantID)
	}
	if capturedReviewer != userID {
		t.Errorf("user_id not propagated: got %v want %v", capturedReviewer, userID)
	}
	if len(bus.events) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(bus.events))
	}
	ev := bus.events[0]
	if ev.Subject != eventbus.SubjectAssetCreated {
		t.Errorf("subject = %q, want %q", ev.Subject, eventbus.SubjectAssetCreated)
	}
	if ev.TenantID != tenantID.String() {
		t.Errorf("event tenant = %q, want %q", ev.TenantID, tenantID.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatalf("event payload not JSON: %v", err)
	}
	if payload["asset_id"] != assetID.String() {
		t.Errorf("payload asset_id = %v, want %s", payload["asset_id"], assetID.String())
	}
	if payload["source"] != "discovery" {
		t.Errorf("payload source = %v, want 'discovery'", payload["source"])
	}
}

// TestApproveDiscoveredAsset_Idempotent_NoEventPublished verifies that when
// the service reports Created=false (idempotent retry), the handler does
// NOT republish the asset.created event. Subscribers must never see a
// duplicate create event for the same asset.
func TestApproveDiscoveredAsset_Idempotent_NoEventPublished(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	discoveredID := uuid.New()
	assetID := uuid.New()

	svc := &mockDiscoveryService{
		approveFn: func(_ context.Context, _, _, _ uuid.UUID) (*discovery.ApproveResult, error) {
			return &discovery.ApproveResult{
				Asset:      dbgen.Asset{ID: assetID, TenantID: tenantID, AssetTag: "DSC-ABCDEF01", Type: "server", Status: "inventoried"},
				Discovered: dbgen.DiscoveredAsset{ID: discoveredID, TenantID: tenantID, Status: "approved"},
				Created:    false, // idempotent path
			}, nil
		},
	}
	bus := &recordingBus{}
	s := &APIServer{discoverySvc: svc, eventBus: bus}

	rec := runHandler(t, func(c *gin.Context) { s.ApproveDiscoveredAsset(c, IdPath(discoveredID)) },
		http.MethodPost, "/discovery/"+discoveredID.String()+"/approve", nil, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": userID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(bus.events) != 0 {
		t.Fatalf("expected 0 events (idempotent path), got %d: %+v", len(bus.events), bus.events)
	}
}

// TestApproveDiscoveredAsset_NotFound_Returns404 verifies that the service's
// ErrNotFound maps to 404. This is how cross-tenant approval attempts are
// signalled — we never return 403, which would leak existence.
func TestApproveDiscoveredAsset_NotFound_Returns404(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	discoveredID := uuid.New()

	svc := &mockDiscoveryService{
		approveFn: func(_ context.Context, _, _, _ uuid.UUID) (*discovery.ApproveResult, error) {
			return nil, discovery.ErrNotFound
		},
	}
	bus := &recordingBus{}
	s := &APIServer{discoverySvc: svc, eventBus: bus}

	rec := runHandler(t, func(c *gin.Context) { s.ApproveDiscoveredAsset(c, IdPath(discoveredID)) },
		http.MethodPost, "/discovery/"+discoveredID.String()+"/approve", nil, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": userID.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — body=%s", rec.Code, rec.Body.String())
	}
	if len(bus.events) != 0 {
		t.Fatalf("expected 0 events on not-found, got %d", len(bus.events))
	}
}

// TestApproveDiscoveredAsset_DuplicateAsset_Returns409 verifies that the
// service's ErrAssetAlreadyExists maps to 409 ASSET_ALREADY_EXISTS. No
// event is published because no asset was created.
func TestApproveDiscoveredAsset_DuplicateAsset_Returns409(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	discoveredID := uuid.New()

	svc := &mockDiscoveryService{
		approveFn: func(_ context.Context, _, _, _ uuid.UUID) (*discovery.ApproveResult, error) {
			return nil, discovery.ErrAssetAlreadyExists
		},
	}
	bus := &recordingBus{}
	s := &APIServer{discoverySvc: svc, eventBus: bus}

	rec := runHandler(t, func(c *gin.Context) { s.ApproveDiscoveredAsset(c, IdPath(discoveredID)) },
		http.MethodPost, "/discovery/"+discoveredID.String()+"/approve", nil, nil,
		map[string]string{"tenant_id": tenantID.String(), "user_id": userID.String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 — body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if errObj, _ := body["error"].(map[string]any); errObj["code"] != "ASSET_ALREADY_EXISTS" {
		t.Errorf("error code = %v, want ASSET_ALREADY_EXISTS", errObj["code"])
	}
	if len(bus.events) != 0 {
		t.Fatalf("expected 0 events on duplicate, got %d", len(bus.events))
	}
}
