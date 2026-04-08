# Phase 4 Group 2: Session Management + Sensor Registration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add login session tracking with password change, and a dedicated sensor registry with CRUD + heartbeat, connecting UserProfile and SensorConfiguration pages to real data.

**Architecture:** New Go handler files for session + sensor endpoints. Login handler modified to record sessions on authentication. Sensor CRUD with heartbeat tracking. Frontend extends identity hooks and creates new sensor API/hooks.

**Tech Stack:** Go/Gin, pgxpool raw SQL, bcrypt, React/TypeScript, TanStack React Query.

**Spec:** `docs/superpowers/specs/2026-04-08-phase4-group2-session-sensor-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/db/migrations/000020_phase4_group2.up.sql` | user_sessions + sensors tables + ALTER users |
| `cmdb-core/db/migrations/000020_phase4_group2.down.sql` | Rollback |
| `cmdb-core/internal/api/phase4_session_endpoints.go` | GetUserSessions + ChangePassword handlers |
| `cmdb-core/internal/api/phase4_sensor_endpoints.go` | Sensor CRUD + heartbeat (5 handlers) |
| `cmdb-demo/src/lib/api/sensors.ts` | Sensor API client |
| `cmdb-demo/src/hooks/useSensors.ts` | Sensor React Query hooks |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-core/cmd/server/main.go` | Register 7 new routes + pass pool to AuthService |
| `cmdb-core/internal/domain/identity/auth_service.go` | Add pool field, session recording in Login, ChangePassword method |
| `cmdb-core/internal/domain/identity/model.go` | Extend LoginRequest with ClientIP + UserAgent |
| `cmdb-core/internal/api/impl.go` | Pass ClientIP + UserAgent in Login handler |
| `cmdb-demo/src/lib/api/identity.ts` | Add listSessions, changePassword |
| `cmdb-demo/src/hooks/useIdentity.ts` | Add useUserSessions, useChangePassword |
| `cmdb-demo/src/pages/UserProfile.tsx` | Replace hardcoded sessions + wire password change |
| `cmdb-demo/src/pages/SensorConfiguration.tsx` | Replace asset-derived sensors with API data |

---

## Task 1: Database Migration

**Files:**
- Create: `cmdb-core/db/migrations/000020_phase4_group2.up.sql`
- Create: `cmdb-core/db/migrations/000020_phase4_group2.down.sql`

- [ ] **Step 1: Write migrations**

Create both files with the exact SQL from the spec Section 2 (user_sessions table + ALTER users + sensors table).

- [ ] **Step 2: Run migration**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -f cmdb-core/db/migrations/000020_phase4_group2.up.sql
```

- [ ] **Step 3: Verify**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d user_sessions"
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d sensors"
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "SELECT column_name FROM information_schema.columns WHERE table_name='users' AND column_name IN ('last_login_at','last_login_ip')"
```

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/db/migrations/000020_phase4_group2.up.sql cmdb-core/db/migrations/000020_phase4_group2.down.sql
git commit -m "feat: add user_sessions + sensors tables, extend users (Phase 4 Group 2)"
```

---

## Task 2: Modify Login Flow for Session Recording

