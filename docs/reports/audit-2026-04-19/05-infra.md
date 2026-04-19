# 05 — Foundational / Infra: Identity / Audit / Sync / API Server

## 模块总览

这是支撑所有业务模块的基础设施层：identity 提供 JWT 登录 + 角色权限发放；audit 写入 `audit_events` 作为所有写操作的法证日志；sync 把中央 / 边缘两端的 domain event 打包成 SyncEnvelope 并经 NATS 传输；dashboard 为前端首屏聚合计数；API 层用 oapi-codegen 生成 `ServerInterface` + 路由，16 个按域拆分的 `impl_*.go` 文件实现它；middleware 链在 `cmd/server/main.go:323-362` 组装（Tracing → Recovery → CORS → SecurityHeaders → RequestID → SyncGate → Prometheus → Auth → RateLimit → RBAC）；platform 层提供 pgx/v5 连接池、AES-256-GCM keyring、Prometheus / OTEL、Redis 缓存、统一 response envelope。整体结构干净、分层清晰，但**鉴权覆盖 ≠ 授权覆盖**：middleware 放行的请求在 handler 层还有相当多遗留的 tenant_id 缺失与 owner 校验缺失问题。

---

## 1. Identity & RBAC

### 鉴权机制

手写 JWT，HMAC-SHA256 签发 / 校验，非标准库实现 —— `internal/middleware/auth.go:60-108`。

- 登录：`identity/auth_service.go:47-67` `Login()` 走 bcrypt 对比，签发 Access（15 分钟）+ Refresh（7 天）。
- Refresh：`auth_service.go:132-149`，Refresh token 是 32 字节 `crypto/rand` base64url，存 Redis `refresh:<token>`，**轮换时删除旧 token**（`s.redis.Del`）。
- Claims 结构：`middleware/auth.go:16-22`，只包含 `user_id / username / tenant_id / dept_id / exp`。**没有 jti / iat / nbf / iss**；也没有 refresh token 族绑定，理论上旧 access token 在 15 分钟内仍可用即便账号被禁（status 检查只在 login 时做）。
- WebSocket 鉴权：`middleware/wsauth.go:16-71`，支持 Authorization、`Sec-WebSocket-Protocol: access_token.<jwt>`、query `?token=xxx`（注释写"legacy fallback, to be removed"但仍在接受）。
- `/healthz /readyz /metrics` 未加任何鉴权 —— 见 `cmd/server/main.go:330-334`，Prometheus endpoint 裸露。

### 用户 / 角色 / 权限数据模型

`db/migrations/000002_tenants_and_identity.up.sql`：

- `tenants(id, slug UNIQUE)`
- `users(id, tenant_id, dept_id, username VARCHAR(100) NOT NULL UNIQUE, password_hash, status, source)` —— **`username` 全局 UNIQUE 而非 `(tenant_id, username)`**（`000002_tenants_and_identity.up.sql:26`）。多租户场景下，租户 A 的 `admin` 会阻止租户 B 创建 `admin`。
- `roles(id, tenant_id NULLABLE, name, permissions JSONB, is_system BOOL)` —— `tenant_id IS NULL` 表示系统级角色（例如 seed 里的 `super-admin`，`permissions = {"*": ["*"]}`）。
- `user_roles(user_id, role_id)` —— 纯 M:N，**无 tenant_id 列**，存在 cross-tenant role assignment 的可能（见下文 IDOR）。

权限模型：`permissions JSONB` 形如 `{"assets": ["read","write"], "*": ["*"]}`。由 `middleware/rbac.go:170-188` `mergePermissions` 合并同一用户下所有角色的权限；写 `write` 自动包含 `read`（`rbac.go:208-211`）。

### 权限检查在哪里执行

- **完全在 middleware** —— `middleware/rbac.go:77-126` 把 `path` 第一段映射成 resource，HTTP method 映射成 action，做一次 map 查表。
- Handler 层**没有**做 per-object ownership / tenant 校验的通用拦截 —— 每个 handler 自己写 `tenant_id = $N`，遗漏就是 IDOR（见第 1.5 节）。
- `resourceMap`（`rbac.go:31-58`）是硬编码字符串表：**新加 endpoint 忘记加表项会被 default-deny**（`rbac.go:89-91`），这是好事；但**已存在但语义错标的映射**（例如 `"bia": "system"`、`"quality": "system"`、`"discovery": "assets"`、`"upgrade-rules": "assets"`）会让真正应当 scope 到 `bia` / `quality` 自己资源的权限被按 `system` / `assets` 判定。

