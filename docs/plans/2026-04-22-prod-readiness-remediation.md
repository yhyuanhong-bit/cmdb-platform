# 生产就绪修复计划 — 三步走

> 基于 2026-04-22 生产就绪评审结果，1 BLOCK + 14 WARN
> 状态: CONDITIONAL — 修复第一波后可上线

---

## 第一波：上线前必修（预估 3-5 天）

### 1.1 [BLOCK] tenantlint 归零（180 个非测试告警）

按类型分批处理，总共 180 个告警：

#### 批次 A：加 allow 注释（56 个，~0.5 天）

无需改逻辑，加 `//tenantlint:allow-direct-pool — <理由>` 即可。

| 文件 | 告警数 | 理由 |
|---|---|---|
| `cmd/audit-archive/archive.go` | 5 | 运维工具，跨租户归档是设计意图 |
| `cmd/backfill-integration-secrets/main.go` | 7 | 一次性迁移脚本，跨租户回填 |
| `cmd/rotate-integration-secrets/main.go` | 7 | 密钥轮换工具，跨租户操作 |
| `cmd/server/main.go` | 9 | 启动初始化（seed 数据、schema 检查），无请求上下文 |
| `internal/platform/database/tenant.go` | 3 | TenantScoped 自身实现 |
| `internal/domain/workflows/sla.go` | 3 | 跨租户定时调度器，按 tenant 循环处理 |
| `internal/domain/workflows/auto_workorders_governance.go` | 5 | 同上，跨租户定时扫描 |
| `internal/domain/workflows/auto_workorders_security.go` | 2 | 同上 |
| `internal/domain/workflows/auto_workorders_warranty.go` | 3 | 同上 |
| `internal/domain/workflows/audit_partitions.go` | 1 | 分区维护，跨租户 |
| `internal/domain/workflows/cleanup.go` | 2 | 过期数据清理，跨租户 |
| `internal/domain/workflows/divergence.go` | 1 | 漂移检测调度器 |
| `internal/domain/workflows/metrics.go` | 1 | 指标��合，跨租户 |
| `internal/domain/workflows/notifications.go` | 7 | 通知调度器，已有 tenant_id WHERE 子句，但用直接 pool |
| **小计** | **56** | |

**验收**: `go run ./tools/tenantlint/cmd/tenantlint ./... 2>&1 | grep -v _test.go | grep "allow-direct-pool" | wc -l` = 56

#### 批次 B：API handler 修复（85 个，~2 天）

所有 `internal/api/impl_*.go` 中的直接 pool 调用改为使用 `database.Scope(ctx, pool)` 或通过已 scoped 的 `dbgen.New(scopedConn)` 查询。

| 文件 | 告警数 | 修复模式 |
|---|---|---|
| `impl_prediction_upgrades.go` | 16 | 每个 handler 开头加 `sc := database.Scope(ctx, s.pool)` 替换所有 `s.pool.` 调用 |
| `impl_sync.go` | 8 | 同上；已有部分用 `pgx.Identifier` 的正确写法 |
| `impl_inventory.go` | 7 | 同上 |
| `impl_capacity_planning.go` | 7 | 同上 |
| `impl_location_detect.go` | 7 | 同上 |
| `impl_asset_lifecycle.go` | 6 | 同上 |
| `impl_energy.go` | 5 | 同上 |
| `impl_inventory_items.go` | 4 | 同上 |
| `impl_notifications.go` | 4 | 同上 |
| `impl_qr.go` | 4 | 同上 |
| `impl_topology.go` | 3 | 同上 |
| `impl_activity.go` | 2 | 同上 |
| `impl_inventory_more.go` | 2 | 同上 |
| `impl_monitoring.go` | 2 | 同上 |
| `impl_rack_stats.go` | 2 | 同上 |
| `impl_assets.go` | 2 | 同上 |
| `impl_alerts.go` | 1 | 同上 |
| `impl_incidents.go` | 1 | 同上 |
| `impl_sensors.go` | 1 | 同上 |
| `impl_system.go` | 1 | 同上 |
| **小计** | **85** | |

**统一修复模式**:
```go
// 修复前
rows, err := s.pool.Query(ctx, "SELECT ... WHERE tenant_id = $1", tenantID)

// 修复后
sc := database.Scope(ctx, s.pool)
rows, err := sc.Query(ctx, "SELECT ... WHERE tenant_id = $1", tenantID)
```

**验收**: `go run ./tools/tenantlint/cmd/tenantlint ./internal/api/... 2>&1 | grep -v _test.go | wc -l` = 0

#### 批次 C：domain 业务代码修复（39 个，~1.5 天）

��高优先级的 3 个安全修复 + 36 个一般修复。

