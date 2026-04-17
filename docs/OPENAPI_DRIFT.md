# OpenAPI Drift Report

Generated: 2026-04-17

## Summary

| Category | Count |
|---|---|
| Routes registered but NOT in OpenAPI | **35** |
| Routes in OpenAPI but NOT registered | 0 |
| Method mismatches | 0 |

All routes defined in `api/openapi.yaml` are correctly registered via `api.RegisterHandlers(v1, apiServer)` at `cmdb-core/cmd/server/main.go:346`. The drift is **one-directional**: custom endpoints added after spec generation lack documentation.

## Undocumented Routes (35)

Routes live in `cmdb-core/cmd/server/main.go` after line 346.

### Asset lifecycle, QR, locations

| Method | Path | main.go line |
|---|---|---|
| GET | `/assets/import-template` | 357 |
| GET | `/assets/{id}/lifecycle` | 387 |
| GET | `/assets/{id}/location-history` | 435 |
| GET | `/assets/{id}/qr-data` | 426 |
| POST | `/assets/{id}/confirm-location` | 428 |
| POST | `/assets/{id}/upgrade-recommendations/{category}/accept` | 389 |
| GET | `/locations/asset-counts` | 373 |

### Upgrade rules CRUD

| Method | Path | main.go line |
|---|---|---|
| GET | `/upgrade-rules` | 390 |
| POST | `/upgrade-rules` | 391 |
| PUT | `/upgrade-rules/{id}` | 392 |
| DELETE | `/upgrade-rules/{id}` | 393 |

### Inventory sub-resources

| Method | Path | main.go line |
|---|---|---|
| GET | `/inventory/tasks/{id}/racks-summary` | 362 |
| GET | `/inventory/tasks/{id}/discrepancies` | 363 |
| GET | `/inventory/tasks/{id}/items/{itemId}/scan-history` | 366 |
| POST | `/inventory/tasks/{id}/items/{itemId}/scan-history` | 367 |
| GET | `/inventory/tasks/{id}/items/{itemId}/notes` | 368 |
| POST | `/inventory/tasks/{id}/items/{itemId}/notes` | 369 |
| POST | `/inventory/tasks/{id}/items/{itemId}/resolve` | 370 |

### Energy, capacity, fleet, predictions

| Method | Path | main.go line |
|---|---|---|
| GET | `/energy/breakdown` | 349 |
| GET | `/energy/summary` | 350 |
| GET | `/energy/trend` | 351 |
| GET | `/capacity-planning` | 438 |
| GET | `/fleet-metrics` | 441 |
| GET | `/prediction/rul/{id}` | 385 |
| GET | `/prediction/failure-distribution` | 386 |

### Topology, racks, network

| Method | Path | main.go line |
|---|---|---|
| GET | `/racks/stats` | 354 |
| GET | `/racks/{id}/maintenance` | 359 |
| GET | `/racks/{id}/network-connections` | 378 |
| POST | `/racks/{id}/network-connections` | 379 |
| DELETE | `/racks/{id}/network-connections/{connectionId}` | 380 |
| GET | `/racks/{id}/qr-data` | 427 |
| GET | `/topology/dependencies` | 374 |
| POST | `/topology/dependencies` | 375 |
| DELETE | `/topology/dependencies/{id}` | 376 |
| GET | `/topology/graph` | 377 |

### Location detection

| Method | Path | main.go line |
|---|---|---|
| GET | `/location-detect/diffs` | 431 |
| GET | `/location-detect/summary` | 432 |
| GET | `/location-detect/anomalies` | 433 |
| GET | `/location-detect/report` | 434 |

### Notifications, users, sessions

| Method | Path | main.go line |
|---|---|---|
| GET | `/notifications` | 409 |
| GET | `/notifications/count` | 410 |
| POST | `/notifications/{id}/read` | 411 |
| POST | `/notifications/read-all` | 412 |
| GET | `/users/{id}/roles` | 403 |
| POST | `/users/{id}/roles` | 404 |
| DELETE | `/users/{id}/roles/{roleId}` | 405 |
| DELETE | `/users/{id}` | 406 |
| GET | `/users/{id}/sessions` | 415 |

### Monitoring, activity, audit, maintenance

| Method | Path | main.go line |
|---|---|---|
| GET | `/activity-feed` | 381 |
| GET | `/audit/events/{id}` | 382 |
| GET | `/monitoring/alerts/trend` | 358 |
| GET | `/maintenance/orders/{id}/comments` | 371 |
| POST | `/maintenance/orders/{id}/comments` | 372 |

### Sync, WebSocket, admin utility

| Method | Path | main.go line |
|---|---|---|
| GET | `/sync/changes` | 444 |
| GET | `/sync/state` | 445 |
| GET | `/sync/conflicts` | 446 |
| POST | `/sync/conflicts/{id}/resolve` | 447 |
| GET | `/sync/snapshot` | 448 |
| GET | `/sync/stats` | 449 |
| GET | `/ws` | 487 |
| POST | `/admin/migrate-statuses` | 396 |

## Recommendations

1. **Priority for spec backfill** — domain-bounded CRUD is highest value:
   - upgrade-rules CRUD (4 routes)
   - users + roles CRUD (4 routes)
   - topology dependencies (4 routes)
   - notifications (4 routes)

2. **Keep out of public spec** — admin/migrate-statuses and `/ws` belong in an internal/admin spec, not the client-facing one.

3. **Effort estimate**: ~35 routes × ~30 lines of spec each ≈ 1000 lines of YAML + schema definitions. Suggest splitting into a follow-up phase rather than bundling into current P1.
