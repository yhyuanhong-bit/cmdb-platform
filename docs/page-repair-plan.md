# CMDB Platform — 完整修復計劃

> 基於頁面功能測試報告，逐頁逐按鈕列出所有修復項
> 生成日期：2026-04-04
> 涵蓋：38 頁面，~340 個互動元素，~85 個死按鈕，~18 組 hardcoded 數據

---

## 修復分級說明

| 級別 | 定義 | 處理方式 |
|------|------|---------|
| **P0-CRITICAL** | 核心流程斷裂 | 修改代碼邏輯 |
| **P1-HIGH** | 功能缺口影響體驗 | 接通 API 或修復 bug |
| **P2-MEDIUM** | 死按鈕 | 接通 handler 或移除按鈕 |
| **P3-LOW** | hardcoded 數據 | 替換為 API 數據或衍生值 |
| **ACCEPT** | 可接受的靜態內容 | 不需修復 |

---

## 1. Login (`/login`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Username input | 49 | `setUsername(e.target.value)` | OK | 無需修復 |
| Password input | 65 | `setPassword(e.target.value)` | OK | 無需修復 |
| Login button | 77 | `handleSubmit → login()` | OK | 無需修復 |
| Demo credentials | 87 | 顯示 admin/admin123 | ACCEPT | 開發環境可接受，生產移除 |

**頁面評級：A- → 無需修復**

---

## 2. Welcome (`/welcome`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Onboarding tabs (5) | 71-86 | 按鈕存在但 `setActiveTab` 未暴露 | P2 | 加 `onClick={() => setActiveTab(tab.id)}` |
| Help icon | 91 | NONE | P2 | 加 `onClick={() => navigate('/help/troubleshooting')}` |
| Settings icon | 98 | NONE | P2 | 加 `onClick={() => navigate('/system/settings')}` |
| Skip button | 160 | `navigate('/dashboard')` | OK | 無需修復 |
| Next Step button | 166 | `navigate('/locations')` | OK | 無需修復 |

---

## 3. Dashboard (`/dashboard`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Total Assets card | 233 | `navigate('/assets')` | OK | 無需修復 |
| Rack Occupancy card | 250 | `navigate('/racks')` | OK | 無需修復 |
| Critical Alarms card | 266 | `navigate('/monitoring')` | OK | 無需修復 |
| Active Inventory card | 280 | `navigate('/inventory')` | OK | 無需修復 |
| Sync now button | 462 | `navigate('/inventory')` | OK | 無需修復 |
| Alert row click | 508 | `navigate('/monitoring')` | OK | 無需修復 |
| Predictive analytics | 540 | `navigate('/predictive')` | OK | 無需修復 |
| View all monitoring | 546 | `navigate('/monitoring')` | OK | 無需修復 |
| Heatmap cells | 375 | 僅 hover tooltip | ACCEPT | 視覺裝飾，可接受 |

**Hardcoded 數據修復：**

| 數據 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Occupancy 76% | 202 | hardcoded | P3 | 從 `useRacks` 計算 used_u/total_u 比例 |
| Heatmap grid | 34-39 | seeded random | P3 | 用 `useMetrics` 取真實溫度數據填充 |
| Asset Lifecycle 82%/13%/5% | 393-399 | hardcoded | P3 | 從 `useAssets` 按 status 統計百分比 |
| "Mar-24 Audit" task | 288 | hardcoded | P3 | 從 `useInventoryTasks` 取最新任務 |
| "12% vs last month" | 246 | hardcoded | P3 | 需後端提供歷史對比端點，暫標記 "N/A" |
| ci_id 顯示 UUID | 512 | 顯示 `evt.ci_id` (UUID) | P1 | 用 `useAssets` 做 asset_id → asset_tag lookup |

---

## 4. GlobalOverview (`/locations`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Add Location button | 356 | `setShowCreateLocation(true)` | OK | 無需修復 |
| Retry button | 334 | `rootLocationsQ.refetch()` | OK | 無需修復 |
| Map markers | 420-423 | `navigate(/locations/${slug})` | OK | 無需修復 |
| Country cards | 515-519 | `navigate(/locations/${slug})` | OK | 無需修復 |
| Alert cards | 539 | `navigate('/monitoring')` | OK | 無需修復 |

**Hardcoded 數據修復：**

| 數據 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| MAP_META slugs | 14-20 | china/japan/singapore vs seed `tw` | P0 | 改 MAP_META key 為 `tw` 或動態從 API 取位置 metadata 的 lat/lng |
| SUMMARY_KPI | 21-23 | hardcoded PUE/power/uptime | P3 | 從 locations metadata 聚合計算 |
| LAST_SYNC | 14 | hardcoded timestamp | P3 | 用 `new Date().toISOString()` 或從後端取 |

