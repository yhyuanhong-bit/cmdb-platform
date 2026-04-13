# CMDB Platform 完整項目分析報告

> 生成日期：2026-04-11
> 分析範圍：後端 (cmdb-core)、前端 (cmdb-demo)、數據引擎 (ingestion-engine)、基礎設施

---

## 一、項目定位與目標

CMDB Platform（品牌名：IronGrid）是一套**全球化 CMDB + AIOps 一體化平台**，面向企業級 IT 基礎設施管理，目標：

1. 維護**權威資產清冊**（伺服器、網路設備、儲存等）
2. 支援**數據驅動的容量規劃與維護排程**
3. 透過 **AI/ML 預測分析**自動化事件回應（故障預測、根因分析）
4. 確保合規——**營業衝擊分析 (BIA)**、RTO/RPO 管理
5. 提供**即時可視化**：3D 機房、拓撲圖、能源監控

平台評分：**95/100**（自評），已具備生產就緒能力。

---

## 二、技術架構

```
┌─ Frontend ─────────────────────────────────────────────┐
│  React 19 + TypeScript + Vite 8 + Tailwind CSS 4       │
│  50+ 頁面 | i18n (en/zh-CN/zh-TW) | React Query         │
└────────────────┬───────────────────────────────────────┘
                 │ HTTP/WS (Vite Proxy → :8080)
┌────────────────▼───────────────────────────────────────┐
│  cmdb-core (Go 1.25 / Gin)                              │
│  80+ REST API | RBAC | JWT | WebSocket Hub               │
│  MCP Server (:3001) — 7 tools for AI agents              │
├─────────┬──────────┬──────────┬─────────────────────────┤
│ PostgreSQL 17      │ Redis 7  │ NATS 2.10 JetStream      │
│ + TimescaleDB      │ (cache)  │ (event bus)               │
│ 30+ tables         │          │                           │
└─────────┴──────────┴──────────┴─────────────────────────┘
┌─ ingestion-engine (Python 3.12 / FastAPI) ──────────────┐
│  13 API endpoints | Celery Workers                        │
│  Collectors: SNMP / SSH / IPMI                            │
└──────────────────────────────────────────────────────────┘
┌─ Observability ─────────────────────────────────────────┐
│  Prometheus + Grafana + Jaeger + Loki + OTEL Collector    │
└──────────────────────────────────────────────────────────┘
```

### 技術棧明細

| 層級 | 技術 | 版本 |
|------|------|------|
| 前端框架 | React + React Router | 19.2 / 7.13 |
| 狀態管理 | React Query + Zustand | 5.95 / 5.0 |
| UI | Tailwind CSS + Material Symbols | 4.2 |
| 構建工具 | Vite | 8.0 |
| 後端框架 | Go + Gin | 1.25 / 1.12 |
| 資料庫 | PostgreSQL + TimescaleDB | 17 |
| 快取/佇列 | Redis | 7.4 |
| 訊息匯流 | NATS JetStream | 2.10 |
| WebSocket | Gorilla WebSocket | 1.5 |
| MCP | mcp-go | 0.46 |
| 數據引擎 | Python + FastAPI + Celery | 3.12 / 0.115 / 5.4 |
| 可觀測性 | OpenTelemetry + Prometheus + Grafana + Jaeger + Loki | Latest |

---

## 三、資料庫架構（30+ 表）

### 核心表

| 表名 | 用途 | 關鍵欄位 |
|------|------|---------|
| `tenants` | 多租戶 | id, name, slug, settings |
| `users` | 使用者 | id, tenant_id, username, password_hash, status |
| `roles` | 角色 | id, tenant_id, name, permissions (JSONB) |
| `user_roles` | 角色指派 | user_id, role_id |
| `departments` | 組織架構 | id, tenant_id, name |

### 資產與拓撲

| 表名 | 用途 | 關鍵欄位 |
|------|------|---------|
| `locations` | 位置階層 (ltree) | id, parent_id, path, level (territory/region/city/campus/idc/module/room) |
| `racks` | 機櫃 | id, location_id, name, total_u, power_capacity_kw |
| `rack_slots` | U 位映射 | rack_id, asset_id, start_u, end_u, side |
| `assets` | 資產清冊 | id, tenant_id, asset_tag, type, status, location_id, rack_id, attributes (JSONB) |
| `asset_dependencies` | 拓撲關係 | source_asset_id, target_asset_id, dependency_type |
| `rack_network_connections` | 實體連線 | rack_id, source_port, connected_asset_id, status |

