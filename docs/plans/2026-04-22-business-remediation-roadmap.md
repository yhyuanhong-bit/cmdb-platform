# CMDB Platform 业务修复路线图 — 2026-04-22

> 基于 `docs/reviews/2026-04-22-business-fit-review.md` 的 12 项业务缺陷
> 目标：从 65% 业务覆盖度提升到 90%+，让平台从「资产管理工具」升级为「真正的 CMDB system of record」
> 假设：3 后端 + 1 前端 + 1 product，全职 6 个月

---

## 总览

```
   M1                     M2                     M3
   ├─ Foundation ─────────┤
   ├──── Service 实体 ────┤
                ├── Discovery 闸 ──┤
                          ├─ ITSM PoC ─┤────── ITSM 完整 ──┤
                                       ├─ 跨页修复 + 占位清理 ──┤
                                                              ├ Service 告警聚合 ┤
                                                                        ├─ CAB ──┤
                                                                                  ├─ Teams ─┤
                                                                                            ├─ Edge 队列 ──→ M4
```

**M1 (8 周)**: Foundation + P0 (#1, #3) — 让数据模型对 + 数据可信
**M2 (8 周)**: ITSM + UX 完整性 (#2, #6, #7, #10) — 接外部 + 体验自洽
**M3 (8 周)**: 服务编排 (#5, #8, #9) — 智能告警 + 治理闸 + 团队
**M4 (4 周, optional)**: Edge 真离线 (#4) — README 兑现

---

## Foundation Wave (M1-W1)

修复路线本身的前置条件。1 周做完，所有后续依赖。

### F.1 引入 specs/ 目录的 schema 改动审查规范
**问题**：现在 schema 改动散落在 migrations/，没有「为什么这么改」的设计文档
**做**：
- 新建 `cmdb-core/db/specs/` — 每个新实体（service、policy、teams）先有 spec 再写 migration
- spec 包含：业务需求、ER 草图、初始数据、reconciliation 策略（如果是新增已有数据的关联表）
- README 加 contributor section

### F.2 加 `db/seed/test-fixture.sql`
**问题**：现在只有一份 production-leaning seed，集成测试和手测共用，相互污染
**做**：
- 拆 `seed.sql` → `seed.sql` (production minimum) + `test-fixture.sql` (开发测试用，含示例 service / 多团队 / 多种 status)
- 集成测试 CI 改用 test-fixture
- 文档化两份 seed 的边界

### F.3 把 OpenAPI 当真相来源
**问题**：`docs/OPENAPI_DRIFT.md` 暗示 spec 和 impl 已经漂移
**做**：
- 提升 `cmd/check-api-routes` 工具到必过 CI（现在只是 lint）
- 加测试：`go test -run TestOpenAPIRoundTrip` 保证 spec 描述的每个 endpoint 都有 handler
- 任何后续 P0/P1 工作必须先改 OpenAPI 再写代码

**预估**：1 周（1 个后端）
**完成判据**：F.3 的 CI 检查在每个 PR 上跑

---

## M1 (Week 2-8) — 让数据模型对 + 数据可信

### 🔴 #1 引入 Service 实体（M1-W2..W4，3 周）

最重要的一项。所有 service-centric 能力（#5 告警聚合、BIA 真正可用、Edge 资源规划）依赖这个。

#### 1.1 数据模型 (W2)

新 migration `000062_business_services.up.sql`:

```sql
-- 业务服务 — 一个用户可见的功能（订单系统、支付网关、邮件）
CREATE TABLE services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(64) NOT NULL,            -- 业务侧引用 ID (ORDER-API)
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    owner_team      VARCHAR(100),                    -- 等 #9 完成后改 FK
    tier            VARCHAR(20) NOT NULL DEFAULT 'normal',  -- 复用 BIA tier 取值
    status          VARCHAR(20) NOT NULL DEFAULT 'active',  -- active/deprecated/decommissioned
    bia_assessment_id UUID REFERENCES bia_assessments(id),  -- 复用现有 BIA
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (tenant_id, code),
    CHECK (status IN ('active', 'deprecated', 'decommissioned')),
    CHECK (tier IN ('critical', 'important', 'normal', 'low'))
);

-- Service ↔ CI 关系，N:M
CREATE TABLE service_assets (
    service_id   UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    asset_id     UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    role         VARCHAR(50) NOT NULL,           -- primary/replica/cache/proxy/dependency
    is_critical  BOOLEAN NOT NULL DEFAULT false, -- service 失败必须该 asset 健康
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id, asset_id)
);

-- 联接 BIA 现有评估到 service（迁移路径）
ALTER TABLE bia_assessments ADD COLUMN service_id UUID REFERENCES services(id);
```

数据迁移：从 `bia_assessments.system_code` 反向创建对应的 service 记录（一次性迁移脚本）。

#### 1.2 后端 API (W2-W3)

新 endpoints (in OpenAPI first per F.3):
- `GET/POST /services` — list + create
- `GET/PUT/DELETE /services/{id}` 
- `GET /services/{id}/assets` — list of CIs in this service
- `POST /services/{id}/assets` / `DELETE /services/{id}/assets/{asset_id}` — manage membership
- `GET /services/{id}/health` — 聚合状态：所有 critical asset 健康才算 healthy
- `GET /services/{id}/dependencies` — service↔service 依赖（通过 asset_dependencies 推算）
- `GET /assets/{id}/services` — 反向查询：这个 asset 服务于哪些 service

新文件：
- `internal/domain/service/model.go`
- `internal/domain/service/service.go`
- `internal/api/impl_services.go`

#### 1.3 前端 (W3-W4)

新页面：
- `src/pages/Services.tsx` — service 列表 + 创建
- `src/pages/ServiceDetail.tsx` — 单个 service 视图：CI 列表、健康度、依赖图、关联工单
- `src/pages/ServiceMap.tsx` — 跨 service 的依赖网络图（类似 ServiceNow service map）

修改既有页面：
- `AssetDetailUnified.tsx` — 加「Belongs to services: ...」section
- `Dashboard.tsx` — 加「Services by tier」tile
- `MonitoringAlerts.tsx` — 加 service filter

**完成判据**：
- 用户能在 UI 创建一个 service，挂上 5 个 asset
- 任何一个 critical asset offline → service health 显示 degraded
- 反向查询：点 asset 详情能看到属于哪些 service

**风险**：现有 BIA assessments 需要决定如何迁移。建议：保留 bia_assessments 做评估记录，新增 service 做实体；用 service_id FK 连接。**不删任何旧表。**

---

### 🔴 #3 Discovery 审核闸门（M1-W5..W6，2 周）

#### 3.1 默认行为改为 review-only

修改 `ingestion-engine/app/pipeline/processor.py`:
- 移除 `auto` 路由模式（或保留但默认不启用）
- `smart` 模式不再自动合并到已存在 CI；改为生成 `discovered_assets` 行 + 标记 `match_confidence`
- 加 `match_strategy` 字段记录匹配理由（serial / asset_tag / hostname / ip）

#### 3.2 新增 review queue UI

新页面 `src/pages/DiscoveryReview.tsx`:
- 分 tab：Pending / Matched (auto) / Conflicts / Ignored
- 每条记录显示：discovered raw data | matched CI（如有）| confidence score | "Approve" / "Reject" / "Manual link"
- 批量操作：选 N 条 → 一键 approve

#### 3.3 冲突自动开工单

修改 `internal/domain/workflows/auto_workorders_governance.go`:
- 新增 trigger：`discovered_assets.status = 'conflict' AND created_at < now() - interval '24 hours'`
- 自动开工单类型 `discovery_conflict`，分配给数据治理团队

#### 3.4 Audit log 增强

discovery 的所有 approve/reject 都写入 audit_events，包含：
- 原始 discovery 数据 hash
- 匹配的 CI ID（如果是 update）
- 操作者 + 理由（required field）

**完成判据**：
- 默认 SNMP 扫描结果不再自动入库
- UI 上能看到「12 条待审核 discovery」
- 24h 未审核的自动开工单
- 可以用 audit log 证明每条 CI 创建都有审核痕迹

**预估**：2 周（1 后端 + 0.5 前端）

---

### #10 状态字段加 CHECK 约束（M1-W7，0.5 周）

**为什么放这里**：在 #1 之前数据已经脏了，趁早冻住边界。

新 migration `000063_status_constraints.up.sql`:

```sql
-- 先清理脏数据（dry-run 先看一遍）
UPDATE assets SET status = 'unknown' 
  WHERE status NOT IN ('planned','in_stock','active','maintenance','retired','disposed','unknown');
UPDATE work_orders SET status = 'draft' 
  WHERE status NOT IN ('draft','submitted','approved','in_progress','completed','verified','rejected','cancelled');
-- ... 类似处理所有状态字段

-- 加约束
ALTER TABLE assets ADD CONSTRAINT chk_assets_status 
  CHECK (status IN ('planned','in_stock','active','maintenance','retired','disposed','unknown'));
ALTER TABLE work_orders ADD CONSTRAINT chk_work_orders_status 
  CHECK (status IN ('draft','submitted','approved','in_progress','completed','verified','rejected','cancelled'));
ALTER TABLE discovered_assets ADD CONSTRAINT chk_discovered_assets_status 
  CHECK (status IN ('pending','matched','conflict','approved','ignored'));
```

**风险**：脏数据清理可能影响生产数据。需要：
- DBA 先 dry-run 跑 SELECT 看哪些行会被改
- 把 cleanup 拆成单独的 cutover SQL（参照 `ops/cutover/2026-04-20-...`）
- 加约束的 migration 单独提交

**完成判据**：
- migrate up from zero clean
- 现有 DB 加约束不报错
- 应用层 invalid status 写入返回明确错误

**预估**：0.5 周（1 后端，主要是数据清理 dry-run）

---

### M1 第 8 周 — 集成 + Demo

- 把 #1, #3, #10 串起来跑：创建 service → 设备扫描 → review queue → approve → service health 反映
- 录制 5 分钟 demo 给 stakeholder
- 写 v1.3 changelog

---

## M2 (Week 9-16) — 接外部 + 体验自洽

### 🔴 #2 ITSM 集成（M2-W1..W6，6 周）

最大的一块，单独工程师全职。

#### 2.1 Adapter 抽象层 (W1-W2)

现有 `adapter_*.go` 只覆盖 inbound。需要做 outbound 抽象：

```go
// internal/domain/integration/itsm.go
type ITSMAdapter interface {
    CreateTicket(ctx context.Context, req TicketRequest) (*TicketResponse, error)
    UpdateTicket(ctx context.Context, ticketID string, req TicketUpdate) error
    GetTicketStatus(ctx context.Context, ticketID string) (*TicketStatus, error)
}

type TicketRequest struct {
    Title       string
    Description string
    Priority    string  // 映射 work_order.priority
    AssetID     uuid.UUID
    ServiceID   uuid.UUID  // 依赖 #1
    Category    string
    Metadata    map[string]string  // 自定义字段
}
```

#### 2.2 ServiceNow adapter (W3-W4)

PoC 先：
- 实现 `internal/domain/integration/itsm_servicenow.go`
- 支持 OAuth2 认证 + Table API
- map：cmdb work_order → ServiceNow `incident` table
- 支持双向同步：ServiceNow ticket 状态变化 webhook 回 CMDB → 更新 work_order

#### 2.3 工单 dispatch 配置 (W4-W5)

```sql
-- 新 migration: ITSM 路由规则
CREATE TABLE itsm_routing_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(255) NOT NULL,
    -- 触发条件
    work_order_type VARCHAR(50),    -- match work_orders.type
    service_tier    VARCHAR(20),    -- match services.tier
    priority_min    VARCHAR(20),    -- only critical+
    -- 路由目标
    adapter_type    VARCHAR(50) NOT NULL,  -- servicenow / jira / generic_webhook
    adapter_config  JSONB NOT NULL,
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

UI 加 `src/pages/ITSMRouting.tsx` 让管理员配规则。

#### 2.4 Workflow subscriber 接 ITSM (W5)

修改 `auto_workorders_governance.go` 等：每次 `maintenanceSvc.Create` 后，检查 routing rules，匹配则推 ITSM。

#### 2.5 Jira adapter + 通用 webhook (W6)

类似 ServiceNow，作为第二个 reference impl。

**完成判据**：
- 设备 warranty 到期 → 自动开 cmdb 工单 + ServiceNow incident
- 工程师在 ServiceNow 标记 close → cmdb 工单状态同步更新
- routing rules UI 可配置「只把 critical 工单推 ServiceNow」

**风险**：ServiceNow 不同 instance 自定义字段差异巨大。先用 minimum viable mapping，后续按客户需求加。

**预估**：6 周（1 后端 + 0.2 前端）

---

### #6 清理 11 个 phase-3.10 占位符（M2-W3..W4 并行，2 周）

不是每个都要补端点，**先做产品决策**：

| 占位 | 决策 |
|---|---|
| `Dashboard.tsx:244` 资产趋势图 | **补端点** — 后端有 audit_events 时序，加 `/dashboard/assets-trend` |
| `Dashboard.tsx:426` 机柜热力图 | **补端点** — 后端有 racks + occupancy，加 `/racks/heatmap` |
| `EnergyMonitor.tsx:351,503,608,622` 4 处 | **下线** — 没有 power 采集 collector，整个 Energy 模块改为「Coming soon」横幅 |
| `InventoryItemDetail.tsx:82,216` | **补链接** — 后端 API 已经有，纯前端工作 |
| `AssetLifecycleTimeline.tsx:238` compliance | **依赖合规模块** — 推迟到 M3 |
| `predictive/TimelineTab.tsx:140,161` | **下线或补端点** — 看 product 是否要继续推预测功能 |

**预估**：2 周（1 后端 + 1 前端 part-time）

---

### #7 跨页导航断链（M2-W5..W6，1.5 周）

机械工作但收益高。系统化梳理：

1. 列出所有 entity-to-entity 引用：
   - Alert → Asset → Rack → Location
   - Work Order → Asset / Service
   - Inventory Task → Asset
   - Audit Event → Target (asset / rack / user / ...)
   - Discovery → Matched Asset (after #3)
   
2. 每对加 `<Link to={`/assets/${id}`}>` — 写一个 helper hook `useEntityLink(type, id)` 集中管理路由映射

3. 删掉 `WorkOrder.tsx` 里 `description.split` 解析 asset id 的脆弱代码 — 改用 `work_orders.asset_id` FK

**完成判据**：从 alert 详情 1 click 到 service map，全前端链路连通

**预估**：1.5 周（1 前端）

---

### M2 第 7-8 周 — 集成 + Demo

跑端到端：discovery → review approve → 关联 service → 高优先级告警 → 自动开 cmdb 工单 + ServiceNow incident

---

## M3 (Week 17-24) — 智能告警 + 治理闸 + 团队

### #5 Service-centric 告警聚合（M3-W1..W2，2 周）

依赖 #1 完成。

#### 5.1 引入 incident 概念

新 migration `000064_incidents.up.sql`:

```sql
CREATE TABLE incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    service_id      UUID REFERENCES services(id),  -- 关联到 service
    severity        VARCHAR(20) NOT NULL,           -- critical/major/minor
    status          VARCHAR(20) NOT NULL DEFAULT 'open',
    title           VARCHAR(255) NOT NULL,
    description     TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('open', 'investigating', 'mitigated', 'resolved', 'closed'))
);

CREATE TABLE incident_alerts (
    incident_id  UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id     UUID NOT NULL REFERENCES alert_events(id) ON DELETE CASCADE,
    PRIMARY KEY (incident_id, alert_id)
);
```

#### 5.2 告警聚合规则

修改 `notifications.go onAlertFired`：
- alert 触发后，查询是否已有 open incident 关联到同一 service
- 有 → 加入现有 incident（不重复开工单）
- 无 → 开新 incident + 单一工单
- service tier 决定 incident severity（critical service 即使是 minor alert 也升级）

#### 5.3 前端 incident 视图

`src/pages/Incidents.tsx` — 类 PagerDuty 视图，按 service 分组，显示关联告警和工单。

**完成判据**：
- 一个 service 三台机同时告警 → 只生成 1 个 incident
- incident 详情显示「3 台机器、2 条告警、1 个工单」
- BIA tier 字段开始真正影响 incident 优先级

**预估**：2 周（1 后端 + 0.5 前端）

---

### #8 变更审批闸门 (CAB)（M3-W3..W5，2.5 周）

#### 8.1 风险分级

新 migration `000065_change_risk.up.sql`:

```sql
-- 给 work_order 分类风险
ALTER TABLE work_orders 
    ADD COLUMN risk_level VARCHAR(20) NOT NULL DEFAULT 'low',
    ADD COLUMN approval_required BOOLEAN GENERATED ALWAYS AS (risk_level IN ('high', 'critical')) STORED,
    ADD CONSTRAINT chk_work_orders_risk CHECK (risk_level IN ('low','medium','high','critical'));

-- 风险计算规则（应用层）：
-- - decommission asset → high
-- - service tier=critical 上的任何变更 → high
-- - production 时段（业务时间）→ +1 级
-- - rollback 困难（无 rollback plan 字段填了）→ +1 级
```

#### 8.2 审批流改造

修改 `internal/domain/maintenance/statemachine.go`：
- `risk_level >= high` 的工单：从 `submitted` → `approved` 必须经过 CAB 角色审批
- 自动工单（governance、auto_workorders_*）默认 `risk_level='low'`，不需要审批
- **decommission 类工单强制 `risk_level='high'`**，即使是自动开的也要人工审

#### 8.3 CAB 审批 UI

`src/pages/ChangeApprovalQueue.tsx` — CAB 成员看待审批工单，逐个 approve/reject + 备注（写入 work_order_logs）

**完成判据**：
- 试图 retire production server → UI 提示「此变更需要 CAB 审批」，不能直接执行
- CAB 成员收到通知，UI 看到队列
- audit log 显示完整 chain：requestor → CAB approver → executor

**风险**：现有自动工单流要确保不被误判为 high risk 而堵在审批队列。Migration 设默认 'low' + 显式标 'high' 才升级。

**预估**：2.5 周（1 后端 + 0.5 前端）

---

### #9 Teams 实体（M3-W6..W7，1.5 周）

#### 9.1 Schema

新 migration `000066_teams.up.sql`:

```sql
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    code        VARCHAR(64) NOT NULL,        -- 业务侧引用 (NETWORK-OPS)
    name        VARCHAR(255) NOT NULL,
    parent_id   UUID REFERENCES teams(id),   -- 支持层级
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);

CREATE TABLE user_teams (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    role        VARCHAR(50) NOT NULL DEFAULT 'member',  -- member/lead/admin
    PRIMARY KEY (user_id, team_id)
);

-- 把现有 owner_team VARCHAR 转成 FK（数据迁移）
-- Step 1: 从 assets.owner_team 反向创建 teams 记录
-- Step 2: 加新列 owner_team_id, backfill from owner_team string match
-- Step 3: 应用层迁移到读 owner_team_id
-- Step 4: drop owner_team VARCHAR (后续 milestone)
ALTER TABLE assets ADD COLUMN owner_team_id UUID REFERENCES teams(id);
ALTER TABLE services ADD COLUMN owner_team_id UUID REFERENCES teams(id);
```

#### 9.2 通知路由

修改 `notifications.go opsAdminUserIDs()` —> 新函数 `notifyTargets(ctx, tenantID, asset_id, service_id)`:
- 优先：service.owner_team 的成员
- 次选：asset.owner_team 的成员
- 兜底：ops-admin 角色

#### 9.3 Teams 管理 UI

`src/pages/Teams.tsx` — 创建团队、加成员、设置 lead

**完成判据**：
- alert on asset X (owner_team=NETWORK) → 只通知 NETWORK 团队成员，不再 spam ops-admin
- 团队 lead 在 dashboard 看到「我团队的待办工单」

**预估**：1.5 周（1 后端 + 0.5 前端，含数据迁移）

---

### M3 第 8 周 — 集成 demo

跑端到端：service health degraded → incident 自动聚合 3 个 alert → CAB 审批 → ServiceNow ticket → NETWORK 团队 lead 收到通知

---

## M4 (Week 25-28, optional) — Edge 真离线（4 周）

只在客户真要 edge offline 时才做。否则保留 SyncGate 的 503 行为 + 改 README。

### 4.1 Edge 端本地写缓冲

新增 `cmdb-core/internal/edge/buffer.go`:
- 用 SQLite 在 edge 节点本地存「待 sync 的 mutations」
- 新写入流程：DB 写 + 写 buffer
- 后台 worker：网络恢复时扫 buffer → 转 SyncEnvelope → 推 NATS

### 4.2 SyncGate 行为改造

不再 503，改为：
- DB 写正常进行（写 buffer 不写 sync_state）
- 读：优先 local DB，但 UI 显示「离线模式 — 数据可能不是最新」横幅

### 4.3 重连后冲突处理

- buffer 回放产生的 envelope 用 LWW（已实现）
- 但要标记「offline-originated」，central 端可以选择优先级

**完成判据**：
- 拔掉 edge 节点上行网络
- 在 edge UI 创建工单 / 改 asset → 成功
- 恢复网络
- 5 分钟内 central 看到 edge 期间的所有变化

**风险**：高。多站点 LWW 在真实场景下经常出现「edge A 和 edge B 同时改了 central 数据」的脑裂。建议先和客户确认 edge 用法是「只读为主」还是「真的并发写」，决定是否值得这 4 周投入。

**预估**：4 周（2 后端 — 这个并发性高，需要 senior pair）

---

## 跨切关注（每个 milestone 都要做）

### CC.1 数据迁移策略
每次新增实体都涉及 backfill。**统一原则**：
- 不删旧表/旧字段（保留 6 个月做 backout）
- 加新字段或表，应用层 dual-read，逐步切换
- 切换完成后再下个 milestone 删旧的
- 每个数据迁移有专门 cutover SQL 在 `ops/cutover/`，DBA review 后执行

### CC.2 测试策略升级
- 每个新实体必须有：unit tests >40%、integration test >20%、E2E spec >1
- 复用 `//go:build integration` 框架（已 CI 自动跑）
- 加一个新 CI job：跑 spec/ 文档的 schema lint，确保新 migration 有对应 spec

### CC.3 OpenAPI 优先
- 所有新 endpoint 先写 OpenAPI 再写代码（F.3 已建立 CI 检查）
- 每个 P0/P1 task 包含 OpenAPI patch + drift check

### CC.4 监控指标
每个新模块自带 metrics：
- 新增 counter/histogram 命名遵循 `cmdb_<domain>_<event>_total`
- 关键新流程定义 SLO（参考 `docs/slo-definition.md` 模式）

### CC.5 文档
每个 milestone 末尾更新：
- `CHANGELOG.md` — 用户视角
- `docs/DATABASE_SCHEMA.md` — 实体变化
- `README.md` — Features section 增删

---

## 资源与里程碑

| Milestone | 周数 | 团队 | 业务能力解锁 |
|---|---|---|---|
| Foundation | 1 | 1 BE | OpenAPI 真相源 + spec 流程 |
| M1 | 7 | 2 BE + 0.5 FE | Service 实体 + Discovery 闸 + 数据约束 |
| M2 | 8 | 2 BE + 1 FE | ITSM 集成 + UI 完整 + 体验顺滑 |
| M3 | 8 | 2 BE + 0.5 FE + 1 product | 智能告警 + CAB + 团队 |
| M4 | 4 | 2 BE (senior) | Edge 真离线（可选） |

**总人月估算**：
- 必做 M1-M3 (24 周) = 24 × 3 人 = **72 人周** ≈ **18 人月**
- M4 加上 = **86 人周** ≈ **21.5 人月**

按 3 BE + 1 FE + 1 product 团队规模：
- 必做：~6 个月日历时间
- 全做：~7 个月日历时间

---

## 关键风险

### R1. Service 实体迁移破坏 BIA
**缓解**：保留 bia_assessments 不删，service 用 FK 关联。所有 BIA 既有 UI 保持工作。

### R2. ServiceNow 集成因实例自定义碰壁
**缓解**：M2 PoC 先在 ServiceNow PDI（Personal Developer Instance）上跑通；客户实例做 mapping config 留给后续。

### R3. CAB 审批流堵塞自动治理
**缓解**：自动工单默认 risk='low'。decommission 这种高风险类型显式标 'high'。M3-W3 第一周做 dry-run 跑 1 周看队列长度。

### R4. Edge 离线写脑裂
**缓解**：M4 启动前先和客户确认场景。如果是只读为主 + 偶发写，4 周够。如果是双向并发写，需要 CRDT 或 vector clock，**翻倍预算**。

### R5. 团队不够 3 BE
**缓解**：M3 #8 (CAB) 和 #9 (Teams) 可以延到 M4，先把 M1-M2 做扎实。**不要并行三件 P0 否则集成会出问题。**

---

## 立即可做的 Quick Wins (本周)

不在 roadmap 关键路径上，但快速提升可信度：

1. **删 11 个 phantom UI** — 把 phase-3.10 占位组件改为「Coming soon」横幅，不要继续展示假数据。1 天。
2. **改 README** — Edge offline 描述改成准确的「failover with sync gate」而不是「offline-capable」。30 分钟。
3. **加 docs/ROADMAP.md** — 公开本路线图给客户/团队。1 天。
4. **跑 quality dashboard** — 看现在所有 asset 的 quality score 分布，找数据治理快速胜利。半天。
5. **fix 那批 alert rule "invalid operator" 的脏数据** — 启动日志里看到的 — 0.5 天。

合计 4 天可见进展，不影响主路线。
