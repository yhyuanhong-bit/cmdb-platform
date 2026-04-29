package middleware

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// writeYAML writes content to a temp file and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "rbac.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return p
}

const validYAML = `version: 1
publicPaths:
  - /api/v1/auth/login
  - /healthz
  - /metrics
resourceMap:
  assets: assets
  users: identity
  monitoring: monitoring
`

func TestLoadRBACConfig_Valid(t *testing.T) {
	p := writeYAML(t, validYAML)
	cfg, err := LoadRBACConfig(p)
	if err != nil {
		t.Fatalf("expected valid config to load, got %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("version: want 1 got %d", cfg.Version)
	}
	if len(cfg.PublicPaths) != 3 {
		t.Errorf("publicPaths: want 3 got %d", len(cfg.PublicPaths))
	}
	if cfg.ResourceMap["assets"] != "assets" {
		t.Errorf("resourceMap[assets]: want assets got %q", cfg.ResourceMap["assets"])
	}
}

func TestLoadRBACConfig_EnvOverridePath(t *testing.T) {
	p := writeYAML(t, validYAML)
	t.Setenv("CMDB_RBAC_CONFIG_PATH", p)
	cfg, err := LoadRBACConfig("")
	if err != nil {
		t.Fatalf("env override path failed: %v", err)
	}
	if cfg == nil || len(cfg.PublicPaths) == 0 {
		t.Fatal("expected cfg populated via env override")
	}
}

func TestLoadRBACConfig_MissingFile(t *testing.T) {
	_, err := LoadRBACConfig("/nonexistent/rbac.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "read rbac config") {
		t.Errorf("error message should mention read failure, got %v", err)
	}
}

func TestLoadRBACConfig_InvalidYAML(t *testing.T) {
	p := writeYAML(t, "not: [valid: yaml")
	_, err := LoadRBACConfig(p)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse rbac config") {
		t.Errorf("error should mention parse, got %v", err)
	}
}

func TestLoadRBACConfig_UnknownYAMLKey(t *testing.T) {
	p := writeYAML(t, `version: 1
publicPaths:
  - /healthz
resourceMap:
  assets: assets
thisFieldIsUnknown: true
`)
	_, err := LoadRBACConfig(p)
	if err == nil {
		t.Fatal("expected error for unknown top-level key, got nil")
	}
	// yaml.v3 Decoder.KnownFields produces "field <name> not found".
	if !strings.Contains(err.Error(), "thisFieldIsUnknown") {
		t.Errorf("error should name the unknown field, got %v", err)
	}
}

func TestRBACConfig_Validate(t *testing.T) {
	// Keep each failing fixture minimal so diagnostics are obvious.
	cases := []struct {
		name    string
		cfg     RBACConfig
		wantErr string
	}{
		{
			name: "wrong version",
			cfg: RBACConfig{
				Version:     2,
				PublicPaths: []string{"/healthz"},
				ResourceMap: map[string]string{"assets": "assets"},
			},
			wantErr: "unsupported rbac config version",
		},
		{
			name: "empty publicPaths",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{},
				ResourceMap: map[string]string{"assets": "assets"},
			},
			wantErr: "publicPaths must not be empty",
		},
		{
			name: "bad public path format",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{"/not-allowed"},
				ResourceMap: map[string]string{"assets": "assets"},
			},
			wantErr: "publicPaths",
		},
		{
			name: "trailing slash",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{"/api/v1/auth/login/"},
				ResourceMap: map[string]string{"assets": "assets"},
			},
			wantErr: "trailing slash",
		},
		{
			name: "duplicate public path",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{"/healthz", "/healthz"},
				ResourceMap: map[string]string{"assets": "assets"},
			},
			wantErr: "duplicate entry",
		},
		{
			name: "empty resourceMap",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{"/healthz"},
				ResourceMap: map[string]string{},
			},
			wantErr: "resourceMap must not be empty",
		},
		{
			name: "unknown resource value",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{"/healthz"},
				ResourceMap: map[string]string{"assets": "asssets"},
			},
			wantErr: "unknown resource",
		},
		{
			name: "bad segment key",
			cfg: RBACConfig{
				Version:     1,
				PublicPaths: []string{"/healthz"},
				ResourceMap: map[string]string{"Assets/!": "assets"},
			},
			wantErr: "must match",
		},
		{
			name: "root segment overlap",
			cfg: RBACConfig{
				Version:     1,
				// /api/v1/debug is a root-level public path; mapping
				// "debug" in resourceMap then contradicts it.
				PublicPaths: []string{"/api/v1/debug"},
				ResourceMap: map[string]string{"debug": "system"},
			},
			wantErr: "segment root is also listed",
		},
		{
			name: "auth sub-path public + auth in resourceMap is OK",
			cfg: RBACConfig{
				Version: 1,
				// Only login is public under /auth, not the whole segment.
				PublicPaths: []string{"/api/v1/auth/login"},
				ResourceMap: map[string]string{"auth": "identity"},
			},
			wantErr: "", // must pass
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestLoadRBACConfig_DefaultYAML verifies the shipped default config is
// valid. It is both a smoke test for the loader and a regression guard
// against accidental edits to rbac_config.yaml that break schema.
func TestLoadRBACConfig_DefaultYAML(t *testing.T) {
	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		t.Fatalf("default config must be valid: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("default cfg version: want 1 got %d", cfg.Version)
	}
	// These are the paths operations traditionally relied on being
	// public. A regression here would be a critical security change.
	requiredPublic := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/logout",
		"/api/v1/ws",
		"/healthz",
		"/metrics",
	}
	have := make(map[string]bool, len(cfg.PublicPaths))
	for _, p := range cfg.PublicPaths {
		have[p] = true
	}
	for _, p := range requiredPublic {
		if !have[p] {
			t.Errorf("default config missing required public path %q", p)
		}
	}
}

