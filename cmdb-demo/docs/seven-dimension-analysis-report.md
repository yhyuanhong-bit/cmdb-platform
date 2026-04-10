# CMDB Platform 七维度深度分析报告

> 日期: 2026-04-10
> 分析方式: 7 个专业 Agent 并行扫描（Security Engineer / Backend Architect / Software Architect / API Tester / Code Reviewer / SRE / Database Optimizer）

---

## 总览评分

| 维度 | 评分 | 关键风险 |
|------|------|---------|
| 1. 安全审计 | **D** (2 Critical, 6 High) | JWT 默认密钥、凭证加密全零密钥、跨租户泄露 |
| 2. 性能分析 | **C** (3 Critical, 9 High) | 导入 N+1 查询、无事务、缺索引、无应用缓存 |
| 3. 架构评审 | **C+** | impl.go 混合业务逻辑、57 处绕过 Service 层的原始 SQL |
| 4. API 完整性 | **B-** (96/96 spec 实现) | 39 个未文档化端点、4 个前端幽灵端点、路径不匹配 |
| 5. 测试覆盖 | **F** (Go 0%, 前端 0%) | 零测试、手写 JWT 无测试、RBAC 无测试 |
| 6. 生产就绪度 | **C** (2 Ready, 4 Needs Work, 1 Missing) | 无 CI/CD、浅层健康检查、无 HTTP 超时 |
| 7. 数据库优化 | **D+** (3 Critical, 7 High) | 18 个缺失 FK 索引、跨租户查询泄露、无审计保留策略 |

---

## 1. 安全审计

### Critical (立即修复)

| # | 问题 | 文件 | 修复 |
|---|------|------|------|
| S1 | JWT 密钥默认 `dev-secret-change-me` | `config/config.go:43` | 启动时校验密钥长度 ≥ 32，生产环境不允许默认值 |
| S2 | 凭证加密密钥全零 (`"0"*64`) | `ingestion-engine/app/config.py:13` | 移除默认值，启动时强制要求 `INGESTION_CREDENTIAL_ENCRYPTION_KEY` |

### High

| # | 问题 | 影响 |
|---|------|------|
| S3 | Sensor 端点无 tenant_id 过滤 | 租户 A 可删改租户 B 的传感器 |
| S4 | RBAC 对未映射路径默认放行 | `/energy/*`, `/sensors/*`, `/activity-feed` 绕过权限检查 |
| S5 | SSH Collector 禁用 host key 验证 | 中间人攻击风险 |
| S6 | CORS 通配符 `*` | 任意网站可调用 API |
| S7 | 缺少安全响应头 | 无 HSTS、X-Frame-Options、CSP |
| S8 | CIDR 扫描无范围限制 | 可扫描云元数据端点 `169.254.169.254` |

### Medium (6 项) / Low (4 项)
- JWT 允许不过期 token、WS query token 残留、无密码复杂度、无登录限流、MCP 无 TLS、SheetJS CVE

---

## 2. 性能分析

### Critical

| # | 问题 | 文件 | 影响 |
|---|------|------|------|
| P1 | 导入循环 N+1 查询（4 次/条目） | `impl.go:1706-1782` | 500 行 Excel = 2000 次 DB 往返 |
| P2 | 导入无事务，中途失败留脏数据 | `impl.go:1770-1781` | 数据不一致 |
| P3 | Python Celery 嵌套事件循环 | `import_task.py:53-54` | 每 50 行创建/销毁一次事件循环 |

### High

| # | 问题 | 修复建议 |
|---|------|---------|
| P4 | 每个分页端点双查询 (List + Count) | 用 `count(*) OVER()` 窗口函数合并 |
| P5 | `assets.ip_address` 无索引 | 添加 `(tenant_id, ip_address)` 索引 |
| P6 | `incidents` 表零索引 | 添加 `(tenant_id, status, started_at DESC)` |
| P7 | Metrics 查询绕过连续聚合 | >24h 用 `metrics_1hour`，1-24h 用 `metrics_5min` |
| P8 | Redis 仅用于 Auth，无应用缓存 | Dashboard stats 缓存 30-60s TTL |
| P9 | xlsx (1MB+) 在主 bundle | 动态 import |
| P10 | Location 页面瀑布请求 | 服务端聚合端点 |
| P11 | Celery 每任务新建 DB 连接池 | 模块级共享池 |
| P12 | Python 批处理逐条串行 | `asyncio.gather()` + Semaphore |

---

## 3. 架构评审

### 主要问题

