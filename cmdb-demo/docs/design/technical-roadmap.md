# IronGrid Enterprise CMDB + AIOps 平台 -- 技術路線圖

> 文件版本: v1.0
> 日期: 2026-03-28
> 狀態: Proposed
> 目標讀者: 技術決策者、架構師、工程主管
> 範圍: 從純前端原型到生產級企業平台的完整演進路線

---

## 目錄

1. [項目現狀總結](#1-項目現狀總結)
2. [推薦技術棧](#2-推薦技術棧)
3. [分階段實施路線圖](#3-分階段實施路線圖)
4. [資料庫架構設計](#4-資料庫架構設計概要)
5. [API 架構設計](#5-api-架構設計概要)
6. [風險與挑戰](#6-風險與挑戰)
7. [團隊建議](#7-團隊建議)
8. [預估時間表](#8-預估時間表)

---

## 1. 項目現狀總結

### 1.1 前端規模與結構

IronGrid CMDB 目前為一個 **純前端原型**，包含完整的 UI 骨架但無後端支撐。

| 維度 | 現狀 |
|------|------|
| 頁面總數 | 39 個頁面（含 4 個位置階層頁面 + 35 個功能頁面） |
| 位置階層頁面 | GlobalOverview, RegionOverview, CityOverview, CampusOverview |
| 路由結構 | React Router v7 嵌套路由，全部使用 `lazy()` 動態載入 |
| 元件庫 | 5 個共用元件（Icon, StatCard, StatusBadge, LocationBreadcrumb, LanguageSwitcher） |
| 佈局 | 單一 MainLayout：左側 sidebar + 頂部 tab bar + 內容區 |
| 資料來源 | 所有頁面使用 inline mock data（硬編碼於各 `.tsx` 檔案內） |

### 1.2 當前技術棧

| 層級 | 技術 | 版本 |
|------|------|------|
| 框架 | React | 19.2.4 |
| 語言 | TypeScript | 6.0.2 |
| 建置工具 | Vite | 8.0.3 |
| 樣式 | Tailwind CSS（含 @tailwindcss/vite 插件） | 4.2.2 |
| 路由 | react-router-dom | 7.13.2 |
| 國際化 | i18next + react-i18next + 瀏覽器語言偵測 | 26.0.1 / 17.0.1 |
| 圖表 | Recharts | 3.8.1 |
| 模組系統 | ESM (type: "module") | -- |

### 1.3 已完成的功能

**已正常運作：**

- 完整的 UI 視覺系統（深色 Industrial Brutalism 主題，自訂 Material Design 色彩 token）
- 三語國際化（EN / zh-CN / zh-TW），`zh-TW` 為 fallback 語言
- 位置階層鑽取導航（Global -> Country -> Region -> City -> Campus），含 URL 參數化與麵包屑
- LocationContext 全域狀態管理（含 localStorage 持久化）
- 位置資料聚合引擎（按 IDC/Campus/City/Region/Country 聚合 KPI）
- 39 個頁面的完整 UI 渲染，涵蓋：資產管理、機櫃管理、盤點、監控告警、維護工單、預測性 AI、稽核、系統設定
- 響應式 sidebar 導航，含可展開/收合的二級選單
- 頂部全域搜尋列（CMD+K 快捷鍵提示，UI 已就緒）
- Welcome 引導頁面（獨立佈局，不含 sidebar）

**設計語言特徵：**

- 字型系統：`font-headline` 用於標題、全大寫 `tracking-wider` 用於標籤
- 卡片樣式：`rounded-lg bg-surface-container p-5`
- 色彩 token：`surface`, `surface-container`, `surface-container-low`, `on-surface`, `on-surface-variant`, `primary`, `error`, `tertiary` 等
- 圖標：Material Symbols Outlined

### 1.4 關鍵缺口

| 缺口 | 影響 | 優先級 |
|------|------|--------|
| **無後端服務** | 所有資料為 mock，無法進行任何真實業務操作 | P0 |
| **無身份認證** | 無登入機制、無 RBAC、sidebar 硬編碼「系統管理員」 | P0 |
| **無即時資料** | 告警、感測器讀數、KPI 均為靜態假資料 | P1 |
| **無 AI 能力** | PredictiveHub 的 AI 對話為硬編碼文字，無 LLM 整合 | P1 |
| **Mock 資料散落** | 每個頁面各自定義 mock data，無統一資料層 | P1 |
| **路由未參數化** | `/assets/detail`, `/racks/detail` 等未使用 `:id` 參數 | P2 |
| **共用元件不足** | 僅 5 個共用元件，DataTable/FilterBar/Modal/Toast 等在各頁面重複實作 | P2 |
| **無表單驗證** | 表單頁面（AddNewRack, AddMaintenanceTask）缺乏系統化驗證 | P2 |
| **無測試** | 零測試覆蓋率，無單元測試、無 E2E 測試 | P2 |
| **Icon 元件重複定義** | Dashboard.tsx、PredictiveHub.tsx 等頁面各自重新定義 `Icon` 函數，未使用 `components/Icon.tsx` | P3 |
| **無全域狀態管理** | 僅有 LocationContext，缺乏使用者狀態、通知狀態、主題狀態的統一管理 | P2 |

### 1.5 Mock 資料分佈分析

目前的 mock 資料模式具有一致性，可作為資料庫 schema 設計的依據：

| 頁面 | 核心資料模型 | 欄位數 |
|------|-------------|--------|
| AssetManagementUnified | Asset（id, name, type, vendor, model, location, biaLevel, status, metrics） | ~12 |
| RackManagement | Rack（id, name, location, usedU, totalU, powerCapacity, powerUsage, status） | ~8 |
| MonitoringAlerts | Alert（timestamp, severity, description, serialNumber, status） | ~6 |
| WorkOrder | WorkOrderItem（id, title, status, requestor, ciName, reason, priority） | ~8 |
| PredictiveHub | 預測資產（name, type, failureDate, rulDays, severity）、AI 對話訊息 | ~7 |
| locationMockData | LocationTree（Country->Region->City->Campus->IDC，含 KPI） | 層級結構 |

---

## 2. 推薦技術棧

### 2.1 前端增強

#### 狀態管理：Zustand v5

**推薦：Zustand** 而非 Redux Toolkit。

| 考量 | Zustand | Redux Toolkit |
|------|---------|---------------|
| 學習曲線 | 低，API 極簡 | 中，需理解 slice/thunk/middleware |
| Bundle 大小 | ~1.1KB gzipped | ~11KB gzipped |
| 與現有程式碼相容性 | 可逐步引入，與 Context 共存 | 需要全面重構 |
| DevTools | 支援（透過 middleware） | 原生支援 |
| TypeScript 支援 | 優秀，型別推斷自然 | 優秀，但模板較多 |

**理由：** 目前專案僅有一個 Context（LocationContext），狀態管理需求相對簡單。Zustand 的漸進式引入策略允許我們先為 auth/notification/theme 建立 store，不需要重構既有的 LocationContext。當系統複雜度增長後，Zustand 的 slice pattern 也足以應對。

**建議建立的 Store：**
- `useAuthStore` -- 使用者身份、JWT token、角色與權限
- `useNotificationStore` -- 未讀通知計數、即時告警佇列
- `usePreferenceStore` -- 主題、語言、sidebar 收合狀態
- `useLocationStore` -- 遷移現有 LocationContext（Phase 1 後期）

#### 資料擷取：TanStack Query (React Query) v5

```
安裝版本建議：@tanstack/react-query ^5.62
```

**核心價值：**
- 自動快取與背景重新擷取（stale-while-revalidate）
- 分頁與無限滾動內建支援（`useInfiniteQuery`）
- 樂觀更新（optimistic updates）用於工單狀態變更、告警確認
- 請求去重（相同查詢自動合併）
- 離線支援與重試機制
- DevTools 用於除錯快取狀態

**架構模式：**
```
src/
  api/
    client.ts          -- axios/fetch 封裝，自動附加 JWT、處理 401
    endpoints/
      locations.ts     -- Location CRUD API 函數
      assets.ts        -- Asset CRUD API 函數
      alerts.ts        -- Alert API 函數
      ...
  hooks/
    useLocations.ts    -- useQuery/useMutation 封裝
    useAssets.ts
    useAlerts.ts
    ...
```

#### 即時通訊：Socket.io Client v4

**推薦：Socket.io** 而非原生 WebSocket。

**理由：** Socket.io 提供自動重連、房間（room）機制、斷線緩衝等企業級特性。對於 CMDB 場景，不同 IDC 的告警可映射為不同 room，使用者僅訂閱其位置上下文的即時事件。

**應用場景：**
- 即時告警推送（新告警、狀態變更）
- 感測器遙測資料串流（溫度、濕度、功耗）
- Dashboard KPI 即時更新
- 工單狀態變更通知
- 盤點進度即時回報

#### 圖表與視覺化：Recharts + D3.js

- **Recharts v3**（已安裝）：保持用於標準圖表（折線圖、長條圖、圓餅圖、面積圖）
- **D3.js v7**（新增）：用於拓撲圖（AlertTopologyAnalysis）、力導向圖（CI 關係圖）、樹狀圖（位置階層視覺化）
- 考慮引入 **@visx/visx**（Airbnb 出品）作為 D3 的 React 封裝，降低直接操作 DOM 的需求

#### 表單管理：React Hook Form v7 + Zod v3

```
安裝版本建議：react-hook-form ^7.54, zod ^3.24, @hookform/resolvers ^3.9
```

**適用頁面：**
- AddNewRack -- 新增機櫃表單
- AddMaintenanceTask -- 新增維護任務表單
- WorkOrder -- 工單建立/審批表單
- SystemSettings -- 系統設定表單
- UserProfile -- 使用者資料編輯
- SensorConfiguration -- 感測器閾值設定
- AssetDetail -- 資產資料編輯

**價值：** 非受控表單（uncontrolled form）效能優異，搭配 Zod schema 可同時做前端驗證與 TypeScript 型別推斷，且 Zod schema 可與後端 API 的 schema 共享（若後端也使用 Zod 或相容格式）。

#### 共用元件庫擴充

目前僅有 5 個共用元件，建議提取以下高頻使用模式：

| 元件 | 用途 | 預估使用頁面數 |
|------|------|----------------|
| `DataTable` | 可排序、可篩選、可分頁的資料表格 | 15+ |
| `FilterBar` | 搜尋框 + 篩選標籤 + 排序控制 | 12+ |
| `Modal` / `Dialog` | 確認對話框、詳情彈窗、表單彈窗 | 10+ |
| `Toast` / `Notification` | 操作回饋、即時告警通知 | 全域 |
| `ProgressBar` | 已在 Dashboard 重複定義多次 | 8+ |
| `Tabs` | 頁面內分頁（如 PredictiveHub 的 6 個 tab） | 8+ |
| `EmptyState` | 空資料佔位圖 | 全部列表頁面 |
| `Skeleton` | 載入骨架屏 | 全部頁面 |
| `ConfirmDialog` | 刪除/下線等危險操作確認 | 10+ |
| `PageHeader` | 頁面標題 + 麵包屑 + 動作按鈕 | 全部頁面 |

#### 測試框架

| 層級 | 工具 | 版本建議 | 用途 |
|------|------|----------|------|
| 單元測試 | Vitest | ^3.1 | 元件測試、hooks 測試、工具函數測試 |
| 元件測試 | @testing-library/react | ^16.1 | DOM 行為測試、使用者互動模擬 |
| E2E 測試 | Playwright | ^1.50 | 跨瀏覽器端對端測試、視覺回歸測試 |
| 覆蓋率 | v8 (Vitest 內建) | -- | 目標：共用元件 80%+、hooks 90%+、頁面 60%+ |

### 2.2 後端

#### 框架選擇：Python FastAPI

經過三個候選框架的比較分析，**推薦 FastAPI** 作為後端框架。

| 考量 | Python FastAPI | Node.js NestJS | Go Gin/Fiber |
|------|----------------|----------------|--------------|
| **AI/ML 生態系** | 原生支援，scikit-learn/PyTorch/pandas 直接整合 | 需透過子行程呼叫 Python | 需透過 gRPC/HTTP 呼叫 Python 服務 |
| **開發速度** | 快，自動產生 OpenAPI 文件 | 中，裝飾器模式需學習 | 慢，靜態型別冗長 |
| **效能** | 高（async/ASGI），單機可處理數千並發 | 高（事件迴圈） | 極高（編譯型語言） |
| **型別安全** | Pydantic v2 模型驗證 | TypeScript + class-validator | 編譯期型別檢查 |
| **團隊招聘** | Python 人才池最大 | 前後端同語言 | Go 人才相對稀缺 |
| **與前端語言一致性** | 不同語言 | 相同語言 (TypeScript) | 不同語言 |

**選擇 FastAPI 的核心理由：**

1. **AI/ML 整合為本平台核心差異化功能**。FastAPI 可直接匯入 scikit-learn 模型、呼叫 PyTorch 推理、使用 pandas 處理遙測資料，無需跨語言服務呼叫。PredictiveHub 的預測性維護、根因分析、異常偵測等功能均需要 Python ML 生態系。

2. **Pydantic v2 的型別驗證**與前端 Zod schema 理念一致，且可自動產生 OpenAPI 3.1 規格文件，前端可透過 openapi-typescript 自動產生型別定義，確保前後端型別同步。

3. **非同步支援** 原生 async/await，搭配 ASGI 伺服器（Uvicorn），單機即可處理 WebSocket 連線與 HTTP 請求的混合負載。

```
建議版本：
- Python 3.12+
- FastAPI ^0.115
- Pydantic ^2.10
- Uvicorn ^0.34
- SQLAlchemy ^2.0 (ORM)
- Alembic ^1.14 (資料庫遷移)
```

#### API 風格：REST + 局部 GraphQL

**主要採用 REST**，原因：

- 團隊熟悉度最高，學習成本最低
- CMDB 的資源模型天然適合 REST（Location, Asset, Rack, Alert 均為明確的資源實體）
- 搭配 OpenAPI 規格可自動產生客戶端 SDK
- TanStack Query 的快取策略與 REST 端點一一對應最為自然

**局部引入 GraphQL**（Phase 3+）：

- Dashboard 聚合查詢（一次請求取得多個 KPI）
- 資產關係圖查詢（CI 的上下游依賴關係）
- 拓撲圖資料查詢（跨多層級的關聯資料）

**不選擇 tRPC 的理由：** tRPC 要求前後端使用 TypeScript，與 Python 後端不相容。

#### 身份認證：JWT + RBAC

```
技術方案：
- 認證：JWT (access token 15min + refresh token 7d)
- 授權：RBAC (Role-Based Access Control)
- 密碼雜湊：bcrypt
- Token 儲存：httpOnly cookie (access token) + Redis (refresh token 黑名單)
```

**預定義角色：**

| 角色 | 權限範圍 |
|------|----------|
| Super Admin | 全系統所有操作 |
| IDC Manager | 特定 IDC 的所有操作 |
| Operator | 資產查看、工單建立、告警確認 |
| Auditor | 唯讀 + 稽核日誌檢視 |
| Viewer | 唯讀存取 |

**權限粒度：** `<resource>:<action>` 模式，例如 `asset:create`, `workorder:approve`, `alert:acknowledge`。

#### 即時通訊伺服器：Socket.io (Python)

```
安裝：python-socketio ^5.11 + uvicorn
```

**事件通道設計：**
- `alert:new` -- 新告警推送
- `alert:status` -- 告警狀態變更
- `sensor:reading` -- 感測器讀數串流
- `workorder:update` -- 工單狀態變更
- `inventory:progress` -- 盤點任務進度
- `discovery:result` -- 自動發現結果

**房間（Room）策略：** 按位置階層分配房間，例如 `room:cn-east-sh-pd-idc01`，使用者登入後自動加入其負責 IDC 的房間。

#### 非同步任務佇列：Celery + Redis

```
安裝：celery ^5.4, redis ^5.2
```

**適用場景：**

| 任務類型 | 預估執行時間 | 頻率 |
|----------|------------|------|
| 自動發現（SNMP 掃描） | 5-30 分鐘 | 每日/手動觸發 |
| 盤點 Excel 匯入 | 1-10 分鐘 | 手動觸發 |
| 報表 PDF 生成 | 30 秒-5 分鐘 | 手動/定時 |
| 遙測資料聚合 | 1-5 分鐘 | 每 5 分鐘 |
| AI 預測模型推理 | 10-60 秒 | 每小時 |
| 告警規則評估 | <5 秒 | 即時 |

#### 檔案處理

- **Excel 匯入/匯出：** openpyxl ^3.1（.xlsx）、pandas ^2.2（資料轉換）
- **PDF 報表生成：** WeasyPrint ^62（HTML/CSS 轉 PDF）或 ReportLab ^4.2（程式化 PDF 生成）
- **檔案儲存：** 本地檔案系統（開發）、MinIO/S3（生產）

### 2.3 資料庫

#### 主資料庫：PostgreSQL 16

**選擇 PostgreSQL 而非 MySQL 的理由：**

| 考量 | PostgreSQL | MySQL |
|------|-----------|-------|
| JSON 支援 | 原生 JSONB，可建索引 | JSON 型別，索引支援有限 |
| 遞迴查詢 | 原生 CTE | 支援但效能較差 |
| 全文搜尋 | 內建 tsvector/tsquery | 需外掛或另建 FULLTEXT |
| 擴充套件生態 | pgvector, TimescaleDB, PostGIS | 有限 |
| 階層資料 | ltree 擴充套件完美支援位置階層 | 需 adjacency list 或 nested set |
| 授權模型 | Row-Level Security (RLS) | 無原生 RLS |

**核心考量：** IronGrid 的位置階層（Country->Region->City->Campus->IDC->Module->Rack->U）天然適合使用 PostgreSQL 的 `ltree` 擴充套件進行高效的階層查詢與聚合。JSONB 則用於儲存各類資產的異質屬性（不同廠商、型號的擴展欄位）。

#### 快取層：Redis 7

```
應用場景：
- Session 儲存（JWT refresh token 黑名單）
- 位置 KPI 聚合快取（TTL 5 分鐘）
- 告警計數快取（即時更新）
- 熱門查詢結果快取
- Celery 任務佇列 broker
- Socket.io adapter（多實例時的訊息同步）
- 速率限制計數器
```

#### 時序資料庫：TimescaleDB v2

**選擇 TimescaleDB 而非 InfluxDB 的理由：**

- TimescaleDB 是 PostgreSQL 的擴充套件，**無需額外維運一套獨立的資料庫系統**
- 使用標準 SQL 查詢，團隊無需學習 InfluxQL/Flux
- 可與主資料庫的資產表進行 JOIN 查詢
- 自動分區（hypertable）與壓縮
- 連續聚合（continuous aggregates）用於 Dashboard KPI

**儲存的時序資料：**
- 感測器讀數（溫度、濕度、功耗、風速）-- 每 30 秒一筆
- 網路遙測（流量、延遲、封包遺失率）-- 每分鐘一筆
- 資產健康指標（CPU、記憶體、磁碟 SMART）-- 每 5 分鐘一筆
- PUE 歷史趨勢 -- 每 15 分鐘一筆

**預估資料量：** 15 個 IDC、38,000+ 資產，每資產 3-5 個感測點 => 每日約 2-5 億筆時序資料點。TimescaleDB 的壓縮功能可將儲存空間壓縮 90%+。

#### 搜尋引擎：Elasticsearch 8

**用途：**
- 全文資產搜尋（名稱、序號、IP、位置、標籤）
- 告警日誌搜尋
- 稽核事件搜尋
- 頂部搜尋列（CMD+K）的全域搜尋後端

**替代方案：** 若初期搜尋需求簡單，可先使用 PostgreSQL 的 `pg_trgm` + `tsvector` 實作，待資料量增長後再遷移至 Elasticsearch。此為 **可逆決策**，建議 Phase 2 使用 PostgreSQL，Phase 4+ 視需求引入 Elasticsearch。

### 2.4 AI/ML

#### LLM 整合：Claude API (Anthropic)

```
SDK: anthropic ^0.42 (Python)
模型: claude-sonnet-4-20250514 (成本效益比最佳)
備選: claude-opus-4-20250514 (複雜推理場景)
```

**應用場景：**

| 功能 | 模型 | 輸入 | 輸出 |
|------|------|------|------|
| 根因分析 | claude-sonnet-4 | 告警序列 + 拓撲圖 + 變更歷史 | 結構化根因報告 |
| 工單風險評估 | claude-sonnet-4 | 工單內容 + CI 資訊 + 歷史工單 | 風險等級 + 建議審批意見 |
| 知識庫問答 | claude-sonnet-4 + RAG | 使用者問題 + 相關文件片段 | 故障排除步驟 |
| 告警摘要 | claude-sonnet-4 | 批量告警資料 | 自然語言摘要 |
| 變更影響分析 | claude-opus-4 | 變更申請 + CI 依賴圖 | 影響範圍評估 |

#### ML 管線：scikit-learn + PyTorch

```
scikit-learn ^1.6 -- 傳統 ML 模型（分類、迴歸、聚類）
PyTorch ^2.6 -- 深度學習模型（時序預測、異常偵測）
pandas ^2.2 -- 資料處理
numpy ^2.2 -- 數值計算
```

**預測性維護模型：**

| 模型 | 演算法 | 輸入特徵 | 輸出 |
|------|--------|----------|------|
| 剩餘使用壽命（RUL） | LSTM / Transformer | 溫度趨勢、振動頻譜、運行時數 | 預估故障日期 |
| 異常偵測 | Isolation Forest / Autoencoder | 感測器讀數向量 | 異常分數 |
| 故障分類 | Random Forest / XGBoost | SMART 數據、錯誤日誌計數 | 故障類型機率 |
| 容量預測 | Prophet / ARIMA | 歷史使用率趨勢 | 未來 N 天使用率 |

#### 向量資料庫：pgvector v0.8

**選擇 pgvector 而非獨立向量資料庫（Pinecone/Weaviate）的理由：**

- 作為 PostgreSQL 擴充套件，無需額外基礎設施
- 知識庫規模預估在 10 萬文件以內，pgvector 效能足夠
- 可與主資料庫的關聯式資料進行 JOIN（例如：找到與特定資產相關的故障排除文件）

**RAG 架構：**
```
使用者提問
    |
    v
Embedding (Claude / text-embedding-3-small)
    |
    v
pgvector 相似度搜尋 (top-k=5)
    |
    v
Context 組裝 + Claude API 呼叫
    |
    v
結構化回答 + 來源引用
```

**知識庫來源：**
- 設備維護手冊
- 歷史故障報告
- 標準作業程序（SOP）
- 廠商技術文件

### 2.5 DevOps

#### 容器化

```
開發環境: Docker Compose v2
  - frontend (Vite dev server)
  - backend (FastAPI + Uvicorn)
  - postgres (PostgreSQL 16 + TimescaleDB + pgvector)
  - redis (Redis 7)
  - celery-worker
  - celery-beat (定時任務排程)

生產環境: Kubernetes (K8s)
  - 建議使用 managed K8s (GKE / EKS / AKS)
  - Helm charts 用於部署管理
  - HPA (Horizontal Pod Autoscaler) 用於自動擴展
```

#### CI/CD：GitHub Actions

```
流水線設計:
  PR 觸發:
    - lint (ESLint + Ruff)
    - type-check (tsc --noEmit + mypy)
    - unit-test (Vitest + pytest)
    - build (Vite build + Docker build)

  Main 分支合併:
    - 以上所有步驟
    - E2E 測試 (Playwright)
    - Docker image push to registry
    - 部署至 staging 環境

  Release tag:
    - 部署至 production
    - Database migration (Alembic)
    - Smoke test
```

#### 監控

| 層級 | 工具 | 用途 |
|------|------|------|
| 指標收集 | Prometheus | API 延遲、請求量、錯誤率、DB 連接池 |
| 視覺化 | Grafana | 營運儀表板、告警規則 |
| 日誌 | Loki + Promtail | 結構化日誌收集與查詢 |
| 追蹤 | OpenTelemetry + Jaeger | 分散式追蹤（API -> DB -> Cache -> AI） |
| 可用性 | Uptime Kuma | 端點健康檢查 |

**選擇 Loki 而非 ELK 的理由：** Loki 的資源消耗遠低於 Elasticsearch，且與 Grafana 原生整合。對於 CMDB 平台的日誌量級，Loki 足以應付，且維運成本顯著較低。

---

## 3. 分階段實施路線圖

### Phase 1: 前端工程化 (2-3 週)

**目標：** 將原型級前端升級為可維護的工程化程式碼基礎，為後端整合做好準備。

#### 1.1 共用元件提取

| 任務 | 來源 | 預估 |
|------|------|------|
| 提取 `DataTable` 元件（排序、分頁、行選取） | AssetManagement, RackManagement, AuditHistory, MonitoringAlerts | 2d |
| 提取 `FilterBar` 元件（搜尋框、篩選標籤、排序） | AssetManagement, MonitoringAlerts, WorkOrder | 1d |
| 提取 `Modal` / `Dialog` 元件 | 散落在多個頁面的彈窗邏輯 | 1d |
| 提取 `Toast` / `Notification` 元件 | 全域通知系統 | 0.5d |
| 提取 `ProgressBar` 元件 | Dashboard, HighSpeedInventory（至少 3 處重複定義） | 0.5d |
| 提取 `Tabs` 元件 | PredictiveHub, WorkOrder, MonitoringAlerts | 0.5d |
| 提取 `PageHeader` 元件（標題 + 麵包屑 + 操作按鈕） | 所有頁面 | 0.5d |
| 清除頁面內重複定義的 `Icon` 函數，統一使用 `components/Icon.tsx` | Dashboard, PredictiveHub 等 | 0.5d |

#### 1.2 全域狀態管理

| 任務 | 預估 |
|------|------|
| 安裝 Zustand，建立 `useAuthStore`（mock 使用者、角色、權限） | 0.5d |
| 建立 `useNotificationStore`（通知佇列、未讀計數） | 0.5d |
| 建立 `usePreferenceStore`（主題、語言偏好、sidebar 狀態） | 0.5d |

#### 1.3 API 抽象層

| 任務 | 預估 |
|------|------|
| 安裝 TanStack Query，建立 `QueryClientProvider` | 0.5d |
| 建立 `src/api/client.ts`（HTTP 客戶端封裝，含攔截器） | 0.5d |
| 為每個資料領域建立 API service + hooks（先使用 mock，介面已就緒） | 2d |
| 將 3-5 個代表性頁面遷移至使用 React Query hooks | 1d |

#### 1.4 表單系統化

| 任務 | 預估 |
|------|------|
| 安裝 React Hook Form + Zod | 0.5d |
| 定義核心 Zod schemas（Asset, Rack, WorkOrder, MaintenanceTask） | 1d |
| 改造 AddNewRack, AddMaintenanceTask, WorkOrder 建立頁面 | 1.5d |

#### 1.5 路由參數化

| 任務 | 預估 |
|------|------|
| `/assets/detail` -> `/assets/:assetId` | 0.5d |
| `/racks/detail` -> `/racks/:rackId` | 0.5d |
| `/audit/detail` -> `/audit/:eventId` | 0.5d |
| `/maintenance/task` -> `/maintenance/task/:taskId` | 0.5d |
| `/inventory/detail` -> `/inventory/:itemId` | 0.5d |

#### 1.6 測試基礎

| 任務 | 預估 |
|------|------|
| 安裝 Vitest + @testing-library/react，配置 vitest.config.ts | 0.5d |
| 為 5 個共用元件撰寫單元測試 | 1.5d |
| 為 LocationContext 撰寫測試 | 0.5d |
| 為 Zustand stores 撰寫測試 | 0.5d |

**Phase 1 交付物：**
- 10+ 個共用元件，含 Storybook 文件（可選）
- Zustand store 架構
- TanStack Query 抽象層（mock adapter）
- 3+ 個表單頁面使用 React Hook Form + Zod
- 參數化路由
- 30+ 個單元測試

---

### Phase 2: 後端 MVP (4-6 週)

**目標：** 建立核心後端服務，實現真實的 CRUD 操作，替換前端 mock 資料。

#### 2.1 基礎設施

| 任務 | 預估 |
|------|------|
| Docker Compose 環境（FastAPI + PostgreSQL + Redis） | 1d |
| FastAPI 專案架構（router, service, repository 分層） | 1d |
| SQLAlchemy 2.0 ORM 模型定義 | 2d |
| Alembic 資料庫遷移設定 + 初始 migration | 1d |
| pytest 測試框架設定 | 0.5d |

#### 2.2 認證與授權

| 任務 | 預估 |
|------|------|
| 使用者註冊/登入 API（JWT 簽發） | 1.5d |
| Refresh token 機制 + Redis 黑名單 | 1d |
| RBAC middleware（角色 + 權限檢查） | 1.5d |
| 前端登入頁面 + auth store 整合 | 1d |
| 受保護路由 HOC / middleware | 0.5d |

#### 2.3 核心 CRUD API

| 資源 | 端點數 | 預估 |
|------|--------|------|
| Location（階層 CRUD + 聚合查詢） | 8 | 2d |
| Asset（CRUD + 搜尋 + 篩選 + 分頁） | 8 | 2d |
| Rack（CRUD + U-position 管理） | 7 | 1.5d |
| CI（配置項 CRUD + 關係管理） | 7 | 1.5d |
| Alert（CRUD + 確認 + 統計） | 6 | 1d |
| WorkOrder（CRUD + 狀態機轉換） | 7 | 1.5d |

#### 2.4 前後端整合

| 任務 | 預估 |
|------|------|
| 更新 API client 指向真實後端 | 0.5d |
| 逐頁面替換 mock data 為 API 呼叫 | 3d |
| 錯誤處理 + loading 狀態 + 空狀態 UI | 1d |

#### 2.5 檔案處理

| 任務 | 預估 |
|------|------|
| Excel 匯入 API（盤點 / 資產批次匯入） | 1.5d |
| Excel 匯出 API（資產清單 / 告警報表） | 1d |
| 匯入任務排程（Celery 非同步處理） | 1d |

**Phase 2 交付物：**
- 可運行的 Docker Compose 開發環境
- JWT 認證 + RBAC 授權
- 6 個核心領域的 CRUD API
- 前端完全連接真實 API
- Excel 匯入/匯出功能
- API 測試覆蓋率 80%+

---

### Phase 3: 監控與即時數據 (3-4 週)

**目標：** 實現即時告警推送、感測器資料串流、Dashboard KPI 即時更新。

#### 3.1 WebSocket 伺服器

| 任務 | 預估 |
|------|------|
| Socket.io 伺服器設定（python-socketio + FastAPI 整合） | 1d |
| 房間（Room）管理（按位置階層訂閱） | 1d |
| 前端 Socket.io client 整合 | 1d |
| 連線狀態管理（斷線重連、狀態指示器） | 0.5d |

#### 3.2 告警系統

| 任務 | 預估 |
|------|------|
| 告警規則引擎（閾值規則 + 複合條件） | 2d |
| 告警生命週期（Open -> Acknowledged -> Resolved / Escalated） | 1d |
| 告警聚合與去重 | 1d |
| 告警通知推送（WebSocket + Email） | 1d |

#### 3.3 感測器資料管線

| 任務 | 預估 |
|------|------|
| TimescaleDB hypertable 設定（sensor_readings） | 0.5d |
| 感測器資料攝取 API（批次寫入） | 1d |
| 資料聚合查詢（5min / 1h / 1d 粒度） | 1d |
| 連續聚合（continuous aggregate）設定 | 0.5d |
| 資料保留策略（raw: 30d, 5min: 1y, 1h: 5y） | 0.5d |

#### 3.4 Dashboard 即時更新

| 任務 | 預估 |
|------|------|
| Dashboard KPI WebSocket 訂閱 | 1d |
| 即時圖表更新（Recharts 串流模式） | 1d |
| 位置階層 KPI 即時聚合 | 1d |

**Phase 3 交付物：**
- WebSocket 即時通訊基礎設施
- 告警規則引擎 + 生命週期管理
- 感測器資料攝取與時序儲存
- Dashboard 即時 KPI 更新
- 告警即時推送通知

---

### Phase 4: 工作流引擎 (3-4 週)

**目標：** 實現完整的工作流程自動化，包含工單審批、維護排程、自動發現、盤點核對。

#### 4.1 工單狀態機

| 任務 | 預估 |
|------|------|
| 工單狀態機引擎（WAIT -> APPROVE -> EXECUTE -> DONE / REJECT） | 1.5d |
| 審批流程（串簽 / 會簽、代理審批） | 2d |
| 工單自動分派規則（按技能 / 位置 / 工作量） | 1d |
| 工單 SLA 追蹤與逾期告警 | 1d |

#### 4.2 維護任務排程

| 任務 | 預估 |
|------|------|
| 預防性維護排程（週期性 / 基於條件） | 1.5d |
| 維護窗口管理（維護時段 + 衝突偵測） | 1d |
| 任務指派 + 行事曆檢視 | 1d |

#### 4.3 自動發現整合

| 任務 | 預估 |
|------|------|
| SNMP v2c/v3 掃描引擎（Celery 非同步任務） | 2d |
| VMware vCenter API 整合（虛擬機發現） | 1.5d |
| 發現結果與 CMDB 比對 + 差異報告 | 1.5d |

#### 4.4 盤點核對

| 任務 | 預估 |
|------|------|
| 盤點任務建立 + 分派 | 1d |
| 實際盤點結果匯入（Excel / 掃碼） | 1d |
| 帳實比對 + 差異報告 | 1d |
| 差異處理工作流（調帳 / 報廢 / 追查） | 1d |

#### 4.5 通知系統

| 任務 | 預估 |
|------|------|
| 通知偏好設定（哪些事件要通知、透過什麼管道） | 1d |
| Email 通知（SMTP / SendGrid） | 0.5d |
| Webhook 通知（企業微信 / 釘釘 / Slack） | 1d |

**Phase 4 交付物：**
- 完整的工單審批流程
- 維護任務排程引擎
- SNMP/VMware 自動發現
- 盤點核對工作流
- 多管道通知系統

---

### Phase 5: AI 智能化 (4-6 週)

**目標：** 整合 AI/ML 能力，實現預測性維護、根因分析、智能推薦。

#### 5.1 預測性維護

| 任務 | 預估 |
|------|------|
| 訓練資料準備（歷史故障記錄 + 遙測資料） | 2d |
| RUL 預測模型訓練（LSTM / Transformer） | 3d |
| 模型服務化（FastAPI 推理端點） | 1d |
| PredictiveHub 頁面整合真實預測結果 | 1.5d |
| 模型效能監控 + 自動重訓練管線 | 1.5d |

#### 5.2 Claude API 整合

| 任務 | 預估 |
|------|------|
| Claude API 服務封裝（含 rate limiting、重試、fallback） | 1d |
| 根因分析功能（告警序列 -> 結構化報告） | 2d |
| 工單風險評估（工單內容 -> 風險等級 + 建議） | 1.5d |
| AI 聊天介面改造（PredictiveHub 的 AI 對話變為真實互動） | 1.5d |

#### 5.3 異常偵測

| 任務 | 預估 |
|------|------|
| 時序異常偵測模型（Isolation Forest / Autoencoder） | 2d |
| 即時串流異常偵測（與 sensor pipeline 整合） | 1.5d |
| 異常告警產生 + 與告警系統整合 | 1d |

#### 5.4 知識庫 RAG

| 任務 | 預估 |
|------|------|
| pgvector 設定 + embedding 管線 | 1d |
| 文件攝取管線（PDF/Markdown -> 分段 -> 向量化） | 1.5d |
| RAG 查詢 API + TroubleshootingGuide 頁面整合 | 1.5d |
| 回答品質評估 + 反饋收集機制 | 1d |

**Phase 5 交付物：**
- 可運行的預測性維護模型
- Claude API 驅動的根因分析
- 即時異常偵測
- 知識庫 RAG 問答系統
- AI 功能的效能監控儀表板

---

### Phase 6: 企業級特性 (持續迭代)

**目標：** 強化安全性、可擴展性、合規性，使平台達到企業級生產就緒狀態。

#### 6.1 多租戶 (2-3 週)
- 租戶隔離策略（Schema-per-tenant 或 Row-Level Security）
- 租戶管理 API + 管理介面
- 資料隔離驗證

#### 6.2 稽核與合規 (1-2 週)
- 完整稽核日誌（who/what/when/where/how）
- 合規報表（SOC2、ISO 27001 等格式）
- 資料保留策略與自動歸檔

#### 6.3 安全強化 (2-3 週)
- API 速率限制（Redis + sliding window）
- 輸入驗證與 XSS/CSRF 防護
- SQL injection 防護（ORM 已天然防護，額外加固 raw query）
- 敏感資料加密（at-rest + in-transit）
- 安全掃描整合（Snyk / Dependabot）

#### 6.4 效能最佳化 (2-3 週)
- 虛擬滾動（DataTable 大量資料場景）
- 查詢最佳化（慢查詢分析 + 索引調優）
- 前端 bundle 分析 + code splitting 最佳化
- CDN 靜態資源加速
- 資料庫連接池調優

#### 6.5 行動裝置支援 (2-3 週)
- 響應式佈局最佳化（目前 sidebar 在行動裝置上不友善）
- PWA 支援（Service Worker + 離線快取）
- 推播通知（Web Push API）

#### 6.6 SSO 整合 (1-2 週)
- LDAP/Active Directory 整合
- SAML 2.0 SSO
- OAuth 2.0 / OIDC（Google / Microsoft / GitHub）

---

## 4. 資料庫架構設計（概要）

### 4.1 核心資料表

以下設計基於前端 mock 資料模型的分析結果，反映了實際的業務實體關係。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         位置階層 (Location Hierarchy)                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  locations                                                              │
│  ├── id (PK, UUID)                                                     │
│  ├── parent_id (FK -> locations.id, nullable)                          │
│  ├── level (ENUM: country/region/city/campus/idc/module)               │
│  ├── name (VARCHAR)                                                    │
│  ├── name_en (VARCHAR)                                                 │
│  ├── slug (VARCHAR, unique within parent)                              │
│  ├── path (LTREE, e.g. 'cn.east.shanghai.pudong.idc01')               │
│  ├── address (TEXT, nullable)                                          │
│  ├── geo_lat (DECIMAL, nullable)                                       │
│  ├── geo_lng (DECIMAL, nullable)                                       │
│  ├── metadata (JSONB)                                                  │
│  ├── created_at (TIMESTAMPTZ)                                          │
│  └── updated_at (TIMESTAMPTZ)                                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         資產與配置 (Assets & CI)                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  equipment                           ci (Configuration Item)            │
│  ├── id (PK, UUID)                   ├── id (PK, UUID)                 │
│  ├── asset_tag (UNIQUE)              ├── ci_code (UNIQUE)              │
│  ├── name (VARCHAR)                  ├── name (VARCHAR)                │
│  ├── type (ENUM: server/            ├── type (ENUM: app/db/           │
│  │   network/storage/ups/other)      │   middleware/os/vm)             │
│  ├── vendor (VARCHAR)                ├── equipment_id (FK, nullable)   │
│  ├── model (VARCHAR)                 ├── biz_system_id (FK)           │
│  ├── serial_number (VARCHAR)         ├── status (ENUM)                │
│  ├── location_id (FK -> locations)   ├── owner (VARCHAR)              │
│  ├── rack_id (FK -> racks)           ├── metadata (JSONB)             │
│  ├── u_start (INT)                   ├── created_at (TIMESTAMPTZ)     │
│  ├── u_end (INT)                     └── updated_at (TIMESTAMPTZ)     │
│  ├── bia_level (ENUM: critical/                                        │
│  │   important/normal/minor)         biz_system                        │
│  ├── status (ENUM: operational/      ├── id (PK, UUID)                │
│  │   maintenance/decommissioned)     ├── name (VARCHAR)               │
│  ├── purchase_date (DATE)            ├── owner (VARCHAR)              │
│  ├── warranty_expires (DATE)         ├── tier (ENUM: tier1/2/3)       │
│  ├── ip_address (INET)              ├── status (ENUM)                │
│  ├── metadata (JSONB)                └── metadata (JSONB)             │
│  ├── created_at (TIMESTAMPTZ)                                          │
│  └── updated_at (TIMESTAMPTZ)                                          │
│                                                                         │
│  ci_relationship                                                        │
│  ├── id (PK, UUID)                                                     │
│  ├── source_ci_id (FK -> ci)                                           │
│  ├── target_ci_id (FK -> ci)                                           │
│  ├── relationship_type (ENUM: depends_on/runs_on/connects_to)          │
│  └── metadata (JSONB)                                                  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         機櫃 (Racks)                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  rack                                                                   │
│  ├── id (PK, UUID)                                                     │
│  ├── rack_code (UNIQUE)                                                │
│  ├── name (VARCHAR)                                                    │
│  ├── location_id (FK -> locations, module level)                       │
│  ├── row_index (INT)                                                   │
│  ├── col_index (INT)                                                   │
│  ├── total_u (INT, default 42)                                         │
│  ├── power_capacity_kw (DECIMAL)                                       │
│  ├── status (ENUM: operational/maintenance/decommissioned/reserved)    │
│  ├── metadata (JSONB)                                                  │
│  ├── created_at (TIMESTAMPTZ)                                          │
│  └── updated_at (TIMESTAMPTZ)                                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         監控 (Monitoring)                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  alert                                sensor_reading (TimescaleDB)      │
│  ├── id (PK, UUID)                    ├── time (TIMESTAMPTZ, PK)       │
│  ├── alert_code (UNIQUE)              ├── sensor_id (FK)               │
│  ├── equipment_id (FK, nullable)      ├── equipment_id (FK)            │
│  ├── location_id (FK)                 ├── metric_type (ENUM:           │
│  ├── severity (ENUM: critical/        │   temperature/humidity/         │
│  │   warning/info)                    │   power/fan_speed/vibration)   │
│  ├── description (TEXT)               ├── value (DOUBLE PRECISION)     │
│  ├── status (ENUM: open/             ├── unit (VARCHAR)               │
│  │   acknowledged/resolved/           └── location_id (FK)            │
│  │   escalated)                                                        │
│  ├── acknowledged_by (FK -> users)    sensor                           │
│  ├── acknowledged_at (TIMESTAMPTZ)    ├── id (PK, UUID)               │
│  ├── resolved_at (TIMESTAMPTZ)        ├── name (VARCHAR)              │
│  ├── created_at (TIMESTAMPTZ)         ├── type (ENUM)                 │
│  └── metadata (JSONB)                 ├── equipment_id (FK)           │
│                                        ├── location_id (FK)           │
│                                        ├── threshold_min (DECIMAL)    │
│                                        ├── threshold_max (DECIMAL)    │
│                                        └── is_active (BOOLEAN)        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         維護與工單 (Maintenance & Work Orders)            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  maintenance_task                     work_order                        │
│  ├── id (PK, UUID)                    ├── id (PK, UUID)               │
│  ├── task_code (UNIQUE)               ├── order_code (UNIQUE)         │
│  ├── title (VARCHAR)                  ├── title (VARCHAR)             │
│  ├── type (ENUM: preventive/          ├── status (ENUM: wait/         │
│  │   corrective/inspection)           │   approve/execute/done/       │
│  ├── equipment_id (FK)                │   reject)                     │
│  ├── location_id (FK)                 ├── requestor_id (FK -> users)  │
│  ├── assigned_to (FK -> users)        ├── ci_id (FK -> ci)           │
│  ├── status (ENUM: scheduled/         ├── reason (TEXT)               │
│  │   in_progress/completed/           ├── priority (ENUM: critical/   │
│  │   cancelled)                       │   high/medium/low)            │
│  ├── scheduled_date (DATE)            ├── approved_by (FK -> users)   │
│  ├── completed_at (TIMESTAMPTZ)       ├── approved_at (TIMESTAMPTZ)   │
│  ├── notes (TEXT)                     ├── created_at (TIMESTAMPTZ)    │
│  ├── created_at (TIMESTAMPTZ)         └── metadata (JSONB)           │
│  └── updated_at (TIMESTAMPTZ)                                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         盤點 (Inventory)                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  inventory_task                       inventory_item                    │
│  ├── id (PK, UUID)                    ├── id (PK, UUID)               │
│  ├── task_code (UNIQUE)               ├── task_id (FK)                │
│  ├── name (VARCHAR)                   ├── equipment_id (FK)           │
│  ├── location_id (FK)                 ├── expected_location (VARCHAR) │
│  ├── assigned_to (FK -> users)        ├── actual_location (VARCHAR)   │
│  ├── status (ENUM: pending/           ├── match_status (ENUM: match/  │
│  │   in_progress/completed)           │   mismatch/missing/surplus)   │
│  ├── progress_pct (DECIMAL)           ├── scanned_at (TIMESTAMPTZ)    │
│  ├── started_at (TIMESTAMPTZ)         └── notes (TEXT)                │
│  ├── completed_at (TIMESTAMPTZ)                                        │
│  └── created_at (TIMESTAMPTZ)         discovered_ci                   │
│                                        ├── id (PK, UUID)              │
│                                        ├── discovery_task_id (FK)     │
│                                        ├── ip_address (INET)          │
│                                        ├── mac_address (MACADDR)      │
│                                        ├── hostname (VARCHAR)         │
│                                        ├── device_type (VARCHAR)      │
│                                        ├── matched_equipment_id (FK)  │
│                                        ├── match_confidence (DECIMAL) │
│                                        └── discovered_at (TIMESTAMPTZ)│
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         使用者與權限 (Users & Permissions)               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  users                               role                              │
│  ├── id (PK, UUID)                   ├── id (PK, UUID)               │
│  ├── username (UNIQUE)               ├── name (VARCHAR, UNIQUE)       │
│  ├── email (UNIQUE)                  ├── description (TEXT)           │
│  ├── display_name (VARCHAR)          ├── is_system (BOOLEAN)         │
│  ├── display_name_en (VARCHAR)       └── created_at (TIMESTAMPTZ)    │
│  ├── password_hash (VARCHAR)                                           │
│  ├── avatar_url (VARCHAR)            permission                       │
│  ├── preferred_lang (ENUM)           ├── id (PK, UUID)               │
│  ├── is_active (BOOLEAN)             ├── resource (VARCHAR)           │
│  ├── last_login (TIMESTAMPTZ)        ├── action (VARCHAR)            │
│  ├── created_at (TIMESTAMPTZ)        └── description (TEXT)          │
│  └── updated_at (TIMESTAMPTZ)                                          │
│                                        role_permission (M:N)          │
│  user_role (M:N)                      ├── role_id (FK)               │
│  ├── user_id (FK)                    └── permission_id (FK)          │
│  └── role_id (FK)                                                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         稽核 (Audit)                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  audit_event                                                            │
│  ├── id (PK, UUID)                                                     │
│  ├── event_type (VARCHAR, e.g. 'asset.create', 'workorder.approve')    │
│  ├── actor_id (FK -> users)                                            │
│  ├── resource_type (VARCHAR)                                           │
│  ├── resource_id (UUID)                                                │
│  ├── changes (JSONB, {field: {old, new}})                              │
│  ├── ip_address (INET)                                                 │
│  ├── user_agent (TEXT)                                                 │
│  ├── location_id (FK -> locations, nullable)                           │
│  └── created_at (TIMESTAMPTZ)                                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 4.2 簡化 ER 圖

```
                                    ┌──────────┐
                                    │  users   │
                                    └────┬─────┘
                                         │ M:N
                                    ┌────┴─────┐
                                    │   role   │
                                    └────┬─────┘
                                         │ M:N
                                    ┌────┴──────────┐
                                    │  permission   │
                                    └───────────────┘

     ┌──────────────┐
     │  locations   │◄──── self-ref (parent_id)
     │  (ltree)     │
     └──┬───┬───┬───┘
        │   │   │
        │   │   └────────────────────────────────┐
        │   │                                    │
        │   ▼                                    ▼
        │ ┌────────┐    ┌──────┐    ┌───────────────────┐
        │ │  rack  │◄───┤equip-│───►│  sensor_reading   │
        │ └────────┘    │ment  │    │  (TimescaleDB)    │
        │               └──┬───┘    └───────────────────┘
        │                  │
        │                  │ 1:N
        │                  ▼
        │               ┌──────┐     ┌──────────────┐
        │               │  ci  │◄───►│ci_relationship│
        │               └──┬───┘     └──────────────┘
        │                  │
        │                  │ FK
        ▼                  ▼
  ┌──────────┐      ┌────────────┐      ┌───────────────────┐
  │  alert   │      │ work_order │      │ maintenance_task  │
  └──────────┘      └────────────┘      └───────────────────┘
                           │
                           │ FK (actor)
                           ▼
                    ┌──────────────┐
                    │ audit_event  │
                    └──────────────┘

  ┌──────────────────┐     ┌──────────────────┐
  │ inventory_task   │────►│ inventory_item   │
  └──────────────────┘     └──────────────────┘

  ┌──────────────────┐
  │  discovered_ci   │
  └──────────────────┘

  ┌──────────────────┐
  │  biz_system      │◄──── ci.biz_system_id
  └──────────────────┘
```

### 4.3 預估資料規模

| 表 | 預估行數（初始） | 年增長率 | 備註 |
|----|----------------|---------|------|
| locations | ~200 | 低 | 階層結構，變動少 |
| equipment | ~38,000 | 10-20% | 核心資產 |
| ci | ~50,000 | 15-25% | 含虛擬機 |
| rack | ~2,400 | 5-10% | 15 IDC * 160 avg |
| alert | ~500,000/年 | -- | 需定期歸檔 |
| sensor_reading | ~2-5 億/年 | -- | TimescaleDB 壓縮 |
| work_order | ~10,000/年 | 10% | -- |
| maintenance_task | ~5,000/年 | 10% | -- |
| audit_event | ~200,000/年 | 20% | 需定期歸檔 |

---

## 5. API 架構設計（概要）

### 5.1 API 設計原則

- RESTful 風格，資源名稱使用複數
- 版本控制：URL path 前綴 `/api/v1/`
- 分頁：`?page=1&page_size=20`（cursor-based 用於時序資料）
- 篩選：`?status=operational&type=server`
- 排序：`?sort=-created_at`（`-` 前綴表示降冪）
- 欄位選取：`?fields=id,name,status`（減少傳輸量）
- 回應格式：統一的 envelope `{ data, meta, errors }`

### 5.2 API 端點清單

#### 認證 (Auth)
```
POST   /api/v1/auth/login              -- 登入，取得 JWT
POST   /api/v1/auth/refresh            -- 重新整理 access token
POST   /api/v1/auth/logout             -- 登出，吊銷 refresh token
GET    /api/v1/auth/me                 -- 取得當前使用者資訊
```

#### 位置 (Locations)
```
GET    /api/v1/locations               -- 取得位置樹（支援 ?depth=N）
GET    /api/v1/locations/:id           -- 取得單一位置詳情
POST   /api/v1/locations               -- 建立位置節點
PUT    /api/v1/locations/:id           -- 更新位置資訊
DELETE /api/v1/locations/:id           -- 刪除位置節點
GET    /api/v1/locations/:id/kpis      -- 取得位置聚合 KPI
GET    /api/v1/locations/:id/children  -- 取得下層子節點
GET    /api/v1/locations/:id/assets    -- 取得位置下所有資產
```

#### 資產 (Equipment)
```
GET    /api/v1/equipment               -- 資產列表（分頁、篩選、排序）
GET    /api/v1/equipment/:id           -- 資產詳情
POST   /api/v1/equipment               -- 建立資產
PUT    /api/v1/equipment/:id           -- 更新資產
DELETE /api/v1/equipment/:id           -- 刪除/報廢資產
GET    /api/v1/equipment/:id/metrics   -- 資產即時指標
GET    /api/v1/equipment/:id/history   -- 資產變更歷史
POST   /api/v1/equipment/import        -- Excel 批次匯入
GET    /api/v1/equipment/export        -- Excel 匯出
```

#### 配置項 (CI)
```
GET    /api/v1/ci                      -- CI 列表
GET    /api/v1/ci/:id                  -- CI 詳情
POST   /api/v1/ci                      -- 建立 CI
PUT    /api/v1/ci/:id                  -- 更新 CI
DELETE /api/v1/ci/:id                  -- 刪除 CI
GET    /api/v1/ci/:id/relationships    -- CI 關係圖
POST   /api/v1/ci/:id/relationships    -- 建立 CI 關係
```

#### 機櫃 (Racks)
```
GET    /api/v1/racks                   -- 機櫃列表
GET    /api/v1/racks/:id               -- 機櫃詳情（含 U-position 佔用圖）
POST   /api/v1/racks                   -- 建立機櫃
PUT    /api/v1/racks/:id               -- 更新機櫃
DELETE /api/v1/racks/:id               -- 刪除機櫃
GET    /api/v1/racks/:id/equipment     -- 機櫃內設備列表
POST   /api/v1/racks/:id/equipment     -- 上架設備
DELETE /api/v1/racks/:id/equipment/:eid -- 下架設備
```

#### 告警 (Alerts)
```
GET    /api/v1/alerts                  -- 告警列表（分頁、篩選）
GET    /api/v1/alerts/:id              -- 告警詳情
POST   /api/v1/alerts/:id/acknowledge  -- 確認告警
POST   /api/v1/alerts/:id/resolve      -- 解決告警
GET    /api/v1/alerts/statistics       -- 告警統計（按嚴重度、狀態、位置）
GET    /api/v1/alerts/topology         -- 告警拓撲關係
```

#### 感測器 (Sensors)
```
GET    /api/v1/sensors                 -- 感測器列表
GET    /api/v1/sensors/:id             -- 感測器設定
PUT    /api/v1/sensors/:id             -- 更新感測器閾值
POST   /api/v1/sensors/readings        -- 批次寫入感測器讀數
GET    /api/v1/sensors/:id/readings    -- 查詢歷史讀數（時間範圍、聚合粒度）
```

#### 維護任務 (Maintenance)
```
GET    /api/v1/maintenance             -- 任務列表
GET    /api/v1/maintenance/:id         -- 任務詳情
POST   /api/v1/maintenance             -- 建立任務
PUT    /api/v1/maintenance/:id         -- 更新任務
POST   /api/v1/maintenance/:id/complete -- 完成任務
GET    /api/v1/maintenance/calendar    -- 行事曆檢視
```

#### 工單 (Work Orders)
```
GET    /api/v1/workorders              -- 工單列表
GET    /api/v1/workorders/:id          -- 工單詳情
POST   /api/v1/workorders              -- 建立工單
PUT    /api/v1/workorders/:id          -- 更新工單
POST   /api/v1/workorders/:id/approve  -- 審批工單
POST   /api/v1/workorders/:id/reject   -- 駁回工單
POST   /api/v1/workorders/:id/execute  -- 開始執行
POST   /api/v1/workorders/:id/complete -- 完成工單
```

#### 盤點 (Inventory)
```
GET    /api/v1/inventory/tasks         -- 盤點任務列表
POST   /api/v1/inventory/tasks         -- 建立盤點任務
GET    /api/v1/inventory/tasks/:id     -- 任務詳情
POST   /api/v1/inventory/tasks/:id/items -- 提交盤點結果
GET    /api/v1/inventory/tasks/:id/report -- 差異報告
```

#### 自動發現 (Discovery)
```
POST   /api/v1/discovery/scan          -- 啟動掃描任務
GET    /api/v1/discovery/tasks         -- 掃描任務列表
GET    /api/v1/discovery/tasks/:id     -- 掃描結果
POST   /api/v1/discovery/tasks/:id/reconcile -- 比對並匯入
```

#### AI (Predictive & Analysis)
```
GET    /api/v1/ai/predictions          -- 預測結果列表
GET    /api/v1/ai/predictions/:assetId -- 特定資產預測
POST   /api/v1/ai/rca                  -- 根因分析請求
POST   /api/v1/ai/chat                 -- AI 對話（串流回應）
POST   /api/v1/ai/knowledge/query      -- 知識庫 RAG 查詢
GET    /api/v1/ai/anomalies            -- 異常偵測結果
```

#### 稽核 (Audit)
```
GET    /api/v1/audit/events            -- 稽核事件列表（分頁、篩選）
GET    /api/v1/audit/events/:id        -- 事件詳情
GET    /api/v1/audit/reports           -- 合規報表列表
POST   /api/v1/audit/reports/generate  -- 生成合規報表
```

#### 系統 (System)
```
GET    /api/v1/system/users            -- 使用者列表
POST   /api/v1/system/users            -- 建立使用者
PUT    /api/v1/system/users/:id        -- 更新使用者
GET    /api/v1/system/roles            -- 角色列表
POST   /api/v1/system/roles            -- 建立角色
PUT    /api/v1/system/roles/:id        -- 更新角色權限
GET    /api/v1/system/settings         -- 系統設定
PUT    /api/v1/system/settings         -- 更新設定
```

### 5.3 WebSocket 事件清單

```
連線: wss://api.example.com/ws?token=<JWT>

Client -> Server:
  subscribe:location    { locationId }     -- 訂閱位置事件
  unsubscribe:location  { locationId }     -- 取消訂閱

Server -> Client:
  alert:new             { Alert }          -- 新告警
  alert:status          { id, status }     -- 告警狀態變更
  sensor:reading        { SensorReading }  -- 感測器即時讀數
  kpi:update            { locationId, KPI } -- KPI 即時更新
  workorder:update      { id, status }     -- 工單狀態變更
  inventory:progress    { taskId, pct }    -- 盤點進度
  discovery:result      { taskId, found }  -- 發現結果
  notification          { Notification }   -- 通用通知
```

---

## 6. 風險與挑戰

### 6.1 技術風險

| 風險 | 機率 | 影響 | 緩解策略 |
|------|------|------|----------|
| **Mock 到真實資料遷移** -- 前端對 mock 資料結構有隱式依賴 | 高 | 中 | Phase 1 先建立 API 抽象層，使頁面不直接依賴資料結構。使用 TypeScript 嚴格型別確保編譯期發現不匹配。 |
| **即時效能瓶頸** -- 15 IDC、38K 資產的即時遙測推送 | 中 | 高 | 分層推送策略：只推送使用者當前檢視位置的資料。使用 Redis Pub/Sub 做 WebSocket 伺服器間訊息同步。設定適當的節流（throttle）與取樣（sampling）頻率。 |
| **AI 模型準確度** -- 預測性維護需要大量標記資料 | 高 | 中 | 初期使用基於規則的異常偵測作為 fallback。逐步收集真實故障資料後改用 ML 模型。設定信心度閾值，低信心度結果僅作為參考而非告警。 |
| **TimescaleDB 儲存成本** -- 每日 2-5 億筆感測器讀數 | 中 | 中 | 啟用 TimescaleDB 原生壓縮（壓縮率 90%+）。設定資料保留策略（raw data 30 天、聚合資料 5 年）。使用連續聚合減少查詢時運算。 |
| **前端 bundle 膨脹** -- 39 頁面 + 新依賴套件 | 低 | 低 | 已使用 lazy() 動態載入。持續監控 bundle 大小。考慮將 D3.js 等大型套件做 dynamic import。 |

### 6.2 業務風險

| 風險 | 機率 | 影響 | 緩解策略 |
|------|------|------|----------|
| **多時區支援複雜度** -- 15 IDC 橫跨多個時區 | 高 | 中 | 所有時間戳統一使用 UTC 儲存（TIMESTAMPTZ）。前端按使用者偏好或 IDC 所在時區顯示。告警時間需特別注意時區轉換。 |
| **舊系統整合** -- SNMP/IPMI 設備的協定版本差異大 | 高 | 高 | 建立設備驅動程式（driver）抽象層。優先支援 SNMP v2c/v3。IPMI 透過 ipmitool 封裝。為每種設備類型維護 MIB/OID 映射表。 |
| **使用者抗拒遷移** -- 從現有工具遷移至新系統 | 中 | 高 | 提供 Excel 匯入/匯出作為過渡機制。支援逐步遷移（先匯入位置和資產，再啟用監控和工單）。保留與現有系統的 API 整合能力。 |
| **合規需求變動** -- 不同地區的資料駐留法規 | 中 | 高 | 使用 PostgreSQL Row-Level Security 實現租戶資料隔離。為跨境資料傳輸預留加密和稽核機制。 |

### 6.3 組織風險

| 風險 | 機率 | 影響 | 緩解策略 |
|------|------|------|----------|
| **團隊技能缺口** -- ML/AI 人才需求 | 高 | 中 | Phase 1-3 不需要 ML 專業人才，可在此期間招募或培訓。初期 AI 功能以 Claude API 呼叫為主，降低自研 ML 模型的門檻。 |
| **範圍蔓延** -- 企業 CMDB 需求無限擴展 | 高 | 高 | 嚴格遵循分階段實施路線圖。每個 Phase 結束時進行 scope review。維護 backlog 但不提前實施。 |

---

## 7. 團隊建議

### 7.1 建議團隊編制

#### 核心團隊（Phase 1-3，最小可行團隊）

| 角色 | 人數 | 關鍵技能 | 職責 |
|------|------|----------|------|
| 技術主管 / 架構師 | 1 | 全端設計、系統架構 | 技術決策、架構設計、Code Review |
| 前端工程師 | 2 | React 19, TypeScript, TanStack Query, Tailwind | 元件庫建設、頁面重構、API 整合 |
| 後端工程師 | 2 | Python, FastAPI, PostgreSQL, Redis | API 開發、資料庫設計、認證授權 |
| DevOps 工程師 | 1 | Docker, K8s, CI/CD, 監控 | 基礎設施搭建、部署流水線 |

**小計：6 人**

#### 擴充團隊（Phase 4-5，加入工作流和 AI）

| 角色 | 人數 | 關鍵技能 | 職責 |
|------|------|----------|------|
| 後端工程師（追加） | 1 | SNMP/IPMI, 網路協定 | 自動發現、設備整合 |
| ML 工程師 | 1 | PyTorch, scikit-learn, 時序分析 | 預測模型訓練、異常偵測 |
| QA 工程師 | 1 | Playwright, 測試策略 | E2E 測試、效能測試 |

**小計：9 人**

#### 長期團隊（Phase 6+，企業級特性）

| 角色 | 人數 | 關鍵技能 | 職責 |
|------|------|----------|------|
| 安全工程師 | 1 | 滲透測試、合規標準 | 安全強化、合規審計 |
| 前端工程師（追加） | 1 | 行動端、PWA | 行動端適配、效能最佳化 |
| SRE | 1 | 可靠性工程、監控 | 生產環境維運、效能調優 |

**小計：12 人**

### 7.2 關鍵招聘優先級

1. **Python 後端工程師**（最優先）-- Phase 2 的瓶頸在於 API 開發速度
2. **高級前端工程師**（高優先）-- 需要有元件庫建設經驗的人主導 Phase 1
3. **DevOps 工程師**（高優先）-- 越早建立 CI/CD 管線，後續開發效率越高
4. **ML 工程師**（Phase 4 前到位即可）-- 可先進行數據收集和清洗準備

---

## 8. 預估時間表

### 8.1 整體時間線

```
2026 Q2                    2026 Q3                    2026 Q4             2027 Q1
Apr      May      Jun      Jul      Aug      Sep      Oct      Nov      Dec      Jan
 |        |        |        |        |        |        |        |        |        |
 |==P1===|                                                                       |
 |  前端  |==== Phase 2: 後端 MVP ====|                                          |
 | 工程化 |                           |=== P3: 監控 ===|                         |
 |        |                           |  即時數據      |=== P4: 工作流 ===|      |
 |        |                           |                |                  |==P5===>
 |        |                           |                |                  | AI    |
 |        |                           |                |                  |       |
 ▼        ▼                           ▼                ▼                  ▼       |
 M1       M2                          M3               M4                M5      |
                                                                                  |
                                                            Phase 6: 企業級 ======>
                                                            （持續迭代）
```

### 8.2 里程碑定義

| 里程碑 | 目標日期 | 交付物 | 成功標準 |
|--------|---------|--------|----------|
| **M1: 前端工程化完成** | 2026-04-18 | 共用元件庫、Zustand store、React Query 抽象層、路由參數化、30+ 單元測試 | 所有頁面正常運行，測試通過 |
| **M2: 後端 MVP** | 2026-06-13 | 可運行的 API 伺服器、JWT 認證、6 個領域 CRUD、前端連接真實 API | 使用者可登入並完成基本 CRUD |
| **M3: 即時監控上線** | 2026-08-08 | WebSocket 即時推送、告警系統、感測器資料管線、Dashboard 即時 KPI | 告警延遲 < 3 秒，Dashboard 自動更新 |
| **M4: 工作流上線** | 2026-09-19 | 工單審批流程、維護排程、自動發現、盤點核對 | 完整工單流程可走通，發現準確率 > 90% |
| **M5: AI 功能上線** | 2026-11-14 | 預測性維護、根因分析、知識庫 RAG | 預測準確率 > 75%，RAG 回答相關性 > 80% |
| **M6: 企業就緒** | 2027-02-28 | 多租戶、稽核合規、安全強化、SSO | 通過安全審計，支援 3+ 租戶 |

### 8.3 關鍵依賴與並行度

```
Phase 1 (前端) ─────────────────┐
                                 ├──> Phase 2 (後端) 依賴 Phase 1 的 API 抽象層
Phase 1 可提前開始 DevOps 設定 ──┘

Phase 2 (後端) ──────┬──> Phase 3 (監控) 依賴 Phase 2 的 API 框架和資料庫
                     └──> Phase 4 (工作流) 依賴 Phase 2 的 CRUD 和認證

Phase 3 (監控) ──────┬──> Phase 5 (AI) 依賴 Phase 3 的遙測資料作為 ML 訓練輸入
Phase 4 (工作流) ────┘

Phase 5 (AI) ──────────> Phase 6 (企業級) 可與 Phase 5 後期並行
```

**可並行的工作：**
- Phase 1 期間：DevOps 環境搭建可同步進行
- Phase 2 期間：前端持續重構、新增測試
- Phase 3 和 Phase 4：有部分重疊期，可由不同工程師分別負責
- Phase 6：可在 Phase 5 進行中就開始安全強化和效能最佳化

### 8.4 預算考量

| 項目 | 月費預估 (USD) | 備註 |
|------|---------------|------|
| 雲端基礎設施（開發/測試） | $500-1,000 | 小型 VM + managed DB |
| 雲端基礎設施（生產） | $2,000-5,000 | K8s cluster + managed services |
| Claude API | $200-800 | 依使用量，初期以 sonnet 為主 |
| Elasticsearch（若採用） | $0 (自建) / $500+ (SaaS) | Phase 4+ 才需要 |
| 第三方 SaaS | $200-500 | SendGrid、監控工具等 |

---

## 附錄 A: 架構決策紀錄 (ADR) 摘要

### ADR-001: 選擇 FastAPI 作為後端框架

- **狀態：** Proposed
- **背景：** 平台需要 AI/ML 整合作為核心差異化功能。三個候選框架為 FastAPI (Python), NestJS (Node.js), Gin (Go)。
- **決策：** 選擇 FastAPI，因為 Python 生態系對 AI/ML 的原生支援是決定性因素。
- **後果：** 前後端語言不一致，需要獨立的型別同步機制（OpenAPI codegen）。獲得 ML 生態系全面支援。

### ADR-002: 選擇 PostgreSQL + TimescaleDB 而非獨立時序資料庫

- **狀態：** Proposed
- **背景：** 需要同時支援關聯式資料和時序資料。候選方案為 PostgreSQL + TimescaleDB 與 PostgreSQL + InfluxDB。
- **決策：** 選擇 TimescaleDB 作為 PostgreSQL 擴充套件，維持單一資料庫引擎。
- **後果：** 減少維運複雜度，可使用 SQL JOIN 關聯設備與其時序資料。效能上限低於獨立的 InfluxDB，但預估資料量級在 TimescaleDB 承受範圍內。

### ADR-003: 選擇 Zustand 而非 Redux Toolkit 作為狀態管理方案

- **狀態：** Proposed
- **背景：** 目前僅有 LocationContext，需要引入全域狀態管理。
- **決策：** 選擇 Zustand，因為其漸進式引入策略與現有 Context 相容性最佳。
- **後果：** 若未來狀態邏輯極端複雜（50+ slice），可能需要考慮遷移。但 Zustand 的 slice pattern 在中等複雜度下表現良好。此為 **可逆決策**。

### ADR-004: 初期使用 PostgreSQL 全文搜尋，延後引入 Elasticsearch

- **狀態：** Proposed
- **背景：** 前端已有全域搜尋 UI（CMD+K），需要後端搜尋能力。
- **決策：** Phase 2-3 使用 PostgreSQL `pg_trgm` + `tsvector`，Phase 4+ 視資料量和搜尋複雜度決定是否引入 Elasticsearch。
- **後果：** 降低初期基礎設施複雜度。PostgreSQL 全文搜尋對 < 100 萬筆記錄效能足夠。需在設計時保持搜尋介面抽象，使日後切換無痛。

---

## 附錄 B: 技術版本總覽

| 類別 | 技術 | 建議版本 | 用途 |
|------|------|----------|------|
| **前端** | | | |
| | React | 19.2+ | UI 框架（保持現有） |
| | TypeScript | 6.0+ | 型別系統（保持現有） |
| | Vite | 8.0+ | 建置工具（保持現有） |
| | Tailwind CSS | 4.2+ | 樣式系統（保持現有） |
| | react-router-dom | 7.13+ | 路由（保持現有） |
| | i18next | 26.0+ | 國際化（保持現有） |
| | Recharts | 3.8+ | 標準圖表（保持現有） |
| | Zustand | 5.0+ | 全域狀態管理（新增） |
| | @tanstack/react-query | 5.62+ | 資料擷取與快取（新增） |
| | react-hook-form | 7.54+ | 表單管理（新增） |
| | zod | 3.24+ | Schema 驗證（新增） |
| | socket.io-client | 4.8+ | WebSocket 客戶端（新增） |
| | d3 | 7.9+ | 拓撲圖/力導向圖（新增） |
| | Vitest | 3.1+ | 單元測試（新增） |
| | @testing-library/react | 16.1+ | 元件測試（新增） |
| | Playwright | 1.50+ | E2E 測試（新增） |
| **後端** | | | |
| | Python | 3.12+ | 執行環境 |
| | FastAPI | 0.115+ | Web 框架 |
| | Pydantic | 2.10+ | 資料驗證 |
| | Uvicorn | 0.34+ | ASGI 伺服器 |
| | SQLAlchemy | 2.0+ | ORM |
| | Alembic | 1.14+ | 資料庫遷移 |
| | Celery | 5.4+ | 非同步任務佇列 |
| | python-socketio | 5.11+ | WebSocket 伺服器 |
| | anthropic | 0.42+ | Claude API SDK |
| | scikit-learn | 1.6+ | 傳統 ML |
| | PyTorch | 2.6+ | 深度學習 |
| **資料庫** | | | |
| | PostgreSQL | 16+ | 主資料庫 |
| | TimescaleDB | 2.17+ | 時序資料擴充套件 |
| | pgvector | 0.8+ | 向量搜尋擴充套件 |
| | Redis | 7.4+ | 快取/佇列 |
| **DevOps** | | | |
| | Docker | 27+ | 容器化 |
| | Docker Compose | 2.30+ | 開發環境編排 |
| | Kubernetes | 1.31+ | 生產編排 |
| | Prometheus | 2.55+ | 指標收集 |
| | Grafana | 11+ | 視覺化監控 |
| | Loki | 3.3+ | 日誌收集 |

---

> 本文件為 IronGrid Enterprise CMDB + AIOps 平台的技術路線圖初版。隨著專案推進和需求演變，本文件將持續更新。每個 Phase 開始前應進行範圍確認，結束時應進行回顧與調整。
>
> 下一步行動：團隊評審本文件並就 Phase 1 的具體任務分配達成共識。
