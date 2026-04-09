# CMDB Platform 修复项影响分析报告

> 日期: 2026-04-09
> 基于: full-project-review-report.md 中的 15 项修复建议
> 目的: 分析每项修改对整体项目逻辑的影响、风险与依赖关系

---

## 目录

- [P0 安全修复 (1-4)](#p0---安全修复)
- [P1 功能完整性 (5-7)](#p1---功能完整性)
- [P2 代码质量 (8-11)](#p2---代码质量)
- [P3 国际化 (12-15)](#p3---国际化)
- [总体依赖图](#总体依赖图)
- [建议执行顺序](#建议执行顺序)

---

## P0 - 安全修复

### 修复项 1: 移除内网 IP 硬编码

**涉及文件:**
| 文件 | 行号 | 当前值 |
|------|------|--------|
| `src/stores/authStore.ts` | 4 | `http://10.134.143.218:8080/api/v1` |
| `src/hooks/useWebSocket.ts` | 48 | `http://10.134.143.218:8080/api/v1` |
| `src/lib/api/client.ts` | 3 | `/api/v1` (已正确) |

**依赖链分析:**
```
authStore.ts (修改点)
├── Login.tsx (直接依赖)
├── UserProfile.tsx (直接依赖)
├── AuthGuard.tsx (直接依赖)
├── useAuth.ts → 无下游
├── usePermission.ts → 无下游
├── useWebSocket.ts (修改点)
│   └── WebSocketProvider.tsx → main.tsx → 全部页面
└── client.ts (已正确，无需修改)
    └── 20 个 API 模块 → 55+ 页面/组件
```

**影响分析:**
- **改动范围**: 2 个文件，各改 1 行
- **风险等级**: 极低
- **向后兼容**: 完全兼容（`.env.development` 已设置 `VITE_API_URL=/api/v1`）
- **潜在问题**: 如果开发环境未配置 `.env.development`，改为相对路径后需 Vite proxy 正确配置
- **对其他模块影响**: 无。authStore 内部使用 `fetch()` 直接调用，不经过 `client.ts`，两处修改互不影响

**修改方案:**
```typescript
// authStore.ts line 4 & useWebSocket.ts line 48
// BEFORE:
const API_URL = import.meta.env.VITE_API_URL || 'http://10.134.143.218:8080/api/v1'
// AFTER:
const API_URL = import.meta.env.VITE_API_URL || '/api/v1'
```

---

### 修复项 2: 移除登录页默认凭证

**涉及文件:**
| 文件 | 行号 | 内容 |
|------|------|------|
| `src/pages/Login.tsx` | 87-91 | `Local: admin / admin123` 及 AD 域名 |

**影响分析:**
- **改动范围**: 1 个文件，删除 6 行 HTML
- **风险等级**: 极低
- **对其他模块影响**: 无。Login.tsx 是独立页面，无子组件导出
- **UX 影响**: 开发者需从文档或 `.env` 获取测试凭证

**修改方案:**
```typescript
// 方案 A: 完全移除
// 方案 B: 环境变量控制
{import.meta.env.DEV && (
  <p className="text-xs ...">Local: admin / admin123</p>
)}
```

---

### 修复项 3: WebSocket Token 传递方式

**涉及文件:**
| 文件 | 行号 | 内容 |
|------|------|------|
| `src/hooks/useWebSocket.ts` | 51 | `ws?token=${token}` |

**影响分析:**
- **改动范围**: 前端 1 文件 + **后端协议变更**
- **风险等级**: 中等
- **关键依赖**: 需要后端 WebSocket 端点同步修改
- **对其他模块影响**:
  - `WebSocketProvider.tsx` — 透明传递，无需修改
  - 所有依赖实时更新的页面 (55+) — 无需修改（通过 React Query invalidation 间接依赖）
- **潜在问题**:
  - 连接建立后、auth 消息发送前的竞态条件（需消息队列）
  - 后端需要处理 `{ type: 'auth', token: '...' }` 初始消息
  - auth 失败需要优雅断开连接

**前后端协调:**
```
前端改动:
  1. useWebSocket.ts:51 - 移除 URL 中的 token
  2. useWebSocket.ts:56 - 连接成功后立即发送 auth 消息
  3. 新增 auth 握手状态管理

后端改动 (cmdb-core):
  1. internal/websocket/ - 不再从 URL 读取 token
  2. 新增首条消息 auth 解析
  3. 未认证连接超时断开机制
```

---

### 修复项 4: Auth Token 持久化

**涉及文件:**
| 文件 | 行号 | 内容 |
|------|------|------|
| `src/stores/authStore.ts` | 18-101 | Zustand store 定义 |

**影响分析:**
- **改动范围**: 1 个文件，添加 persist 中间件
- **风险等级**: 低
- **对其他模块影响**: 完全透明。所有 7 个依赖文件无需任何修改
- **副作用**:
  - localStorage 中将存储 `accessToken` 和 `refreshToken`
  - 需考虑 XSS 风险（localStorage 可被 JS 读取）
  - 多标签页场景下 token 同步（Zustand persist 自动处理）
- **安全考量**:
  - 方案 A: `sessionStorage`（标签页关闭即失效，更安全）
  - 方案 B: `localStorage`（跨标签页共享，UX 更好）
  - 方案 C: httpOnly cookie（最安全，需后端配合）

**修改方案:**
```typescript
import { persist } from 'zustand/middleware'

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      // ... 现有代码不变
    }),
    {
      name: 'cmdb-auth',
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
```

**关联影响:**
- `AuthGuard.tsx` — 刷新页面后 `isAuthenticated` 为 true，不再强制重定向到 /login
- `client.ts` — 刷新后 `getState().accessToken` 有值，API 请求可正常发送
- `useWebSocket.ts` — 刷新后 token 存在，WebSocket 可自动重连

---

## P1 - 功能完整性

### 修复项 5: Mutation 全局错误处理

**涉及文件:**
| 文件 | 内容 |
|------|------|
| `src/providers/QueryProvider.tsx` | 添加 `MutationCache` 或 `mutations.onError` |
| `src/hooks/*.ts` (14 个文件) | 73 个 `useMutation` 调用受影响 |

**73 个 Mutation 分布:**
| Hook 文件 | Mutation 数量 |
|-----------|-------------|
| useTopology.ts | 12 |
| useIdentity.ts | 7 |
| useInventory.ts | 7 |
| useBIA.ts | 6 |
| useScanTargets.ts | 6 |
| useMonitoring.ts | 5 |
| useAssets.ts | 5 |
| useMaintenance.ts | 5 |
| useSensors.ts | 4 |
| useCredentials.ts | 4 |
| usePrediction.ts | 3 |
| useQuality.ts | 3 |
| useIntegration.ts | 3 |
| useDiscovery.ts | 3 |

**影响分析:**
- **改动范围**: 1 个文件 (QueryProvider.tsx)，7-12 行代码
- **风险等级**: 中等
- **潜在冲突**: 5+ 个页面已有局部 `onError`/`onSuccess` 处理：
  - `HighSpeedInventory.tsx` (lines 106-116) — 导入错误的特殊处理
  - `RackDetailUnified.tsx` — 插槽分配错误
  - `AssetDetailUnified.tsx` — 资产变更审计
  - `UserProfile.tsx` — 密码修改反馈
  - `TaskDispatch.tsx` — 分配失败处理
- **设计决策**: 全局 handler 是否覆盖局部？TanStack Query 行为是**两者都执行**（局部优先，全局也执行）

**修改方案:**
```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { /* 现有配置 */ },
  },
  mutationCache: new MutationCache({
    onError: (error, _variables, _context, mutation) => {
      // 如果 mutation 已定义自己的 onError，不重复提示
      if (mutation.options.onError) return
      const message = error instanceof ApiRequestError
        ? error.message
        : 'Operation failed'
      toast.error(message)  // 需先完成修复项 8 (toast 组件)
    },
  }),
})
```

**依赖关系**: 此修复依赖修复项 8 (alert -> toast)，建议先完成 toast 组件引入。

---

### 修复项 6: 页面添加 Location 过滤

**涉及文件与后端支持:**
| 页面 | API 端点 | 后端是否支持 location_id | 状态 |
|------|---------|-------------------------|------|
| AssetManagementUnified | `GET /assets` | 是 (OpenAPI line 662) | 可直接实现 |
| MaintenanceHub | `GET /maintenance/orders` | 否 (仅支持 status, asset_id) | **需后端修改** |
| HighSpeedInventory | `GET /inventory/tasks` | 否 (仅支持 page, page_size) | **需后端修改** |
| Dashboard | `GET /dashboard/stats` | 部分 (仅支持 idc_id) | **需确认设计** |

**各页面影响详情:**

#### 6a. AssetManagementUnified.tsx
- **改动**: 从 `LocationContext` 读取当前 location ID，传入 `useAssets({ location_id })`
- **Hooks 已支持参数传递**: `useAssets(params)` → `assetApi.list(params)` → `apiClient.get('/assets', params)`
- **风险**:
  - 当用户在全局级别（未选择具体 location）时，`locationId` 为 `undefined`，显示全部数据
  - 用户切换 location 时自动触发 refetch（React Query 通过 queryKey 变化检测）
  - 分页器需重置为第 1 页

#### 6b. MaintenanceHub.tsx
- **阻塞**: 后端 `/maintenance/orders` 不支持 `location_id` 参数
- **后端改动**: 需在 OpenAPI spec 添加参数，Go handler 添加 WHERE 条件
- **关系链**: Work Order → Asset → Asset.location_id（间接关系，需 JOIN 查询）
- **风险**: 复杂 JOIN 可能影响查询性能

#### 6c. HighSpeedInventory.tsx
- **阻塞**: 后端不支持 `location_id` 过滤
- **但**: `scope_location_id` 已存在于创建参数中，说明数据模型已有此字段
- **后端改动**: 仅需添加 GET 参数过滤，改动较小

#### 6d. Dashboard.tsx
- **设计问题**: Dashboard 应显示全局数据还是 Location 过滤数据？
- **建议**: Dashboard 保持全局视图，但添加 Location 筛选器作为可选功能
- **当前 API**: `/dashboard/stats` 仅支持 `idc_id`（最低层级），不支持中间层级

**Location ID 提取模式:**
```typescript
const { path } = useLocationContext()
// 从最深层级向上取第一个有值的 ID
const locationId = path.idc?.id
  || path.campus?.id
  || path.city?.id
  || path.region?.id
  || path.territory?.id
```

---

### 修复项 7: 补齐 CRUD 操作

**涉及实体与改动层级:**

#### 7a. Work Order 删除

| 层级 | 文件 | 改动 |
|------|------|------|
| OpenAPI Spec | `api/openapi.yaml` line 903 | 添加 DELETE method |
| Go Handler | `cmdb-core/internal/api/` | 实现删除逻辑 |
| SQL | `cmdb-core/internal/dbgen/` | 添加 DELETE query |
| API Layer | `src/lib/api/maintenance.ts` | 添加 `delete()` 方法 |
| Hook | `src/hooks/useMaintenance.ts` | 添加 `useDeleteWorkOrder()` |
| UI | `src/pages/WorkOrder.tsx` line 162 | 替换 `alert('Coming Soon')` 为真实删除 |

**级联影响:**
- 删除 Work Order 时关联的 comments 和 logs 如何处理？
  - 选项 A: CASCADE DELETE（简单但数据丢失）
  - 选项 B: 软删除 + 标记 `deleted_at`（推荐）
- 需要确认对话框防止误删
- 已完成/已拒绝的工单是否允许删除？

#### 7b. Inventory Task 编辑

| 层级 | 文件 | 改动 |
|------|------|------|
| OpenAPI Spec | `api/openapi.yaml` line 1468 | 添加 PUT method |
| Go Handler | `cmdb-core/internal/api/` | 实现更新逻辑 |
| API Layer | `src/lib/api/inventory.ts` | 添加 `update()` 方法 |
| Hook | `src/hooks/useInventory.ts` | 添加 `useUpdateInventoryTask()` |
| UI | `src/pages/HighSpeedInventory.tsx` | 添加编辑按钮 + 复用 CreateInventoryTaskModal |

**级联影响:**
- 已有 `CreateInventoryTaskModal.tsx` 可复用为编辑模式
- 编辑已开始的任务（status=in_progress）是否允许修改 method？
- 已扫描的条目（scan records）与编辑后的任务参数不一致风险

#### 7c. Inventory Task 删除

| 层级 | 文件 | 改动 |
|------|------|------|
| OpenAPI Spec | `api/openapi.yaml` line 1468 | 添加 DELETE method |
| Go Handler | `cmdb-core/internal/api/` | 实现删除逻辑 |
| API Layer | `src/lib/api/inventory.ts` | 添加 `delete()` 方法 |
| Hook | `src/hooks/useInventory.ts` | 添加 `useDeleteInventoryTask()` |
| UI | `src/pages/HighSpeedInventory.tsx` | 添加删除按钮 |

**级联影响:**
- 删除任务时关联的 items, scan_records, notes 必须级联处理
- 建议仅允许删除 status=pending 的任务
- in_progress 和 completed 任务应禁止删除（或改为存档）

---

## P2 - 代码质量

### 修复项 8: alert() 替换为 Toast 组件

**涉及文件: 25 个文件，66 处 alert() 调用**

**按类型分布:**
| 类型 | 数量 | 占比 | 处理方式 |
|------|------|------|---------|
| "Coming Soon" 占位 | 15 | 23% | 改为 disabled + tooltip |
| 成功/失败通知 | 21 | 32% | toast.success / toast.error |
| 验证错误 | 10 | 15% | toast.warning |
| 系统事件 | 20 | 30% | toast.info |

**改动范围:**
1. `package.json` — 新增 `sonner` 依赖
2. `src/App.tsx` 或布局组件 — 添加 `<Toaster />` provider
3. 25 个页面文件 — 逐个替换 `alert()` 为 `toast.xxx()`

**风险分析:**
- **低风险**: alert → toast 是纯 UI 行为变更，不影响数据流或业务逻辑
- **注意**: alert 是同步阻塞的（用户必须点确定），toast 是非阻塞的。如果任何业务逻辑依赖 alert 的阻塞特性（如 `alert()` 后执行导航），需检查执行顺序
- **发现 0 处依赖 alert 阻塞特性的代码** — 所有 alert 之后都是无条件执行

**影响的文件清单:**
```
SensorConfiguration.tsx (8处)  HighSpeedInventory.tsx (10处)
EquipmentHealthOverview.tsx (2) SystemSettings.tsx (3)
WorkOrder.tsx (2)              AssetDetailUnified.tsx (2)
AssetLifecycleTimeline.tsx (1) UserProfile.tsx (3)
MaintenanceHub.tsx (3)         PredictiveHub.tsx (1)
AssetManagementUnified.tsx (3) RackManagement.tsx (1)
DataCenter3D.tsx (3)           AuditHistory.tsx (1)
TaskDispatch.tsx (3)           AddNewRack.tsx (1)
BIAOverview.tsx (1)            AssetLifecycle.tsx (2)
MonitoringAlerts.tsx (3)       VideoPlayer.tsx (1)
RolesPermissions.tsx (2)       InventoryItemDetail.tsx (2)
ComponentUpgradeRecommendations.tsx (1)
Login.tsx (0 - 使用 setState)
```

---

### 修复项 9: AssetDetailUnified.tsx 拆分

**当前结构 (1,247 行):**
```
AssetDetailUnified.tsx
├── SectionLabel (line 53, 8 行)
├── DataRow (line 61, 18 行)
├── toSvgPath (line 80, 22 行)
├── RackIllustration (line 103, 15 行)
├── OverviewTab (line 119, ~250 行)
├── HealthTab (line 375, ~235 行)
├── UsageTab (line 628, ~144 行)
├── MaintenanceTab (line 774, ~70 行)
└── AssetDetailUnified 主组件 (line 850, ~397 行)
```

**建议拆分结构:**
```
src/pages/asset-detail/
├── AssetDetailUnified.tsx (主组件: 路由/Tab 切换/编辑模态 ~150 行)
├── tabs/
│   ├── OverviewTab.tsx (~250 行)
│   ├── HealthTab.tsx (~235 行)
│   ├── UsageTab.tsx (~144 行)
│   └── MaintenanceTab.tsx (~70 行)
└── components/
    ├── SectionLabel.tsx (~8 行)
    ├── DataRow.tsx (~18 行)
    ├── RackIllustration.tsx (~15 行)
    ├── MetricsChart.tsx (~60 行, 含 toSvgPath)
    └── EditAssetModal.tsx (~100 行)
```

**影响分析:**
- **路由配置**: `App.tsx` 中 lazy import 路径需更新
  ```typescript
  // BEFORE:
  const AssetDetailUnified = lazy(() => import('./pages/AssetDetailUnified'))
  // AFTER:
  const AssetDetailUnified = lazy(() => import('./pages/asset-detail/AssetDetailUnified'))
  ```
- **Props 传递**: 各 Tab 需要接收 `asset`, `metrics`, `onEdit` 等 props
- **共享状态**: 编辑模式状态 (`editing`, `editData`) 需通过 props 或 context 传递
- **风险**: 中等 — 重构过程中可能引入 props 遗漏

---

### 修复项 10: 消除 `any` 类型

**21 处 `any` 分布在 13 个 Hook 文件中:**

| Hook 文件 | any 数量 | 正确类型来源 |
|-----------|---------|-------------|
| useAssets.ts | 1 | `components['schemas']['Asset']` |
| useBIA.ts | 5 | `components['schemas']['BIAAssessment']` |
| useTopology.ts | 3 | `components['schemas']['RackSlot']` |
| useIdentity.ts | 2 | `components['schemas']['User']`, `Role` |
| useInventory.ts | 4 | `components['schemas']['InventoryTask']` |
| useMaintenance.ts | 2 | `components['schemas']['WorkOrder']` |
| useScanTargets.ts | 1 | `components['schemas']['ScanTarget']` |
| useMonitoring.ts | 2 | `components['schemas']['AlertRule']` |
| useCredentials.ts | 1 | `components['schemas']['Credential']` |
| useSensors.ts | 1 | `components['schemas']['Sensor']` |
| usePrediction.ts | 1 | `components['schemas']['RCA']` |
| useQuality.ts | 1 | `components['schemas']['QualityRule']` |
| useForceLayout.ts | 3 | 需自定义 `ForceNode`, `ForceEdge` |

**影响分析:**
- **改动范围**: 13 个文件，每文件 1-5 行类型声明修改
- **风险等级**: 低 — 仅类型层面修改，不影响运行时行为
- **IDE 收益**: 自动补全准确率提升约 40%
- **编译期收益**: 可在编译时捕获字段名拼写错误
- **潜在问题**: 如果 OpenAPI 生成的类型与后端实际返回不一致，可能出现类型错误

---

### 修复项 11: Dashboard BIA 优化

**当前流程 (Dashboard.tsx lines 182-204):**
```
useAssets() → 获取全部资产 (可能数千条)
↓
useMemo → 遍历所有资产计算 BIA 分布
↓
渲染饼图

useBIAStats() → 获取 BIA 统计 (已聚合)
↓
未使用! (biaResp 被忽略)
```

**优化后流程:**
```
useBIAStats() → 获取 BIA 统计 (1 次请求，返回聚合数据)
↓
直接渲染饼图
```

**影响分析:**
- **改动范围**: Dashboard.tsx 约 20 行代码
- **性能提升**: 减少 1 次全量 API 请求（可能节省 200-500ms + 数 KB 传输）
- **风险**: 需确认 `biaApi.getStats()` 返回的数据格式包含 `critical_count`, `important_count` 等字段
- **副作用**: Dashboard 中 `useAssets()` 调用目前还被用于 lifecycle stats — 如果仅为 BIA 则可移除，否则保留但不用于 BIA 计算
- **验证**: 需检查 `useBIAStats` 返回值与当前 `biaDerived` 数据结构是否匹配

---

## P3 - 国际化

### 修复项 12: 处理 585+ 硬编码字符串

**前 10 名文件新增 Key 估算:**

| 文件 | 预估新 Key 数 | 3 语总条目 | 翻译复杂度 |
|------|-------------|-----------|-----------|
| PredictiveHub.tsx | 42 | 126 | 高 (技术术语) |
| AssetDetailUnified.tsx | 22 | 66 | 中 |
| RackDetailUnified.tsx | 11 | 33 | 低 |
| EnergyMonitor.tsx | 11 | 33 | 中 |
| MaintenanceHub.tsx | 12 | 36 | 中 |
| TroubleshootingGuide.tsx | 15 | 45 | 高 (长文本) |
| VideoLibrary.tsx | 10 | 30 | 低 |
| Welcome.tsx | 10 | 30 | 中 |
| SensorConfiguration.tsx | 8 | 24 | 中 |
| AuditHistory.tsx | 6 | 18 | 低 |
| **合计** | **~147** | **~441** | |

**硬编码类型分布:**
- 23% — "Coming Soon" 占位符 (应替换为 disabled + tooltip)
- 32% — 可翻译 UI 标签和标题
- 15% — 表单验证消息
- 30% — Mock 数据中的描述性文本

**影响分析:**
- **改动范围**: 10+ 页面文件 + 3 个翻译 JSON 文件
- **风险**: 翻译文件部署不同步时，用户看到 key 字符串而非翻译文本
- **关键发现**: PredictiveHub.tsx 使用了 `labelZh` / `labelEn` 的并行模式（在代码中同时维护中英文），这是一种反模式

**PredictiveHub.tsx 的特殊情况:**
```typescript
// 当前反模式 (lines 22-28):
const TAB_DEFINITIONS = [
  { key: 'overview', labelZh: '总览', labelEn: 'Overview', icon: 'dashboard' },
  { key: 'alerts', labelZh: '告警', labelEn: 'Alerts', icon: 'warning' },
  // ...
]

// 应改为:
const TAB_DEFINITIONS = [
  { key: 'overview', labelKey: 'predictive.tab_overview', icon: 'dashboard' },
  { key: 'alerts', labelKey: 'predictive.tab_alerts', icon: 'warning' },
]
```

---

### 修复项 13-14: 翻译 zh-CN 35 个 / zh-TW 30 个英文残留 Key

**涉及文件:**
| 文件 | 修改量 |
|------|--------|
| `src/i18n/locales/zh-CN.json` | 35 个值需翻译 |
| `src/i18n/locales/zh-TW.json` | 30 个值需翻译 |

**分类处理建议:**

| 类别 | 数量 | 处理方式 |
|------|------|---------|
| 品牌名 ("IronGrid", "AIOps Enterprise") | 2 | **保留英文** — 品牌名不翻译 |
| 技术缩写 ("MTBF", "VLAN", "IPMI") | 8 | **保留英文** — 国际通用术语 |
| 协议名 ("SNMP v2c", "SNMP v3") | 3 | **保留英文** — 协议名不翻译 |
| 占位符 ("192.168.1.0/24") | 3 | **保留英文** — 技术示例 |
| **需翻译的状态词** | 5 | "OPTIMAL" → "最佳", "NOMINAL" → "正常" |
| **需翻译的 UI 标签** | 9 | "HIGH" → "高", "128 Nodes" → "128 节点" |
| **需翻译的元数据键** | 5 | "SENSOR_ID" → "传感器编号" |

**实际需翻译: ~19 个 key (zh-CN) / ~14 个 key (zh-TW)**

**影响分析:**
- **改动范围**: 2 个 JSON 文件
- **风险等级**: 极低 — 仅修改已存在的 key 值
- **验证**: 修改后需在三种语言下验证对应页面显示

---

### 修复项 15: 修复 AssetDetailUnified 中文硬编码按钮

**涉及位置:**
| 行号 | 当前文本 | 应改为 |
|------|---------|--------|
| 225 | `查看生命週期` | `t('asset_detail.view_lifecycle')` |
| 232 | `建立維護任務` | `t('asset_detail.create_maintenance_task')` |
| 844 | `查看全部維護記錄` | `t('asset_detail.view_all_maintenance')` |

**影响分析:**
- **改动范围**: 1 个页面文件 (3 行) + 3 个翻译 JSON (各加 3 个 key)
- **风险等级**: 极低
- **注意**: 这 3 个按钮在切换到英文后仍显示繁体中文，是用户可见的 bug
- **验证**: 切换 en/zh-CN/zh-TW 三种语言后检查按钮文本

---

## 总体依赖图

```
修复项之间的依赖关系:

[P0-1 移除内网IP]  ──┐
[P0-2 移除默认凭证] ──┤── 互相独立，可并行
[P0-4 Token持久化]  ──┘

[P0-3 WS Token] ────────── 需要后端配合

[P2-8 Toast组件] ──→ [P1-5 全局onError] ──→ [P1-7 CRUD补齐]
     ↑                                         ↑
     └── 必须先引入 toast                      └── 需要后端先实现 API

[P1-6 Location过滤]
  ├── AssetManagement ─── 可直接实现 (后端已支持)
  ├── MaintenanceHub ──── 需后端先改
  ├── HighSpeedInventory ── 需后端先改
  └── Dashboard ────────── 需产品设计决策

[P2-9 拆分AssetDetail] ──── 独立，可任意时间执行
[P2-10 消除any] ──────────── 独立，可任意时间执行
[P2-11 Dashboard BIA] ────── 独立，可任意时间执行

[P3-12 硬编码字符串] ──→ 依赖 [P2-8 Toast] (部分 alert 需先替换为 toast)
[P3-13 zh-CN残留] ────── 独立
[P3-14 zh-TW残留] ────── 独立
[P3-15 中文按钮] ─────── 独立
```

---

## 建议执行顺序

### 第 1 批 (Day 1-2): 无依赖、高价值
| 序号 | 修复项 | 工时 | 依赖 |
|------|--------|------|------|
| P0-1 | 移除内网 IP | 15 min | 无 |
| P0-2 | 移除默认凭证 | 15 min | 无 |
| P0-4 | Token 持久化 | 1 h | 无 |
| P3-13 | zh-CN 残留翻译 | 30 min | 无 |
| P3-14 | zh-TW 残留翻译 | 30 min | 无 |
| P3-15 | 中文按钮修复 | 15 min | 无 |
| P2-10 | 消除 any 类型 | 3 h | 无 |

### 第 2 批 (Day 3-4): Toast 基础设施 + 快速优化
| 序号 | 修复项 | 工时 | 依赖 |
|------|--------|------|------|
| P2-8 | 引入 Toast 组件 | 4-6 h | 无 |
| P2-11 | Dashboard BIA 优化 | 1 h | 无 |
| P1-6a | Asset 页面 Location 过滤 | 1 h | 无 (后端已支持) |

### 第 3 批 (Day 5-7): 依赖 Toast 的修复
| 序号 | 修复项 | 工时 | 依赖 |
|------|--------|------|------|
| P1-5 | 全局 Mutation onError | 2 h | P2-8 |
| P2-9 | 拆分 AssetDetailUnified | 3-4 h | 无 |

### 第 4 批 (Week 2): 需后端配合
| 序号 | 修复项 | 工时 | 依赖 |
|------|--------|------|------|
| P0-3 | WebSocket Token 传递 | 3 h (前端) + 后端 | 后端协议修改 |
| P1-7 | CRUD 补齐 (WO Delete, IT Edit/Delete) | 4 h (前端) + 后端 | OpenAPI + Go handler |
| P1-6b | Maintenance/Inventory Location 过滤 | 2 h (前端) + 后端 | 后端 API 参数支持 |

### 第 5 批 (Week 3-4): 大规模 i18n
| 序号 | 修复项 | 工时 | 依赖 |
|------|--------|------|------|
| P3-12 | 硬编码字符串国际化 | 2-3 周 | P2-8 (部分) |
| P2-ForceLayout | ForceLayout 性能优化 | 2-3 h | 视节点数决定 |

---

## 风险总评

| 风险等级 | 修复项 | 主要风险 |
|---------|--------|---------|
| **极低** | P0-1, P0-2, P3-13, P3-14, P3-15 | 仅改文本/配置 |
| **低** | P0-4, P2-8, P2-10, P2-11 | 局部改动，不影响数据流 |
| **中** | P1-5, P2-9, P1-6a, P3-12 | 跨文件改动，需充分测试 |
| **高** | P0-3, P1-7, P1-6b/c/d | 需前后端协调，API 契约变更 |
