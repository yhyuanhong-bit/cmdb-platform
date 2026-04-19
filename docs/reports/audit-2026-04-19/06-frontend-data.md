# 06 — Frontend / Ingestion / Data Contract

> 审计日期: 2026-04-19 · 只读审计,不包含任何修改建议的具体 diff

## Part A: Frontend (cmdb-demo)

### A.1 技术栈与构建

核心来自 `cmdb-demo/package.json:21-64`:

| 维度 | 选择 |
|------|------|
| 框架 | React **19.2** + react-dom 19.2 + react-router-dom **7.13** |
| 语言 | TypeScript **6.0** |
| 构建 | Vite **8.0** + `@vitejs/plugin-react` 6 |
| 样式 | Tailwind CSS **v4**(`@tailwindcss/vite` 插件注入,零 PostCSS 配置) |
| 服务端状态 | `@tanstack/react-query` **5.95** |
| 客户端状态 | `zustand` **5.0**(仅 auth) |
| i18n | `i18next` 26 + `react-i18next` 17 + `i18next-browser-languagedetector` |
| 校验 | `zod` 4 |
| UI 资产库 | `@xyflow/react` 12(拓扑)、`recharts` 3(图表)、`react-leaflet` 5 + `leaflet`(地图)、`elkjs`(布局算法)、`html5-qrcode`(扫码)、`xlsx`(Excel 导入)、`sonner`(toast) |
| 测试 | `vitest` 4 + `@testing-library/react` 16 + `jsdom` 29(单测);`@playwright/test` 1.59(e2e) |

构建入口 `cmdb-demo/vite.config.ts:8-27`:dev server 端口 5175,三路代理:
- `/api/v1/ingestion` → localhost:8081(Python ingestion engine,带 `rewrite` 去前缀)
- `/api/v1/ws` → localhost:8080(WebSocket,`ws: true`)
- `/api/v1` → localhost:8080(cmdb-core Go 服务)

路径别名 `@` → `./src`。

`cmdb-demo/package.json:9-16` 中 `build` 脚本把 `tsc --noEmit` 串在 `vite build` 之前(严格模式构建),且提供 `generate-api` 脚本基于 `openapi-typescript` 从 `../api/openapi.yaml` 生成 `src/generated/api-types.ts`——这是前后端契约的关键锚点。

### A.2 路由与页面清单

入口 `cmdb-demo/src/App.tsx`。41 个 `src/pages/*.tsx` + `bia/` 5 个 + `locations/` 4 个 + `asset-detail/` (1 + 4 tabs + 4 sub components)。**全部使用 `React.lazy()` 按需加载**(`App.tsx:8-82`),这在 SPA 下是正确姿势——首屏不会把 41 个页面一次性打包进 main chunk。

路由拓扑由公开(Login / Welcome)+ 受保护组(`AuthGuard` + `MainLayout`)构成,`AuthGuard` 仅 14 行(`components/AuthGuard.tsx`),是纯跳转守卫。

按职能分组,结合代码行数分布:

