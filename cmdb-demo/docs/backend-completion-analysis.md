# CMDB 平台後端補全分析報告

## 一、項目現狀概覽

| 維度 | 現狀 |
|------|------|
| **前端** | React 19 + TypeScript SPA，36 個頁面，59 個 API 端點定義 |
| **後端** | **尚未實現**，僅有前端 API Client 定義（指向 `http://10.134.143.218:8080/api/v1`） |
| **數據** | 全部使用頁面內 Mock 數據，僅 `useAssets` hook 有部分 API 整合嘗試 |
| **API 模組** | 9 個模組：assets, topology, maintenance, monitoring, inventory, audit, identity, prediction, integration |

---

## 二、跨頁面聯動邏輯全景圖

### 2.1 核心實體關係

```
                    ┌─────────────┐
                    │  Location   │
                    │ (層級樹狀)   │
                    └──────┬──────┘
                           │ 1:N
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌─────────┐  ┌─────────┐  ┌──────────┐
        │  Rack   │  │InventoryTask│  │ WorkOrder │
        └────┬────┘  └─────────┘  └──────────┘
             │ 1:N                      │
             ▼                          │
        ┌─────────┐                     │
        │  Asset  │◄────────────────────┘
        │ (CI核心) │     asset_id/ci_id
        └────┬────┘
             │ 1:N (被引用)
     ┌───────┼───────┬───────────┬──────────┐
     ▼       ▼       ▼           ▼          ▼
 AlertEvent  Audit  Prediction  Maintenance  Incident
             Event  Result      Task
```

### 2.2 頁面間聯動矩陣

| 源頁面 | 目標頁面 | 聯動方式 | 傳遞數據 | 後端需求 |
|--------|----------|----------|----------|----------|
| **Dashboard** | Assets/Racks/Monitoring/Inventory | 統計卡片點擊 | 無參數 | `/locations/{id}/stats` 聚合 API |
| **AssetDetail** | RackDetail | 機架定位點擊 | `rack_id` | `GET /racks/{id}` |
| **AssetDetail** | MaintenanceAdd | 建維護工單 | `asset_id, asset_name` | `POST /maintenance/orders` 需含 `asset_id` |
| **AssetDetail** | AuditHistory | 查操作歷史 | `target_id=asset_id` | `GET /audit/events?target_id={id}` |
| **RackDetail** | AssetDetail | 設備槽位點擊 | `asset_id` | `GET /racks/{id}/assets`（**缺少**） |
| **MonitoringAlerts** | AssetDetail | 告警行點擊 | `ci_id` (=asset serial) | 需 `GET /assets?serial_number={sn}` 或告警含 `asset_id` |
| **AlertTopology** | AssetDetail | 拓撲節點點擊 | `ci_id` | 需影響範圍 API（**缺少**） |
| **MaintenanceTask** | AssetDetail | 關聯設備點擊 | `asset_id` | WorkOrder 需含 `asset_ids[]` 欄位 |
| **WorkOrder** | AssetDetail | CI 名稱點擊 | `ci_name` → `asset_id` | 需 `ciName` 解析為 `asset_id` |
| **PredictiveHub** | MaintenanceAdd | AI 建議建工單 | `asset_id, prediction_id` | `POST /maintenance/orders` 含 `prediction_id` |
| **PredictiveHub** | Monitoring | 查看系統監控 | 無 | — |
| **EquipmentHealth** | MaintenanceAdd | 健康異常建工單 | `asset_id` | 同上 |
| **EquipmentHealth** | AssetDetail | 快速跳轉 | `asset_id` | — |
| **Inventory** | AssetDetail | 盤點項目點擊 | `asset_id` | `GET /inventory/tasks/{id}/items` 需含 asset 引用 |
| **AuditDetail** | AssetDetail | 目標資源點擊 | `target_id` | `GET /audit/events/{id}` 需含完整 target 信息 |
| **Location 層級** | Dashboard | 園區進入 | `idc_id` | Location 層級遍歷 API |
| **SystemHealth** | AssetDetail | 受影響設備 | `asset_id` | 需系統健康聚合 API（**缺少**） |

### 2.3 關鍵聯動鏈路（端到端）

