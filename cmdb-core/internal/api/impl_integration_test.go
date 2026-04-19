package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// mock integrationService
// ---------------------------------------------------------------------------

// mockIntegrationService implements integrationService for handler tests.
// Each *Fn field may be nil; calling through a nil hook fails the test so
// that accidental dependencies show up immediately.
type mockIntegrationService struct {
	listAdaptersFn   func(ctx context.Context, tenantID uuid.UUID) ([]dbgen.IntegrationAdapter, error)
	getAdapterFn     func(ctx context.Context, id, tenantID uuid.UUID) (dbgen.IntegrationAdapter, error)
	createAdapterFn  func(ctx context.Context, params dbgen.CreateAdapterParams) (dbgen.IntegrationAdapter, error)
	updateAdapterFn  func(ctx context.Context, params dbgen.UpdateAdapterParams) (dbgen.IntegrationAdapter, error)
	deleteAdapterFn  func(ctx context.Context, id, tenantID uuid.UUID) error
	listWebhooksFn   func(ctx context.Context, tenantID uuid.UUID) ([]dbgen.WebhookSubscription, error)
	getWebhookFn     func(ctx context.Context, id, tenantID uuid.UUID) (dbgen.WebhookSubscription, error)
	createWebhookFn  func(ctx context.Context, params dbgen.CreateWebhookParams) (dbgen.WebhookSubscription, error)
	updateWebhookFn  func(ctx context.Context, params dbgen.UpdateWebhookParams) (dbgen.WebhookSubscription, error)
	deleteWebhookFn  func(ctx context.Context, id, tenantID uuid.UUID) error
	listDeliveriesFn func(ctx context.Context, webhookID uuid.UUID, limit int) ([]dbgen.WebhookDelivery, error)
}

func (m *mockIntegrationService) ListAdapters(ctx context.Context, tenantID uuid.UUID) ([]dbgen.IntegrationAdapter, error) {
	if m.listAdaptersFn == nil {
		return nil, errors.New("mock: ListAdapters not implemented")
	}
	return m.listAdaptersFn(ctx, tenantID)
}
func (m *mockIntegrationService) GetAdapterByID(ctx context.Context, id, tenantID uuid.UUID) (dbgen.IntegrationAdapter, error) {
	if m.getAdapterFn == nil {
		return dbgen.IntegrationAdapter{}, errors.New("mock: GetAdapterByID not implemented")
	}
	return m.getAdapterFn(ctx, id, tenantID)
}
func (m *mockIntegrationService) CreateAdapter(ctx context.Context, params dbgen.CreateAdapterParams) (dbgen.IntegrationAdapter, error) {
	if m.createAdapterFn == nil {
		return dbgen.IntegrationAdapter{}, errors.New("mock: CreateAdapter not implemented")
	}
	return m.createAdapterFn(ctx, params)
}
func (m *mockIntegrationService) UpdateAdapter(ctx context.Context, params dbgen.UpdateAdapterParams) (dbgen.IntegrationAdapter, error) {
	if m.updateAdapterFn == nil {
		return dbgen.IntegrationAdapter{}, errors.New("mock: UpdateAdapter not implemented")
	}
	return m.updateAdapterFn(ctx, params)
}
func (m *mockIntegrationService) DeleteAdapter(ctx context.Context, id, tenantID uuid.UUID) error {
	if m.deleteAdapterFn == nil {
		return errors.New("mock: DeleteAdapter not implemented")
	}
	return m.deleteAdapterFn(ctx, id, tenantID)
}
func (m *mockIntegrationService) ListWebhooks(ctx context.Context, tenantID uuid.UUID) ([]dbgen.WebhookSubscription, error) {
	if m.listWebhooksFn == nil {
		return nil, errors.New("mock: ListWebhooks not implemented")
	}
	return m.listWebhooksFn(ctx, tenantID)
}
func (m *mockIntegrationService) GetWebhookByID(ctx context.Context, id, tenantID uuid.UUID) (dbgen.WebhookSubscription, error) {
	if m.getWebhookFn == nil {
		return dbgen.WebhookSubscription{}, errors.New("mock: GetWebhookByID not implemented")
	}
	return m.getWebhookFn(ctx, id, tenantID)
}
func (m *mockIntegrationService) CreateWebhook(ctx context.Context, params dbgen.CreateWebhookParams) (dbgen.WebhookSubscription, error) {
	if m.createWebhookFn == nil {
		return dbgen.WebhookSubscription{}, errors.New("mock: CreateWebhook not implemented")
	}
	return m.createWebhookFn(ctx, params)
}
func (m *mockIntegrationService) UpdateWebhook(ctx context.Context, params dbgen.UpdateWebhookParams) (dbgen.WebhookSubscription, error) {
	if m.updateWebhookFn == nil {
		return dbgen.WebhookSubscription{}, errors.New("mock: UpdateWebhook not implemented")
	}
	return m.updateWebhookFn(ctx, params)
}
func (m *mockIntegrationService) DeleteWebhook(ctx context.Context, id, tenantID uuid.UUID) error {
	if m.deleteWebhookFn == nil {
		return errors.New("mock: DeleteWebhook not implemented")
	}
	return m.deleteWebhookFn(ctx, id, tenantID)
}
func (m *mockIntegrationService) ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]dbgen.WebhookDelivery, error) {
	if m.listDeliveriesFn == nil {
		return nil, errors.New("mock: ListDeliveries not implemented")
	}
	return m.listDeliveriesFn(ctx, webhookID, limit)
}