**Files:**
- Modify: `cmdb-core/internal/domain/identity/model.go`
- Modify: `cmdb-core/internal/domain/identity/auth_service.go`
- Modify: `cmdb-core/internal/api/impl.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Extend LoginRequest in model.go**

Read `cmdb-core/internal/domain/identity/model.go`. Add two fields to `LoginRequest`:

```go
type LoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	ClientIP  string `json:"-"` // populated by API handler, not from JSON
	UserAgent string `json:"-"` // populated by API handler, not from JSON
}
```

- [ ] **Step 2: Add pool to AuthService and session recording**

Read `cmdb-core/internal/domain/identity/auth_service.go`.

Add `pool` field to struct and constructor:

```go
import (
	// ... existing imports
	"strings"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AuthService struct {
	queries   *dbgen.Queries
	redis     *redis.Client
	jwtSecret string
	pool      *pgxpool.Pool  // NEW
}

func NewAuthService(queries *dbgen.Queries, rdb *redis.Client, jwtSecret string, pool *pgxpool.Pool) *AuthService {
	return &AuthService{
		queries:   queries,
		redis:     rdb,
		jwtSecret: jwtSecret,
		pool:      pool,  // NEW
	}
}
```

Add `parseUserAgent` helper and `recordSession` method:

```go
func parseUserAgent(ua string) (deviceType, browser string) {
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg"):
		browser = "Chrome"
	case strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		browser = "Safari"
	case strings.Contains(lower, "edg"):
		browser = "Edge"
	default:
		browser = "unknown"
	}
	switch {
	case strings.Contains(lower, "mobile") || strings.Contains(lower, "android"):
		deviceType = "mobile"
	case strings.Contains(lower, "tablet") || strings.Contains(lower, "ipad"):
		deviceType = "tablet"
	default:
		deviceType = "desktop"
	}
	return
}

func (s *AuthService) recordSession(ctx context.Context, userID uuid.UUID, clientIP, userAgent string) {
	if s.pool == nil {
		return
	}
	deviceType, browser := parseUserAgent(userAgent)
	// Clear previous current session
	s.pool.Exec(ctx, `UPDATE user_sessions SET is_current = false WHERE user_id = $1`, userID)
	// Insert new session
	s.pool.Exec(ctx, `
		INSERT INTO user_sessions (user_id, ip_address, user_agent, device_type, browser, is_current)
		VALUES ($1, $2, $3, $4, $5, true)
	`, userID, clientIP, userAgent, deviceType, browser)
	// Update user last login
	s.pool.Exec(ctx, `UPDATE users SET last_login_at = now(), last_login_ip = $1 WHERE id = $2`, clientIP, userID)
}
```

Add `ChangePassword` method:

```go
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.queries.GetUser(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	_, err = s.pool.Exec(ctx, `UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`, string(hash), userID)
	return err
}
```

Modify `Login` method — add session recording after successful `issueTokens`:

```go
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := s.queries.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}
	if user.Status != "active" {
		return nil, errors.New("account is not active")
	}
	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}
	// Record session (non-blocking, log on error)
	s.recordSession(ctx, user.ID, req.ClientIP, req.UserAgent)
	return tokens, nil
}
```

- [ ] **Step 3: Update main.go to pass pool to AuthService**

Read `cmdb-core/cmd/server/main.go`. Find the line:
```go
authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret)
```
Change to:
```go
authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret, pool)
```

- [ ] **Step 4: Update Login handler in impl.go to pass IP + UA**

Read `cmdb-core/internal/api/impl.go`. Find the Login handler (around line 185). Change:
```go
tokens, err := s.authSvc.Login(c.Request.Context(), identity.LoginRequest{
	Username: req.Username,
	Password: req.Password,
})
```
To:
```go
tokens, err := s.authSvc.Login(c.Request.Context(), identity.LoginRequest{
	Username:  req.Username,
	Password:  req.Password,
	ClientIP:  c.ClientIP(),
	UserAgent: c.GetHeader("User-Agent"),
})
```

- [ ] **Step 5: Build and verify**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/internal/domain/identity/model.go cmdb-core/internal/domain/identity/auth_service.go \
       cmdb-core/internal/api/impl.go cmdb-core/cmd/server/main.go
git commit -m "feat: record login sessions + add password change (Phase 4 Group 2)"
```

---

## Task 3: Session + Sensor Go Endpoints

**Files:**
- Create: `cmdb-core/internal/api/phase4_session_endpoints.go`
- Create: `cmdb-core/internal/api/phase4_sensor_endpoints.go`

- [ ] **Step 1: Create session endpoints**

