# 04 — Ops Intelligence: Maintenance / Monitoring / Prediction / Quality / BIA

## 模块总览

这五个模块在平台里共同构成“运维智能闭环”:

- **Monitoring** 负责感知 — 把外部告警(Prometheus / Zabbix / Custom REST)和平台内部异常(location mismatch 等)汇聚成 `alert_events`。
- **Prediction** 负责解释与预判 — 基于 `alert_events` + `incidents` 调用 AI provider 做 RCA,或结合历史 metrics + 保修期 / EOL 计算 RUL、失效分布、升级建议。
- **BIA** 负责“这件事有多严重” — 给业务系统打 tier,并通过 `bia_dependencies` 把 tier 反推 (propagate) 到资产的 `assets.bia_level` 字段,供其他模块决策使用。
- **Maintenance** 负责收敛 — 任何需要人工处理的事情最终落成一张工单 (work_order),有审批 / SLA / 双维度状态机。
- **Quality** 是横切的“数据体检” — 评估 CMDB 本身的数据完整性 / 准确性 / 及时性 / 一致性,并在写入时做 gate、在检测到 CMDB 位置和网络发现位置不一致时扣分。

关键协作:Monitoring 发事件 → workflows 订阅 → Prediction(RCA) + Maintenance(自动工单) + Quality(scan 触发校验)。五个模块不是独立的,它们是 `eventbus` + `workflows/subscriber.go` + `auto_workorders.go` 粘在一起的。

---

## 1. Maintenance — 工单与维护任务

### 数据模型 (maintenance_orders / tasks)

- 主表 `work_orders`: `cmdb-core/db/migrations/000005_maintenance.up.sql:1-22`。
- SLA 字段(`approved_at` / `sla_deadline` / `sla_warning_sent` / `sla_breached`)由 `000025_work_order_redesign.up.sql:4-9` 追加。
- 历史表 `work_order_logs`: `000005_maintenance.up.sql:28-37`。
- 评论表 `work_order_comments` 在独立 endpoint 里操作(`cmdb-core/internal/api/maintenance_endpoints.go:13-88`)。
- `prediction_id UUID` 字段在 schema 里已留坑,但代码里没被任何 service 使用。
- 注意:除了 `status` 以外还有两条隐形维度 `execution_status` 和 `governance_status`(见下)— 这些列在后续 migration 里加入;原始 `000005` 并没有。

### 生命周期与状态机

双轨状态机,定义在 `cmdb-core/internal/domain/maintenance/statemachine.go`:

- **governance_status**(Central 控制): `submitted → approved/rejected`, `approved → verified`, `rejected → submitted`。代码见 `statemachine.go:71-75`。
- **execution_status**(Edge 控制): `pending → working → done`。`statemachine.go:65-68`。
- **status** 是从两者 derive 出来的显示态。优先级:`verified > rejected > (exec 映射)`;`DeriveStatus` 函数还容忍脏数据把 `"in_progress" / "completed"` 当成 `approved`,见 `statemachine.go:80-113`。
- 统一视图意义:`submitted → approved → in_progress → completed → verified`,其中任何时候都可能 `rejected`。
- Transition 分派:`service.go:150-170` 把请求按目标态路由到 `TransitionGovernance` 或 `TransitionExecution`,两个分支都用 `ExecutionStatus_2 / GovernanceStatus_2` 做乐观锁(`service.go:194, 267`)。

### 业务规则

