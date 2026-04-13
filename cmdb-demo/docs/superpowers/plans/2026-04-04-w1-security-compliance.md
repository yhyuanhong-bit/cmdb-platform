# W1: Security & Compliance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add RBAC permission enforcement middleware and automatic audit logging for all write operations — making the platform secure and compliance-ready.

**Architecture:** RBAC middleware sits between JWT auth and handlers, maps HTTP method+path to resource+action, checks against user's merged role permissions (cached in Redis 5min). Audit logging is synchronous — each write handler in impl.go calls auditSvc.Record() after success. Both use existing infrastructure (roles table, audit_events table, Redis cache).

**Tech Stack:** Go (Gin middleware), existing sqlc/pgx stack, Redis for permission cache

**Spec Reference:** `docs/platform-completion-roadmap.md` — Section 1.1 and 1.2

---

## File Structure

```
cmdb-core/
├── internal/middleware/
│   └── rbac.go                          # NEW: permission check middleware
├── internal/domain/audit/
│   └── service.go                       # MODIFY: add Record() method
├── internal/api/
│   └── impl.go                          # MODIFY: add audit calls to 7 write methods
├── cmd/server/
│   └── main.go                          # MODIFY: mount RBAC middleware + pass auditSvc to APIServer
└── db/seed/
    └── seed.sql                         # MODIFY: ensure role permissions are complete
```

---

## Task 1: RBAC Permission Middleware

**Files:**
- Create: `cmdb-core/internal/middleware/rbac.go`
- Modify: `cmdb-core/db/seed/seed.sql`

- [ ] **Step 1: Create rbac.go**

Create `cmdb-core/internal/middleware/rbac.go`.

Read these files first to understand the existing middleware pattern and types:
```bash
cat internal/middleware/auth.go
cat internal/middleware/cors.go
```

The RBAC middleware must:
1. Run AFTER auth middleware (user_id is already in gin context)
2. Skip public paths (login, refresh, healthz, metrics)
3. Extract resource + action from HTTP method + path
4. Load user permissions (from Redis cache or DB)
5. Check if permissions allow the action
6. Return 403 Forbidden if not allowed

```go
package middleware

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/cmdb-platform/cmdb-core/internal/dbgen"
    "github.com/cmdb-platform/cmdb-core/internal/platform/response"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
)

// RBAC checks if the authenticated user has permission to access the requested resource.
func RBAC(queries *dbgen.Queries, redisClient *redis.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

        // Skip public/system paths
        if isPublicPath(path) {
            c.Next()
            return
        }

        // Skip if no user_id (auth middleware handles 401)
        userIDStr := c.GetString("user_id")
        if userIDStr == "" {
            c.Next()
            return
        }

        userID, err := uuid.Parse(userIDStr)
        if err != nil {
            c.Next()
            return
        }

        // Resolve resource + action from path + method
        resource := resolveResource(path)
        action := resolveAction(method)

        // Load permissions (cached)
        perms, err := loadPermissions(c.Request.Context(), queries, redisClient, userID)
        if err != nil {
            response.InternalError(c, "failed to load permissions")
            c.Abort()
            return
        }

        // Check permission
        if !hasPermission(perms, resource, action) {
            response.Forbidden(c, fmt.Sprintf("insufficient permissions for %s:%s", resource, action))
            c.Abort()
            return
        }

        c.Next()
    }
}

func isPublicPath(path string) bool {
    publicPrefixes := []string{
        "/api/v1/auth/login",
        "/api/v1/auth/refresh",
        "/healthz",
        "/metrics",
    }
    for _, p := range publicPrefixes {
        if strings.HasPrefix(path, p) {
            return true
        }
    }
    return false
}

func resolveResource(path string) string {
    // Strip /api/v1/ prefix and get first segment
    trimmed := strings.TrimPrefix(path, "/api/v1/")
    parts := strings.Split(trimmed, "/")
    if len(parts) == 0 {
        return "unknown"
    }

    segment := parts[0]
    resourceMap := map[string]string{
        "assets":      "assets",
        "locations":   "topology",
        "racks":       "topology",
        "maintenance": "maintenance",
        "monitoring":  "monitoring",
        "inventory":   "inventory",
        "audit":       "audit",
        "dashboard":   "dashboard",
        "users":       "identity",
        "roles":       "identity",
        "auth":        "identity",
        "prediction":  "prediction",
        "integration": "integration",
        "system":      "system",
    }

    if r, ok := resourceMap[segment]; ok {
        return r
    }
    return segment
}

func resolveAction(method string) string {
    switch method {
    case "GET":
        return "read"
    case "POST", "PUT", "PATCH":
        return "write"
    case "DELETE":
        return "delete"
    default:
        return "read"
    }
}

// loadPermissions returns merged permissions for a user, cached in Redis for 5 min.
func loadPermissions(ctx context.Context, queries *dbgen.Queries, redisClient *redis.Client, userID uuid.UUID) (map[string][]string, error) {
    cacheKey := fmt.Sprintf("perms:%s", userID.String())

    // Try Redis cache first
    if redisClient != nil {
        cached, err := redisClient.Get(ctx, cacheKey).Result()
        if err == nil {
            var perms map[string][]string
            if json.Unmarshal([]byte(cached), &perms) == nil {
                return perms, nil
            }
        }
    }

    // Cache miss — load from DB
    roles, err := queries.ListUserRoles(ctx, userID)
    if err != nil {
        return nil, err
    }

    merged := make(map[string][]string)
    for _, role := range roles {
        var rolePerms map[string][]string
        if err := json.Unmarshal(role.Permissions, &rolePerms); err != nil {
            continue
        }
        for resource, actions := range rolePerms {
            merged[resource] = appendUnique(merged[resource], actions...)
        }
    }

    // Write to Redis cache (5 min TTL)
    if redisClient != nil {
        data, _ := json.Marshal(merged)
        redisClient.Set(ctx, cacheKey, string(data), 5*time.Minute)
    }

    return merged, nil
}

func hasPermission(perms map[string][]string, resource, action string) bool {
    // Super-admin wildcard
    if actions, ok := perms["*"]; ok {
        for _, a := range actions {
            if a == "*" {
                return true
            }
        }
    }

    // Check specific resource
    actions, ok := perms[resource]
    if !ok {
        return false
    }
    for _, a := range actions {
        if a == action || a == "*" {
            return true
        }
    }

    // "write" permission implies "read"
    if action == "read" {
        for _, a := range actions {
            if a == "write" || a == "delete" {
                return true
            }
        }
    }

    return false
}

func appendUnique(existing []string, items ...string) []string {
    seen := make(map[string]bool)
    for _, e := range existing {
        seen[e] = true
    }
    for _, item := range items {
        if !seen[item] {
            existing = append(existing, item)
            seen[item] = true
        }
    }
    return existing
}
```

