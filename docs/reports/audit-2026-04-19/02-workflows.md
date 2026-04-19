# 02 — Workflows / Automation / Eventing

## 模块总览

`cmdb-core/internal/domain/workflows/` 是平台的**跨域编排层**,承担了两类职责:

1. **NATS 事件订阅者(subscriber)** — 响应其他域(maintenance/alert/asset/inventory/import/scan)发布的 domain event,执行跨模块副作用(建通知、自动工单、自动审批紧急工单、自动解决 alert)。
2. **周期性 cron/ticker 任务** — 多组独立 goroutine 按固定间隔执行数据治理扫描(过保、EOL、影子 IT、重复序列号、缺位置、固件落后)、SLA 监控、session/冲突清理、外部监控系统指标拉取、dual-write 一致性校验。

所以它**两种都是,不是纯 cron 也不是纯 subscriber**。整个域只有一个入口类 `WorkflowSubscriber`(见 `subscriber.go:13-19`),通过 `New(pool, queries, bus, maintenanceSvc, cipher)` 构造,持有 pgx 连接池、sqlc 查询层、事件总线、`maintenance.Service`、`crypto.Cipher`。

与其他域的耦合方向是**单向依赖** — workflows 会调用 `maintenance.Service.Create/Transition` 和 `integration.DecryptConfigWithFallback`,但 maintenance/integration 不反向引用 workflows。通信靠 `eventbus.Event` 解耦(`onXxx` handler) 或直接 SQL (`w.pool.Exec/Query`)。

重构痕迹:原 `subscriber.go` 1322 行已被拆成 9 个文件;当前 `subscriber.go` 只剩 49 行,只保留构造器和 `Register()` 订阅注册。

---

## 入口与生命周期

### Subscriber 启动(谁调谁、订阅了哪些 NATS subject)

唯一调用点:`cmdb-core/cmd/server/main.go:432-443`,在 `bus != nil` 分支内执行:

```go
wfSub := workflows.New(pool, queries, bus, maintenanceSvc, cipher)
wfSub.Register()
wfSub.StartSLAChecker(ctx)
wfSub.StartSessionCleanup(ctx)
wfSub.StartConflictAndDiscoveryCleanup(ctx)
wfSub.StartMetricsPuller(ctx)
wfSub.StartWarrantyChecker(ctx)
wfSub.StartAssetVerificationChecker(ctx)
wfSub.StartDivergenceChecker(ctx)
```

`Register()`(`subscriber.go:33-47`) 向 `eventbus.Bus` 注册 7 个订阅:

| Subject (常量) | 字符串值 | Handler | 文件位置 |
|---|---|---|---|
| `SubjectOrderTransitioned` | `maintenance.order_transitioned` | `onOrderTransitioned` | `notifications.go:22` |
| `"alert.fired"` (硬编码) | `alert.fired` | `onAlertFired` | `notifications.go:79` |
| `SubjectAssetCreated` | `asset.created` | `onAssetCreatedNotify` | `notifications.go:207` |
| `SubjectInventoryTaskCompleted` | `inventory.task_completed` | `onInventoryCompletedNotify` | `notifications.go:232` |
| `SubjectImportCompleted` | `import.completed` | `onImportCompletedNotify` | `notifications.go:286` |
| `SubjectScanDifferencesDetected` | `scan.differences_detected` | `onScanDifferencesDetected` | `auto_workorders.go:213` |
| `SubjectBMCDefaultPassword` | `scan.bmc_default_password` | `onBMCDefaultPassword` | `auto_workorders.go:657` |

> **注意**:`"alert.fired"` 是字符串字面量(`subscriber.go:39`),但 `eventbus/subjects.go:27` 里已有 `SubjectAlertFired = "alert.fired"` 常量。这是**一致性缺口**,不影响功能但违反了其他六条都用常量的约定。

### 定时器(哪些 ticker/cron 在哪里启动,间隔多少)

| 任务 | 间隔 | 启动函数 | 预热 | 文件:行 |
|---|---|---|---|---|
| SLA breach + warning | 60s | `StartSLAChecker` | 无 | `sla.go:13-28` |
| Session cleanup | 1h | `StartSessionCleanup` | 无 | `cleanup.go:12-26` |
| Conflict SLA + Discovery TTL | 1h | `StartConflictAndDiscoveryCleanup` | 无 | `cleanup.go:59-74` |
| Metrics 拉取(Prom/Zabbix/…) | 5m | `StartMetricsPuller` | 无 | `metrics.go:59-73` |
| Warranty / EOL / Lifespan / Firmware | 24h | `StartWarrantyChecker` | **立即跑一次** | `auto_workorders.go:19-34` |
| Missing assets / Shadow IT / Dedup serial / Missing location | 7d | `StartAssetVerificationChecker` | 无 | `auto_workorders.go:110-124` |
| Dual-write divergence 采样 | 15m | `StartDivergenceChecker` | **立即跑一次** | `divergence.go:44-75`(gated) |

