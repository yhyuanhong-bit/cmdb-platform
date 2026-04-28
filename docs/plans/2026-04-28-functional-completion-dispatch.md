# CMDB Platform 補全計劃 — Agent 分工與並行策略

> **配套文件**：`2026-04-28-functional-completion-plan.md`
> **目的**：將 7 個 Wave 拆解為可並行 / 必須序列的 agent 任務，最大化吞吐量。
> **建立日期**：2026-04-28

---

## 一、全局並行原則

**並行安全 = 不修同檔 + 不依賴對方輸出**

| 衝突類型 | 處理 |
|---|---|
| 同檔修改 | 必須序列（單 agent 或 sequential 排隊） |
| 同 migration 編號 | 必須預先分配編號才能 parallel |
| 同 OpenAPI spec section | 序列 |
| 跨 frontend / backend | 通常可並行 |
| 跨 domain package | 可並行 |
| 結構性 refactor 中的單檔拆分 | 單 agent 完成（high cohesion） |

**worktree 使用建議**：Wave 4 結構性 refactor **必須用 `isolation: worktree`**；其餘 wave 多 agent 改不同檔可共用主分支。

---

## 二、各 Wave 分工表

### Wave 0 — 安全與啟動保證（4 並行 agent）

| Task | 檔案 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W0.1 + W0.2 | `auth.go` + `health.go`（合併，同 internal/api 域） | Backend Architect | A | 0.5d |
| W0.3 | `ingestion-engine/app/config.py` | general-purpose (Python) | B | 0.25d |
| W0.4 | 5 個 domain Go 檔案 SQL sanitize | Security Engineer | C | 0.5d |
| W0.5 | `.github/workflows/ci.yml` | DevOps Automator | D | 0.1d |

**並行度**：4；**分支策略**：共用 `worktree-fixer-predictive`，commit 互不衝突
**Reviewer follow-up**：go-reviewer + python-reviewer 各 1 次（序列，1h）

---

### Wave 1 — ROADMAP M1（6 並行 agent）

**前置動作（5 分鐘人工或 orchestrator）**：預分配 migration 編號
- W1.2 → `000022_business_services.up.sql`
- W1.4 → `000023_status_check_constraints.up.sql`

| Task | 檔案/範圍 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W1.1 OpenAPI gate | `api/openapi.yaml` + `ci.yml` + 修剩餘 drift | DevOps Automator | A | 1d |
| W1.2-backend Business Service | migration 000022 + 新 `domain/service/` + `impl_services.go` | Backend Architect | B | 2d |
| W1.2-frontend Services CRUD | `pages/Services.tsx` + 新 hook + form | Frontend Developer | C | 1.5d |
| W1.3 Discovery Review Gate | `ingestion-engine/pipeline/processor.py` + `internal/api/impl_discovery.go` | general-purpose（跨語言） | D | 1.5d |
| W1.4 status CHECK | migration 000023 純 SQL | Database Optimizer | E | 0.5d |
| W1.5 跨頁 nav links | 4-5 個前端 page | Frontend Developer | F | 1d |

**並行度**：6；**worktree**：W1.2-backend 建議 `isolation: worktree`（新 domain 改動大）
**依賴**：W1.2-frontend 需等 W1.2-backend 的 OpenAPI 產出 generated client（半天時差，可用 stub 並行起跑）

---

### Wave 2 — Placeholder 接真實 API（5 並行 agent）

| Task | 檔案 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W2.1 + W2.2 + W2.3 | **Dashboard.tsx 三項合併** — 同檔禁止並行 | Frontend Developer | A | 1d |
| W2.4 | `EnergyMonitor.tsx`（4 處 placeholder） | Frontend Developer | B | 1d |
| W2.5 | `InventoryItemDetail.tsx` | Frontend Developer | C | 0.5d |
| W2.6 | `predictive/TimelineTab.tsx` | Frontend Developer | D | 0.5d |
| W2.7-backend | 新 `GET /assets/{id}/compliance-scan` | Backend Architect | E1 | 0.5d |
| W2.7-frontend | `AssetLifecycleTimeline.tsx` 接 API | Frontend Developer | E2（等 E1） | 0.25d |

**並行度**：5（W2.7 內 backend → frontend 序列）

