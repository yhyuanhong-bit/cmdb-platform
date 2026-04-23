# Services — Business Service Entity

**Status**: Approved
**Author**: Claude (technical draft); business boundaries confirmed by project lead
**Reviewer**: Project lead
**Approved-by**: Project lead (2026-04-23)
**Date**: 2026-04-22 (drafted), 2026-04-23 (approved)
**Related Decision**: [docs/decisions/2026-04-22-day-0.md](../../../docs/decisions/2026-04-22-day-0.md) §D1
**Related ROADMAP item**: M1 Wave 2 (Business Service entity)

---

## 1. Background — 为什么要这个实体

**业务问题**

现在的 CMDB 没法回答 BCM (Business Continuity Management) 的核心问题：

1. "订单系统挂了影响哪些客户？" — 当前只能从告警看到具体 asset（server-01 CPU high），看不到这台 server 属于哪个业务功能
2. "DR 切换需要几台机？" — 没有 service→assets 的聚合视图
3. "这台 server 能不能退役？" — 无法判断它支撑了哪些对外服务
4. BIA 评估记录的 RTO/RPO 数字存了**但没验证过**：没有 service→assets 的关联，就没法核对「关键业务 service 的所有支撑资产是不是都能在 RTO 内恢复」

**当前 workaround**：`bia_assessments` 表用 `system_name VARCHAR(255)` 字符串记录业务系统名，但这个字符串**不是外键**、**不跟 assets 关联**。结果 BIA 是孤岛数据。

**不解决会发生什么**：
- CMDB 继续只能承担「资产盘点」角色，无法升级为「业务服务连续性管理」系统
- 评审报告给出的业务覆盖度 65% 里，`F 分` 的那一块永远卡住
- 告警治理做不到 service-level（M1 Wave 6 的 incident 聚合依赖此实体）
- Teams 的 owner 路由（M3 Wave 7）也依赖 service 作为通知聚合单位

**Out of scope**（本 spec 明确不解决）：
- Service 之间的依赖关系（service→service DAG）— 留给 Wave 6 的 incident 聚合 spec
- Service 级 SLO（uptime、latency）— 留给 M5 metrics 覆盖度 wave
- 自动 service discovery（从 tag / label 推断 service）— 留给后续
- 跨租户 shared service — 本 spec 假设所有 service 租户私有

---

## 2. ER 图 + 字段定义

### 主表

```sql
CREATE TABLE services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),

    -- 业务身份
    code            VARCHAR(64) NOT NULL,       -- e.g. "ORDER-API", "PAYMENT"
                                                -- 业务侧人类可读的引用 ID
                                                -- unique per tenant
                                                -- format: ^[A-Z][A-Z0-9_-]{1,63}$ (Q1 sign-off)
    name            VARCHAR(255) NOT NULL,      -- "订单系统", "支付网关"
    description     TEXT,

    -- 分级 — 复用 BIA tier 取值，不引入新枚举
    tier            VARCHAR(20) NOT NULL DEFAULT 'normal',
                    -- critical / important / normal / low
                    -- 同 bia_scoring_rules.tier_name

    -- 所有权 — 本 spec 阶段是字符串。M3 Wave 7 Teams 实体上线后
    -- 会加 owner_team_id UUID REFERENCES teams(id)，然后逐步迁移。
    owner_team      VARCHAR(100),

    -- 关联 BIA 评估 — 本 spec 设计成 0..1
    -- 一个 service 可以有对应的 BIA assessment（存 RTO/RPO 等正式值）
    -- 或者没有（新建 service 还没做 BIA 评估）
    -- bia_assessments 表保持不动，通过这个 FK 双向可达
    bia_assessment_id UUID REFERENCES bia_assessments(id),

    -- 生命周期
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
                    -- active / deprecated / decommissioned
                    -- deprecated = 不再接受新功能但还在运行
                    -- decommissioned = 已经停用，保留做历史审计

    -- 标签 — 复用 asset.tags 模式，支持跨 service 的自由聚合
    tags            TEXT[] DEFAULT '{}',

    -- 元数据
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      UUID REFERENCES users(id),   -- 审计谁创建了 service
    deleted_at      TIMESTAMPTZ,                 -- soft delete
    sync_version    BIGINT NOT NULL DEFAULT 0,   -- 同其他 CMDB 主表

    -- 约束
    UNIQUE (tenant_id, code),
    CHECK (status IN ('active', 'deprecated', 'decommissioned')),
    CHECK (tier IN ('critical', 'important', 'normal', 'low')),
    CHECK (code ~ '^[A-Z][A-Z0-9_-]{1,63}$')
);

CREATE INDEX idx_services_tenant ON services(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_tier ON services(tenant_id, tier) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_owner_team ON services(tenant_id, owner_team) WHERE owner_team IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_services_status ON services(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_sync_version ON services(tenant_id, sync_version);
```