7d 的 `StartAssetVerificationChecker` **没有预热**(`auto_workorders.go:111-124`),意味着服务重启后要等整整 7 天才会第一次执行 `checkMissingAssets` / `checkShadowIT` / `checkDuplicateSerials` / `checkMissingLocation`。而 24h 的 `StartWarrantyChecker` 显式 `w.runDailyChecks(ctx)` 预热,`StartDivergenceChecker` 也显式预热 — **三者行为不一致**。

### 关停顺序(context cancel 如何传递、goroutine 如何回收)

**结论:关停路径是脏的。** 证据:

- `main.go:67` 用 `ctx := context.Background()` — **没有 cancel 函数**。传给所有 `StartXxx` 的 ctx 永远不会被取消。
- `main.go:475-487` 的关停只做 `srv.Shutdown(shutdownCtx)`,HTTP server 的 shutdownCtx 是另一个独立的 context(`context.WithTimeout(context.Background(), 10*time.Second)`,`main.go:480`)。
- 所有 `StartXxx` 内部都是 `for { select { case <-ctx.Done(): return; case <-ticker.C: ... } }` 的标准模式,本身是正确的,但由于上层永不 Done,**所有 goroutine 在进程退出前都不会主动退出** — 依赖进程终止强制回收。
- `divergence.go:62` 的 "run once immediately" 调用路径在进程退出中断时会留下一个正在运行的 `checkAdapterDivergence` 或 `checkWebhookDivergence`,没有任何超时控制;同理 `pullMetricsFromAdapters` 里的 HTTP 请求超时(`prometheus.go:36` 10s,`adapter_zabbix.go:100` 15s,`adapter_custom_rest.go:62` 15s)决定了关停时最久阻塞多少秒。

**风险**:这是一个**中等级别的 shutdown 泄漏**。对一个 backend 来说尚可接受(进程死了 goroutine 也死),但会让 Postgres 侧在滚动发布时看到被中断的 connection、不优雅的 query。

---

## 主要子系统逐个剖析

### 2.1 `auto_workorders.go` — 自动工单生成

**职责**:用周期扫描 + 事件驱动两种方式,把数据治理规则转成 `work_orders` 行 + 给 ops-admin 发通知。

**触发条件**:
- `StartWarrantyChecker`(24h ticker)→ `runDailyChecks` → 4 个子检查(`auto_workorders.go:37-42`):`checkWarrantyExpiry`、`checkEOLReached`、`checkOverLifespan`、`checkFirmwareOutdated`。
- `StartAssetVerificationChecker`(7d ticker)→ `runWeeklyChecks` → 4 个子检查(`auto_workorders.go:127-132`):`checkMissingAssets`、`checkShadowIT`、`checkDuplicateSerials`、`checkMissingLocation`。
- NATS event `scan.differences_detected` → `onScanDifferencesDetected` → `checkScanDifferences`(`auto_workorders.go:213-286`)。
- NATS event `scan.bmc_default_password` → `onBMCDefaultPassword` → `createBMCSecurityWO`(`auto_workorders.go:623-677`)。

**关键业务规则**:

1. **保修临期 30 天内自动建 `warranty_renewal` 工单**(`auto_workorders.go:44-58`):`warranty_end BETWEEN now() AND now()+30d`,且同一资产无 open 的同类型工单。
2. **EOL 已过 → 建 `decommission` 工单,优先级 `high`**(`auto_workorders.go:290-326`);状态 `disposed`/`decommission` 的资产被排除。
3. **连续 30 天未被任何扫描检测到 → 建 `asset_verification` 工单**(`auto_workorders.go:134-153`),排除 `status IN ('disposed','decommission','procurement')`。
4. **BMC 默认密码 → 立即建 `critical` 级 `security_hardening` 工单**(`auto_workorders.go:623-655`),通过 `onBMCDefaultPassword` 事件触发,无 dedup 时间窗(但有 "同 asset 没有 open security_hardening" 的查重)。
5. **Missing location 有批量熔断**(`auto_workorders.go:525-560`):前 10 个建单独工单,超过 10 个合并成一张 `medium` 级 bulk 工单,避免工单风暴。