// ---------------------------------------------------------------------------
// mock auditService (records calls so success-path tests can assert on them)
// ---------------------------------------------------------------------------

type mockAuditService struct {
	records []auditRecord
	queryFn func(ctx context.Context, tenantID uuid.UUID, module, targetType *string, targetID *uuid.UUID, limit, offset int32) ([]dbgen.AuditEvent, int64, error)
}

type auditRecord struct {
	Action     string
	Module     string
	TargetType string
	TargetID   uuid.UUID
	Diff       map[string]any
}

func (m *mockAuditService) Record(_ context.Context, _ uuid.UUID, action, module, targetType string, targetID, _ uuid.UUID, diff map[string]any, _ string) error {
	m.records = append(m.records, auditRecord{
		Action:     action,
		Module:     module,
		TargetType: targetType,
		TargetID:   targetID,
		Diff:       diff,
	})
	return nil
}

func (m *mockAuditService) Query(ctx context.Context, tenantID uuid.UUID, module, targetType *string, targetID *uuid.UUID, limit, offset int32) ([]dbgen.AuditEvent, int64, error) {
	if m.queryFn == nil {
		return nil, 0, nil
	}
	return m.queryFn(ctx, tenantID, module, targetType, targetID, limit, offset)
}

// ---------------------------------------------------------------------------
// mock Cipher — deterministic so tests can assert on the encrypted output
// ---------------------------------------------------------------------------

type mockCipher struct{}

func (mockCipher) Encrypt(plaintext []byte) ([]byte, error) {
	// Prefix so encrypted bytes are visually distinct from plaintext in
	// diagnostics, and different from the plaintext so ciphertext == input
	// assertions don't accidentally pass.
	out := make([]byte, 0, len(plaintext)+4)
	out = append(out, []byte("ENC:")...)
	out = append(out, plaintext...)
	return out, nil
}

