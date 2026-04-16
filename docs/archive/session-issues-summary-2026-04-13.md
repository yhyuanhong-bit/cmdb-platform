# Session 問題匯總與修復建議

> 日期：2026-04-11 ~ 2026-04-13
> 範圍：前端、後端、基礎設施、架構設計

---

## 一、已修復的問題（本次 Session）

### 前端問題

| # | 問題 | 根因 | 修復 | 檔案 |
|---|------|------|------|------|
| 1 | WebSocket 無限渲染迴圈 (Maximum update depth) | `useEffect` 依賴 `invalidateByEvent`（useCallback）導致每次重連觸發重渲染 | 改用 `useRef` 存回調，effect 只依賴 `token` 和 `isAuthenticated` | `useWebSocket.ts` |
| 2 | WebSocket 開發環境外部 IP 連不上 | Vite WS proxy 對外部 IP 不可靠 + Auth/RBAC middleware 阻擋 `/ws` 路徑 | 後端跳過 Auth/RBAC 給 `/ws`（WSAuth 獨立認證）；前端保持走 Vite proxy | `main.go`, `rbac.go`, `useWebSocket.ts` |
| 3 | WebSocket 慢輪詢每 30s 觸發 5 次快速重試 | `retries = 0` 重置後重新進入快速重試循環 | 慢輪詢不重置 retries | `useWebSocket.ts` |
| 4 | WebSocket 死代碼 | 重構後 `reconnectTimerRef` 和 `retriesRef` 不再使用 | 刪除 | `useWebSocket.ts` |
| 5 | WebSocket token 過期後無法重連 | Token 15 分鐘過期，WS 不嘗試 refresh | 重連時讀取最新 token + 觸發 refreshTokens | `useWebSocket.ts` |
| 6 | favicon.ico 404 | 缺少圖標檔案 | 建立 `public/favicon.ico` + `<link rel="icon">` | `index.html`, `favicon.ico` |
| 7 | SensorConfiguration 無限迴圈 | `useEffect` 依賴 `[apiSensors, allAssets]`，每次渲染引用變化 | 移除 asset fallback，空狀態顯示提示 | `SensorConfiguration.tsx` |
| 8 | DataCenter3D duplicate key 警告 | `buildRackGrid` 用 `rack.name` 或 `id.slice(0,6)` 作為 key，可能重複 | 拆分 `id`（UUID）和 `label`（顯示名），key 用完整 UUID | `DataCenter3D.tsx` |
| 9 | FacilityMap duplicate key | 同上，加了 idx 反模式 | 恢復 `key={rack.id}`（UUID 本身唯一） | `FacilityMap.tsx` |
| 10 | PredictiveHub 使用假資料 | 6 個 tab 中 5 個用 `data/fallbacks/predictive.ts` 的硬編碼資料 | 全部改用真實 API hook（useAlerts, useFailureDistribution, useWorkOrders） | `PredictiveHub.tsx` |
| 11 | Vite ingestion proxy 端口錯誤 | 配置指向 `localhost:8000`，實際 ingestion-engine 運行在 `8081` | `8000` → `8081` | `vite.config.ts` |

### 後端問題

| # | 問題 | 根因 | 修復 | 檔案 |
|---|------|------|------|------|
| 12 | 碳排放係數硬編碼 | `0.0005` tCO2/kWh 寫死在程式碼中 | 改為環境變數 `CARBON_EMISSION_FACTOR`，Config 傳入 APIServer | `config.go`, `energy_endpoints.go`, `impl.go` |
| 13 | MCP query_metrics 為 stub | 回傳固定 "not implemented" 訊息 | 接入 TimescaleDB 真實查詢，支援 asset_id/metric_name/time_range | `tools.go` |
| 14 | user_sessions 無限增長 | 無清理排程 | 每小時清理：7 天過期標記 + 30 天歷史刪除 + 每用戶保留 20 筆 | `subscriber.go`, `main.go` |
| 15 | 通知模組不完整 | 只有讀取端（4 端點），缺少事件觸發建立 | 新增 4 個事件觸發（資產建立、告警、盤點完成、匯入完成）+ opsAdminUserIDs helper | `subscriber.go` |
| 16 | 機櫃佔用率 TODO | `topology/service.go` 佔用率硬編碼為 0 | 真實 SQL 查詢 rack_slots 計算平均佔用率 | `topology/service.go` |
| 17 | Dashboard 快取 TTL 偏短 | 30 秒 TTL 導致頻繁查詢 | 調至 60 秒 | `dashboard/service.go` |