| 组 | 路由 / 代表页 | 核心文件(LOC) | 性质 |
|-----|------------|---------------|-----|
| **Location hierarchy** | `/locations/:t/:r/:c` 4 级下钻 | `locations/GlobalOverview.tsx:576`, `CampusOverview.tsx:591`, `CityOverview.tsx:572`, `RegionOverview.tsx:423` | 主力(默认首页 `App.tsx:184` fallback 到 `/locations`) |
| **Dashboard** | `/dashboard` | `Dashboard.tsx:611` | 主力(包含 BIA 段、热力图等固定装饰数据,见 `Dashboard.tsx:15-40` 的 `BIA_SEGMENTS`/`heatmapData`,**装饰用 mock**) |
| **Assets(统一页)** | `/assets`, `/assets/:id`, `/assets/lifecycle`, `/assets/discovery`, `/assets/upgrades`, `/assets/equipment-health` | `AssetManagementUnified.tsx:513`, `asset-detail/AssetDetailUnified.tsx:378` + 4 个 tabs(Overview 377, Health 233, Usage 168, Maintenance 124) | 主力 |
| **Racks** | `/racks*`, `/racks/3d`, `/racks/facility-map`, `/racks/add` | `RackDetailUnified.tsx:**1227**`, `DataCenter3D.tsx:699`, `FacilityMap.tsx:392`, `RackManagement.tsx:512` | 主力,RackDetailUnified **严重超 800 行阈值**(体量仅次于 PredictiveHub) |
| **Inventory** | `/inventory`, `/inventory/detail` | `HighSpeedInventory.tsx:759`, `InventoryItemDetail.tsx:470` | 主力 |
| **Monitoring** | `/monitoring` + 5 个子路由(health, sensors, energy, topology, location-detect) | `SensorConfiguration.tsx:902`, `EnergyMonitor.tsx:772`, `AlertTopologyAnalysis.tsx:740`, `MonitoringAlerts.tsx:426`, `SystemHealth.tsx:465`, `LocationDetection.tsx:229` | 主力,Sensor/Energy/Topology 都 700+ 行级 |
| **Maintenance** | `/maintenance*`, `/maintenance/add`, `/maintenance/workorder`, `/maintenance/dispatch` | `MaintenanceHub.tsx:645`, `WorkOrder.tsx:621`, `MaintenanceTaskView.tsx:452`, `TaskDispatch.tsx:546`, `AddMaintenanceTask.tsx:242` | 主力 |
| **Predictive AI** | `/predictive` 单页多 tab(overview/alerts/insights/recommendations/timeline/forecast) | `PredictiveHub.tsx:**1416**`(单文件6 个 tab,最大页) | 主力,但体量失控 |
| **Audit** | `/audit`, `/audit/detail` | `AuditHistory.tsx:428`, `AuditEventDetail.tsx:342` | 主力 |
| **Quality** | `/quality` | `QualityDashboard.tsx:378` | 中等 |
| **System** | `/system`(roles/permissions),`/system/settings`, `/system/profile`, `/system/sync` | `SystemSettings.tsx:586`, `RolesPermissions.tsx:458`, `SyncManagement.tsx:407`, `UserProfile.tsx:301` | 主力 |
| **BIA** | `/bia` 5 个子路由 | `BIAOverview.tsx:502`, `DependencyMap.tsx:304`, `ScoringRules.tsx:265`, `RtoRpoMatrices.tsx:184`, `SystemGrading.tsx:159` | 主力 |
| **Help / Knowledge** | `/help/troubleshooting`, `/help/videos`, `/help/videos/player` | `TroubleshootingGuide.tsx:378`, `VideoLibrary.tsx:364`, `VideoPlayer.tsx:174` | **疑似空壳/早期实验**——VideoLibrary/VideoPlayer 与 CMDB 核心职能关系薄弱,可能是早期 Demo 遗留 |
| **Onboarding** | `/welcome` | `Welcome.tsx:231` | 轻度(纯静态五步卡片墙,仅 i18n 文案,无 API 调用) |
| **Login** | `/login` | `Login.tsx:99` | 最小 |

"几乎空壳"的候选:`Welcome.tsx`(纯静态引导页,无业务逻辑)、`VideoLibrary` / `VideoPlayer`(`VideoPlayer.tsx` 只有 174 行,且 `VideoLibrary` 和真实 CMDB 数据无联动)、`AssetLifecycleTimeline.tsx:331`(似乎只是 `AssetLifecycle` 页的衍生展示层)。

未挂载到侧边栏/路由的页面:无——41 个全部被路由注册。

### A.3 状态管理

**`stores/` 目录只有 1 个 store**——`authStore.ts`(120 行):

- 使用 Zustand + `persist` middleware + `createJSONStorage(sessionStorage)`(`authStore.ts:19,109-118`)
- 持久字段:`accessToken` / `refreshToken` / `user` / `isAuthenticated`
- 职责:login / logout / refreshTokens / fetchCurrentUser(全部直接 `fetch`,绕开 `apiClient`,避免循环依赖)

**服务端状态完全交给 TanStack Query**。
- 25 个 hook 文件(`src/hooks/use*.ts`),每个对应一个 API 域(`useAssets`, `useMonitoring`, `useBIA`, `useInventory`, ...)
- `QueryProvider.tsx:5-21` 配置:`staleTime: 30s`, `retry: 2`, `refetchOnWindowFocus: false`,`MutationCache.onError` 统一 toast 提示(只要 mutation 没有自定义 `onError`)
- WebSocket 驱动失效:`useWebSocket.ts:16-41` 订阅 asset/alert/maintenance/prediction 四个事件类别,按事件 type 精确 `invalidateQueries`——**服务端推送 → 客户端按域刷新**,这个模式是整份前端代码里最成熟的一块

**URL state 基本没有。**`useSearchParams` 只出现在 3 个文件(`InventoryItemDetail.tsx` / `AuditEventDetail.tsx` / `VideoPlayer.tsx`),用作资源 ID 提取。过滤器/分页/排序均为组件内 `useState`,**刷新页面丢失,且无法分享链接**——这是前端的一个明确短板。

**其它状态载体:**
- `LocationContext.tsx`(React Context)承载当前选中的位置层级(从 URL 派生),见 `main.tsx:16` 的 Provider 栈
- `SyncingOverlay` 通过 `window.dispatchEvent('sync-in-progress')` 事件(`lib/api/client.ts:82`),不是 Zustand,是浏览器自带 CustomEvent——**跨组件通信的这个路径有点野**

