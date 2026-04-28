# CMDB Platform 功能補全計劃

> **建立日期**：2026-04-28
> **基準分支**：`worktree-fixer-predictive`
> **方法**：5 agent 平行業務分析 + 2 agent 平行 sizing + ROADMAP/14-WARN 比對
> **原則**：先填已知技術債（D-Day 0 reviewer 標的 14 WARN）→ 完成 ROADMAP M1 已立案的核心功能 → 真功能缺口（最小 MVP）→ 結構性 refactor。明確標示 **不做** 的項目以防範過度開發。
> **總工時估算**：約 22-30 工作天（單線）；雙人並行約 12-15 天。

---

## 一、整體策略

### 必補（已知缺陷）
14 WARN debt + 11 個前端 placeholder + 4 個 ROADMAP M1 已立案項

### 選擇性補（提升完整度）
3 個真功能缺口：MCP 多租戶 / 壽命可配置 / 工單溢出修復

### 明確不做（YAGNI / 違反架構決定）

| 項目 | 不做理由 |
|---|---|
| 外部 webhook inbound 告警 | 中小機房自建夠用，無具體租戶需求即跳過 |
| Metric Sources Prometheus pull exporter | 心跳模式已是合理 MVP，違反 YAGNI |
| 真 ML 訓練管線（PredictFailure） | D6 決定：數據量不足，Phase 2.12 已刪 stub，**不要復活** |
| Edge / 多區同步 | D5 永久取消（user 2026-04-28 確認「以後也不需要」） |
| TroubleshootingGuide 改後端管理知識庫 | 文件性內容靜態即可，YAGNI |
| asset_type_config 獨立表 | tenant_settings JSONB 一欄解決，免新表 |
| sqlc analyzer AST 解析 SQL 字串 | 收益遞減，改補 integration test 更實際 |

---

## 二、分波執行計劃

### Wave 0 — 安全與啟動保證（1-2 天，S）

**目標**：消除 fail-open / 啟動弱默認 / SQL 注入風險。能單一 commit 修一項。

| # | 項目 | 檔案位置 | 修法 |
|---|---|---|---|
| W0.1 | Auth Redis 不可達 fail-closed | `internal/middleware/auth.go:103-104,119-120` | `AUTH_FAIL_POLICY` 預設改 `closed`，回 503 |
| W0.2 | `/readyz` 補 NATS 健檢 | `internal/api/health.go` | `HealthHandler` 加 `nc *nats.Conn`，readyz 檢查 IsConnected |
| W0.3 | ingestion 零金鑰啟動失敗 | `ingestion-engine/app/config.py:72` | 非 dev 環境 `credential_encryption_key` 全零 → raise 終止啟動 |
| W0.4 | SQL 識別字 sanitize | `domain/sync/service.go:102,171`、`agent.go:471,492`、`asset/service.go:274`、`maintenance/service.go:482`、`topology/service.go:356` | `fmt.Sprintf("...%s...", table)` → `pgx.Identifier{table}.Sanitize()` |
| W0.5 | CI master branch trigger | `.github/workflows/ci.yml:5,7` | 加 `master` 到 push/pull_request branches |

**驗收**：
- [ ] `AUTH_FAIL_POLICY=closed` 預設行為測試通過
- [ ] kill NATS → /readyz 回 503
- [ ] ingestion 啟動時 `CREDENTIAL_ENCRYPTION_KEY=00...` 直接 panic
- [ ] SQL 注入 fuzz test（單元）pass
- [ ] master push 觸發 CI

---

### Wave 1 — ROADMAP M1 已立案（5-7 天，M-L）

> 這是**已被 D-Day 0 決定要做**的，不是新規劃。

| # | 項目 | 範圍 |
|---|---|---|
| W1.1 | **OpenAPI Block gate** | spec drift PR 直接擋下；修完最後幾個 drift 後切 strict（D3） |
| W1.2 | **Business Service 實體 + asset N:M**（D1） | 新表 `services`、`service_asset_dependencies`；BIA `bia_assessments` 加 `service_id` FK；前端 `Services.tsx` 補 CRUD（已有頁面骨架） |
| W1.3 | **Discovery Review Gate** | 移除 silent auto_merge，改為一律進 `import_conflict` 待審；保留 `mode=auto` 旗標讓信任的 collector 仍可自動入 |
| W1.4 | **status 欄位 CHECK constraints** | `assets.status / racks.status / work_orders.status / incidents.status / inventory_tasks.status` 全部加 DB-level CHECK，避開字串污染 |
| W1.5 | **跨頁 nav links** | 11 處業務缺陷之 #5：asset detail → rack / location / open work orders 互相導航 |