**读/写 DB 表**:
- 读:`assets`、`work_orders`、`discovered_assets`、`users`、`user_roles`、`roles`。
- 写:通过 `maintenanceSvc.Create(…)` 间接写 `work_orders`;通过 `createNotification` 写 `notifications`。

**向外发什么**:
- NATS:通过 `createNotification` 发 `SubjectNotificationCreated`(`notifications.go:177-181`)给 WebSocket 推送;`maintenanceSvc.Create` 内部会发 `maintenance.order_created`(间接)。
- 无 webhook / 邮件。

**当前风险或缺口**:

- **Scan error 被 continue 掉无告警**:`auto_workorders.go:70`、`auto_workorders.go:164`、`auto_workorders.go:315` 等所有 `rows.Scan(...) != nil { continue }` 分支直接吞 scan 错误,不记日志。如果 column 类型漂移,会静默丢行。
- **`checkShadowIT` 用 `wo.description LIKE '%' || da.ip_address || '%'` 做 dedup**(`auto_workorders.go:413`):这是子串匹配且缺索引,在 `work_orders` 表膨胀后将做全表扫描;IP 如 `10.0.0.1` 会误命中 `10.0.0.10`、`10.0.0.100`。
- **`checkDuplicateSerials` 的 dedup 同样靠 `description LIKE '%serial%'`**(`auto_workorders.go:476`),相同问题。
- **`checkFirmwareOutdated` 的 "最新版本" 定义是 `MAX(bmc_firmware)`**(`auto_workorders.go:572`):把字符串 lexicographical max 当版本比较,`1.10.0 < 1.2.0`。这是实打实的**业务逻辑 bug**。
- **`checkShadowIT` 跨租户扫 `discovered_assets` 但工单写到各自 tenant**(`auto_workorders.go:405-444`):SQL 没有 `WHERE tenant_id = ?` 过滤,跨租户聚合查询意外依赖了 `tenantID` 从行里带出。这是一个**租户隔离薄弱点**,若 `discovered_assets.tenant_id` 列有 NULL 或漂移,工单会写错 tenant。
- **`checkMissingLocation` 的 bulk 工单写到 `firstTenantID`**(`auto_workorders.go:551`):若多个 tenant 都有 missing location,bulk 只会落到第一个看到的 tenant,其他 tenant 丢失 bulk 信号。
- **`maintenanceSvc.Create` 失败只 `zap.Debug`**(如 `auto_workorders.go:88`、`auto_workorders.go:183`):用 Debug 级别遮盖了真实失败,除非 log level 开到 Debug 否则看不见。

### 2.2 `notifications.go` — 通知/提醒

**职责**:把 domain event 翻译成 `notifications` 行 + WebSocket 推送事件;另外承担两类**副作用编排**(不只是通知)。

**触发条件**:纯事件驱动,7 个订阅里 5 个由此文件处理。

**关键业务规则**:

1. **工单 `completed` 自动解决关联资产的 firing alert**(`notifications.go:47-57`):`UPDATE alert_events SET status='resolved' WHERE asset_id=? AND status='firing'`。这是一个"完成工单 → 关闭告警"的闭环。
2. **Critical alert 自动创建 emergency 工单并推进到 `in_progress`**(`notifications.go:104-156`):跳过审批,`maintenanceSvc.Transition` 两次(`approved` → `in_progress`),operatorID 用 `uuid.Nil` 作为 "system" 信号。
3. **Alert dedup 靠同资产的 open 工单计数**(`notifications.go:117-128`):只要有任意 open 工单(不限类型)就不再建 emergency 工单。这意味着一个与 alert 无关的 warranty_renewal 工单会阻止 emergency 工单创建。
4. **Inventory 完成后,若差异 > 5 自动建 inspection 工单**(`notifications.go:262-281`):阈值硬编码。
5. **ops-admin 的判定**(`notifications.go:186-205`):role name IN (`ops-admin`, `super-admin`),status = `active`;用作所有"群体通知"的收件人。

**读/写 DB 表**:
- 读:`work_orders`、`alert_events`、`users`/`user_roles`/`roles`、`inventory_tasks`、`inventory_items`。
- 写:`notifications`、`alert_events`(update)、`assets`(update updated_at)。

