package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
)

// TestMain seeds the RBAC runtime so tests that call extractResource or
// RBAC without their own ConfigureRBAC still work. Individual tests may
// override via ResetRBACForTesting. When `go test` runs inside this
// package the cwd is the package directory, so the YAML is one filename
// away.
func TestMain(m *testing.M) {
	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		panic("TestMain: failed to load default rbac config: " + err.Error())
	}
	ResetRBACForTesting(cfg)
	os.Exit(m.Run())
}

func TestExtractResource(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/assets", "assets"},
		{"/api/v1/assets/123", "assets"},
		{"/api/v1/locations/abc/children", "topology"},
		{"/api/v1/racks/xyz", "topology"},
		{"/api/v1/maintenance/orders", "maintenance"},
		{"/api/v1/energy/breakdown", "monitoring"},
		{"/api/v1/sensors/123", "monitoring"},
		{"/api/v1/topology/graph", "topology"},
		{"/api/v1/unknown/path", ""},
		{"/healthz", ""},
		{"/metrics", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractResource(tt.path)
		if got != tt.expected {
			t.Errorf("extractResource(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestMethodToAction(t *testing.T) {
	tests := []struct {
		method   string
		expected string
	}{
		{http.MethodGet, "read"},
		{http.MethodPost, "write"},
		{http.MethodPut, "write"},
		{http.MethodPatch, "write"},
		{http.MethodDelete, "delete"},
		{http.MethodHead, "read"},
	}
	for _, tt := range tests {
		got := methodToAction(tt.method)
		if got != tt.expected {
			t.Errorf("methodToAction(%q) = %q, want %q", tt.method, got, tt.expected)
		}
	}
}

func TestCheckPermission(t *testing.T) {
	tests := []struct {
		name     string
		perms    map[string][]string
		resource string
		action   string
		expected bool
	}{
		{"direct match", map[string][]string{"assets": {"read"}}, "assets", "read", true},
		{"no match", map[string][]string{"assets": {"read"}}, "assets", "write", false},
		{"write implies read", map[string][]string{"assets": {"write"}}, "assets", "read", true},
		{"delete does not imply read", map[string][]string{"assets": {"delete"}}, "assets", "read", false},
		{"wildcard admin", map[string][]string{"*": {"*"}}, "anything", "delete", true},
		{"unknown resource", map[string][]string{"assets": {"read"}}, "unknown", "read", false},
		{"empty perms", map[string][]string{}, "assets", "read", false},
	}
	for _, tt := range tests {
		got := checkPermission(tt.perms, tt.resource, tt.action)
		if got != tt.expected {
			t.Errorf("%s: checkPermission = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestMergePermissions(t *testing.T) {
	role1Perms, _ := json.Marshal(map[string][]string{"assets": {"read"}, "topology": {"read"}})
	role2Perms, _ := json.Marshal(map[string][]string{"assets": {"write"}, "maintenance": {"read", "write"}})

	roles := []dbgen.Role{
		{Permissions: role1Perms},
		{Permissions: role2Perms},
	}

	merged := mergePermissions(roles)

	if len(merged["assets"]) != 2 {
		t.Errorf("assets should have 2 actions, got %v", merged["assets"])
	}
	if !containsStr(merged["assets"], "read") || !containsStr(merged["assets"], "write") {
		t.Errorf("assets should contain read and write, got %v", merged["assets"])
	}
	if len(merged["maintenance"]) != 2 {
		t.Errorf("maintenance should have 2 actions, got %v", merged["maintenance"])
	}
	if len(merged["topology"]) != 1 {
		t.Errorf("topology should have 1 action, got %v", merged["topology"])
	}
}

func TestMergePermissions_EmptyRoles(t *testing.T) {
	merged := mergePermissions(nil)
	if len(merged) != 0 {
		t.Errorf("expected empty map for nil roles, got %v", merged)
	}
}

func TestIsSuperAdmin(t *testing.T) {
	tests := []struct {
		name  string
		perms map[string][]string
		want  bool
	}{
		{"wildcard star-star", map[string][]string{"*": {"*"}}, true},
		{"wildcard without star action", map[string][]string{"*": {"read"}}, false},
		{"no wildcard", map[string][]string{"assets": {"*"}}, false},
		{"empty", map[string][]string{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		if got := isSuperAdmin(tt.perms); got != tt.want {
			t.Errorf("%s: isSuperAdmin = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMergePermissions_InvalidJSON(t *testing.T) {
	roles := []dbgen.Role{
		{Permissions: []byte(`not-json`)},
		{Permissions: json.RawMessage(`{"assets":["read"]}`)},
	}
	merged := mergePermissions(roles)
	// Invalid JSON role is skipped; valid one is merged.
	if len(merged["assets"]) != 1 || merged["assets"][0] != "read" {
		t.Errorf("expected assets=[read] after skipping invalid role, got %v", merged["assets"])
	}
}