---

### Wave 3 — 真功能缺口（4 並行 agent）

| Task | 檔案 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W3.1 MCP 多租戶 | `internal/mcp/tools.go` + tests | Backend Architect | A | 0.5d |
| W3.2-backend lifespan config | migration + `impl_prediction_upgrades.go` + 新 settings API | Backend Architect | B | 1d |
| W3.2-frontend lifespan UI | `SystemSettings.tsx` 加 section | Frontend Developer | C（等 B） | 0.5d |
| W3.3 TroubleshootingGuide | `TroubleshootingGuide.tsx` 計數動態化（可能新 group_by API） | Frontend Developer | D | 0.5d |

**並行度**：4（W3.2 內部序列）

---

### Wave 4 — 結構性 refactor（5 並行 worktree agent）

> **規則**：每個拆分由**單一 agent** 完成；agent 之間用 `isolation: worktree` 隔離

| Task | 檔案 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W4.1 main.go 拆 | `cmd/server/main.go` → bootstrap + routes | code-simplifier (Go) | A (worktree) | 1d |
| W4.2 PredictiveHub.tsx 拆 7 檔 | `pages/predictive/*` | code-simplifier (TS) | B (worktree) | 1.5d |
| W4.3 RackDetailUnified.tsx 拆 5 檔 | `pages/rack-detail/*` | code-simplifier (TS) | C (worktree) | 1.5d |
| W4.4 SensorConfiguration.tsx 拆 4 檔 | `pages/sensor-configuration/*` | code-simplifier (TS) | D (worktree) | 1d |
| W4.5 Bundle splitting | `vite.config.ts` + 動態 import elkjs/xlsx | Frontend Developer | E (主分支) | 0.5d |

**並行度**：5；**合併策略**：完成順序 W4.5 → W4.1 → W4.2 → W4.3 → W4.4，每次合併後跑完整 test battery
**Reviewer follow-up**：每個 refactor PR 過 typescript-reviewer 或 go-reviewer

---

### Wave 5 — 維運完整性（5 並行 agent）

| Task | 檔案 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W5.1 DB MaxConns env var | `internal/platform/database/postgres.go` | Backend Architect | A | 0.25d |
| W5.2 Workflow tx commit | `domain/workflows/*` | silent-failure-hunter → Backend Architect 修復 | B | 1d |
| W5.3 SLO Prometheus + Grafana | `deploy/prometheus/rules/slo.yml` + Grafana JSON | SRE | C | 1.5d |
| W5.4 Python lockfile | `ingestion-engine/requirements.lock` + CI | DevOps Automator | D | 0.5d |
| W5.5 operator_id FK | migration + `users` seed system user + audit_events 引用 | Database Optimizer + Backend Architect | E | 1d |

**並行度**：5

---

### Wave 6 — UI 收尾與測試覆蓋（6 並行 agent）

| Task | 檔案/範圍 | 推薦 agent | 並行組 | 工作量 |
|---|---|---|---|---|
| W6.1 Layout overflow 修復 | `MainLayout.tsx` + 3 頁 — **禁止並行** | Frontend Developer + Evidence Collector（多 viewport 截圖） | A | 1.5d |
| W6.2-identity coverage | `domain/identity/*_test.go` | go-reviewer + general-purpose | B | 1d |
| W6.2-workflows coverage | `domain/workflows/*_test.go` | go-reviewer + general-purpose | C | 1d |
| W6.2-eventbus coverage | `eventbus/*_test.go` | go-reviewer + general-purpose | D | 1d |
| W6.3 sqlc 跨租戶 integration test | `domain/{roles,bia,quality}/*_integration_test.go` | API Tester | E | 0.5d |
| W6.4 E2E Playwright CI | `.github/workflows/ci.yml` + `playwright.config.ts` | e2e-runner | F | 0.5d |

**並行度**：6

---

## 三、跨 Wave 並行機會

| 重疊組 | 條件 | 風險 |
|---|---|---|
| Wave 0 + Wave 1.4 + Wave 1.5 | 都不修同檔 | 低；可同時起跑 |
| Wave 2 + Wave 3 | 前端 placeholder 與 MCP/lifespan 互不影響 | 低 |
| Wave 5 + Wave 6 | 維運與測試覆蓋無衝突 | 低 |
| **Wave 4 必須序列在 Wave 0-3 後** | refactor 涉及 main.go / 巨型 page，與功能改動 merge 失敗率高 | 高 |