### 关系表

```sql
CREATE TABLE service_assets (
    service_id   UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    asset_id     UUID NOT NULL REFERENCES assets(id)   ON DELETE CASCADE,
    tenant_id    UUID NOT NULL REFERENCES tenants(id), -- 冗余但必要，用于
                                                       -- tenantlint + 索引

    -- 角色 — 描述该 asset 在 service 中的功能位置
    role         VARCHAR(50) NOT NULL DEFAULT 'component',
                 -- primary       = 主节点（单点故障点）
                 -- replica       = 副本（冗余）
                 -- cache         = 缓存层
                 -- proxy         = 反代 / 负载均衡
                 -- storage       = 持久化存储
                 -- dependency    = 依赖服务（DNS / Auth）
                 -- component     = 其他组件（默认兜底值）

    -- 关键性 — service 失败该 asset 必须健康
    -- true  = critical path，asset offline 就整个 service 挂
    -- false = 冗余组件，单独挂不影响 service
    is_critical  BOOLEAN NOT NULL DEFAULT false,

    -- 元数据
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by   UUID REFERENCES users(id),

    PRIMARY KEY (service_id, asset_id),
    CHECK (role IN ('primary', 'replica', 'cache', 'proxy', 'storage', 'dependency', 'component'))
);

CREATE INDEX idx_service_assets_asset ON service_assets(asset_id);
CREATE INDEX idx_service_assets_tenant ON service_assets(tenant_id);
CREATE INDEX idx_service_assets_critical ON service_assets(service_id) WHERE is_critical = true;
```

### BIA 现有数据连接

不改 `bia_assessments` 表结构，**反向由 services 引用**：
- 新建 service 时可以选一个现有的 bia_assessment 关联
- 现有的 bia_assessments 通过 `system_code` 反向匹配迁移（见第 4 节）

### ER 关系图（文字）

```
tenants (1) ─── (N) services (N) ─── (N) service_assets (N) ─── (1) assets
                      │                                                 ↑
                      │ (0..1)                                          │ (N:M)
                      ↓                                                 │
               bia_assessments ──────────────── (FK 不直接存在，通过 service
                                                 关联推算每条 asset 的 BIA)

tenants (1) ─── (N) users (1) ─── (N) services.created_by
```

---

## 3. 边界问题 — 必须当场回答的设计选择

| Q | 选择 | 理由 |
|---|---|---|
| 跨租户共享？ | No | 多租户 SaaS 模型一致性；共享 service 场景未来用 service_sharing 表实现不改本表 |
| 软删还是硬删？ | **Soft** (deleted_at) | 审计需求；历史 service 即使 decommissioned 也要能查 |
| 状态字段需要 history？ | Yes | 通过 audit_events (module='service', action='update') 追踪 tier/status 变化 |
| 是否参与 sync (edge ↔ central)？ | **Yes** | sync_version 字段已加；service 是业务定义，edge 应能读（只读） |
| Edge 能否修改 service？ | **No** | central-wins 策略：service 是集中治理的业务定义，edge 看不是改 |
| RBAC 粒度？ | tenant-scoped + owner_team 未来可用于 team-level | 本 spec 阶段 tenant 级，M3 加 team 级 |
| 是否暴露给 webhook？ | **Yes** | 新事件 subjects: service.created / updated / deleted / status_changed |
| 是否参与 audit_events？ | **Yes** | module='service'; 所有写操作都写审计 |
| **跨租户唯一性约束？** | `code` unique per tenant（不全局） | 不同客户的 "ORDER-API" 是不同东西，允许重名 |
| **删除 service 时 service_assets 怎么办？** | CASCADE | 关系表行随 service 删除而删；asset 本身不受影响 |
| **删除 asset 时 service_assets 怎么办？** | CASCADE | asset 没了关系也没了；service 仍在，但 asset 数量-1 |
| **删除 bia_assessment 时 services.bia_assessment_id 怎么办？** | SET NULL | 保留 service，只断开评估关联 |
| **一个 asset 能属于多少 service？** | 不限（N:M） | 一台 db server 同时服务订单和支付，正常 |
| **一个 service 的 assets 能跨 location？** | Yes | 多站点 service 是常态 |
| **service 能否属于 service？** (service-of-services) | **No**（本 spec 阶段） | 推迟到 Wave 6 service→service 依赖 spec |

---

## 4. 数据迁移策略

### 现有数据映射

**bia_assessments.system_code** → **services.code**

现有的 `bia_assessments` 记录，每条都有一个 `system_code`（比如 "ORDER-SYS"）。这些字符串是业务系统的引用 ID，正好是 services.code 的语义。

