package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// permsCacheTTL is how long merged permissions stay in Redis.
const permsCacheTTL = 5 * time.Minute

// rbacRuntime holds the routing tables built from RBACConfig. It is set
// exactly once at startup (ConfigureRBAC) and then treated as immutable
// for the process lifetime. The middleware closes over *rbacRuntime to
// avoid any per-request synchronisation cost.
type rbacRuntime struct {
	publicPaths map[string]struct{}
	resourceMap map[string]string
}

var (
	rbacState    *rbacRuntime
	rbacStateMu  sync.RWMutex
	rbacConfigOK bool
)

// ConfigureRBAC installs the validated RBACConfig as the process-wide
// routing table. MUST be called exactly once, before any call to RBAC,
// typically from main() immediately after LoadRBACConfig succeeds.
//
// Calling it twice panics — this is intentional: the runtime tables are
// expected to be immutable after startup. A second call indicates either
// accidental re-initialisation in tests (use ResetRBACForTesting instead)
// or a real logic bug that should surface loudly.
func ConfigureRBAC(cfg *RBACConfig) {
	if cfg == nil {
		panic("middleware.ConfigureRBAC: cfg must not be nil")
	}

	rt := buildRuntime(cfg)

	rbacStateMu.Lock()
	defer rbacStateMu.Unlock()
	if rbacConfigOK {
		panic("middleware.ConfigureRBAC: already configured — RBAC tables are immutable after startup")
	}
	rbacState = rt
	rbacConfigOK = true
}

// ResetRBACForTesting lets tests swap the runtime between cases. It is
// intentionally named to stand out in a review diff; production code must
// not call it.
func ResetRBACForTesting(cfg *RBACConfig) {
	rbacStateMu.Lock()
	defer rbacStateMu.Unlock()
	if cfg == nil {
		rbacState = nil
		rbacConfigOK = false
		return
	}
	rbacState = buildRuntime(cfg)
	rbacConfigOK = true
}

func buildRuntime(cfg *RBACConfig) *rbacRuntime {
	pub := make(map[string]struct{}, len(cfg.PublicPaths))
	for _, p := range cfg.PublicPaths {
		pub[p] = struct{}{}
	}
	// Copy the resourceMap so callers mutating the source after
	// configuration can't affect routing. This is cheap: ≤ 30 entries.
	rm := make(map[string]string, len(cfg.ResourceMap))
	for k, v := range cfg.ResourceMap {
		rm[k] = v
	}
	return &rbacRuntime{publicPaths: pub, resourceMap: rm}
}

// currentRBAC snapshots the active runtime under read-lock. The returned
// pointer is never mutated, so callers can safely read from its maps
// without holding the lock.
func currentRBAC() *rbacRuntime {
	rbacStateMu.RLock()
	defer rbacStateMu.RUnlock()
	return rbacState
}

// methodToAction converts an HTTP method to an RBAC action.
func methodToAction(method string) string {
	switch method {
	case http.MethodGet:
		return "read"
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return "write"
	case http.MethodDelete:
		return "delete"
	default:
		return "read"
	}
}

