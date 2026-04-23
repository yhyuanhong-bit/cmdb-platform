# CMDB Platform 业务适配性评审 — 2026-04-22

> 评审范围：cmdb-core (Go backend), cmdb-demo (React frontend), ingestion-engine (Python), 数据模型，集成与工作流
> 评审目的：基于 CMDB 的本质业务目的，判断该平台对真实数据中心运维团队的业务合理性、业务可用性、业务连贯性
> 评审方法：4 个并行 agent + 主线交叉校验，所有结论均带证据引用

---

## 一句话结论

**它是一个能跑的资产 + 治理引擎，不是一个能用的 CMDB。**

技术上完成度很高（175 个 API 端点，48 个前端页面，10 个自动工单触发器，60+ migrations）；业务上**缺三块决定 CMDB 之所以是 CMDB 的核心能力**：service-centric 视角、外部 ITSM 集成、discovery 审核闸门。当前形态适合内部资产盘点 + 自动化治理，不适合作为数据中心运维团队的核心 system of record。

**业务覆盖度估算：65%**

---

## 评分卡（按 CMDB 业务支柱）

| 业务支柱 | 数据层 | 后端 API | 前端 UI | 集成 | 综合 |
|---|---|---|---|---|---|
| 资产清单 (CI Inventory) | 完整 | 完整 | 完整 | — | **A** |
| 物理拓扑 (Locations / Racks) | 完整（LTREE 层级 + U-slot） | 完整 | 完整（含 3D） | — | **A** |
| 关系/依赖 (Relationships) | 部分（通用 N:M 表，4 类） | 完整（含时序遍历） | 部分（仅 asset↔asset） | — | **B-** |
| 历史 / 审计 (History) | 完整（月分区 snapshots + 审计） | 完整（state-at, lifecycle, diff） | 部分（per-asset 有，timeline 部分占位） | — | **B+** |
| 自动发现 (Discovery) | 部分（staging 表存在） | 部分（无审核闸） | 部分（手工 reconcile） | 部分（pull-only） | **C+** |
| 工单 / 工作流 | 完整（双维度状态机） | 完整 | 完整 | 缺 ITSM 出口 | **B** |
| 监控集成 | — | 完整（拉模型） | 完整 | **缺推模型** — 新资产不会自动注册 Datadog | **C+** |
| 多租户隔离 | 完整 | 完整 | — | — | **A**（wave 1 修复后） |
| 业务服务 (Services) | **缺** — bia_assessments 用字符串伪装 | **缺** — 无 services 端点 | **缺** — 无 service-centric 视图 | — | **F** |
| Edge / 联邦 | 部分（sync_state, conflicts） | 部分（sync API） | 部分（冲突解决 UI） | **缺离线写队列** | **C** |
| 合规 / 政策 | **缺**（quality_rules 但无 policy 表） | 部分 | 部分（compliance scan 占位） | — | **D** |
| 报表 / 分析 | 完整（TimescaleDB cont-agg） | 完整 | 完整 | — | **A-** |

---

## Top 10 业务缺陷（按业务影响排序）

### 🔴 P0 — 直接削弱「CMDB 之所以是 CMDB」的能力

#### 1. 没有真正的 Business Service 实体
**证据**：
- `db/migrations/000013_bia_tables.up.sql` — `bia_assessments` 用 `system_name VARCHAR(255)` 当 service 名，没有 services 表，没有 service↔assets 的 FK
- 前端 `DependencyMap` 是 asset↔asset，没有 "this service depends on these 6 assets" 视图
- 后端 OpenAPI 没有 `/services` endpoint

**业务影响**：
- 无法回答 BCM 必问题：「订单系统挂了，影响哪些客户？」「DR 切换需要几台机？」
- BIA 评估孤立 — RTO/RPO 数值存了但没办法验证「这个 service 的 7 台服务器是不是都能在 RTO 内恢复」
- 监控告警只能落到 asset，没法升级到 service-level incident

**修复路径**：新增 `services`, `service_assets`, `service_owners` 三张表 + service-centric 拓扑视图。**预估 3 周。**

#### 2. 不能跟外部 ITSM 系统对接
**证据**：
- 集成 audit：「Work orders are CMDB-internal only. No outbound adapter to ServiceNow, Jira, or similar」
- `internal/domain/workflows/adapter_*.go` 只有入站（Prometheus/Zabbix/Datadog **拉**指标），没有出站到 ITSM
- `integration_adapters` 表 `direction` 字段有 inbound/outbound/bidirectional 三态，但 outbound 没实现