**驗收**：
- [ ] `make check-api-routes` strict 通過 + CI 阻擋 drift
- [ ] Service N:M 可建並透過 BIA 計算受影響資產
- [ ] Discovery 結果 100% 走 review queue（除 mode=auto）
- [ ] 5 個 status 欄位 CHECK 約束生效，違規 INSERT 失敗
- [ ] 4 個跨頁導航 link 可點擊

---

### Wave 2 — 11 個前端 placeholder 接真實 API（3-4 天，M）

**目標**：消除 README 之外的所有「假數據」，避免商業展示時被質疑。

| # | 頁面 | 動作 |
|---|---|---|
| W2.1 | `Dashboard.tsx:244` 資產趨勢 | 接 `GET /assets?count_by=created_at` 時序聚合 |
| W2.2 | `Dashboard.tsx:426` 機架熱力圖 | 接 `GET /racks?include=power_kw,temp` |
| W2.3 | `Dashboard.tsx:15-70` BIA / HEATMAP / EVENTS 假資料 | 移除 `fallbacks/*.ts`，全接 API；空 state 設計優於假資料 |
| W2.4 | `EnergyMonitor.tsx:351,503,608,622` 4 處 | peak 日期、UPS autonomy、機架熱圖、power_events 流 — 後端已有 metrics 表，補 4 個 query |
| W2.5 | `InventoryItemDetail.tsx:82,216` | asset link + 不一致詳情接 `/inventory/items/{id}` |
| W2.6 | `predictive/TimelineTab.tsx:140,161` | 機架佔用率 + 環境指標接既有 endpoint |
| W2.7 | `AssetLifecycleTimeline.tsx:238` compliance scan | **後端缺**：新增 `GET /assets/{id}/compliance-scan` 取最近一次掃描結果（從 audit_events filter） |

**驗收**：每頁開啟時 Network tab 有對應 API 呼叫，無 hardcode 數字。

---

### Wave 3 — 真功能缺口（最小 MVP）（2-3 天，S+M）

| # | 項目 | 工作量 | 範圍 |
|---|---|---|---|
| W3.1 | **MCP 多租戶切換**（Gap 1） | S | `internal/mcp/tools.go:81-91` 7 個 tool 加 `tenant_id` 選填 arg；`defaultTenantID` 改先讀 arg → fallback；補 UUID 驗證 |
| W3.2 | **壽命設定可配置化**（Gap 5） | M | `tenant_settings` 加 JSONB `asset_lifespan_config`；`impl_prediction_upgrades.go:87-92` map 改先 SELECT；新 `GET/PUT /settings/asset-lifespan` API；前端 SystemSettings 頁加區塊 |
| W3.3 | **TroubleshootingGuide 計數動態化**（Gap 4） | S | `TroubleshootingGuide.tsx:32-39` `CATEGORIES` count 改呼叫 `GET /incidents?group_by=category` 或 `GET /audit-events?group_by=module`；`COMMON_ISSUES` 維持靜態（文件性內容） |

**驗收**：
- [ ] MCP `search_assets` 帶 tenant_id="UUID-A" 與 "UUID-B" 結果不同
- [ ] 改變租戶 lifespan 設定後預測升級結果反映
- [ ] TroubleshootingGuide 數字隨真實 incident 變動

---

### Wave 4 — 結構性 refactor（5-6 天，L）

> 主因：`main.go` 692 行 30 天改 86 次、3 個 page > 900 行、bundle 過重。Wave 0-3 完成後再做，避免邊改邊衝突。

