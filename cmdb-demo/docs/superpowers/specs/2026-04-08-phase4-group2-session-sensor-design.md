# Phase 4 Group 2: Session Management + Sensor Registration

**Date:** 2026-04-08
**Scope:** User session tracking, password change, sensor CRUD + heartbeat
**Depends on:** Phase 1-3 completed, auth system exists (JWT + Redis refresh tokens)

---

## 1. Overview

Two independent subsystems:

1. **Session Management** — Track login sessions (IP, device, time), display in UserProfile, support password change. 2FA and session revocation are Phase 2 scope (先只讀展示，後面加踢 session).
2. **Sensor Registration** — Dedicated sensor table (not derived from assets), CRUD management, heartbeat tracking, connect SensorConfiguration page to real data.

```
Session Management:
  ├── user_sessions table           → track login history
  ├── ALTER users ADD last_login_at → login timestamp
  ├── POST /auth/change-password    → password change
  ├── GET /users/{id}/sessions      → session list
  └── UserProfile page connected

Sensor Registration:
  ├── sensors table                 → dedicated sensor registry
  ├── CRUD endpoints (4)           → GET/POST/PUT/DELETE
  ├── Heartbeat tracking           → last_heartbeat column
  └── SensorConfiguration page connected
```

---

## 2. Database Schema

### Migration: `000020_phase4_group2.up.sql`

```sql
-- 1. User sessions — tracks login events
CREATE TABLE user_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address     VARCHAR(50),
    user_agent     TEXT,
    device_type    VARCHAR(30),    -- desktop / mobile / tablet / unknown
    browser        VARCHAR(50),    -- Chrome / Firefox / Safari / Edge / unknown
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expired_at     TIMESTAMPTZ,    -- NULL = still active
    is_current     BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX idx_user_sessions_user ON user_sessions(user_id, created_at DESC);

-- 2. Add last_login_at to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(50);

-- 3. Sensors — dedicated sensor registry
CREATE TABLE sensors (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id),
    asset_id         UUID REFERENCES assets(id) ON DELETE SET NULL,
    name             VARCHAR(200) NOT NULL,
    type             VARCHAR(50) NOT NULL,      -- temperature / humidity / power / network / cpu / memory / disk
    location         VARCHAR(200),              -- human-readable location description
    polling_interval INT NOT NULL DEFAULT 30,   -- seconds
    enabled          BOOLEAN NOT NULL DEFAULT true,
    status           VARCHAR(20) NOT NULL DEFAULT 'offline',  -- online / offline / degraded
    last_heartbeat   TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sensors_tenant ON sensors(tenant_id);
CREATE INDEX idx_sensors_asset ON sensors(asset_id);
```

### Down migration: `000020_phase4_group2.down.sql`

```sql
DROP TABLE IF EXISTS sensors;
DROP TABLE IF EXISTS user_sessions;
ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;
ALTER TABLE users DROP COLUMN IF EXISTS last_login_ip;
```

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| `user_sessions` has no tenant_id | Tenant isolation via user_id FK chain (user → tenant) — same as work_order_logs pattern |
| `expired_at` NULL = active | Simpler than boolean; allows querying active sessions as `WHERE expired_at IS NULL` |
| `sensors.asset_id` nullable + ON DELETE SET NULL | Sensor can exist without an asset (standalone sensor), asset deletion doesn't delete sensor |
| `sensors.type` is free-form VARCHAR | Extensible for future sensor types without schema changes |
| `device_type` + `browser` parsed from User-Agent | Go backend parses UA string on login |

---

## 3. API Endpoints

### 3.1 Session Management (3 endpoints)

**GET `/users/:id/sessions`** — List user's login sessions

```json
{
  "sessions": [
    {
      "id": "uuid",
      "ip_address": "10.134.143.218",
      "device_type": "desktop",
      "browser": "Chrome",
      "created_at": "2026-04-08T09:00:00Z",
      "last_active_at": "2026-04-08T10:30:00Z",
      "is_current": true
    },
    {
      "id": "uuid",
      "ip_address": "192.168.1.50",
      "device_type": "mobile",
      "browser": "Safari",
      "created_at": "2026-04-07T14:00:00Z",
      "last_active_at": "2026-04-07T18:00:00Z",
      "is_current": false
    }
  ]
}
```

