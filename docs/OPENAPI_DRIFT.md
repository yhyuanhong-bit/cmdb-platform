# OpenAPI Drift Report

Last updated: 2026-04-17

## Summary

| Category | Count |
|---|---|
| Documented routes (`api/openapi.yaml`) | ~140 |
| Routes registered in `main.go` but **NOT** documented | 2 (infrastructure-only) |
| Routes in spec but **NOT** registered | 0 |
| Spec operations manually excluded from `RegisterHandlers` | 60 |

The spec now documents all user-facing API routes. `make generate-api`
succeeds cleanly and the resulting `generated.go` compiles without
hand-editing. Two infrastructure-only endpoints remain intentionally
undocumented:

- `GET /ws` — WebSocket upgrade (not REST)
- `POST /admin/migrate-statuses` — one-off admin utility

## Two-track architecture

The project uses a **hybrid routing model**:

### Track A — generated `RegisterHandlers` path

Routes whose handler signatures match the oapi-codegen `ServerInterface`
(typed path params, typed request bodies, typed query params). These are
auto-registered via `api.RegisterHandlers(v1, apiServer)` at
`cmdb-core/cmd/server/main.go:346`.

### Track B — manual `v1.GET/POST/PUT/DELETE`

Routes whose handlers use `*gin.Context` directly (parse `c.Param()` and
`c.ShouldBindJSON()` themselves). These are registered by name in
`cmd/server/main.go` lines 349–449.

To keep the spec a complete source of truth for API consumers while
preserving the hand-rolled handler signatures, the Track-B operations are
excluded from interface generation via
`cmdb-core/oapi-codegen.yaml:output-options.exclude-operation-ids`.

## Excluded operations (Track B)

The authoritative list lives in `cmdb-core/oapi-codegen.yaml`.
Regeneration via `make generate-api` leaves `ServerInterface` unaware of
these — they are routed explicitly in `main.go`. Categories:

- **Asset lifecycle / QR / templates:** `downloadImportTemplate`,
  `getAssetLifecycle`, `getAssetLocationHistory`, `getAssetQRData`,
  `confirmAssetLocation`, `getLocationAssetCounts`, `getRackQRData`
- **Upgrade rules CRUD + accept flow:** `getUpgradeRules`,
  `createUpgradeRule`, `updateUpgradeRule`, `deleteUpgradeRule`,
  `acceptUpgradeRecommendation`, `getAssetUpgradeRecommendations`,
  `getAssetRUL`
- **Inventory scan / items / notes / discrepancies:** `getItemScanHistory`,
  `createItemScanRecord`, `getItemNotes`, `createItemNote`,
  `getInventoryDiscrepancies`, `getInventoryRacksSummary`,
  `resolveInventoryDiscrepancy`
- **User roles & sessions:** `listUserRoles`, `assignRoleToUser`,
  `removeRoleFromUser`, `listUserSessions`
- **Notifications:** `listNotifications`, `countUnreadNotifications`,
  `markNotificationRead`, `markAllNotificationsRead`
- **Location detection:** `locationDetectGetDiffs`,
  `locationDetectGetSummary`, `locationDetectGetAnomalies`,
  `locationDetectGetReport`
- **Capacity / fleet / energy:** `getCapacityPlanning`,
  `getFleetMetricsSummary`, `getEnergyBreakdown`, `getEnergyTrend`
- **Sync:** `syncGetChanges`, `syncGetState`, `syncGetConflicts`,
  `syncResolveConflict`, `syncSnapshot`, `syncStats`
- **Maintenance / work orders comments:** `getMaintenanceOrderComments`,
  `createMaintenanceOrderComment`, `listWorkOrderComments`,
  `createWorkOrderComment`, `getRackMaintenance`
- **Racks:** `listRackNetworkConnections`, `createRackNetworkConnection`,
  `deleteRackNetworkConnection`
- **Topology / dependencies:** `listAssetDependencies`,
  `deleteAssetDependency`, `getTopologyGraph`
- **Sensors:** `listSensors`, `updateSensor`, `deleteSensor`,
  `sensorHeartbeat`
- **Audit / activity:** `getActivityFeed`, `getAuditEventDetail`

## Resolved drift (2026-04-17)

The previous version of this document listed a known caveat: running
`make generate-api` would fail to build because `generated.go` had been
hand-edited with asset fields (BMC, warranty, purchase) and Location
lat/long that were not in the spec. That drift has now been fixed by:

1. Adding 11 fields to the spec's `Asset` schema: `bmc_ip`, `bmc_type`,
   `bmc_firmware`, `purchase_date`, `purchase_cost`, `warranty_start`,
   `warranty_end`, `warranty_vendor`, `warranty_contract`, `eol_date`,
   `expected_lifespan_months`.
2. Adding matching fields to the `updateAsset` inline request body, plus
   `serial_number` and `ip_address`.
3. Adding `latitude` / `longitude` to the `Location` schema.
4. Adding `property_number` / `control_number` to the inventory-import
   item schema.
5. Expanding the `exclude-operation-ids` list from 34 to 60 to cover all
   Track-B handlers whose signatures use untyped `*gin.Context`.
6. Updating `ListInventoryItems` to accept the typed
   `ListInventoryItemsParams` struct oapi-codegen now generates (since
   the spec declares Page / PageSize query params).

After these changes, `make generate-api && go build ./...` succeeds
without hand-editing `generated.go`.

## Remediation backlog

1. Evaluate whether any Track-B operations warrant migration to Track A
   (typed signatures). High-value candidates: upgrade-rules CRUD,
   user-roles CRUD, notifications.
2. Add a CI check that runs `make generate-api` and fails if
   `generated.go` changes, catching future drift at PR time.
3. Add a CI check that compares `main.go` route registrations against
   spec paths.
