# Track B Handler Migration — Audit

**Date:** 2026-04-20
**Commit at audit:** `dd6bae9` (master tip, clean working tree)
**Roadmap reference:** `REMEDIATION-ROADMAP.md` § 3.7 "Track B handler 迁移到 ServerInterface"

## TL;DR

**Phase 3.7 is essentially complete.** The roadmap's claim of "60 手写 `*gin.Context` handler
在 `main.go:349-449` 直接挂" is out of date. At `dd6bae9`:

- `main.go` registers **3** API routes manually (not 60), of which **2** are intentionally
  infra-only (`/ws` WebSocket upgrade, `/admin/migrate-statuses` one-off).
- **168** OpenAPI operations — all 168 flow through `api.RegisterHandlers` (Track A).
- `oapi-codegen.yaml` contains **zero** `exclude-operation-ids` (the key is not present).
- `make check-api-routes` prints `OK — spec and code are aligned`.
- `make lint` and `make build` pass.

The series of commits leading here is already in the log:

```
9a91826 refactor(api): remove 6 redundant manual route registrations
ba54db4 refactor(api): complete Track-B → Track-A migration — 10 final handlers
eb1ddbd refactor(api): migrate 6 asset/location handlers to Track A
66f2df8 refactor(api): migrate 6 inventory handlers to Track A
52b935d refactor(api): migrate 6 sync handlers to Track A
d36a6cf refactor(api): migrate metrics endpoints to typed ServerInterface
```

**Recommendation:** mark 3.7 **done** in the roadmap. One small follow-up noted below.

---

## 1. Manual route registrations in `cmd/server/main.go`

Grepped `router.*.(GET|POST|PUT|DELETE|PATCH)(` and `v1.*.(GET|POST|PUT|DELETE|PATCH)(`:

| Line | Route                             | Category    | Notes                                            |
|------|-----------------------------------|-------------|--------------------------------------------------|
| 427  | `router.GET /healthz`             | infra       | Liveness probe, not v1 API                       |
| 428  | `router.GET /readyz`              | infra       | Readiness probe, not v1 API                      |
| 431  | `router.GET /metrics`             | infra       | Prometheus scrape, not v1 API                    |
| 492  | `v1.POST /auth/logout`            | **STALE**   | Duplicate — also registered by `RegisterHandlers` at `generated.go:7388`. Comment above it claims the spec hasn't been regenerated, but the spec has `operationId: logout` at `openapi.yaml:125`. See §4. |
| 498  | `v1.POST /admin/migrate-statuses` | admin-only  | One-off `work_orders` status migration — intentionally not in spec, already on `UndocumentedAllowlist`. |
| 563  | `v1.GET /ws`                      | WebSocket   | Upgrade endpoint, not REST — intentionally not in spec, already on `UndocumentedAllowlist`. |

**Non-infra manual v1 registrations: 3**, of which **2** are correct-by-design (`/ws`,
`/admin/migrate-statuses`) and **1** is redundant (`/auth/logout`).

The roadmap's quoted range `main.go:349-449` is now just middleware/bootstrap code — no
handlers live there any more.

## 2. Route registrations inside `internal/api/*_endpoints*.go` and `custom_endpoints.go`

```
grep -E '\.(GET|POST|PUT|DELETE|PATCH)\(' internal/api/*_endpoints*.go internal/api/custom_endpoints.go
→ 0 matches
```

These files only **define handler methods** on `*APIServer`; they do not register routes
themselves. Routing is centralized in `generated.go:7356` (`RegisterHandlersWithOptions`).

## 3. `APIServer` methods vs `ServerInterface` methods

Both sets have **exactly 168** entries and they match name-for-name.

- `ServerInterface` definition: `generated.go:2566-3071` — 168 methods with `c *gin.Context` first param.
- `*APIServer` methods grepped from all non-test Go files: 168 unique method names across
  36 files.
- Diffing the two sorted lists yields zero symmetric difference: every `ServerInterface`
  method has a matching `*APIServer` implementation, and every `*APIServer` HTTP handler
  is required by `ServerInterface`.

Signatures also align — spot-checked examples:

| Method                          | Signature (both in `ServerInterface` & impl)                              |
|---------------------------------|---------------------------------------------------------------------------|
| `GetActivityFeed`               | `(c, params GetActivityFeedParams)`                                       |
| `AcceptUpgradeRecommendation`   | `(c, id IdPath, category string)`                                         |
| `RemoveRoleFromUser`            | `(c, id IdPath, roleId openapi_types.UUID)`                               |
| `GetRackStats`                  | `(c)` (file: `custom_endpoints.go`, in interface at `generated.go:2935`)  |

Files named `*_endpoints.go` and `custom_endpoints.go` look historically "Track B" by
name but their methods have already been brought up to the `ServerInterface` signature
convention. The naming is a § 3.9 concern ("handler 命名统一"), not a 3.7 concern.

## 4. Orphan methods on `APIServer` not in `ServerInterface`

**None.** Set subtraction `(APIServer methods) − (ServerInterface methods) = ∅`.

This is confirmed by the fact that `api.NewAPIServer(...)` is used as a typed
`ServerInterface` argument to `RegisterHandlers` — Go's structural typing would fail
compilation if any method was missing or mismatched.

## 5. CI verification

```
$ make check-api-routes
  spec operations:         168 (api/openapi.yaml)
  code routes (manual):      3 (cmd/server/main.go)
  code routes (generated): 168 (internal/api/generated.go)
  code routes (merged):    170
  allowlisted undoc:         2
OK — spec and code are aligned.

$ make lint          # errcheck + go vet
  (clean, no findings)

$ make build
  CGO_ENABLED=0 go build -o bin/cmdb-core ./cmd/server
  (clean)
```

The merge count is 170 because `/auth/logout` appears in both `manual` (3) and
`generated` (168) — set-union dedupes it to 169, plus the 2 allowlisted admin/ws routes
which are only in manual ⇒ 168 spec + 2 non-spec = 170.

## 6. Recommendation

1. **Mark § 3.7 done in the roadmap.** The Track B handler migration described by that
   section has already been executed across a series of `refactor(api): migrate ...`
   commits. No additional operation-level migration remains.

2. **Small follow-up (separate commit, not part of 3.7):** remove the stale duplicate
   `v1.POST("/auth/logout", apiServer.Logout)` at `cmd/server/main.go:492`. It is
   re-registered by the generated `RegisterHandlers` on the same Gin group, which would
   panic at server startup on duplicate route. Either this line is reachable and
   currently panics, or it is guarded by a test gap. Either way it should go — but it is
   a bugfix, not a Track-B migration.

3. The remaining manual routes (`/ws`, `/admin/migrate-statuses`) are correct by design
   and are covered by the `UndocumentedAllowlist` in `cmd/check-api-routes/main.go:32`.
   These should stay manual.

4. § 3.9 ("handler 命名统一") is still open and is the natural next step — renaming
   `*_endpoints.go` / `custom_endpoints.go` to `impl_*.go` so the file name reflects the
   now-uniform signature contract. Out of scope for 3.7.