### 維運與告警

| 表名 | 用途 | 關鍵欄位 |
|------|------|---------|
| `work_orders` | 工單 | id, code, status, priority, asset_id, sla_deadline |
| `work_order_logs` | 狀態機歷史 | order_id, action, from_status, to_status |
| `work_order_comments` | 工單討論 | order_id, author_id, text |
| `alert_rules` | 告警規則 | id, metric_name, condition (JSONB), severity |
| `alert_events` | 告警事件 | id, rule_id, asset_id, status (firing/acked/resolved) |
| `incidents` | 事件追蹤 | id, title, status, severity |

### 盤點與預測

| 表名 | 用途 | 關鍵欄位 |
|------|------|---------|
| `inventory_tasks` | 盤點任務 | id, code, scope_location_id, status, assigned_to |
| `inventory_items` | 盤點項目 | id, task_id, asset_id, expected/actual (JSONB), status |
| `inventory_scan_history` | 掃描記錄 | item_id, method (RFID/barcode/manual) |
| `prediction_models` | AI 模型 | id, name, type, provider (dify/llm/custom) |
| `prediction_results` | 預測結果 | id, asset_id, prediction_type, result (JSONB) |
| `rca_analyses` | 根因分析 | id, incident_id, reasoning (JSONB), confidence |

### BIA 與品質

| 表名 | 用途 | 關鍵欄位 |
|------|------|---------|
| `bia_assessments` | 營業衝擊 | id, system_name, tier, rto_hours, rpo_minutes |
| `bia_dependencies` | BIA 資產關聯 | assessment_id, asset_id, criticality |
| `quality_rules` | 資料品質規則 | id, ci_type, dimension, weight |
| `quality_scores` | 品質分數 | asset_id, completeness, accuracy, timeliness, consistency |

### 整合與稽核

| 表名 | 用途 | 關鍵欄位 |
|------|------|---------|
| `discovered_assets` | 自動發現 | id, source, hostname, ip_address, status |
| `integration_adapters` | 外部系統 | id, name, type (rest/snmp), direction |
| `webhook_subscriptions` | Webhook 訂閱 | id, url, events, secret |
| `audit_events` | 稽核軌跡 | id, action, module, user_id, diff (JSONB) |
| `metrics` | 時序指標 (TimescaleDB) | asset_id, metric_name, value, timestamp |
| `notifications` | 通知 | id, user_id, type, title, read_at |

---

## 四、後端 API 總覽（80+ 端點）

### 認證與授權（4 端點）
- `POST /auth/login` — 登入取得 JWT (15min) + Refresh Token (7d)
- `POST /auth/refresh` — 刷新 Token
- `POST /auth/change-password` — 修改密碼
- `GET /auth/me` — 取得當前使用者資訊

### 資產管理（7 端點）
- `GET/POST /assets` — 列表/建立資產
- `GET/PUT/DELETE /assets/{id}` — 查看/更新/刪除資產
- `GET /assets/lifecycle-stats` — 生命週期統計
- `GET /assets/{id}/upgrade-recommendations` — AI 升級建議

### 拓撲與位置（13 端點）
- `GET/POST /locations` — 根節點列表/建立位置
- `GET/PUT/DELETE /locations/{id}` — CRUD
- `GET /locations/{id}/ancestors|children|descendants|racks|stats` — 階層查詢
- `GET /topology/graph|dependencies` — 拓撲圖 / 依賴關係
- `POST/DELETE /topology/dependencies` — 建立/刪除依賴

### 機櫃與實體佈局（12 端點）
- `GET/PUT/DELETE /racks/{id}` — 機櫃 CRUD
- `GET /racks/{id}/assets|slots|maintenance|network-connections` — 關聯查詢
- `POST/DELETE /racks/{id}/slots` — U 位指派
- `GET /racks/stats` — 統計

### 維運工單（9 端點）
- `GET/POST /maintenance/orders` — 列表/建立工單
- `GET/PUT/DELETE /maintenance/orders/{id}` — CRUD
- `POST /maintenance/orders/{id}/transition` — 狀態機流轉
- `GET /maintenance/orders/{id}/logs|comments` — 歷史/評論