- [ ] **Step 2: Update role permissions in seed.sql**

Read `db/seed/seed.sql`, find the roles INSERT, and update to ensure permissions are complete:

```sql
-- ops-admin: full CRUD on assets/maintenance/monitoring, read on others
UPDATE roles SET permissions = '{"assets":["read","write","delete"],"maintenance":["read","write"],"monitoring":["read","write"],"topology":["read"],"inventory":["read","write"],"audit":["read"],"dashboard":["read"],"prediction":["read"],"system":["read"]}'
WHERE name = 'ops-admin';

-- viewer: read-only everywhere
UPDATE roles SET permissions = '{"assets":["read"],"topology":["read"],"maintenance":["read"],"monitoring":["read"],"inventory":["read"],"audit":["read"],"dashboard":["read"]}'
WHERE name = 'viewer';
```

Also update the INSERT in seed.sql so fresh seeds have correct permissions.

- [ ] **Step 3: Verify build**

```bash
go build ./internal/middleware/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/middleware/rbac.go db/seed/seed.sql
git commit -m "feat: add RBAC permission middleware with Redis cache + complete role definitions"
```

---

## Task 2: Mount RBAC Middleware in main.go

**Files:**
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Read current main.go**

```bash
cat cmd/server/main.go
```

Find where the auth middleware is applied and the route registration happens.

- [ ] **Step 2: Add RBAC middleware after auth**

The current setup has a custom auth-skipping middleware. Add RBAC after it:

```go
// Find this block in main.go:
v1.Use(func(c *gin.Context) {
    path := c.Request.URL.Path
    if path == "/api/v1/auth/login" || path == "/api/v1/auth/refresh" {
        c.Next()
        return
    }
    authMW(c)
})

// ADD RBAC middleware right after:
v1.Use(middleware.RBAC(queries, redisClient))
```

This ensures:
1. Auth runs first → sets user_id in context
2. RBAC runs next → checks permissions
3. Handler runs last

- [ ] **Step 3: Apply role permissions to running DB**