### 多租户隔离

- `tenant_id` 完全从 JWT claims 读出（`middleware/auth.go:50`），handler 通过 `tenantIDFromContext(c)` 取（`internal/api/impl.go:110-113`）。
- 没有 DB 级别 RLS / session-level `SET app.tenant_id`。**完全依赖每个 SQL 语句手写 `WHERE tenant_id = $N`**。
- 一旦某个 handler 漏写，租户之间就能互访。下面第 1.5 节列出了实际存在的漏洞。

### 有无安全隐患（实际代码扫描结果）

#### CRITICAL

1. **`SyncResolveConflict` 同时具 IDOR + 动态 SQL 注入**（`internal/api/sync_endpoints.go:188`）
   - Line 188 `SELECT … FROM sync_conflicts WHERE id = $1 AND resolution='pending'` —— **没有 `AND tenant_id = $2`**，任一租户认证用户可读取任一 conflict 的 `entity_type / entity_id / remote_diff`。
   - Line 196-198 `UPDATE sync_conflicts SET resolution=… WHERE id=$3` —— 同样无 tenant 限制。
   - Line 217-218：`fmt.Sprintf("UPDATE %s SET %s, updated_at=now() WHERE id=$%d", entityType, strings.Join(setClauses,", "), argIdx)`，其中 `setClauses` 里的列名 `key` **直接来自攻击者可控的 `remote_diff` JSON 字段名**（`diffMap` 的 key），拼进 SQL 未转义。这是一个可利用的 SQL 注入（通过投毒一个 `remote_diff = {"name=x,password_hash='...';--": 1}` 这类 key）。结合 CRITICAL #1 的 entityType 来自 sync_conflicts 行（已经不可信），攻击面更大。

2. **`/admin/migrate-statuses` 未做权限检查**（`cmd/server/main.go:368-372`）
   - 挂在 `v1` 下，虽然 RBAC middleware 会尝试把 `admin` 映射成 resource —— 但 `admin` 不在 `resourceMap` 中，默认 deny。**这里是安全的**（惰性验证过 default-deny 路径），但写法上应该显式 `system` role 检查，而不是依赖 resourceMap 的缺席。

3. **JWT secret 校验脆弱**：`cmd/server/main.go` 读 `cfg.JWTSecret` 但没有强制最小长度或强度；如果开发环境用了 `"dev"` 之类弱 secret 上了 prod，全部 token 可伪造。没看到 startup-time 校验。

#### HIGH

4. **`ListUserSessions` IDOR**（`internal/api/session_endpoints.go:39-82`）
   - URL 参数 `:id` 直接作为 `WHERE user_id = $1`（line 45），**没有比对 `userIDFromContext(c)`，也没有 admin 校验**。任意登录用户可读任意用户的 session 列表（IP、UA、浏览器、时间）。

5. **Prediction 接口缺 tenant 过滤**
   - `GetAssetRUL`（`prediction_endpoints.go:37-56`）：`SELECT … FROM assets WHERE id=$1` 单参数，跨租户 asset 可读。对比同文件 `GetAssetUpgradeRecommendations`（line 262-265）已正确用 `id=$1 AND tenant_id=$2`，证明是遗漏不是有意。
   - `UpdateUpgradeRule`（`prediction_endpoints.go:680-693`）：`WHERE id=$1`，跨租户可改 upgrade rule。
   - `DeleteUpgradeRule`（`prediction_endpoints.go:698-705`）：`DELETE FROM upgrade_rules WHERE id=$1`，跨租户可删。

6. **`DeleteAssetDependency` 无 tenant 过滤**（`topology_endpoints.go:115-132`）
   - `DELETE FROM asset_dependencies WHERE id=$1`，跨租户可删拓扑边。

7. **Inventory task 状态 flip 无 tenant 过滤**
   - `impl_inventory.go:199` 和 `:311`、`inventory_resolve_endpoint.go:74` 都有 `UPDATE inventory_tasks SET status='in_progress' WHERE id=$1 AND status='planned'`。技术上 task_id 是 UUID 难猜但依然构成跨租户副作用通道。

