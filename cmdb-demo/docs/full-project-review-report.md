# CMDB Platform 全面审查报告

> 审查日期: 2026-04-09
> 审查范围: 业务逻辑、代码质量、三语翻译 (en / zh-CN / zh-TW)
> 涵盖: 46 页面 + 13 弹窗组件

## 项目概览

| 项目 | 详情 |
|------|------|
| 前端 | React 19 + TypeScript + Vite + TailwindCSS |
| 后端 | Go (Gin) + PostgreSQL + Redis + NATS |
| 数据采集 | Python (Celery + IPMI/SNMP/SSH) |
| 国际化 | i18next (en / zh-CN / zh-TW) |
| 状态管理 | TanStack React Query + Zustand |
| 总页面数 | 46 页 + 13 个弹窗组件 |

---

## 一、严重安全问题 (RED BLOCKERS)

### 1. 内网 IP 硬编码泄露
- **文件**: `stores/authStore.ts:4`, `hooks/useWebSocket.ts:48`
- **问题**: fallback URL 使用 `http://10.134.143.218:8080/api/v1`，生产构建会将内网拓扑暴露给所有用户
- **修复**: 改为相对路径 `'/api/v1'`（与 `client.ts` 保持一致）

### 2. 登录页面显示默认账密
- **文件**: `pages/Login.tsx:87-91`
- **问题**: UI 直接展示 `admin / admin123` 及 AD 域名 `tw.company.com`
- **修复**: 用环境变量 `VITE_DEV_MODE` 控制，或彻底移除

### 3. Token 通过 URL 查询参数传递
- **文件**: `hooks/useWebSocket.ts:51`
- **问题**: `ws?token=${token}` 导致 JWT 出现在浏览器历史、代理日志中
- **修复**: 改为连接后首条消息发送 token

### 4. Auth 状态不持久化（刷新即登出）
- **文件**: `stores/authStore.ts:18-101`
- **问题**: Zustand store 无 `persist` 中间件，页面刷新即丢失 token
- **修复**: 添加 `persist` 中间件或改用 httpOnly cookie

### 5. 所有 Mutation 无错误处理
- **范围**: `hooks/` 目录全部 25 个文件
- **问题**: 无任何 `onError` 回调，API 失败时用户无反馈
- **修复**: 在 QueryClient 的 `MutationCache` 添加全局 `onError`

---

## 二、业务逻辑问题

### 1. Location 作用域未贯穿
| 页面 | 是否按 Location 过滤 | 状态 |
|------|----------------------|------|
| Dashboard | 否，查询全局数据 | :x: |
| AssetManagementUnified | 否，`useAssets()` 无 location_id | :x: |
| MaintenanceHub | 否，`useWorkOrders()` 无过滤 | :x: |
| HighSpeedInventory | 否，`useInventoryTasks()` 无过滤 | :x: |
| RackManagement | 是，正确使用 `path.idc?.id` | :white_check_mark: |
| Location 系列页面 | 是 | :white_check_mark: |

### 2. CRUD 完整性缺失
| 实体 | Create | Read | Update | Delete |
|------|--------|------|--------|--------|
| Assets | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| Racks | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| Locations | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| Work Orders | :white_check_mark: | :white_check_mark: | :white_check_mark: | :x: 缺失 |
| Inventory Tasks | :white_check_mark: | :white_check_mark: | :x: 缺失 | :x: 缺失 |
| BIA Assessments | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

### 3. Auth 流程缺陷
- `expires_in` 返回但未使用，无主动刷新 token 机制
- Logout 仅清除客户端状态，未调用服务端注销接口
- 无 CSRF 保护

---

## 三、代码质量问题

### 1. 大量使用 `any` 类型（30+ 处）
- `hooks/useAssets.ts:30` -> `Partial<any>`（等同于 `any`）
- `hooks/useBIA.ts:4` -> `params?: any`
- `hooks/useForceLayout.ts:3` -> `nodes: any[], edges: any[]`
- **建议**: 从 `generated/api-types.ts` 引入正确类型

### 2. `alert()` 作为主要用户反馈（60+ 处）
- 分布在 30+ 页面中
- 阻塞主线程、无法定制样式
- **建议**: 引入 toast 组件（如 sonner / react-hot-toast）

### 3. AssetDetailUnified.tsx 单文件过大（1,246 行）
- 包含 4 个 Tab 子组件 + 多个 helper + mock 数据 + 编辑逻辑
- **建议**: 拆分为 `src/pages/asset-detail/` 目录

### 4. Force Layout 同步计算性能问题
- `hooks/useForceLayout.ts:11-41` 在 `useMemo` 中跑 100 次 O(n^2) 迭代
- **建议**: 移至 Web Worker 或使用 d3-force

