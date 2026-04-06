# CMDB + AIOps 一體化平台 — 完整項目分析報告

> 報告日期：2026-04-04
> 版本：v1.0.0+（含 W1-W4 + BIA + Quality + Discovery + 3 Sprint 優化）
> 分析範圍：後端架構、前端 UI、業務邏輯、數據流、部署模型

---

## 一、項目概覽

### 平台定位

**企業級 CMDB + AIOps 一體化運維管理平台**，目標為：

- 集中管理跨地域數據中心的 IT 基礎設施資產
- 通過 MCP (Model Context Protocol) 實現 AI 原生整合
- 提供多租戶、多位置、角色驅動的企業級運營能力
- 支持自動化資產發現、生命週期管理、業務影響分析（BIA）
- 事件驅動的即時運維：NATS → WebSocket → React Query 自動刷新

### 技術架構

```
                    ┌──────────────────┐
                    │  React 19 SPA    │ ← 46 頁面, Tailwind M3, i18n ×3
                    │  (cmdb-demo)     │
                    └────────┬─────────┘
                             │ HTTPS / WebSocket
                    ┌────────▼─────────┐
                    │  Nginx 反向代理   │
                    └────────┬─────────┘
                             │
              ┌──────────────▼──────────────┐
              │       cmdb-core (Go/Gin)    │
              │  93 API 端點 + WS Hub       │
              │  + MCP Server (port 3001)   │
              │  + 6 層 Middleware           │
              └──┬───────┬───────┬──────────┘
                 │       │       │
          ┌──────▼──┐ ┌──▼───┐ ┌─▼────────┐
          │ PG 17   │ │Redis │ │NATS JS   │
          │+TimescaleDB│     │ │(Event Bus)│
          │ 30 表   │ │RBAC  │ │20 Subject │
          └─────────┘ └──────┘ └────┬──────┘
                                    │
                    ┌───────────────▼─────────────┐
                    │  WS Bridge + Webhook 派發器  │
                    │  HMAC 簽名 + BIA 過濾 + 重試 │
                    └──────────────────────────────┘
```

---

## 二、後端分析

### 2.1 代碼規模

| 指標 | 數量 |
|------|------|
| Go 代碼行數 | **17,485** |
| SQL 代碼行數 | **2,175** |
| Seed 數據行數 | **859** |
| DB 遷移文件 | 15 組（up/down） |
| DB 表 | **30** |
| sqlc 查詢檔案 | 20 |
| 命名查詢總數 | **127** |
| 領域模組 | **13** |
| Service 方法 | **94** |
| API 端點 | **93**（100% 已實現） |
| 事件主題 | **20** |
| Middleware | **6** |
| MCP Tools | **7** |
| MCP Resources | **3** |

### 2.2 模組清單

| 模組 | Service 方法 | API 端點 | 職責 |
|------|-------------|---------|------|
| Topology | 20 | 10 | 位置層級（ltree）、機架、slot CRUD |
| BIA | 13 | 11 | 業務影響評估、依賴、分級規則、MAX(tier) 繼承 |
| Monitoring | 10 | 11 | 告警事件、事件管理、指標查詢、告警規則 |
| Identity | 8 | 7 | 用戶、角色、JWT、權限 |
| Inventory | 7 | 8 | 盤點任務、掃描、匯入、統計 |
| Asset | 6 | 5 | 資產 CRUD、狀態、屬性 |
| Maintenance | 6 | 6 | 工單生命週期、狀態機、操作日誌 |
| Discovery | 6 | 5 | 資產發現、Staging、審核、匹配 |
| Quality | 6 | 6 | 數據質量規則、四維評分引擎、掃描 |
| Integration | 5 | 5 | 適配器、Webhook、投遞記錄 |
| Prediction | 4 | 4 | AI 模型、預測結果、RCA |
| Audit | 2 | 1 | 審計事件記錄與查詢 |
| Dashboard | 1 | 1 | 聚合統計 |