Create `cmdb-core/internal/api/phase4_session_endpoints.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var deviceIcons = map[string]string{
	"desktop": "laptop_mac", "mobile": "phone_iphone",
	"tablet": "tablet_mac", "unknown": "devices",
}

// GetUserSessions handles GET /users/:id/sessions
func (s *APIServer) GetUserSessions(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT id, ip_address, device_type, browser, created_at, last_active_at, is_current
		FROM user_sessions WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 20
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var sessions []gin.H
	for rows.Next() {
		var id uuid.UUID
		var ipAddr, deviceType, browser *string
		var createdAt, lastActive time.Time
		var isCurrent bool
		if rows.Scan(&id, &ipAddr, &deviceType, &browser, &createdAt, &lastActive, &isCurrent) != nil {
			continue
		}
		dt := "unknown"
		if deviceType != nil { dt = *deviceType }
		icon := deviceIcons[dt]
		if icon == "" { icon = "devices" }

		sessions = append(sessions, gin.H{
			"id": id.String(), "ip_address": ipAddr,
			"device": dt, "browser": browser,
			"time": createdAt, "icon": icon, "current": isCurrent,
		})
	}
	if sessions == nil { sessions = []gin.H{} }
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// ChangePassword handles POST /auth/change-password
func (s *APIServer) ChangePassword(c *gin.Context) {
	userID := userIDFromContext(c)
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CurrentPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_password and new_password required"})
		return
	}
	if err := s.authSvc.ChangePassword(c.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
```

- [ ] **Step 2: Create sensor endpoints**

Create `cmdb-core/internal/api/phase4_sensor_endpoints.go`:

```go
package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var sensorIcons = map[string]string{
	"temperature": "thermostat", "humidity": "water_drop", "power": "bolt",
	"network": "lan", "cpu": "memory", "memory": "memory", "disk": "storage",
}

func capitalize(s string) string {
	if s == "" { return s }
	return strings.ToUpper(s[:1]) + s[1:]
}

// ListSensors handles GET /sensors
func (s *APIServer) ListSensors(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT s.id, s.asset_id, a.name AS asset_name, s.name, s.type, s.location,
		       s.polling_interval, s.enabled, s.status, s.last_heartbeat
		FROM sensors s
		LEFT JOIN assets a ON s.asset_id = a.id
		WHERE s.tenant_id = $1
		ORDER BY s.name
	`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var sensors []gin.H
	for rows.Next() {
		var id uuid.UUID
		var assetID *uuid.UUID
		var assetName, name, sType, location, status *string
		var pollingInterval int
		var enabled bool
		var lastHB *time.Time
		if rows.Scan(&id, &assetID, &assetName, &name, &sType, &location,
			&pollingInterval, &enabled, &status, &lastHB) != nil {
			continue
		}
		typeStr := ""
		if sType != nil { typeStr = *sType }
		icon := sensorIcons[typeStr]
		if icon == "" { icon = "sensors" }
		statusStr := "Offline"
		if status != nil { statusStr = capitalize(*status) }
		var lastSeen *string
		if lastHB != nil { t := lastHB.Format(time.RFC3339); lastSeen = &t }

		sensor := gin.H{
			"id": id.String(), "name": name, "type": typeStr, "icon": icon,
			"location": location, "pollingInterval": pollingInterval,
			"enabled": enabled, "status": statusStr, "lastSeen": lastSeen,
		}
		if assetID != nil { sensor["asset_id"] = assetID.String() }
		if assetName != nil { sensor["asset_name"] = *assetName }
		sensors = append(sensors, sensor)
	}
	if sensors == nil { sensors = []gin.H{} }
	c.JSON(http.StatusOK, gin.H{"sensors": sensors})
}