| 问题 | 严重度 | 详情 |
|------|--------|------|
| **impl.go 混合关注点** | High | 2,893 行，含原始 SQL、业务规则、ITSM 工作流 |
| **57 处原始 SQL 在 API 层** | High | `custom_endpoints.go`, `energy_endpoints.go`, `phase3_*`, `phase4_*` 全部绕过 Service 层 |
| **Domain Service 过度贫血** | Medium | 大部分 Service 是单行 dbgen 委托，业务逻辑在 Handler |
| **NewAPIServer 16 个参数** | Medium | 无依赖注入容器 |
| **WS Hub 单节点瓶颈** | Medium | 内存 map，不支持多实例广播 |
| **响应格式不一致** | Medium | 生成端点用 `response.OK()`，自定义端点用 `c.JSON(gin.H{})` |
| **`math/rand` 生成工单号** | Low | 非加密随机、可重复 |

### 正面发现
- 事件总线接口抽象良好 (`eventbus.Bus`)
- 前后端通过 REST 解耦，无实现细节泄露
- 域包之间无循环依赖
- TimescaleDB 连续聚合设计良好

---

## 4. API 完整性

### 整体统计

| 指标 | 数值 |
|------|------|
| OpenAPI 定义端点 | 96 |
| Go 实现率 | 96/96 (100%) |
| 前端调用率 | 93/96 (96.9%) |
| **未文档化自定义端点** | **39** |
| **前端幽灵端点 (无后端)** | **4** |
| 响应格式不匹配 | 4 |
| 缺失错误响应声明 | 17+ |

### P0 — 运行时 404/405

| 前端调用 | 实际后端 | 后果 |
|---------|---------|------|
| `GET /quality/scores/{id}` | `GET /quality/history/{id}` | 路径不匹配 → 404 |
| `POST /prediction/results` | 不存在 | 404 |
| `POST /monitoring/metrics` | 仅 GET | 405 |
| `/ingestion/*` (14 个端点) | 全部不存在 | 整个采集 UI 无法工作 |

### P1 — 39 个未文档化端点
能源模块 (3)、传感器 (5)、拓扑扩展 (7)、盘点扩展 (7)、维护评论 (2)、预测扩展 (4)、会话/密码 (2) 等全部缺少 OpenAPI 文档。

---

## 5. 测试覆盖

| 组件 | 测试文件 | 用例数 | 覆盖率 |
|------|---------|--------|--------|
| **Go Backend** | 0 | 0 | **0%** |
| **React Frontend** | 0 | 0 | **0%** |
| **Python Ingestion** | 9 | 38 | ~35-40% |

### 最优先编写的 10 个测试

| # | 测试 | 原因 |
|---|------|------|
| 1 | `auth_test.go` — JWT 验证 | 手写 JWT，bug = auth bypass |
| 2 | `rbac_test.go` — 权限检查 | 授权门，纯函数易测 |
| 3 | `statemachine_test.go` — 工单状态机 | 纯函数，表驱动测试 |
| 4 | `maintenance/service_test.go` — 软删除守卫 | 数据完整性 |
| 5 | `inventory/service_test.go` — 状态守卫 | 数据完整性 |
| 6 | `auth_service_test.go` — 登录/刷新 | bcrypt、token 轮换 |
| 7 | `authStore.test.ts` — 前端 Auth | 驱动整个前端 |
| 8 | `client.test.ts` — 401 自动刷新 | 微妙的重试逻辑 |
| 9 | `AuthGuard.test.tsx` — 路由保护 | 安全门 |
| 10 | `test_processor.py` — 管道编排 | 数据正确性核心 |

---

## 6. 生产就绪度

| 领域 | 评级 | 关键问题 |
|------|------|---------|
| 环境配置 | NEEDS WORK | JWT 默认密钥可在生产启动、无 HTTP 超时 |
| 日志 & 可观测性 | **READY** | zap + Prometheus + OTel + Jaeger + Grafana |
| 错误处理 & 韧性 | NEEDS WORK | 无断路器、Redis 故障导致 RBAC 全拒 |
| 健康检查 | NEEDS WORK | `/healthz` 不检查 DB/Redis/NATS |
| 数据库 | **READY** | 22 个 migration、up/down 完整 |
| 部署 | READY (缺 CI/CD) | Docker Compose 完整，无 GitHub Actions |
| 限流 & DoS | NEEDS WORK | 仅 Nginx 层，应用层无限流 |

### 生产前 Top 5 修复

1. **HTTP 超时**: `ReadTimeout: 30s, WriteTimeout: 60s` — 防 slowloris
2. **JWT 密钥校验**: 非 dev 模式拒绝默认密钥
3. **深度健康检查**: `/healthz` ping DB + Redis + NATS，新增 `/readyz`
4. **应用层限流**: `github.com/ulule/limiter` 或 Redis token bucket
5. **CI/CD 流水线**: `go test -race` + Docker build + 部署门禁