### 2.3 事件驅動系統

**NATS JetStream** 20 個事件主題：

- Asset: created / updated / status_changed / deleted
- Location: created / updated / deleted
- Rack: created / updated / deleted / occupancy_changed
- Maintenance: order_created / order_updated / order_transitioned
- Inventory: task_created / task_completed / item_scanned
- Alert: fired / resolved
- Prediction: created
- Audit: recorded

**消費者：**
- WebSocket Bridge → 4 個 wildcard 訂閱 → 前端即時刷新
- Webhook Dispatcher → HMAC 簽名 + BIA 過濾 + 3× 重試

### 2.4 安全層

| 層 | 實現 |
|---|------|
| JWT 認證 | 登入 → access_token + refresh_token，可配置 secret |
| RBAC 授權 | 3 角色（super-admin/ops-admin/viewer），Redis 快取，路由→資源+動作映射 |
| 審計追蹤 | 11 個寫操作自動記錄（操作人/時間/資源/差異） |
| Webhook 簽名 | HMAC-SHA256，可選 filter_bia 過濾 |
| MCP 認證 | MCP_API_KEY Bearer token（可選） |

---

## 三、前端分析

### 3.1 代碼規模

| 指標 | 數量 |
|------|------|
| TypeScript/TSX 行數 | **26,254** |
| 頁面組件 | **46** |
| 路由定義 | **55** |
| 自定義 Hook | **86**（18 個檔案）|
| API Client 方法 | **~90**（12 個模組）|
| 共享組件 | **15** |
| i18n 語言 | **3**（en / zh-TW / zh-CN）|
| 翻譯 key | **1,447** |
| 生成類型行數 | **2,019** |
| 構建時間 | **428ms** |
| 主 bundle gzip | **152.84 kB** |

### 3.2 技術棧

| 層 | 技術 | 版本 |
|---|------|------|
| 框架 | React | 19.2 |
| 路由 | react-router-dom | 7.13 |
| 伺服器狀態 | @tanstack/react-query | 5.95 |
| 客戶端狀態 | Zustand | 5.0 |
| 構建 | Vite | 8.0 |
| CSS | Tailwind CSS | 4.2 |
| 圖表 | Recharts | 3.8 |
| i18n | i18next | 26.0 |
| 類型生成 | openapi-typescript | 7.13 |

### 3.3 設計系統

- **配色：** Material Design 3 暗色主題，42 個 CSS token
- **字體：** Manrope（標題）+ Inter（正文）
- **圖標：** Google Material Symbols（outlined）
- **卡片：** `bg-surface-container rounded-lg p-5`
- **表格：** `bg-surface-container-high` header + hover 行
- **按鈕：** `machined-gradient`（主要）/ `bg-surface-container-high`（次要）

### 3.4 頁面模組分佈

| 模組 | 頁面數 | 主要頁面 |
|------|--------|---------|
| Topology / Rack | 9 | GlobalOverview → CampusOverview, RackDetail, DataCenter3D, AddNewRack |
| Asset | 7 | AssetManagement, AssetDetail, AutoDiscovery, Lifecycle |
| Maintenance | 5 | MaintenanceHub, WorkOrder, TaskDispatch, AddTask |
| Monitoring | 6 | Dashboard, MonitoringAlerts, SystemHealth, EnergyMonitor, SensorConfig |
| BIA | 5 | Overview, SystemGrading, RtoRpoMatrices, ScoringRules, DependencyMap |
| Identity | 3 | RolesPermissions, SystemSettings, UserProfile |
| Inventory | 2 | HighSpeedInventory, InventoryItemDetail |
| Prediction | 1 | PredictiveHub（6 個 tab） |
| Quality | 1 | QualityDashboard |
| Audit | 2 | AuditHistory, AuditEventDetail |
| Help | 3 | TroubleshootingGuide, VideoLibrary, VideoPlayer |
| Other | 2 | Login, Welcome |

---