**向外发什么**:
- NATS `SubjectNotificationCreated` — 每次通知都推一次,供 WebSocket bridge 转发到浏览器(`notifications.go:171-182`)。
- 无 webhook / 邮件。

**当前风险或缺口**:

- **`createNotification` 完全无错误处理**(`notifications.go:166-168`):`w.pool.Exec(...)` 返回值被丢,INSERT 失败会静默。
- **`onOrderTransitioned` 里两次 `Exec` 都未检查错误**(`notifications.go:48-50, 61-63`):alert 自动解决和 `assets.updated_at` 更新失败都悄无声息。
- **Alert dedup 过宽**(`notifications.go:119`):看任意类型的 open 工单就跳过,critical alert 在有 warranty/lifespan 工单时不会升级。
- **Emergency 工单双 transition 互相独立**(`notifications.go:145-156`):第一次 `approved` 失败会继续尝试 `in_progress`,第二次必然失败,但仅 Error 日志,工单留在 `pending` 状态 — 没有补偿/重试。
- **`opsAdminUserIDs` 在每次事件中都执行 N+1 风格查询**(`notifications.go:186-205`):每个事件 → 1 次查 roles → M 次 `createNotification` 再 M 次 NATS publish。高频 alert 场景下性能退化明显。
- **`alert.fired` 用硬编码字符串**(`subscriber.go:39`):其他 6 个都用常量,唯独这一条。
- **`uuid.Nil` 作 system operatorID 的约定未文档化**(`notifications.go:145 注释`):对 `maintenanceSvc.Transition` 签名的隐式契约,重构风险高。

### 2.3 `sla.go` — SLA 跟踪

**职责**:每 60s 扫 `work_orders`,把过 deadline 的标记为 breach,把临近 deadline 的发 warning。

**关键业务规则**:

1. **Breach 标记**(`sla.go:30-55`):`status IN ('approved','in_progress') AND sla_deadline < now() AND sla_breached = false` → UPDATE + 给 assignee 发通知。
2. **Warning 阈值**(`sla.go:57-81`):`sla_deadline - (sla_deadline - approved_at) * 0.25 < now()` 即剩余时间 ≤ 总 SLA 时长的 25% 时触发;`sla_warning_sent` 作为幂等标志。
3. **通知目标仅 assignee,不通知 requestor**(`sla.go:46-51, 73-78`):SLA 信号对工单发起人是透明的。

**读/写**:读/写 `work_orders` 的 `sla_breached`/`sla_warning_sent`;写 `notifications`。

**当前风险或缺口**:

- **所有 error 都被彻底吞掉**(`sla.go:33-35, 60-62`):`if err != nil { return }` 不 log,不上报指标,SLA 检查器挂了只能靠缺失的 breach 行推断。
- **UPDATE 无错误检查**(`sla.go:45, 72`):如果并发更新冲突或 connection 断掉,`sla_breached` 不会被设,导致每 60s 重复给 assignee 发通知(没有 dedup)。
- **60s 间隔对大租户可能太密**:每次全表扫 `work_orders`,如果 `approved/in_progress` 工单多且 `sla_deadline` 列无索引,查询会变贵。
- **Warning SQL 在 `approved_at IS NULL` 时分母为零**(`sla.go:59`):SQL 里已加 `AND approved_at IS NOT NULL` 防御,但阈值公式对紧贴 deadline 创建的工单会 "warning" + "breach" 同时触发,产生双份通知。

### 2.4 `cleanup.go` — 定期清理

**职责**:session 清理、import/sync conflict 过期、discovery 过期,三个 1h 任务分两组启动。

**关键业务规则**:

1. **Session 三段策略**(`cleanup.go:28-56`):(a) `last_active_at < now()-7d` 置 `expired_at`;(b) `created_at < now()-30d` DELETE;(c) 每用户保留最新 20 条(`ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) > 20` 删)。
2. **Sync conflict 4 天警告 + 7 天自动过期**(`cleanup.go:80-109`):`created_at BETWEEN 3d 和 4d` 之间给 ops-admin 发警告;`created_at < 7d` 的置为 `auto_expired`/`auto_resolved`。
3. **Discovered asset 14 天 TTL**(`cleanup.go:121-130`):`status='pending' AND discovered_at < 14d` → `status='expired'`。

**读/写**:`user_sessions`、`sync_conflicts`、`import_conflicts`、`discovered_assets`、`notifications`。

**当前风险或缺口**:

