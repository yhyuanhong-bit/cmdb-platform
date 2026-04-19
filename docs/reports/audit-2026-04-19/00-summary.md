# 00 — 全平台审计总结报告

> 审计日期:2026-04-18 → 2026-04-19
> 范围:cmdb-core (Go)、cmdb-demo (React 19 / TS)、ingestion-engine (Python 3.12)
> 方法:6 个并行 audit agent 分域剖析,本文负责横向汇总 + 优先级排序

## 子报告索引

| # | 报告 | 覆盖域 | 行数 |
|---|------|--------|------|
| [01](01-entities.md) | **Core Entities** | Location / Asset / Rack / Discovery 领域服务 | 372 |
| [02](02-workflows.md) | **Workflows / Eventing** | NATS subscriber、cron/ticker、SLA、cleanup、dual-write divergence、metrics puller、adapter 实现 | 326 |
| [03](03-integration.md) | **Integration / 加密层** | inbound adapter、webhook、AES-256-GCM KeyRing、dual-write rollout、CLI 工具 | 329 |
| [04](04-ops-intel.md) | **Ops Intelligence** | Maintenance / Monitoring / Prediction / Quality / BIA | 362 |
| [05](05-infra.md) | **Infra** | Identity/RBAC、Audit、Sync protocol、Dashboard、API 层、Middleware 链、Platform | 390 |
| [06](06-frontend-data.md) | **Frontend / Ingestion / Data Contract** | cmdb-demo、ingestion-engine、OpenAPI、sqlc、DB migrations | 359 |

---

## 1. 模块成熟度排序(高 → 低)

| 名次 | 模块 | 成熟度判据 |
|------|------|-----------|
| 1 | **Maintenance(工单)** | 双维度状态机 + SLA + 乐观锁 + 11 条自动工单规则 + 完整审批逻辑。写得最扎实 |
| 2 | **Integration 加密 rollout** | Dual-write → backfill → divergence monitor → cleanup → 密钥轮换,五阶段闭环、每阶段幂等 + audit + dry-run |
| 3 | **BIA** | 数据模型 + propagation SQL + 多消费方(prediction/topology/rack/webhook)。唯一短板:新增 dependency 不触发 propagate |
| 4 | **Prediction Upgrade Recommendations** | 275 行业务密集代码 + BIA boost + peer comparison + cost。单模块内质量最高的一段 |
| 5 | **Sync Protocol** | Envelope + layers + last-write-wins + work_orders 双维 status 融合。但 sync_conflicts 插入路径未启用 |
| 6 | **Quality Engine** | 四维度权重 + evaluateAsset 写得不错 + location_detect 交叉检查 |
| 7 | **前端架构** | React 19 + Tailwind v4 + TanStack Query + WebSocket 驱动 invalidation 成熟 |
| 8 | **Ingestion Pipeline** | normalize → dedup → validate → authority 清晰 + source authority 字段级权威 |
| 9 | **Prediction RCA** | 接口齐但上下文不塞(RelatedAlerts / AffectedAssets 从未填充),AI 调用接近空手 |
| 10 | **Monitoring**(**最薄**) | `alert_rules` 表**从未被评估**,前端"New Rule"按钮点完对 runtime 无效。alert_events 的生产来源只有 location_detect + sync + seed |

---

## 2. 跨模块共性风险(按出现次数降序)

### 2.1 静默吞错(出现在 01 / 02 / 03 / 05 / 06)

系统性问题。最严重的几处:

- `workflows/cleanup.go:30,34,38,102,107,122` — 6 处 `Exec` 用 `_`,清理任务失败无任何信号
- `workflows/notifications.go:47-57, 166-168` — `createNotification` 完全不看 err;alert 自动解决也不看 err
- `workflows/sla.go:33-35, 60-62` — SLA checker 完全吞掉 query/update error
- `ingestion-engine/pipeline/processor.py:145-146` — 批量 import 错误计数 `stats["errors"] += 1` 但丢失行号 + 原因
- `workflows/auto_workorders.go:70, 164, 315` 等 — `rows.Scan(...) != nil { continue }` 多处静默丢行
- `workflows/adapter_*.go` — 五处 `json.Unmarshal` 带 `//nolint:errcheck`,配置解析失败拿 zero value 继续跑

### 2.2 Tenant 隔离泄漏(出现在 01 / 02 / 05)

**这是安全基线最大的债务。** middleware 只把 tenant_id 写入 context,强制 DB 过滤完全靠每条 SQL 手写 `WHERE tenant_id = $N`。实际已发现:

| 严重度 | 位置 | 问题 |
|--------|------|------|
| **CRITICAL** | `sync_endpoints.go:188,197,217` | `SyncResolveConflict` 无 tenant 过滤(IDOR)**+ 动态 SQL 注入**(diffMap key 拼列名,来自攻击者可控的 remote_diff JSON) |
| HIGH | `session_endpoints.go:45` | `ListUserSessions` 任意用户 session 可读 |
| HIGH | `prediction_endpoints.go:51,687,700` | `GetAssetRUL` / `UpdateUpgradeRule` / `DeleteUpgradeRule` 跨租户 |
| HIGH | `topology_endpoints.go:115-132` | `DeleteAssetDependency` 跨租户删拓扑边 |
| HIGH | `000002_tenants_and_identity.up.sql:49-53` | `user_roles` 表**无 tenant_id**,`AssignRole` 可跨租户赋权 |
| HIGH | `000002_tenants_and_identity.up.sql:26` | `users.username` 全局 UNIQUE(应 `(tenant_id, username)`);多租户场景下租户 A 的 `admin` 阻止租户 B 创建 `admin` |
| HIGH | `workflows/auto_workorders.go:405-444` | `checkShadowIT` 跨租户扫 `discovered_assets` 写工单到 `firstTenantID` |
| MEDIUM | `impl_inventory.go:199,311`, `inventory_resolve_endpoint.go:74` | Inventory task 状态 flip 无 tenant 过滤 |

### 2.3 Context 生命周期断裂(出现在 02 / 03)

- `cmd/server/main.go:67` 用 `context.Background()`,**无 cancel 函数**。所有 `StartXxx` 的 `<-ctx.Done()` 是死代码
- `webhook_dispatcher.go:94` 用 `context.Background()` 投递 webhook,服务 shutdown 时在途 webhook 跑完全部 3 次重试(最长 ~36s)
- 结果:进程依赖强制终止回收 goroutine;滚动发布时 Postgres 端看到被截断的连接

### 2.4 SSRF 未设防(出现在 02 / 03)

- `workflows/metrics.go` 的 puller 直接拿 `a.Endpoint.String` 给 HTTP client。租户 admin 可建 `endpoint=http://169.254.169.254/latest/meta-data/` 的 Prometheus adapter,云元数据 + 内网服务都能被拉回
- `adapter_custom_rest.go:33-36` 允许 config 里覆写 URL,进一步绕过 endpoint 字段校验
- **整个 integration 层没有 URL 白名单 / 黑名单机制**

### 2.5 Soft/Hard delete 语义不一致(出现在 01 / 05)

- `assets` 表有 `deleted_at` 列,但 `DeleteAsset` 是**硬 DELETE**(`01-entities.md` 明确列出)
- `racks` 走 soft delete(`Dashboard.TotalRacks` 过滤 `deleted_at IS NULL`,`service.go:78`)
- `asset_dependencies` 硬 DELETE,无 tenant 过滤
- 无统一约定,每个域各写各的

### 2.6 跨域隐式依赖 / 半成品粘合(出现在 01 / 02 / 04)

- Discovery `Approve` **不创建 Asset**(`01-entities.md`)— 审批"通过"只改 discovered_assets 状态,Asset 表没有新增行,是一个沉默的断链
- Monitoring `alert_rules` 表**配置写入后无 evaluator** — UI 上配了规则,runtime 完全忽略
- `SubjectInventoryTaskCompleted` 有 subscriber 但**从未 publish**
- `workflows.StartPeriodicDetection` 是 dead code
- `prediction_results` 表有 Create query 但**生产代码零调用** — `GET /prediction/results/ci/{id}` 返回的是 seed
- `workflows/adapter_placeholder.go` — SNMP / Datadog / Nagios 在 `adapterRegistry` 里注册但 Fetch 返回 "not yet implemented" — 用户用失败去发现未实现
- `ai.AIProvider.PredictFailure` 三个 provider 都实现了,**零调用方**
- ServiceNow / bidirectional adapter 被 `ListDuePullAdapters` 的 `direction='inbound'` 硬编码筛掉 — 出站 adapter 是**占位**
- `work_orders.prediction_id` 字段保留但无人写入

### 2.7 幂等性 / 并发正确性(出现在 02 / 04 / 05)