**P0 安全修复（identity/auth_service.go — 3 个）**:

| 行号 | 当前 SQL | 修复 |
|---|---|---|
| 218 | `UPDATE users SET last_login_at=now(), last_login_ip=$1 WHERE id=$2` | 加 `AND tenant_id=$3`，从 context 取 tenant_id |
| 245 | `UPDATE users SET password_hash=$1, password_changed_at=now() WHERE id=$2` | 加 `AND tenant_id=$3` |
| 433 | `SELECT password_changed_at FROM users WHERE id=$1` | 加 `AND tenant_id=$2` |

**其余 domain 修复**:

| 文件 | 告警数 | 修复模式 |
|---|---|---|
| `domain/sync/agent.go` | 12 | sync agent 有自己的 tenant config，改用 `database.Scope()` |
| `domain/topology/service.go` | 6 | `incrementSyncVersion` 等改用 scoped pool |
| `domain/location_detect/detector.go` | 5 | 已手动带 tenant_id 参数，改用 `database.Scope()` 增加防护网 |
| `domain/location_detect/service.go` | 5 | 同上 |
| `domain/monitoring/evaluator.go` | 2 | 改用 scoped pool |
| `domain/sync/service.go` | 2 | entityType 来自编译时硬编码表，安全，但仍应改用 scoped pool |
| `domain/asset/service.go` | 1 | `incrementSyncVersion` 改用 `pgx.Identifier` + scoped pool |
| `domain/dashboard/service.go` | 1 | 改用 scoped pool |
| `domain/maintenance/service.go` | 1 | 同上 |
| `domain/quality/service.go` | 1 | 同上 |
| **小计** | **39** | |

**验收**: `go run ./tools/tenantlint/cmd/tenantlint ./... 2>&1 | grep -v _test.go | wc -l` = 0

### 1.2 CI branch trigger 修复（5 分钟）

**文件**: `.github/workflows/ci.yml`

```yaml
# 修复前 (line 5)
branches: [main, feat/*]
# 修复后
branches: [main, master, feat/*]

# 修复前 (line 7)
branches: [main]
# 修复后
branches: [main, master]
```

**验收**: push 到 master，确认 CI 触发。

### 1.3 第一波完成标准

- [ ] `go run ./tools/tenantlint/cmd/tenantlint ./... 2>&1 | grep -v _test.go` 输出为空（零告警）
- [ ] `go test ./... -race` 全绿
- [ ] CI 在 master push 时触发
- [ ] PR 经 code review 后 merge

---

## 第二波：上线后 7 天内（安全加固 + 可用性）

### 2.1 auth middleware 改 fail-closed（0.5 天）

**文件**: `internal/middleware/auth.go:103-104, 119-120`

**当前行为**: Redis 宕机时 log warn → `c.Next()`（放行）

**修复方案**: 添加环境变量控制策略，默认 fail-closed:

```go
// 新增配置
// AUTH_REDIS_FAIL_POLICY=open|closed (default: closed)

// 修复后逻辑
if err != nil {
    zap.L().Error("redis revocation check failed", zap.Error(err))
    if os.Getenv("AUTH_REDIS_FAIL_POLICY") != "open" {
        response.Error(c, http.StatusServiceUnavailable, "auth service temporarily unavailable")
        c.Abort()
        return
    }
    // legacy: fail-open for explicit opt-in
}
```

**验收**: 停掉 Redis → 请求返回 503（非 200）

### 2.2 readyz 加 NATS 检查（0.5 天）

**文件**: `internal/api/health.go`

**当前**: `HealthHandler` struct 只有 `pool *pgxpool.Pool` + `rdb *redis.Client`

**修复**:
1. struct 加 `nc *nats.Conn` 字段
2. `NewHealthHandler` 构造函数加 `nc` 参数
3. `Readiness()` 方法加 NATS 检查:
```go
if h.nc != nil {
    nStart := time.Now()
    if h.nc.Status() != nats.CONNECTED {
        checks["nats"] = gin.H{"status": "down"}
        healthy = false
    } else {
        checks["nats"] = gin.H{"status": "up", "latency_ms": time.Since(nStart).Milliseconds()}
    }
}
```
4. `cmd/server/main.go` 中传入 NATS conn

**验收**: 停 NATS → `GET /readyz` 返回 503 + `nats: down`

### 2.3 ingestion-engine 删除零 key 默认值（0.5 天）

**文件**: `ingestion-engine/app/config.py:72`

**当前**: `credential_encryption_key = "0" * 64` (dev mode fallback)

**修复**:
```python
# 修复后
if deploy_mode == "development":
    logger.warning("DEV MODE: using insecure zero key for credential encryption")
    credential_encryption_key = "0" * 64
else:
    credential_encryption_key = os.environ["INGESTION_CREDENTIAL_ENCRYPTION_KEY"]
    # 无 fallback，缺 key 直接 KeyError 崩溃
```