8. **`user_roles` 表无 `tenant_id`**：`AssignRole(userID, roleID)`（`identity/service.go:103-109`）不会阻止把 T1 的 role 赋给 T2 的 user，也不阻止跨租户分配。当前 API 层也没 check `role.tenant_id == user.tenant_id`。

#### MEDIUM

9. **`publicPaths` 硬编码**（`rbac.go:22-28`）和 `cmd/server/main.go:341` 再重复一次 auth bypass 列表。两份列表同步维护，已见过分叉风险（`/api/v1/ws` 在两边都有，但如果未来加第三个公共路径只改一处就会出 bug）。

10. **Refresh token 撤销语义**：登出接口缺失。修改密码后未 invalidate 其它 refresh tokens（`auth_service.go:113-129`）。

11. **`seedAdmin` 回退路径**（`cmd/server/main.go:184-198`）如果 seed 文件不存在，**把随机 8 字符明文密码 WARN 到日志里**（line 197）。日志收集系统会归档该 secret。

12. **`CORS()` 默认放行全量 origin**（`middleware/cors.go:27-29`），`CORS_ALLOWED_ORIGINS` 未设时 `Access-Control-Allow-Origin: *` 且 `Allow-Credentials: true` —— 浏览器会拒绝但开发期间容易误以为生产可用。

---

## 2. Audit

### 数据模型

`db/migrations/000008_audit.up.sql`：

```sql
audit_events (id, tenant_id, action VARCHAR(50), module VARCHAR(30), target_type VARCHAR(30),
              target_id UUID, operator_id UUID REFERENCES users(id),
              diff JSONB, source VARCHAR(20) DEFAULT 'web', created_at TIMESTAMPTZ)
```

索引：`(tenant_id, created_at DESC)`、`(target_type, target_id)`、`(operator_id)`。够用。

### 写入时机

通过 `APIServer.recordAudit(c, action, module, targetType, targetID, diff)`（`impl.go:193-204`）调用，统一 `source="api"`。全 repo 共 67 处 `recordAudit(` 调用，分布在 23 个 handler 文件里。

**覆盖不均**：

- `impl_users.go` 8 处、`impl_inventory.go` 6 处、`impl_integration.go` 6 处、`impl_racks.go` 5 处、`impl_bia.go` 5 处 —— 主流写操作都有。
- `sync_endpoints.go` 只有 1 处（只 audit 了 `conflict_resolved`），**sync 应用到 DB 的 mutation（`agent.go` 各 applyX 函数）没有写 audit**。从 Central 推来的 update 对本地是"历史蒸发"的。
- `prediction_endpoints.go` 只 1 处 —— `UpdateUpgradeRule` / `DeleteUpgradeRule` 都没写 audit（和 #5、#6 的 IDOR 组合起来后果更严重）。
- Audit 错误被**吞掉**（`impl.go:200-203` 只 `zap.L().Error`，不向用户报错也不回滚事务），这是刻意的（审计不应阻塞业务），但意味着 audit 可能静默丢失。

### 存储策略

- **无保留策略 / 无分区 / 无压缩** —— 除 `(tenant_id, created_at DESC)` 索引外，`audit_events` 就是一张普通表无限增长。
- 查询分页靠 `QueryAuditEvents`（`domain/audit/service.go:24`），`LIMIT + OFFSET`，大数据量下 OFFSET 会变慢。
- 没有归档到冷存储 / S3 的机制。
- TimescaleDB 迁移 `000026_timescaledb_compression.up.sql` 只覆盖了 metrics，没动 audit_events。

### 跨租户 / system 操作

- `uuid.Nil operator_id` 用于标识系统操作 —— `domain/workflows/notifications.go:144` 注释：*"uuid.Nil as operatorID signals a system operation, bypassing self-approval checks"*；`impl_maintenance.go:118`、`maintenance/service.go:306` 都据此分支。
- `operator_id` 列是 `NULLABLE` 且 `REFERENCES users(id)` —— `uuid.Nil` 不是 NULL，而是 `'00000000-0000-0000-0000-000000000000'`，**不存在于 users 表**，应该会命中 FK 违反。`CreateAuditEvent` 写 `pgtype.UUID{Bytes: targetID, Valid: true}` 会写入 `all-zero UUID` → FK 应失败。实际运行时多半靠 ON DELETE NO ACTION 没 check（Postgres 外键在 zero UUID 上会 reject）。建议确认这条路径是否真的在被执行。
- `source` 字段语义：当前只有 `"api"`（来自 `recordAudit`）、`"web"`（表默认值）、sync envelope 的 `env.Source`（edge node ID 或 `"central"`）。这个字段同时被两种语义使用（"审计的来源"和"sync 的节点"），混在一起。
- per-tenant `diff` 是自由 JSON，格式不统一（有的写 `{"username": ...}`，有的写 `{"role_id": ..., "entity_type": ...}`），前端 `AuditEventDetail.tsx` 只能当 JSON 渲染。