// TestDefaultYAML_MatchesHistoricalMaps is a regression guard: after the
// 4.9 extraction, the YAML must reproduce the pre-existing hardcoded
// maps byte-for-byte, minus values that fail the new strict validation.
// It pins the expected shape so future edits go through code review
// rather than sneaking in via a YAML change.
func TestDefaultYAML_MatchesHistoricalMaps(t *testing.T) {
	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		t.Fatalf("load default: %v", err)
	}

	expectedResource := map[string]string{
		"assets":            "assets",
		"locations":         "topology",
		"racks":             "topology",
		"maintenance":       "maintenance",
		"monitoring":        "monitoring",
		"inventory":         "inventory",
		"audit":             "audit",
		"dashboard":         "dashboard",
		"users":             "identity",
		"roles":             "identity",
		"auth":              "identity",
		"prediction":        "prediction",
		"integration":       "integration",
		"system":            "system",
		"energy":            "monitoring",
		"sensors":           "monitoring",
		"topology":          "topology",
		"activity-feed":     "audit",
		"quality":           "system",
		"bia":               "system",
		"discovery":         "assets",
		"upgrade-rules":     "assets",
		"sync":              "sync",
		"notifications":     "system",
		"capacity-planning": "assets",
		"fleet-metrics":     "monitoring",
		"services":          "services",
		// Added 2026-04-28 (audit H6) — see rbac_config.yaml comment.
		"admin":      "system",
		"metrics":    "monitoring",
		"predictive": "prediction",
		// Added 2026-04-28 (W3.2-backend) — per-tenant settings store.
		"settings": "settings",
	}
	if len(cfg.ResourceMap) != len(expectedResource) {
		t.Errorf("resourceMap size drift: got %d want %d — update rbac_config.yaml AND this test together", len(cfg.ResourceMap), len(expectedResource))
	}
	for k, wantV := range expectedResource {
		if got := cfg.ResourceMap[k]; got != wantV {
			t.Errorf("resourceMap[%q]: got %q want %q", k, got, wantV)
		}
	}
}

// TestOpenAPIDriftContract scans the committed openapi.yaml for every
// /api/v1/<segment>/... path and asserts that each segment is either
// covered by publicPaths or by resourceMap. Catches the
// "new endpoint shipped but RBAC config forgotten" class of bug.
//
// Known drift is tracked via knownOpenAPIDrift below. When a segment
// appears only there the test warns (t.Log) so SRE sees it, but does not
// fail — giving product/security time to decide the correct resource
// mapping before the fix lands.
func TestOpenAPIDriftContract(t *testing.T) {
	openapiPath := findOpenAPIYAML(t)
	segments := extractAPISegments(t, openapiPath)

	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}

	publicSegments := publicAPISegments(cfg.PublicPaths)

	// Segments that are known to be missing from the RBAC config but
	// cannot be fixed as part of pure "config extraction" without
	// changing behaviour (see docs/reports/phase4/4.9-*.md §5.1).
	//
	// Review before removing an entry:
	//   - location-detect: not wired into production RBAC today; every
	//     request returns 403. Product/security must decide the correct
	//     resource mapping (identity? monitoring? a new "location" root
	//     resource?) before we remove this entry.
	knownOpenAPIDrift := map[string]string{
		"location-detect": "pre-existing: route in openapi.yaml but missing from hardcoded resourceMap before 4.9; product/security to decide target resource",
	}

	unexpected := make([]string, 0)
	for _, seg := range segments {
		if publicSegments[seg] {
			continue
		}
		if _, ok := cfg.ResourceMap[seg]; ok {
			continue
		}
		if note, ok := knownOpenAPIDrift[seg]; ok {
			t.Logf("KNOWN DRIFT — segment %q present in openapi.yaml but missing from rbac config: %s", seg, note)
			continue
		}
		unexpected = append(unexpected, seg)
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Errorf("openapi.yaml exposes segments with no RBAC config entry: %v — add them to resourceMap in internal/middleware/rbac_config.yaml OR to publicPaths", unexpected)
	}
}