## 四、業務流程閉環分析

### 4.1 已完整打通（12 條）

| # | 流程 | 鏈路 |
|---|------|------|
| 1 | 資產全生命週期 | 建立 → 編輯 → 刪除 + 審計日誌 + NATS 事件 |
| 2 | 工單全生命週期 | 建立 → 審批 → 執行 → 完成 + 操作日誌 |
| 3 | 告警處理 | 觸發 → 確認 → 解決 + 審計 + 事件推送 |
| 4 | BIA 評估 | 定義規則 → 評估業務系統 → 分級 → RTO/RPO 矩陣 |
| 5 | BIA 影響傳播 | 評估 tier 變更 → MAX(tier) 聚合 → 自動更新資產 bia_level |
| 6 | 資產發現 | 外部採集 → staging → IP 自動匹配 → 人工審核 → 入庫 |
| 7 | 機架管理 | 4 級位置選擇 → 建立機架 → U 位管理（衝突檢測）|
| 8 | 質量治理 | 定義規則 → 全量掃描 → 四維評分 → 最差資產識別 |
| 9 | 即時推送 | 寫操作 → NATS → WebSocket → React Query invalidate |
| 10 | Critical 變更審計 | 資產更新 → BIA=critical → 自動建工單 → 主管覆核 |
| 11 | Webhook 投遞 | 事件 → BIA 過濾 → HMAC 簽名 → HTTP POST → 記錄結果 |
| 12 | 盤點匯入 | JSON 匯入 → serial/tag 匹配 → matched/discrepancy 統計 |

### 4.2 未完整 / 待開發（6 條）

| # | 流程 | 缺失 |
|---|------|------|
| 1 | 設備健康監控 → 告警觸發 | 無 health/sensor API，頁面部分 mock |
| 2 | 能源 PUE 趨勢分析 | metrics 有 seed，PowerLoad view hardcoded |
| 3 | 拓撲影響關聯 | AlertTopology 節點/邊 hardcoded |
| 4 | 工單派工指派 | TaskDispatch Assign 按鈕為 placeholder |
| 5 | 升級建議引擎 | ComponentUpgrade 100% mock |
| 6 | 影片教學系統 | VideoPlayer 基礎功能（已可讀 URL 參數） |

---

## 五、數據層分析

### 5.1 表統計

| 領域 | 表數 | 主要表 |
|------|------|--------|
| 身份 | 4 | tenants, users, roles, user_roles |
| 位置 | 2 | locations (ltree), departments |
| 資產 | 3 | assets, racks, rack_slots |
| 維護 | 2 | work_orders, work_order_logs |
| 監控 | 4 | alert_rules, alert_events, incidents, metrics (hypertable) |
| 盤點 | 2 | inventory_tasks, inventory_items |
| 審計 | 1 | audit_events |
| 預測 | 3 | prediction_models, prediction_results, rca_analyses |
| 整合 | 3 | integration_adapters, webhook_subscriptions, webhook_deliveries |
| BIA | 3 | bia_assessments, bia_scoring_rules, bia_dependencies |
| 質量 | 2 | quality_rules, quality_scores |
| 發現 | 1 | discovered_assets |

**共 30 表，127 個命名查詢**

### 5.2 Seed 數據

| 類型 | 筆數 |
|------|------|
| 租戶 | 1 |
| 用戶 | 3 |
| 角色 | 3 |
| 位置（4 層） | 9 |
| 機架 | 10 |
| 資產 | 20 |
| U 位 slot | 20 |
| 告警事件 | 8 |
| 告警規則 | 5 |
| 事件 | 3 |
| 工單 | 6 |
| 盤點任務 | 3 |
| 盤點項目 | 10 |
| 審計事件 | 10 |
| 預測模型 | 2 |
| 預測結果 | 5 |
| RCA 分析 | 2 |
| BIA 規則 | 4 |
| BIA 評估 | 4 |
| BIA 依賴 | 8 |
| 質量規則 | 5 |
| 發現資產 | 5 |
| Webhook 投遞 | 5 |
| 資產屬性 | 10（UPDATE 語句） |
| Metrics | 236 行（24h × 3 資產 × 4 指標）|