| # | 項目 | 檔案 | 拆分目標 |
|---|---|---|---|
| W4.1 | `cmd/server/main.go` 拆分（WARN #11） | 692 行 | → `bootstrap.go`（DB/Redis/NATS init）+ `routes.go`（路由註冊）+ 保留 `main.go` < 200 行 |
| W4.2 | `PredictiveHub.tsx` 1416 行拆 | 1 → 7 檔 | 按 tab 拆：`tabs/RUL.tsx` / `tabs/Capex.tsx` / `tabs/Refresh.tsx` 等 |
| W4.3 | `RackDetailUnified.tsx` 1227 行拆 | 1 → ~5 檔 | 按 section（U-grid / power / inventory / sensors / activity） |
| W4.4 | `SensorConfiguration.tsx` 902 行拆 | 1 → 4 個組件 | rule list / threshold form / preview / history |
| W4.5 | Bundle splitting（WARN #10） | `vite.config.ts` | elkjs (1.4MB) + xlsx (493KB) 改 dynamic import；只在 Topology / Excel import 時載入 |

**驗收**：
- [ ] `main.go` < 250 行，build pass
- [ ] 3 個前端頁面拆完，每檔 < 400 行
- [ ] 首屏 JS bundle gzipped < 300KB（目前估 600KB+）
- [ ] Lighthouse performance score 提升 ≥ 10 分

---

### Wave 5 — 維運完整性（3-4 天，M）

| # | 項目 | 範圍 |
|---|---|---|
| W5.1 | `DB MaxConns` 環境變數化（WARN #4） | `internal/platform/database/postgres.go:78-79` 讀 `DB_MAX_CONNS` / `DB_MIN_CONNS` |
| W5.2 | Workflow tx commit 補完（WARN #9） | `domain/workflows/auto_workorders_governance.go` 等：每個 `pool.Begin` 確保有 `Commit()` 或 defer rollback |
| W5.3 | SLO Prometheus rules + Grafana dashboard（WARN #8） | `deploy/prometheus/rules/slo.yml` 三條 SLO 對應的 burn-rate 規則；Grafana JSON dashboard import |
| W5.4 | Python `requirements.lock`（WARN #7） | `ingestion-engine/` `pip-compile` 產 lock；CI 加 `pip install -r requirements.lock` |
| W5.5 | `operator_id` FK 設計收尾（WARN #14） | 採方案 A：新增 `system` 用戶於 `users` 表，UUID 固定；audit_events 引用該 ID 取代 `uuid.Nil` |

**驗收**：
- [ ] `DB_MAX_CONNS=100` 生效
- [ ] kill 中途 workflow → DB 無孤兒 row
- [ ] Grafana 顯示三條 SLO burn-rate 圖
- [ ] `pip install` from lock 能完整重建環境
- [ ] `audit_events.operator_id` 不再有 `uuid.Nil`

---

### Wave 6 — UI 收尾與測試覆蓋（3-4 天，M）

| # | 項目 | 範圍 |
|---|---|---|
| W6.1 | `/inventory /maintenance /monitoring` 溢出 217px | **不要再嘗試一行修法**（Wave N 4 次失敗）。改採：(a) 記錄三頁實際 max-width；(b) MainLayout sidebar 確認 ml-56 = 14rem；(c) 各頁 container 加 `min-w-0 overflow-x-auto`；(d) 多 viewport (1280/1440/1920) 視覺驗收 |
| W6.2 | identity / workflows / eventbus 測試覆蓋率 → 40%（WARN #5） | 三個 package 補單元測試；先補 happy path 再補 edge case |
| W6.3 | sqlc 跨租戶 integration test 加 2-3 cases | `roles` / `bia` / `quality` 跨 tenant 讀寫測試，防範未來盲點 |
| W6.4 | E2E Playwright 在 CI 重啟 | 確認 15 條 spec pass，`master` 與 PR 都跑 |

**驗收**：
- [ ] 1280/1440/1920 三 viewport 三頁無橫向 scroll
- [ ] `go test -cover ./internal/domain/identity/...` ≥ 40%
- [ ] CI 上 Playwright 綠燈

---

## 三、總工時與里程碑

