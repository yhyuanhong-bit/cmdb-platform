package workflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
)

// CustomRESTAdapter.Fetch must refuse a loopback/metadata endpoint before
// issuing any HTTP request. This is the SSRF defense for operators who
// might (accidentally or maliciously) point an adapter at 169.254.169.254.
func TestCustomRESTAdapter_RejectsBlockedEndpoint(t *testing.T) {
	// Install a strict guard for this test (restore when done so other
	// tests in this package don't see spill-over state).
	prev := GetNetGuard()
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	SetNetGuard(g)
	t.Cleanup(func() { SetNetGuard(prev) })

	a := &CustomRESTAdapter{}
	_, err = a.Fetch(context.Background(), "http://169.254.169.254/meta", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected SSRF rejection for metadata endpoint")
	}
	if !strings.Contains(err.Error(), "netguard") {
		t.Fatalf("expected netguard in error, got %v", err)
	}
}

func TestCustomRESTAdapter_RejectsBlockedConfigURL(t *testing.T) {
	prev := GetNetGuard()
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	SetNetGuard(g)
	t.Cleanup(func() { SetNetGuard(prev) })

	a := &CustomRESTAdapter{}
	// Endpoint is public, but config.url overrides to a private IP.
	cfg := json.RawMessage(`{"url":"http://10.0.0.5/fetch"}`)
	_, err = a.Fetch(context.Background(), "https://metrics.example.com/", cfg)
	if err == nil {
		t.Fatal("expected SSRF rejection for config.url override")
	}
}

func TestSetGetNetGuard_RoundTrip(t *testing.T) {
	g, err := netguard.New(nil, []string{"example.com"})
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	prev := GetNetGuard()
	SetNetGuard(g)
	t.Cleanup(func() { SetNetGuard(prev) })

	got := GetNetGuard()
	if got != g {
		t.Fatal("GetNetGuard should return the installed guard")
	}
}

func TestGetNetGuard_LazyDefault(t *testing.T) {
	// Reset to no guard, verify GetNetGuard lazily constructs one with
	// default blocks.
	prev := GetNetGuard()
	SetNetGuard(nil)
	t.Cleanup(func() { SetNetGuard(prev) })

	g := GetNetGuard()
	if g == nil {
		t.Fatal("GetNetGuard must always return a non-nil Guard")
	}
	// The lazy default must still block loopback.
	if err := g.ValidateURL("http://127.0.0.1/"); err == nil {
		t.Fatal("lazy default guard must block loopback")
	}
}
