package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
)

// These tests exercise the CustomRESTAdapter.Fetch happy + error
// paths by standing up a local httptest server. We install a
// permissive netguard (tests only) so the adapter can target
// 127.0.0.1 — production deployments always install a restrictive
// guard via SetNetGuard(netguard.New(...)).

// installPermissiveGuard swaps in a netguard that allows 127.0.0.1,
// restoring the previous guard at end-of-test. Required because
// httptest.NewServer binds to loopback which the default guard blocks.
func installPermissiveGuard(t *testing.T) {
	t.Helper()
	prev := GetNetGuard()
	SetNetGuard(netguard.Permissive())
	t.Cleanup(func() { SetNetGuard(prev) })
}

// TestCustomRESTAdapter_Fetch_HappyPath drives the full
// request/response cycle: config parse → netguard check → HTTP GET →
// JSON parse → ResultPath walk → field extraction. A single passing
// test exercises the whole non-error branch so a refactor that
// subtly breaks the ResultPath walk or field name lookup is caught.
func TestCustomRESTAdapter_Fetch_HappyPath(t *testing.T) {
	installPermissiveGuard(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != "GET" {
			t.Errorf("expected GET, got %s", got)
		}
		if got := r.Header.Get("X-Test-Header"); got != "on" {
			t.Errorf("custom header not forwarded: got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"data": {
				"metrics": [
					{"name":"cpu.util","value":42.5,"ip":"10.0.0.1","timestamp":1713000000},
					{"name":"mem.used","value":80.1,"ip":"10.0.0.2","timestamp":1713000000}
				]
			}
		}`)
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{
		"url": "%s",
		"headers": {"X-Test-Header": "on"},
		"result_path": "data.metrics",
		"name_field": "name",
		"value_field": "value",
		"ip_field": "ip",
		"timestamp_field": "timestamp"
	}`, srv.URL)

	a := &CustomRESTAdapter{}
	points, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].Name != "cpu.util" || points[0].Value != 42.5 || points[0].IP != "10.0.0.1" {
		t.Errorf("first point wrong: %+v", points[0])
	}
	if points[1].Name != "mem.used" || points[1].Value != 80.1 {
		t.Errorf("second point wrong: %+v", points[1])
	}
}

// TestCustomRESTAdapter_Fetch_Non200ReturnsError: a 500 status from
// the upstream must surface as an error with the body attached so
// operators can see the actual failure reason.
func TestCustomRESTAdapter_Fetch_Non200ReturnsError(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "backend overloaded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := &CustomRESTAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

// TestCustomRESTAdapter_Fetch_ResultPathMisses: a ResultPath that
// doesn't resolve to an array must fail with an explicit error — not
// silently return an empty slice (which would look like a healthy
// empty adapter).
func TestCustomRESTAdapter_Fetch_ResultPathMisses(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data": {"metrics": "not an array"}}`)
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{"url":"%s","result_path":"data.metrics"}`, srv.URL)
	a := &CustomRESTAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err == nil {
		t.Fatal("expected error when result_path misses")
	}
	if !strings.Contains(err.Error(), "array") {
		t.Errorf("error should mention array, got %v", err)
	}
}

// TestCustomRESTAdapter_Fetch_BadResponseJSONErrors: malformed JSON
// from the upstream must return an explicit parse error — the
// pre-fix code swallowed the unmarshal error and returned (nil, nil),
// which looked like a healthy idle adapter.
func TestCustomRESTAdapter_Fetch_BadResponseJSONErrors(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{broken json`)
	}))
	defer srv.Close()

	a := &CustomRESTAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "custom_rest: parse response") {
		t.Errorf("error should mention custom_rest parse: %v", err)
	}
}

// TestCustomRESTAdapter_Fetch_DefaultFieldNames: when no *_field is
// configured, the adapter defaults to "name", "value", "timestamp",
// "ip". This locks in the documented defaults so a regression that
// ships a different default fires a test.
func TestCustomRESTAdapter_Fetch_DefaultFieldNames(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"load","value":1.5,"ip":"1.2.3.4"}]`)
	}))
	defer srv.Close()

	// No result_path, no *_field overrides.
	a := &CustomRESTAdapter{}
	points, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(points) != 1 || points[0].Name != "load" || points[0].Value != 1.5 || points[0].IP != "1.2.3.4" {
		t.Fatalf("default field extraction failed: %+v", points)
	}
}

// TestCustomRESTAdapter_Fetch_ValueAsString: the adapter must tolerate
// numeric values encoded as JSON strings (common for Prometheus-style
// APIs). A regression that strict-type-checks would drop every
// string-encoded value.
func TestCustomRESTAdapter_Fetch_ValueAsString(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"x","value":"3.14"}]`)
	}))
	defer srv.Close()

	a := &CustomRESTAdapter{}
	points, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(points) != 1 || points[0].Value != 3.14 {
		t.Fatalf("string value not parsed: %+v", points)
	}
}

// TestCustomRESTAdapter_Fetch_POSTWithBody: the "method":"POST" +
// "body" path is used for APIs that require a POST query. Drives
// the same response-parsing code but the request side exercises
// NewRequestWithContext(method, targetURL, bodyReader).
func TestCustomRESTAdapter_Fetch_POSTWithBody(t *testing.T) {
	installPermissiveGuard(t)
	var gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{"url":"%s","method":"POST","body":"{\"q\":\"load\"}"}`, srv.URL)
	a := &CustomRESTAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotBody, `"q":"load"`) {
		t.Errorf("body not forwarded: %q", gotBody)
	}
}