1. **自审批禁止**: 创建人不能审批自己的单。`service.go:321-325` 在 `validateApproval` 里比较 `operatorID == requestorID`。
2. **审批角色白名单**: 只有 `super-admin` 或 `ops-admin` 能 approve/reject,其他角色直接报“insufficient permissions”。`service.go:310-320`。
3. **`uuid.Nil` 是系统操作符**: 当 operatorID 为零值时跳过审批检查和自审批检查(`service.go:306-308`)。所有自动工单(auto_workorders、critical alert 自动派单、SLA 自动检查)用的就是这个身份,见 `cmdb-core/internal/domain/workflows/auto_workorders.go:80` / `notifications.go:131`。这是“系统行为可审批、可绕过人类规则”的唯一入口,安全审计时要重点看。
4. **只有 submitted 和 rejected 可编辑/删除**: `service.go:341-343, 359-361`,其他态一律 Forbidden。
5. **approve 时盖 SLA**: `service.go:254-260` 用 `StampWorkOrderApproval` 一次性写入 `approved_by` / `sla_deadline`,SLA duration 来自 `sla.go:6-11` 的硬编码:`critical=4h / high=8h / medium=24h / low=72h`。
6. **拒绝需 comment**: `service.go:241-243`,approved/rejected 时 comment 必填。
7. **完成 + 被拒 的异常组合**: 如果 exec 已 done 但 gov 是 rejected,触发 `SubjectOrderAnomaly` 事件(`service.go:213-221, 291-298`)— 这是给审计做兜底。
8. **与资产的关联**: `asset_id` 可空;`location_id` 可空。工单完成后会自动解决该资产上所有 `firing` 状态的 `alert_events`(`workflows/notifications.go:46-56`)并 bump `assets.updated_at`(用于 timeliness 分数)。
9. **与库存关联**: 弱关联。`inventory_tasks` 完成时如果 discrepancy > 5 会自动新建 `type='inspection'` 的工单(`workflows/notifications.go:262-281`),但没有外键。

### API 暴露面

`cmdb-core/internal/api/impl_maintenance.go`:
- `GET /maintenance/orders` — 分页 + `status` / `location_id` 过滤。
- `POST /maintenance/orders` — 创建(priority 校验在 service 里)。
- `GET/PUT/DELETE /maintenance/orders/{id}` — 单条读写。
- `POST /maintenance/orders/{id}/transition` — 状态流转,会 fetch 用户角色做审批校验(`impl_maintenance.go:127-136`)。
- `GET /maintenance/orders/{id}/logs` — 审计轨迹。
- `GET/POST /maintenance/orders/{id}/comments` — 在 `maintenance_endpoints.go` 里直接裸 SQL,绕开了 service 层(历史遗留)。

Edge 行为:Edge 节点在 `CreateAlertRule` 会被 403(`impl_monitoring.go:90-93`),但 Maintenance 没这个分区守卫,任何节点都能写工单(然后靠 sync 冲突解决)。

### 自动工单机制

**这是 Maintenance 最核心的业务粘合点**,在 `cmdb-core/internal/domain/workflows/auto_workorders.go` 里实现,入口是 `WorkflowSubscriber.Register()`(`subscriber.go:33-47`)。十条自动规则,覆盖率相当全:

| # | 触发条件 | 工单类型 | 节奏 | 代码位置 |
|---|---|---|---|---|
| 1 | 保修 ≤30d 到期 | `warranty_renewal` (medium) | daily | `auto_workorders.go:44-105` |
| 2 | 30d 内没被扫描过 | `asset_verification` (low) | weekly | `auto_workorders.go:134-199` |
| 3 | 扫描值 ≠ CMDB 值 (事件驱动) | `data_correction` (low) | 事件 `scan.differences_detected` | `auto_workorders.go:213-286` |
| 4 | EOL 已过 | `decommission` (high) | daily | `auto_workorders.go:290-342` |
| 5 | 超预期寿命 | `lifespan_evaluation` (medium) | daily | `auto_workorders.go:346-399` |
| 6 | 发现但没登记 >7d | `shadow_it_registration` (medium) | weekly | `auto_workorders.go:403-445` |
| 7 | SN 重复 | `dedup_audit` (high) | weekly | `auto_workorders.go:449-498` |
| 8 | 无 location/rack | `location_completion` (low,超过10条走bulk) | weekly | `auto_workorders.go:502-565` |
| 9 | BMC firmware 落后 | `firmware_upgrade` (low) | daily | `auto_workorders.go:569-617` |
| 10 | BMC 默认密码 (事件驱动) | `security_hardening` (critical) | 事件 `scan.bmc_default_password` | `auto_workorders.go:623-677` |
| 11 | critical alert (事件驱动) | `emergency` (critical, 自动 approve + in_progress) | 事件 `alert.fired` | `notifications.go:79-163` |

