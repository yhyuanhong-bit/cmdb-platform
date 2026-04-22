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
)

// TestZabbixAdapter_Fetch_HappyPath drives the full Zabbix flow:
// login → host.get → item.get → MetricPoint assembly. Each RPC is
// matched on method and the adapter assembles a point per
// (host, item). A regression in any stage surfaces here.
func TestZabbixAdapter_Fetch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "user.login":
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":"fake-token","id":1}`)
		case "host.get":
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":[
				{"hostid":"10001","host":"web1","interfaces":[{"ip":"10.0.0.1"}]},
				{"hostid":"10002","host":"web2","interfaces":[{"ip":"10.0.0.2"}]}
			],"id":1}`)
		case "item.get":
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":[
				{"itemid":"i1","hostid":"10001","name":"CPU","lastvalue":"42.5","lastclock":"1713000000"},
				{"itemid":"i2","hostid":"10002","name":"CPU","lastvalue":"38.0","lastclock":"1713000000"}
			],"id":1}`)
		default:
			http.Error(w, "unknown method "+req.Method, 400)
		}
	}))
	defer srv.Close()

	cfg := `{"username":"admin","password":"p","items":["cpu.util"]}`
	a := &ZabbixAdapter{}
	points, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	// Verify the host→IP mapping survived.
	ipSet := map[string]bool{points[0].IP: true, points[1].IP: true}
	if !ipSet["10.0.0.1"] || !ipSet["10.0.0.2"] {
		t.Errorf("expected IPs [10.0.0.1, 10.0.0.2], got %v", ipSet)
	}
	// Values parsed from strings.
	for _, p := range points {
		if p.Value != 42.5 && p.Value != 38.0 {
			t.Errorf("unexpected value %v", p.Value)
		}
	}
}

// TestZabbixAdapter_Fetch_APITokenBypassesLogin: supplying an API
// token directly must skip the login RPC. Catches a regression that
// always calls login even when a token is present — which both
// wastes an RPC and breaks token-only deployments.
func TestZabbixAdapter_Fetch_APITokenBypassesLogin(t *testing.T) {
	var loginCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "user.login":
			loginCalls++
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":"should-not-be-called","id":1}`)
		case "host.get":
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":[],"id":1}`)
		case "item.get":
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":[],"id":1}`)
		default:
			http.Error(w, "unknown method "+req.Method, 400)
		}
	}))
	defer srv.Close()

	cfg := `{"api_token":"preshared-token","items":["x"]}`
	a := &ZabbixAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if loginCalls != 0 {
		t.Errorf("login RPC called %d times, want 0 (api_token should bypass it)", loginCalls)
	}
}

// TestZabbixAdapter_Fetch_RPCErrorPropagates: a Zabbix-level error
// (e.g. "invalid auth") on any RPC must surface — the pre-fix code
// would return (nil, nil) which hid real failures.
func TestZabbixAdapter_Fetch_RPCErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params","data":"bad token"},"id":1}`)
	}))
	defer srv.Close()

	cfg := `{"api_token":"bad","items":["x"]}`
	a := &ZabbixAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err == nil {
		t.Fatal("expected error from zabbix RPC failure")
	}
	if !strings.Contains(err.Error(), "zabbix") {
		t.Errorf("error should mention zabbix: %v", err)
	}
}

// TestZabbixAdapter_Fetch_LoginParseError: a user.login response that
// doesn't decode as a string must fail cleanly with a "parse login
// token" error — not return an empty token that causes every
// downstream RPC to 401.
func TestZabbixAdapter_Fetch_LoginParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "user.login":
			// Return an object where a string was expected.
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":{"id":1},"id":1}`)
		default:
			http.Error(w, "should not reach here", 500)
		}
	}))
	defer srv.Close()

	cfg := `{"username":"u","password":"p","items":["x"]}`
	a := &ZabbixAdapter{}
	_, err := a.Fetch(context.Background(), srv.URL, json.RawMessage(cfg))
	if err == nil {
		t.Fatal("expected login parse error")
	}
	if !strings.Contains(err.Error(), "parse login token") {
		t.Errorf("error should mention token parse: %v", err)
	}
}