### A.4 API 层

**分层清晰:**

1. `src/generated/api-types.ts`(**2021 行**,由 `openapi-typescript` 从 `api/openapi.yaml` 生成,见 `package.json:11`)——契约来源
2. `src/lib/api/client.ts`(130 行)——唯一 `fetch` 封装,导出 `apiClient` 单例与 `ApiRequestError` 类
3. `src/lib/api/{domain}.ts`(19 个域文件:assets, monitoring, bia, inventory, identity, topology, maintenance, prediction, quality, discovery, integration, ingestion, notifications, audit, activity, sensors, sync, upgradeRules + types + client)——每个域导出一个 `{domain}Api` 对象,手写薄包装,调用 `apiClient.get/post/put/patch/del`。共 161 次调用
4. `src/hooks/use{Domain}.ts`(25 个)——把 domain API 包成 `useQuery` / `useMutation`

**客户端特性(`client.ts`):**
- 请求中注入 `Authorization: Bearer {accessToken}`,从 `authStore.getState()` 读取(`client.ts:33-35`)
- 429 限流:指数退避 + `Retry-After` 解析,最多 3 次重试(`client.ts:16-26,52-58`)
- 401 + `INVALID_TOKEN`:自动调用 `refreshTokens()` 并重放一次原请求;失败则 logout(`client.ts:73-79`)
- 503 + `SYNC_IN_PROGRESS`:派发 `sync-in-progress` CustomEvent 供 `SyncingOverlay` 监听(`client.ts:81-83`)
- Dev-only 校验:响应若不是期望的 envelope 形状则 `console.warn`(`client.ts:89-93`)——不报错,有噪声价值但不强约束

**错误模型:**`ApiResponse<T> = { data, meta }`、`ApiListResponse<T> = { data[], pagination, meta }`、`ApiError = { error: { code, message }, meta }`(`lib/api/types.ts:1-27`)——与 openapi 里的 `Meta` / `Pagination` / `ErrorBody`(`api/openapi.yaml:4246-4270`)完全对齐。

**Loading/错误模式:**
- Loading: 每个页面通过 `useQuery` 的 `isLoading`/`isFetching` 自行处理;App 层 `Suspense fallback={<Loading />}`(`App.tsx:99`)
- Toast: `sonner` 全局挂载(`main.tsx:19`),`MutationCache.onError` 自动 toast.error;成功提示是组件级手写 `toast.success(...)`
- 一致性尚可,但缺乏统一的"业务失败(code=X)→ 本地化文案"映射表

### A.5 i18n

三语 locale 全部对齐:`en.json` / `zh-CN.json` / `zh-TW.json` 每个 **2736 行**(完全一致的 key 结构)。

配置 `src/i18n/index.ts:9-27`:**fallbackLng: `zh-TW`**(这是繁体中文优先的非常特定决策),检测顺序 `localStorage → navigator`,缓存到 `localStorage`。

**硬编码中文字符串普查:**
- `.tsx` 文件:14 处,分布于 8 个文件(`WorkOrder.tsx:1`, `LanguageSwitcher.tsx:2`, `HighSpeedInventory.tsx:5`, `SystemHealth.tsx:1`, `MaintenanceTaskView.tsx:2`, `SystemSettings.tsx:1`, `UserProfile.tsx:1`, `AlertTopologyAnalysis.tsx:1`)——量级很小,属于少数漏网
- `.ts`(非组件)文件:29 处,**全部集中在** `src/data/locationMockData.ts:29`(示范数据)

结论:**覆盖度极高**。24 个组件内出现"mock"/"fallback"/"hardcoded" 字样的属大量装饰性示例文案,不是 i18n 漏网——见 `Dashboard.tsx:15-70` 里的 `BIA_SEGMENTS`/`CRITICAL_EVENTS` 等。这些**硬编码演示数据**才是更大的债务(下文 A.8)。

### A.6 UI 设计一致性

**没有 Radix / shadcn / MUI** 等第三方 headless 组件库。所有 UI primitive 自写:

- `src/components/` 24 个自写组件,其中 Modal 占大头(`Create*Modal`、`Edit*Modal` 共 14 个),尺寸从 22 行(`CreateLocationModal`)到 336 行(`ScanManagementTab`)不等
- `StatCard.tsx:22`、`StatusBadge.tsx:29`、`Icon.tsx:3` 是最基础的 atoms,薄到近于空壳
- 无"设计系统"级别的结构:没有 `ui/Button`、`ui/Input`、`ui/Dialog` 的共享 primitive