Query: `SELECT * FROM user_sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 20`

**POST `/auth/change-password`** — Change password

```json
// Request
{
  "current_password": "old123",
  "new_password": "new456"
}
// Response
{ "message": "Password changed successfully" }
```

Logic:
1. Get user_id from JWT context
2. Load user from DB, verify current_password with bcrypt
3. Hash new_password with bcrypt
4. UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2
5. Return 200 or 401 if current password wrong

**Login session recording** — Modify existing login handler (no new endpoint)

On successful login (`POST /auth/login`), add:
1. Parse User-Agent header for device_type + browser
2. Get IP from `c.ClientIP()`
3. INSERT into user_sessions (user_id, ip_address, user_agent, device_type, browser, is_current=true)
4. UPDATE users SET last_login_at = now(), last_login_ip = clientIP WHERE id = $1
5. Set previous sessions' is_current = false for this user

### 3.2 Sensor Registration (5 endpoints)

**GET `/sensors?tenant_id=`** — List sensors

```json
{
  "sensors": [
    {
      "id": "uuid",
      "asset_id": "uuid",
      "asset_name": "SRV-PROD-001",
      "name": "Rack A01 Temperature",
      "type": "temperature",
      "location": "IDC-A / Room 4A / Rack A01",
      "polling_interval": 30,
      "enabled": true,
      "status": "online",
      "last_heartbeat": "2026-04-08T10:29:50Z"
    }
  ]
}
```

Query: JOIN assets for asset_name when asset_id is not null.

**POST `/sensors`** — Create sensor

```json
// Request
{
  "tenant_id": "uuid",
  "asset_id": "uuid",
  "name": "Rack A01 Temperature",
  "type": "temperature",
  "location": "IDC-A / Room 4A / Rack A01",
  "polling_interval": 30,
  "enabled": true
}
```

**PUT `/sensors/:id`** — Update sensor

```json
// Request (partial update)
{
  "name": "Updated name",
  "polling_interval": 60,
  "enabled": false
}
```

**DELETE `/sensors/:id`** — Delete sensor

Returns 204 on success.

**POST `/sensors/:id/heartbeat`** — Record heartbeat (called by sensor/collector)

```json
// Request (optional body)
{ "status": "online" }
// Response
{ "message": "ok" }
```

Logic: UPDATE sensors SET last_heartbeat = now(), status = $1, updated_at = now() WHERE id = $2

---

## 4. Go Backend

### Modified Files

**`cmdb-core/internal/domain/identity/auth_service.go`** — Modify Login method:
- After successful token issuance, INSERT into user_sessions
- UPDATE users SET last_login_at, last_login_ip
- Parse User-Agent for device_type + browser

**`cmdb-core/internal/api/impl.go`** — Modify Login handler:
- Pass `c.ClientIP()` and `c.GetHeader("User-Agent")` to auth service

### New Files

| File | Content |
|------|---------|
| `cmdb-core/db/migrations/000020_phase4_group2.up.sql` | 2 new tables + ALTER users |
| `cmdb-core/db/migrations/000020_phase4_group2.down.sql` | Rollback |
| `cmdb-core/internal/api/phase4_session_endpoints.go` | GetUserSessions, ChangePassword (2 handlers) |
| `cmdb-core/internal/api/phase4_sensor_endpoints.go` | Sensor CRUD + heartbeat (5 handlers) |

### Route Registration

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

### User-Agent Parsing (simple, no external library)

```go
func parseUserAgent(ua string) (deviceType, browser string) {
    ua = strings.ToLower(ua)
    // Browser detection
    switch {
    case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg"):
        browser = "Chrome"
    case strings.Contains(ua, "firefox"):
        browser = "Firefox"
    case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
        browser = "Safari"
    case strings.Contains(ua, "edg"):
        browser = "Edge"
    default:
        browser = "unknown"
    }
    // Device detection
    switch {
    case strings.Contains(ua, "mobile") || strings.Contains(ua, "android"):
        deviceType = "mobile"
    case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
        deviceType = "tablet"
    default:
        deviceType = "desktop"
    }
    return
}
```

---

## 5. Frontend Changes

### New API Methods