**狀態機**：`draft → submitted → approved → in_progress → completed/rejected → verified`

**SLA 規則**：Critical 4h | High 8h | Medium 24h | Low 72h

### 監控告警（15 端點）
- `GET /monitoring/alerts` — 告警列表
- `POST /monitoring/alerts/{id}/acknowledge|resolve` — 確認/解決
- `GET/POST/PUT/DELETE /monitoring/rules` — 告警規則 CRUD
- `GET/POST/PUT /monitoring/incidents` — 事件管理
- `GET /monitoring/metrics` — 時序指標查詢
- `GET /monitoring/alerts/trend` — 趨勢

### 盤點管理（12+ 端點）
- `GET/POST /inventory/tasks` — 任務 CRUD
- `POST /inventory/tasks/{id}/complete` — 完成任務
- `POST /inventory/tasks/{id}/import-items` — Excel 匯入
- `POST /inventory/tasks/{id}/items/{itemId}/scan` — 掃描
- `GET /inventory/tasks/{id}/discrepancies` — 差異項

### 預測分析與 AI（8 端點）
- `GET /prediction/models` — AI 模型列表
- `GET /prediction/results/{assetId}` — 預測結果
- `GET /prediction/rul/{assetId}` — 剩餘使用壽命 (RUL)
- `POST /prediction/rca` — 觸發根因分析
- `GET/POST /prediction/rca/{id}` — RCA 結果/驗證

### 營業衝擊分析 BIA（11 端點）
- `GET/POST/PUT/DELETE /bia/assessments` — 評估 CRUD
- `GET/POST/DELETE /bia/dependencies` — 依賴管理
- `GET /bia/impact/{id}` — 級聯衝擊分析
- `GET/PUT /bia/scoring-rules` — 評分規則

### 資料品質（5 端點）
- `GET /quality/dashboard|rules|worst-assets` — 品質看板
- `POST /quality/scan|rules` — 掃描/建立規則

### 自動發現（7 端點）
- `POST /discovery/ingest` — 接收發現資產
- `GET /discovery/assets|stats` — 列表/統計
- `POST /discovery/assets/{id}/approve|ignore` — 審批

### 整合（5 端點）
- `GET/POST /integration/adapters|webhooks` — 整合器/Webhook CRUD

### 身份管理（10 端點）
- `GET/POST/PUT/DELETE /users` — 使用者 CRUD
- `GET/POST/DELETE /users/{id}/roles` — 角色指派
- `GET /roles` — 角色列表

### 其他
- `GET /dashboard/stats` — 儀表板統計
- `GET /activity-feed` — 活動動態
- `GET /energy/breakdown|summary|trend` — 能源監控
- `GET /notifications` — 通知
- `GET /audit/events` — 稽核日誌
- `GET /sensors` — 感測器管理
- `GET /ws` — WebSocket 即時推送
- `GET /healthz|readyz|metrics` — 健康/指標

---

## 五、前端頁面清單（50+ 頁面）

### 公開頁面

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/login` | Login | 登入表單，JWT 認證 |
| `/welcome` | Welcome | 5 步驟引導頁 |

### 位置階層導航

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/locations` | GlobalOverview | 世界地圖 + 領域標記 + KPI |
| `/locations/:territorySlug` | RegionOverview | 區域地圖 + 子區域 |
| `/locations/:t/:regionSlug` | CityOverview | 城市檢視 + 園區列表 |
| `/locations/:t/:r/:citySlug` | CampusOverview | 園區詳情 + IDC 列表 |

### 儀表板

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/dashboard` | Dashboard | 4 張統計卡、BIA 甜甜圈圖、機櫃熱力圖、生命週期財務、關鍵事件表 |

### 資產管理

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/assets` | AssetManagementUnified | 資產列表（表格/卡片切換）、篩選、匯入/匯出 |
| `/assets/:assetId` | AssetDetailUnified | 4 分頁：總覽、健康、使用、維護 |
| `/assets/lifecycle` | AssetLifecycle | 生命週期階段分佈 + 財務摘要 |
| `/assets/lifecycle/timeline/:id` | AssetLifecycleTimeline | 單一資產時間線 |
| `/assets/discovery` | AutoDiscovery | 自動發現資產審批（approve/ignore） |
| `/assets/upgrades` | ComponentUpgradeRecommendations | AI 升級建議 |
| `/assets/equipment-health` | EquipmentHealthOverview | 設備健康概覽 + 風險評估 |