### 5. Dashboard 获取全部 Assets 仅为计算 BIA 分布
- `Dashboard.tsx:182-204` 无分页获取所有资产
- 后端已有 `/bia/stats` 端点可直接使用

---

## 四、国际化 (i18n) 分析

### 总体状态
- 三语翻译 key 数量一致：**1,950 keys** :white_check_mark:
- 无缺失 key :white_check_mark:

### 未翻译内容（英文原文直接出现在中文文件中）
| 类别 | zh-CN | zh-TW | 示例 |
|------|-------|-------|------|
| 品牌名称 | 2 | 2 | "IronGrid", "AIOps Enterprise" |
| 技术术语 | 12 | 12 | "MTBF", "VLAN", "IPMI", "SNMP v2c" |
| 状态词 | 5 | 3 | "OPTIMAL", "NOMINAL", "HIGH" |
| 占位符 | 4 | 4 | "192.168.1.0/24", "https://example.com/webhook" |
| 其他 | 12 | 9 | "SENSOR_ID", "CPU (%)", "128 Nodes" |
| **合计** | **35** | **30** | |

### 严重问题：硬编码字符串（585+ 处，55 个文件）

**前 10 名违规文件**:

| 文件 | 硬编码数 | 典型内容 |
|------|---------|---------|
| PredictiveHub.tsx | 73 | Tab 标签、资产名、故障描述 |
| EnergyMonitor.tsx | 59 | 设备类别、机架名称 |
| MaintenanceHub.tsx | 46 | 日历标签、优先级 |
| TroubleshootingGuide.tsx | 46 | FAQ 内容、步骤说明 |
| AssetDetailUnified.tsx | 39 | 硬件规格、中文按钮文字 |
| SensorConfiguration.tsx | 34 | 传感器配置项 |
| VideoLibrary.tsx | 30 | 视频标题、描述 |
| Welcome.tsx | 30 | 功能介绍文案 |
| VideoPlayer.tsx | 24 | 播放器标签 |
| AuditHistory.tsx | 21 | 审计类型标签 |

### AssetDetailUnified.tsx 中的中文硬编码
```
查看生命週期        (line 226)
建立維護任務        (line 232)
查看全部維護記錄     (line 844)
```
这些按钮在切换英文后仍显示中文。

---

## 五、各页面逐一分析

### A. 认证与导航

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **Login** | :white_check_mark: 基本完整 | :warning: 泄露凭证 | :white_check_mark: | 硬编码默认账密 |
| **Welcome** | :white_check_mark: 功能展示 | :warning: 30 处硬编码 | :x: 大量未翻译 | 功能介绍文案未国际化 |

### B. Location 层级（4 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **GlobalOverview** | :white_check_mark: API + 地图 | :white_check_mark: 良好 | :white_check_mark: | 依赖 mock fallback |
| **RegionOverview** | :white_check_mark: 多级钻取 | :white_check_mark: 良好 | :white_check_mark: | -- |
| **CityOverview** | :white_check_mark: 排序/视图切换 | :white_check_mark: 良好 | :white_check_mark: | -- |
| **CampusOverview** | :white_check_mark: 4 级链路 | :white_check_mark: 良好 | :white_check_mark: | -- |

### C. 资产管理（5 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **AssetManagementUnified** | :warning: 无 location 过滤 | :white_check_mark: | :white_check_mark: | 导入/导出完整，但显示全局数据 |
| **AssetDetailUnified** | :warning: Usage/Maint Tab 全 mock | :warning: 1246 行单文件 | :x: 中文硬编码 | 需拆分、替换 mock 数据 |
| **AssetLifecycle** | :warning: 部分 mock | :white_check_mark: | :warning: 硬编码 | 生命周期状态图 |
| **AssetLifecycleTimeline** | :warning: mock 事件 | :white_check_mark: | :warning: 硬编码 | 时间线数据非真实 |
| **AutoDiscovery** | :white_check_mark: 完整扫描流程 | :white_check_mark: | :white_check_mark: | -- |

### D. 机架管理（5 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **RackManagement** | :white_check_mark: 正确使用 location | :white_check_mark: | :white_check_mark: | -- |
| **RackDetailUnified** | :white_check_mark: U 位布局 | :white_check_mark: | :white_check_mark: | -- |
| **DataCenter3D** | :warning: 可能 mock 渲染 | :white_check_mark: | :warning: | 3D 数据来源不明 |
| **FacilityMap** | :white_check_mark: SVG 布局 | :white_check_mark: | :white_check_mark: | -- |
| **AddNewRack** | :white_check_mark: 表单验证 | :white_check_mark: | :white_check_mark: | -- |