**鏈路 1：告警 → 定位 → 維修**
```
MonitoringAlerts → (ci_id) → AssetDetail → (asset_id) → MaintenanceAdd → (POST) → MaintenanceHub
```
後端需要：告警帶 `asset_id`、資產詳情帶 `rack_id + location_id`、維護工單關聯 `asset_id`

**鏈路 2：AI 預測 → 預防性維護**
```
PredictiveHub → (prediction_result) → MaintenanceAdd → (POST with prediction_id) → TaskDispatch
```
後端需要：預測結果帶 `asset_id`、工單關聯 `prediction_id`

**鏈路 3：盤點 → 差異處理**
```
HighSpeedInventory → (rack scan) → InventoryDetail → (discrepancy) → AssetDetail
```
後端需要：盤點項含 `asset_id`、差異含 `expected vs actual` 比對

**鏈路 4：Location 下鑽 → 全局監控**
```
GlobalOverview → RegionOverview → CityOverview → CampusOverview → Dashboard (IDC filtered)
```
後端需要：每級 Location 的 `stats` 聚合（assets count, rack occupancy, PUE, critical alerts）

---

## 三、後端 API 缺口分析

### 3.1 已定義但需實現的 API（59 個端點）

#### Auth 模組（3 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/auth/login` | POST | 登入取得 Token | P0 |
| `/auth/refresh` | POST | Token 刷新 | P0 |
| `/auth/me` | GET | 當前用戶信息 | P0 |

#### Assets 模組（5 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/assets` | GET | 資產列表（分頁+篩選） | P0 |
| `/assets/{id}` | GET | 資產詳情 | P0 |
| `/assets` | POST | 新增資產 | P1 |
| `/assets/{id}` | PUT | 更新資產 | P1 |
| `/assets/{id}` | DELETE | 刪除資產 | P2 |

#### Topology 模組（14 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/locations` | GET | 根節點列表 | P0 |
| `/locations/{id}` | GET | Location 詳情 | P0 |
| `/locations/{id}/children` | GET | 子節點列表 | P0 |
| `/locations/{id}/descendants` | GET | 全部後代節點 | P1 |
| `/locations/{id}/ancestors` | GET | 祖先鏈路 | P0 |
| `/locations/{id}/stats` | GET | 聚合統計 | P0 |
| `/locations` | POST | 新增 Location | P2 |
| `/locations/{id}` | PUT | 更新 Location | P2 |
| `/locations/{id}` | DELETE | 刪除 Location | P2 |
| `/locations/{id}/racks` | GET | Location 下機架 | P0 |
| `/racks` | POST | 新增機架 | P1 |
| `/racks/{id}` | GET | 機架詳情 | P0 |
| `/racks/{id}` | PUT | 更新機架 | P1 |
| `/racks/{id}` | DELETE | 刪除機架 | P2 |

#### Maintenance 模組（6 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/maintenance/orders` | GET | 工單列表 | P0 |
| `/maintenance/orders/{id}` | GET | 工單詳情 | P0 |
| `/maintenance/orders` | POST | 建立工單 | P0 |
| `/maintenance/orders/{id}` | PUT | 更新工單 | P1 |
| `/maintenance/orders/{id}/transition` | POST | 狀態流轉 | P0 |
| `/maintenance/orders/{id}/logs` | GET | 操作日誌 | P1 |

#### Monitoring 模組（11 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/monitoring/metrics` | POST | 指標寫入 | P1 |
| `/monitoring/metrics` | GET | 指標查詢 | P0 |
| `/monitoring/rules` | GET | 告警規則列表 | P1 |
| `/monitoring/rules` | POST | 建立告警規則 | P2 |
| `/monitoring/alerts` | GET | 告警事件列表 | P0 |
| `/monitoring/alerts/{id}/ack` | POST | 確認告警 | P0 |
| `/monitoring/alerts/{id}/resolve` | POST | 解決告警 | P0 |
| `/monitoring/incidents` | GET | 事件列表 | P1 |
| `/monitoring/incidents` | POST | 建立事件 | P1 |
| `/monitoring/incidents/{id}` | GET | 事件詳情 | P1 |
| `/monitoring/incidents/{id}` | PUT | 更新事件 | P1 |