### 機櫃與基礎設施

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/racks` | RackManagement | 機櫃列表、搜尋、活動動態 |
| `/racks/:id` | RackDetailUnified | 前/後視圖、U 位佈局、設備、告警、網路連線 |
| `/racks/3d` | DataCenter3D | 3D 機房視圖 + 位置階層樹 |
| `/racks/facility-map` | FacilityMap | 樓層平面圖 |
| `/racks/add` | AddNewRack | 新增機櫃表單 |

### 盤點

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/inventory` | HighSpeedInventory | 高速盤點（Excel 匯入、任務掃描、差異解決） |
| `/inventory/detail` | InventoryItemDetail | 盤點項目詳情 + 掃描歷史 |

### 監控

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/monitoring` | MonitoringAlerts | 告警列表 + 嚴重度篩選 + 趨勢圖 |
| `/monitoring/health` | SystemHealth | 系統健康儀表板 |
| `/monitoring/topology` | AlertTopologyAnalysis | 告警拓撲分析 |
| `/monitoring/sensors` | SensorConfiguration | 感測器配置 + 閾值管理 |
| `/monitoring/energy` | EnergyMonitor | 能源監控（功耗、PUE、效率） |

### 維運

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/maintenance` | MaintenanceHub | 工單列表 + 維護日曆 |
| `/maintenance/task/:id` | MaintenanceTaskView | 工單詳情 + 狀態流轉 |
| `/maintenance/workorder` | WorkOrder | 工單審批列表 |
| `/maintenance/add` | AddMaintenanceTask | 建立維護任務 |
| `/maintenance/dispatch` | TaskDispatch | 任務派工排程 |

### 預測 AI

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/predictive` | PredictiveHub | 6 分頁：概覽、告警、洞察、建議、時間線、預測 |

### 稽核與品質

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/audit` | AuditHistory | 稽核日誌（即時/歷史/歸檔）+ diff 檢視 |
| `/audit/detail` | AuditEventDetail | 事件詳情 |
| `/quality` | QualityDashboard | 資料品質分數（完整性/準確性/即時性/一致性） |

### BIA 營業衝擊分析

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/bia` | BIAOverview | 評估列表 + 層級甜甜圈圖 |
| `/bia/grading` | SystemGrading | 系統分級 + 分數分佈 |
| `/bia/rto-rpo` | RtoRpoMatrices | RTO/RPO 矩陣 |
| `/bia/rules` | ScoringRules | 評分規則管理 |
| `/bia/dependencies` | DependencyMap | 依賴地圖視覺化 |

### 系統管理

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/system` | RolesPermissions | 角色權限矩陣（read/write/delete/export） |
| `/system/settings` | SystemSettings | 6 分頁：權限、使用者、整合器、Webhook、憑證、健康 |
| `/system/profile` | UserProfile | 個人資料 |

### 幫助

| 路由 | 頁面 | 功能 |
|------|------|------|
| `/help/troubleshooting` | TroubleshootingGuide | 故障排除指南 |
| `/help/videos` | VideoLibrary | 影片庫 |
| `/help/videos/player` | VideoPlayer | 影片播放 |

---

## 六、12 大業務模組詳解

### 1. 資產管理 (Asset)
- CRUD + 軟刪除 + 稽核軌跡
- 類型：server / network / storage / power / sensor
- 狀態：procurement → deploying → operational → maintenance → decommission → disposed
- JSONB attributes 彈性欄位（CPU、記憶體、儲存、網路等）
- 事件發布：asset.created / updated / status_changed / deleted

### 2. 拓撲管理 (Topology)
- 7 層 ltree 階層：territory → region → city → campus → idc → module → room
- 機櫃 U 位管理（42U，前/後兩面）
- 資產依賴關係圖（source → target + type）
- 網路連線追蹤（port → connected_asset + status）
- 統計聚合：子節點資產數、機櫃佔用率、告警數