func (mockCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 4 || string(ciphertext[:4]) != "ENC:" {
		return nil, errors.New("mock: not encrypted")
	}
	return ciphertext[4:], nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newIntegrationTestServer(svc *mockIntegrationService, audit *mockAuditService) *APIServer {
	// Tests default to no netGuard (nil) so unrelated suites can continue to
	// use plain example.com URLs without DNS roundtrips. Suites that
	// exercise SSRF behaviour install a real guard via the returned server.
	return &APIServer{
		integrationSvc: svc,
		auditSvc:       audit,
		cipher:         mockCipher{},
	}
}

// newIntegrationTestServerWithGuard builds a server wired to a real
// netguard for SSRF-specific tests.
func newIntegrationTestServerWithGuard(t *testing.T, svc *mockIntegrationService, audit *mockAuditService) *APIServer {
	t.Helper()
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	return &APIServer{
		integrationSvc: svc,
		auditSvc:       audit,
		cipher:         mockCipher{},
		netGuard:       g,
	}
}

func stubAdapter(id, tenantID uuid.UUID, name string) dbgen.IntegrationAdapter {
	return dbgen.IntegrationAdapter{
		ID:              id,
		TenantID:        tenantID,
		Name:            name,
		Type:            "rest",
		Direction:       "inbound",
		Config:          []byte(`{"token":"old"}`),
		ConfigEncrypted: []byte("ENC:{\"token\":\"old\"}"),
		Enabled:         pgtype.Bool{Bool: true, Valid: true},
	}
}

func stubWebhook(id, tenantID uuid.UUID, name string) dbgen.WebhookSubscription {
	return dbgen.WebhookSubscription{
		ID:              id,
		TenantID:        tenantID,
		Name:            name,
		Url:             "https://example.com/hook",
		Secret:          pgtype.Text{String: "old-secret", Valid: true},
		SecretEncrypted: []byte("ENC:old-secret"),
		Events:          []string{"alert.fired"},
		Enabled:         pgtype.Bool{Bool: true, Valid: true},
	}
}

func idParams(id uuid.UUID) gin.Params {
	return gin.Params{{Key: "id", Value: id.String()}}
}

// ---------------------------------------------------------------------------
// UpdateAdapter
// ---------------------------------------------------------------------------

func TestUpdateAdapter_Success(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()
	newName := "renamed"
	enabled := false

	var capturedParams dbgen.UpdateAdapterParams
	svc := &mockIntegrationService{
		getAdapterFn: func(_ context.Context, id, tID uuid.UUID) (dbgen.IntegrationAdapter, error) {
			if id != adapterID || tID != tenantID {
				t.Errorf("ownership check mismatch: got id=%v tenant=%v", id, tID)
			}
			return stubAdapter(adapterID, tenantID, "original"), nil
		},
		updateAdapterFn: func(_ context.Context, p dbgen.UpdateAdapterParams) (dbgen.IntegrationAdapter, error) {
			capturedParams = p
			updated := stubAdapter(adapterID, tenantID, newName)
			updated.Enabled = pgtype.Bool{Bool: enabled, Valid: true}
			return updated, nil
		},
	}
	audit := &mockAuditService{}
	s := newIntegrationTestServer(svc, audit)

	body := UpdateAdapterJSONRequestBody{
		Name:    &newName,
		Enabled: &enabled,
	}
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateAdapter(c, IdPath(adapterID))
	}, http.MethodPatch, "/integration/adapters/"+adapterID.String(), body,
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if capturedParams.ID != adapterID || capturedParams.TenantID != tenantID {
		t.Errorf("tenant scoping not propagated: %+v", capturedParams)
	}
	if !capturedParams.Name.Valid || capturedParams.Name.String != newName {
		t.Errorf("name not applied: %+v", capturedParams.Name)
	}
	if !capturedParams.Enabled.Valid || capturedParams.Enabled.Bool != enabled {
		t.Errorf("enabled not applied: %+v", capturedParams.Enabled)
	}
	// Unspecified fields must stay NULL so COALESCE preserves them.
	if capturedParams.Type.Valid {
		t.Errorf("type should be NULL, got %+v", capturedParams.Type)
	}
	if capturedParams.Config != nil || capturedParams.ConfigEncrypted != nil {
		t.Errorf("config fields should be nil when not supplied")
	}
	if len(audit.records) != 1 || audit.records[0].Action != "adapter.updated" {
		t.Errorf("expected one adapter.updated audit event, got %+v", audit.records)
	}
}