**设计 token 集中度:高。**`src/index.css:30+` 用 Tailwind v4 `@theme { ... }` 集中定义了 Material 风格 token:`--color-primary`、`--color-primary-container`、`--color-on-primary`、`--color-surface`、`--color-background`、`--color-error` 等(Material Design 3 调色板风格),组件里直接用 `bg-surface-container`、`text-on-surface-variant` 等语义类——全仓统一 dark 主题,**没有 light/dark 切换开关**(`LanguageSwitcher.tsx:56` 只有语言切换)。

**图标体系:**`material-symbols-outlined`(Google Material Symbols)通过 CSS font 加载(`index.css:84+`),组件里包装成 `<Icon name="dns" />`(3 行的 `Icon.tsx`)。

整体判断:**视觉语言统一(Material 风 dark)**,但缺 primitive 层抽象,14 个 Modal 的共性代码几乎肯定有重复——这是 A.8 的一个具体债务。

### A.7 测试

**单元测试(vitest)覆盖面极薄。**仅 4 个 `.test.ts`:
- `src/stores/authStore.test.ts`
- `src/lib/api/client.test.ts`
- `src/lib/api/topology.test.ts`
- `src/contexts/LocationContext.test.ts`

**零组件测试**(没有任何 `*.test.tsx`),违反 CLAUDE.md 规定的"80% 覆盖"要求。

**E2E (Playwright) 结构合理但浅:**

- `e2e/critical/` 5 个 spec:alerts、asset-crud、auth-dashboard、inventory、work-order(核心用户流)
- `e2e/extended/` 10 个 spec:audit、bia、datacenter-3d、energy、locations、permissions、predictive、quality、settings、sync
- `e2e/helpers/auth.ts` 统一登录辅助

但抽样的 `asset-crud.spec.ts` 全文只有 9 行——**仅验证 `/assets` 页能出现 `Asset` 文本**,不是真正的 CRUD 验证;这是"存在感"级别的冒烟测试而非完整覆盖。

`playwright.config.ts`:`webServer` 靠 `npm run dev`,`reuseExistingServer: true`,CI 时 `workers: 1`、`retries: 1`——配置规范,但**没配跨浏览器**(缺 `projects: [{chromium}, {firefox}, {webkit}]`),也**没跑响应式断点**(违反 /root/.claude/rules/web/testing.md 里关于 320/768/1024/1440 的要求)。

### A.8 前端风险 / 缺口

| # | 具体问题 | 位置 / 证据 | 严重度 |
|---|---------|-----------|-------|
| 1 | **巨型单文件 page**,超 CLAUDE.md 800 行硬阈值 | `PredictiveHub.tsx:1416`(6 tab 融一锅)、`RackDetailUnified.tsx:1227`、`SensorConfiguration.tsx:902` | HIGH |
| 2 | **组件测试完全缺失** | 0 个 `*.test.tsx`;仅 4 个 store/lib 级单测 | HIGH |
| 3 | **E2E 流于浅层冒烟** | `asset-crud.spec.ts` 仅 9 行断言"能看到 Asset 字样" | HIGH |
| 4 | **URL state 缺席** | 过滤器/分页/排序不进 URL(`useSearchParams` 仅 3 处,均做 id 读取),违反 /root/.claude/rules/web/patterns.md | MEDIUM |
| 5 | **硬编码装饰数据遍布页面** | `Dashboard.tsx:15-70`(`BIA_SEGMENTS`/`HEATMAP`/`CRITICAL_EVENTS`)、`src/data/fallbacks/*.ts`(fallback 静态数据,`predictive.ts` 118 行)、`src/data/locationMockData.ts:385` | MEDIUM(UI 看起来"已完成",真实数据缺位时感知不到) |
| 6 | **CustomEvent 跨组件通信** | `client.ts:82` 派发 `sync-in-progress`,`SyncingOverlay` 订阅 | MEDIUM(不 typesafe,但只用 1 处,可控) |
| 7 | **Modal 家族缺统一抽象** | 14 个 `Create*Modal`/`Edit*Modal` 各写各的,尺寸 22–336 行 | MEDIUM |
| 8 | **没有 light/dark 切换** | `index.css` 只有一套 token,无 `[data-theme="light"]` | LOW |
| 9 | **`authStore.ts:36,52` 仍有 `console.error`** | 虽受 `import.meta.env.DEV` 门控,但生产代码规范应走 logger | LOW |
| 10 | **疑似近乎空壳页** | `VideoLibrary.tsx:364` / `VideoPlayer.tsx:174` / `Welcome.tsx:231`(纯静态卡片,无 API) | LOW(审视是否删除) |
| 11 | **`as any` 仅 1 处** | `locations/GlobalOverview.tsx:1`(全仓 1 次),配合最近的"50/51 casts 消除"commit(eb72127),类型收紧基本已完成 | GOOD |

---

## Part B: Ingestion Engine

### B.1 职责与技术栈

