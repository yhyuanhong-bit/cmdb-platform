package workflows

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestGetAdapter_KnownTypes verifies the registry returns a non-nil
// adapter for every type documented in the roadmap. A typo in
// adapterRegistry would surface here instead of at runtime with a
// baffling "nil adapter" downstream.
func TestGetAdapter_KnownTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		adapterType string
		wantNil     bool
	}{
		{"prometheus", false},
		{"zabbix", false},
		{"custom_rest", false},
		{"snmp", false},
		{"datadog", false},
		{"nagios", false},
		{"does-not-exist", true},
		{"", true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.adapterType, func(t *testing.T) {
			t.Parallel()
			got := GetAdapter(tc.adapterType)
			if tc.wantNil && got != nil {
				t.Errorf("GetAdapter(%q) = %T, want nil", tc.adapterType, got)
			}
			if !tc.wantNil && got == nil {
				t.Errorf("GetAdapter(%q) returned nil, want non-nil adapter", tc.adapterType)
			}
		})
	}
}

// TestSupportedAdapterTypes_Enumerates every expected type and does
// not duplicate. Catches an accidental `adapterRegistry[k] = ...`
// overwrite that would shadow a registered adapter.
func TestSupportedAdapterTypes(t *testing.T) {
	t.Parallel()
	types := SupportedAdapterTypes()
	sort.Strings(types)

	want := []string{"custom_rest", "datadog", "nagios", "prometheus", "snmp", "zabbix"}
	if len(types) != len(want) {
		t.Fatalf("expected %d types, got %d (%v)", len(want), len(types), types)
	}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("index %d: got %q want %q", i, types[i], w)
		}
	}
}

// TestPlaceholderAdapters_ReturnErrors locks in the "not implemented"
// contract for SNMP/Datadog/Nagios. A regression that returns (nil, nil)
// would look like a healthy idle adapter and silently drop every metric
// the operator expected to collect.
func TestPlaceholderAdapters_ReturnErrors(t *testing.T) {
	t.Parallel()

	adapters := []struct {
		name    string
		adapter MetricsAdapter
	}{
		{"snmp", &SNMPAdapter{}},
		{"datadog", &DatadogAdapter{}},
		{"nagios", &NagiosAdapter{}},
	}
	for _, a := range adapters {
		a := a
		t.Run(a.name, func(t *testing.T) {
			t.Parallel()
			points, err := a.adapter.Fetch(context.Background(), "http://localhost", json.RawMessage(`{}`))
			if err == nil {
				t.Errorf("%s.Fetch returned nil error — placeholder must signal not-implemented", a.name)
			}
			if points != nil {
				t.Errorf("%s.Fetch returned %d points — placeholder must not fabricate data", a.name, len(points))
			}
		})
	}
}

// TestPrometheusAdapter_BadConfigReturnsError is the regression guard
// for the "silent empty" bug: a malformed config JSON used to fall
// through to an empty-queries path and Fetch returned (nil, nil). Now
// the error must surface so the adapter-failure counter ticks.
func TestPrometheusAdapter_BadConfigReturnsError(t *testing.T) {
	t.Parallel()
	a := &PrometheusAdapter{}
	_, err := a.Fetch(context.Background(), "http://localhost", json.RawMessage(`{not-json}`))
	if err == nil {
		t.Fatal("expected parse error for malformed config")
	}
	if !strings.Contains(err.Error(), "prometheus") {
		t.Errorf("error should mention prometheus: %v", err)
	}
}

// TestPrometheusAdapter_EmptyQueriesReturnsNilNil: the "no queries
// configured" path is a legitimate no-op and must return (nil, nil).
// Distinguishes operator intent (nothing configured) from a broken
// adapter.
func TestPrometheusAdapter_EmptyQueriesReturnsNilNil(t *testing.T) {
	t.Parallel()
	a := &PrometheusAdapter{}
	points, err := a.Fetch(context.Background(), "http://localhost", json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("empty config should not error: %v", err)
	}
	if points != nil {
		t.Errorf("no queries configured should yield nil points, got %+v", points)
	}
}

// TestZabbixAdapter_BadConfigReturnsError parallels the prometheus
// guard: malformed config must surface, not silently no-op.
func TestZabbixAdapter_BadConfigReturnsError(t *testing.T) {
	t.Parallel()
	a := &ZabbixAdapter{}
	_, err := a.Fetch(context.Background(), "http://localhost", json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("expected parse error for malformed zabbix config")
	}
	if !strings.Contains(err.Error(), "zabbix") {
		t.Errorf("error should mention zabbix: %v", err)
	}
}

// TestZabbixAdapter_EmptyItemsIsNoOp: zero configured items returns
// (nil, nil). Matches the "nothing to fetch" path of prometheus.
func TestZabbixAdapter_EmptyItemsIsNoOp(t *testing.T) {
	t.Parallel()
	a := &ZabbixAdapter{}
	points, err := a.Fetch(context.Background(), "http://localhost", json.RawMessage(`{"items":[]}`))
	if err != nil {
		t.Errorf("empty items should not error: %v", err)
	}
	if points != nil {
		t.Errorf("empty items should yield nil points, got %+v", points)
	}
}

// TestZabbixAdapter_NoAuthRejected: if no api_token is configured AND
// no username is supplied, login cannot succeed — the adapter must
// refuse to issue any RPC and return an explicit error.
func TestZabbixAdapter_NoAuthRejected(t *testing.T) {
	t.Parallel()
	a := &ZabbixAdapter{}
	// One item configured so we bypass the early "empty items" path,
	// but no credentials at all.
	_, err := a.Fetch(context.Background(), "http://localhost", json.RawMessage(`{"items":["cpu.util"]}`))
	if err == nil {
		t.Fatal("expected error when no credentials configured")
	}
}

// TestCustomRESTAdapter_BadConfigReturnsError: malformed config JSON
// must surface as a real error on the custom REST path too.
func TestCustomRESTAdapter_BadConfigReturnsError(t *testing.T) {
	t.Parallel()
	a := &CustomRESTAdapter{}
	_, err := a.Fetch(context.Background(), "https://metrics.example.com/api", json.RawMessage(`{not-json}`))
	if err == nil {
		t.Fatal("expected parse error for malformed custom_rest config")
	}
	if !strings.Contains(err.Error(), "custom_rest") {
		t.Errorf("error should mention custom_rest: %v", err)
	}
}
