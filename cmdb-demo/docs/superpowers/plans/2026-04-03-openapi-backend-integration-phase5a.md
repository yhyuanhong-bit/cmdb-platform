# OpenAPI Backend Integration (Phase 5a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the openapi.yaml single source of truth, generate Go server types via oapi-codegen, build the DTO conversion layer (dbgen → API types), implement the unified API server (impl.go), add location slug lookup, and replace per-module handlers — so all 33 REST endpoints return frontend-compatible JSON.

**Architecture:** Write openapi.yaml defining all 33 endpoints and 18 schemas matching the frontend TypeScript interfaces exactly. Use oapi-codegen to generate Go types and a ServerInterface. Hand-write impl.go (delegates to existing domain services) and convert.go (dbgen pgtype → clean JSON). Replace 9 per-module handler.go files with the single impl.go. Main.go uses generated RegisterHandlers for route registration.

**Tech Stack:** OpenAPI 3.1, oapi-codegen v2.4+, Go 1.23+, existing Gin/sqlc stack

**Spec Reference:** `docs/superpowers/specs/2026-04-03-frontend-backend-integration-design.md`

---

## File Structure

```
cmdb-platform/
├── api/
│   └── openapi.yaml                          # NEW: single source of truth
├── cmdb-core/
│   ���── internal/api/
│   │   ├── generated.go                      # NEW: auto-generated types + ServerInterface
│   ��   ├── convert.go                        # NEW: dbgen → API type converters
│   │   └── impl.go                           # NEW: implements ServerInterface
│   ├── internal/domain/
│   │   ├── asset/handler.go                  # DELETE (replaced by impl.go)
│   │   ├── topology/handler.go               # DELETE
│   │   ├── maintenance/handler.go            # DELETE
│   │   ├── monitoring/handler.go             # DELETE
│   │   ├── inventory/handler.go              # DELETE
│   │   ├── audit/handler.go                  # DELETE
│   │   ├── dashboard/handler.go              # DELETE
│   ���   ├── identity/handler.go               # DELETE
│   │   └── prediction/handler.go             # DELETE
│   ├── internal/domain/topology/service.go   # MODIFY: add GetBySlug method
│   ├── db/queries/locations.sql              # MODIFY: add GetLocationBySlug query
│   ├── cmd/server/main.go                    # MODIFY: use RegisterHandlers
│   ├── oapi-codegen.yaml                     # NEW: codegen config
│   └── Makefile                              # MODIFY: add generate-api target
└── Makefile                                  # NEW: root generate command
```

---

## Task 1: Write openapi.yaml

**Files:**
- Create: `api/openapi.yaml`

This is the largest single file. It defines all 18 schemas and 33 endpoints, matching both the frontend TypeScript interfaces and the backend capabilities.

- [ ] **Step 1: Create the openapi.yaml**

Create `api/openapi.yaml`. This file must match the frontend interfaces in `cmdb-demo/src/lib/api/*.ts` exactly — same field names, same nullability, same JSON structure.

Key rules for the YAML:
- All UUIDs are `type: string, format: uuid` (frontend expects `"uuid-string"`, not objects)
- All timestamps are `type: string, format: date-time` (frontend expects RFC3339 strings)
- Nullable fields use `type: ["string", "null"]` (OpenAPI 3.1 syntax)
- Arrays that can be empty use `default: []`
- The response envelope `{data, meta}` and `{data, pagination, meta}` must match `ApiResponse<T>` and `ApiListResponse<T>` from `types.ts`

The file should contain:
- `info`, `servers` (`/api/v1`), `security` (BearerAuth)
- `components/securitySchemes` (BearerAuth)
- `components/schemas`: Meta, Pagination, ErrorBody, ErrorResponse, TokenPair, CurrentUser, Asset, Location, Rack, WorkOrder, WorkOrderLog, AlertEvent, InventoryTask, InventoryItem, AuditEvent, User, Role, PredictionModel, PredictionResult, RCAAnalysis, LocationStats, DashboardStats
- `paths`: all 33 endpoints grouped by tag (auth, topology, assets, maintenance, monitoring, inventory, audit, dashboard, identity, prediction)

For each endpoint, define:
- operationId (e.g., `listAssets`, `getAsset`, `createWorkOrder`)
- parameters (path params as `format: uuid`, query params with defaults)
- requestBody where needed (login, create order, transition, create RCA, verify RCA)
- responses with proper schema refs