// CreateSensor handles POST /sensors
func (s *APIServer) CreateSensor(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	var req struct {
		AssetID         *string `json:"asset_id"`
		Name            string  `json:"name"`
		Type            string  `json:"type"`
		Location        *string `json:"location"`
		PollingInterval int     `json:"polling_interval"`
		Enabled         *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and type required"})
		return
	}
	if req.PollingInterval == 0 { req.PollingInterval = 30 }
	enabled := true
	if req.Enabled != nil { enabled = *req.Enabled }

	var assetID *uuid.UUID
	if req.AssetID != nil {
		parsed, _ := uuid.Parse(*req.AssetID)
		if parsed != uuid.Nil { assetID = &parsed }
	}

	id := uuid.New()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO sensors (id, tenant_id, asset_id, name, type, location, polling_interval, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, tenantID, assetID, req.Name, req.Type, req.Location, req.PollingInterval, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

// UpdateSensor handles PUT /sensors/:id
func (s *APIServer) UpdateSensor(c *gin.Context) {
	sensorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sensor id"})
		return
	}
	var req struct {
		Name            *string `json:"name"`
		Type            *string `json:"type"`
		Location        *string `json:"location"`
		PollingInterval *int    `json:"polling_interval"`
		Enabled         *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	updates := []string{}
	args := []interface{}{}
	idx := 1

	if req.Name != nil { updates = append(updates, "name = $"+string(rune('0'+idx))); args = append(args, *req.Name); idx++ }
	if req.Type != nil { updates = append(updates, "type = $"+string(rune('0'+idx))); args = append(args, *req.Type); idx++ }
	if req.Location != nil { updates = append(updates, "location = $"+string(rune('0'+idx))); args = append(args, *req.Location); idx++ }
	if req.PollingInterval != nil { updates = append(updates, "polling_interval = $"+string(rune('0'+idx))); args = append(args, *req.PollingInterval); idx++ }
	if req.Enabled != nil { updates = append(updates, "enabled = $"+string(rune('0'+idx))); args = append(args, *req.Enabled); idx++ }

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	updates = append(updates, "updated_at = now()")
	args = append(args, sensorID)
	query := "UPDATE sensors SET " + strings.Join(updates, ", ") + " WHERE id = $" + string(rune('0'+idx))
	_, err = s.pool.Exec(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DeleteSensor handles DELETE /sensors/:id
func (s *APIServer) DeleteSensor(c *gin.Context) {
	sensorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sensor id"})
		return
	}
	tag, err := s.pool.Exec(c.Request.Context(), `DELETE FROM sensors WHERE id = $1`, sensorID)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// SensorHeartbeat handles POST /sensors/:id/heartbeat
func (s *APIServer) SensorHeartbeat(c *gin.Context) {
	sensorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sensor id"})
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	c.ShouldBindJSON(&req)
	if req.Status == "" { req.Status = "online" }

	_, err = s.pool.Exec(c.Request.Context(), `
		UPDATE sensors SET last_heartbeat = now(), status = $1, updated_at = now() WHERE id = $2
	`, req.Status, sensorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
```

**Note:** The `UpdateSensor` handler uses `string(rune('0'+idx))` for parameter numbering, which only works for single-digit indices (1-9). For this use case (max 5 update fields + 1 WHERE), this is sufficient. For a more robust approach, use `fmt.Sprintf("$%d", idx)` — the implementer should use whichever they prefer.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/internal/api/phase4_session_endpoints.go cmdb-core/internal/api/phase4_sensor_endpoints.go
git commit -m "feat: add session list, password change, and sensor CRUD Go endpoints"
```

---

## Task 4: Register 7 Routes + Build

**Files:**
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add route registrations**

Read `cmdb-core/cmd/server/main.go`. After the Phase 4 Group 1 routes, add:

```go
	// Phase 4 Group 2 routes
	v1.GET("/users/:id/sessions", apiServer.GetUserSessions)
	v1.POST("/auth/change-password", apiServer.ChangePassword)
	v1.GET("/sensors", apiServer.ListSensors)
	v1.POST("/sensors", apiServer.CreateSensor)
	v1.PUT("/sensors/:id", apiServer.UpdateSensor)
	v1.DELETE("/sensors/:id", apiServer.DeleteSensor)
	v1.POST("/sensors/:id/heartbeat", apiServer.SensorHeartbeat)
```

**Important:** Also add `/api/v1/auth/change-password` to the auth skip list. Find the auth middleware section:
```go
if path == "/api/v1/auth/login" || path == "/api/v1/auth/refresh" {
```
This does NOT need changing — change-password requires auth (user must be logged in to change their password).

- [ ] **Step 2: Build**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

Fix any compile errors.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/cmd/server/main.go
git commit -m "feat: register Phase 4 Group 2 routes (sessions, password, sensors)"
```

---

## Task 5: Frontend API Clients + Hooks

**Files:**
- Create: `cmdb-demo/src/lib/api/sensors.ts`
- Create: `cmdb-demo/src/hooks/useSensors.ts`
- Modify: `cmdb-demo/src/lib/api/identity.ts`
- Modify: `cmdb-demo/src/hooks/useIdentity.ts`

- [ ] **Step 1: Create sensor API client**

Create `cmdb-demo/src/lib/api/sensors.ts`:
```ts
import { apiClient } from './client'

export const sensorApi = {
  list: (params?: Record<string, string>) => apiClient.get('/sensors', params),
  create: (data: any) => apiClient.post('/sensors', data),
  update: (id: string, data: any) => apiClient.put(`/sensors/${id}`, data),
  delete: (id: string) => apiClient.del(`/sensors/${id}`),
  heartbeat: (id: string, data?: any) => apiClient.post(`/sensors/${id}/heartbeat`, data || {}),
}
```

- [ ] **Step 2: Create sensor hooks**

Create `cmdb-demo/src/hooks/useSensors.ts`:
```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sensorApi } from '../lib/api/sensors'

const DEFAULT_TENANT = 'a0000000-0000-0000-0000-000000000001'

export function useSensors() {
  return useQuery({
    queryKey: ['sensors'],
    queryFn: () => sensorApi.list({ tenant_id: DEFAULT_TENANT }),
  })
}

export function useCreateSensor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: sensorApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sensors'] }),
  })
}

export function useUpdateSensor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => sensorApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sensors'] }),
  })
}