`ingestion-engine/pyproject.toml:6-26`:

- **Python 3.12+** FastAPI 0.115 服务
- 数据库:`asyncpg` 0.30(直接用 asyncpg,**不走 ORM**——和 cmdb-core 的 sqlc 生成代码保持"SQL 原生"的一致风格)
- 任务队列:`celery[redis]` 5.4 + Redis 5.2;主服务 FastAPI + `uvicorn[standard]`
- 消息总线:`nats-py` 2.9(与 cmdb-core 共享的 eventbus)
- 数据校验:`pydantic` 2.10 + `pydantic-settings`
- 文件解析:`openpyxl` 3.1(Excel)+ `python-multipart`(上传)
- **网络采集**:`pysnmp` 7(SNMP v1/v2c/v3)、`asyncssh` 2.14(SSH)、`pyghmi` 1.5(IPMI/BMC)
- 加密:`cryptography` 42(凭据加密存储,见 `credentials/encryption.py`)
- HTTP 客户端:`httpx`(调用 cmdb-core REST;见 `tasks/discovery_task.py:22` 硬编码 `CMDB_CORE_URL = "http://localhost:8080/api/v1"`,**dev-only**,需要迁移到配置)

**代码规模:**`app/` 共 3911 行 Python(`wc -l`)。最大文件是 `collectors/snmp.py:510`,其次 `collectors/ipmi.py:299`、`collectors/ssh.py:278`、`pipeline/processor.py:279`。

**它 ingest 什么?**
- **文件导入**:Excel + CSV → `importers/excel_parser.py:203`(`routes/imports.py` 5 个 endpoint:upload / preview / confirm / progress / templates)
- **自动发现**:SNMP / SSH / IPMI 三种 collector 扫 CIDR 段;三种 collectors 共享 `collectors/base.py` 抽象(`CollectorRegistry`),`manager.py:70` 做 dispatch
- **MAC 扫描**:`tasks/mac_scan_task.py:159` + `main.py:20,23-40` 开启**进程内周期任务**(每 300 秒),扫 switch MAC 表推位置检测
- **BMC 采集**:IPMI 采 `bmc_ip` / `bmc_type` / `bmc_firmware`

全部走统一的 pipeline:`pipeline/processor.py`(279 行)。

### B.2 数据流

路线(从 `pipeline/processor.py:process_single`):

```
RawAssetData
  │
  ▼ pipeline/normalize.py (211)   字段别名归一:hostname→name, ilo→bmc_ip, ...
  │
  ▼ pipeline/deduplicate.py (97)  serial_number → bmc_ip → ip_address → asset_tag 四级匹配
  │
  ├── 新资产 ──► validate_for_create ──► _create_asset (processor.py:159-279, 直接 INSERT)
  │
  └── 已有资产 ──► validate_for_update
                   ──► pipeline/authority.py (137)  按 (field, source) 优先级决策
                        │
                        ├── auto_merge(source 优先级 >= 最大 且 > 0)
                        └── create_conflicts 写入 import_conflicts,等人工处理
```

**Source authority** 用 `asset_field_authorities` 表做字段级权威(`ingestion-engine/db/migrations/000010_ingestion_tables.up.sql:1-9`);例如 `bmc_ip` 的优先级 ipmi(100) > snmp(80) > excel(60) > manual(50)(见 `000011_bmc_field_authorities.up.sql`)——这是一个**非常清晰的"按来源可信度消冲突"**模型。

**和 cmdb-core 的分工边界:**

- **cmdb-core (Go)**:交易型 REST CRUD、BIA、monitoring/alerts、maintenance workflow、RBAC、审计、TimescaleDB 指标查询——即所有"业务规则 + 直接被 UI 读写"的面
- **ingestion-engine (Python)**:**数据流入**唯一通道——文件批量导入、网络自动发现、位置检测。**只管写**(写 assets / discovery_* / import_* 表),读得很少。

**为何另起一个服务而不是在 core 里做?** 从代码证据推测三条理由:
1. **生态契合度**:`pysnmp` / `asyncssh` / `pyghmi`(IPMI)在 Python 生态是事实标准,Go 里没有成熟等价物
2. **工作负载隔离**:长跑的批量 import/discovery(Celery 任务)与低延迟 REST 流量解耦,资源独立扩容
3. **pipeline 可插拔**:`normalize → dedup → validate → authority` 是典型的数据清洗管线,Python 表达更紧凑

**调用拓扑:**
- 前端 `/api/v1/ingestion/*` → Vite 代理 rewrite 到 `localhost:8081`(去前缀,`vite.config.ts:12-16`),打到 ingestion-engine
- ingestion-engine → PostgreSQL(直接写,和 cmdb-core **同库**)
- ingestion-engine → NATS(`events.py:45`,`close_nats` / `connect_nats`,`publish_event`),cmdb-core 订阅这些事件反过来更新自己的派生状态
- ingestion-engine → cmdb-core REST(`tasks/discovery_task.py:22` 的 `CMDB_CORE_URL`,用 `httpx`)