### 3. 維運管理 (Maintenance)
- 工單狀態機：draft → submitted → approved → in_progress → completed/rejected → verified
- SLA 自動計算：Critical 4h / High 8h / Medium 24h / Low 72h
- 審批驗證：需 ops-admin 角色，禁止自我審批
- 樂觀鎖：from_status 比對防止並發衝突
- 評論系統 + 狀態變更日誌

### 4. 監控告警 (Monitoring)
- 告警規則：metric_name + condition (JSONB) + severity + threshold
- 告警事件生命週期：firing → acknowledged → resolved
- 事件管理 (Incidents)：建立、分配、解決、記錄
- 時序指標存儲（TimescaleDB hypertable）
- 趨勢分析：24h 每小時統計

### 5. 盤點管理 (Inventory)
- 任務制盤點：指定範圍（location）、指派人員
- 掃描方式：RFID / Barcode / Manual
- Expected vs Actual 比對 → 差異標記
- Excel 批量匯入（中英文表頭支援）
- 差異解決工作流

### 6. 預測分析 (Prediction)
- AI 模型支援：Dify (workflow) / LLM (Claude/OpenAI) / Custom (REST)
- 預測類型：故障概率 / 剩餘壽命 (RUL) / 根因分析 (RCA)
- RCA 流程：觸發 → AI 推理 → 結論 + 置信度 → 人工驗證
- 升級建議：基於規則引擎產生硬體升級推薦

### 7. 營業衝擊分析 BIA
- 評估維度：系統名稱、層級、RTO (Recovery Time Objective)、RPO (Recovery Point Objective)、MTPD
- 4 層分級：Tier-1 Critical / Tier-2 Important / Tier-3 Normal / Tier-4 Minor
- 級聯影響分析：計算下游受影響系統
- 合規追蹤 + 評分規則自定義

### 8. 資料品質 (Quality)
- 4 維度評分：完整性 / 準確性 / 即時性 / 一致性
- 自動掃描所有資產
- 規則引擎：per CI type + dimension + weight
- 最差資產排行（Bottom 10）

### 9. 自動發現 (Discovery)
- 數據來源：SNMP / SSH / IPMI / API webhook
- 流程：發現 → 去重比對 → pending → approve (建立資產) / ignore
- 衝突偵測：hostname / IP / MAC 比對現有資產

### 10. 整合管理 (Integration)
- 整合器類型：REST / SNMP，方向：inbound / outbound / bidirectional
- 預設整合：Prometheus (inbound), SNMP Poller, ServiceNow ITSM
- Webhook：事件訂閱 + HMAC 簽名 + 3x 重試 + 投遞日誌

### 11. 身份管理 (Identity)
- JWT 認證（15 分鐘 access + 7 天 refresh）
- RBAC 權限模型：角色 → 資源 + 動作 (read/write/delete)
- 系統角色：super-admin / ops-admin / viewer
- 自定義角色（JSONB permissions）
- Session 追蹤（IP、瀏覽器、裝置）

### 12. 稽核合規 (Audit)
- 所有寫操作自動記錄
- 欄位級 diff（JSONB 格式）
- 按 action / module / user / 時間篩選
- 保留期限：365 天

---

## 七、數據引擎 (ingestion-engine)

### 技術棧
Python 3.12 + FastAPI + Celery + asyncpg + nats-py

### 13 個 API 端點

**匯入流程**：
1. `POST /import/upload` → 上傳 Excel/CSV
2. `GET /import/{job_id}/preview` → 預覽資料
3. `POST /import/{job_id}/confirm` → 確認匯入（Celery 非同步處理）
4. `GET /import/{job_id}/progress` → 查詢進度

**自動發現**：
1. `POST /discovery/scan` → 觸發掃描
2. `GET /discovery/tasks` → 任務列表
3. `GET /discovery/tasks/{id}` → 任務詳情

**憑證管理**：
- `GET/POST/PUT/DELETE /credentials` — Fernet 加密存儲

**收集器管理**：
- `GET /collectors` — 列表
- `POST /collectors/{name}/start|stop|test` — 啟停/測試

### 處理流程
```
上傳檔案 → 解析 → 預覽 → 確認 → Celery 非同步 → 正規化 → 去重 →
驗證 → 權威欄位合併 → 衝突偵測 → 建立/更新資產 → NATS 事件發布
```