---

## 5. RegionOverview (`/locations/:countrySlug`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| GLOBAL breadcrumb | 255 | `navigate("/locations")` | OK | 無需修復 |
| Retry button | 238 | `refetch()` | OK | 無需修復 |
| Region cards | 313-320 | `navigate(...)` | OK | 無需修復 |

**頁面評級：B+ → 僅 metadata 依賴問題（P3）**

---

## 6. CityOverview (`/locations/:countrySlug/:regionSlug`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| All navigation buttons | 多處 | 全部正常 | OK | 無需修復 |
| Card/List view toggle | 496/507 | 正常 | OK | 無需修復 |
| Sort dropdown | 523 | 正常 | OK | 無需修復 |

**Hardcoded：** sparkline 欄位 seed 無值 → P3：在 seed.sql location metadata 加 `"sparkline": [80,82,79,84,81]`

---

## 7. CampusOverview (`/locations/:countrySlug/:regionSlug/:citySlug`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| All breadcrumbs | 515-529 | 全部正常 | OK | 無需修復 |
| IDC accordion | 249-251 | `setExpanded(!expanded)` | OK | 無需修復 |

**關鍵修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| seed 無 IDC 層級位置 → campus 子項空 | P1 | 在 seed.sql 加 IDC 子位置（如 Neihu IDC-A、IDC-B）或改 UI 直接顯示 campus 下的 racks |
| 缺 used/available/reserved metadata | P3 | 在 seed.sql campus metadata 加這三個欄位 |

---

## 8. RackManagement (`/racks`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Add Rack button | 144 | `navigate('/racks/add')` | OK | 無需修復 |
| Search input | 162 | `setSearch(e.target.value)` | OK | 無需修復 |
| Table row click | 207 | `navigate(/racks/${rack.id})` | OK | 無需修復 |
| View button (per row) | 265 | NONE | P2 | 加 `onClick={() => navigate(/racks/${rack.id})}` |
| Edit button (per row) | 271 | NONE | P2 | 加 `onClick={() => navigate(/racks/${rack.id}?edit=true)}` |
| More button (per row) | 278 | NONE | P2 | 加 dropdown menu (delete/export) 或移除 |

**Hardcoded 數據：**

| 數據 | 行號 | 級別 | 修復方案 |
|------|------|------|---------|
| recentEvents | 8-39 | P3 | 用 `useAuditEvents({ module: 'topology' })` 取最近事件 |
| rackA01Layout | 41-57 | P3 | 用 `useRackAssets(selectedRackId)` 取真實設備佈局 |
| Breadcrumb "IDC Alpha > Module 1" | ~145 | P3 | 用 locationContext 真實路徑 |

---

## 9. RackDetail (`/racks/:id`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Edit button | ~850 | `setEditingRack(true)` | OK | 無需修復 |
| Delete button | ~855 | `deleteRack.mutate(rackId)` | OK | 無需修復 |
| Save (edit mode) | ~860 | `updateRack.mutate({id, data})` | OK | 無需修復 |
| Cancel (edit mode) | ~865 | `setEditingRack(false)` | OK | 無需修復 |
| View toggle (FRONT/REAR) | 182 | `setView(...)` | OK | 無需修復 |
| Tab buttons | 909-923 | `setActiveTab(...)` | OK | 無需修復 |
| Asset click | 221 | `navigate('/assets/detail')` | P1 | 改為 `navigate(/assets/${asset.id})` |

**Hardcoded 數據：**

| 數據 | 行號 | 級別 | 修復方案 |
|------|------|------|---------|
| alerts (3 items) | ~160 | P3 | 用 `useAlerts({ asset_id })` 取關聯告警 |
| networkConnections | ~170 | P3 | 暫保留，需後端拓撲端點 |
| maintenanceHistory | ~180 | P3 | 用 `useWorkOrders({ asset_id })` |
| environmentMetrics | ~190 | P3 | 用 `useMetrics` 取真實數據 |
| Console tab gauges | ~800 | P3 | 用 `useMetrics` 取 power/temp/humidity |

---

## 10. DataCenter3D (`/racks/3d`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Rack blocks click | 354-356 | `navigate("/racks/detail")` | P0 | 改為 `navigate(/racks/${rack.id})` |
| Deploy Agent button | 276 | NONE | P2 | 移除或加 placeholder toast |
| Zoom +/- buttons | 399-405 | NONE | P2 | 加 CSS transform scale 或移除 |
| Heat mode toggle | 376 | `setHeatMode(...)` | OK | 無需修復 |
| Full Analytics | 416 | `navigate("/monitoring/energy")` | OK | 無需修復 |

**Hardcoded 數據：**