**Important field mappings (frontend TS → openapi.yaml):**

| Frontend field | Frontend type | openapi.yaml type |
|---------------|--------------|-------------------|
| `id` | `string` | `string, format: uuid` |
| `rack_id` | `string \| null` | `type: [string, null], format: uuid` |
| `vendor` | `string` | `type: [string, null]` |
| `created_at` | `string` | `string, format: date-time` |
| `attributes` | `Record<string, any>` | `type: object, additionalProperties: true` |
| `tags` | `string[]` | `type: array, items: {type: string}, default: []` |
| `permissions` | `Record<string, string[]>` | `type: object, additionalProperties: {type: array, items: {type: string}}` |
| `trigger_value` | `number` | `type: [number, null]` |

**Note on monitoring.ts:** Frontend defines `ci_id` but backend uses `asset_id`. The openapi.yaml should use `asset_id` (matching the DB) and the frontend monitoring.ts interface will be updated in Plan 5b to match.

- [ ] **Step 2: Validate the YAML**

```bash
# Install a validator if needed
pip install openapi-spec-validator 2>/dev/null || true
python3 -c "
from openapi_spec_validator import validate
import yaml
with open('api/openapi.yaml') as f:
    spec = yaml.safe_load(f)
validate(spec)
print('OpenAPI spec is valid')
"
```

Expected: "OpenAPI spec is valid"

- [ ] **Step 3: Commit**

```bash
git add api/openapi.yaml
git commit -m "feat: add openapi.yaml - single source of truth for 33 endpoints, 18 schemas"
```

---

## Task 2: oapi-codegen Setup + Generate Go Types

**Files:**
- Create: `cmdb-core/oapi-codegen.yaml`
- Generate: `cmdb-core/internal/api/generated.go`
- Modify: `cmdb-core/Makefile`

- [ ] **Step 1: Create oapi-codegen config**

Create `cmdb-core/oapi-codegen.yaml`:

```yaml
package: api
output: internal/api/generated.go
generate:
  gin-server: true
  models: true
  embedded-spec: false
output-options:
  skip-prune: true
```

- [ ] **Step 2: Install oapi-codegen and generate**

```bash
cd /cmdb-platform/cmdb-core
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen --config oapi-codegen.yaml ../api/openapi.yaml
```

Expected: `internal/api/generated.go` created with ServerInterface + all type structs.

- [ ] **Step 3: Verify generated code compiles**

```bash
go mod tidy
go build ./internal/api/...
```

If there are compile errors from the generated code, the openapi.yaml likely has issues. Fix the yaml and regenerate.

- [ ] **Step 4: Add generate-api target to Makefile**

Append to `cmdb-core/Makefile`:

```makefile
# Generate API types from OpenAPI spec
generate-api:
	oapi-codegen --config oapi-codegen.yaml ../api/openapi.yaml
```

- [ ] **Step 5: Create root Makefile**

Create `/cmdb-platform/Makefile`:

```makefile
.PHONY: generate

# Generate all code from OpenAPI spec
generate:
	cd cmdb-core && make generate-api
	@echo "Go types generated"
	@echo "Run 'cd cmdb-demo && npm run generate-api' for TypeScript types"
```

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/oapi-codegen.yaml cmdb-core/internal/api/generated.go cmdb-core/Makefile Makefile
git commit -m "feat: add oapi-codegen config + generate Go server types from openapi.yaml"
```

---

## Task 3: DTO Conversion Layer (convert.go)

**Files:**
- Create: `cmdb-core/internal/api/convert.go`

This is the bridge between internal dbgen types (with pgtype.*) and the clean API types generated by oapi-codegen (with string, *string, etc.).

- [ ] **Step 1: Create convert.go**

Create `cmdb-core/internal/api/convert.go`.

Read the generated `internal/api/generated.go` FIRST to understand the exact API type field names and types. Then read `internal/dbgen/models.go` for the source types.

The file must contain:

**Helper functions:**
```go
package api

import (
    "encoding/json"
    "time"
    "github.com/jackc/pgx/v5/pgtype"
)