---

## 7. 数据库优化

### Critical

| # | 问题 | 影响 |
|---|------|------|
| D1 | `metrics.tenant_id` 允许 NULL | 租户隔离在最高量表失效 |
| D2 | 18 个外键列缺索引 | CASCADE DELETE 全表扫描 |
| D3 | 18 个 GET/DELETE 查询缺 `tenant_id` 过滤 | 跨租户数据泄露 |

### High

| # | 问题 | 修复 |
|---|------|------|
| D4 | `OR` 可选过滤阻止索引使用 | 添加复合索引或动态 SQL |
| D5 | `ILIKE '%...'` 全表扫描 | 添加 `pg_trgm` GIN 索引 |
| D6 | `audit_events` 无保留策略 | 添加 90 天 retention 或分区 |
| D7 | `webhook_deliveries` 无保留策略 | 添加索引 + 清理任务 |
| D8 | `ListPredictionsByTenant` 无 ORDER BY | 分页不确定性 |
| D9 | `ListWebhooksByEvent` 跨租户 | 添加 `tenant_id` 过滤 |
| D10 | `assets` 表无软删除 | 添加 `deleted_at` 列 |

### 建议 Migration (000023)

```sql
-- 18 个缺失 FK 索引
CREATE INDEX CONCURRENTLY idx_work_order_logs_order_id ON work_order_logs(order_id);
CREATE INDEX CONCURRENTLY idx_work_orders_location_id ON work_orders(location_id);
CREATE INDEX CONCURRENTLY idx_alert_events_rule_id ON alert_events(rule_id);
CREATE INDEX CONCURRENTLY idx_incidents_tenant_id ON incidents(tenant_id);
-- ... (共 18 个)

-- 文本搜索
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX CONCURRENTLY idx_assets_name_trgm ON assets USING GIN(name gin_trgm_ops);

-- UNIQUE 约束
ALTER TABLE departments ADD CONSTRAINT uq_dept_tenant_slug UNIQUE(tenant_id, slug);
ALTER TABLE locations ADD CONSTRAINT uq_loc_tenant_slug_level UNIQUE(tenant_id, slug, level);

-- 资产软删除
ALTER TABLE assets ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- NOT NULL 修复
ALTER TABLE metrics ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE metrics ALTER COLUMN asset_id SET NOT NULL;
```

---

## 跨维度关联发现

以下问题在多个维度中重复出现，是系统性风险：

| 系统性问题 | 涉及维度 | 影响 |
|-----------|---------|------|
| **跨租户数据泄露** | 安全 + 数据库 + 架构 | GET/DELETE 查询缺 tenant_id、Sensor 端点、Webhook 事件分发 |
| **缺少测试** | 测试 + 安全 + 生产 | 手写 JWT/RBAC 零测试，无 CI/CD 门禁 |
| **原始 SQL 在 API 层** | 架构 + 性能 + 安全 | 57 处绕过 Service 层，无事务，无租户校验 |
| **配置硬编码** | 安全 + 生产 + 架构 | JWT 密钥、加密密钥、连接池参数、设备寿命 |

---

## 修复优先级路线图

### Phase 1: 安全加固 (1-2 天)
- [ ] JWT 密钥启动校验 + 移除默认值
- [ ] 凭证加密密钥移除全零默认
- [ ] 所有 GET/DELETE SQL 添加 `AND tenant_id = $N`
- [ ] Sensor 端点添加 tenant_id
- [ ] RBAC 未映射路径改为拒绝
- [ ] CORS 改为前端域名白名单
- [ ] 添加安全响应头中间件

### Phase 2: 数据库加固 (1 天)
- [ ] Migration 000023: 18 个 FK 索引 + pg_trgm + NOT NULL + UNIQUE
- [ ] 修复 ListWebhooksByEvent 跨租户
- [ ] 添加 audit_events 保留策略

### Phase 3: 测试基础 (2-3 天)
- [ ] Go: auth_test.go + rbac_test.go + statemachine_test.go
- [ ] Frontend: 安装 vitest + 编写 authStore/client/AuthGuard 测试
- [ ] CI/CD: GitHub Actions (go test + tsc + build)

### Phase 4: 性能优化 (2-3 天)
- [ ] 导入改为批量 INSERT + 事务
- [ ] HTTP 超时 + 深度健康检查
- [ ] Dashboard stats Redis 缓存
- [ ] Metrics 查询改用连续聚合

### Phase 5: 架构改进 (持续)
- [ ] 原始 SQL 迁移到 Service 层
- [ ] 39 个自定义端点添加到 OpenAPI spec
- [ ] 修复 4 个前端幽灵端点
- [ ] 统一响应格式