所有自动工单都用 `uuid.Nil` 作为 operatorID,并通过 `NOT EXISTS` 子查询做幂等 dedup。这一层做得很成熟,**是整个仓库 business logic 最密的区域**。

---

## 2. Monitoring — 告警与可观测性

### 数据模型

`cmdb-core/db/migrations/000006_monitoring.up.sql`:
- `alert_rules` (tenant_id, metric_name, condition JSONB, severity, enabled) — `:1-10`。
- `alert_events` (rule_id, asset_id, status in [firing/acked/resolved], severity, trigger_value, fired_at/acked_at/resolved_at) — `:12-24`。
- `incidents` (title, status in [open], severity, started_at/resolved_at) — `:30-38`。

### 告警从哪里来

**不是从 alert_rules 评估出来的。** 全仓库搜 `INSERT INTO alert_events` 和 `CreateAlertEvent` 只有三处真正插入:

1. `cmdb-core/internal/domain/location_detect/detector.go:157-161` — 当 MAC cache 发现 asset 物理位置和 CMDB 记录不一致时,直接写一条 `severity=warning, status=firing` 的告警。这是**平台内部**告警源,不需要规则。
2. `cmdb-core/internal/domain/sync/agent.go:217-221` — Edge → Central sync 时 UPSERT。告警的真正产生方在 Edge,而不是 Core。
3. `cmdb-core/db/seed/seed.sql:212` — 种子数据(8 条演示告警)。

**没有 alert_rule → alert_event 的 evaluator**。`ingestion-engine/app/collectors/` 里收集 SNMP / IPMI / SSH 数据但不跑规则。`workflows/metrics.go:59-72` 的 `StartMetricsPuller` 会去 Prometheus / Zabbix / Custom REST 拉 metrics 塞进 `metrics` 表,但同样**不做阈值判断**。`workflows/adapter_placeholder.go` 确认 SNMP / Datadog / Nagios 都是 stub。

结论:**`alert_rules` 表是配置占位 + sync fixture,没有被运行时消费过**。这是这个模块最大的坑,前端 `MonitoringAlerts.tsx` 里配了规则 UI,但配完规则不会产生新告警。真正的告警路径是:外部监控系统(Prometheus 等)在它自己那边触发,然后通过 Prometheus Alertmanager webhook / Zabbix action 把告警推到平台 — 但这个 webhook 接口我在 monitoring 模块里没看到。换句话说,`alert_events` 当前基本只有 seed + sync + 位置异常检测。

### 告警去哪里

- **通知**:`workflows/notifications.go:79-103` 订阅 `alert.fired`,对**所有严重级别**给 `ops-admin` / `super-admin` 发站内通知。
- **自动工单**:`critical` 告警 → 去重 (`notifications.go:117-128`) → 创建 `emergency` 工单 → 自动 approve → 自动 in_progress(`notifications.go:131-156`)。非 critical 不产单。
- **Webhook**:`webhook_subscriptions.filter_bia` 列在 `000014_webhook_bia_filter.up.sql` 加了,允许按 BIA tier 过滤外发 webhook — 但主订阅表的实现我没在这次审计范围内看。

### 业务规则

1. **Edge 只读告警规则**: `impl_monitoring.go:90-93, 144-147` 在 `EdgeNodeID != ""` 时直接 403,告警规则只能在 Central 管。
2. **状态机**:`firing → acked → resolved`(`service.go:167-183`)。ack 和 resolve 都是 idempotent 且 tenant-scoped。
3. **完工自动收敛**:`workflows/notifications.go:47-57` 在 `order.completed` 事件上把该资产所有 `firing` 告警批量置为 `resolved`。这个“工单解决了 = 告警自动消失”的假设挺 aggressive,没有人工确认环节。
4. **去重**:在“critical alert → emergency 工单”路径上以 `work_orders WHERE asset_id=? AND status NOT IN (完成类)` 做去重(`notifications.go:117-127`),但在 `alert_events` 本身并没有 dedup / 降噪(没有相同 rule_id+asset_id 的合并逻辑)。
5. **SLA 检查**:`workflows/sla.go` 每 60 秒扫一次,不是针对告警,是针对工单 — `sla_deadline < now()` → 打 `sla_breached=true` + 通知 assignee。