| 數據 | 行號 | 級別 | 修復方案 |
|------|------|------|---------|
| locationTree (Germany/Frankfurt) | 22-54 | P1 | 用 `useRootLocations` + `useLocationChildren` 建樹 |
| alerts | 92-96 | P3 | 用 `useAlerts` |
| Right panel stats | ~100 | P3 | 從 rack metadata 衍生 |

---

## 11. FacilityMap (`/racks/facility-map`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| View Details button | 275/353 | `navigate('/racks/detail')` | P0 | 改為 `navigate(/racks/${selectedRack.id})` |
| Maintenance button | 361 | NONE | P2 | 加 `navigate(/maintenance/add)` |
| Breadcrumb links | 275 | NONE | P2 | 加導航到 `/racks` |

---

## 12. AddNewRack (`/racks/add`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Create Rack button | 207-228 | `createRack.mutate(...)` | OK | 無需修復 |
| Cancel button | 202 | `navigate('/racks')` | OK | 無需修復 |
| All form inputs | 88-194 | 全部有 state binding | OK | 無需修復 |

**Bug 修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| row_label 設為完整 rack ID | P1 | 加獨立 row_label 欄位（如 A/B/C 選擇器）|
| 位置下拉 hardcoded | P3 | 用 `useRootLocations` + `useLocationChildren` 動態取值 |

---

## 13. AssetManagement (`/assets`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Create Asset button | 255-261 | `setShowCreateAsset(true)` | OK | 無需修復 |
| Import button | 262 | NONE | P2 | 加檔案上傳 modal 或標記 "Coming Soon" |
| Export CSV button | 266 | NONE | P2 | 加 `onClick` 用前端生成 CSV 下載 |
| View mode toggles | 227-248 | `setViewMode(...)` | OK | 無需修復 |
| Row click | 306 | `navigate(/assets/${asset.id})` | OK | 無需修復 |
| Row view button | 336 | NONE | P2 | 加 `navigate(/assets/${asset.id})` |
| Row more button | 341 | NONE | P2 | 加 dropdown (edit/delete) |
| Pagination | 372-411 | `setCurrentPage(p)` | OK | 無需修復 |
| Search input | 180-186 | `setSearch(e.target.value)` 但後端不支持 | P1 | 後端 ListAssets 加 search 參數（模糊匹配 name/asset_tag）|
| Location filter | 215-223 | 欄位不存在 | P1 | 改用 location_id 搭配位置 dropdown，或改為 client-side filter |
| Type filter | 190-200 | 正常送 API | OK | 無需修復 |
| Status filter | 203-212 | 正常送 API | OK | 無需修復 |

---

## 14. AssetDetail (`/assets/:assetId`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Edit button | ~866 | `setEditing(true)` | OK | 無需修復 |
| Delete button | ~870 | `deleteAsset.mutate(assetId)` | OK | 無需修復 |
| Save (edit mode) | ~875 | `updateAsset.mutate({id, data})` | OK | 無需修復 |
| Cancel (edit mode) | ~880 | `setEditing(false)` | OK | 無需修復 |
| Audit Log link | ~876 | `navigate('/audit')` | OK | 無需修復 |
| Tab buttons | 887-900 | `setActiveTab(...)` | OK | 無需修復 |
| Filter Logs button | 683 | NONE | P2 | 加 filter modal 或移除 |
| Schedule Entry button | 689 | NONE | P2 | 加 `navigate('/maintenance/add')` |
| Maintenance pagination | 749-754 | NONE | P2 | 加 client-side 分頁 |

**Hardcoded Tab 數據：**

| Tab | 行號範圍 | 級別 | 修復方案 |
|------|---------|------|---------|
| Health tab 全 mock | 100-500 | P3 | 用 `useMetrics(assetId, 'cpu_usage')` + `useMetrics(assetId, 'temperature')` |
| Usage tab 全 mock | 484-510 | P3 | 用 `useMetrics` 取 CPU/memory 歷史數據 |
| Maintenance tab 全 mock | 666-754 | P3 | 用 `useWorkOrders({ asset_id })` 取關聯工單 |

---

## 15. AssetLifecycle (`/assets/lifecycle`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Timeline link | 105 | `navigate('/assets/lifecycle/timeline')` | OK | 無需修復 |
| Table row click | 222 | `navigate(/assets/${id})` | OK | 無需修復 |
| Filters button | 111 | NONE | P2 | 加 filter modal 或移除 |
| Export Report button | 115 | NONE | P2 | 加前端 CSV 導出 |

**Hardcoded：** financialItemsData ($48.2M) → P3：需後端聚合端點或移除財務區塊

---

## 16. AssetLifecycleTimeline (`/assets/lifecycle/timeline`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Back breadcrumb | 153-159 | `navigate('/assets/lifecycle')` | OK | 無需修復 |
| View Details buttons | 262 | NONE | P2 | 加詳情展開 |
| Generate Audit Report | 344 | NONE | P2 | 加報告生成或移除 |