### E. 库存管理（2 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **HighSpeedInventory** | :warning: 任务不可编辑/删除 | :warning: 多处 alert() | :white_check_mark: | CRUD 不完整 |
| **InventoryItemDetail** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |

### F. 监控告警（5 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **MonitoringAlerts** | :white_check_mark: 确认/解决流程 | :white_check_mark: | :white_check_mark: | AIOps 分析部分可能 mock |
| **SystemHealth** | :white_check_mark: 健康度计算 | :white_check_mark: | :white_check_mark: | -- |
| **SensorConfiguration** | :white_check_mark: CRUD 完整 | :warning: alert() 反馈 | :x: 34 处硬编码 | -- |
| **EnergyMonitor** | :warning: 大量 mock 数据 | :warning: 59 处硬编码 | :x: | 设备类别、机架名全硬编码 |
| **AlertTopologyAnalysis** | :white_check_mark: | :warning: O(n^2) 布局 | :white_check_mark: | 性能问题 |

### G. 维护工单（4 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **MaintenanceHub** | :warning: 无 location 过滤、无删除 | :warning: 46 处硬编码 | :x: | 日历标签未翻译 |
| **MaintenanceTaskView** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |
| **WorkOrder** | :warning: 无删除操作 | :white_check_mark: | :white_check_mark: | -- |
| **AddMaintenanceTask** | :white_check_mark: 表单验证 | :white_check_mark: | :white_check_mark: | -- |

### H. 预测性 AI（4 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **PredictiveHub** | :warning: 大量 mock 预测数据 | :warning: 73 处硬编码 | :x: | 最严重的硬编码文件 |
| **QualityDashboard** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |
| **ComponentUpgrades** | :warning: mock | :white_check_mark: | :warning: | -- |
| **EquipmentHealth** | :warning: mock | :white_check_mark: | :warning: | -- |

### I. BIA 业务影响分析（5 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **BIAOverview** | :white_check_mark: | :warning: 20 处硬编码 | :warning: | -- |
| **SystemGrading** | :white_check_mark: 分级完整 | :white_check_mark: | :white_check_mark: | -- |
| **RtoRpoMatrices** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |
| **ScoringRules** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |
| **DependencyMap** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |

### J. 系统管理（4 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **RolesPermissions** | :white_check_mark: RBAC 完整 | :warning: 18 处硬编码 | :x: | 权限名未翻译 |
| **SystemSettings** | :white_check_mark: 多 Tab 管理 | :warning: 18 处硬编码 | :x: | -- |
| **AuditHistory** | :white_check_mark: 审计日志 | :warning: 21 处硬编码 | :x: | 事件类型标签 |
| **UserProfile** | :white_check_mark: | :white_check_mark: | :white_check_mark: | -- |

### K. 帮助中心（3 页）

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **TroubleshootingGuide** | :white_check_mark: | :warning: 46 处硬编码 | :x: | FAQ 内容全硬编码 |
| **VideoLibrary** | :white_check_mark: | :warning: 30 处硬编码 | :x: | 视频标题/描述 |
| **VideoPlayer** | :white_check_mark: | :warning: 24 处硬编码 | :x: | -- |

### L. Dashboard

| 页面 | 业务逻辑 | 代码质量 | i18n | 主要问题 |
|------|---------|---------|------|---------|
| **Dashboard** | :warning: 无 location 过滤；获取全部 assets 仅为 BIA 统计 | :warning: heatmap mock、硬编码百分比 | :warning: | "12% vs last month" 硬编码 |

---

## 六、优先修复建议

### P0 - 立即修复（安全）
1. 移除 `authStore.ts` 和 `useWebSocket.ts` 中的内网 IP
2. 移除 Login 页面的默认凭证显示
3. WebSocket token 改为消息内传递
4. 添加 Auth token 持久化

### P1 - 一周内修复（功能完整性）
5. 所有 mutation 添加全局 `onError` 处理
6. 资产/维护/库存页面添加 location_id 过滤
7. 补齐 Work Order 删除、Inventory Task 编辑/删除

### P2 - 两周内修复（代码质量）
8. `alert()` 替换为 toast 组件
9. AssetDetailUnified.tsx 拆分
10. 消除 `any` 类型（30+ 处）
11. Dashboard 改用 `/bia/stats` 端点

### P3 - 持续改进（i18n）
12. 处理 585+ 硬编码字符串（优先: PredictiveHub、EnergyMonitor、MaintenanceHub）
13. 翻译 zh-CN 的 35 个英文残留 key
14. 翻译 zh-TW 的 30 个英文残留 key
15. 修复 AssetDetailUnified 中的中文硬编码按钮