- **所有 `Exec` 错误被丢**(`cleanup.go:30, 34, 38, 102, 107, 122`):使用 `_` 或 `_, _` 模式,清理静默失败无任何信号。
- **`import_conflicts` 可能不存在**(`cleanup.go:107 注释`):代码注释说 "if the table exists",但运行时如果表不存在 Exec 会报错 — 被 `_` 吞掉,仅此而已。应该通过 migration 统一存在性或显式用 `IF EXISTS` 语义检查。
- **Session 策略(c) 无 `WHERE tenant_id`**(`cleanup.go:38-44`):全局分区,跨租户执行。没有安全问题,但大表下 `ROW_NUMBER()` OVER 全表分区代价高,缺索引 `(user_id, created_at DESC)` 会扫全表。
- **"4 天警告"窗口只有 1 天**(`cleanup.go:82` `created_at < now()-3d AND created_at >= now()-4d`):若 ticker 跳过一次(进程重启、卡顿),这一天的警告会整体丢失且永不补发。

### 2.5 `divergence.go` — dual-write 一致性校验(最近加)

**职责**:migration 000038 引入 `integration_adapters.config_encrypted` + `webhook_subscriptions.secret_encrypted` 双写,此 sampler 每 15m 抽样 500 行比对解密结果与明文,记录 `IntegrationDualWriteDivergenceTotal` Prometheus counter + Error 日志。

**关键业务规则**:

1. **功能默认关闭,env flag `CMDB_INTEGRATION_DIVERGENCE_CHECK=1` 显式启用**(`divergence.go:45-49`)。启用策略保守,启动时打一行 decision log。
2. **无 cipher 拒绝启动**(`divergence.go:50-55`):避免 false positive 淹没 counter。
3. **JSON 语义比对而非字节比对**(`divergence.go:214-230`):`jsonEqual` 反序列化再规范化 marshal,忽略 key 顺序和空白;非 JSON fallback 到 byte compare。
4. **永不 log secret 内容**(`divergence.go:135, 195`):只打 ID、tenant、table、长度。安全意识明确。
5. **每 tick 每表上限 500 行**(`divergence.go:31` + `LIMIT $1`):观察性任务,非穷举。

**读/写**:只读 `integration_adapters`、`webhook_subscriptions`。不写 DB,只写 Prometheus counter + zap log。

**当前风险或缺口**:

- **`ORDER BY id LIMIT 500` 每次扫同样 500 行**(`divergence.go:100-101, 159-160`):若总行数 > 500,后面 500+ 的行永远不会被采样。应该用时间轮转或 `TABLESAMPLE`/随机 offset。
- **无每租户采样均衡**:若某 tenant 有 1000 adapter,前 500 全是该 tenant 的。
- **counter 在 decrypt 失败时 +1(`divergence.go:125-127`),与真正的 divergence 混在一个 label 下**:无法从 metrics 区分 "ciphertext 解密失败" vs "明文与密文不一致"。
- **ctx 传进 Query 但无独立超时**(`divergence.go:94`):用的是全局 `context.Background()` 衍生 ctx,卡住的查询会阻塞 ticker 下一次触发(ticker 本身不 drop tick,但 goroutine 永远卡在 Query 里,直到 connection 自己超时)。

### 2.6 `metrics.go` + `prometheus.go` — 指标拉取

**职责**:`StartMetricsPuller` 每 5m 轮询 `ListDuePullAdapters`(SQL 过滤 `direction='inbound' AND enabled=true AND (next_attempt_at IS NULL OR next_attempt_at <= now())`,`dbgen/integration.sql.go:308-314`),调用对应 adapter.Fetch,写 `metrics` 表。

**关键业务规则**:

1. **失败 backoff 计划 30s → 2m → 10m → 30m cap**(`metrics.go:37-48`),由 `computeAdapterBackoff` 纯函数定义,测试锁定(`metrics_test.go:13-48`)。backoff 在 SQL 侧落库到 `next_attempt_at`,**重启不会重置进度**。
2. **连续失败 ≥ 3 → 自动禁用 adapter + 写 audit 事件 + 通知 ops-admin**(`metrics.go:142-144, 190-219`)。阈值常量 `adapterDisableThreshold=3` 由测试锁定(`metrics_test.go:80-84`)。
3. **Failure reason 截断至 500 字节**(`metrics.go:28, 52-57`):保护 audit 列 + 日志大小;纯函数测试覆盖。
4. **`type='rest'` 回落到 Prometheus adapter**(`metrics.go:148-151`):向后兼容旧 adapter 类型名。
5. **`metrics` 表 INSERT 不使用 sqlc 而是裸 SQL**(`metrics.go:174-176`):绕过类型生成层,保留可变 labels JSONB。