**關鍵修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| **路由缺 `:assetId` → API 永不觸發** | **P0** | App.tsx 路由改為 `/assets/lifecycle/timeline/:assetId`；AssetLifecycle 的 Timeline 連結改為 `navigate(/assets/lifecycle/timeline/${asset.id})` |

---

## 17. AutoDiscovery (`/assets/discovery`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Batch Approve | 139 | NONE | P2 | 加 `selectedRows.forEach(id => approveAsset(id))` 或標記 Coming Soon |
| Ignore Selected | 131 | NONE | P2 | 同上 |
| Per-row Approve | 327 | NONE | P2 | 同上 |
| Per-row Ignore | 333 | NONE | P2 | 同上 |
| Source filter | 212 | `setSourceFilter(...)` | OK | 無需修復 |
| Status filter | 230 | `setStatusFilter(...)` | OK | 無需修復 |
| Manage Schedule | 440 | NONE | P2 | 移除或標記 Coming Soon |
| Pagination buttons | 355-380 | NONE | P2 | 加 client-side 分頁邏輯 |

**結構性問題：** 此頁用 assets API 冒充 discovery — 需後端 discovery 端點或重新定位為 "Asset Import Review"

---

## 18. EquipmentHealth (`/assets/equipment-health`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Download Report | 112 | NONE | P2 | 移除或加前端導出 |
| Alert Settings | 118 | NONE | P2 | 加 `navigate('/monitoring/sensors')` |
| Create Work Order | 293 | `navigate('/maintenance/add')` | OK | 無需修復 |
| Quick actions | 330 | `navigate(item.action)` | OK | 無需修復 |

**整頁 100% mock** → P3：需後端 health monitoring 端點或降級為 Dashboard 子區塊

---

## 19. ComponentUpgrade (`/assets/upgrades`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Filter tabs | 152-163 | `setActiveFilter(...)` | OK | 無需修復 |
| Card toggle | 275 | `toggleSelect(card.id)` | OK | 無需修復 |
| Learn More | 284 | NONE | P2 | 移除或加詳情展開 |
| Schedule Maintenance | 322 | `navigate('/maintenance/add')` | OK | 無需修復 |
| Request Selection | 328 | NONE | P2 | 移除或加工單建立邏輯 |

**整頁 100% mock** → P3：需後端 recommendation 端點或降級為靜態指南

---

## 20. MaintenanceHub (`/maintenance`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Month nav (← today →) | 179-185 | NONE | P2 | 加 `setCurrentWeek(w => w ± 1)` 或移除 |
| Calendar slot cards | 213-222 | NONE | P2 | 加 `navigate(/maintenance/task/${task.id})` |
| Row divs | 244 | `navigate('/maintenance/task')` | P0 | **改為 `navigate(/maintenance/task/${wo.id})`** |
| Task Dispatch | 378 | `navigate('/maintenance/dispatch')` | OK | 無需修復 |
| Work Order Management | 385 | `navigate('/maintenance/workorder')` | OK | 無需修復 |
| Create Window | 392 | `navigate('/maintenance/add')` | OK | 無需修復 |
| View toggles | 429/441 | `setViewMode(...)` | OK | 無需修復 |
| Search | 463 | `setSearch(...)` | OK | 無需修復 |
| Type filter | 489 | `setTypeFilter(...)` 但未消費 | P1 | 在 records 篩選中消費 `typeFilter` |
| Status filter | 502 | `setStatusFilter(...)` 但未消費 | P1 | 在 records 篩選中消費 `statusFilter` |
| Date from/to | 342-343 | 有 state 但未消費 | P1 | 加日期範圍篩選邏輯 |
| Pagination (6 buttons) | 532-549 | NONE | P2 | 加 client-side 分頁 |

**Hardcoded：** weekData 週曆 → P3：從 workOrders 按 scheduled_start 分組

**Summary cards bug：** `||` 應為 `??`（0 被當 falsy）→ P0

---

## 21. WorkOrder (`/maintenance/workorder`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Review (approve) | 157 | `onTransition(order.id, 'APPROVED')` | OK | 無需修復 |
| CI name click | 132 | `navigate('/assets/detail')` | P1 | 改為 `navigate(/assets/${asset.id})` |
| View Task | 148 | `navigate('/maintenance/task')` | P1 | 改為帶 ID |
| History button | 163 | NONE | P2 | 加歷史展開或移除 |
| New Change Request | 340 | NONE | P2 | 加 `navigate('/maintenance/add')` |
| AI auto-review | 272 | NONE | P2 | 標記 Coming Soon |
| Filter tabs | 377 | `setActiveTab(...)` 但未消費 | P1 | 在 order list filter 中消費 activeTab |
| Pagination | ~400 | `currentPage` 但無 setter | P2 | 修復 useState 解構，加分頁邏輯 |