### API 暴露面

`impl_monitoring.go` + `custom_endpoints.go`:
- `GET /monitoring/alerts` (分页, status/severity/asset_id 过滤)
- `POST /monitoring/alerts/{id}/ack|resolve`
- `GET/POST/PUT /monitoring/rules` (Edge 只读)
- `GET /monitoring/alerts/trend?hours=24` — 按小时 bucket 的历史,供前端画柱状图。

---

## 3. Prediction — 预测与推荐

### 预测了什么

实际运行的有三件事:

1. **RCA(Root-Cause Analysis)**:接入 AI provider,基于 `incident_id` + context 生成推理 JSON 存入 `rca_analyses`。`POST /prediction/rca`,`cmdb-core/internal/domain/prediction/service.go:49-102`。
2. **RUL(Remaining Useful Life)**:纯 SQL 计算,基于 `purchase_date` / `warranty_expiry` / 资产类型的硬编码预期寿命(server=5y, network=7y, storage=5y, power=10y)。`cmdb-core/internal/api/prediction_endpoints.go:39-129`。
3. **升级推荐**:基于 `upgrade_rules` 表 + TimescaleDB `metrics` 的 avg / p95,结合 BIA 分数 boost 优先级,再用同型号资产做 peer 比较。`prediction_endpoints.go:250-526`。这段逻辑相当复杂(275 行),是这个模块里写得最扎实的一块。
4. **Failure Distribution**:把过去 90 天 alert_events 的 message + work_orders 的 title 用关键字匹配塞进五个桶(Thermal/Electrical/Mechanical/Software/Other)。是关键字分类,不是模型。`prediction_endpoints.go:133-245`。

### 占位/未实现的

1. **`PredictFailure`**:接口 `ai/provider.go:18` 定义了,三个 provider 都实现了(`dify.go:37` / `llm.go:41` / `custom.go:34`),**但全仓库没有任何调用方**。没有定时 job 跑它,也没有 endpoint 触发它。
2. **`prediction_results` 表**:有 create query (`prediction_results.sql.go:49` 的 `CreatePredictionResult`),**生产代码从未调用**。只有 seed.sql 会写数据。`GET /prediction/results/ci/{ciId}` 返回的就是 seed 数据。
3. **默认 prediction model** 在 migration 里插了一条 `enabled=false` 的 Dify fixture(`000011_prediction_tables.up.sql:40-43`)。

### 数据输入

- RCA:`incident_id` → `ai.RCARequest{RelatedAlerts, AffectedAssets, Context}`(`provider.go:50-56`)。但 `CreateRCA` service 层(`service.go:83-96`)只传了 tenant+incident+context,**没有把 related alerts 和 affected assets 真正查出来塞进去**。所以目前送给 AI 的上下文是空的 — RCA 拿不到有用信息。
- Upgrade Recommendations:`metrics` 表(TimescaleDB hypertable, see `000009_timescaledb_metrics.up.sql`),`upgrade_rules` 表(在 phase3 migration 里),`assets` 的 BIA 和保修字段。
- RUL:`assets.attributes` JSONB 里的 `purchase_date` / `warranty_expiry` 字符串。

### 输出形式

| 端点 | 形式 |
|---|---|
| `POST /prediction/rca` | 异步存 rca_analyses,前端轮询 |
| `POST /prediction/rca/{id}/verify` | 人工标记 `human_verified=true` |
| `GET /prediction/rul/{id}` | 单 asset 的 RUL JSON |
| `GET /assets/{id}/upgrade-recommendations` | 推荐列表 + cost estimate + alternatives + peer comparison |
| `POST /assets/{id}/upgrade-recommendations/{category}/accept` | **把推荐一键转成工单**(跨模块调用 `maintenanceSvc.Create`,`prediction_endpoints.go:553-562`) |
| `GET /prediction/failure-distribution` | 5 桶聚合 |
| `GET /prediction/models` | 返回所有 prediction_models 配置 |