---

## 八、事件驅動架構

### NATS 事件主題
```
asset.created / asset.updated / asset.status_changed / asset.deleted
location.created / location.updated / location.deleted
rack.created / rack.updated / rack.occupancy_changed
maintenance.order_created / maintenance.order_transitioned
inventory.task_created / inventory.task_completed / inventory.item_scanned
alert.fired / alert.resolved
import.completed / import.conflict_created
prediction.created
notification.created
audit.recorded
```

### 跨模組工作流
1. **工單完成** → 自動解決相關告警 → 更新資產 → 通知申請人
2. **告警觸發** → (計劃) 自動建立事件 → Webhook 投遞
3. **NATS → WebSocket** 橋接：即時推送至前端
4. **NATS → Webhook** 分發：HMAC 簽名 + 3x 重試

---

## 九、基礎設施

### Docker Compose 服務

| 服務 | 映像 | 端口 | 用途 |
|------|------|------|------|
| cmdb-core | Go binary | 8080, 3001 | API + MCP |
| ingestion-engine | Python/FastAPI | 8081 | 數據引擎 |
| ingestion-worker | Celery | - | 非同步任務 |
| postgres | timescale/timescaledb:pg17 | 5432 | 主資料庫 |
| redis | redis:7.4-alpine | 6379 | 快取 + Broker |
| nats | nats:2.10-alpine | 4222, 7422, 8222 | 訊息匯流 |
| nginx | nginx:1.27-alpine | 80 | 反向代理 |
| otel-collector | OTEL contrib | 4317, 4318 | 遙測收集 |
| jaeger | jaeger:1.64 | 16686 | 分散式追蹤 |
| prometheus | prometheus:v3.1 | 9090 | 指標 |
| loki | loki:3.3 | 3100 | 日誌聚合 |
| promtail | promtail:3.3 | - | 日誌採集 |
| grafana | grafana:11.4 | 3000 | 視覺化面板 |

### 部署模式
- **Cloud（中央）**：多租戶、全服務、NATS 聯邦
- **Edge（邊緣）**：單租戶、最小依賴、離線優先

---

## 十、安全機制

| 層面 | 機制 |
|------|------|
| 認證 | JWT (HMAC-SHA256, 15min TTL) + Refresh Token (Redis, 7d) |
| 授權 | RBAC — 角色 → 資源 + 動作，Redis 快取 (5min TTL) |
| 密碼 | bcrypt 雜湊 |
| API 安全 | Security Headers middleware (CSP, X-Frame-Options 等) |
| Webhook | HMAC-SHA256 簽名 |
| 憑證 | Fernet 對稱加密 (ingestion-engine) |
| 稽核 | 所有寫操作記錄 (audit_events) |
| 多租戶 | tenant_id 隔離 + WebSocket 租戶過濾 |

---

## 十一、國際化 (i18n)

| 語言 | 代碼 | 翻譯量 |
|------|------|--------|
| English | en | ~102KB |
| 簡體中文 | zh-CN | ~98KB |
| 繁體中文 (預設) | zh-TW | ~99KB |

---

## 十二、前端設計系統

- **CSS 框架**：Tailwind CSS 4.2 自定義 Design Token
- **圖標**：Material Symbols Outlined
- **配色**：Material Design 3 風格（surface/primary/error token）
- **圖表**：自製 SVG（甜甜圈、進度條、熱力圖、折線圖、甘特圖）+ Recharts
- **表單**：useState 受控元件 + Sonner toast 通知
- **對話框**：13+ 自定義 Modal 元件
- **響應式**：Tailwind 斷點 (sm/md/lg/xl)
- **動畫**：animate-spin / animate-ping / transition-all

---

## 十三、已知問題與改善空間

1. **WebSocket**：開發環境透過外部 IP 訪問時 Vite proxy WS 不穩定
2. **指標查詢**：`/monitoring/metrics` 為 placeholder，TimescaleDB 查詢待完善
3. **AI 整合**：部分 AI Provider (local) 尚未實作
4. **前端效能**：SensorConfiguration 頁面 useEffect 依賴問題（已修復）
5. **資料品質**：Quality scan 為全表掃描，大量資產時需分頁
6. **Edge 模式**：離線同步機制尚未完全實作