---

## 3. Sync — Edge Agent 协议

### 目的

支持 "Central + Edge" 部署：边缘节点把本地 data center 的 asset / work_order / alert 数据异步推到中央；中央把 governance 决定（审批、分配）推回边缘。跨网络分区期间 edge 仍然可读可写。通过 `cfg.DeployMode = "central" | "edge"` 区分角色（`main.go:301`）。

### 协议

- **传输层**：NATS。主题 pattern `sync.{tenant_id}.{entity_type}.{action}`（`service.go:102`），agent 订阅 `sync.>`（`agent.go:50`）。
- **Envelope**（`envelope.go:14-47`）：

  ```go
  type SyncEnvelope struct {
      ID, Source, TenantID, EntityType, EntityID, Action string
      Version   int64           // 从该表 sync_version 列读
      Timestamp time.Time
      Diff      json.RawMessage // 原始 event payload
      Checksum  string          // sha256(entityID:version:diff)
  }
  ```

  Checksum 只校验完整性，**不做 HMAC 签名**：NATS 上任何能发布到 `sync.>` 的节点都能伪造 envelope；这对通常 NATS 部署（内网、有认证）可接受，但外部 edge 场景下值得加 MAC。
- **Layers**（`layers.go`）：硬编码的 5 层依赖顺序。`LayerOf("assets")` 返回 1，告诉 agent 必须先应用 layer 0（locations）才能 apply layer 1（assets）。实际 `handleIncomingEnvelope`（`agent.go:101-166`）只是**用 layer 值做日志/校验**，并没有真的排序缓冲 —— 如果乱序到达就靠 FK 约束 + `WHERE sync_version < EXCLUDED.sync_version` 幂等条件。
- **Service 侧**（`service.go`）：Central 把 22 种 domain event（`SubjectAssetCreated` 等，`service.go:40-66`）订阅转发为 SyncEnvelope。`onDomainEvent` 会先查当前表的 `sync_version` 列（`service.go:92-97`）—— 这里有 race：publish 时读到的 version 不一定是写入的版本。
- **Agent 侧**（`agent.go`）：每个 entity 类型一个专用 `applyX` 函数，都是 `INSERT ... ON CONFLICT (id) DO UPDATE SET ... WHERE sync_version < EXCLUDED.sync_version`，last-write-wins with version gate。
- **work_orders 双维状态**（`agent.go:168-209`）：central 推 `governance_status`，edge 推 `execution_status`，通过 `deriveStatusSQL` 融合 —— 这是为了让两侧都能独立决策同一条工单。

### 冲突处理

`sync_conflicts` 表（`000027_sync_system.up.sql:48-62`）记录 `local_version / remote_version / local_diff / remote_diff / resolution`。

- **没找到产生 conflict 行的代码** —— 搜索 `INSERT INTO sync_conflicts`，仅出现在测试和 workflows 的 `autoResolveStaleConflicts` 的 UPDATE。**目前 agent 的 apply 策略是 last-write-wins 静默覆盖，conflict 表看起来是预留但未启用**。
- Conflict resolution API 已存在（`SyncResolveConflict`）但正如上述，带 CRITICAL IDOR + SQL injection。

### TTL / 过期策略

- `workflows/cleanup.go:78-118` `autoResolveStaleConflicts`：
  - 先 3 天发 "conflict_sla_warning" 给 ops-admin。
  - 7 天 auto-resolve：`UPDATE sync_conflicts SET resolution='auto_expired'`。
  - `import_conflicts` 表同样 7 天 `status='auto_resolved'`。
- 注意 auto-expire **不会** apply `local_diff` 或 `remote_diff` 任何一方，只是标状态 —— 等于"放弃解决"。如果上面的 insert 路径真有数据会造成数据漂移；因为 insert 路径当前未启用，此处先放过。
- `expireStaleDiscoveries`：14 天 pending 发现标 expired（`cleanup.go:121-130`）。
- `sync_state` reconciliation：`service.go:128-198`，5 分钟跑一次，1 小时外 lag 的发 `sync.resync_hint`，24 小时外 lag 直接把 node 标 `status='error'`。
- Session 清理：`cleanup.go:28-55`，7 天未活跃 `expired`、30 天删除、每用户保留最多 20 条。

