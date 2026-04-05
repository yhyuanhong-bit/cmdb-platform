# CMDB Platform — 頁面級功能測試報告

> 測試日期：2026-04-04
> 測試方法：5 個並行 Agent 分模組追蹤完整鏈路（頁面 → Hook → API Client → 後端 Handler → Service → sqlc Query → Seed Data）
> 涵蓋範圍：38 個頁面，67 個後端端點，全部前端 Hook 和 API Client

---

## 一、總覽評分

| 評級 | 頁面數 | 佔比 | 說明 |
|------|--------|------|------|
| A (全功能) | 2 | 5% | Login, TroubleshootingGuide |
| B (核心可用) | 12 | 32% | 核心 CRUD 正常，部分次要功能缺失 |
| C (部分可用) | 17 | 45% | 主數據從 API 載入，但大量 hardcoded + 死按鈕 |
| D (基本不可用) | 2 | 5% | SensorConfig, EnergyMonitor — API 接了但數據為空 |
| MOCK (純靜態) | 5 | 13% | Welcome, VideoLibrary, VideoPlayer, EquipmentHealth, ComponentUpgrade |

**整體完整度：~55%**（API 鏈路完整的功能佔比）

---

## 二、逐頁測試結果

### Location / Topology（9 頁）

| 頁面 | 評級 | 數據查詢 | 寫入操作 | 關鍵問題 |
|------|------|---------|---------|---------|
| GlobalOverview | C+ | 3 API 正常 | CreateLocation modal 可用 | MAP_META slug 不匹配 seed（china/japan vs tw）；SUMMARY_KPI hardcoded |
| RegionOverview | B+ | 3 API 正常 | 無 | metadata 依賴可能為空 |
| CityOverview | B+ | 4 API 正常 | 無 | sparkline 欄位 seed 無值 |
| CampusOverview | C+ | 5 API + descendants 正常 | 無 | seed 無 IDC 層級位置→campus 子項永遠為空；缺 used/available/reserved 欄位 |
| RackManagement | C+ | API 正常 | 導航到 AddNewRack | 表格行操作按鈕(view/edit/more)無 handler；sidebar 事件 hardcoded |
| RackDetail | B- | useRack + useRackAssets 正常 | Edit/Delete 已接通 | alerts/network/maintenance/environment 全 hardcoded；asset 連結指向 `/assets/detail`（無 ID） |
| DataCenter3D | C | useRacks 正常 | 無 | 位置樹 sidebar 顯示 Germany/Frankfurt 而非 Taiwan；導航到 `/racks/detail` 無 ID |
| FacilityMap | C | useRacks 正常 | 無 | 溫度假算法；Maintenance 按鈕無 handler；導航無 rack ID |
| AddNewRack | B- | 無需查詢 | createRack mutation 可用 | 位置下拉 hardcoded；row_label 填成完整 rack ID |

### Asset（7 頁）

| 頁面 | 評級 | 數據查詢 | 寫入操作 | 關鍵問題 |
|------|------|---------|---------|---------|
| AssetManagement | B- | useAssets 分頁正常 | CreateAsset modal 可用 | search 送到後端但後端不支持；location filter 欄位不存在；Import/Export 按鈕死 |
| AssetDetail | B- | useAsset + attributes 正常 | Edit/Delete 已接通 | Health/Usage/Maintenance 三個 tab 100% mock；financial 欄位全 dash |
| AssetLifecycle | C+ | useAssets 正常 | 無 | 財務摘要 hardcoded ($48.2M)；Filter/Export 按鈕死 |
| AssetLifecycleTimeline | D+ | **路由缺 :assetId 參數→API 永不觸發** | 無 | 關鍵 bug：整頁永遠顯示 fallback 數據 |
| AutoDiscovery | D+ | useAssets（語意錯誤）| 4 個 Approve/Ignore 按鈕全死 | 無 discovery API；discovery 欄位 seed 無值；分頁 fake |
| EquipmentHealth | MOCK | useAssets（結果未使用） | 無 | 整頁 100% hardcoded；Download/Alert 按鈕死 |
| ComponentUpgrade | MOCK | useAssets（結果未使用） | 無 | 整頁 100% hardcoded；Request Upgrade 按鈕死 |

### Maintenance + Inventory（7 頁）