### AI 服务

`cmdb-core/internal/ai/`:
- `provider.go` — `AIProvider` 接口(PredictFailure / AnalyzeRootCause / HealthCheck)。
- `dify.go` — Dify workflow,POST `/v1/workflows/run`,解包 `{"data":{"outputs":...}}`。
- `llm.go` — OpenAI-compatible chat completions(openai / claude / local_llm 都共享)。
- `custom.go` — 通用 POST `/predict` / `/rca`。
- `registry.go` — 启动时从 `prediction_models` 表把 enabled 模型实例化成 Provider;`LoadFromDB` 容忍空 lister。

实际调用链:`POST /prediction/rca` → `predictionSvc.CreateRCA(ModelName="Default RCA")` → `registry.Get("Default RCA")` → 若没找到(默认 enabled=false)就存一条 `{"status":"no_ai_provider_configured"}` 的占位 RCA(`service.go:57-70`)— **所以开箱即用时 RCA 全部是占位数据**。

---

## 4. Quality — 数据质量

### 检查了什么

四个维度,权重总和 1.0(`service.go:247`):

- **Completeness 0.4**:规则是 `dimension='completeness', rule_type='required'`,字段为空就扣权重。
- **Accuracy 0.3**:`rule_type='regex'`,JSONB 的 `rule_config.regex` 模式匹配不上就扣权重。
- **Timeliness 0.1**:硬编码规则 — `assets.updated_at > 90d` 就直接把该维度打到 60 分(`service.go:225-230`)。
- **Consistency 0.2**:硬编码规则 + 用户规则的组合:
  - server 类型没 rack_id 扣 50(`service.go:233-238`)。
  - `mac_address_cache.detected_rack_id ≠ assets.rack_id` 再扣 50(`service.go:96-119`)。这是和 location_detect 模块交叉。

不及格门槛:`total < 40` 在资产**创建时**会阻塞 — `ValidateForCreation`(`service.go:141-169`)是 asset creation 的软 gate。

### 触发时机

1. **写入时(gate)**:资产创建前,质量分 < 40 直接拒。在 `impl_assets.go` 的 CreateAsset 里调用。
2. **人工触发扫描**:`POST /quality/scan` → `ScanAllAssets`(`service.go:76-136`),一次扫 tenant 全部(limit=10000)。
3. **没有定时 job**。没有看到 `StartQualityScanner` 之类的 ticker。Dashboard 的“24 小时内平均分”完全依赖手动扫描的频率。

### 结果呈现

`cmdb-core/internal/api/impl_quality.go`:
- `GET /quality/dashboard` — 24h 汇总(avg 各维度 + 总分)。
- `GET /quality/worst` — Bottom 10。
- `GET /quality/history/{id}` — 单资产最近 30 次历史。
- `GET/POST /quality/rules` — 规则 CRUD。
- `POST /quality/scan` — 手动触发。

前端 `QualityDashboard.tsx:59-67` 按这套接口画仪表盘 + 规则列表 + "Run Scan" 按钮。

### 业务规则

1. **总分公式**:`total = completeness*0.4 + accuracy*0.3 + timeliness*0.1 + consistency*0.2`(`service.go:247`)。
2. **规则按资产类型过滤**:`rule.ci_type` 设了就只对该类型生效,NULL 则 global(`service.go:179-181`)。
3. **硬编码规则 vs 用户规则共存**:即使用户一条规则都没建,server 缺 rack / 资产 >90 天没更新依然会扣分 — 这意味着所有 switch 类资产默认 100,所有 server 默认至少扣 50 consistency。`TestEvaluateAsset_NoRules` 里明确验证了这个 90 分(`validation_test.go:11-24`)。
4. **位置一致性交叉检查**:只有当 `pool` 注入且资产有 rack_id 时才去查 mac_address_cache;否则该检查静默跳过(`service.go:96-99`)。这是 quality 和 location_detect 模块之间**唯一**的耦合。
5. **分数不会低于 0**:`service.go:242-245` 的 clamp 在维度级别,不在总分级别。