func TestUpdateAdapter_ConfigDualWrite(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()

	var capturedParams dbgen.UpdateAdapterParams
	svc := &mockIntegrationService{
		getAdapterFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.IntegrationAdapter, error) {
			return stubAdapter(adapterID, tenantID, "original"), nil
		},
		updateAdapterFn: func(_ context.Context, p dbgen.UpdateAdapterParams) (dbgen.IntegrationAdapter, error) {
			capturedParams = p
			return stubAdapter(adapterID, tenantID, "original"), nil
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	newConfig := map[string]interface{}{"token": "new-value", "host": "api.example.com"}
	body := UpdateAdapterJSONRequestBody{Config: &newConfig}

	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateAdapter(c, IdPath(adapterID))
	}, http.MethodPatch, "/integration/adapters/"+adapterID.String(), body,
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if capturedParams.Config == nil {
		t.Fatal("Config must be written when caller supplies it")
	}
	if capturedParams.ConfigEncrypted == nil {
		t.Fatal("ConfigEncrypted must be written when caller supplies config")
	}
	// Plaintext column holds raw JSON bytes
	var roundTrip map[string]interface{}
	if err := json.Unmarshal(capturedParams.Config, &roundTrip); err != nil {
		t.Fatalf("Config not valid JSON: %v", err)
	}
	if roundTrip["token"] != "new-value" {
		t.Errorf("Config mismatch: %+v", roundTrip)
	}
	// Encrypted column uses the cipher mock prefix
	if string(capturedParams.ConfigEncrypted[:4]) != "ENC:" {
		t.Errorf("ConfigEncrypted not encrypted: %q", capturedParams.ConfigEncrypted)
	}
}