| 頁面 | 評級 | 數據查詢 | 寫入操作 | 關鍵問題 |
|------|------|---------|---------|---------|
| MaintenanceHub | C+ | useWorkOrders 正常 | 導航到 create/dispatch | 週曆 hardcoded；4 個 filter 全不生效；分頁 fake；行點擊導航無 ID |
| WorkOrder | B- | useWorkOrders 正常 | Transition 已接通 | 統計卡 hardcoded(12/28/05/19)；filter tab 不生效；分頁無功能 |
| AddMaintenanceTask | B | 無需查詢 | createWorkOrder 正常 | assignee 未送到 API；priority 標籤 bug（兩個 Medium）|
| MaintenanceTaskView | B- | useWorkOrder + useWorkOrderLogs | Complete transition 正常 | taskSteps/associatedAssets/envMetrics hardcoded；Edit/Post Comment 按鈕死 |
| TaskDispatch | C- | useWorkOrders 正常 | **所有操作按鈕死** | Assign/Auto Assign/Confirm/Add Task 全無 handler；technician/zone 全 hardcoded |
| HighSpeedInventory | B- | useInventoryTasks 正常 | Complete + CreateTask modal 可用 | 掃描網格/差異表/導入進度全 hardcoded；Scan QR/Report 按鈕死；導航無 taskId |
| InventoryItemDetail | C+ | useInventoryTask + Items 正常 | Verify/Flag/Resolve 已接通 | scan history/discrepancy notes hardcoded；note 輸入框是 div 非 input；Escalate/Print 死 |

### Monitoring + Dashboard（6 頁）

| 頁面 | 評級 | 數據查詢 | 寫入操作 | 關鍵問題 |
|------|------|---------|---------|---------|
| Dashboard | B- | 3 API 正常(stats/alerts/assets) | 無 | occupancy 76% hardcoded；heatmap/lifecycle/task 進度 hardcoded；ci_id 顯示 UUID |
| MonitoringAlerts | B- | useAlerts + filter 正常 | Ack/Resolve 已接通 | 分頁渲染但不生效；趨勢圖 hardcoded；日期/位置 filter 死；Silence/Export 按鈕死 |
| SystemHealth | C+ | useAlerts + useSystemHealth | 無 | health donut/trend/resource/uptime/sync 全 hardcoded；BIA 欄顯示 dash |
| SensorConfiguration | D+ | useAssets（語意錯誤） | **Save 不持久化** | 已有 useAlertRules hook 卻不用；所有配置變更只存 local state |
| EnergyMonitor | D+ | useMetrics 正確接通 | 無 | **metrics 表無 seed data→API 永遠返回空**；Power Load tab 100% hardcoded |
| AlertTopology | C- | useAlerts 正常 | 無 | 拓撲節點/邊 100% hardcoded；BIA/Domain filter 不生效；所有 alert 映射到 node-1 |

### Audit + Identity + Prediction + Help（11 頁）

| 頁面 | 評級 | 數據查詢 | 寫入操作 | 關鍵問題 |
|------|------|---------|---------|---------|
| AuditHistory | C+ | useAuditEvents 正常 | 無 | search/eventType/user filter 全裝飾品；導航到 detail 無 event ID |
| AuditEventDetail | C+ | useAuditEvents(target_id) | 無 | 因無 ID 參數永遠顯示 fallback；diffMode 無法切換 |
| RolesPermissions | B- | useUsers + useRoles 正常 | Create/Delete Role 可用 | **Save Changes 不持久化**；is_system 屬性未映射→系統角色可被刪除 |
| SystemSettings | B- | 5 API 全正常 | CreateUser/Adapter/Webhook 可用 | Edit/Delete User 死；role 顯示 bug（全部顯示同一角色）；stats hardcoded |
| UserProfile | C+ | authStore 正常 | UpdateUser(display_name) 可用 | employeeId/department 不持久化；password/2FA/revoke 全死；sessions hardcoded |
| PredictiveHub | B- | Models 正常 | CreateRCA modal + Verify 可用 | **selectedAssetId 永為空→predictions API 永不觸發**；大量 tab hardcoded |
| TroubleshootingGuide | B | 純靜態（可接受） | 無 | Submit Issue 按鈕死；breadcrumb 自我指向 |
| VideoLibrary | B | 純靜態（可接受） | 無 | 導航到 player 無 video ID |
| VideoPlayer | C+ | 純靜態 | 無 | 章節/播放/下載全不可用 |
| Login | A- | auth API 完整 | 登入正常 | 無 |
| Welcome | C+ | 無 | 導航正常 | onboarding tab 無法切換；配色與全站不一致 |