---

## 4. Dashboard

### 聚合了什么数据

`domain/dashboard/service.go:17-22`：

```go
type Stats struct {
    TotalAssets, TotalRacks, CriticalAlerts, ActiveOrders int64
}
```

- `TotalAssets` = `CountAssets(tenant_id)`（sqlc 生成）
- `CriticalAlerts` = `CountAlerts(tenant_id, status='firing')`
- `ActiveOrders` = `CountWorkOrders(tenant_id, status='in_progress')`
- `TotalRacks` = **手写 SQL**：`SELECT count(*) FROM racks WHERE tenant_id=$1 AND deleted_at IS NULL`（`service.go:78`）—— 其它三个走 sqlc，不一致。

全部被 `tenantID` filter，这里 tenant isolation OK。

### 是实时查还是缓存

- Redis 缓存，key `dashboard:stats:{tenant_id}`，TTL 60 秒（`service.go:25, 42-52`）。
- Read-through + write-through，cache miss 不是致命的 —— 落到 DB 直查。
- 失效策略：**被动过期**，没有在 asset / alert / order 写路径做 invalidate。这意味着 60 秒以内数据可能滞后。

### 被哪些页面消费

`cmdb-demo/src/hooks/useDashboard.ts` + `cmdb-demo/src/pages/Dashboard.tsx` + `cmdb-demo/src/pages/locations/GlobalOverview.tsx`。

**与业务的错位**：前端还有很多 KPI（能耗、机架利用率、维护工单 pending 数），但 `GetDashboardStats` 只返 4 个字段 → 前端要单独再拉 `fleet-metrics`、`capacity-planning`、`energy`、`monitoring/alerts` 凑数，dashboard 作为"聚合接口"实际上只聚合了一小半。

---

## 5. API 层

### OpenAPI + oapi-codegen 工作流（Track A ServerInterface）

- Spec：`/cmdb-platform/api/openapi.yaml`（5682 行，顶层 spec 在 monorepo 根目录，不在 `cmdb-core/api/`）。
- 生成器配置：`cmdb-core/oapi-codegen.yaml`：

  ```yaml
  package: api
  output: internal/api/generated.go
  generate:
    models: true
    gin-server: true
    embedded-spec: false
  ```

- 产物：`internal/api/generated.go`（**7517 行**，单文件，自动生成 DO NOT EDIT）—— 包含所有 request/response model、`ServerInterface` 接口、`ServerInterfaceWrapper`（把 HTTP 参数绑定到接口方法）、`RegisterHandlers(router, server)`。
- 触发：`Makefile`（未读，但从 header `// Code generated by github.com/oapi-codegen/oapi-codegen/v2 version v2.6.0 DO NOT EDIT.` 推断是 `make generate` / 手动 `oapi-codegen -config oapi-codegen.yaml openapi.yaml`）。
- 编译期保证：`impl.go:32` `var _ ServerInterface = (*APIServer)(nil)` —— 缺任何一个方法编译就失败，这是最强的一条契约。

### 按域拆分的 handler 文件（最近做的重构）

按 commit `6891768 refactor(api): split impl.go (3396 lines) into 16 domain handler files`，现状：

- `impl.go`（252 行）：只留 `APIServer` 结构 + helper（tenantIDFromContext、paginationDefaults、pg-type 转换、recordAudit、publishEvent、ciTypeSoftValidation）。
- `impl_auth.go` (74)、`impl_users.go` (284)、`impl_audit.go` (53)、`impl_assets.go` (377)、`impl_locations.go` (338)、`impl_racks.go` (298)、`impl_maintenance.go` (248)、`impl_monitoring.go` (188)、`impl_inventory.go` (341)、`impl_bia.go` (351)、`impl_integration.go` (356)、`impl_quality.go` (125)、`impl_discovery.go` (130)、`impl_prediction.go` (98)、`impl_incidents.go` (230)、`impl_system.go` (40)。
- 另有 "endpoint" 后缀的文件（`sync_endpoints.go`、`prediction_endpoints.go` 706、`topology_endpoints.go`、`notification_endpoints.go`、`sensor_endpoints.go`、`session_endpoints.go`、`rack_endpoints.go` 等）—— **命名分裂**：`impl_X.go` vs `X_endpoints.go` 两套命名共存，可读性差。建议统一。
- `convert.go`（874 行）：所有 db → API DTO 的转换函数（toAPIUser、toAPIRole、toAPIAuditEvent 等）。稍大，按域再拆可以。