**迁移三步**（均在同一个 PR 的 migration 内完成）：

1. **Phase A (本 PR migration 000062_business_services.up.sql)**
   - 创建 `services` 表 + `service_assets` 表（字段定义见第 2 节）
   - `services.bia_assessment_id` 初始全 NULL

2. **Phase B (同 migration 末尾，idempotent backfill)**
   - 对每条 `bia_assessments` 记录，`INSERT INTO services` 一条：
     ```sql
     INSERT INTO services (tenant_id, code, name, tier, bia_assessment_id, created_at)
     SELECT
         b.tenant_id,
         b.system_code,
         b.system_name,
         b.tier,
         b.id,
         b.created_at
     FROM bia_assessments b
     WHERE NOT EXISTS (
         SELECT 1 FROM services s
         WHERE s.tenant_id = b.tenant_id AND s.code = b.system_code
     )
     ON CONFLICT (tenant_id, code) DO NOTHING;
     ```
   - `service_assets` 留空 — 人工在 UI 挂 asset 到 service

3. **Phase C (本 PR，在 backfill 后)**
   - 反向更新 `bia_assessments` 加 `service_id` FK 指回对应 service：
     ```sql
     ALTER TABLE bia_assessments ADD COLUMN service_id UUID REFERENCES services(id);
     UPDATE bia_assessments b
     SET service_id = s.id
     FROM services s
     WHERE s.tenant_id = b.tenant_id AND s.code = b.system_code;
     ```

### 不删什么

- **不删 bia_assessments.system_code / system_name**（保留 6 个月做 backout）
- 应用层双写 `services.code` 和 `bia_assessments.system_code` 直到下个 milestone 确认无回归

### 回滚预案

Down migration:
```sql
ALTER TABLE bia_assessments DROP COLUMN IF EXISTS service_id;
DROP TABLE IF EXISTS service_assets;
DROP TABLE IF EXISTS services;
```

应用层回退：把 API handler / UI 回到读 `bia_assessments` 的旧路径。BIA 数据完整保留不丢。

### 生产切换 SQL

`ops/cutover/2026-MM-DD-services-backfill.sql`（本 migration 之后按需）：
- 针对每个生产租户再跑一遍 backfill check，确认 bia_assessments 数量 == services 数量

---

## 5. 测试 + 验收

### 单元测试（目标 >40% 覆盖率）

- `service_test.go`: CRUD 路径 + edge cases
- `service_assets_test.go`: add/remove/list + is_critical flag
- `service_health_test.go`: critical asset down → service degraded 逻辑
- 边界 case：
  - service with 0 assets（valid，显示 empty）
  - service with 100+ assets（性能）
  - 同一 asset 出现在多个 service（N:M 正确）
  - 删除 service 级联 service_assets（不影响 assets）

### 集成测试（`//go:build integration`）

- `service_tenant_isolation_integration_test.go`
  - 创建 tenant A 和 tenant B 各一个 service
  - 确认 tenant A 看不到 tenant B 的 service（即使知道 UUID）
  - 确认不能把 tenant A 的 asset 挂到 tenant B 的 service
- `service_bia_backfill_integration_test.go`
  - 在 bia_assessments 有数据的 DB 上跑 migration
  - 验证 services 表被正确 backfill
  - 验证 bia_assessments.service_id 反向 FK 正确

### E2E 测试（Playwright，≥1 spec）

新文件 `e2e/critical/service-centric.spec.ts`：
1. 创建 service "test-order-api"
2. 挂 3 个 asset 到 service（1 critical + 2 non-critical）
3. 点 critical asset 的详情页
4. 验证 "Belongs to: test-order-api" 链接存在
5. 从 service 详情页删除 1 个 asset
6. 验证 asset 本身还在（未被级联删）

### 手动验收清单

- [ ] migrate up from zero clean
- [ ] migrate down clean + 无 orphan 数据
- [ ] tenantlint 零告警
- [ ] go vet + race detector 通过
- [ ] OpenAPI spec 包含所有 7 个新 endpoint + 与 handler 一致
- [ ] CHANGELOG 更新
- [ ] docs/DATABASE_SCHEMA.md 加入 services / service_assets
- [ ] 现有 BIA 页面仍然工作（回归）
- [ ] 在 UI 能完成：创建 service → 挂 5 个 asset → 标 1 个 critical → 在 AssetDetail 看到反向关联

---

## 6. 性能 + 监控

### 预期负载

| 指标 | 估算 |
|---|---|
| 单租户 service 数量 | 10-500（大客户） |
| service 平均 asset 数量 | 3-50 |
| service_assets 总行数（大客户） | 最多 ~25000 行 |
| service health 查询 QPS | ~10/s（dashboard 刷新） |

### 索引策略