再加一层防护 — 启动时校验非 dev 环境的 key 不是零 key:
```python
if deploy_mode != "development" and credential_encryption_key == "0" * 64:
    raise ValueError("Production credential key must not be the zero key")
```

**验收**: `INGESTION_DEPLOY_MODE=production` 不设 key → 启动崩溃

### 2.4 DB MaxConns 改为环境变量（0.5 天）

**文件**: `internal/platform/database/postgres.go:78-79`

**当前**: 硬编码 `cfg.MaxConns = 50; cfg.MinConns = 5`

**修复**:
```go
cfg.MaxConns = int32(envIntOr("DB_MAX_CONNS", 50))
cfg.MinConns = int32(envIntOr("DB_MIN_CONNS", 5))
```

同步更新:
- `deploy/docker-compose.yml` 加 `DB_MAX_CONNS=50`
- `deploy/.env.example` 加注释说明
- `docs/` 部署文档更新

**验收**: 设 `DB_MAX_CONNS=10` → 启动日志显示 pool max=10

### 2.5 第二波完成标准

- [ ] Redis 故障时 API 返回 503（不放行已吊销 token）
- [ ] NATS 故障时 readyz 返回 503
- [ ] 非 dev 环境缺加密 key 启动失败
- [ ] DB 连接池可配置
- [ ] 全部 `go test ./... -race` 绿
- [ ] 相关测试补齐（至少 auth middleware fail-closed 的单测）

---

## 第三波：上线后 14-30 天（质量 + 性能 + 可观测）

### 3.1 测试覆盖率提升（14 天，~5 天工作量）

#### 3.1.1 安装覆盖率工具

```bash
# cmdb-demo
cd cmdb-demo && npm install -D @vitest/coverage-v8
# 更新 vitest.config.ts 加 coverage 配置

# ingestion-engine
# pyproject.toml [tool.pytest.ini_options] 加 --cov=app --cov-report=term-missing
```

#### 3.1.2 优先补测试的包（按风险排序）

| 包 | 当前覆盖率 | 目标 | 测试重点 |
|---|---|---|---|
| `domain/identity` | 5.2% | 40%+ | auth_service 的 login/password/session 流程 |
| `domain/workflows` | 11.9% | 40%+ | auto_workorders 触发条件、SLA 计算、通知发送 |
| `eventbus` | 10.3% | 40%+ | publish/subscribe 生命周期、reconnect |
| `domain/audit` | 20.7% | 40%+ | 审计事件记录完整性 |
| `domain/integration` | 24.9% | 40%+ | 外部集成凭证加解密 |
| `domain/maintenance` | 29.4% | 40%+ | 工单状态机转换 |

#### 3.1.3 修复失败测试

- `ingestion-engine/tests/test_ssh_collector.py::test_ssh_supported_fields` — 断言中加 `sub_type` 字段

#### 3.1.4 补 E2E 场景

- `e2e/critical/credential.spec.ts` ��� 凭证 CRUD + 加密验证
- `e2e/critical/topology.spec.ts` — 拓扑图加载 + 依赖关系查看

**验收**:
- 所有 domain/ 包 >= 40% 覆盖率
- vitest --coverage 能出报告
- pytest --cov 能出报告
- 0 个测试失败

### 3.2 SQL 安全加固 — pgx.Identifier（14 天，~1 天）

将所有 `fmt.Sprintf` 拼接表名的代码改用 `pgx.Identifier{}.Sanitize()`:

| 文件 | 行号 | 当前写法 |
|---|---|---|
| `domain/sync/service.go` | 102, 171 | `fmt.Sprintf("SELECT ... FROM %s ...", entityType)` |
| `domain/sync/agent.go` | 471, 492 | `fmt.Sprintf("UPDATE %s ...", env.EntityType)` |
| `domain/asset/service.go` | 274 | `fmt.Sprintf("UPDATE %s ...", table)` |
| `domain/maintenance/service.go` | 482 | 同上 |
| `domain/topology/service.go` | 356 | 同上 |

**修复模式** (参照 `impl_sync.go:423` 已有正确写法):
```go
// 修复前
fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", table)
// 修复后
tableIdent := pgx.Identifier{table}.Sanitize()
fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", tableIdent)
```

**验收**: `grep -rn 'fmt.Sprintf.*"%s.*WHERE' internal/domain/ --include="*.go"` 结果为零

### 3.3 Python 依赖锁定（14 天，~0.5 天）

```bash
cd ingestion-engine
pip freeze > requirements.lock
# 或迁移到 uv: uv pip compile pyproject.toml -o requirements.lock
```