**业务影响**：
- 运维真实工作流是「告警 → ServiceNow 工单 → 工程师执行」，CMDB 工单只是平台内部状态，工程师还是要去 ServiceNow 重复填一遍
- 自动工单的 10 个治理触发器（warranty 到期、shadow IT、duplicate serial）发到 CMDB 内部，**实际响应人不在 CMDB 里**
- 失去"权威 system of record"地位 — 工单真相在 ServiceNow，CMDB 只能跟着抄

**修复路径**：实现 ServiceNow + Jira outbound adapter（webhook 已存在但缺 ITSM 协议适配层）。**预估 2 周/适配器。**

#### 3. Discovery 没有审核闸门
**证据**：
- `ingestion-engine/app/pipeline/processor.py` 三种路由模式：auto / review / smart，但 auto 直接合并、smart 也"existing→pipeline"自动合
- 集成 audit：「Reconciliation silently dedup-matches on serial_number OR asset_tag; no manual review gate for conflicts before merge」
- 冲突进 staging 表但没有自动升级到工单或通知

**业务影响**：
- 一台扫描器配置错了（比如 community string 漂移）→ CMDB 数据被静默污染
- shadow IT（未登记的设备）发现后等人去看；运维团队习惯不去 review，形成长期漂移
- 合规审计时无法证明「每条 CI 创建都有人审核过」

**修复路径**：默认改 review 模式 + 冲突自动开 governance 工单 + UI 加一个「待审核 discovery」队列。**预估 1.5 周。**

### 🟡 P1 — 影响日常使用流畅度

#### 4. Edge 节点不能离线写
**证据**：
- `internal/middleware/sync_gate.go:11-25` — edge 模式且初始未同步时，所有 `/api/v1/*` 写返回 503 + `Retry-After: 30`
- 没有「写入 → 本地缓存 → 重连后回放」的队列实现
- README 声称 "Offline-capable Edge nodes" — 实际只是「网络断了暂时不写，恢复后再写」，**不是真正的离线优先**

**业务影响**：
- 边缘机房网络抖动 = 操作员看到一片 503
- 跟核心 hub 网络分区时，本地工程师无法在 CMDB 里建工单或更新资产
- README 的卖点站不住

**修复路径**：edge 端加本地 SQLite 写缓冲 + 重连后通过 sync envelope 回放。**预估 4 周。**

#### 5. 没有 service-centric 告警 → 工单
**证据**：
- `notifications.go onAlertFired` 只为 `severity=critical` 自动开工单，且工单只关联 asset
- 没有逻辑：「这台 asset 属于 service X → 把 service X 的所有 active alert 聚合升级 incident」
- 监控告警和 BIA 评估之间没有自动桥梁

**业务影响**：
- 一个 service 三台机同时告警 → 创建 3 个独立 emergency 工单，运维不知道这是同一件事
- 工单堆积，淹没真正的 incident
- BIA tier 信息没用上，所有告警同等优先级处理

**修复路径**：依赖 #1（service 实体）。先实现 service 后做 incident aggregation。**预估 1.5 周（依赖 #1）。**

#### 6. 11 个前端页面带 phase-3.10 占位符
**证据**（前端 audit 完整列表）：
- `Dashboard.tsx:244` 资产趋势图占位
- `Dashboard.tsx:426` 机柜热力图占位
- `EnergyMonitor.tsx:351,503,608,622` peak 日期、UPS autonomy、机柜功率热图、power events 流（4 处）
- `InventoryItemDetail.tsx:82,216` asset 链接、不一致详情（2 处）
- `AssetLifecycleTimeline.tsx:238` compliance scan 端点（1 处）
- `predictive/TimelineTab.tsx:140,161` 机柜占用、环境指标（2 处）

**业务影响**：
- 用户看到漂亮图表但里面是静态/虚假数据 → 信任度下降
- Energy 模块基本不能用（4 个核心面板都是占位）
- 前端 UI 设计走在后端 API 前面，造成 phantom features

**修复路径**：补这 11 个对应的后端端点，或在前端明确标记「Coming soon」隐藏这些组件。**预估 2 周。**