**`src/lib/api/identity.ts`** — extend:
```ts
  listSessions: (userId: string) => apiClient.get(`/users/${userId}/sessions`),
  changePassword: (data: { current_password: string; new_password: string }) =>
    apiClient.post('/auth/change-password', data),
```

**New file: `src/lib/api/sensors.ts`**
```ts
export const sensorApi = {
  list: (params?: Record<string, string>) => apiClient.get('/sensors', params),
  create: (data: any) => apiClient.post('/sensors', data),
  update: (id: string, data: any) => apiClient.put(`/sensors/${id}`, data),
  delete: (id: string) => apiClient.del(`/sensors/${id}`),
  heartbeat: (id: string, data?: any) => apiClient.post(`/sensors/${id}/heartbeat`, data),
}
```

### New Hooks

**`src/hooks/useIdentity.ts`** — extend:
```ts
  useUserSessions(userId: string)
  useChangePassword()
```

**New file: `src/hooks/useSensors.ts`**
```ts
  useSensors()
  useCreateSensor()
  useUpdateSensor()
  useDeleteSensor()
```

### Page Changes

**UserProfile** (`src/pages/UserProfile.tsx`)

Replace:
- Hardcoded `sessions` array (1 item) → `useUserSessions(userId)` API
- "Coming Soon" password change → modal with `useChangePassword().mutate()` (current_password + new_password fields)
- Hardcoded 2FA status → display `user.two_factor_enabled` (always false for now, 2FA implementation is future scope)
- Last login IP from "—" → `user.last_login_ip` (populated on login)

**SensorConfiguration** (`src/pages/SensorConfiguration.tsx`)

Replace:
- Sensors derived from assets → `useSensors()` API with real sensor records
- "Discover Sensors" button → modal to manually register sensor (POST /sensors)
- Sensor enable/disable toggle → `useUpdateSensor().mutate({ id, data: { enabled } })`
- Sensor polling interval → `useUpdateSensor().mutate({ id, data: { polling_interval } })`
- Sensor last seen → from `sensor.last_heartbeat` timestamp
- Sensor status → from `sensor.status` (online/offline/degraded)
- Keep existing alert rules section (already connected in Phase 1)

---

## 6. File Structure

### New Files

| File | Content |
|------|---------|
| `cmdb-core/db/migrations/000020_phase4_group2.up.sql` | user_sessions + sensors tables + ALTER users |
| `cmdb-core/db/migrations/000020_phase4_group2.down.sql` | Rollback |
| `cmdb-core/internal/api/phase4_session_endpoints.go` | GetUserSessions + ChangePassword |
| `cmdb-core/internal/api/phase4_sensor_endpoints.go` | Sensor CRUD + heartbeat (5 handlers) |
| `cmdb-demo/src/lib/api/sensors.ts` | Sensor API client |
| `cmdb-demo/src/hooks/useSensors.ts` | Sensor hooks |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-core/cmd/server/main.go` | Register 7 new routes |
| `cmdb-core/internal/domain/identity/auth_service.go` | Add session recording on login |
| `cmdb-core/internal/api/impl.go` | Pass ClientIP + UserAgent to login |
| `cmdb-demo/src/lib/api/identity.ts` | Add listSessions, changePassword |
| `cmdb-demo/src/hooks/useIdentity.ts` | Add useUserSessions, useChangePassword |
| `cmdb-demo/src/pages/UserProfile.tsx` | Replace hardcoded sessions + wire password change |
| `cmdb-demo/src/pages/SensorConfiguration.tsx` | Replace asset-derived sensors with API data |

---

## 7. Scope Boundaries (Phase 1 vs Future)

### This Phase (Phase 1 of Session + Sensor)

| Feature | Included |
|---------|----------|
| Login session recording | Yes |
| Session list display | Yes (read-only) |
| Password change | Yes |
| Sensor manual CRUD | Yes |
| Sensor heartbeat | Yes |
| Sensor polling config | Yes |

### Future (Phase 2)

| Feature | Not Included |
|---------|-------------|
| Kick/revoke sessions | Future — needs token invalidation in Redis |
| 2FA (TOTP/SMS) | Future — needs OTP library + QR generation |
| Sensor auto-discovery | Future — needs collector integration |
| Sensor data ingestion | Future — needs metrics pipeline |