- `workflows/sla.go:31` — SELECT-then-UPDATE 非原子,两次 tick 之间状态变化会导致 `sla_breached=true` 误设 + 重复通知(无 dedup)
- `notifications.go:145-156` — Emergency 工单双 transition(`approved` → `in_progress`)独立,第一次失败会继续尝试第二次,无补偿
- `workflows/notifications.go:117-128` — Critical alert dedup 过宽:**任意类型** open 工单就跳过,warranty 工单会阻止 emergency 工单创建
- `impl_integration.go` — 无 ETag / concurrency token,并发 PATCH 后写覆盖先写
- `sync/service.go:92-97` — publish 时读到的 `sync_version` 不保证等于写入版本(读写 race)

### 2.8 无上限并发 / 资源泄漏(出现在 03)

- `webhook_dispatcher.go:84-88` — 每事件 spawn goroutine **无上限**,突发下几百上千 goroutine + HTTP 连接
- `webhook_deliveries` 表**无 retention** — grep `DELETE FROM webhook_deliveries` 零命中,`cleanup.go` 不涉及
- `metrics.go:85-110` — 所有 adapter 串行轮询,1 个慢 adapter 阻塞整轮 tick

### 2.9 业务逻辑 bug(散见)

- `checkFirmwareOutdated` 用 `MAX(bmc_firmware)` 做字符串 lexicographical max — `1.10.0 < 1.2.0`(实打实业务 bug)
- `prometheus.go:90-98` `extractIP` 用 `strings.LastIndex(":")` 对 `[::1]:9090` 的 IPv6 解析错误
- `checkShadowIT` / `checkDuplicateSerials` 用 `description LIKE '%...%'` 做 dedup,缺索引 + 子串误匹配(`10.0.0.1` 会命中 `10.0.0.10`)
- Quality `total = 0.4*completeness + 0.3*accuracy + 0.1*timeliness + 0.2*consistency` — server 类资产缺 rack 默认扣 50,所有 switch 默认 100,硬编码偏置
- BIA 新增 dependency **不触发 `PropagateBIALevel`** — 只有 UpdateBIAAssessment 改 tier 才触发

---

## 3. 代码质量债务

| 分类 | 具体 | 证据 |
|------|------|------|
| **巨型文件** | 3 个前端 page > 800 行硬阈值 | `PredictiveHub.tsx` **1416**、`RackDetailUnified.tsx` **1227**、`SensorConfiguration.tsx` **902** |
| **测试空白** | 前端 0 个 `*.test.tsx`;E2E `asset-crud.spec.ts` 仅 9 行断言"能看到 Asset 字样" | `06-frontend-data.md` A.7 |
| **契约脱离** | 60 个 Track B handler 绕开 ServerInterface,无编译期保证 | `docs/OPENAPI_DRIFT.md` Remediation backlog 1 |
| **sqlc 覆盖不足** | 52 张表中 sqlc 只覆盖 22 张(42%);其余走手写 SQL 或 ingestion-engine | `06-frontend-data.md` C.3 |
| **迁移序号冲突** | cmdb-core 和 ingestion-engine 共享 `schema_migrations` 序号空间,靠"000010 让给 ingestion"的约定 | `06-frontend-data.md` B.3.1 |
| **handler 命名分裂** | `impl_X.go` 和 `X_endpoints.go` 两套命名共存 | `05-infra.md` §5 |
| **Modal 碎片化** | 14 个 `Create*Modal` / `Edit*Modal` 各写各的(22–336 行) | `06-frontend-data.md` A.6 |
| **硬编码装饰数据** | `Dashboard.tsx:15-70` 的 `BIA_SEGMENTS`/`HEATMAP`/`CRITICAL_EVENTS`;`src/data/fallbacks/*.ts` | `06-frontend-data.md` A.8.5 |
| **URL state 缺席** | `useSearchParams` 仅 3 处作 id 提取,过滤/分页全是 `useState` | `06-frontend-data.md` A.3 |
| **Audit 无 retention** | `audit_events` 普通表无限增长,无分区 + 无压缩 + 无归档 | `05-infra.md` §2 |
| **JWT 缺字段** | 无 `jti`/`iat`/`nbf`;登出不 invalidate 其它 refresh tokens;改密不踢其它 session | `05-infra.md` §1 HIGH |

---

## 4. 优先级修复清单

### P0 — 立刻修(安全 / 数据正确性)