#### 7. 跨页导航断链
**证据**（前端 audit）：
- Alert 详情 → 没有跳转到关联 asset 的链接
- Work Order → asset 关系靠解析 `description.split` 字符串，**脆弱**
- Inventory Task → asset 需要用户手工搜索
- AlertTopologyAnalysis 与 MonitoringAlerts 是两个独立页面，没有 drill-down

**业务影响**：
- 排障流程被打断：「这条告警是哪台机器」需要复制粘贴搜索
- 运维效率随规模线性下降
- 数据其实在 backend 都有（FK 都对的），纯粹前端没接

**修复路径**：每个跨实体引用加 `<Link to=...>`，全前端梳理一遍。**预估 1 周。**

#### 8. 没有变更审批闸门 (CAB)
**证据**：
- 工单状态机有 `governance_status` (审批) 和 `execution_status` (执行) 两轴，但 `auto_workorders.go` 自动开的工单**自动 approve + 自动 in_progress**（`onAlertFired` 用「single SQL UPDATE」atomic flip）
- 后端 `RequiresApproval(status)` 函数存在但只覆盖手工创建的工单
- 没有「decommission asset 必须经过 CAB」的强制流程

**业务影响**：
- 高风险变更（比如 retire production server）可以跳过审批
- ISO 27001 / SOC2 合规要求「change control」，当前实现只能算 "audit log"，不算 "control"
- 自动治理工单虽然有用，但**也跳过了人工 review**

**修复路径**：定义 change-risk classification，高风险类别强制 CAB approval。**预估 2 周。**

### 🟢 P2 — 锦上添花

#### 9. owner_team 只是字符串
**证据**：
- `db/migrations/000060_asset_owner_team.up.sql:22` — `owner_team VARCHAR(100) NULL`
- 没有 teams / org_units 表
- 前端 RolesPermissions 只管 user-role，没有 user-team 概念

**业务影响**：
- 无法做团队级别的权限（"team A 能改 team A 的 asset"）
- 无法做团队级别的 dashboard / 通知
- 字符串 typo 就分裂成两个团队，无 referential integrity

**修复路径**：加 teams 表 + asset.owner_team 改 FK + 迁移脚本归一化现有字符串。**预估 1 周。**

#### 10. 状态字段缺 CHECK 约束
**证据**（datamodel audit）：
- `assets.status`, `work_orders.status`, `discovered_assets.status` 都是 VARCHAR，没有 enum 或 CHECK constraint
- 应用层 validate，但绕过 API 直接写 DB（migration / 脚本）能写脏数据
- 已经发生：alert evaluator startup 日志里有「skipping rule with malformed condition: invalid operator ""」 — 数据已经污染

**业务影响**：
- 数据漂移随时间累积
- 报表 group by status 会出现意料外的 bucket
- migration 时容易遗漏新增的合法值

**修复路径**：每个 status 字段加 CHECK constraint，配套 migration 清理现有脏数据。**预估 0.5 周。**

---

## 业务连贯性问题（横跨多层）

### A. "看得到，做不了"
backend 有 API，frontend 没接：
- `/assets/{id}/state-at` — 后端能查任意时刻的 asset 快照，前端没暴露这个能力（AssetLifecycleTimeline 只显示 timeline，不能"回到那一天"）
- `/topology/dependencies?at=T` — 同上，时序拓扑只在 API 里
- 历史 BIA 评分趋势 — DB 里有，UI 没图

### B. "做得到，看不到"
frontend 能触发，结果在 UI 看不到：
- 自动工单（10 个治理规则）触发后，**没有专门页面展示「最近自动开了哪些工单 / 触发原因 / 哪些是被去重掉的」** — 运维不知道系统帮自己做了什么
- Excel import 完成后只在 inventory_tasks 看到结果，没有 import history 页面
- Webhook delivery 失败 → 后端 circuit-break，前端没有 indicator

### C. "存在，但孤岛"
模块间数据没有打通：
- BIA tier 是 `bia_assessments.tier`，但 alert 评估、工单优先级、监控告警**都没引用 BIA tier**
- `asset.owner_team` 字段加了，但通知调度仍然是 ops-admin 全员，没有「只通知 owning team」的逻辑
- `quality_rules` 评分了 asset，但**评分结果没影响其他模块的展示**（除了 quality dashboard 自己）

---

## 做得好的地方（务必保留）

