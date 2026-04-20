package prediction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// -----------------------------------------------------------------------------
// NOTE: The fake provider below satisfies the ai.AIProvider interface with only
// the methods RCA needs. PredictFailure was removed in Phase 2.12 (YAGNI).
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Fakes
// -----------------------------------------------------------------------------

// fakeProvider implements ai.AIProvider and captures the RCA request it
// receives so tests can make assertions about the data the service passed in.
type fakeProvider struct {
	name    string
	typ     string
	lastRCA ai.RCARequest
	calls   int
	err     error
	result  *ai.RCAResult
}

func newFakeProvider(name string) *fakeProvider {
	return &fakeProvider{
		name:   name,
		typ:    "llm",
		result: &ai.RCAResult{Reasoning: json.RawMessage(`{"ok":true}`), Confidence: 0.5},
	}
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Type() string { return f.typ }

func (f *fakeProvider) AnalyzeRootCause(_ context.Context, req ai.RCARequest) (*ai.RCAResult, error) {
	f.calls++
	f.lastRCA = req
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeProvider) HealthCheck(context.Context) error { return nil }

// fakeQueries is a minimal in-memory stand-in for the dbgen.Queries subset the
// prediction service uses. It simulates tenant scoping by refusing to return
// rows whose tenant does not match the request.
type fakeQueries struct {
	alertsByTenantAndIncident map[tenantIncidentKey][]dbgen.ListAlertEventsByIncidentRow
	assetsByTenantAndIncident map[tenantIncidentKey][]dbgen.Asset

	lastCreateRCA         dbgen.CreateRCAParams
	lastListAlertsParams  dbgen.ListAlertEventsByIncidentParams
	lastListAssetsParams  dbgen.ListAssetsForIncidentParams
	createRCAErr          error
	listAlertsErr         error
	listAssetsErr         error
}

type tenantIncidentKey struct {
	tenant   uuid.UUID
	incident uuid.UUID
}

func newFakeQueries() *fakeQueries {
	return &fakeQueries{
		alertsByTenantAndIncident: map[tenantIncidentKey][]dbgen.ListAlertEventsByIncidentRow{},
		assetsByTenantAndIncident: map[tenantIncidentKey][]dbgen.Asset{},
	}
}

func (f *fakeQueries) ListAllModels(context.Context) ([]dbgen.PredictionModel, error) {
	return nil, nil
}

func (f *fakeQueries) CreateRCA(_ context.Context, arg dbgen.CreateRCAParams) (dbgen.RcaAnalysis, error) {
	f.lastCreateRCA = arg
	if f.createRCAErr != nil {
		return dbgen.RcaAnalysis{}, f.createRCAErr
	}
	return dbgen.RcaAnalysis{
		ID:         uuid.New(),
		TenantID:   arg.TenantID,
		IncidentID: arg.IncidentID,
		Reasoning:  arg.Reasoning,
		CreatedAt:  time.Now(),
	}, nil
}

func (f *fakeQueries) VerifyRCA(context.Context, dbgen.VerifyRCAParams) (dbgen.RcaAnalysis, error) {
	return dbgen.RcaAnalysis{}, nil
}

func (f *fakeQueries) ListAlertEventsByIncident(_ context.Context, arg dbgen.ListAlertEventsByIncidentParams) ([]dbgen.ListAlertEventsByIncidentRow, error) {
	f.lastListAlertsParams = arg
	if f.listAlertsErr != nil {
		return nil, f.listAlertsErr
	}
	// Simulates SQL WHERE tenant_id = $1. Only returns rows stored under the
	// exact (tenant, incident) pair — mirroring real tenant isolation.
	return f.alertsByTenantAndIncident[tenantIncidentKey{arg.TenantID, arg.IncidentID}], nil
}

func (f *fakeQueries) ListAssetsForIncident(_ context.Context, arg dbgen.ListAssetsForIncidentParams) ([]dbgen.Asset, error) {
	f.lastListAssetsParams = arg
	if f.listAssetsErr != nil {
		return nil, f.listAssetsErr
	}
	return f.assetsByTenantAndIncident[tenantIncidentKey{arg.TenantID, arg.IncidentID}], nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func makeAlertRow(alertID, assetID uuid.UUID, severity, message string, firedAt time.Time) dbgen.ListAlertEventsByIncidentRow {
	return dbgen.ListAlertEventsByIncidentRow{
		ID:       alertID,
		TenantID: uuid.Nil, // not checked in mapper
		AssetID:  pgtype.UUID{Bytes: assetID, Valid: true},
		Status:   "firing",
		Severity: severity,
		Message:  pgtype.Text{String: message, Valid: true},
		FiredAt:  firedAt,
	}
}

func makeAsset(id uuid.UUID, name, typ, status string) dbgen.Asset {
	return dbgen.Asset{
		ID:     id,
		Name:   name,
		Type:   typ,
		Status: status,
	}
}

func setupService(t *testing.T) (*Service, *fakeQueries, *fakeProvider) {
	t.Helper()
	q := newFakeQueries()
	reg := ai.NewRegistry()
	p := newFakeProvider("Default RCA")
	reg.Register(p)
	return NewService(q, reg), q, p
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestCreateRCA_PopulatesAlertsAndAssets(t *testing.T) {
	svc, q, p := setupService(t)

	tenantID := uuid.New()
	incidentID := uuid.New()
	asset1 := uuid.New()
	asset2 := uuid.New()
	firedAt := time.Now().Add(-5 * time.Minute)

	q.alertsByTenantAndIncident[tenantIncidentKey{tenantID, incidentID}] = []dbgen.ListAlertEventsByIncidentRow{
		makeAlertRow(uuid.New(), asset1, "high", "cpu spiking", firedAt),
		makeAlertRow(uuid.New(), asset1, "high", "memory pressure", firedAt.Add(time.Second)),
		makeAlertRow(uuid.New(), asset2, "critical", "disk full", firedAt.Add(2*time.Second)),
	}
	q.assetsByTenantAndIncident[tenantIncidentKey{tenantID, incidentID}] = []dbgen.Asset{
		makeAsset(asset1, "web-01", "server", "degraded"),
		makeAsset(asset2, "db-01", "database", "down"),
	}

	_, err := svc.CreateRCA(context.Background(), tenantID, CreateRCARequest{
		IncidentID: incidentID,
		ModelName:  "Default RCA",
		Context:    "user context",
	})
	if err != nil {
		t.Fatalf("CreateRCA: %v", err)
	}

	// Query params must carry the context-tenant, not something from elsewhere.
	if q.lastListAlertsParams.TenantID != tenantID {
		t.Errorf("alerts query tenant_id = %v, want %v", q.lastListAlertsParams.TenantID, tenantID)
	}
	if q.lastListAlertsParams.IncidentID != incidentID {
		t.Errorf("alerts query incident_id = %v, want %v", q.lastListAlertsParams.IncidentID, incidentID)
	}
	if q.lastListAssetsParams.TenantID != tenantID {
		t.Errorf("assets query tenant_id = %v, want %v", q.lastListAssetsParams.TenantID, tenantID)
	}

	if p.calls != 1 {
		t.Fatalf("provider.AnalyzeRootCause called %d times, want 1", p.calls)
	}
	if p.lastRCA.TenantID != tenantID {
		t.Errorf("RCA request tenant_id = %v, want %v", p.lastRCA.TenantID, tenantID)
	}
	if p.lastRCA.IncidentID != incidentID {
		t.Errorf("RCA request incident_id = %v, want %v", p.lastRCA.IncidentID, incidentID)
	}
	if got := len(p.lastRCA.RelatedAlerts); got != 3 {
		t.Errorf("RCA request RelatedAlerts len = %d, want 3", got)
	}
	if got := len(p.lastRCA.AffectedAssets); got != 2 {
		t.Errorf("RCA request AffectedAssets len = %d, want 2", got)
	}
	if p.lastRCA.Context != "user context" {
		t.Errorf("RCA request Context = %q, want %q", p.lastRCA.Context, "user context")
	}

	// Spot-check content passed through the mapper.
	if p.lastRCA.RelatedAlerts[0].Severity != "high" {
		t.Errorf("alert[0].Severity = %q, want high", p.lastRCA.RelatedAlerts[0].Severity)
	}
	if p.lastRCA.AffectedAssets[0].Name == "" {
		t.Errorf("asset[0].Name is empty, want non-empty")
	}
}

func TestCreateRCA_EmptyAlertsAndAssetsStillSucceed(t *testing.T) {
	svc, q, p := setupService(t)

	tenantID := uuid.New()
	incidentID := uuid.New()
	// Intentionally populate nothing for this (tenant, incident) pair.
	_ = q

	rca, err := svc.CreateRCA(context.Background(), tenantID, CreateRCARequest{
		IncidentID: incidentID,
		ModelName:  "Default RCA",
	})
	if err != nil {
		t.Fatalf("CreateRCA: %v", err)
	}
	if rca == nil {
		t.Fatalf("CreateRCA returned nil rca")
	}
	if p.calls != 1 {
		t.Fatalf("provider.AnalyzeRootCause called %d times, want 1", p.calls)
	}
	if p.lastRCA.RelatedAlerts == nil {
		t.Errorf("RelatedAlerts should be a non-nil empty slice")
	}
	if len(p.lastRCA.RelatedAlerts) != 0 {
		t.Errorf("RelatedAlerts len = %d, want 0", len(p.lastRCA.RelatedAlerts))
	}
	if len(p.lastRCA.AffectedAssets) != 0 {
		t.Errorf("AffectedAssets len = %d, want 0", len(p.lastRCA.AffectedAssets))
	}
}

func TestCreateRCA_CrossTenantIsolation(t *testing.T) {
	svc, q, p := setupService(t)

	tenantA := uuid.New()
	tenantB := uuid.New()
	incidentID := uuid.New()
	assetOther := uuid.New()

	// Data owned by tenant B that happens to share the same incident UUID.
	// A correctly written SQL query scopes by tenant_id, so a caller acting
	// as tenant A MUST NOT see any of this.
	q.alertsByTenantAndIncident[tenantIncidentKey{tenantB, incidentID}] = []dbgen.ListAlertEventsByIncidentRow{
		makeAlertRow(uuid.New(), assetOther, "critical", "leaked alert", time.Now()),
	}
	q.assetsByTenantAndIncident[tenantIncidentKey{tenantB, incidentID}] = []dbgen.Asset{
		makeAsset(assetOther, "tenant-b-secret", "server", "up"),
	}

	_, err := svc.CreateRCA(context.Background(), tenantA, CreateRCARequest{
		IncidentID: incidentID,
		ModelName:  "Default RCA",
	})
	if err != nil {
		t.Fatalf("CreateRCA: %v", err)
	}

	if q.lastListAlertsParams.TenantID != tenantA {
		t.Errorf("alerts query tenant_id leaked: got %v, want %v", q.lastListAlertsParams.TenantID, tenantA)
	}
	if q.lastListAssetsParams.TenantID != tenantA {
		t.Errorf("assets query tenant_id leaked: got %v, want %v", q.lastListAssetsParams.TenantID, tenantA)
	}
	if len(p.lastRCA.RelatedAlerts) != 0 {
		t.Errorf("cross-tenant alert leaked into RCA request: %d rows", len(p.lastRCA.RelatedAlerts))
	}
	if len(p.lastRCA.AffectedAssets) != 0 {
		t.Errorf("cross-tenant asset leaked into RCA request: %d rows", len(p.lastRCA.AffectedAssets))
	}
}

func TestCreateRCA_NoProviderStoresPlaceholder(t *testing.T) {
	q := newFakeQueries()
	svc := NewService(q, ai.NewRegistry()) // empty registry → placeholder path

	tenantID := uuid.New()
	incidentID := uuid.New()

	rca, err := svc.CreateRCA(context.Background(), tenantID, CreateRCARequest{
		IncidentID: incidentID,
	})
	if err != nil {
		t.Fatalf("CreateRCA: %v", err)
	}
	if rca == nil {
		t.Fatalf("expected placeholder RCA, got nil")
	}
	if q.lastCreateRCA.TenantID != tenantID {
		t.Errorf("placeholder CreateRCA tenant_id = %v, want %v", q.lastCreateRCA.TenantID, tenantID)
	}
	// Placeholder path must NOT call the list queries — there is no provider
	// to feed, so the extra DB roundtrips would be wasted.
	if (q.lastListAlertsParams != dbgen.ListAlertEventsByIncidentParams{}) {
		t.Errorf("placeholder path unexpectedly called ListAlertEventsByIncident")
	}
	if (q.lastListAssetsParams != dbgen.ListAssetsForIncidentParams{}) {
		t.Errorf("placeholder path unexpectedly called ListAssetsForIncident")
	}
}

func TestCreateRCA_AlertsQueryErrorPropagates(t *testing.T) {
	svc, q, _ := setupService(t)
	q.listAlertsErr = errors.New("db down")

	_, err := svc.CreateRCA(context.Background(), uuid.New(), CreateRCARequest{
		IncidentID: uuid.New(),
		ModelName:  "Default RCA",
	})
	if err == nil {
		t.Fatal("expected error from alerts query, got nil")
	}
}

func TestMapAlertsForRCA_HandlesNullFields(t *testing.T) {
	row := dbgen.ListAlertEventsByIncidentRow{
		ID:       uuid.New(),
		Severity: "warning",
		FiredAt:  time.Now(),
		AssetID:  pgtype.UUID{Valid: false},
		Message:  pgtype.Text{Valid: false},
	}
	out := mapAlertsForRCA([]dbgen.ListAlertEventsByIncidentRow{row})
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1", len(out))
	}
	if out[0].Message != "" {
		t.Errorf("Message = %q, want empty", out[0].Message)
	}
	if out[0].AssetID != uuid.Nil {
		t.Errorf("AssetID = %v, want zero UUID", out[0].AssetID)
	}
}
