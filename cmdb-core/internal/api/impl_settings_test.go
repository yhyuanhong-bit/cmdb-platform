package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/domain/settings"
)

// These tests pin down the wire-format conversion and the
// happy/sad-path branching of the settings handlers without spinning
// up Postgres. End-to-end DB coverage lives in the integration tests
// (impl_settings_integration_test.go would be the natural home if we
// later need it; the existing service_test.go covers the data-shape
// invariants).

func TestToAssetLifespanWire_PopulatesAllFields(t *testing.T) {
	cfg := settings.AssetLifespanConfig{Server: 5, Network: 7, Storage: 5, Power: 10}
	wire := toAssetLifespanWire(cfg)
	if wire.Server == nil || *wire.Server != 5 {
		t.Errorf("Server pointer wrong: %+v", wire.Server)
	}
	if wire.Network == nil || *wire.Network != 7 {
		t.Errorf("Network pointer wrong: %+v", wire.Network)
	}
	if wire.Storage == nil || *wire.Storage != 5 {
		t.Errorf("Storage pointer wrong: %+v", wire.Storage)
	}
	if wire.Power == nil || *wire.Power != 10 {
		t.Errorf("Power pointer wrong: %+v", wire.Power)
	}
}

func TestFromAssetLifespanWire_NilPointersBecomeZero(t *testing.T) {
	body := UpdateAssetLifespanSettingsJSONRequestBody{}
	cfg := fromAssetLifespanWire(body)
	if cfg != (settings.AssetLifespanConfig{}) {
		t.Errorf("expected zero config, got %+v", cfg)
	}
}

func TestFromAssetLifespanWire_PopulatedPointersAreCopied(t *testing.T) {
	server := int64(8)
	power := int64(15)
	body := UpdateAssetLifespanSettingsJSONRequestBody{
		Server: &server,
		Power:  &power,
	}
	cfg := fromAssetLifespanWire(body)
	if cfg.Server != 8 || cfg.Power != 15 {
		t.Errorf("got %+v, want {Server:8 Power:15}", cfg)
	}
	if cfg.Network != 0 || cfg.Storage != 0 {
		t.Errorf("unset fields leaked non-zero: %+v", cfg)
	}
}

// TestGetAssetLifespanSettings_MissingTenantReturns401 covers the
// fast-fail branch — no settings service call needed because the
// tenant guard fires first.
func TestGetAssetLifespanSettings_MissingTenantReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/settings/asset-lifespan", nil)
	// no tenant_id set in context

	srv := &APIServer{}
	srv.GetAssetLifespanSettings(c)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestUpdateAssetLifespanSettings_MissingTenantReturns401 mirrors the
// GET test for the PUT endpoint.
func TestUpdateAssetLifespanSettings_MissingTenantReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body, _ := json.Marshal(map[string]int64{"server": 5})
	c.Request = httptest.NewRequest(http.MethodPut, "/settings/asset-lifespan", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	srv := &APIServer{}
	srv.UpdateAssetLifespanSettings(c)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestUpdateAssetLifespanSettings_MalformedBodyReturns400 ensures we
// don't reach the service layer on a parser failure.
func TestUpdateAssetLifespanSettings_MalformedBodyReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/settings/asset-lifespan", bytes.NewReader([]byte("{not-json")))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("tenant_id", uuid.New().String())

	srv := &APIServer{}
	srv.UpdateAssetLifespanSettings(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
