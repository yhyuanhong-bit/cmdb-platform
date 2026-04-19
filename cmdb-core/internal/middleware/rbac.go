package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// permsCacheTTL is how long merged permissions stay in Redis.
const permsCacheTTL = 5 * time.Minute

// publicPaths that bypass RBAC entirely.
var publicPaths = map[string]bool{
	"/api/v1/auth/login":   true,
	"/api/v1/auth/refresh": true,
	"/api/v1/ws":           true,
	"/healthz":             true,
	"/metrics":             true,
}

// resourceMap maps the first path segment after /api/v1/ to a resource name.
var resourceMap = map[string]string{
	"assets":        "assets",
	"locations":     "topology",
	"racks":         "topology",
	"maintenance":   "maintenance",
	"monitoring":    "monitoring",
	"inventory":     "inventory",
	"audit":         "audit",
	"dashboard":     "dashboard",
	"users":         "identity",
	"roles":         "identity",
	"auth":          "identity",
	"prediction":    "prediction",
	"integration":   "integration",
	"system":        "system",
	"energy":        "monitoring",
	"sensors":       "monitoring",
	"topology":      "topology",
	"activity-feed": "audit",
	"quality":       "system",
	"bia":           "system",
	"discovery":     "assets",
	"upgrade-rules": "assets",
	"sync":          "sync",
	"notifications":     "system",
	"capacity-planning": "assets",
	"fleet-metrics":     "monitoring",
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

// RBAC returns a Gin middleware that enforces permission checks based on the
// authenticated user's roles. Permissions are cached in Redis (key perms:{user_id})
// with a 5-minute TTL and fall back to a DB query via ListUserRoles.
func RBAC(queries *dbgen.Queries, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip public paths.
		if publicPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Extract resource from path.
		resource := extractResource(c.Request.URL.Path)
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

// extractResource parses the first segment after /api/v1/ and maps it to a
// resource name via resourceMap. Returns "" for unrecognised paths.
func extractResource(path string) string {
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