1. **`SyncResolveConflict` SQL 注入 + IDOR** — `sync_endpoints.go:188,197,217`。动态拼列名来自攻击者可控的 `remote_diff` JSON key。**加 `tenant_id = $N` + 列名白名单校验**
2. **user_roles 加 tenant_id(或 AssignRole 校验 `user.tenant_id == role.tenant_id`)** — 当前可跨租户赋权
3. **users.username 唯一性改 `(tenant_id, username)`** — 当前租户 A 的 `admin` 阻止租户 B 建 `admin`
4. **Prediction / Topology 四个 IDOR** — `GetAssetRUL`、`UpdateUpgradeRule`、`DeleteUpgradeRule`、`DeleteAssetDependency`。补 `tenant_id`
5. **SSRF 防御** — Integration adapter 的 endpoint + Custom REST 的 cfg.URL。加元数据网段(169.254.0.0/16、127.0.0.0/8)黑名单或白名单
6. **ListUserSessions 权限** — 要么只返当前用户,要么要求 admin
7. **JWT secret 启动校验** — 最小长度 32 字节 + entropy check
8. **Login/refresh 独立 IP rate limit** — 当前 rate limit 在 auth 之后,未认证请求完全不限速,密码爆破无防护

### P1 — 尽快修(功能完整性)

9. **Monitoring alert evaluator** — 现在是空的。选一个:① 实现一个读 Timescale `metrics` + `alert_rules` 的内部 evaluator;② 加 Alertmanager webhook receiver。**没这个 Monitoring 整个模块是假的**
10. **Discovery Approve 实际创建 Asset** — 现在只改 discovered_assets.status,Asset 表无新行
11. **Context 生命周期修正** — `main.go` 用 `signal.NotifyContext`,关停时先 cancel 再 `srv.Shutdown`。修复所有 `<-ctx.Done()` 死代码
12. **Webhook dispatcher 熔断 + DLQ** — 加 `webhook_consecutive_failures` + auto-disable + DLQ,和 adapter 侧对称
13. **webhook_deliveries retention** — 加 30 天清理任务
14. **BIA dependency 变更触发 PropagateBIALevel** — Create/Delete dependency 也要调
15. **RCA 上下文填充** — `CreateRCA` 真正查 incident 关联的 alerts + affected assets
16. **SLA checker 原子化** — `UPDATE ... RETURNING` 一步做,避免双通知
17. **Prediction RUL / UpgradeRule CRUD 补 audit_events** — 现在完全无审计
18. **`checkFirmwareOutdated` 改用 semver 比较** — 而非字符串 max

### P2 — 系统性整顿(债务)

19. **Repo-layer tenant enforcement** — 引入 Postgres RLS 或 `tenantScoped(pool, tenantID).Query(...)` helper,把 `WHERE tenant_id=$N` 从 handler 移到不可绕过的地方
20. **统一错误处理策略** — 所有 `workflows/*.go` 文件内 3 档错误处理(吞 / Warn / Warn+Audit+Notify)统一成 "至少 Warn + metric counter"
21. **拆分 3 个巨型前端 page** — `PredictiveHub.tsx` / `RackDetailUnified.tsx` / `SensorConfiguration.tsx` 按 tab / section 拆
22. **补组件测试 + E2E 深度** — 目前 0 个 `*.test.tsx`,`asset-crud.spec.ts` 只验证"字样存在"
23. **Track B 60 个 handler 迁移到 ServerInterface** — 纳入编译期契约
24. **sqlc 覆盖补齐** — 手写 SQL 最多的 sync / prediction / notification 层切到 sqlc,消灭 dynamic SQL
25. **URL state 补齐** — 过滤/分页/排序持久化到 searchParams
26. **WorkflowSubscriber 神类拆分** — 每个子系统独立 struct,`workflows.New` 只做 composition
27. **ingestion-engine 多副本化准备** — MAC 扫描加 leader election,Celery 任务配 `autoretry_for` + `retry_backoff`
28. **迁移序号 registry** — cmdb-core 和 ingestion-engine 共享 `schema_migrations` 表但各管各的迁移文件,需要一个顶层 registry 防止撞号
29. **Audit 分区 + 冷归档** — 按月分区 + 90 天后归档 S3
30. **DB / NATS / Redis tracer propagation** — 当前 OTel trace 在 HTTP handler 出去后断掉

---

## 5. 架构层判断

### 做得特别好的