#### Inventory 模組（7 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/inventory/tasks` | GET | 盤點任務列表 | P0 |
| `/inventory/tasks/{id}` | GET | 任務詳情 | P0 |
| `/inventory/tasks` | POST | 建立盤點任務 | P1 |
| `/inventory/tasks/{id}/complete` | POST | 完成任務 | P1 |
| `/inventory/tasks/{id}/items` | GET | 盤點項列表 | P0 |
| `/inventory/tasks/{id}/items/{itemId}/scan` | POST | 掃描確認 | P1 |
| `/inventory/tasks/{id}/summary` | GET | 盤點摘要 | P1 |

#### Audit 模組（1 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/audit/events` | GET | 審計事件查詢 | P0 |

#### Identity 模組（7 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/users` | GET | 用戶列表 | P1 |
| `/users/{id}` | GET | 用戶詳情 | P1 |
| `/users` | POST | 建立用戶 | P2 |
| `/users/{id}` | PUT | 更新用戶 | P2 |
| `/roles` | GET | 角色列表 | P1 |
| `/roles` | POST | 建立角色 | P2 |
| `/roles/{id}` | DELETE | 刪除角色 | P2 |

#### Prediction 模組（5 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/prediction/models` | GET | 模型列表 | P2 |
| `/prediction/results/ci/{ciId}` | GET | CI 預測結果 | P1 |
| `/prediction/results` | POST | 建立預測 | P2 |
| `/prediction/rca` | POST | 根因分析 | P2 |
| `/prediction/rca/{id}/verify` | POST | 人工驗證 RCA | P2 |

#### Integration 模組（5 個）

| 端點 | 方法 | 用途 | 優先級 |
|------|------|------|--------|
| `/integration/adapters` | GET | 適配器列表 | P2 |
| `/integration/adapters` | POST | 建立適配器 | P2 |
| `/integration/webhooks` | GET | Webhook 列表 | P2 |
| `/integration/webhooks` | POST | 建立 Webhook | P2 |
| `/integration/webhooks/{id}/deliveries` | GET | 投遞記錄 | P2 |

### 3.2 前端需要但 API 定義中缺少的端點

| 缺失端點 | 使用場景 | 建議路徑 | 優先級 |
|----------|----------|----------|--------|
| 機架下的資產列表 | RackDetail 顯示槽位設備 | `GET /racks/{id}/assets` | **P0** |
| 資產按序列號查詢 | 告警頁面跳轉資產詳情 | `GET /assets?serial_number={sn}` | **P0** |
| Dashboard 聚合統計 | Dashboard 總覽數據 | `GET /dashboard/stats?idc_id={id}` | **P0** |
| 資產維護歷史 | AssetDetail 維護 Tab | `GET /assets/{id}/maintenance-history` | **P1** |
| 資產告警歷史 | AssetDetail 告警 Tab | `GET /assets/{id}/alerts` | **P1** |
| 資產審計歷史 | AssetDetail 審計 Tab | `GET /audit/events?target_id={id}&target_type=asset` | **P1** |
| 資產預測結果 | AssetDetail 預測 Tab | 已有 `GET /prediction/results/ci/{ciId}` | — |
| 設備健康概覽 | EquipmentHealth 頁面 | `GET /assets/health-overview` | **P1** |
| 系統健康聚合 | SystemHealth 頁面 | `GET /monitoring/health-summary` | **P1** |
| 能耗監控數據 | EnergyMonitor 頁面 | `GET /monitoring/metrics?metric_name=power_*` | **P1** |
| 告警拓撲圖數據 | AlertTopology 頁面 | `GET /monitoring/alerts/{id}/topology` | **P1** |
| 資產生命週期事件 | AssetLifecycleTimeline | `GET /assets/{id}/lifecycle-events` | **P1** |
| 自動發現掃描 | AutoDiscovery 頁面 | `POST /assets/discovery/scan` | **P2** |
| 元件升級建議 | ComponentUpgrades 頁面 | `GET /assets/upgrade-recommendations` | **P2** |
| Sensor 配置列表 | SensorConfig 頁面 | `GET /monitoring/sensors` | **P2** |
| 維護任務指派 | TaskDispatch 頁面 | `PUT /maintenance/orders/{id}/assign` | **P1** |

---

## 四、數據模型補全建議

### 4.1 現有模型欄位對齊問題

