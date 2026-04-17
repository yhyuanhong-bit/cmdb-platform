# OpenAPI Drift Report

Last updated: 2026-04-17

## Summary

| Category | Count |
|---|---|
| Documented routes (`api/openapi.yaml`) | ~130 |
| Routes registered in `main.go` but **NOT** documented | 2 (infrastructure-only) |
| Routes in spec but **NOT** registered | 0 |
| Spec operations manually excluded from `RegisterHandlers` | 34 |

The spec now documents all user-facing API routes. Two infrastructure-only endpoints remain intentionally undocumented:

- `GET /ws` — WebSocket upgrade (not REST)
- `POST /admin/migrate-statuses` — one-off admin utility

## Two-track architecture

The project uses a **hybrid routing model**:

### Track A — generated `RegisterHandlers` path

Routes whose handler signatures match the oapi-codegen `ServerInterface`
(typed path params, typed request bodies). These are auto-registered via
`api.RegisterHandlers(v1, apiServer)` at `cmdb-core/cmd/server/main.go:346`.

### Track B — manual `v1.GET/POST/PUT/DELETE`

Routes whose handlers use `*gin.Context` directly (parse `c.Param()` and
`c.ShouldBindJSON()` themselves). These are registered by name in
`cmd/server/main.go` lines 349–449.

To keep the spec a complete source of truth for API consumers while
preserving the hand-rolled handler signatures, the Track-B operations are
excluded from interface generation via
`cmdb-core/oapi-codegen.yaml:output-options.exclude-operation-ids`.

## Excluded operations (Track B)

Listed in `cmdb-core/oapi-codegen.yaml`. Regeneration via
`make generate-api` leaves `ServerInterface` unaware of these — they are
routed explicitly in `main.go`. The 34 excluded operations:

- `downloadImportTemplate`, `getAssetLifecycle`, `getAssetLocationHistory`,
  `getAssetQRData`, `confirmAssetLocation`, `getLocationAssetCounts`
- `getUpgradeRules`, `createUpgradeRule`, `updateUpgradeRule`, `deleteUpgradeRule`
- `getItemScanHistory`, `createItemScanRecord`
- `listUserRoles`, `assignRoleToUser`, `removeRoleFromUser`
- `listNotifications`, `countUnreadNotifications`, `markNotificationRead`,
  `markAllNotificationsRead`
- `getRackQRData`
- `locationDetectGetDiffs`, `locationDetectGetSummary`,
  `locationDetectGetAnomalies`, `locationDetectGetReport`
- `getCapacityPlanning`, `getFleetMetricsSummary`
- `syncGetChanges`, `syncGetState`, `syncGetConflicts`, `syncResolveConflict`,
  `syncSnapshot`, `syncStats`
- `getMaintenanceOrderComments`, `createMaintenanceOrderComment`

## Known caveats

Running `make generate-api` today will **still fail to compile** because the
committed `generated.go` contains asset fields (BMC, warranty, purchase)
added by direct edits in `feat: add BMC/iLO/iDRAC fields to assets` and
`feat: add warranty & lifecycle fields` — these fields are **not** declared
in the spec's `Asset` schema. A separate cleanup pass should either (a)
backfill those fields into the schema or (b) restore generated.go from a
clean regeneration and re-curate. That cleanup is tracked outside this
drift report.

## Remediation backlog

1. Backfill Asset schema with BMC / warranty / purchase fields, so
   `make generate-api` succeeds again. Until then, do not run the codegen.
2. Evaluate whether any Track-B operations warrant migration to Track A
   (typed signatures). High-value candidates: upgrade-rules CRUD,
   user-roles CRUD, notifications.
3. Add a CI check that compares `main.go` route registrations against spec
   paths so new drift is caught at PR time.