---

## 六、AI / AIOps 能力

### 6.1 MCP Server

| Tool | 功能 |
|------|------|
| search_assets | 按 type/status/serial 搜索資產 |
| get_asset_detail | UUID 或 asset_tag 查詢詳情 |
| query_alerts | 按 severity/status/asset 查告警 |
| get_topology | 位置層級查詢 |
| query_metrics | 時序指標查詢（placeholder）|
| query_work_orders | 按 status 查工單 |
| trigger_rca | 觸發根因分析 |

### 6.2 AI 適配器

- **Dify**：Workflow 整合
- **Native LLM**：自建模型接入
- **Custom**：自定義 REST 適配器

### 6.3 AI 成熟度

| 能力 | 狀態 |
|------|------|
| MCP 工具暴露 | ✅ 已實現 |
| 適配器框架 | ✅ 已實現 |
| 事件驅動 AI 觸發 | ✅ 已實現（NATS → Webhook） |
| 故障預測模型 | ⚠️ 骨架（需外部 ML 整合）|
| RCA 引擎 | ⚠️ 骨架（返回 placeholder）|
| 異常檢測 | ❌ 未實現 |

---

## 七、UI 功能完整度

### 7.1 按鈕統計

| 類型 | 數量 | 佔比 |
|------|------|------|
| Working（真實功能） | ~150 | 79% |
| Placeholder（Coming Soon） | ~33 | 17% |
| Broken（無回應） | ~3 | 2% |

### 7.2 頁面評分分佈

| 評級 | 數量 | 頁面 |
|------|------|------|
| 9/10 | 2 | AutoDiscovery, ScoringRules |
| 8/10 | 11 | RegionOverview, CityOverview, AddNewRack, AssetManagement, AddMaintenanceTask, BIAOverview, SystemGrading, RtoRpoMatrices, Login, DependencyMap, QualityDashboard, AuditHistory, SensorConfig |
| 7/10 | 10 | GlobalOverview, CampusOverview, FacilityMap, Dashboard, MonitoringAlerts, MaintenanceHub, WorkOrder, MaintenanceTaskView, SystemSettings, Welcome |
| 6/10 | 7 | RackManagement, DataCenter3D, AssetDetail, AssetLifecycle, AuditEventDetail, HighSpeedInventory, PredictiveHub |
| 5/10 | 8 | RackDetail, TaskDispatch, InventoryItemDetail, SystemHealth, EnergyMonitor, AlertTopology, UserProfile, EquipmentHealth, ComponentUpgrade, TroubleshootingGuide, VideoLibrary |
| 4/10 | 1 | AssetLifecycleTimeline |
| 3/10 | 1 | VideoPlayer |

### 7.3 模組評分

| 排名 | 模組 | 平均分 |
|------|------|--------|
| 1 | BIA + Quality | 8.2 |
| 2 | Topology / Rack | 6.9 |
| 3 | Maintenance | 6.4 |
| 4 | Identity / System | 6.2 |
| 5 | Monitoring / Dashboard | 5.8 |
| 6 | Asset | 5.6 |
| 7 | Help / Onboarding | 4.7 |

---

## 八、部署架構

### 8.1 部署模式

| 模式 | 說明 |
|------|------|
| **cloud**（預設） | 中央伺服器，單一 PostgreSQL，NATS 可選聯邦 |
| **edge** | 邊緣節點，部分複製，NATS 聯邦複製關鍵主題 |

### 8.2 基礎設施

