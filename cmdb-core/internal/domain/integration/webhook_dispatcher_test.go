package integration

import (
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
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