CI 中加:
```yaml
- name: Check dependency lock freshness
  run: pip install -r requirements.lock --dry-run --no-deps
```

### 3.4 SLO 定义 + burn-rate alert（21 天，~2 天）

基于已有指标:

| SLO | SLI 指标 | 目标 | 窗口 |
|---|---|---|---|
| 可用性 | `http_requests_total{status!~"5.."}` / `http_requests_total` | 99.9% | 30 天 |
| 延迟 | `histogram_quantile(0.99, http_request_duration_seconds_bucket)` | < 1s | 30 天 |
| 数据新鲜度 | `sync_envelope_applied_total` lag | < 5 min | 30 天 |

**交付物**:
- `deploy/prometheus/rules/slo.yml` — recording rules + burn-rate alerts
- `deploy/grafana/dashboards/slo-overview.json` — SLO dashboard
- `docs/slo-definition.md` — SLO 文档

### 3.5 workflows 事件链加事务（21 天，~1.5 天）

**文件**: `domain/workflows/auto_workorders_governance.go` 等

已有 `pool.Begin(ctx)` 模式但部分缺 `tx.Commit()`。逐文件审查:
- `auto_workorders_governance.go` — 确认每个 tx 都有 Commit
- `notifications.go` — 多步写操作包裹事务
- `sla.go` — breach 标记 + 通知发送包裹事务

**验收**: 每个 `pool.Begin` 都有对应的 `tx.Commit`，`defer tx.Rollback` 在最前面

### 3.6 前端 bundle 优化（30 天，~1 天）

**文件**: `cmdb-demo/vite.config.ts`

添加 chunk 拆分:
```typescript
build: {
  rollupOptions: {
    output: {
      manualChunks: {
        'vendor-react': ['react', 'react-dom', 'react-router-dom'],
        'vendor-ui': ['@headlessui/react', 'lucide-react'],
        'vendor-charts': ['recharts'],
        'elk': ['elkjs'],        // 1.4MB → 按需加载
        'xlsx': ['@e965/xlsx'],  // 493KB → 按需加载
      }
    }
  }
}
```

页面级动态 import:
```typescript
// DataCenter3D.tsx — elk 仅在 3D 页面使用
const ELK = lazy(() => import('elkjs'));

// HighSpeedInventory.tsx — xlsx 仅在导入导出时使用
const handleExport = async () => {
  const XLSX = await import('@e965/xlsx');
  // ...
};
```

**验收**:
- `npx vite build` 无 >500KB chunk 警告
- elk/xlsx 不在首屏加载

### 3.7 main.go 拆分（30 天，~1 天）

**当前**: `cmd/server/main.go` 692 行，30 天改 86 次

**拆分方案**:
```
cmd/server/
├── main.go           (~80 行) — 入口，调用 bootstrap + run
├── bootstrap.go      (~200 行) — DB/NATS/Redis 初始化、seed 数据
├── routes.go         (~200 行) — 路由注册 + middleware 装配
├── seed_password.go   (已有)
└── jwt_validation.go  (已有)
```

**验收**: `wc -l cmd/server/*.go` 每个文件 < 300 行

### 3.8 第三波完成标准

- [ ] 所有 domain/ 包覆盖率 >= 40%
- [ ] 覆盖率报告在 CI 中自动生成
- [ ] 0 个 `fmt.Sprintf` 直接拼接表名
- [ ] Python 依赖有 lockfile
- [ ] SLO dashboard 可用，burn-rate alert 配置完成
- [ ] workflows 事务完整性验证通过
- [ ] 首屏 JS bundle < 300KB gzip
- [ ] main.go 拆分完成，单文件 < 300 行

---

## 时间线总览

```
Day 0-5   ████████████████████  第一波：tenantlint 归零 + CI 修复
Day 5     ▲ GO/NO-GO 决策点 — tenantlint 零告警即可上线
Day 5-12  ███���████████████      第二波：auth fail-closed + readyz NATS + MaxConns
Day 12-19 ████████████████      第三波前半：测试覆盖率 + pgx.Identifier + lockfile
Day 19-26 ████████████████      第三波中：SLO + 事务加固
Day 26-35 ████████████████      第三波后半：bundle 优化 + main.go 拆分
```

## 并行化建议

第一波的三个批次可以并行执行:
- Agent 1: 批次 A（56 个 allow 注释）
- Agent 2: 批次 B（85 个 API handler）
- Agent 3: 批次 C（39 个 domain 代码）

第二波的 4 个子项互相独立，也可并行。

第三波中，3.1(测试) 和 3.2(pgx.Identifier) 和 3.6(bundle) 互相独立，可并行。