### generated.go 的自动生成规则

- 从 spec 的 `components.schemas` 生成 struct + Valid() 枚举验证方法；
- 从 `paths` 生成 ServerInterface 方法签名；
- path params 变成方法参数，query params 打包进 `XxxParams` struct；
- request body 生成 `XxxJSONBody` / `XxxJSONRequestBody`；
- 路由注册：`router.POST(baseURL+"/auth/change-password", wrapper.ChangePassword)`（`generated.go:7366` 一类），`Wrapper` 完成 binding 然后调用 `Handler.ChangePassword(c)`。

### 错误处理惯例（platform/response）

`platform/response/response.go` 提供统一 envelope：

- 成功：`{data, meta: {request_id}}`；分页 `{data, pagination, meta}`。
- 失败：`{error: {code, message}, meta}`。
- Helper：`OK / OKList / Created / BadRequest / NotFound / Unauthorized / Forbidden / InternalError / Err(status, code, message)`。
- 所有 handler 都调这些，非常一致 —— 只在一两处 (`impl_assets.go:103-114` QUALITY_GATE_FAILED、`sync_endpoints.go:91` gin.H) 直接用 `c.JSON`，破坏了 envelope。

---

## 6. Middleware 链

`cmd/server/main.go:323-362` 的实际注册顺序（执行顺序同）：

```go
router.Use(telemetry.TracingMiddleware("cmdb-core"))           // L323
router.Use(middleware.Recovery(), middleware.CORS(),
           middleware.SecurityHeaders(), middleware.RequestID()) // L324
router.Use(middleware.SyncGateMiddleware(&initialSyncDone, cfg.DeployMode)) // L325
router.Use(telemetry.PrometheusMiddleware())                   // L326

// v1 group:
v1 := router.Group("/api/v1")
v1.Use(<inline auth-bypass-for-login>)                          // L339-346
v1.Use(rl.Middleware())     // if RateLimitEnabled              // L356
v1.Use(middleware.RBAC(queries, redisClient))                  // L362
```

| 顺序 | Middleware | 职责 |
|------|------------|------|
| 1 | `TracingMiddleware` (otelgin) | OpenTelemetry span wrap，采样率 10%（`tracing.go:47`） |
| 2 | `Recovery()` | panic → 500 INTERNAL_ERROR，打印 stack（`recovery.go:13-24`） |
| 3 | `CORS()` | `CORS_ALLOWED_ORIGINS` 白名单，未设则 `*`（开发期） |
| 4 | `SecurityHeaders()` | XCTO / XFO=DENY / XSS / Referrer / Permissions-Policy / HSTS（仅 TLS）|
| 5 | `RequestID()` | `X-Request-Id` 透传或生成 uuid v4 |
| 6 | `SyncGateMiddleware` | edge 模式首次同步前返回 503 SYNC_IN_PROGRESS |
| 7 | `PrometheusMiddleware` | `http_requests_total` + `http_request_duration_seconds` |
| 8 | `TenantContext` | **声明但未挂载**（`tenant.go:8-12` 本来就是 no-op） |
| 9 | inline auth-skipper | `/api/v1/auth/login|refresh|ws` 跳过 auth |
| 10 | `Auth(jwtSecret)` | JWT 校验，写 `user_id / username / tenant_id / dept_id` |
| 11 | `RateLimiter.Middleware()` | 每 key token bucket，默认 100 rps / burst 200，优先 user_id，否则 client IP |
| 12 | `RBAC(queries, redis)` | `resourceMap[path[0]] + methodToAction` 权限 check |

### 注释掉但还在 mount 的 middleware

- `TenantContext` (`middleware/tenant.go`) —— 是 no-op passthrough。未挂载，但也未删除，以文件形式存在。相当于死代码。

### 次序问题