### B.3 观察到的风险

1. **共享同一个 DB schema**——`ingestion-engine/db/migrations/` 只有 2 个迁移(000010、000011),和 cmdb-core/db/migrations 是**同一序号空间**(cmdb-core 主动**跳过 000010**:`000009_timescaledb_metrics` 之后直接 `000011_prediction_tables`——说明 000010 号位被 ingestion-engine 占用,001011 在 cmdb-core 是 prediction_tables,在 ingestion-engine 是 bmc_field_authorities——**撞号但内容完全无关**)。这是一个**隐形的耦合**:两边的迁移 SQL 文件各自独立,靠人类约定不冲突,迁移工具(golang-migrate / 或类似)的 `schema_migrations` 表只有一个全局序号——**风险:若有人同时加了两个 000040 迁移,会死锁**。应在根仓库有一个统一迁移注册表。

2. **失败重试策略:**
   - HTTP 级:ingestion-engine 对外(调 cmdb-core)没看到统一重试;import pipeline 内部 `process_batch` 用 `try/except` 吞所有异常并仅计 `stats["errors"] += 1`(`processor.py:145-146`),**丢弃错误上下文**——只计数不记录哪一行哪个原因,对"Fix implementation, not tests"原则的支持很弱
   - Celery 任务:`@celery_app.task(bind=True, ...)` 没配 `autoretry_for` / `retry_backoff`(见 `discovery_task.py:55-56`、`import_task.py:28-29`)——任务失败就是死信
   - WebSocket 事件推送失败(`events.py`)也未见重试;只 best-effort publish

3. **进程内周期 MAC 扫描**(`main.py:_periodic_mac_scan`)——用 `asyncio.create_task` + `while True`:sleep 5 分钟。**单副本场景下 OK,多副本部署时会并发重复扫描**(没做 leader election / Redis 锁)。这是典型"能跑起来的 MVP,水平扩展会打脸"。

4. **凭据加密 dev fallback:**`config.py:22-29` — `deploy_mode == "development"` 时允许 `credential_encryption_key = "0" * 64`——**生产必须 FATAL,development 自动放行**,逻辑写得明确,但注意若 env 没设 `INGESTION_DEPLOY_MODE`,默认是 `"development"`(`config.py:11`),部署忘配时会静默使用 0 key。

5. **`CMDB_CORE_URL` 硬编码**(`discovery_task.py:22`)——应读 env。

6. **Test 覆盖**:`tests/` 有 8 个 pytest 文件(authority / discovery_task / encryption / excel_parser / ipmi_collector / normalize / snmp_collector / ssh_collector / validate),覆盖 pipeline 和各 collector——比前端好,但没 route 层集成测试(Imports / Discovery endpoint 的 contract test 缺)。

---

## Part C: 数据契约

### C.1 OpenAPI (api/openapi.yaml)

**规模:**5682 行,**126 个 path**(grep `^  /`)、~140 个 operation(按 `docs/OPENAPI_DRIFT.md:9` 声明)。

**认证:** `BearerAuth` = HTTP bearer JWT,默认全局 `security: [BearerAuth: []]`(`openapi.yaml:8-9,4194-4198`),个别 public endpoint 需要覆写为 `security: []`(未检查,但从 `/auth/login` 模型推断是这种约定)。

**错误模型:**统一 envelope:

```yaml
Meta: { request_id }                       # 4246-4251
Pagination: { page, page_size, total, ...} # 4253-4264
ErrorBody: { code, message }               # 4266+
ErrorResponse: { error: ErrorBody, meta }  # (未展开但 BadRequest/Unauthorized/NotFound refs 指向它)
```

三个共享参数:`IdPath`(uuid)、`Page`(min=1 default=1)、`PageSize`(min=1 max=100 default=20)——标准化良好。

**Tags 分类**(`openapi.yaml:11-41`):auth / topology / assets / maintenance / monitoring / inventory / audit / dashboard / identity / prediction / system / integration / bia / quality / discovery —— 15 个,与前端 `src/lib/api/{domain}.ts` 几乎一一对应。

**Drift 状况**(`docs/OPENAPI_DRIFT.md`):