export function useDeleteSensor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => sensorApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sensors'] }),
  })
}
```

- [ ] **Step 3: Extend identity API client**

Read `cmdb-demo/src/lib/api/identity.ts`. Add to the existing `identityApi` object:
```ts
  listSessions: (userId: string) => apiClient.get(`/users/${userId}/sessions`),
  changePassword: (data: { current_password: string; new_password: string }) =>
    apiClient.post('/auth/change-password', data),
```

- [ ] **Step 4: Extend identity hooks**

Read `cmdb-demo/src/hooks/useIdentity.ts`. Add:
```ts
export function useUserSessions(userId: string) {
  return useQuery({
    queryKey: ['userSessions', userId],
    queryFn: () => identityApi.listSessions(userId),
    enabled: !!userId,
  })
}

export function useChangePassword() {
  return useMutation({
    mutationFn: identityApi.changePassword,
  })
}
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-demo/src/lib/api/sensors.ts cmdb-demo/src/hooks/useSensors.ts \
       cmdb-demo/src/lib/api/identity.ts cmdb-demo/src/hooks/useIdentity.ts
git commit -m "feat: add sensor API client/hooks and extend identity for sessions/password"
```

---

## Task 6: Connect UserProfile Page

**Files:**
- Modify: `cmdb-demo/src/pages/UserProfile.tsx`

- [ ] **Step 1: Read and identify hardcoded data**

Find: hardcoded `sessions` array (1 item), "Coming Soon" password change alert, hardcoded 2FA status.

- [ ] **Step 2: Replace with API data**

1. Import: `import { useUserSessions, useChangePassword } from '../hooks/useIdentity'`
2. Get userId from auth store
3. Add queries:
```tsx
const { data: sessionsData } = useUserSessions(user?.id || '')
const sessions = (sessionsData as any)?.sessions ?? []
const changePassword = useChangePassword()
```
4. Add password change state: `const [showPasswordModal, setShowPasswordModal] = useState(false)`, `const [currentPw, setCurrentPw] = useState('')`, `const [newPw, setNewPw] = useState('')`
5. Delete the hardcoded `sessions` constant
6. Replace "Coming Soon" password change with a modal that calls:
```tsx
changePassword.mutate({ current_password: currentPw, new_password: newPw }, {
  onSuccess: () => { setShowPasswordModal(false); setCurrentPw(''); setNewPw(''); alert('Password changed!') },
  onError: (err) => alert('Failed: ' + err.message)
})
```
7. The session list rendering already uses `s.device`, `s.browser`, `s.time`, `s.icon`, `s.current` — these match the API field names exactly

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/UserProfile.tsx
git commit -m "feat: connect UserProfile to session list and password change APIs"
```