### 測試問題

| # | 問題 | 修復 |
|---|------|------|
| 18 | 後端僅 4 個測試檔案 | 新增 8 個測試模組：config, response, eventbus, mcp, maintenance/service, quality/scoring, bia/tier, sync/envelope |
| 19 | 前端零測試 | 新增 4 個 vitest 測試檔：authStore, client, LocationContext, topology |

### 基礎設施問題

| # | 問題 | 修復 |
|---|------|------|
| 20 | TimescaleDB 無壓縮策略 | Migration 000026：7 天自動壓縮 + 90 天保留 |
| 21 | Help 頁面為純佔位 | 充實 TroubleshootingGuide（6 類問題 + 6 個故障排除指南）和 VideoLibrary（6 部教學影片），3 語言 i18n |

### 安全問題

| # | 問題 | 修復 |
|---|------|------|
| 22 | .gitignore 缺少安全規則 | 合併安全規則（密鑰、憑證、雲服務、Docker、API token 等）+ 保留項目規則 + SQL 排除例外 |
| 23 | 全局 .gitignore 不存在 | 建立 `~/.gitignore_global` 脫敏規則，修正 `*password*`/`*secret*` 過於激進的匹配 |

---

## 二、已設計未實作的功能

### Edge 離線同步 Phase 1（已完成實作）

| 項目 | 狀態 |
|------|------|
| Migration 000027（sync_version, sync_state, sync_conflicts, 工單雙維度） | 已建立 |
| Config（SyncEnabled, EdgeNodeID） | 已加入 |
| SyncEnvelope + Layer 定義 | 已建立 |
| NATS CMDB_SYNC stream | 已加入 |
| Domain services sync_version 遞增 | 已加入 |
| SyncService（事件訂閱 + envelope 發布） | 已建立 |
| Sync API 5 端點 | 已建立 |
| SyncAgent（Edge 節點） | 已建立 |

### Edge 離線同步 Phase 2-4（設計完成，待實作）

RFC 文檔：`/cmdb-platform/docs/design/edge-offline-sync-rfc.md`

| Phase | 內容 | 估計時間 |
|-------|------|---------|
| Phase 2 | 維運同步（工單雙維度、告警同步、衝突 UI） | 3 週 |
| Phase 3 | 盤點與稽核同步 + 監控面板 | 2 週 |
| Phase 4 | 壓力/混沌測試 + 文件 | 2 週 |

---

## 三、發現但未修復的問題（待處理）

### P0 — 高優先級

| # | 問題 | 影響 | 建議修復 |
|---|------|------|---------|
| 1 | **API 直接更新繞過權威檢查** | `PUT /assets/{id}` 不檢查 `asset_field_authorities`，手動修改可覆蓋自動匯入的值，破壞 SSOT | 在 asset.Update() 中加入權威矩陣校驗：若 API 來源優先級 < 欄位當前權威來源 → 拒絕或建衝突 |
| 2 | **品質規則僅評分不阻擋** | 可建立缺少必要欄位的資產，品質分數降低但垃圾資料已入庫 | 在 CreateAsset 前執行必要欄位規則，完整性 < 60 分 → 拒絕建立 |

### P1 — 中優先級

| # | 問題 | 影響 | 建議修復 |
|---|------|------|---------|
| 3 | **衝突佇列無 SLA** | `import_conflicts` 可無限期擱置，資料不一致累積 | 3 天未處理自動升級通知，7 天自動採用高優先級來源 |
| 4 | **盤點差異不自動建工單** | 發現資產位置不對只記錄，不觸發後續動作 | 差異 > 5 筆時自動建立維護工單 |
| 5 | **發現審批無超時** | staging 中的資產可永遠 pending | pending 超過 14 天自動標記 expired |
| 6 | **無定期拉取外部資料** | CMDB 不主動從 Prometheus/ServiceNow 拉取，依賴推送 | 背景 job 每 5 分鐘從 Prometheus adapter 拉取指標 |
| 7 | **SSH collector 不自動判斷虛擬機/實體機** | 掃描到設備後不填 `sub_type`，靠人工標記 | SSH 掃描時檢查 `dmidecode product_name` 是否含 VMware/KVM/Hyper-V |
| 8 | **後端進程無守護** | `go run` 前台運行，terminal 關閉或崩潰後不自動重啟 | 用 systemd service 或 Docker 守護 |