---

## 5. BIA — Business Impact Analysis

### 数据模型

`cmdb-core/db/migrations/000013_bia_tables.up.sql`:
- `bia_assessments` — 业务系统一条一条录,有 score/tier/RTO/RPO/MTPD/三个 compliance bool + owner + description(`:2-21`)。
- `bia_scoring_rules` — tier 定义(min_score/max_score/rto_threshold/rpo_threshold/color/icon),seed 通常写四档 critical/important/normal/minor(`:26-40`)。
- `bia_dependencies` — 业务系统到资产的多对多(`assessment_id × asset_id`),UNIQUE,带 `dependency_type='runs_on'` 和 `criticality`(`:43-54`)。

### 业务含义

1. **tier**:critical / important / normal / minor,**字符串比较 + 优先级**,不是枚举。
2. **BIA score** 是整数(0-100 or 0-1000 看业务约定),但 service 层不校验它和 tier 的对应关系 — 建单时 tier 和 score 是两个独立字段。
3. **RTO/RPO/MTPD**:NUMERIC(10,2),没有单位约束,前端约定 RTO=小时、RPO=分钟、MTPD=小时。
4. **三 compliance**:`data_compliance / asset_compliance / audit_compliance`,纯 bool,手动打勾。平均合规率 = 三者通过数 / 三者总检查数(`bia/service.go:160-164`)。

### 计算方式

**完全是静态配置**。不存在“基于 topology 或 metrics 自动算 tier”的路径。tier 是人在前端建 assessment 时直接选的(`CreateBIAAssessmentParams`,`impl_bia.go:44-50`)。

**传播机制**(propagation)倒是自动的:
- `PropagateBIALevelByAssessment`(`dbgen/bia.sql.go:475-495`)用一段 SQL 把 `bia_dependencies` 里所有关联资产的 `assets.bia_level` 更新为“该资产被所有关联业务系统中最严重的 tier”:
  ```sql
  CASE WHEN 'critical' = ANY(agg) THEN 'critical'
       WHEN 'important' = ANY(agg) THEN 'important'
       WHEN 'normal'    = ANY(agg) THEN 'normal'
       ELSE 'minor' END
  ```
- 触发时机只有一个:`impl_bia.go:173-178`,`UpdateBIAAssessment` 且 `req.Tier != nil` 时。**创建 assessment、新增 dependency、删除 dependency 都不会触发** — 这是一个明显漏洞,新加依赖不会让资产 bia_level 升级,除非手动改一下 tier 再改回来。

### 被谁使用

1. **Prediction / Upgrade Recommendations**:`prediction_endpoints.go:262-401` 把 `bia_level ∈ {critical, important}` 的资产推荐优先级向上提一档,并在推荐文案后缀 `[BIA: critical — prioritized]`。
2. **EOL 保护**:同上 (`prediction_endpoints.go:272-282`),eol 12 个月内的资产干脆不推荐 upgrade。
3. **Topology**:`topology_endpoints.go:169` 和 `:264` 在查询里带出 `bia_level`,前端节点着色用。
4. **Webhook filter**:`webhook_subscriptions.filter_bia TEXT[]`(`000014_webhook_bia_filter.up.sql`)支持按 BIA tier 过滤外发。
5. **Rack visualization**:`rack_slots` 视图也带了 `bia_level`(`dbgen/rack_slots.sql.go:93`)。
6. **Maintenance**:虽然 auto_workorders 很多,但**没有一个把 BIA tier 当成优先级来源** — 比如 warranty 到期工单对 critical 系统仍然是 medium 优先级(`auto_workorders.go:82-86`)。这是一个明显可以改进的联动点。

### API 暴露面(`impl_bia.go`)