---

## 三、關鍵問題分類

### CRITICAL — 阻塞核心功能（7 項）

| # | 問題 | 影響頁面 | 說明 |
|---|------|---------|------|
| 1 | AssetLifecycleTimeline 路由缺 `:assetId` | AssetLifecycleTimeline | API 永不觸發，整頁 fallback |
| 2 | PredictiveHub `selectedAssetId` 永為空 | PredictiveHub | predictions API 永不觸發 |
| 3 | metrics 表無 seed data | EnergyMonitor | 所有 metric 查詢返回空 |
| 4 | 導航到 `/racks/detail` 無 ID | DataCenter3D, FacilityMap | rack detail 頁無法載入 |
| 5 | AuditHistory → AuditEventDetail 無 event ID | AuditHistory | detail 永遠顯示 fallback |
| 6 | MaintenanceHub 行導航無 work order ID | MaintenanceHub | TaskView 永遠顯示 fallback |
| 7 | RolesPermissions Save 不持久化 | RolesPermissions | 權限矩陣修改無法保存 |

### HIGH — 顯著功能缺口（12 項）

| # | 問題 | 影響 |
|---|------|------|
| 8 | GlobalOverview MAP_META slug 不匹配 | 地圖標記位置錯誤 |
| 9 | CampusOverview seed 無 IDC 層級位置 | campus 子項永遠空 |
| 10 | AssetManagement search 後端不支持 | 搜索無效 |
| 11 | AssetManagement location filter 欄位不存在 | 篩選無效 |
| 12 | SensorConfig 不用已有的 useAlertRules hook | 配置與後端脫節 |
| 13 | ci_id 顯示 UUID 而非 asset_tag | Dashboard/Alerts/Health 4 頁 |
| 14 | SystemSettings role 顯示 bug | 所有用戶顯示同一角色 |
| 15 | RolesPermissions is_system 未映射 | 系統角色可被誤刪 |
| 16 | TaskDispatch 所有操作按鈕死 | 派工功能完全不可用 |
| 17 | AutoDiscovery 4 個操作按鈕全死 | 自動發現功能不可用 |
| 18 | AlertTopology BIA/Domain filter 不生效 | 篩選裝飾品 |
| 19 | MonitoringAlerts 分頁渲染但不生效 | 翻頁無效 |

### MEDIUM — 死按鈕統計（~45 個）

| 模組 | 死按鈕數 | 典型例子 |
|------|---------|---------|
| Asset 模組 | 12 | Import, Export, row actions, Filter Logs, Schedule Entry |
| Maintenance 模組 | 8 | Edit Task, Post Comment, Assign, Auto Assign |
| Monitoring 模組 | 8 | Export Report, Silence, Save Configuration, Discover Sensors |
| Audit 模組 | 4 | Advanced Filters, Run Report, Export Log |
| Prediction 模組 | 3 | Isolate Node, Filter, Export |
| Identity 模組 | 5 | Edit User, Delete User, Deploy Changes, Emergency Stop |
| Help 模組 | 3 | Submit Issue, Download SOP, Play |
| Other | 2 | Welcome Help/Settings |
| **合計** | **~45** | |

### LOW — Hardcoded 數據殘留

| 類型 | 數量 | 影響 |
|------|------|------|
| 統計卡片 hardcoded | 15+ | Dashboard/WorkOrder/MaintenanceHub/SystemHealth 等 |
| 圖表/趨勢 hardcoded | 10+ | heatmap/sparkline/trend chart/donut |
| 環境數據 hardcoded | 5+ | 溫度/濕度/功率/PUE |
| 時間線/歷史 hardcoded | 5+ | MaintenanceTaskView steps/AssetDetail health |
| 人員/團隊 hardcoded | 3 | TaskDispatch technicians/AddMaintenance assignees |

---

## 四、端到端鏈路驗證摘要

### 查詢鏈路（Read）