| Wave | 主軸 | 工時 | 里程碑 |
|---|---|---|---|
| 0 | 安全 fail-closed | 1-2d | v3.4.0：production-ready 安全基線 |
| 1 | M1 已立案功能 | 5-7d | v3.5.0：Business Service 入庫 |
| 2 | placeholder 真實化 | 3-4d | v3.5.1：UI 無假數據 |
| 3 | 真功能缺口 MVP | 2-3d | v3.6.0：MCP 多租戶 + 壽命可配 |
| 4 | 結構性 refactor | 5-6d | v3.7.0：bundle 減重、巨檔拆分 |
| 5 | 維運完整性 | 3-4d | v3.7.1：SLO/lockfile/FK 修齊 |
| 6 | UI 收尾 + 覆蓋率 | 3-4d | v3.8.0：可宣稱 GA |

**總計**：22-30 工作天（單線）；雙人並行（前端 / 後端拆分）約 12-15 天。

---

## 四、執行注意事項

1. **每完成一項即 commit + push + tag**（feedback_auto_commit_push.md + feedback_auto_semver_tag.md）
2. **每次 backend 改完跑完整 test battery**（feedback_completeness_test_after_change.md：build/vet/tenantlint/unit/integration/tsc/vitest/build/migration）
3. **遷移檔建立後立即在 running DB 執行**（feedback_run_migrations.md）
4. **後端 binary 改完要 rebuild + restart**（feedback_rebuild_restart_after_change.md）
5. **W6.1 溢出修復禁止再做「一行 layout 修法」**（project_overflow_regression_open.md），必須多 viewport 視覺驗收
6. **不要在這個計劃內擴張**：M2-M5 ROADMAP（ServiceNow / Jira / CAB / Teams / PDU SNMP / Prophet / RAG）**留給下一個 Milestone**，本次補全週期不開新功能線

---

## 五、後續（M2 之後，僅供路標，不在本計劃 scope）

ROADMAP 上仍掛著但**目前 user 場景不需立刻做**的：
- ServiceNow / Jira outbound adapters
- Service-centric incident aggregation（3 alerts → 1 incident）
- CAB approval gate / Teams 實體
- PDU/UPS SNMP collectors（APC/Schneider/Eaton）
- Prophet 容量預測 / 碳足跡報告
- LLM-RCA + RAG

這些屬於「擴張型」需求，補完 Wave 0-6 之後可依實際租戶反饋決定優先序。

---

## 附錄 A：14 個 WARN debt 對照（出處 `docs/plans/2026-04-22-prod-readiness-remediation.md`）

| # | WARN 標題 | 對應 Wave |
|---|---|---|
| 1 | auth middleware fail-closed | W0.1 |
| 2 | readyz 缺 NATS 健康檢查 | W0.2 |
| 3 | ingestion-engine zero-key 默認值 | W0.3 |
| 4 | DB MaxConns 硬編碼 | W5.1 |
| 5 | 測試覆蓋率不足（identity/workflows/eventbus） | W6.2 |
| 6 | fmt.Sprintf 拼接表名 | W0.4 |
| 7 | Python 依賴無 lockfile | W5.4 |
| 8 | SLO 未正式定義 + 無 burn-rate alert | W5.3 |
| 9 | workflows 事件鏈事務不完整 | W5.2 |
| 10 | 前端 bundle 過大 | W4.5 |
| 11 | main.go 692 行 | W4.1 |
| 12 | CI branch trigger 缺 master | W0.5 |
| 13 | tenantlint 未接 CI | **已完成**（W0 略過，CI baseline-gated 已在線） |
| 14 | operator_id = uuid.Nil FK 設計 | W5.5 |

## 附錄 B：D-Day 0 架構決定（不可違背）

| 決定 | 內容 |
|---|---|
| D1 | services 表代表業務功能；asset N:M service；service 間依賴推遲到 Wave 6 |
| D2 | README 不承諾 Edge offline-capable；Predictive 標 beta |
| D3 | OpenAPI gate Block 模式（spec drift = PR fail） |
| D4 | Spec 模板路徑 `db/specs/_template.md`；負責人 sign-off 才能進 implementation |
| D5 | Edge 離線模組永久砍掉，不做真離線寫佇列 |
| D6 | Predictive AI Phase 1 不做真 ML 訓練；RUL 重命名為 "Asset Health Score"；加 RAG 增強 LLM-RCA |
| D7 | Energy 模組 Phase 2 用 Prophet 統計預測，不是 ML |
| D8 | 11 項業務缺陷全修，除 #4 Edge offline |