- `GET/POST/GET{id}/PUT/DELETE /bia/assessments`
- `GET/PUT /bia/rules`(只能 update,不能 create — scoring rules 由 migration seed)
- `GET/POST /bia/assessments/{id}/dependencies`
- `GET /bia/stats` — 聚合(by_tier count + 三 compliance 平均)
- `GET /bia/impact/{id}` — 传入 asset_id,反查出这个资产支撑了哪些业务系统。**该接口在表没 migrate 时静默返回空数组**(`impl_bia.go:345-349`),这是一个有趣的兼容性设计,避免 phase 切换时 500。

---

## 模块协作图

```
                           ┌──────────────────┐
                           │ ingestion-engine │   (Python: SNMP/IPMI/SSH 收集)
                           │   collectors     │
                           └────────┬─────────┘
                                    │ 写 metrics / discovered_assets
                                    ▼
   ┌────────────┐   拉 metrics   ┌───────────┐    SubjectAlertFired    ┌─────────────┐
   │ Prometheus │◀───────────────│ workflows │◀────────────────────────│ Monitoring  │
   │  Zabbix    │   MetricsPuller│ /adapters │      (seed + sync +      │ alert_events│
   │ CustomREST │────────────────▶│           │    location_detect 写入)│             │
   └────────────┘                 └─────┬─────┘                         └──────┬──────┘
                                        │                                     │
                                        │ WorkflowSubscriber                  │
                                        │                                     │
                    ┌───────────────────┼───────────────────────────┐         │
                    ▼                   ▼                           ▼         │
         ┌──────────────────┐  ┌──────────────────┐    ┌──────────────────┐   │
         │   Maintenance    │  │ auto_workorders  │    │   notifications  │◀──┘
         │   work_orders    │◀─│ (11 triggers)    │    │   (站内通知)      │
         │  (status 机)     │  └──────────────────┘    └──────────────────┘
         └────┬─────────────┘
              │ POST /rca              ┌──────────────────┐
              ▼                        │     ai.Registry  │
         ┌─────────────┐   调用         │  dify/llm/custom │
         │ Prediction  │◀─────────────▶│   (RCA only)     │
         │  RCA / RUL  │               └──────────────────┘
         │  Recommend  │
         └──────┬──────┘
                │ BIA priority boost
                ▼
         ┌──────────────┐   propagate   ┌──────────────┐
         │     BIA      │──────────────▶│  assets      │
         │ assessments  │  tier→asset   │  .bia_level  │
         │ dependencies │               └──────┬───────┘
         └──────────────┘                      │ 读
                                               ▼
                                      ┌──────────────────┐
                                      │ Quality Engine   │
                                      │  (4 dims, scan)  │
                                      └──────────────────┘
```

关键事件流(按编号看 `eventbus/subjects.go`):

- `alert.fired` → `workflows.onAlertFired` → 通知 + (critical) 自动工单
- `scan.differences_detected` → `workflows.onScanDifferencesDetected` → data_correction 工单
- `scan.bmc_default_password` → `workflows.onBMCDefaultPassword` → security_hardening 工单(critical)
- `maintenance.order_transitioned` → `workflows.onOrderTransitioned` → 解决告警 + 通知 + bump updated_at
- `maintenance.order_anomaly` → 审计事件(目前只记 audit,没 handler)
- `inventory.task_completed` → 通知 + 可能自动建 inspection 工单

Quality 和 BIA **不是事件驱动的** — 完全 pull / command 模式。

---

## 整体评估

### 成熟度排序(高→低)