避免只看缺陷误以为整个项目不好：

1. **多租户隔离架构性正确** — TenantScoped + tenantlint 静态检查，wave 1 修复后 0 直接 pool 调用
2. **审计模型完整** — append-only + 月分区 + immutable + state-at 查询，技术上是 enterprise-grade
3. **数据模型扩展性强** — JSONB attributes 让新 CI 类型不需要 migration
4. **物理拓扑模型扎实** — LTREE locations + 严格的 U-slot constraint，比大部分商业 CMDB 都细
5. **可观测性设施齐全** — Prometheus + Grafana + OpenTelemetry + scrubber 防泄密
6. **测试覆盖在快速提升** — wave 3 测试 + integration test CI 让 identity 75%, workflows 49%
7. **Edge 同步框架设计严谨** — checksum + HMAC + LWW 策略 + sync_conflicts 表 + grace window 机制，**虽然功能不全但基础对了**
8. **govulncheck + npm audit 进 CI** — 依赖安全持续监控

---

## 推荐路线图

按业务影响 / 技术依赖排序，假设 3 人后端 + 1 人前端 + 1 人 product：

### Q1 (8 周) — 让它真的是 CMDB
| 周 | 项目 | 解锁能力 |
|---|---|---|
| 1-3 | Service 实体 + service-centric 视图 (#1) | BIA 真正可用，service map |
| 4-5 | Discovery 审核闸门 (#3) + ServiceNow adapter PoC (#2) | 数据可信 + ITSM 桥接 |
| 6 | 跨页导航断链修复 (#7) + status CHECK 约束 (#10) | 日常体验提升 |
| 7-8 | 11 个 phase-3.10 占位符补齐或下线 (#6) | 移除 phantom features |

### Q2 (8 周) — 让它真的可信任
| 周 | 项目 | 解锁能力 |
|---|---|---|
| 1-2 | Service-centric 告警聚合 (#5) | 减少告警噪音 |
| 3-4 | 变更审批闸门 (#8) | 合规通过率 |
| 5-6 | Teams 实体 (#9) + owner-team 通知路由 | 多团队场景可用 |
| 7-8 | Webhook delivery 审计 + DLQ | 集成可靠性 |

### Q3 (8 周) — 让它真的能 Edge
| 周 | 项目 | 解锁能力 |
|---|---|---|
| 1-4 | Edge 离线写队列 (#4) | README 卖点兑现 |
| 5-6 | Monitoring push（自动注册新资产到 Datadog） | discover-to-monitor 自动化 |
| 7-8 | Compliance / Policy 模块（policy 表 + 例外审批） | 合规真闭环 |

---

## 评审者盲区

我没充分验证的：

1. **真实生产负载下的性能** — 没跑过 100K+ assets 的查询性能、tenantlint 改造后的分页查询是否退化
2. **NATS JetStream 的可用性配置** — 没看 stream 复制、retention、ack 配置是否正确
3. **预测 AI 模块** — `prediction_models` 是 global 表（没 tenant_id）— datamodel agent flagged 但我没深入这个模块的合理性
4. **Energy 模块的真实性** — 一堆 phase-3.10 占位 + 没看到对应硬件协议的 collector，怀疑这模块整体是 mock
5. **3D 数据中心可视化** — `DataCenter3D.tsx` (1.4MB elk.js bundle) 真的有人用吗？还是 demo-only？
6. **AD/LDAP 集成** — README 提到 "AD: username@domain"，但没找到 LDAP authenticator 实现 — 可能未实现
7. **Backup / DR** — 看了 docs/backup-recovery.md 但没验证 restore 流程

---

## 给决策者的一句话

**这个 CMDB 已经投入了大量工程努力做对的事（多租户、审计、Edge sync 框架、测试覆盖），但它还差最后一公里——把 IT 资产管理升级成业务服务管理。**

如果你的目标是：
- ✅ **取代 Excel 管资产** — **现在就能用**
- ✅ **统一数据中心物理资产视图** — **现在就能用**
- ⚠️ **作为 ITSM/监控的 source of truth** — **缺 ITSM adapter + 监控 push，3-6 个月可达**
- ❌ **业务服务连续性管理 (BCM)** — **缺 service 实体，6+ 个月**
- ❌ **多站点 Edge 离线优先运维** — **缺写队列，README 名不副实**