// RBAC returns a Gin middleware that enforces permission checks based on
// the authenticated user's roles. Permissions are cached in Redis
// (key perms:{user_id}) with a 5-minute TTL and fall back to a DB query
// via ListUserRoles.
//
// The routing tables (publicPaths, resourceMap) are loaded from the RBAC
// config YAML at startup via LoadRBACConfig + ConfigureRBAC; this
// constructor panics if ConfigureRBAC has not run yet to preserve
// fail-closed semantics.
func RBAC(queries *dbgen.Queries, redisClient *redis.Client) gin.HandlerFunc {
	rt := currentRBAC()
	if rt == nil {
		// Fail-closed at wiring time: if main.go forgot to call
		// ConfigureRBAC we refuse to hand back a middleware that would
		// default-deny every request (that was the old "fail deadlock"
		// mode — see §4.4 of the phase 4.9 plan).
		panic("middleware.RBAC: ConfigureRBAC must be called before constructing the middleware")
	}

	// Capture the runtime in the closure. Since ConfigureRBAC freezes
	// the tables for the process lifetime, no per-request RLock is
	// needed — the maps are read-only after this point.
	publicPaths := rt.publicPaths
	resourceMap := rt.resourceMap

	return func(c *gin.Context) {
		// Skip public paths.
		if _, ok := publicPaths[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		// Extract resource from path.
		resource := extractResourceWith(c.Request.URL.Path, resourceMap)
		if resource == "" {
			// Default deny: unknown resource paths are blocked
			response.Forbidden(c, "access denied: unknown resource")
			c.Abort()
			return
		}

		action := methodToAction(c.Request.Method)

		// user_id is set by the Auth middleware.
		rawUID, exists := c.Get("user_id")
		if !exists {
			response.Forbidden(c, "missing user identity")
			c.Abort()
			return
		}

		userID, err := uuid.Parse(fmt.Sprintf("%v", rawUID))
		if err != nil {
			response.Forbidden(c, "invalid user identity")
			c.Abort()
			return
		}

		perms, err := loadPermissions(c.Request.Context(), queries, redisClient, userID)
		if err != nil {
			response.Forbidden(c, "unable to load permissions")
			c.Abort()
			return
		}

		// Surface admin status so downstream handlers can make ownership
		// decisions (e.g. self-or-admin checks on user-scoped resources)
		// without re-querying the database.
		c.Set("is_admin", isSuperAdmin(perms))

		if !checkPermission(perms, resource, action) {
			response.Forbidden(c, fmt.Sprintf("insufficient permissions for %s:%s", resource, action))
			c.Abort()
			return
		}

		c.Next()
	}
}

// isSuperAdmin reports whether the merged permission map grants wildcard
// access on every resource ("*":["*"]).
func isSuperAdmin(perms map[string][]string) bool {
	wildcard, ok := perms["*"]
	return ok && containsStr(wildcard, "*")
}

// extractResource parses the first segment after /api/v1/ and maps it to
// a resource name via the currently-configured resourceMap. Returns "" for
// unrecognised paths.
//
// This is kept as a thin shim over extractResourceWith so existing tests
// and callers that do not inject a map continue to work.
func extractResource(path string) string {
	rt := currentRBAC()
	if rt == nil {
		return ""
	}
	return extractResourceWith(path, rt.resourceMap)
}

// extractResourceWith is the pure form used by RBAC's hot path — it takes
// an explicit resourceMap so the closure holds it directly with no
// package-state indirection per request.
func extractResourceWith(path string, resourceMap map[string]string) string {
	const prefix = "/api/v1/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := path[len(prefix):]
	seg := strings.SplitN(rest, "/", 2)[0]
	return resourceMap[seg]
}

// loadPermissions tries Redis first, then falls back to DB.
func loadPermissions(ctx context.Context, queries *dbgen.Queries, rc *redis.Client, userID uuid.UUID) (map[string][]string, error) {
	cacheKey := fmt.Sprintf("perms:%s", userID.String())

	// Try Redis cache.
	val, err := rc.Get(ctx, cacheKey).Result()
	if err == nil {
		var perms map[string][]string
		if jsonErr := json.Unmarshal([]byte(val), &perms); jsonErr == nil {
			return perms, nil
		}
	}

	// Fallback: query DB and merge permissions from all roles.
	roles, err := queries.ListUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	merged := mergePermissions(roles)

	// Cache in Redis (best-effort).
	if data, err := json.Marshal(merged); err == nil {
		_ = rc.Set(ctx, cacheKey, string(data), permsCacheTTL).Err()
	}

	return merged, nil
}

// mergePermissions combines permissions JSON from all roles into a single map.
func mergePermissions(roles []dbgen.Role) map[string][]string {
	merged := make(map[string][]string)
	for _, role := range roles {
		var rp map[string][]string
		if err := json.Unmarshal(role.Permissions, &rp); err != nil {
			continue
		}
		for resource, actions := range rp {
			existing := merged[resource]
			for _, a := range actions {
				if !containsStr(existing, a) {
					existing = append(existing, a)
				}
			}
			merged[resource] = existing
		}
	}
	return merged
}

// checkPermission evaluates whether the merged permission map grants the
// requested action on the given resource.
func checkPermission(perms map[string][]string, resource, action string) bool {
	// Super-admin: wildcard resource with wildcard action.
	if isSuperAdmin(perms) {
		return true
	}

	actions, ok := perms[resource]
	if !ok {
		return false
	}

	// Direct match.
	if containsStr(actions, action) {
		return true
	}

	// "write" implies "read".
	if action == "read" && containsStr(actions, "write") {
		return true
	}

	return false
}

// containsStr reports whether s is present in the slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// AuthBypassPaths returns the subset of publicPaths that should also skip
// the Auth middleware (login, refresh, ws, healthz, metrics — everything
// except the explicit "authenticated-but-RBAC-public" exception:
// /api/v1/auth/logout requires a valid access token so the handler can
// revoke the jti, even though it is RBAC-public).
//
// Callers MUST invoke ConfigureRBAC before calling this. Returns a fresh
// map safe for the caller to retain.
func AuthBypassPaths() map[string]struct{} {
	rt := currentRBAC()
	if rt == nil {
		panic("middleware.AuthBypassPaths: ConfigureRBAC must be called first")
	}
	const logoutPath = "/api/v1/auth/logout"
	out := make(map[string]struct{}, len(rt.publicPaths))
	for p := range rt.publicPaths {
		if p == logoutPath {
			continue
		}
		out[p] = struct{}{}
	}
	return out
}
