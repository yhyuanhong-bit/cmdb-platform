package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPrometheusAdapter_Fetch_HappyPath drives the full config parse
// → HTTP GET → promQL response parse → metric-point conversion path.
// The Prometheus adapter re-uses fetchPromMetrics + parsePromResponse;
// a regression in either surfaces here.
func TestPrometheusAdapter_Fetch_HappyPath(t *testing.T) {
	installPermissiveGuard(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /query?query=up — Prometheus client appends /query.
		if !strings.Contains(r.URL.Path, "/query") {
			t.Errorf("expected path with /query, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "up" {
			t.Errorf("query param = %q, want 'up'", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"status":"success",
			"data":{
				"resultType":"vector",
				"result":[
					{"metric":{"__name__":"up","instance":"10.0.1.5:9100"},"value":[1713000000,"1"]}
				]
			}
		}`)
	}))
	defer srv.Close()

	cfg := `{"queries":["up"]}`
	a := &PrometheusAdapter{}
	points, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Name != "up" {
		t.Errorf("name = %q, want up", points[0].Name)
	}
	if points[0].Value != 1.0 {
		t.Errorf("value = %f, want 1.0", points[0].Value)
	}
	if points[0].IP != "10.0.1.5" {
		t.Errorf("ip = %q, want 10.0.1.5 (port stripped)", points[0].IP)
	}
}

// TestPrometheusAdapter_Fetch_UpstreamError: when the promQL endpoint
// returns status=error, the adapter must surface it. Pre-fix code
// would return (nil, nil), looking like a healthy empty response.
func TestPrometheusAdapter_Fetch_UpstreamError(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"error","errorType":"bad_data","error":"parse error"}`)
	}))
	defer srv.Close()

	a := &PrometheusAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(`{"queries":["up"]}`))
	if err == nil {
		t.Fatal("expected error from upstream prometheus error")
	}
}

// TestPrometheusAdapter_Fetch_MultipleQueries: two configured queries
// mean two HTTP calls; results concatenate into one point slice. A
// regression that fails on the second query or drops its results
// would be caught here.
func TestPrometheusAdapter_Fetch_MultipleQueries(t *testing.T) {
	installPermissiveGuard(t)

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		// Return a single-point vector per call; the metric name
		// echoes the query so we can verify both calls happened.
		q := r.URL.Query().Get("query")
		fmt.Fprintf(w, `{
			"status":"success",
			"data":{"resultType":"vector","result":[
				{"metric":{"__name__":"%s","instance":"10.0.0.1:9100"},"value":[1713000000,"2"]}
			]}
		}`, q)
	}))
	defer srv.Close()

	cfg := `{"queries":["q1","q2"]}`
	a := &PrometheusAdapter{}
	points, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", calls)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
}

// TestFetchPromMetrics_HTTPErrorPropagates: the lower-level fetcher
// must propagate non-200 status codes with the response body so
// operators can debug from the error message alone.
func TestFetchPromMetrics_HTTPErrorPropagates(t *testing.T) {
	installPermissiveGuard(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := fetchPromMetrics(context.Background(), srv.URL, "up")
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention status code: %v", err)
	}
}