func pguuidToPtr(v pgtype.UUID) *string {
    if !v.Valid { return nil }
    s := fmt.Sprintf("%x-%x-%x-%x-%x", v.Bytes[0:4], v.Bytes[4:6], v.Bytes[6:8], v.Bytes[8:10], v.Bytes[10:16])
    return &s
}
// OR use uuid library if available

func pgtextToPtr(v pgtype.Text) *string {
    if !v.Valid { return nil }
    return &v.String
}

func pgtsToStr(v time.Time) string {
    return v.Format(time.RFC3339)
}

func pgtsToPtr(v pgtype.Timestamptz) *string {
    if !v.Valid { return nil }
    s := v.Time.Format(time.RFC3339)
    return &s
}

func pgnumericToPtr(v pgtype.Numeric) *float64 {
    if !v.Valid { return nil }
    f, _ := v.Float64Value()
    val := f.Float64
    return &val
}

func pgboolToPtr(v pgtype.Bool) *bool {
    if !v.Valid { return nil }
    return &v.Bool
}

func pgdateToPtr(v pgtype.Date) *string {
    if !v.Valid { return nil }
    s := v.Time.Format("2006-01-02")
    return &s
}

func bytesToMap(b []byte) map[string]any {
    if len(b) == 0 { return nil }
    var m map[string]any
    json.Unmarshal(b, &m)
    return m
}

func convertSlice[F any, T any](items []F, fn func(F) T) []T {
    result := make([]T, len(items))
    for i, item := range items {
        result[i] = fn(item)
    }
    return result
}
```

**14 conversion functions** (one per business type). Each maps every field from dbgen struct to API struct. The exact field names come from generated.go — read it before writing.

Pattern for each:
```go
func toAPIAsset(db dbgen.Asset) Asset {
    return Asset{
        Id:             db.ID.String(),
        AssetTag:       db.AssetTag,
        PropertyNumber: pgtextToPtr(db.PropertyNumber),
        ControlNumber:  pgtextToPtr(db.ControlNumber),
        Name:           db.Name,
        Type:           db.Type,
        SubType:        pgtextToPtr(db.SubType),
        Status:         db.Status,
        BiaLevel:       db.BiaLevel,
        LocationId:     pguuidToPtr(db.LocationID),
        RackId:         pguuidToPtr(db.RackID),
        Vendor:         pgtextToPtr(db.Vendor),
        Model:          pgtextToPtr(db.Model),
        SerialNumber:   pgtextToPtr(db.SerialNumber),
        Attributes:     bytesToMap(db.Attributes),  // or raw json
        Tags:           db.Tags,
        CreatedAt:      pgtsToStr(db.CreatedAt),
        UpdatedAt:      pgtsToStr(db.UpdatedAt),
    }
}
```

Write all 14: toAPIAsset, toAPILocation, toAPIRack, toAPIWorkOrder, toAPIWorkOrderLog, toAPIAlertEvent, toAPIInventoryTask, toAPIInventoryItem, toAPIAuditEvent, toAPIUser, toAPIRole, toAPIPredictionModel, toAPIPredictionResult, toAPIRCAAnalysis.

**CRITICAL:** Read generated.go for exact field names. oapi-codegen converts snake_case YAML to CamelCase Go (e.g., `asset_tag` → `AssetTag`). But JSON tags will be snake_case. Match the generated struct fields exactly.

- [ ] **Step 2: Verify build**

```bash
go build ./internal/api/...
```

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/internal/api/convert.go
git commit -m "feat: add DTO conversion layer - 14 dbgen-to-API converters + helpers"
```

---

## Task 4: Location Slug Lookup Query

**Files:**
- Modify: `cmdb-core/db/queries/locations.sql`
- Modify: `cmdb-core/internal/domain/topology/service.go`

- [ ] **Step 1: Add slug query to locations.sql**

Append to `cmdb-core/db/queries/locations.sql`:

```sql
-- name: GetLocationBySlug :one
SELECT * FROM locations
WHERE tenant_id = $1 AND slug = $2 AND level = $3;
```

- [ ] **Step 2: Regenerate sqlc**

```bash
cd /cmdb-platform/cmdb-core
sqlc generate
go build ./internal/dbgen/...
```

- [ ] **Step 3: Add GetBySlug to topology service**

Read `internal/domain/topology/service.go`, then add this method:

```go
func (s *Service) GetBySlug(ctx context.Context, tenantID uuid.UUID, slug, level string) (*dbgen.Location, error) {
    loc, err := s.queries.GetLocationBySlug(ctx, dbgen.GetLocationBySlugParams{
        TenantID: tenantID,
        Slug:     slug,
        Level:    level,
    })
    if err != nil {
        return nil, err
    }
    return &loc, nil
}
```

Note: Check the exact param types from generated `GetLocationBySlugParams` and adapt. The slug and level fields may be `string` or `pgtype.Text` depending on how sqlc resolved the column types.

- [ ] **Step 4: Verify build**

```bash
go build ./internal/domain/topology/...
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/db/queries/locations.sql cmdb-core/internal/dbgen/ cmdb-core/internal/domain/topology/service.go
git commit -m "feat: add location slug+level lookup query for frontend routing"
```

---

## Task 5: Unified API Server (impl.go)

**Files:**
- Create: `cmdb-core/internal/api/impl.go`

This is the largest hand-written file. It implements the generated `ServerInterface` by delegating to existing domain services.

- [ ] **Step 1: Read the generated ServerInterface**

```bash
grep -A2 'type ServerInterface' cmdb-core/internal/api/generated.go | head -50
```

This shows all methods that need implementing. Each method corresponds to an operationId from openapi.yaml.

- [ ] **Step 2: Create impl.go**

Create `cmdb-core/internal/api/impl.go`.

Read the EXACT method signatures from generated.go. The methods will look like:
```go
type ServerInterface interface {
    PostAuthLogin(c *gin.Context)
    PostAuthRefresh(c *gin.Context)
    GetAuthMe(c *gin.Context)
    GetAssets(c *gin.Context, params GetAssetsParams)
    GetAssetsId(c *gin.Context, id string)
    // ... etc
}
```

The struct:
```go
package api

import (
    "github.com/cmdb-platform/cmdb-core/internal/domain/asset"
    "github.com/cmdb-platform/cmdb-core/internal/domain/audit"
    "github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
    "github.com/cmdb-platform/cmdb-core/internal/domain/identity"
    "github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
    "github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
    "github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
    "github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
    "github.com/cmdb-platform/cmdb-core/internal/domain/topology"
    "github.com/cmdb-platform/cmdb-core/internal/platform/response"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type APIServer struct {
    authSvc       *identity.AuthService
    identitySvc   *identity.Service
    topologySvc   *topology.Service
    assetSvc      *asset.Service
    maintenanceSvc *maintenance.Service
    monitoringSvc *monitoring.Service
    inventorySvc  *inventory.Service
    auditSvc      *audit.Service
    dashboardSvc  *dashboard.Service
    predictionSvc *prediction.Service
}

func NewAPIServer(
    authSvc *identity.AuthService,
    identitySvc *identity.Service,
    topologySvc *topology.Service,
    assetSvc *asset.Service,
    maintenanceSvc *maintenance.Service,
    monitoringSvc *monitoring.Service,
    inventorySvc *inventory.Service,
    auditSvc *audit.Service,
    dashboardSvc *dashboard.Service,
    predictionSvc *prediction.Service,
) *APIServer {
    return &APIServer{
        authSvc: authSvc, identitySvc: identitySvc, topologySvc: topologySvc,
        assetSvc: assetSvc, maintenanceSvc: maintenanceSvc, monitoringSvc: monitoringSvc,
        inventorySvc: inventorySvc, auditSvc: auditSvc, dashboardSvc: dashboardSvc,
        predictionSvc: predictionSvc,
    }
}
```

Then implement EVERY method in ServerInterface. Each follows the pattern:
1. Parse tenant_id from gin context: `uuid.Parse(c.GetString("tenant_id"))`
2. Parse path/query params (may be pre-parsed by generated code)
3. Call the appropriate domain service method
4. Convert result with toAPIXxx()
5. Return with response.OK() or response.OKList()

For auth endpoints (login, refresh): these are public, read from request body.
For GET /locations with slug param: check if slug query param exists, if so call GetBySlug, otherwise call ListRootLocations.

**IMPORTANT:** Read each domain service's method signatures from their service.go files before implementing. The param types must match exactly.

- [ ] **Step 3: Verify build**

```bash
go build ./internal/api/...
```

