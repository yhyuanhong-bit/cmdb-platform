# [Entity Name] — Schema Spec

> Spec-first migration process. Fill out every section. PR review of this file
> happens BEFORE any migration / handler / UI code is written. Approved spec
> is the contract — significant deviations need spec-level revision PR.

**Status**: Draft | Reviewed | Approved | Implemented | Deprecated
**Author**: _name_
**Reviewer**: _name_
**Approved-by**: _name_
**Date**: YYYY-MM-DD
**Related Decision**: _link to docs/decisions/...md_
**Related ROADMAP item**: _e.g. M1 #1_

---

## 1. Background — 为什么要这个实体

**业务问题** — 1-2 段，用业务语言（不是技术语言）描述当前痛点：
- 谁现在受困？(persona)
- 痛点的具体 instance？(real-world example)
- 不解决会发生什么？(business impact)

**Out of scope** — 这个 spec 明确**不**解决什么。把容易混淆的相邻问题列出来。

---

## 2. ER 图 + 字段定义

### 主表

```sql
CREATE TABLE <table_name> (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    -- 业务字段
    code            VARCHAR(64) NOT NULL,    -- 业务侧唯一引用 ID
    name            VARCHAR(255) NOT NULL,
    -- ... fill in fields with comments explaining each
    -- 标准元字段
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,             -- soft delete
    UNIQUE (tenant_id, code),
    CHECK (...)                              -- enum constraints
);

CREATE INDEX idx_<table>_tenant ON <table_name>(tenant_id) WHERE deleted_at IS NULL;
```

### 关系表（如有）

```sql
CREATE TABLE <relation_name> (
    -- N:M 或 1:N 关联
);
```

### 字段决策表

每个字段的设计选择（不是 obvious 的字段省略）：

| 字段 | 类型 | 设计理由 |
|---|---|---|
| `code` | VARCHAR(64) | 业务侧引用 ID（人类可读，e.g. ORDER-API），unique per tenant |
| `tier` | VARCHAR(20) + CHECK | 复用 BIA tier 取值（critical/important/normal/low），不引入新枚举 |
| ... | | |

---

## 3. 边界问题 — 必须当场回答的设计选择

每个问题选 yes/no 并给理由。模糊回答 ("看情况") = spec 没准备好。

| Q | 选择 | 理由 |
|---|---|---|
| 跨租户共享？ | No | 多租户 SaaS 模型一致性 |
| 软删还是硬删？ | Soft | 审计需求 |
| 状态字段需要 history？ | Yes / No | _理由_ |
| 是否参与 sync (edge ↔ central)？ | Yes / No | _理由_ |
| RBAC 粒度？ | tenant / team / per-row | _理由_ |
| 是否暴露给 webhook？ | Yes / No | _理由_ |
| 是否参与 audit_events？ | Yes / No | _理由_ |
| **跨租户唯一性约束？** | code unique per tenant / global | _理由_ |
| **删除时其他表怎么办？** | CASCADE / RESTRICT / SET NULL | 每个外键单独决定 |

---

## 4. 数据迁移策略

### 现有数据如何处理？

如果这个实体替换/扩展现有数据：
- **不删旧表/旧字段**（保留 6 个月做 backout）
- 加新字段或表，应用层 dual-read，逐步切换
- 切换完成后**下个 milestone 再删**旧的

具体步骤（按时间序）：

1. **Phase A (本 PR)**: 创建新表 + 加新 FK 字段（NULL 允许）
2. **Phase B (本 PR)**: backfill 脚本（cutover SQL 在 `ops/cutover/`）
3. **Phase C (next PR)**: 应用层切换到新数据
4. **Phase D (M+1 milestone)**: 删除旧字段/旧表

### 回滚预案

每个 phase 必须有 rollback：
- 如何 down migration
- 如何应用层回退
- 数据丢失风险评估

---

## 5. 测试 + 验收

### 单元测试 (>40% 覆盖率)

- 主要 mutation 路径
- 边界 case (空 / null / max len)
- 错误路径

### 集成测试 (>20% 覆盖率)

- 跨表查询
- Tenant isolation 强制（必须有跨 tenant attack test，参考 auth_service_tenant_isolation_test.go）
- soft delete 行为

### E2E 测试 (>= 1 spec)

- 用户能完成典型 workflow

### 手动验收清单

完成 implementation 后人工检验：

- [ ] migrate up from zero 通过
- [ ] migrate down 通过（数据 rollback 可行）
- [ ] tenantlint 零告警
- [ ] go vet + race detector 通过
- [ ] OpenAPI spec 包含所有 endpoint 且与 handler 一致
- [ ] CHANGELOG 更新
- [ ] docs/DATABASE_SCHEMA.md 更新

---

## 6. 性能 + 监控

### 预期负载

- 写 QPS：~_N_/s
- 读 QPS：~_N_/s
- 单租户最大行数：~_N_

### 索引策略

- 列出每个 index + 它服务的查询模式

### 新增 metrics

- `cmdb_<entity>_<event>_total` (counter)
- `cmdb_<entity>_<operation>_duration_seconds` (histogram)

---

## 7. 开放问题 (sign-off 前必须解决)

- [ ] _问题 1_
- [ ] _问题 2_

---

## 8. 决策权限

- **Spec 级决策**（影响后续 milestone 的）：项目负责人 sign-off
- **Implementation 级决策**（不影响其他模块的）：implementer 自己定
- 边界模糊时升级给项目负责人