---

## 四、不可並行 / 必須單獨 agent 的項目

| 項目 | 原因 |
|---|---|
| W2.1 + W2.2 + W2.3 Dashboard.tsx 三項 | 同檔，必序列 |
| W4.1-W4.4 任一 page 拆分**內部** | 單檔重構，high cohesion；禁止拆給多個 agent |
| W6.1 Layout overflow | 視覺一致性決策必須單一決策者；歷史上 Wave N 4 次失敗 |
| OpenAPI spec drift 修復（W1.1） | 同 spec 文件，序列化 |
| Migration 編號分配 | 必須先協調再 dispatch |

---

## 五、風險與協調點

| 風險 | 對策 |
|---|---|
| Migration 編號衝突 | 每 Wave 開始前由 orchestrator 預分配 0000XX 編號 |
| OpenAPI spec 多人加 endpoint | W1.2 / W2.7 / W3.2 都新增 endpoint；建議由一個 agent 先 dummy-stub 加 3 個 path，再各自填內容 |
| i18n keys 衝突 | 前端 agent 各自加 key 易合併衝突；建議集中加完再 dispatch |
| Worktree 殘留 | Wave 4 用完立即 ExitWorktree，不留多餘分支 |
| Tag/release 節點 | 每 Wave 結束打 tag（W0→v3.4.0, W1→v3.5.0, W2→v3.5.1, W3→v3.6.0, W4→v3.7.0, W5→v3.7.1, W6→v3.8.0） |

---

## 六、推薦執行時程

### 單人 protocol（22-30 工作天）

| Day | 內容 | 並行 agent 數 |
|---|---|---|
| Day 1 | Wave 0（4 agent 並行）→ tag v3.4.0 | 4 |
| Day 2-7 | Wave 1（6 agent 並行）→ tag v3.5.0 | 6 |
| Day 8-10 | Wave 2 + Wave 3 重疊 → tag v3.5.1, v3.6.0 | 9 |
| Day 11-15 | Wave 4（5 agent worktree）→ tag v3.7.0 | 5 |
| Day 16-18 | Wave 5 + Wave 6 重疊 → tag v3.7.1, v3.8.0 | 11 |

### 雙人並行 protocol（12-15 工作天）

- A 線：後端 + 維運（Wave 0 後端組 → Wave 1.2-backend → Wave 3 後端 → Wave 4.1 → Wave 5）
- B 線：前端（Wave 0 前端組 → Wave 1.5/1.2-frontend → Wave 2 → Wave 3 前端 → Wave 4.2-4.5 → Wave 6.1）

---

## 七、Agent 類型參考表

| Agent type | 適合任務 |
|---|---|
| Backend Architect | Go domain / handler / migration 設計 |
| Frontend Developer | React 頁面、hook、UI 邏輯 |
| Database Optimizer | migration、index、query 優化 |
| Security Engineer | SQL 注入修復、auth fail-closed、ssrf |
| DevOps Automator | CI workflow、Docker、Grafana provisioning |
| SRE | SLO Prometheus rules、burn-rate alert |
| code-simplifier | 巨型檔案拆分（Wave 4 主力） |
| silent-failure-hunter | Workflow tx commit 漏洞偵測 |
| API Tester | integration test、跨租戶 case |
| e2e-runner | Playwright spec |
| Evidence Collector | 多 viewport 截圖驗收（Wave 6.1） |
| Accessibility Auditor | 視覺/鍵盤可用性檢查 |
| general-purpose | Python 任務、跨語言任務 |
| go-reviewer / typescript-reviewer / python-reviewer | 完成後 code review |

---

## 八、Dispatch checklist（每 Wave 開始前）

- [ ] Migration 編號預分配
- [ ] OpenAPI 預先 stub 新 endpoint
- [ ] i18n key 集中加
- [ ] worktree 是否需要
- [ ] reviewer agent 排程
- [ ] tag 計劃確認
- [ ] 完成 / 驗收標準寫入每個 agent prompt