- **Tracing 在最外层，好**：span 能覆盖所有子中间件。
- **RateLimit 在 Auth 之后**：注释说是为了 user_id 级别限流避开 NAT 误伤 —— 合理。但代价是**未认证的请求（login、refresh）完全不经过 rate limit**，密码爆破无速率保护。建议给 login 单独加一个更紧的 IP-based 限流。
- **Prometheus 在 SyncGate 之后、Auth 之前**：SYNC_IN_PROGRESS 返回的 503 也会被计入 `http_requests_total`，没问题；但未认证的 401 会暴露全部路径到 metrics 的 label（高 cardinality），靠 `c.FullPath()` 规范化缓解了一部分。
- **Auth 和 RBAC 之间没挂 session revocation 检查** —— JWT 签出后 15 分钟内即便账号禁用 / 被删，仍然通行。

---

## 7. Platform 层

### `database/` — pgxpool 封装

`platform/database/postgres.go`（25 行）：

- `NewPool(ctx, url)` → 解析 config、`MaxConns=50, MinConns=5` 硬编码、Ping。
- **没有**：慢查询日志、查询超时默认值、连接生命周期控制、observability hooks（pgx 有 `tracer` 可注入 OTel span，但没做）。
- 每个 handler 里直接 `s.pool.Exec / Query / QueryRow`，没有 repo 封装 —— 和 `dbgen.Queries` 混用，一部分查询走 sqlc 生成的类型安全 API，一部分（尤其 sync / prediction / notification 的 dynamic SQL）走 raw pgx。这就是第 1.5 节多数 tenant_id bug 的温床。

### `crypto/` — AES-256-GCM KeyRing

`crypto.go`：单 key `aesGCMCipher`，干净、标准。
`keyring.go`：多版本 key 管理，on-disk 格式 `v{N}:nonce||ciphertext||tag`，`parseVersionPrefix` 优雅处理无前缀的 legacy 数据 → v1。

亮点：

- Env 命名：`CMDB_SECRET_KEY_V1..V32` + `CMDB_SECRET_KEY_ACTIVE`，legacy `CMDB_SECRET_KEY` 回退为 v1。
- `main.go:82-91` 启动必须加载 keyring —— 缺 key `Fatal`，不 fallback plaintext。
- Cipher interface 单方法 `Encrypt/Decrypt`，KeyRing 直接实现，下游无感。
- 分别有 `crypto_test.go` / `keyring_test.go`。

仅存的一个小问题：`NewAESGCMCipher` 接受 key slice 但不 copy（line 71 注释承认了），调用方误改会破坏 cipher 状态；mitigation 文档化但未强制。

### `telemetry/` — metrics + logging + tracing

- `logging.go`（30 行）：zap production JSON，ISO8601 time key。替代 global via `zap.ReplaceGlobals`（`main.go:64`）。无 log rotation（托管给外部 logrotate）。
- `metrics.go`（123 行）：
  - HTTP：`http_requests_total{method,path,status}`、`http_request_duration_seconds`（bucket 5ms..5s）。
  - WS：`ws_active_connections`。
  - NATS：`nats_messages_published_total{subject}`。
  - DB：`db_query_duration_seconds{query}`（**声明但 grep 未发现调用点** —— 未实际使用）。
  - Sync：`cmdb_sync_envelope_applied/skipped/failed_total{entity_type}`、`cmdb_sync_reconciliation_runs_total`（用得上）。
  - Integration：`integration_decrypt_fallback_total`、`integration_dual_write_divergence_total` —— 文档化良好。
- `tracing.go`（63 行）：`InitTracer(endpoint, serviceName, version)`，OTLP gRPC，采样 `TraceIDRatioBased(0.1)`。endpoint 空就返回 no-op —— 可在本地关掉。`otelgin.Middleware` 覆盖所有 HTTP。**DB / NATS 层未挂 tracer propagation**，trace 在 HTTP handler 出去后就断了。

### `response/` — API envelope

见第 5 节。干净、一致、有单元测试。只有一点：`atoi`（`response.go:146-167`）手写 int parse，标准库 `strconv.Atoi` 就够，没必要重写。

### `cache/` — Redis

`platform/cache/redis.go`（32 行）：极薄封装 `redis.NewClient + Ping`，加 3 个全局函数（`Set/Get/Del`），但**调用方都不用这 3 个函数**，全部直接注入 `*redis.Client` 自己调。封装等于装饰。

---

## 整体评估

### 安全基线