| 狀態 | 數量 | 說明 |
|------|------|------|
| 完整可用 | 35 | 頁面 → Hook → API → Backend → Seed Data 全通 |
| 接通但無數據 | 3 | metrics 表空、discovery 欄位空、prediction 因 bug 不觸發 |
| 語意錯誤 | 2 | SensorConfig/AutoDiscovery 用 assets API 冒充其他實體 |

### 寫入鏈路（Write）

| 狀態 | 數量 | 操作 |
|------|------|------|
| 完整接通 | 23 | Login, CRUD Asset, CRUD Location, CRUD Rack, Create/Transition/Update WorkOrder, Ack/Resolve Alert, Create AlertRule, CRUD Incident, Create/Complete InventoryTask, Scan Item, CRUD User, CRUD Role, Create Adapter, Create Webhook, Create/Verify RCA |
| Hook 存在但未接 UI | 0 | 本輪全部已接 |
| UI 按鈕存在但無 handler | ~45 | 見死按鈕統計 |

---

## 五、模組完整度排名

| 排名 | 模組 | 完整度 | 說明 |
|------|------|--------|------|
| 1 | Auth / Login | 95% | 完整登入/刷新/登出流程 |
| 2 | Asset CRUD | 75% | list/create/detail/edit/delete 全通，但 detail 子 tab 全 mock |
| 3 | Maintenance CRUD | 70% | create/list/transition/complete 全通，但 TaskDispatch 不可用 |
| 4 | Monitoring Alerts | 65% | list/ack/resolve 通，但分頁/filter/趨勢 mock |
| 5 | Topology | 60% | CRUD 全通，但 CampusOverview 數據空、DataCenter3D 導航 bug |
| 6 | Identity | 55% | CRUD 通，但 permissions save 不持久化、role 顯示 bug |
| 7 | Inventory | 50% | create/complete/scan 通，但 UI 大量 hardcoded |
| 8 | Integration | 50% | create adapter/webhook/deliveries 通，但無刪除/更新 |
| 9 | Prediction | 40% | RCA create/verify 通，但 predictions 因 bug 不載入 |
| 10 | Audit | 35% | 查詢通但 filter 全死、detail 無 ID |
| 11 | Dashboard | 35% | 4 統計真實，其餘 hardcoded |
| 12 | Energy/Sensor | 15% | 架構接通但無數據、無持久化 |

---

## 六、修復優先級建議

### P0 — 立即修復（影響核心業務流程）

1. **修復路由**：AssetLifecycleTimeline 加 `:assetId` 參數
2. **修復 bug**：PredictiveHub `selectedAssetId` 接到資產選擇器
3. **修復導航**：所有 `/racks/detail` → `/racks/${rackId}`；所有 `/maintenance/task` → `/maintenance/task/${orderId}`；AuditHistory → AuditEventDetail 帶 event ID
4. **填充數據**：metrics 表加 seed data（inject-metrics.py 可能已存在）

### P1 — 短期修復（提升可用性）

5. 修復 RolesPermissions is_system 映射 + Save 持久化（需加 PUT /roles/{id} 端點）
6. 修復 SystemSettings role 顯示 bug（用 user_roles join 查詢）
7. 修復 MonitoringAlerts 分頁（送 page 參數到 API 或 client-side slice）
8. SensorConfig 改用 useAlertRules hook
9. ci_id 改顯示 asset_tag（後端 join 或前端 lookup）

### P2 — 中期改善（消除死按鈕）

10. 移除或接通 45 個死按鈕（優先 TaskDispatch Assign、MaintenanceHub filters）
11. AssetDetail Health/Usage/Maintenance tab 接真實 API
12. 替換統計卡 hardcoded 值為 API 衍生值

### P3 — 長期完善

13. 加 discovery API（或移除 AutoDiscovery 頁面）
14. 加 sensor/equipment health API（或移除相關頁面）
15. 統一 i18n 覆蓋率

---

## 七、結論

**項目狀態：開發中後期，核心 CRUD 可用**

- 67 個後端端點全部實現，無 stub
- 23 個寫入操作前後端完整接通
- 35 個讀取查詢鏈路完整可用
- 38 頁中 14 頁評級 B 以上
- 7 個 CRITICAL bug 需立即修復
- ~45 個死按鈕需處理
- 整體功能完整度約 **55%**

**預估修復工時**：
- P0（7 項）：1-2 天
- P1（5 項）：2-3 天
- P2（死按鈕 + hardcoded）：1-2 週
- P3（新 API）：2-4 週