**Hardcoded：** StatCards (12/28/05/19) → P3：從 workOrders 統計

---

## 22. AddMaintenanceTask (`/maintenance/add`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| All form inputs | 69-180 | 全部正常 | OK | 無需修復 |
| Create Task | 199 | `createWorkOrder.mutate(...)` | OK | 無需修復 |
| Cancel | 192 | `navigate('/maintenance')` | OK | 無需修復 |

**Bug 修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| assignee 未送 API | P1 | 在 mutation payload 加 `assignee_id: selectedAssignees[0]` |
| Priority 標籤 bug（兩個 Medium）| P1 | 修正 line 103 label 為 "Low" |
| Assignees hardcoded | P3 | 用 `useUsers` 取真實用戶列表 |

---

## 23. MaintenanceTaskView (`/maintenance/task/:id`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Complete button | 142 | `transitionWO.mutate(...)` | OK | 無需修復 |
| Back button | 92 | `navigate('/maintenance')` | OK | 無需修復 |
| Create WO | 124 | `navigate('/maintenance/workorder')` | OK | 無需修復 |
| Dispatch | 131 | `navigate('/maintenance/dispatch')` | OK | 無需修復 |
| Edit Task | 138 | NONE | P2 | 加 `navigate(/maintenance/task/${taskId}?edit=true)` 或 inline edit |
| Post Comment | 230 | NONE | P2 | 在 mutation payload 或用 workOrderLogs 的 comment 機制 |
| Asset ID links | 192 | `navigate('/assets/detail')` | P1 | 改為帶 asset ID |

**Hardcoded：** taskSteps/associatedAssets/envMetrics → P3：分別用 API 替換

---

## 24. TaskDispatch (`/maintenance/dispatch`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Task list items | 243 | `setSelectedTask(...)` | OK | 無需修復 |
| Task ID link | 279 | `navigate('/maintenance/task')` | P1 | 改為帶 ID |
| Add Task | 190 | NONE | P1 | 加 `navigate('/maintenance/add')` |
| Assign button | 309 | NONE | P1 | 加 `useUpdateWorkOrder` 設定 assignee_id |
| Auto-assign | 371 | NONE | P2 | 簡易實現：選第一個 available technician |
| Confirm assign | 378 | NONE | P2 | 確認並呼叫 mutation |

**Hardcoded：** TECHNICIANS/ZONE_DATA → P3：用 `useUsers` 取技術員列表

---

## 25. HighSpeedInventory (`/inventory`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| New Task button | 155 | `setShowCreateTask(true)` | OK | 無需修復 |
| Complete Task | 174 | `completeTask.mutate(...)` | OK | 無需修復 |
| Error toggle | 226 | `setShowErrors(...)` | OK | 無需修復 |
| Rack cards | 298 | `navigate('/inventory/detail')` | P0 | **改為 `navigate(/inventory/detail?taskId=${currentTask.id})`** |
| Discrepancy items | 364 | `navigate('/inventory/detail')` | P0 | 同上，帶 taskId |
| Scan QR button | 161 | NONE | P2 | 加 modal 手動輸入 asset_tag → `useScanItem` |
| Manual QR | 165 | NONE | P2 | 同上 |
| Generate Report | 169 | NONE | P2 | 加前端 CSV 導出 |
| Verify/Add/Register | 393-408 | NONE | P2 | 分別接 `useScanItem` 或 `useCreateAsset` |

---

## 26. InventoryItemDetail (`/inventory/detail`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Back button | 159 | `navigate('/inventory')` | OK | 無需修復 |
| Verify Asset | 189 | `scanItem.mutate(...)` | OK | 無需修復 |
| Flag Issue | 199 | `scanItem.mutate(...)` | OK | 無需修復 |
| Mark Resolved | 489 | `scanItem.mutate(...)` | OK | 無需修復 |
| Print Label | 204 | NONE | P2 | 加 `window.print()` 或移除 |
| Escalate | 493 | NONE | P2 | 加 `navigate('/maintenance/add')` 預填資訊 |
| Attach file | 469 | NONE | P2 | 標記 Coming Soon |
| Photo | 476 | NONE | P2 | 標記 Coming Soon |
| Submit Note | 478 | NONE | P2 | note 輸入框改為 `<textarea>`，submit 用 console.log 暫存 |

---