| Index | 服务哪些查询 |
|---|---|
| `idx_services_tenant` | list services for tenant |
| `idx_services_tier` | dashboard 按 tier 分组 |
| `idx_services_owner_team` | M3 Wave 7 的团队级视图 |
| `idx_service_assets_asset` | 反向：这个 asset 属于哪些 service（单 asset 详情页必用） |
| `idx_service_assets_critical` | health 检查只扫 critical asset |

### 新增 metrics

- `cmdb_services_total{tier=critical|important|normal|low, status=active|deprecated|decommissioned}` — gauge
- `cmdb_service_health_degraded_total{service_code}` — counter（每次 critical asset offline）
- `cmdb_service_assets_total{service_code}` — gauge
- `cmdb_service_api_duration_seconds{endpoint}` — histogram

---

## 7. Sign-off 决策

全部 5 个开放问题按推荐答案裁决（2026-04-23）。

### Q1: service.code 格式约束 ✅ 加

**决策**：`code` 加 regex CHECK：`^[A-Z][A-Z0-9_-]{1,63}$`

**含义**：
- 首字符必须大写英文字母
- 后续字符限于 `A-Z`, `0-9`, `_`, `-`
- 总长 2-64 字符（Kubernetes label 兼容）
- 中文、空格、`@#$` 等均被拒绝

**示例合法**：`ORDER-API`, `PAYMENT_GATEWAY`, `CUSTOMER_PORTAL`
**示例非法**：`order-api` (小写), `订单系统` (中文), `-api` (首字符), `a` (太短)

### Q2: decommissioned service 的 service_assets ✅ 保留

**决策**：选项 B — 保留作为历史

**含义**：
- `service.status = 'decommissioned'` 时，`service_assets` 行**不自动清空**
- UI 层显示为 read-only + 灰色
- 唯一清空方式：DELETE 整个 service（CASCADE 触发）
- 历史审计：下架后仍能查「当时涉及哪些服务器」

### Q3: service_assets.role 枚举 ✅ 保持 7 种

**决策**：保持精简 — primary / replica / cache / proxy / storage / dependency / component

**候选外但不加的映射**：
- `load_balancer` → 归入 `proxy`
- `database` → 归入 `primary` (主库) 或 `storage` (只读副本)
- `firewall` → 归入 `dependency`

**演进策略**：具体客户需要细分时再加 migration 扩展；用 tags 字段承载临时分类需求。

### Q4: BIA 重复 system_code 迁移 ✅ DISTINCT ON 取最新

**决策**：backfill 用 `DISTINCT ON (tenant_id, system_code) ORDER BY last_assessed DESC`

**SQL**：
```sql
INSERT INTO services (tenant_id, code, name, tier, bia_assessment_id, created_at)
SELECT DISTINCT ON (b.tenant_id, b.system_code)
    b.tenant_id, b.system_code, b.system_name, b.tier, b.id, b.created_at
FROM bia_assessments b
ORDER BY b.tenant_id, b.system_code, b.last_assessed DESC
ON CONFLICT (tenant_id, code) DO NOTHING;
```

**含义**：
- 同一 system_code 的多次评估版本中，最近一次绑定到 service
- 旧评估记录保留在 bia_assessments，不删
- `bia_assessments.service_id` FK 反向可达

### Q5: Edge 读 service ✅ Yes (read-only)

**决策**：services + service_assets 加入 SyncLayers 作为可同步实体

**含义**：
- Edge 节点能查询 service 定义（list、detail、health）
- Edge 无法创建 / 修改 / 删除 service（central-wins 策略）
- 解锁 edge-side service-centric 告警聚合（Wave 6 依赖）

**实现**：在 `internal/domain/sync/layers.go` `SyncLayers[0]` 加 `"services"` 和 `"service_assets"`


---

## 8. 决策权限

- 本 spec 的 Q1-Q5 需要 **项目负责人 sign-off** 才能 status → Approved
- 第 2 节的字段定义（除 Q1-Q5 提到的部分）已经走 decision log D1 决策，不再需要重新讨论
- Implementation 阶段的微调（字段顺序、索引名、SQL 风格）不需要再开 spec 讨论

---

## 9. 后续依赖链

本 spec 完成后解锁的工作：

- **Wave 4** (跨页导航)：AssetDetail 加 "Belongs to services" 链接需要 service_assets 表存在
- **Wave 6** (Incident 聚合)：incidents.service_id FK 需要 services 表存在
- **Wave 7** (CAB)：risk_level 判断需要 service.tier
- **Wave 7** (Teams)：通知路由需要 service.owner_team → service.owner_team_id
- **Wave 8** (Energy)：rack heatmap 未来可能按 service 上色
- **Wave 9** (Predictive)：service-level health score