**Tenant 隔离**：middleware 只负责写 context，不强制 DB 过滤。后者完全靠 handler 自觉写 `tenant_id = $N`。已发现的遗漏：

| 严重度 | 位置 | 问题 |
|--------|------|------|
| CRITICAL | `sync_endpoints.go:188,197,217` | `SyncResolveConflict` IDOR + 动态 SQL 注入（diffMap key 拼列名） |
| HIGH | `session_endpoints.go:45` | `ListUserSessions` 任意用户 session 可读 |
| HIGH | `prediction_endpoints.go:51,687,700` | RUL / UpdateUpgradeRule / DeleteUpgradeRule 跨租户 |
| HIGH | `topology_endpoints.go:119` | `DeleteAssetDependency` 跨租户 |
| HIGH | `db/migrations/000002:49-53` | `user_roles` 无 tenant_id，`AssignRole` 可跨租户赋权 |
| HIGH | `db/migrations/000002:26` | `users.username` 全局 UNIQUE（应 `(tenant_id, username)`） |
| MEDIUM | `impl_inventory.go:199,311`, `inventory_resolve_endpoint.go:74` | Inventory task 状态 flip 无 tenant 过滤（UUID 难猜但仍构成副作用通道） |
| MEDIUM | login / refresh endpoint | 无 IP-based 速率限制（rate limit 在 auth 之后） |

**鉴权深度**：JWT 校验 + RBAC middleware 覆盖所有 `/api/v1/*` 除白名单。没有 endpoint 被误放行到 public list（白名单明确）。但 RBAC 是"resource prefix + action"粗粒度，不做 per-row ownership —— 这正是上面 IDOR 漏洞的根源。

**加密 / 密钥管理**：AES-256-GCM keyring 实现到位，密钥轮换可用 `CMDB_SECRET_KEY_V{N}` + `CMDB_SECRET_KEY_ACTIVE`；启动时 fail-closed。**JWT secret 强度未在启动时校验**，建议加。

### 可观测性

- **Log**：zap JSON 结构化，request_id 透传；handler 大部分 error 都 `zap.L().Error` —— 合格。
- **Metric**：HTTP + NATS + sync + integration decrypt 都有。**DB `db_query_duration_seconds` 声明未使用**；**业务维度指标缺失**（audit 写入速率、RBAC cache miss 率、webhook dispatch 成功率等）。
- **Trace**：HTTP 层 otelgin 覆盖；**DB / NATS / Redis 未接入 propagator**，tracer 只看到 HTTP in/out，看不到 span 内部。
- **Audit**：67 处调用点，主流写操作已覆盖，但 sync agent 的 DB apply、prediction 的 rule CRUD 漏写。

### 还需要补的基础设施

1. **Per-tenant enforcement 机制**：引入 Postgres RLS 或者一个 repo-layer helper（`tenantScoped(pool, tenantID).Query(...)`），把 `WHERE tenant_id=$N` 从 handler 搬到一个不可绕过的地方。
2. **`user_roles` 加 `tenant_id`**（或在 AssignRole 时校验 `user.tenant_id == role.tenant_id`）。
3. **修复 `users.username` 唯一性语义**：改成 `UNIQUE (tenant_id, username)`。
4. **JWT 升级**：加 `jti`、`iat`；登出 / 改密时把活跃 jti 打入 Redis 黑名单；刷新 token 族绑定。
5. **Login / refresh 的 IP rate limit**：前置独立 limiter，5 次 / 分钟粒度。
6. **Audit 保留策略**：时间分区 + 冷归档 —— 当前无限增长。
7. **Sync envelope HMAC**：跨 NATS 的 envelope 加 tenant-scoped HMAC，防伪造。
8. **Sync conflict insert 路径**：目前只有读 / 解决 / 清理，没有写入；要么接上要么删除 conflict 表。
9. **DB / NATS tracer 注入**：补 pgx `Tracer` 接口 + NATS header propagation，串起 end-to-end trace。
10. **统一 endpoint 命名**：`impl_X.go` 和 `X_endpoints.go` 两套命名并存 —— 全部改成 `impl_X.go` 或全部改成 `X_endpoints.go`。
11. **Dashboard 扩字段**：目前只有 4 个计数，前端还要单独拉 5+ 个接口才能凑出首页，不如在 service 里扩 stats。
12. **`dashboardSvc` 失效策略**：在 asset/alert/order 写路径打一个 invalidate event，让 Redis stats 即时过期。