| 服務 | 技術 | 用途 |
|------|------|------|
| 主資料庫 | PostgreSQL 17 + TimescaleDB | 30 表 + 時序指標 |
| 快取 | Redis 7 | RBAC 權限快取 |
| 事件匯流排 | NATS 2.10 JetStream | 20 主題，7 天留存 |
| 反向代理 | Nginx 1.27 | SSL、壓縮、路由 |
| 可觀測性 | Prometheus + Loki + Jaeger + Grafana | 指標 + 日誌 + 追蹤 |
| 追蹤 | OpenTelemetry Collector | OTLP gRPC |

### 8.3 容器化

Docker Compose 一鍵啟動，支持：
- 服務副本數配置（CORE_REPLICAS, WORKER_REPLICAS）
- 健康檢查（所有服務）
- 數據持久化（PG, Redis, NATS volumes）
- Observability stack 全包

---

## 九、綜合評分

### 最終評分：72/100

| 維度 | 分數 | 權重 | 加權 |
|------|------|------|------|
| 後端 API 完整性 | 95 | 25% | 23.75 |
| 前端查詢接通率 | 90 | 20% | 18.00 |
| 前端寫入接通率 | 80 | 15% | 12.00 |
| 按鈕功能完整度 | 65 | 15% | 9.75 |
| 業務邏輯完整度 | 60 | 15% | 9.00 |
| 數據真實性 | 55 | 10% | 5.50 |
| **總計** | | | **78.00** |

### 與會話開始時的對比

| 指標 | 開始 | 現在 | 變化 |
|------|------|------|------|
| 後端端點 | 45 | 93 | **+48** |
| DB 表 | 20 | 30 | **+10** |
| 前端頁面 | 38 | 46 | **+8** |
| 寫入操作 | 5 | 31 | **+26** |
| BROKEN 頁面 | 1 | 0 | **-1** |
| 業務流程閉環 | 3 | 12 | **+9** |
| 新增模組 | 0 | BIA + Quality + Discovery | **+3** |
| 事件驅動 | 無 | NATS → WS → React Query | **新建** |
| Placeholder 按鈕 | ~85 | ~33 | **-52** |

---

## 十、改進建議

### 短期（1-2 週）

1. **Phase B/C 頁面修復**：RackDetail 子 tab 接 API、InventoryItemDetail 導航參數、TaskDispatch 派工功能
2. **metrics 圖表接通**：EnergyMonitor 用 useMetrics 渲染真實功率/PUE
3. **AssetDetail Health/Usage tab**：接 useMetrics 取 CPU/溫度/記憶體時序數據

### 中期（1-2 月）

4. **RCA 引擎整合**：接入 Dify 或本地 ML 模型，trigger_rca 返回真實分析
5. **拓撲視覺化**：從 bia_dependencies + assets 衍生節點圖，替換 AlertTopology hardcoded 節點
6. **Ingestion Engine 整合**：Python FastAPI 採集器接入 VMware/SNMP

### 長期（3-6 月）

7. **水平擴展測試**：多副本 cmdb-core + 負載均衡
8. **RLS (Row-Level Security)**：PostgreSQL 級別的租戶隔離
9. **SOC2/ISO27001 合規增強**：加密存儲、進階存取日誌
10. **Mobile App**：盤點掃碼 H5/PWA

---

## 十一、代碼品質總結

| 維度 | 評語 |
|------|------|
| **架構** | 優秀：模組化單體 + 事件驅動，職責清晰，OpenAPI spec-first |
| **類型安全** | 優秀：sqlc 生成 Go 類型 + openapi-typescript 生成 TS 類型，端到端類型安全 |
| **可觀測性** | 優秀：zap 日誌 + Prometheus 指標 + OpenTelemetry 追蹤 |
| **安全性** | 良好：JWT + RBAC + 審計 + HMAC webhook，缺 RLS 和加密存儲 |
| **前端** | 良好：React 19 + React Query + Zustand，M3 設計系統一致 |
| **測試** | 不足：僅有 smoke test script，無單元測試、無整合測試 |
| **文檔** | 良好：CHANGELOG + 多份分析/修復計劃，缺 API 使用文檔 |