**读/写**:
- 读:`integration_adapters`(通过 sqlc)、`assets`(FindAssetByIP)。
- 写:`metrics` 表、`integration_adapters.consecutive_failures/next_attempt_at/last_failure_reason/enabled`、`audit_events`、`notifications`。

**向外发什么**:发 `notifications` + NATS `SubjectNotificationCreated`。无 outbound webhook。

**当前风险或缺口**:

- **`ListDuePullAdapters` 无 tenant_id 过滤**(`dbgen/integration.sql.go:308-314`):跨租户扫描。这是设计选择(后台任务)但意味着任何 tenant 的 adapter 失败会占用同一轮 HTTP 预算。
- **Adapter 串行轮询**(`metrics.go:85-110`):for-each 逐个 Fetch,无并发、无 per-adapter 超时包装。1 个 adapter 慢 10s,整轮变慢 10s × N。如果 5m ticker 内没跑完,下一次 tick 会和当前 tick 重叠执行(因为 `go func() { ... }` 在同一 goroutine 串行,不会重叠,但**下一轮会延迟**)。
- **`metrics` INSERT 单行单次 Exec**(`metrics.go:174-176`):N 个点 N 次 round-trip。大量 Prometheus 结果下延迟爆炸。应 COPY 或 batch INSERT。
- **`FindAssetByIP` 失败时 assetID 保持 zero-UUID**(`metrics.go:163-172`):metric 落库时 `asset_id` 为无效 UUID,后续 join 会拿不到。业务逻辑没有区分 "IP 未匹配" 和 "查询错误"。
- **`json.Unmarshal` 错误全部忽略**(`adapter_prometheus.go:18`、`adapter_zabbix.go:46, 127, 152, 182`、`adapter_custom_rest.go:31, 76`):带 `//nolint:errcheck` 注释,配置解析失败直接拿 zero value 继续跑,行为不可预期。
- **`extractIP` 对 IPv6 错误**(`prometheus.go:90-98`):`strings.LastIndex(":")` 对 `[::1]:9090` 会在 `[::1]` 中找到 `:`,返回错误 IP。
- **`zabbixGetItemValues` 失败整静默 continue**(`adapter_zabbix.go:75-77`):`if err != nil { continue }`,Zabbix API 中断或权限问题看不见。

### 2.7 `adapters.go` + `adapter_*.go` — 三方集成适配器

**职责**:定义 `MetricsAdapter` 小接口(`adapters.go:18-24`),注册表 map 做查找(`adapters.go:27-34`)。每个具体 adapter 实现 `Fetch(ctx, endpoint, configJSON) → []MetricPoint`。

| Adapter | 文件 | 实现状态 |
|---|---|---|
| Prometheus | `adapter_prometheus.go` | 完整,复用 `prometheus.go` 的 HTTP client |
| Zabbix | `adapter_zabbix.go` | 完整,支持 API token 或 user/pass 登录 |
| Custom REST | `adapter_custom_rest.go` | 完整,dot-path JSON 导航 |
| SNMP / Datadog / Nagios | `adapter_placeholder.go` | **Stub,`Fetch` 直接返回错误**(`:12-28`) |

**关键业务规则**:

1. **统一中间格式 `MetricPoint`**(`adapters.go:10-16`):name/value/timestamp/IP/labels,跨 adapter 规范化。
2. **Prometheus 每 query 一次 HTTP**(`adapter_prometheus.go:25-29`):`cfg.Queries` 数组里 N 个 query → N 次 HTTP,任一失败立即返回(放弃整轮)。
3. **Zabbix 支持 group-filtered host 发现 → item 值拉取**(`adapter_zabbix.go:131-199`):两阶段 RPC,hostid → IP 映射只取第一个 interface。
4. **Custom REST 配置化**(`adapter_custom_rest.go:17-27`):URL、headers、method、body、result_path dot-notation,字段名可映射。

**当前风险或缺口**:

- **HTTP client 没有连接复用**:每个 `Fetch` `http.Client{Timeout: ...}` 新建一次(`prometheus.go:36`、`adapter_zabbix.go:100`、`adapter_custom_rest.go:62`),丢失 Keep-Alive 优势。应共享一个包级 client。
- **所有 adapter 在 HTTP !=200 时把 response body 整体拼回错误字符串**(`prometheus.go:52`、`adapter_zabbix.go`、`adapter_custom_rest.go:71`):真实故障时几 MB 错误页会被截断但已经走了 I/O,且泄露到日志。
- **`adapter_custom_rest.go` 在 `cfg.URL` 覆盖 `endpoint` 时**(`:33-36`)**跳过 tenant 的 endpoint 配置**:这是个危险的 tenant-local config 逃生门 — 如果 config 在 UI 上可编辑,tenant 可以把请求发到任意 URL(SSRF 风险,参见下方风险)。
- **SNMP/Datadog/Nagios 是 placeholder**(`adapter_placeholder.go:12-28`):但在 `adapterRegistry` 里正常注册(`adapters.go:31-33`)。创建一个 `type='snmp'` 的 adapter 时不会有校验拦截,5 分钟后在 puller 里 Fetch 失败并计入 `consecutive_failures`,3 次后自动 disable — 本质上**让用户通过失败来发现"没实现"**。应在 adapter 创建接口侧校验。
- **Custom REST 的 SSRF**:`cfg.URL` + `cfg.Headers` 完全可控,可以指向 `http://169.254.169.254/` (云元数据)或内网服务。没看到任何 URL 白名单/黑名单。
- **Zabbix auth token 在 `zabbixRPCRequest.Auth` 字段直传**(`adapter_zabbix.go:30`):序列化进 request body,通过 HTTP(可能非 TLS)发到 endpoint。当前代码不强制 HTTPS。
- **JSONPath 用 `strings.Split(".")` 实现**(`adapter_custom_rest.go:81-86`):对包含 `.` 的 key、数组下标均不支持。

---

## 错误处理与回退

错误处理总体呈**"日志 + 继续"**模式,有三档明显差异:

| 档次 | 例子 | 问题 |
|---|---|---|
| **完全吞掉(最差)** | `cleanup.go:30, 34, 38` 用 `_`;`notifications.go:166-168` 的 `createNotification` 完全不看 err;`sla.go:45, 72` 的 UPDATE 无错误检查 | 无法诊断失败,无指标,无告警 |
| **只打日志** | `auto_workorders.go` 所有 `zap.L().Warn("... query failed", zap.Error(err)); return` | 可见,但无指标联动、无告警订阅 |
| **日志 + 指标 + audit** | `metrics.go:196-253` 的 `disableAdapter` 路径:warn 日志 + `audit_events` INSERT + ops-admin notification | 做对了,但整个 workflows 里只有 metrics puller 这一条路径这么完整 |

**有告警**的只有:
- Adapter auto-disable(`metrics.go:211-218`)— 发 ops-admin 通知。
- BMC 默认密码(`auto_workorders.go:647-653`)— 发 ops-admin 通知。
- SLA breach / warning(`sla.go:46-51, 73-78`)— 发 assignee 通知。
- Conflict 4-day warning(`cleanup.go:86-98`)— 发 ops-admin 通知。
- Divergence(`divergence.go:136-142, 195-201`)— 只 Error 日志 + Prom counter,**不发通知**。

**只记日志不告警**:所有 `runDailyChecks` / `runWeeklyChecks` 的 query error(8 处),所有 `maintenanceSvc.Create` 失败(全走 `zap.Debug`,默认看不见)。

**完全无可观测**:`createNotification` 的 INSERT 失败、`cleanupSessions` 的 3 个 Exec、`onOrderTransitioned` 的 alert 自动解决 / asset updated_at 更新。

---

## 并发与顺序性

**goroutine 布局**:
- 7 个独立 `Start*` 每个起 1 个 goroutine,各自持 ticker,互不通信。
- NATS subscriber handler 由 `eventbus.Bus` 的实现决定 — 需要看 NATS 侧(不在 workflows 范围)是否对每条 message 起新 goroutine,默认 NATS JetStream 是并发 delivery。

**潜在并发风险**:

1. **多 goroutine 共享同一 `WorkflowSubscriber` 实例**:字段 `pool`、`queries`、`bus`、`maintenanceSvc`、`cipher` 都是只读,`WorkflowSubscriber` 无可变字段,所以对象本身是并发安全的。
2. **`createNotification` 从多个并发 handler 调用**(`notifications.go:165-183`):INSERT + NATS Publish 非原子。若 NATS publish 中断,DB 有 notification 行但 WebSocket 收不到。反过来,若 INSERT 失败(静默) + publish 成功,WebSocket 通知没有对应 DB 行,前端刷新时丢失。
3. **`checkScanDifferences` 和 24h `checkEOLReached` 等**可能同时对同一 asset 起多张工单:每个检查有独立 "same-type open WO" 查重,但**不同类型**的查重不跨类型 — 这是 by design(一个 asset 可以同时有 EOL + warranty 工单)。
4. **`opsAdminUserIDs` 的 query → multi-notify 顺序**:alert 风暴时,N 个 alert event 各自启动 `onAlertFired` → 各自查一遍 roles → 各自 publish。数据库层可能承压(有 cache 收益但未实现)。
5. **SLA ticker 的 UPDATE 无原子 CTE**(`sla.go:31`):先 SELECT 再 UPDATE,同一行在两次 tick 之间被 API 修改(例如工单被改为 completed)→ SLA ticker 仍会把 `sla_breached=true`,与实际不符。应改成 `UPDATE ... WHERE id=? AND status IN (...) AND sla_deadline < now() RETURNING id, code, assignee_id` 一步做。
6. **MetricsPuller 串行但事件订阅并发**:如果一次 pull 耗时超过 5m,下一次 tick 会叠加(`time.NewTicker` 会丢失错过的 tick,所以不会叠加,但会漂移)。

**没看到竞态**:没有共享可变 map / slice / counter,`telemetry.IntegrationDualWriteDivergenceTotal` 是 Prometheus counter 本身线程安全。

---

## 整体评估

**拆分收益明确**:从 1322 行的单文件到 9 个按子系统划分的文件(最大的 `auto_workorders.go` 677 行、`notifications.go` 312 行),每个文件职责聚焦。每个 `Start*` 入口放在它所服务的子系统文件里,而非堆在 subscriber.go。测试也按子系统分(`metrics_test.go`、`divergence_test.go`、`prometheus_test.go`、`metrics_integration_test.go`),覆盖率模式健康。`subscriber.go` 瘦身到 49 行只保留构造 + 订阅注册,**主干清晰度达标**。

**但解耦不完整**:

1. **`WorkflowSubscriber` 这个神类还在**:所有 cron + 所有 subscriber 都挂在同一个 receiver 上。文件拆分了,但**运行时仍是一个对象**,内部状态(pool、bus、cipher)共享 —— 任何子系统的增加都要改同一个 `New(...)` 签名,main.go 串联 8 个 `Start*` 也说明"子系统"是虚的分组。真正的解耦应让每个子系统有自己的 struct 和构造器,`workflows.New` 只做 composition。
2. **跨文件隐式依赖**:`notifications.go:185-205` 的 `opsAdminUserIDs` 被至少 5 个文件调用(`auto_workorders.go`、`metrics.go`、`cleanup.go`、`notifications.go` 内部);`createNotification` 同理。这些应该提到一个显式的 helper 文件或 service 抽象,避免 `auto_workorders.go` 改动牵连通知格式。
3. **error handling 不统一**:见"错误处理"一节,同一个文件内 3 档错误策略并存,没有顶层约定。
4. **ctx 生命周期断裂**:main.go 用 `context.Background()` 传入,所有 `Start*` 的 `<-ctx.Done()` 分支都是**死代码**。重构没有修这一条。
5. **事件订阅和 cron 混在一类**:语义上 subscriber(事件响应)和 scheduler(时间驱动)是两种不同的 runtime,放一个包可以,但放一个 struct 会让测试替身膨胀。

**具体给下一步改进的建议**(非本报告职责,但基于审计结论):

- 把 8 个 `Start*` 抽成 `Scheduler` 接口,`main.go` 只调 `workflows.StartAll(ctx, cfg)`。
- 把 subscriber 从 `WorkflowSubscriber` 拆出,改为独立 `NotificationSubscriber`、`AutoWorkOrderSubscriber`、`ScanReactionSubscriber`。
- 统一 "maintenanceSvc.Create 失败" 和 "query 失败" 的日志级别(至少 Warn,不要 Debug)。
- `main.go` 的 `ctx` 改为 `signal.NotifyContext` 或手动 `WithCancel`,关停时先 cancel 再 `srv.Shutdown`。
- 字符串常量 `"alert.fired"` 替换为 `eventbus.SubjectAlertFired`。
- `checkFirmwareOutdated` 的 "latest" 语义修掉(lexicographical max 不是 semver)。
- `checkShadowIT`/`checkDuplicateSerials` 的 `description LIKE` dedup 改成显式关联列或专用 dedup 表。