## 27. MonitoringAlerts (`/monitoring`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Search | 133 | `setSearch(...)` | OK | 無需修復 |
| Severity filter | 142 | `setSeverity(...)` 正確送 API | OK | 無需修復 |
| Acknowledge | 251 | `acknowledgeAlert.mutate(...)` | OK | 無需修復 |
| Resolve | 259 | `resolveAlert.mutate(...)` | OK | 無需修復 |
| Topology analysis | 168 | `navigate('/monitoring/topology')` | OK | 無需修復 |
| Date filter | 152 | onChange 未綁定 | P1 | 綁定 state + 傳入 API query |
| Location filter | 158 | onChange 未綁定 | P1 | 綁定 state + client-side filter |
| Export Report | 176 | NONE | P2 | 加前端 CSV 導出 |
| Silence Management | 182 | NONE | P2 | 標記 Coming Soon |
| Per-alert Silence | 270 | NONE | P2 | 標記 Coming Soon |
| Pagination | 289-310 | `setCurrentPage` 但未 slice data | P1 | 加 `filtered.slice((page-1)*10, page*10)` |

**Hardcoded：** TREND_DATA → P3：需 metrics 聚合端點

---

## 28. SystemHealth (`/monitoring/health`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Breadcrumb | 136 | `navigate("/monitoring")` | OK | 無需修復 |
| Asset ID in table | 406 | `navigate("/assets/detail")` | P1 | 改為帶 asset ID |
| View all alerts | 439 | `navigate("/monitoring")` | OK | 無需修復 |
| Open button (table) | 425 | NONE | P2 | 加 `navigate(/monitoring?alert_id=${id})` |

**Hardcoded：** health donut/trend/resource/uptime/sync → P3：逐步用 `useSystemHealth` 擴展

---

## 29. SensorConfiguration (`/monitoring/sensors`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Sensor toggles | 361 | `toggleSensor(...)` local state | P1 | 需後端 sensor CRUD 或連接 alert_rules |
| Threshold sliders | 445/467 | `updateThreshold(...)` local state | P1 | 同上 |
| Global polling | 495 | `setGlobalPolling(...)` local state | P2 | 暫可接受為 local |
| **Save Configuration** | **274** | **NONE** | **P0** | **用 `useCreateAlertRule` 把 threshold 轉為 alert_rules 保存** |
| Discover Sensors | 282 | NONE | P2 | 標記 Coming Soon |
| Reset All | 522 | NONE | P2 | `setThresholds(INITIAL)` |
| Export Configuration | 530 | NONE | P2 | JSON.stringify + download |
| Import Configuration | 536 | NONE | P2 | file input + parse |
| Edit rule | 582 | NONE | P2 | 加 inline edit |
| Delete rule | 590 | NONE | P2 | 加 confirm + delete（需後端端點）|
| Add New Rule | 600 | NONE | P2 | 加 modal → `useCreateAlertRule` |
| Polling dropdown (per sensor) | 389 | `onChange={() => {}}` stub | P2 | 實現或移除 |

**結構性修復：** 改用 `useAlertRules()` 替換 INITIAL_RULES → P1

---

## 30. EnergyMonitor (`/monitoring/energy`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Bar chart tabs | 334-346 | `setBarTab(...)` | OK | 無需修復 |

**關鍵修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| **metrics 表無 seed data** | **P0** | 執行 `inject-metrics.py --backfill 24h` 或在 seed.sql 加 metrics INSERT |

**Hardcoded：** PowerLoadView 100% static → P3：需 metrics 數據後逐步替換

---

## 31. AlertTopology (`/monitoring/topology`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Breadcrumb | 298 | `navigate('/monitoring')` | OK | 無需修復 |
| BIA filter | 319 | `setBiaFilter(...)` 未消費 | P1 | 在 NODES filter 中消費 |
| Domain filter | 332 | `setDomainFilter(...)` 未消費 | P1 | 在 NODES filter 中消費 |
| Alert list items | 373 | `setSelectedNodeId(...)` | OK | 無需修復 |
| Reset View | 342 | NONE | P2 | `setBiaFilter(''); setDomainFilter('')` |
| Export Report | 346 | NONE | P2 | 移除或加導出 |

**Hardcoded：** NODES/EDGES 100% mock → P3：需後端拓撲端點或從 assets 關係衍生

---

## 32. AuditHistory (`/audit`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Tab switches | 151-164 | `setActiveTab(...)` 但未 filter | P1 | 在查詢中根據 tab filter |
| Row click | 278 | `navigate('/audit/detail')` | P0 | **改為 `navigate(/audit/detail?id=${event.id})`** |
| Row expand | 319 | `setExpandedRow(...)` | OK | 加 `e.stopPropagation()` 防止同時觸發 navigate |
| Search | 221 | 未綁定 | P1 | 加 `onChange` + client-side filter |
| Event type filter | 229 | 未綁定 | P1 | 加 `onChange` + API query param |
| User filter | 236 | 未綁定 | P1 | 加 `onChange` + API query param |
| Advanced Filters | 252 | NONE | P2 | 標記 Coming Soon |
| Run Report | 374 | NONE | P2 | 移除或加導出 |