- **Documented** ~140,**missing from spec** 2 个基础设施 endpoint(`GET /ws`、`POST /admin/migrate-statuses`——白名单允许),**in spec but unregistered** 0
- **混合 handler 模型**——Track A(oapi-codegen `RegisterHandlers` 生成的 typed handler)和 Track B(手写 `*gin.Context`)**并行**。`cmdb-core/oapi-codegen.yaml` 的 `exclude-operation-ids` 列表共 **60 个 Track B 操作**,由 `main.go:349-449` 显式路由
- **CI 守卫:**`.github/workflows/openapi-health.yml` 两道闸门:① `make generate-api` 后 git diff 必须干净(防止 generated.go 被手编);② `cmd/check-api-routes` 比对 spec vs main.go,存在白名单机制

**关键结论:**前端与后端契约的**权威来源是 openapi.yaml**(前端靠 `openapi-typescript` 生成 2021 行类型,后端靠 `oapi-codegen` 生成 ServerInterface);drift 已系统化治理。

**欠缺:**
- 60 个 Track B 操作不进入 ServerInterface,handler 签名自由——它们的请求/响应**还是靠 spec 校验,但没有编译期保证**(手写 handler 可以和 spec 漂移而不报错)。OPENAPI_DRIFT.md 也承认这一债务("Remediation backlog 1")
- 无 contract test(`docs/reports/` 里也没看到 schemathesis / prism 类的 runtime spec validation CI job)

### C.2 数据库迁移

**cmdb-core/db/migrations:** `000001` → `000039`,**38 个实际迁移**(000010 由 ingestion-engine 占用,000024 是显式 `SELECT 1;` 占位符——见 `000024_placeholder.up.sql:1-2` 注释"Intentionally skipped")。

**成长脉络(按阶段):**

| 阶段 | 序号 | 主题 |
|-----|------|-----|
| Bootstrap | 001–008 | 扩展 / 多租户 / locations / assets+racks / maintenance / monitoring / inventory / audit |
| TimescaleDB | 009 | metrics 时序表 |
| **ingestion** | **010** | (由 ingestion-engine 提供)import_conflicts / discovery_tasks / authorities |
| 功能线迭代 | 011–017 | prediction / integration / bia / webhook_bia_filter / quality / discovered_assets / discovery_collectors |
| **Phase 3–4** | 018–020 | 按 phase 分组扩容(`phase3_tables` / `phase4_group1` / `phase4_group2`)——说明代码里存在"大阶段性工程"批量建表 |
| 质量 + 硬化 | 021–023 | soft_delete_location_filter / inventory_items_indexes / database_hardening |
| 占位 | 024 | `SELECT 1;`(保持序号连续) |
| 25–34 | work_order_redesign / timescaledb_compression / sync_system / audit_and_constraints / unified_soft_delete / sync_phase_a / inventory_items_sync / location_detect / location_coordinates / uniqueness_constraints |
| **近期 5 个**(000035–000039) | 下一节 |

**近 5 个迁移(意图):**

| 序号 | 文件 | 意图 |
|-----|------|-----|
| **000035** | `asset_bmc_fields.up.sql` | 给 `assets` 加 `bmc_ip/bmc_type/bmc_firmware` 三列 + 部分索引。**为 IPMI 采集铺路**,对应 Part B 的 BMC collector |
| **000036** | `asset_warranty_lifecycle.up.sql` | 加 `purchase_date / purchase_cost / warranty_* / expected_lifespan_months / eol_date`(8 列)+ warranty_end 索引。**资产生命周期管理表面化**,对应前端 `AssetLifecycle.tsx` / `AssetLifecycleTimeline.tsx` |
| **000037** | `rack_management_fixes.up.sql` | 两处修复:① `rack_slots.asset_id` FK 改为 `ON DELETE SET NULL`(原本 restrict/cascade 导致删 asset 失败);② `racks` 加 `updated_at` 以支持审计 |
| **000038** | `encrypt_integration_secrets.up.sql` | 加 `integration_adapters.config_encrypted BYTEA` + `webhook_subscriptions.secret_encrypted BYTEA`——**双写滚动迁移第一阶段**,配合 `docs/integration-encryption-deployment.md` 和 `db/scripts/cleanup/clear_integration_plaintext.sql` 最后清 plaintext |
| **000039** | `adapter_failure_state.up.sql` | `integration_adapters` 加 `consecutive_failures / last_failure_at / last_failure_reason / next_attempt_at` 四列 + 部分索引。**修复 in-memory `adapterFailures` race condition**,让指数退避 gating 跨进程重启保持(注释:"avoids the previous in-memory-only adapterFailures map race condition")。这是从 Bug 反推出的 schema 演化,非常健康 |

**`db/scripts/cleanup/` 的维护脚本**(`cmdb-core/db/scripts/cleanup/`):
- `clear_integration_plaintext.sql` + `README.md`(48 行说明)
- **不会自动执行**:`main.go` 的启动 migrator 只扫 `db/migrations/`(README.md:3-5 明确说明)。运维手动跑,前提条件(000038 已应用、双写周期走完、有 pg_dump)在 README 里清楚列出。**一次性事务、pre-flight guard、per-tenant 审计事件、幂等** —— 写得专业