// TestResourceMapValuesAreKnownResources is a belt-and-braces check on
// the validator: every value in the default YAML must be a declared
// knownResource. This guards against someone disabling Validate while
// debugging and shipping a typo.
func TestResourceMapValuesAreKnownResources(t *testing.T) {
	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	for k, v := range cfg.ResourceMap {
		if _, ok := knownResources[v]; !ok {
			t.Errorf("resourceMap[%q]=%q: value not in knownResources whitelist", k, v)
		}
	}
}

// TestAuthBypassPaths_ExcludesLogout verifies the auth bypass set
// derived from the config strips /auth/logout (RBAC-public but
// auth-required) while keeping login, refresh, ws, healthz and metrics.
func TestAuthBypassPaths_ExcludesLogout(t *testing.T) {
	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	// Refresh runtime so AuthBypassPaths sees the just-loaded config.
	ResetRBACForTesting(cfg)
	t.Cleanup(func() {
		defaultCfg, _ := LoadRBACConfig("rbac_config.yaml")
		ResetRBACForTesting(defaultCfg)
	})

	bypass := AuthBypassPaths()
	mustInclude := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/ws",
		"/healthz",
		"/metrics",
	}
	for _, p := range mustInclude {
		if _, ok := bypass[p]; !ok {
			t.Errorf("auth bypass missing %q", p)
		}
	}
	if _, ok := bypass["/api/v1/auth/logout"]; ok {
		t.Errorf("auth bypass MUST NOT include /api/v1/auth/logout (logout revokes a jti so it needs an authenticated request)")
	}
}

// TestConfigureRBAC_SecondCallPanics ensures the immutability guarantee
// is enforced. Uses ResetRBACForTesting to start from a clean state;
// otherwise the test pollutes package-level singleton state for peers.
func TestConfigureRBAC_SecondCallPanics(t *testing.T) {
	cfg, err := LoadRBACConfig("rbac_config.yaml")
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	ResetRBACForTesting(nil) // clear state
	t.Cleanup(func() {
		defaultCfg, _ := LoadRBACConfig("rbac_config.yaml")
		ResetRBACForTesting(defaultCfg)
	})

	ConfigureRBAC(cfg) // first call ok

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on second ConfigureRBAC call")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "already configured") {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	ConfigureRBAC(cfg) // should panic
}

// findOpenAPIYAML walks upwards from the package dir to locate the
// shared api/openapi.yaml.
func findOpenAPIYAML(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../../../api/openapi.yaml",
		"../../api/openapi.yaml",
		"../../../../api/openapi.yaml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Skip("openapi.yaml not reachable from package dir; skipping contract test")
	return ""
}

// extractAPISegments returns the distinct first-segment names under
// /api/v1 referenced by operation paths in the OpenAPI spec. It uses a
// line-level regex rather than a full YAML parser because the spec
// format is stable (path keys always start with two spaces, at column
// 0 of the paths: block) and pulling in a YAML dependency at this
// boundary is overkill for one test.
func extractAPISegments(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	// Paths look like "  /foo/bar:" — two-space indent, leading slash,
	// ending in ":". We only care about the first segment.
	re := regexp.MustCompile(`(?m)^  /([a-z0-9][a-z0-9-]*)[/:]`)
	matches := re.FindAllStringSubmatch(string(raw), -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, done := seen[m[1]]; done {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

// publicAPISegments returns the first-segment names under /api/v1 listed
// in publicPaths. Segments shared with resourceMap are expected to map
// to a sub-path (e.g. /api/v1/auth/login public + auth -> identity).
func publicAPISegments(paths []string) map[string]bool {
	out := make(map[string]bool)
	const prefix = "/api/v1/"
	for _, p := range paths {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := strings.TrimPrefix(p, prefix)
		seg := strings.SplitN(rest, "/", 2)[0]
		// Only treat as "publicly covered" if the ENTIRE segment root
		// is public; a specific leaf path (login) shouldn't exempt the
		// whole "auth" segment from RBAC.
		if p == prefix+seg {
			out[seg] = true
		}
	}
	return out
}