---

## 33. AuditEventDetail (`/audit/detail`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Back breadcrumb | 139 | `navigate('/audit')` | OK | 無需修復 |
| Export Log | 169 | NONE | P2 | 加 JSON 下載 |
| View Asset | 245 | `navigate('/assets/detail')` | P1 | 改為帶 asset ID |

**前置修復：** 依賴 #32 AuditHistory 傳 event ID（P0）

---

## 34. RolesPermissions (`/system`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Add New Role | 219 | `setShowRoleModal(true)` | OK | 無需修復 |
| Create Role submit | 401 | `createRole.mutate(...)` | OK | 無需修復 |
| Role selection | 232 | `setSelectedRole(role.id)` | OK | 無需修復 |
| Delete Role | 257 | `deleteRole.mutate(role.id)` | OK | 無需修復 |
| Permission toggles | 343-350 | `togglePerm(key, col)` | OK | 無需修復 |
| **Save Changes** | **307** | **NONE** | **P0** | **需加後端 `PUT /roles/{id}` 端點 + `useUpdateRole` hook → 把 permOverrides 轉成 permissions JSON 送 API** |
| Cancel | 303 | NONE | P2 | `setPermOverrides({})` reset |
| Emergency Stop | 194 | NONE | P2 | 標記 Coming Soon |
| Deploy Changes | 201 | NONE | P2 | 標記 Coming Soon |

**Bug 修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| is_system 未映射 → 系統角色可被刪除 | P1 | API Role type 加 `is_system` 欄位，或在前端 hardcode 系統角色 ID 列表 |
| User count 所有角色顯示相同 | P1 | 需後端 user-role join 查詢，或前端 `useUserRoles` 按角色統計 |

---

## 35. SystemSettings (`/system/settings`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Tab switches | 83-95 | `setActiveTab(...)` | OK | 無需修復 |
| New User | 73 | `setShowUserModal(true)` | OK | 無需修復 |
| Create User submit | modal | `createUser.mutate(...)` | OK | 無需修復 |
| Add Adapter | 105 | `setShowCreateAdapter(true)` | OK | 無需修復 |
| Add Webhook | 131 | `setShowCreateWebhook(true)` | OK | 無需修復 |
| Edit User | 211 | NONE | P2 | 加 modal → `useUpdateUser` |
| Delete User | 216 | NONE | P2 | 加 confirm → 需後端 `DELETE /users/{id}` |
| Regenerate QR | 277 | NONE | P2 | 標記 Coming Soon |

**Bug 修復：** role 顯示 bug → P1：`apiRoles.find(r => r.name)` 改為按 user_roles 查對應角色

**Hardcoded：** stats (1,284 users etc.) → P3：從 API 衍生

---

## 36. UserProfile (`/system/profile`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Display Name input | 78 | `setDisplayName(...)` | OK | 無需修復 |
| Update Profile | 108 | `updateUser.mutate(...)` | OK | 無需修復 |
| Employee ID | 89 | 有 state 但不送 API | P2 | 加到 mutation payload（需後端支持或用 attributes）|
| Department | 99 | 有 state 但不送 API | P2 | 同上 |
| Change Password | 123 | NONE | P2 | 加 modal（需後端密碼更新端點）|
| Reset 2FA | 143 | NONE | P2 | 標記 Coming Soon |
| Revoke Session | 164 | NONE | P2 | 標記 Coming Soon |
| Notification toggles | 194 | local state only | P3 | 需後端用戶偏好端點 |

---

## 37. PredictiveHub (`/predictive`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Tab switches (6 tabs) | 1341-1356 | `setActiveTab(...)` | OK | 無需修復 |
| Run RCA button | modal | `createRCA.mutate(...)` via CreateRCAModal | OK | 無需修復 |
| Verify button | ~550 | `verifyRCA.mutate(...)` | OK | 無需修復 |
| Pagination | 425-445 | `setCurrentPage(...)` 但未 slice | P2 | 加 client-side 分頁 |
| Alert tab filters | 513 | `setActiveFilter(...)` 未消費 | P1 | 在 alert data filter 中消費 |
| Isolate Node button | ~1206 | NONE | P2 | 標記 Coming Soon |
| Schedule Maintenance | 1284 | `navigate('/maintenance/add')` | OK | 無需修復 |

**關鍵修復：**

| 問題 | 級別 | 修復方案 |
|------|------|---------|
| **selectedAssetId 永為空 → predictions 不載入** | **P0** | 初始化為第一個 seed asset ID `f0000000-0000-0000-0000-000000000001`，或從 `useAssets` 取第一個資產 ID |

---

## 38. Help 頁面（3 頁）