### C.3 schema 与代码一致性

**sqlc 生成(`cmdb-core/internal/dbgen/`):**22 个 `*.sql.go` 文件,每个对应一张表/域(alert_events / alert_rules / assets / audit_events / bia / discovery / incidents / integration / inventory_tasks / locations / metrics / notifications / prediction_* / quality / rack_slots / racks / roles / tenants / users / work_orders)。

`sqlc.yaml`(`cmdb-core/sqlc.yaml`)配置:
- queries dir: `db/queries`(22 个 `.sql` 手写查询,和 dbgen 一一对应)
- schema: `db/migrations/`——**直接读迁移 SQL 作为 schema,不需要单独维护 DDL**
- overrides: `uuid → google/uuid.UUID`、`ltree → string`、`jsonb → encoding/json.RawMessage`、`timestamptz → time.Time`、`text[] → pgx/v5/pgtype.FlatArray[string]`

**覆盖情况**:`docs/DATABASE_SCHEMA.md:9` 报告共 **52 张表**,sqlc 覆盖 22 张。**剩下的 30 张**(含 `asset_dependencies`、`asset_field_authorities`、`asset_location_history`、`import_conflicts`、`import_jobs`、`discovery_candidates`、`mac_address_cache`、`switch_port_mapping`、`sync_*`、`user_sessions`、`webhook_*`、`work_order_comments/logs`、`sensors`、`rack_network_connections`、`prediction_models/results`……)——要么是 ingestion-engine 独占的表(它走 asyncpg 原生 SQL,**完全绕开 sqlc**),要么是 cmdb-core 手写 query 绕过 sqlc 的候选。

**偏差验证入口**:前文 `OPENAPI_DRIFT.md` 里的 Track B 60 个操作(含 upgrade-rules、user-roles、sensors、sync、location-detect、notifications……)——**很大概率正是"手写 gin handler + 手写 DB query,绕开 sqlc"的集合**。这是一个结构性债务:三个维度互相印证:**typed handler 少一组、sqlc 生成缺一组、手写 SQL 多一组**。

---

## 整体评估

### 前后端契约健康度

**中等偏上。**

**优点:**
- `openapi.yaml` 是单一真相源,前端用 `openapi-typescript`、后端用 `oapi-codegen`、CI 用 `check-api-routes` 三重护航
- envelope / error / pagination / meta 四类模型标准化,19 个前端 domain API 和 15 个后端 tag 结构一一对应
- 最近一次 commit(`eb72127`)把 `as any` 从 51 降到 1,类型收紧已几乎完成
- WebSocket 事件驱动的 React Query invalidation 是整套方案里最成熟的机制

**硬伤:**
- **60 个 Track B 操作脱离编译期契约**——手写 handler + 可能手写 SQL,和 spec 的一致只靠人工。这是后端最大结构债务
- **sqlc 覆盖 22/52 张表**,52 张里只有 42%(另外一部分走 ingestion-engine,但 core 内手写 SQL 的比重仍不小)
- ingestion-engine 和 cmdb-core **共享 DB schema 但迁移序号各管各的**,靠约定不撞号(目前已约定在 000010 这个号位)

### 哪个部分是最大的债务

**排序(最大到最小):**

1. **前端巨型单文件 page**——`PredictiveHub.tsx` 1416 行、`RackDetailUnified.tsx` 1227 行、`SensorConfiguration.tsx` 902 行。违反项目自身 CLAUDE.md 的 800 行硬阈值。影响所有后续维护、测试、code-reviewer agent 的工作量
2. **前端组件测试空白**——0 个 `*.test.tsx`,E2E 又是冒烟级(`asset-crud` 9 行断言一个字符串)。"80% coverage" 目标目前无法成立
3. **Track B 60 个 handler 脱离类型契约**——`docs/OPENAPI_DRIFT.md` "Remediation backlog 1" 已承认,但没有行动计划
4. **前端 URL state 缺席**——所有筛选/分页用 `useState`,刷新即丢,链接无法分享。工作量不大但体验债务明显
5. **ingestion-engine 的进程内定时扫描 + 无 Celery 重试配置**——单副本能跑,横向扩展就会打脸;错误被吞成 `stats["errors"] += 1`,丢失上下文
6. **装饰性硬编码数据遍布多个 page**——`Dashboard.tsx` 等页面的 `BIA_SEGMENTS`/`HEATMAP`/`CRITICAL_EVENTS`、`src/data/fallbacks/`——让 UI 看起来"完整"但真实接入时会错位
7. **cmdb-core 和 ingestion-engine 迁移序号共享空间**——目前靠"000010 让给 ingestion"约定,无工具层护栏