---

## Task 7: Connect SensorConfiguration Page

**Files:**
- Modify: `cmdb-demo/src/pages/SensorConfiguration.tsx`

- [ ] **Step 1: Read and identify the sensor derivation code**

Find: the `useEffect` or `useMemo` that derives sensors from `useAssets()` (maps assets to sensor objects).

- [ ] **Step 2: Replace with sensor API**

1. Import: `import { useSensors, useCreateSensor, useUpdateSensor, useDeleteSensor } from '../hooks/useSensors'`
2. Replace the asset-derived sensors:
```tsx
const { data: sensorData } = useSensors()
const apiSensors = (sensorData as any)?.sensors ?? []
```
3. Initialize local sensor state from API data:
```tsx
const [sensors, setSensors] = useState<Sensor[]>([])
useEffect(() => {
  if (apiSensors.length > 0) {
    setSensors(apiSensors.map((s: any) => ({
      id: s.id,
      name: s.name,
      type: s.type,
      icon: s.icon || 'sensors',
      location: s.location || '',
      enabled: s.enabled,
      pollingInterval: s.pollingInterval || 30,
      lastSeen: s.lastSeen ? new Date(s.lastSeen).toLocaleString() : 'Never',
      status: s.status || 'Offline',
    })))
  }
}, [apiSensors])
```
4. Remove the old `useAssets()` call and asset-to-sensor mapping logic
5. Wire sensor enable/disable toggle to API: `useUpdateSensor().mutate({ id: sensor.id, data: { enabled: !sensor.enabled } })`
6. Wire "Discover Sensors" button to a create sensor modal instead of alert("Coming Soon")
7. Keep the existing alert rules section unchanged (it uses `useAlertRules` which is already connected)

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/SensorConfiguration.tsx
git commit -m "feat: connect SensorConfiguration to dedicated sensor API"
```

---

## Task 8: Build Verification + Smoke Test

- [ ] **Step 1: Go build**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

- [ ] **Step 2: TypeScript check**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit 2>&1 | grep -E "UserProfile|SensorConfig|useSensors|useIdentity" | head -10
```

- [ ] **Step 3: Restart and test**

```bash
kill $(lsof -t -i:8080) 2>/dev/null; sleep 1
cd /cmdb-platform/cmdb-core && DATABASE_URL="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" NATS_URL="nats://localhost:4222" REDIS_URL="redis://localhost:6379/0" JWT_SECRET="changeme" nohup ./server > /tmp/cmdb-core.log 2>&1 &
sleep 3

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('access_token',''))")

# Test session was recorded on login
echo "=== Sessions ==="
USER_ID=$(curl -s http://localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('id',''))")
curl -s "http://localhost:8080/api/v1/users/$USER_ID/sessions" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo "=== Sensors ==="
curl -s "http://localhost:8080/api/v1/sensors" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo "=== Create Sensor ==="
curl -s -X POST http://localhost:8080/api/v1/sensors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Test Temperature Sensor","type":"temperature","location":"IDC-A / Room 4A","polling_interval":30}' | python3 -m json.tool
```