- **Integration 加密 rollout** 从 000038 迁移 → dual-write → divergence monitor → backfill CLI → cleanup SQL → 密钥轮换 CLI 的**七阶段闭环**,每阶段幂等、有 audit、dry-run 默认、SQL 带 pre-flight guard。这是一次教科书级的 at-rest 加密引入,可以作为其他敏感字段加密的模板
- **Maintenance 双维度状态机**(governance × execution)在分布式 Central+Edge 部署下允许两侧独立决策同一张工单,deriveStatus 函数容忍脏数据 — 分布式一致性场景的 pragmatic 设计
- **OpenAPI 驱动的前后端契约**:spec 是单一真相源,前端 `openapi-typescript`、后端 `oapi-codegen`、CI `check-api-routes` 三重护航。commit `eb72127` 已把 `as any` 从 51 降到 1
- **TanStack Query + WebSocket 事件驱动 invalidation**(`useWebSocket.ts:16-41`)— 是整份前端里最成熟的一块

### 架构层"洞"

- **Monitoring 名不副实** — 表齐全、API 齐全、UI 齐全,**runtime evaluator 不存在**。用户看到的 alert 都来自 location_detect + sync + seed
- **出站集成全部是占位** — `direction='bidirectional'/'outbound'` 的 adapter 永远不会被 pick up(`ListDuePullAdapters` 硬编码 `direction='inbound'`)。对外同步靠 webhook
- **Tenant 隔离是"软约定"** — 没 RLS,handler 漏写即泄漏。已发现 1 CRITICAL + 5 HIGH 是结构性的,不是偶发
- **Prediction AI 接近空手** — RCA 上下文不塞;`PredictFailure` 零调用;默认 provider `enabled=false`;`prediction_results` 表 runtime 不写。开箱体验是"AI 已接入"的幻觉
- **Sync conflict 表未启用** — 写入路径缺失,只有读 / 解决 / 过期。要么补上 insert 逻辑,要么删掉这张表的使用

### 代码质量 vs. 业务表面积

- **后端按域拆分整齐**(`impl_X.go` 16 个域 handler,`workflows/` 9 个子系统文件),最近 3 个 commit 都在做 refactor — 维护势能是正向的
- **前端三个巨型 page** 把所有 tab 放一个文件(1416 行 `PredictiveHub`),和整体按域拆分的后端形成对比。这不是架构问题,是没人拆
- **测试倒金字塔** — 后端有完整的 `*_test.go`(metrics_test / divergence_test / service_test / sla_test / statemachine_test),前端 0 个组件测试,E2E 也浅。这种"后端测试扎实 + 前端没测试"的组合特别容易在 refactor 时炸

---

## 6. 业务领域成熟度图

```
成熟度:█████ 生产就绪  ████ 可用待优化  ███ 主流程通  ██ 半壳  █ 占位

Maintenance       █████  双维状态机 / 11 条自动规则 / SLA / 审批
BIA               █████  CRUD + propagation + 5 处消费方
Integration 加密  █████  Dual-write + backfill + rotate + cleanup + CLI
Quality           ████   4 维度 + evaluateAsset,但无定时扫描
Assets / Racks    ████   核心 CRUD OK,soft/hard delete 不一致
Prediction Upgrade████   275 行业务代码,BIA boost + peer compare
Sync              ████   Envelope + layers + 双维融合,conflict insert 未启用
Frontend          ████   架构成熟,WebSocket 驱动,但 3 个巨型 page + 0 测试
Ingestion         ████   Pipeline 干净,但单副本、无 Celery 重试
Audit             ███    67 处埋点,sync/prediction 漏埋,无 retention
Prediction RCA    ██     接口齐,上下文不塞,AI 空手调
Discovery         ██     Approve 不创建 Asset,TTL OK
Monitoring        █      alert_rules 不被评估,整体空壳
Outbound adapter  █      占位,placeholder 未实现
```

---

## 7. 如果只能做三件事

1. **补 Monitoring evaluator + 修 Discovery Approve + SyncResolveConflict SQL 注入**
   这三条决定"功能是真是假"和"会不会今天就被打穿"

2. **引入 Postgres RLS 或 repo-layer tenant helper**
   1 个 CRITICAL + 6 个 HIGH 的 tenant IDOR 根源不修,后面再补也会再漏

3. **拆 3 个巨型前端 page + 补 20 个核心组件测试**
   没这个后端任何 refactor 都不敢跨到前端,测试金字塔倒挂是工程速度的硬上限

---

**维护人**:audit orchestrator
**下一次全量审计建议**:2026-07-19(每季度一次)或任何重大架构变更之后