func TestUpdateAdapter_CrossTenant_404(t *testing.T) {
	adapterID := uuid.New()
	callerTenant := uuid.New()
	svc := &mockIntegrationService{
		getAdapterFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.IntegrationAdapter, error) {
			// Row exists but belongs to a different tenant — service layer
			// returns an error just like the "not found" case.
			return dbgen.IntegrationAdapter{}, errors.New("get adapter: not found")
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	name := "x"
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateAdapter(c, IdPath(adapterID))
	}, http.MethodPatch, "/integration/adapters/"+adapterID.String(),
		UpdateAdapterJSONRequestBody{Name: &name},
		idParams(adapterID),
		map[string]string{"tenant_id": callerTenant.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateAdapter_UnknownID_404(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		getAdapterFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.IntegrationAdapter, error) {
			return dbgen.IntegrationAdapter{}, errors.New("not found")
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	name := "x"
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateAdapter(c, IdPath(adapterID))
	}, http.MethodPatch, "/integration/adapters/"+adapterID.String(),
		UpdateAdapterJSONRequestBody{Name: &name},
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateAdapter_InvalidJSON_400(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()
	s := newIntegrationTestServer(&mockIntegrationService{}, &mockAuditService{})

	// body = "not-json" — ShouldBindJSON will reject this
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateAdapter(c, IdPath(adapterID))
	}, http.MethodPatch, "/integration/adapters/"+adapterID.String(),
		"not-json",
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DeleteAdapter
// ---------------------------------------------------------------------------

func TestDeleteAdapter_Success(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()

	deleteCalled := false
	svc := &mockIntegrationService{
		getAdapterFn: func(_ context.Context, id, tID uuid.UUID) (dbgen.IntegrationAdapter, error) {
			return stubAdapter(adapterID, tenantID, "doomed"), nil
		},
		deleteAdapterFn: func(_ context.Context, id, tID uuid.UUID) error {
			deleteCalled = true
			if id != adapterID || tID != tenantID {
				t.Errorf("delete tenant scoping mismatch: id=%v tenant=%v", id, tID)
			}
			return nil
		},
	}
	audit := &mockAuditService{}
	s := newIntegrationTestServer(svc, audit)

	rec := runHandler(t, func(c *gin.Context) {
		s.DeleteAdapter(c, IdPath(adapterID))
	}, http.MethodDelete, "/integration/adapters/"+adapterID.String(), nil,
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if !deleteCalled {
		t.Error("DeleteAdapter was not called")
	}
	if len(audit.records) != 1 || audit.records[0].Action != "adapter.deleted" {
		t.Errorf("expected one adapter.deleted audit event, got %+v", audit.records)
	}
	if audit.records[0].Diff["name"] != "doomed" {
		t.Errorf("audit diff should carry pre-delete name: %+v", audit.records[0].Diff)
	}
}

func TestDeleteAdapter_CrossTenant_404(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		getAdapterFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.IntegrationAdapter, error) {
			return dbgen.IntegrationAdapter{}, errors.New("not found")
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	rec := runHandler(t, func(c *gin.Context) {
		s.DeleteAdapter(c, IdPath(adapterID))
	}, http.MethodDelete, "/integration/adapters/"+adapterID.String(), nil,
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateWebhook
// ---------------------------------------------------------------------------

func TestUpdateWebhook_Success(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()
	newName := "renamed-hook"
	enabled := false

	var capturedParams dbgen.UpdateWebhookParams
	svc := &mockIntegrationService{
		getWebhookFn: func(_ context.Context, id, tID uuid.UUID) (dbgen.WebhookSubscription, error) {
			return stubWebhook(webhookID, tenantID, "original"), nil
		},
		updateWebhookFn: func(_ context.Context, p dbgen.UpdateWebhookParams) (dbgen.WebhookSubscription, error) {
			capturedParams = p
			hook := stubWebhook(webhookID, tenantID, newName)
			hook.Enabled = pgtype.Bool{Bool: enabled, Valid: true}
			return hook, nil
		},
	}
	audit := &mockAuditService{}
	s := newIntegrationTestServer(svc, audit)

	body := UpdateWebhookJSONRequestBody{
		Name:    &newName,
		Enabled: &enabled,
	}
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateWebhook(c, IdPath(webhookID))
	}, http.MethodPatch, "/integration/webhooks/"+webhookID.String(), body,
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if capturedParams.ID != webhookID || capturedParams.TenantID != tenantID {
		t.Errorf("tenant scoping not propagated: %+v", capturedParams)
	}
	if !capturedParams.Name.Valid || capturedParams.Name.String != newName {
		t.Errorf("name not applied: %+v", capturedParams.Name)
	}
	// Unspecified fields stay NULL
	if capturedParams.Url.Valid {
		t.Errorf("url should be NULL, got %+v", capturedParams.Url)
	}
	if capturedParams.Secret.Valid || capturedParams.SecretEncrypted != nil {
		t.Errorf("secret fields should be empty when not supplied")
	}
	if len(audit.records) != 1 || audit.records[0].Action != "webhook.updated" {
		t.Errorf("expected one webhook.updated audit event, got %+v", audit.records)
	}
}

func TestUpdateWebhook_SecretDualWrite(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()

	var capturedParams dbgen.UpdateWebhookParams
	svc := &mockIntegrationService{
		getWebhookFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.WebhookSubscription, error) {
			return stubWebhook(webhookID, tenantID, "hook"), nil
		},
		updateWebhookFn: func(_ context.Context, p dbgen.UpdateWebhookParams) (dbgen.WebhookSubscription, error) {
			capturedParams = p
			return stubWebhook(webhookID, tenantID, "hook"), nil
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	newSecret := "rotated-token"
	body := UpdateWebhookJSONRequestBody{Secret: &newSecret}
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateWebhook(c, IdPath(webhookID))
	}, http.MethodPatch, "/integration/webhooks/"+webhookID.String(), body,
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !capturedParams.Secret.Valid || capturedParams.Secret.String != newSecret {
		t.Errorf("secret plaintext not applied: %+v", capturedParams.Secret)
	}
	if capturedParams.SecretEncrypted == nil {
		t.Fatal("SecretEncrypted must be written when caller supplies secret")
	}
	if string(capturedParams.SecretEncrypted) != "ENC:"+newSecret {
		t.Errorf("SecretEncrypted wrong: %q", capturedParams.SecretEncrypted)
	}
}

func TestUpdateWebhook_InvalidURL_400(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()

	svc := &mockIntegrationService{
		getWebhookFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.WebhookSubscription, error) {
			return stubWebhook(webhookID, tenantID, "hook"), nil
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	// ftp:// scheme is rejected (Create uses the same check)
	badURL := "ftp://not-http.example.com/"
	body := UpdateWebhookJSONRequestBody{Url: &badURL}

	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateWebhook(c, IdPath(webhookID))
	}, http.MethodPatch, "/integration/webhooks/"+webhookID.String(), body,
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateWebhook_CrossTenant_404(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()

	svc := &mockIntegrationService{
		getWebhookFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.WebhookSubscription, error) {
			return dbgen.WebhookSubscription{}, errors.New("not found")
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	name := "x"
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateWebhook(c, IdPath(webhookID))
	}, http.MethodPatch, "/integration/webhooks/"+webhookID.String(),
		UpdateWebhookJSONRequestBody{Name: &name},
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateWebhook_InvalidJSON_400(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()
	s := newIntegrationTestServer(&mockIntegrationService{}, &mockAuditService{})

	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateWebhook(c, IdPath(webhookID))
	}, http.MethodPatch, "/integration/webhooks/"+webhookID.String(),
		"not-json",
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DeleteWebhook
// ---------------------------------------------------------------------------

func TestDeleteWebhook_Success(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()

	deleteCalled := false
	svc := &mockIntegrationService{
		getWebhookFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.WebhookSubscription, error) {
			return stubWebhook(webhookID, tenantID, "doomed-hook"), nil
		},
		deleteWebhookFn: func(_ context.Context, id, tID uuid.UUID) error {
			deleteCalled = true
			if id != webhookID || tID != tenantID {
				t.Errorf("delete tenant scoping mismatch: id=%v tenant=%v", id, tID)
			}
			return nil
		},
	}
	audit := &mockAuditService{}
	s := newIntegrationTestServer(svc, audit)

	rec := runHandler(t, func(c *gin.Context) {
		s.DeleteWebhook(c, IdPath(webhookID))
	}, http.MethodDelete, "/integration/webhooks/"+webhookID.String(), nil,
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if !deleteCalled {
		t.Error("DeleteWebhook was not called")
	}
	if len(audit.records) != 1 || audit.records[0].Action != "webhook.deleted" {
		t.Errorf("expected one webhook.deleted audit event, got %+v", audit.records)
	}
	if audit.records[0].Diff["name"] != "doomed-hook" {
		t.Errorf("audit diff should carry pre-delete name: %+v", audit.records[0].Diff)
	}
}

func TestDeleteWebhook_CrossTenant_404(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		getWebhookFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.WebhookSubscription, error) {
			return dbgen.WebhookSubscription{}, errors.New("not found")
		},
	}
	s := newIntegrationTestServer(svc, &mockAuditService{})

	rec := runHandler(t, func(c *gin.Context) {
		s.DeleteWebhook(c, IdPath(webhookID))
	}, http.MethodDelete, "/integration/webhooks/"+webhookID.String(), nil,
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// SSRF defense (netguard wiring)
// ---------------------------------------------------------------------------

func TestCreateAdapter_BlockedEndpoint_400(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		// createAdapterFn intentionally absent — if we ever get past the
		// netguard check, the mock panics the test.
	}
	s := newIntegrationTestServerWithGuard(t, svc, &mockAuditService{})

	badEndpoint := "http://169.254.169.254/latest/meta-data/"
	body := CreateAdapterJSONRequestBody{
		Name:      "meta",
		Type:      "custom_rest",
		Direction: "outbound",
		Endpoint:  &badEndpoint,
	}
	rec := runHandler(t, s.CreateAdapter, http.MethodPost, "/integration/adapters", body,
		nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "blocked network") {
		t.Errorf("expected body to mention blocked network, got %q", rec.Body.String())
	}
}

func TestCreateAdapter_BlockedConfigURL_400(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockIntegrationService{}
	s := newIntegrationTestServerWithGuard(t, svc, &mockAuditService{})

	// Endpoint OK, but config.url is the metadata endpoint — blocked.
	endpoint := "https://metrics.example.com/api"
	cfg := map[string]interface{}{"url": "http://10.0.0.1:8080/exfil"}
	body := CreateAdapterJSONRequestBody{
		Name:      "sneaky",
		Type:      "custom_rest",
		Direction: "outbound",
		Endpoint:  &endpoint,
		Config:    &cfg,
	}
	rec := runHandler(t, s.CreateAdapter, http.MethodPost, "/integration/adapters", body,
		nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "blocked network") {
		t.Errorf("expected body to mention blocked network, got %q", rec.Body.String())
	}
}

func TestUpdateAdapter_BlockedEndpoint_400(t *testing.T) {
	adapterID := uuid.New()
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		getAdapterFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.IntegrationAdapter, error) {
			return stubAdapter(adapterID, tenantID, "existing"), nil
		},
	}
	s := newIntegrationTestServerWithGuard(t, svc, &mockAuditService{})

	bad := "http://127.0.0.1:8080/"
	body := UpdateAdapterJSONRequestBody{Endpoint: &bad}
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateAdapter(c, IdPath(adapterID))
	}, http.MethodPatch, "/integration/adapters/"+adapterID.String(), body,
		idParams(adapterID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateWebhook_BlockedURL_400(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockIntegrationService{}
	s := newIntegrationTestServerWithGuard(t, svc, &mockAuditService{})

	// Valid scheme, but private IP.
	body := CreateWebhookJSONRequestBody{
		Name:   "leak",
		Url:    "http://192.168.1.10/ingest",
		Events: []string{"alert.fired"},
	}
	rec := runHandler(t, s.CreateWebhook, http.MethodPost, "/integration/webhooks", body,
		nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "blocked network") {
		t.Errorf("expected body to mention blocked network, got %q", rec.Body.String())
	}
}

func TestUpdateWebhook_BlockedURL_400(t *testing.T) {
	webhookID := uuid.New()
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		getWebhookFn: func(context.Context, uuid.UUID, uuid.UUID) (dbgen.WebhookSubscription, error) {
			return stubWebhook(webhookID, tenantID, "hook"), nil
		},
	}
	s := newIntegrationTestServerWithGuard(t, svc, &mockAuditService{})

	bad := "http://169.254.169.254/token"
	body := UpdateWebhookJSONRequestBody{Url: &bad}
	rec := runHandler(t, func(c *gin.Context) {
		s.UpdateWebhook(c, IdPath(webhookID))
	}, http.MethodPatch, "/integration/webhooks/"+webhookID.String(), body,
		idParams(webhookID),
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAdapter_PublicEndpoint_Allowed(t *testing.T) {
	// Sanity check: a public IP literal must NOT trip the guard.
	adapterID := uuid.New()
	tenantID := uuid.New()
	svc := &mockIntegrationService{
		createAdapterFn: func(_ context.Context, p dbgen.CreateAdapterParams) (dbgen.IntegrationAdapter, error) {
			return stubAdapter(adapterID, tenantID, p.Name), nil
		},
	}
	s := newIntegrationTestServerWithGuard(t, svc, &mockAuditService{})

	endpoint := "https://1.1.1.1/api"
	body := CreateAdapterJSONRequestBody{
		Name:      "public",
		Type:      "custom_rest",
		Direction: "outbound",
		Endpoint:  &endpoint,
	}
	rec := runHandler(t, s.CreateAdapter, http.MethodPost, "/integration/adapters", body,
		nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