```bash
cd deploy && docker compose exec -T postgres psql -U cmdb -d cmdb << 'SQL'
UPDATE roles SET permissions = '{"assets":["read","write","delete"],"maintenance":["read","write"],"monitoring":["read","write"],"topology":["read"],"inventory":["read","write"],"audit":["read"],"dashboard":["read"],"prediction":["read"],"system":["read"]}'
WHERE name = 'ops-admin';

UPDATE roles SET permissions = '{"assets":["read"],"topology":["read"],"maintenance":["read"],"monitoring":["read"],"inventory":["read"],"audit":["read"],"dashboard":["read"]}'
WHERE name = 'viewer';
SQL
```

- [ ] **Step 4: Rebuild and test RBAC**

```bash
go build ./cmd/server

# Restart server, then test:
# 1. Login as admin (super-admin) — should access everything
TOKEN_ADMIN=$(curl -s -X POST .../auth/login -d '{"username":"admin","password":"admin123"}' | ...)
curl -s .../assets -H "Authorization: Bearer $TOKEN_ADMIN"
# Expected: 200 OK

# 2. Login as mike.chen (viewer) — should be read-only
TOKEN_VIEWER=$(curl -s -X POST .../auth/login -d '{"username":"mike.chen","password":"admin123"}' | ...)
curl -s .../assets -H "Authorization: Bearer $TOKEN_VIEWER"
# Expected: 200 OK (GET = read)

curl -s -X POST .../maintenance/orders -H "Authorization: Bearer $TOKEN_VIEWER" -d '{"title":"test","type":"inspection"}'
# Expected: 403 Forbidden (POST = write, viewer has no maintenance write)
```

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: mount RBAC middleware in request pipeline after JWT auth"
```

---

## Task 3: Audit Service Record Method

**Files:**
- Modify: `cmdb-core/internal/domain/audit/service.go`

- [ ] **Step 1: Read current audit service**

```bash
cat internal/domain/audit/service.go
```

- [ ] **Step 2: Add Record method**

Add to `internal/domain/audit/service.go`:

```go
// Record creates a new audit event entry.
func (s *Service) Record(ctx context.Context, tenantID uuid.UUID, action, module, targetType string, targetID, operatorID uuid.UUID, diff map[string]any, source string) error {
    diffJSON, _ := json.Marshal(diff)

    _, err := s.queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
        TenantID:   tenantID,
        Action:     action,
        Module:     pgtype.Text{String: module, Valid: true},
        TargetType: pgtype.Text{String: targetType, Valid: true},
        TargetID:   pgtype.UUID{Bytes: targetID, Valid: true},
        OperatorID: pgtype.UUID{Bytes: operatorID, Valid: true},
        Diff:       diffJSON,
        Source:     source,
    })
    return err
}
```

Add necessary imports: `"encoding/json"`, `"github.com/jackc/pgx/v5/pgtype"`.

- [ ] **Step 3: Verify build**

```bash
go build ./internal/domain/audit/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/domain/audit/service.go
git commit -m "feat: add audit service Record() method for writing audit events"
```

---

## Task 4: Add Audit Logging to All Write Operations

**Files:**
- Modify: `cmdb-core/internal/api/impl.go`

This is the main task — add `auditSvc.Record()` calls to all 7 write methods.

- [ ] **Step 1: Read impl.go write methods**

```bash
grep -n "func (s \*APIServer).*Create\|func (s \*APIServer).*Transition\|func (s \*APIServer).*Acknowledge\|func (s \*APIServer).*Resolve\|func (s \*APIServer).*Verify" internal/api/impl.go
```

Read each method to understand where the success point is (after the service call succeeds, before the response).

- [ ] **Step 2: Add audit helper to impl.go**

Add a helper function at the top of impl.go to reduce boilerplate:

```go
// recordAudit logs an audit event. Errors are logged but don't fail the request.
func (s *APIServer) recordAudit(ctx context.Context, c *gin.Context, action, module, targetType string, targetID uuid.UUID, diff map[string]any) {
    tenantID := getTenantID(c)
    operatorID, _ := uuid.Parse(c.GetString("user_id"))
    if err := s.auditSvc.Record(ctx, tenantID, action, module, targetType, targetID, operatorID, diff, "api"); err != nil {
        // Log but don't fail the request
        zap.L().Warn("audit record failed", zap.Error(err), zap.String("action", action))
    }
}
```

Add import for `"go.uber.org/zap"` if not already present.

- [ ] **Step 3: Add audit calls to each write method**

For each of the 7 write methods, add the audit call AFTER the successful service call and BEFORE the response.

**CreateAsset:**
```go
// After: created, err := s.assetSvc.Create(...)
s.recordAudit(ctx, c, "asset.created", "asset", "asset", created.ID, map[string]any{
    "asset_tag": created.AssetTag,
    "name":      created.Name,
    "type":      created.Type,
})
```

**CreateWorkOrder:**
```go
// After: order, err := s.maintenanceSvc.Create(...)
s.recordAudit(ctx, c, "order.created", "maintenance", "work_order", order.ID, map[string]any{
    "code":     order.Code,
    "title":    order.Title,
    "priority": order.Priority,
})
```

**TransitionWorkOrder:**
```go
// After: updated, err := s.maintenanceSvc.Transition(...)
s.recordAudit(ctx, c, "order.transitioned", "maintenance", "work_order", uuid.UUID(id), map[string]any{
    "status": map[string]any{"new": req.Status},
})
```

**AcknowledgeAlert:**
```go
// After: alert, err := s.monitoringSvc.Acknowledge(...)
s.recordAudit(ctx, c, "alert.acknowledged", "monitoring", "alert", uuid.UUID(id), map[string]any{
    "status": map[string]any{"old": "firing", "new": "acknowledged"},
})
```

**ResolveAlert:**
```go
// After: alert, err := s.monitoringSvc.Resolve(...)
s.recordAudit(ctx, c, "alert.resolved", "monitoring", "alert", uuid.UUID(id), map[string]any{
    "status": map[string]any{"old": "firing/acknowledged", "new": "resolved"},
})
```

**CreateRCA:**
```go
// After: rca, err := s.predictionSvc.CreateRCA(...)
s.recordAudit(ctx, c, "rca.created", "prediction", "rca", rca.ID, map[string]any{
    "incident_id": req.IncidentId,
})
```

**VerifyRCA:**
```go
// After: rca, err := s.predictionSvc.VerifyRCA(...)
s.recordAudit(ctx, c, "rca.verified", "prediction", "rca", uuid.UUID(id), map[string]any{
    "human_verified": true,
})
```

IMPORTANT: Read each method carefully. The variable names for the successful result (e.g., `created`, `order`, `updated`, `alert`, `rca`) may differ. Match them exactly.

IMPORTANT: `getTenantID(c)` and getting `user_id` from gin context must work. Check how other methods do it.

- [ ] **Step 4: Verify build**

```bash
go build ./...
```

- [ ] **Step 5: Test audit logging**

```bash
# Restart server, then:

# 1. Create a work order
TOKEN=$(curl -s -X POST .../auth/login -d '{"username":"admin","password":"admin123"}' | ...)
curl -s -X POST .../maintenance/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Audit Test Order","type":"inspection","priority":"low"}'

# 2. Check audit events
curl -s ".../audit/events?module=maintenance" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json
d = json.load(sys.stdin)
for e in d['data']:
    print(f'{e[\"action\"]:25} | {e[\"module\"]:15} | {e[\"target_type\"]}')
"
# Expected: should see 'order.created | maintenance | work_order' at the top
```

- [ ] **Step 6: Commit**

```bash
git add internal/api/impl.go
git commit -m "feat: add audit logging to all 7 write operations (create/transition/ack/resolve/verify)"
```

---

## Verification

After all 4 tasks, verify both RBAC and Audit work end-to-end:

```bash
# === RBAC Test ===

# Admin (super-admin) — can do everything
curl -s -X POST .../maintenance/orders -H "Authorization: Bearer $TOKEN_ADMIN" ...
# Expected: 201 Created

# Viewer — cannot write
curl -s -X POST .../maintenance/orders -H "Authorization: Bearer $TOKEN_VIEWER" ...
# Expected: 403 {"error":{"code":"FORBIDDEN","message":"insufficient permissions for maintenance:write"}}

# Viewer — can read
curl -s .../assets -H "Authorization: Bearer $TOKEN_VIEWER"
# Expected: 200 OK

# === Audit Test ===

# After any write operation, check audit events:
curl -s .../audit/events -H "Authorization: Bearer $TOKEN_ADMIN"
# Expected: new audit entries beyond the original 10 seed entries

# Check specific target:
curl -s ".../audit/events?target_type=work_order" -H "Authorization: Bearer $TOKEN_ADMIN"
# Expected: order.created events for any orders created during testing
```

## Summary

| Task | Files | Outcome |
|------|-------|---------|
| 1 | `middleware/rbac.go` + seed | RBAC middleware with Redis-cached permissions |
| 2 | `main.go` | RBAC mounted in request pipeline |
| 3 | `audit/service.go` | Record() method for writing audit events |
| 4 | `impl.go` | 7 write methods log audit events |

Score impact: **72 → 80** (RBAC 0%→100%, Audit 0%→100%)
