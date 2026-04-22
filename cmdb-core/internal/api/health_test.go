package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// stubNATS lets us control IsConnected() output without a real NATS server.
type stubNATS struct{ connected bool }

func (s *stubNATS) IsConnected() bool { return s.connected }

// runReadiness exercises the Readiness handler with a stubbed NATS checker
// and no DB/Redis. We cannot stub pgxpool without a real pool, so the DB
// probe will fail — that still lets us assert the NATS check contributes
// correctly to the overall status and is included in the response body.
func runReadiness(t *testing.T, h *HealthHandler) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.Readiness(c)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	return rec, body
}

func TestReadiness_NATSDisconnectedFailsCheck(t *testing.T) {
	h := &HealthHandler{nats: &stubNATS{connected: false}}
	rec, body := runReadiness(t, h)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when NATS is disconnected", rec.Code)
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatalf("body missing checks: %v", body)
	}
	nats, ok := checks["nats"].(map[string]any)
	if !ok {
		t.Fatalf("checks missing nats: %v", checks)
	}
	if nats["status"] != "down" {
		t.Errorf("nats.status = %v, want \"down\"", nats["status"])
	}
}

func TestReadiness_NATSConnectedReportsUp(t *testing.T) {
	h := &HealthHandler{nats: &stubNATS{connected: true}}
	_, body := runReadiness(t, h)

	checks := body["checks"].(map[string]any)
	nats := checks["nats"].(map[string]any)
	if nats["status"] != "up" {
		t.Errorf("nats.status = %v, want \"up\"", nats["status"])
	}
}

func TestReadiness_NATSNilReportsNotConfigured(t *testing.T) {
	h := &HealthHandler{nats: nil}
	_, body := runReadiness(t, h)

	checks := body["checks"].(map[string]any)
	nats := checks["nats"].(map[string]any)
	if nats["status"] != "not_configured" {
		t.Errorf("nats.status = %v, want \"not_configured\"", nats["status"])
	}
}
