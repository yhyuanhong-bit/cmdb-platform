package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RBACConfig is the parsed, validated form of rbac_config.yaml.
//
// Construction always goes through LoadRBACConfig; tests may build
// fixtures inline but production code must never bypass Validate.
//
// Once passed to ConfigureRBAC the struct is treated as immutable —
// mutating any field after that point is a data race with in-flight
// requests.
type RBACConfig struct {
	Version     int               `yaml:"version"`
	PublicPaths []string          `yaml:"publicPaths"`
	ResourceMap map[string]string `yaml:"resourceMap"`
}

// knownResources is the closed set of resource names that may appear as
// values in ResourceMap and as keys in the roles.permissions JSONB.
//
// Adding an entry here is a deliberate contract change — both the config
// YAML and the roles table need coordinated updates. Keep this list
// alphabetised so diffs are easy to review.
var knownResources = map[string]struct{}{
	"assets":      {},
	"audit":       {},
	"bia":         {},
	"dashboard":   {},
	"identity":    {},
	"integration": {},
	"inventory":   {},
	"maintenance": {},
	"monitoring":  {},
	"prediction":  {},
	"quality":     {},
	"services":    {},
	"settings":    {},
	"sync":        {},
	"system":      {},
	"topology":    {},
}

// KnownResources returns a copy of the closed resource whitelist. Useful
// for tests and for contract checks that must not mutate the package
// state.
func KnownResources() []string {
	out := make([]string, 0, len(knownResources))
	for r := range knownResources {
		out = append(out, r)
	}
	return out
}

const (
	// rbacConfigPathEnv lets operators override the on-disk config
	// location without rebuilding the binary (e.g. K8s ConfigMap mount).
	rbacConfigPathEnv = "CMDB_RBAC_CONFIG_PATH"

	// defaultRBACConfigPath is resolved relative to the process working
	// directory. main.go sets cwd to the binary's parent in production,
	// so the default lands at <repo>/internal/middleware/rbac_config.yaml
	// during dev / tests. In production the env var override is expected.
	defaultRBACConfigPath = "internal/middleware/rbac_config.yaml"
)

var (
	publicPathRe = regexp.MustCompile(`^(/healthz|/metrics|/api/v1/.+)$`)
	segmentKeyRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// LoadRBACConfig reads, parses and validates the RBAC routing config.
//
// Path precedence:
//  1. explicit path argument (non-empty wins)
//  2. $CMDB_RBAC_CONFIG_PATH
//  3. defaultRBACConfigPath
//
// Returns a fully validated config or an error the caller MUST treat as
// fatal — the server must not start on a validation failure.
func LoadRBACConfig(path string) (*RBACConfig, error) {
	resolved := resolveConfigPath(path)

	raw, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("read rbac config %q: %w", resolved, err)
	}

	var cfg RBACConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true) // unknown top-level keys = fatal
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse rbac config %q: %w", resolved, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rbac config %q: %w", resolved, err)
	}
	return &cfg, nil
}

func resolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv(rbacConfigPathEnv); env != "" {
		return env
	}
	return defaultRBACConfigPath
}

// Validate enforces the schema described in
// docs/reports/phase4/4.9-rbac-config-externalization.md §4.4.
//
// Rules:
//  1. Version must equal 1.
//  2. Every publicPath matches /healthz|/metrics|/api/v1/...
//  3. No trailing slash on any publicPath.
//  4. No duplicate publicPaths.
//  5. resourceMap must be non-empty.
//  6. Every resourceMap key is a URL-safe segment ([a-z0-9][a-z0-9-]*).
//  7. Every resourceMap value is a member of knownResources.
//  8. resourceMap keys do not collide with segments already exposed as
//     publicPaths (a path is either public OR RBAC-checked, never both).
func (c *RBACConfig) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported rbac config version %d (expected 1)", c.Version)
	}

	if len(c.PublicPaths) == 0 {
		return errors.New("publicPaths must not be empty")
	}

	seenPaths := make(map[string]bool, len(c.PublicPaths))
	for _, p := range c.PublicPaths {
		if !publicPathRe.MatchString(p) {
			return fmt.Errorf("publicPaths[%q]: path must match %s", p, publicPathRe)
		}
		if strings.HasSuffix(p, "/") && p != "/" {
			return fmt.Errorf("publicPaths[%q]: trailing slash not allowed", p)
		}
		if seenPaths[p] {
			return fmt.Errorf("publicPaths[%q]: duplicate entry", p)
		}
		seenPaths[p] = true
	}

	// Compute the set of URL segments already exposed as publicPaths so
	// we can reject overlap with resourceMap keys.
	publicSegments := make(map[string]bool)
	const apiPrefix = "/api/v1/"
	for _, p := range c.PublicPaths {
		if strings.HasPrefix(p, apiPrefix) {
			rest := strings.TrimPrefix(p, apiPrefix)
			seg := strings.SplitN(rest, "/", 2)[0]
			publicSegments[seg] = true
		}
	}

	if len(c.ResourceMap) == 0 {
		return errors.New("resourceMap must not be empty")
	}

	for k, v := range c.ResourceMap {
		if !segmentKeyRe.MatchString(k) {
			return fmt.Errorf("resourceMap key %q: must match %s", k, segmentKeyRe)
		}
		if _, ok := knownResources[v]; !ok {
			return fmt.Errorf("resourceMap[%q] = %q: unknown resource (add to knownResources if this is a new RBAC resource)", k, v)
		}
		// A segment that is also listed wholesale in publicPaths cannot
		// simultaneously map to a resource — the middleware would
		// short-circuit on the public check and never evaluate the
		// resource, so such a mapping is dead config and a review trap.
		//
		// Note: we only flag overlap when publicPaths lists the SEGMENT
		// ROOT (e.g. "/api/v1/auth" as public and "auth" in resourceMap
		// — conflict). Listing a SUB-path like "/api/v1/auth/login" as
		// public while mapping "auth" in resourceMap is the normal case
		// and must continue to work. We approximate the root-conflict
		// check by looking for an entry that has no further segments
		// under the API prefix.
		if publicSegments[k] {
			conflict := false
			for _, p := range c.PublicPaths {
				if p == apiPrefix+k { // exact root match
					conflict = true
					break
				}
			}
			if conflict {
				return fmt.Errorf("resourceMap[%q]: segment root is also listed in publicPaths — a path is either public OR RBAC-checked, not both", k)
			}
		}
	}

	return nil
}