### P2 — 低優先級

| # | 問題 | 影響 | 建議修復 |
|---|------|------|---------|
| 9 | **稽核日誌不防篡改** | DB admin 可刪除 audit_events | DB trigger 禁止 DELETE/UPDATE on audit_events |
| 10 | **資產狀態無資料庫約束** | VARCHAR 欄位可插入無效值 | 加 CHECK constraint 限定合法狀態 |
| 11 | **軟刪除不一致** | assets 有 deleted_at，locations/racks/users 沒有 | 統一加 deleted_at + partial index |
| 12 | **同步 reconciliation 只記日誌** | Edge 延遲超過 1 小時只 warn 不修復 | 自動觸發增量重新同步 |
| 13 | **WebSocket 開發環境不穩定** | 外部 IP 訪問 Vite WS proxy 偶爾失敗 | 正式環境用 Nginx 反向代理解決，開發環境可接受 |
| 14 | **稽核覆蓋率 85%** | 品質規則 CRUD、衝突解決、Webhook 訂閱操作未記錄 | 在對應 handler 中加 recordAudit 調用 |

---

## 四、架構層面的改進建議

### 短期（1-2 週）

| 建議 | 理由 | 影響範圍 |
|------|------|---------|
| API 權威檢查 | SSOT 最大漏洞 | asset service + API handler |
| 品質閘門 | 防止垃圾資料入庫 | asset service |
| SSH 虛擬機自動偵測 | sub_type 自動填充 | ingestion-engine SSH collector |
| 衝突 SLA + 發現 TTL | 防止佇列堆積 | workflow subscriber + cron |

### 中期（1-2 月）

| 建議 | 理由 | 影響範圍 |
|------|------|---------|
| Edge Sync Phase 2-4 | 完成離線同步全功能 | sync package + 前端 |
| 定期外部拉取 | CMDB 保持與外部系統同步 | integration module + cron |
| 稽核防篡改 | 合規要求 | DB migration + trigger |
| 前端 E2E 測試 | 品質保障 | Playwright + CI |

### 長期（3-6 月）

| 建議 | 理由 | 影響範圍 |
|------|------|---------|
| CMDB 聯邦 | 多 Central 跨區域部署 | 架構重設計 |
| 自動化修復 | 品質掃描發現問題 → 自動建工單 → 自動修正 | 全流程整合 |
| 機器學習基線 | 用歷史資料建立正常行為基線，異常自動告警 | prediction module |

---

## 五、本次 Session 產出的文件

| 文件 | 路徑 | 說明 |
|------|------|------|
| 項目完整分析報告 | `docs/project-full-analysis-2026-04-11.md` | 13 章節，覆蓋全平台 |
| 完整性審計報告 | `docs/completeness-audit-2026-04-11.md` | 後端 92% / 前端 78% / 測試 15% |
| Edge 離線同步 RFC | `docs/design/edge-offline-sync-rfc.md` | 14 章節設計文檔，已 Approved |
| Edge Sync Phase 1 實作計劃 | `docs/superpowers/plans/2026-04-13-edge-sync-phase1.md` | 9 個 Task，已全部完成 |
| 本問題匯總 | `docs/session-issues-summary-2026-04-13.md` | 本文件 |

---

## 六、量化改善

| 指標 | Session 前 | Session 後 | 變化 |
|------|-----------|-----------|------|
| 後端測試模組 | 4 | 9 (+sync) | +125% |
| 前端測試檔案 | 0 | 4 | 從零到有 |
| 已知 Bug 修復 | — | 23 項 | — |
| API 端點 | 80+ | 85+（+5 sync） | +6% |
| 通知觸發事件 | 3 | 7 | +133% |
| 文件產出 | — | 5 份 | — |