| 模型 | 前端 Mock 欄位 | API 定義欄位 | 差異 |
|------|----------------|-------------|------|
| **Asset** | `location`, `lastMaintenance`, `metrics[]`, `healthScore` | 無這些欄位 | 需增加計算/關聯欄位 |
| **WorkOrder** | `ciName`, `requestor`, `reason` | 無 `ciName`，有 `assignee_id` | 需加 `asset_id`, `requestor_id`, `reason` |
| **AlertEvent** | `serialNumber`, `description` | 有 `ci_id`, `message` | `serialNumber` 需通過 Asset 關聯解析 |
| **InventoryTask** | `racks[]`, `discrepancies[]` | 無嵌套數據 | 需加 `items` 和 `summary` 子資源 |
| **AuditEvent** | `operator`(name), `role`, `description`, `source` | `operator_id`(ID), 無 role/source | 需加 operator 展開或獨立查詢 |

### 4.2 建議新增的關聯模型

```typescript
// 機架-資產關聯（槽位管理）
RackSlot {
  rack_id: string
  asset_id: string
  start_u: number
  end_u: number
  side: "front" | "back"
}

// 資產生命週期事件
AssetLifecycleEvent {
  id: string
  asset_id: string
  event_type: "procured" | "deployed" | "maintained" | "upgraded" | "decommissioned"
  occurred_at: string
  description: string
  operator_id: string
}

// 盤點差異記錄
InventoryDiscrepancy {
  id: string
  task_id: string
  asset_id: string | null
  location: string
  issue_type: "missing" | "unexpected" | "mismatch"
  expected: Record<string, any>
  actual: Record<string, any>
  resolved: boolean
}

// Dashboard 聚合快照
DashboardStats {
  total_assets: number
  total_racks: number
  critical_alerts: number
  avg_pue: number
  avg_occupancy: number
  bia_distribution: { critical: number, important: number, normal: number, minor: number }
  recent_events: AlertEvent[]
}
```

---

## 五、實施優先級建議

### Phase 1 — 核心骨架（P0，解鎖基本瀏覽）

1. **Auth**：login / refresh / me（3 個端點）
2. **Topology**：Location CRUD + 層級遍歷 + stats（7 個端點）
3. **Assets**：list + getById + 按序列號查詢（3 個端點）
4. **Racks**：getById + listByLocation + **listAssets**（3 個端點）
5. **Dashboard**：聚合統計 API（1 個端點）
6. **Monitoring**：listAlerts + ack + resolve（3 個端點）
7. **Maintenance**：list + getById + create + transition（4 個端點）
8. **Audit**：query（1 個端點）
9. **Inventory**：list + getById + listItems（3 個端點）

**合計：28 個端點，覆蓋所有頁面的基本數據展示和核心聯動鏈路。**

### Phase 2 — 功能完善（P1，解鎖交互操作）

- Assets CRUD 完整、lifecycle events、health overview
- Maintenance assign、logs
- Monitoring metrics query、health summary、topology
- Inventory scan/complete/summary
- Identity users/roles 列表
- Prediction results 查詢

### Phase 3 — 高級功能（P2，解鎖 AI 和整合）

- Prediction models、RCA
- Integration adapters、webhooks
- Auto discovery、sensor config
- Asset creation/deletion
- Role/User management 完整 CRUD

---

## 六、關鍵技術要點

### 6.1 API 回應格式（已統一）

```json
// 單一資源
{ "data": {...}, "meta": { "request_id": "uuid" } }

// 列表資源
{
  "data": [...],
  "pagination": { "page": 1, "page_size": 20, "total": 100, "total_pages": 5 },
  "meta": { "request_id": "uuid" }
}

// 錯誤
{ "error": { "code": "NOT_FOUND", "message": "..." }, "meta": { "request_id": "uuid" } }
```

### 6.2 認證機制

- Bearer Token（JWT）
- 401 + `INVALID_TOKEN` 觸發自動刷新
- 刷新失敗自動登出

### 6.3 核心聯動約束

- **Asset 是全系統的核心 CI**：`asset_id` 在 monitoring(`ci_id`)、maintenance、audit(`target_id`)、inventory、prediction(`ci_id`) 中被廣泛引用
- **Location 是空間維度的核心**：Dashboard、Inventory、Racks 都通過 `location_id` 過濾
- **告警 → 資產解析**：前端使用 `serialNumber` 顯示，需要後端在 Alert 中嵌入 `asset_id` 或提供反查接口