### TroubleshootingGuide (`/help/troubleshooting`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Search | 270 | `setSearchQuery(...)` | OK | 無需修復 |
| Category filters | 289 | `setActiveCategory(...)` | OK | 無需修復 |
| Issue expand | 314 | `handleToggle(...)` | OK | 無需修復 |
| Submit New Issue | 256 | NONE | P2 | 加 `navigate('/maintenance/add')` 或標記 Coming Soon |
| Breadcrumb | 238 | 自我指向 | P2 | 改為指向 `/help` |

### VideoLibrary (`/help/videos`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Search | 252 | `setSearch(...)` | OK | 無需修復 |
| View toggles | 261/274 | `setViewMode(...)` | OK | 無需修復 |
| Category filters | 292+ | `setActiveCategory(...)` | OK | 無需修復 |
| Video card click | 110-187 | `navigate('/help/videos/player')` | P1 | 改為帶 video ID |

### VideoPlayer (`/help/videos/player`)

| 元素 | 行號 | 現狀 | 級別 | 修復方案 |
|------|------|------|------|---------|
| Back button | 30 | `navigate('/help/videos')` | OK | 無需修復 |
| Tab switches | 44 | `setActiveTab(...)` | OK | 無需修復 |
| Play button | 63 | NONE | ACCEPT | 無實際影片，可接受 |
| Download SOP | 111 | NONE | P2 | 移除或標記 Coming Soon |
| Chapter buttons | 130 | NONE | ACCEPT | 靜態頁面可接受 |

---

## 修復統計總覽

### 按級別統計

| 級別 | 數量 | 說明 |
|------|------|------|
| **P0-CRITICAL** | 9 | 路由/導航/數據斷裂 |
| **P1-HIGH** | 18 | 功能缺口/filter 不生效/導航缺 ID |
| **P2-MEDIUM** | 52 | 死按鈕 |
| **P3-LOW** | 25+ | hardcoded 數據 |
| **ACCEPT** | ~215 | 正常工作 |

### P0 完整清單

| # | 頁面 | 問題 | 修復動作 |
|---|------|------|---------|
| 1 | AssetLifecycleTimeline | 路由缺 `:assetId` | App.tsx 加路由參數 |
| 2 | PredictiveHub | `selectedAssetId` 永為空 | 初始化為第一個資產 ID |
| 3 | EnergyMonitor | metrics 表無 seed data | 執行 inject-metrics.py 或加 SQL |
| 4 | DataCenter3D | `navigate("/racks/detail")` 無 ID | 改為 `/racks/${rack.id}` |
| 5 | FacilityMap | `navigate("/racks/detail")` 無 ID | 改為 `/racks/${selectedRack.id}` |
| 6 | AuditHistory → Detail | 導航無 event ID | 加 `?id=${event.id}` |
| 7 | MaintenanceHub | 行導航無 work order ID | 改為 `/maintenance/task/${wo.id}` |
| 8 | RolesPermissions | Save Changes 不持久化 | 加 `PUT /roles/{id}` + mutation |
| 9 | MaintenanceHub | summaryCards `\|\|` 應為 `??` | 改 fallback 運算子 |

### P1 完整清單

| # | 頁面 | 問題 | 修復動作 |
|---|------|------|---------|
| 1 | GlobalOverview | MAP_META slug 不匹配 | 改 key 為 `tw` |
| 2 | CampusOverview | seed 無 IDC 層級 | 加 IDC 位置到 seed |
| 3 | AssetManagement | search 後端不支持 | 後端加 search 參數 |
| 4 | AssetManagement | location filter 欄位不存在 | 改用 location_id |
| 5 | SensorConfig | 不用 useAlertRules | 改用真實 hook |
| 6 | Dashboard/Alerts/Health | ci_id 顯示 UUID | 加 asset_tag lookup |
| 7 | SystemSettings | role 顯示 bug | 修 find 邏輯 |
| 8 | RolesPermissions | is_system 未映射 | 加欄位映射 |
| 9 | MonitoringAlerts | 分頁不生效 | 加 data slice |
| 10 | MaintenanceHub | 4 個 filter 不消費 | 加 filter 邏輯 |
| 11 | WorkOrder | filter tab 不消費 | 加 filter 邏輯 |
| 12 | AddMaintenanceTask | assignee 未送 API | 加到 payload |
| 13 | AddMaintenanceTask | priority 標籤 bug | 修正 label |
| 14 | RackDetail | asset 連結無 ID | 改為帶 ID |
| 15 | DataCenter3D | 位置樹 hardcoded | 改用 API |
| 16 | AddNewRack | row_label bug | 加獨立欄位 |
| 17 | AlertTopology | BIA/Domain filter 不消費 | 加 filter 邏輯 |
| 18 | VideoLibrary | video card 無 video ID | 帶 ID 導航 |
