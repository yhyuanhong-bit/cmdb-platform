package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/google/uuid"
)

// NewWebhookDispatcher must refuse a nil guard — silent fallback to a raw
// transport would re-open the SSRF hole.
func TestNewWebhookDispatcher_NilGuardPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil guard")
		}
	}()
	_ = NewWebhookDispatcher(nil, nil, nil)
}

func TestNewWebhookDispatcher_AcceptsGuard(t *testing.T) {
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	d := NewWebhookDispatcher(nil, nil, g)
	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
	if d.client == nil || d.client.Transport == nil {
		t.Fatal("dispatcher must install a guarded transport on its client")
	}
}

// TestBuildPayload_ShapeAndTimestamp asserts the delivery payload shape so
// downstream signing and receiver code can rely on the field set.
func TestBuildPayload_ShapeAndTimestamp(t *testing.T) {
	fixedTime := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	d := NewWebhookDispatcher(nil, nil, netguard.Permissive())
	d.now = func() time.Time { return fixedTime }
	tenantID := uuid.New().String()
	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: tenantID,
		Payload:  []byte(`{"hello":"world"}`),
	}
	body := d.buildPayload(event)

	var parsed struct {
		EventType string          `json:"event_type"`
		TenantID  string          `json:"tenant_id"`
		Payload   json.RawMessage `json:"payload"`
		Timestamp string          `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if parsed.EventType != "asset.created" {
		t.Errorf("event_type: got %q", parsed.EventType)
	}
	if parsed.TenantID != tenantID {
		t.Errorf("tenant_id: got %q", parsed.TenantID)
	}
	if parsed.Timestamp != fixedTime.Format(time.RFC3339) {
		t.Errorf("timestamp: got %q", parsed.Timestamp)
	}
}

// TestGuard_BlocksLoopbackByDefault is a regression test for the
// default-guard loopback denial that the dispatcher's SSRF rejection branch
// depends on.
func TestGuard_BlocksLoopbackByDefault(t *testing.T) {
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	if err := g.ValidateURL("http://127.0.0.1:8080/hook"); err == nil {
		t.Fatal("default guard must block loopback URLs — dispatcher SSRF branch depends on it")
	}
}

// TestAttemptOnce_CounterAdvancesAcrossCalls asserts the low-level HTTP
// path is reached once per attemptOnce invocation.
func TestAttemptOnce_CounterAdvancesAcrossCalls(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher(nil, nil, netguard.Permissive())
	sub := dbgen.WebhookSubscription{ID: uuid.New(), Url: srv.URL}
	event := eventbus.Event{Subject: "asset.created", TenantID: uuid.New().String()}
	body := d.buildPayload(event)

	for i := 0; i < 3; i++ {
		status, _, _ := d.attemptOnce(context.Background(), sub, event, body, "")
		if status != 503 {
			t.Fatalf("attempt %d: want 503, got %d", i+1, status)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("server should have seen 3 POSTs, got %d", got)
	}
}