This MUST compile. If a method signature doesn't match the interface, fix it.

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/api/impl.go
git commit -m "feat: add unified API server implementing ServerInterface with all 33 endpoints"
```

---

## Task 6: Replace Handlers in main.go + Delete Old Handlers

**Files:**
- Modify: `cmdb-core/cmd/server/main.go`
- Delete: 9 handler.go files

- [ ] **Step 1: Read current main.go**

Understand the current wiring: which services are created, how handlers are registered, what middleware is applied.

- [ ] **Step 2: Update main.go**

Replace the handler creation + registration section with:

```go
// Replace all per-module handler creation with:
apiServer := api.NewAPIServer(
    authSvc, identitySvc, topologySvc, assetSvc, maintenanceSvc,
    monitoringSvc, inventorySvc, auditSvc, dashboardSvc, predictionSvc,
)

// Replace all per-module Register() calls with:
// The generated RegisterHandlers takes care of all route registration
api.RegisterHandlersWithOptions(r.Group("/api/v1"), apiServer, api.GinServerOptions{
    BaseURL: "",
    Middlewares: []api.MiddlewareFunc{},
    // Auth middleware needs to be applied selectively:
    // login/refresh are public, everything else needs auth
})
```

Note: oapi-codegen's generated Gin handler may or may not support per-route middleware. Read the generated `RegisterHandlers` function signature. If it doesn't support selective auth, you may need to:
1. Register public routes (login, refresh) directly
2. Use RegisterHandlers for protected routes with auth middleware applied to the group

Keep all existing infrastructure: config, DB pool, Redis, NATS, event bus, MCP server, WebSocket hub. Only replace the handler layer.

- [ ] **Step 3: Delete old handler files**

```bash
rm cmdb-core/internal/domain/asset/handler.go
rm cmdb-core/internal/domain/topology/handler.go
rm cmdb-core/internal/domain/maintenance/handler.go
rm cmdb-core/internal/domain/monitoring/handler.go
rm cmdb-core/internal/domain/inventory/handler.go
rm cmdb-core/internal/domain/audit/handler.go
rm cmdb-core/internal/domain/dashboard/handler.go
rm cmdb-core/internal/domain/identity/handler.go
rm cmdb-core/internal/domain/prediction/handler.go
```

- [ ] **Step 4: Verify full build**

```bash
go build ./...
```

The build MUST succeed. If domain packages have other files importing from handler.go, those imports need updating.

- [ ] **Step 5: Verify server starts**

```bash
# If infra is running:
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" go run ./cmd/server &
sleep 2
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/api/v1/assets | python3 -m json.tool | head -5
kill %1
```

Expected: healthz returns OK, assets returns proper JSON with `data` and `meta` fields.

- [ ] **Step 6: Commit**

```bash
git add -A cmdb-core/
git commit -m "feat: replace 9 per-module handlers with unified API server + delete old handlers"
```

---

## Endpoint Verification Checklist

After all 6 tasks, verify these key endpoints return frontend-compatible JSON:

| Endpoint | Expected JSON shape |
|----------|-------------------|
| `POST /api/v1/auth/login` | `{"data":{"access_token":"...","refresh_token":"...","expires_in":900},"meta":{...}}` |
| `GET /api/v1/assets` | `{"data":[{"id":"uuid","asset_tag":"SRV-001","vendor":"Dell",...}],"pagination":{...},"meta":{...}}` |
| `GET /api/v1/locations` | `{"data":[{"id":"uuid","slug":"tw","level":"country",...}],"meta":{...}}` |
| `GET /api/v1/locations?slug=tw&level=country` | `{"data":{"id":"uuid","slug":"tw","level":"country",...},"meta":{...}}` |
| `GET /api/v1/monitoring/alerts` | `{"data":[{"id":"uuid","asset_id":"uuid","severity":"critical",...}],"pagination":{...},"meta":{...}}` |
| `GET /api/v1/dashboard/stats` | `{"data":{"total_assets":4,"critical_alerts":2,...},"meta":{...}}` |

Key checks:
- UUIDs are strings (`"uuid-string"`), not objects
- Nullable fields are `null` or omitted, not `{"Valid":false,"String":""}`
- Timestamps are RFC3339 strings
- Arrays are `[]` not `null`
- Pagination has page/page_size/total/total_pages