1. **Maintenance**:最成熟。双维度状态机、SLA、optimistic locking、log trail、auto_workorders 覆盖 11 个业务场景、完整审批逻辑 + uuid.Nil 系统身份。service_test + sla_test + statemachine_test 都有。是整个运维智能模块的中枢。
2. **BIA**:数据模型和 CRUD 完整,propagation SQL 写得干净,被多个模块(prediction / topology / rack / webhook)消费。美中不足是只有 `UpdateBIAAssessment` 改 tier 时才 propagate,新增 dependency 不触发。
3. **Quality**:引擎本身(evaluateAsset)写得不错,权重公式清晰,四维度有逻辑支撑,和 location_detect 的交叉检查是亮点。但**没有定时扫描**,完全靠人点按钮;"24h dashboard"会骗人。
4. **Prediction**:**两极分化**。Upgrade Recommendations 那段(275 行,含 P95 / BIA boost / peer comparison / cost / alternatives)是整个仓库里逻辑最密的一段业务代码。但 AI RCA 路径基本是壳(没传 related alerts / affected assets),`PredictFailure` 全链路未接,`prediction_results` 表在运行时是 dead-letter。
5. **Monitoring**:**最薄**。数据模型齐,API 齐,但**没有 rule evaluator**。alert_events 唯一的生产来源是 location_detect + sync + seed,用户配的 alert_rule 不会产出任何事件。前端 `MonitoringAlerts.tsx` 的"添加规则"按钮点完没有任何运行时效果。这是目前 Ops Intel 最需要补的地方。

### 占位/半成品清单

- `workflows/adapter_placeholder.go` — SNMP / Datadog / Nagios adapters 全部是 `not yet implemented` 错误。
- `ai.AIProvider.PredictFailure` — 三个 provider 全实现了,**零调用方**。
- `prediction_results` — 运行时 never inserted。`GET /prediction/results/ci/{id}` 返回的是 seed。
- 默认 `Default RCA` Dify provider 在 migration 里 `enabled=false`(`000011_prediction_tables.up.sql:42`),所以开箱的 RCA 一律返回 `no_ai_provider_configured` 占位记录。
- `CreateRCARequest.Context` 在 service 里接受,但 `ai.RCARequest.RelatedAlerts / AffectedAssets` 从来没被填充 — AI 收到的上下文只有 tenant_id + incident_id 字符串。
- Quality 没有定时任务,"24h 聚合"需要手动 Scan 才有意义。
- `work_orders.prediction_id` 字段设计上想做 "prediction → work order" 链接,但代码里没人写入这个字段。Upgrade recommendation 一键转工单也没用它。
- `BIA` 新增 dependency 不触发 `PropagateBIALevel`。
- Monitoring 缺一个基于 `alert_rules` 的 evaluator(或接 Alertmanager webhook)。

### 业务逻辑 vs UI 期望对齐度

| 前端页面 | 后端提供度 | Gap |
|---|---|---|
| `MaintenanceHub.tsx` (645行) | ✅ 工单 CRUD + transition + 评论 + logs 全齐 | 前端写了 `description.split(' ')[0]` 提取 asset,这种脆弱解析说明 API 层的 response 结构可以更好 |
| `MonitoringAlerts.tsx` (426行) | ⚠️ 告警 ack/resolve + trend 齐,但规则创建不产生告警 | 前端有"New Rule"按钮,用户会误以为配完规则平台会监控,实际不会 |
| `PredictiveHub.tsx` (1416行!) | ⚠️ 接口齐但很多数据靠 fallback 或者拿不到真数据 | 文件 import `RACK_SLOTS` from `data/fallbacks/predictive` 说明至少有一部分是前端 mock |
| `QualityDashboard.tsx` (378行) | ✅ dashboard/worst/history/rules 都接好 | 没有 "auto-scan schedule" 概念,用户每次要手点扫描 |
| `bia/*.tsx` (1414行共 5 页) | ✅ assessment / rule / dependency / stats / impact 都有 API | "Dependency Map" 在新增依赖后不会自动刷新资产的 bia_level,用户要手动编辑一次 tier 才会 propagate |

### 最值得先补的三件事

1. **Monitoring rule evaluator**。没这个,`alert_rules` 表是死的,整个监控模块的 UI 都是假的。要么做一个每分钟评估 Timescale `metrics` 表的内部 evaluator,要么加一个 Alertmanager webhook 接收器。
2. **BIA dependency 变更自动 propagate**。加两行,把 `CreateBIADependency / DeleteBIADependency` 也绑上 `PropagateBIALevel`。
3. **RCA 上下文填充**。在 `CreateRCA` service 里真正去查 incident 关联的 alerts + affected assets,塞到 `ai.RCARequest` 里。现在的 AI 调用基本是在空手问问题。
