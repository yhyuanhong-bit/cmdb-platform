# 审计修复路线图(Remediation Roadmap)

> 基于 [00-summary.md](00-summary.md) 的发现,按**风险暴露 + 依赖顺序 + 工程复杂度**排序,分 5 个 Phase 递进修复。
>
> **原则**:
> - 同 Phase 内的修复可以并行,跨 Phase 不可并行(后一个 Phase 依赖前一个 Phase 的基线)
> - 每个修复必须给出:**代码位置 / 修改要点 / 验证方式**,不写"应当考虑"这类软建议
> - TDD:每条修复**先写失败的测试**,再改实现,最后回归
> - 原子 commit:每条修复一个 commit,conventional commit format
> - 按 `feedback_auto_commit_push.md` 要求,每完成一条立即 push

---

## Phase 0:紧急止血(本周内完成)

**目标**:堵住 1 个 CRITICAL + 密码爆破 + 敏感日志泄漏。每条完成即发版。

**入场条件**:无。可即刻开工。

**出场条件**:
- 外部安全扫描(或手工 curl)无法复现下面任意一条
- CI 通过,且有针对性的 regression test

### 0.1 修 `SyncResolveConflict` SQL 注入 + IDOR(**CRITICAL**)

- **位置**:`cmdb-core/internal/api/sync_endpoints.go:188-225`
- **改动要点**:
  1. `SELECT ... WHERE id=$1` 改为 `WHERE id=$1 AND tenant_id=$2`(把 `tenantID` 从 context 取)
  2. `UPDATE sync_conflicts ... WHERE id=$3` 同样加 `AND tenant_id`
  3. 动态 `UPDATE` 拼的列名做**白名单校验**:
     ```go
     allowedColumns := map[string]map[string]bool{
         "assets":    {"name": true, "description": true, "status": true, ...},
         "racks":     {"name": true, "location_id": true, ...},
         // 每个 entity_type 列出白名单,不在表里的 key 直接 400
     }
     if _, ok := allowedColumns[entityType][key]; !ok {
         return response.BadRequest(c, "INVALID_FIELD", fmt.Sprintf("field %q not resolvable", key))
     }
     ```
  4. 即使通过白名单,列名也只拼**常量 identifier**(用 `pgx.Identifier.Sanitize()` 或静态 map),**永远不要 `fmt.Sprintf` 用户输入做列名**
- **验证**:
  - 单测:构造 `remote_diff = {"name=x,password_hash='injected'--": 1}`,断言返回 400
  - 单测:用 tenant B 的 JWT 去解决 tenant A 的 conflict,断言 404
- **commit**:`fix(sync): prevent IDOR and SQL injection in conflict resolution`

### 0.2 修 4 个 Prediction / Topology IDOR(**HIGH**)

| 位置 | 改动 |
|------|------|
| `prediction_endpoints.go:51` GetAssetRUL | `SELECT ... WHERE id=$1` → `AND tenant_id=$2` |
| `prediction_endpoints.go:687` UpdateUpgradeRule | 加 `AND tenant_id=$N`,并在 UPDATE 前先 SELECT 校验归属 |
| `prediction_endpoints.go:700` DeleteUpgradeRule | 同上 |
| `topology_endpoints.go:119` DeleteAssetDependency | 加 `AND tenant_id=$2`(或 subquery 校验两端 asset 均属当前 tenant) |

- **验证**:每条 endpoint 写一条"tenant B 操作 tenant A 资源 → 404"的 regression test
- **commit**:`fix(api): enforce tenant_id on prediction and topology endpoints`

### 0.3 修 `ListUserSessions` IDOR(**HIGH**)

- **位置**:`session_endpoints.go:39-82`
- **改动**:
  ```go
  if pathUserID != currentUserID && !hasAdminRole(c) {
      return response.Forbidden(c, "FORBIDDEN", "cannot list other users' sessions")
  }
  ```
- **验证**:tenant A 的 user X 去请求 user Y 的 sessions → 403
- **commit**:`fix(session): restrict session listing to self or admin`

### 0.4 JWT secret 启动强度校验

- **位置**:`cmd/server/main.go` 加载 config 之后、启动 HTTP 之前
- **改动**:
  ```go
  if len(cfg.JWTSecret) < 32 {
      zap.L().Fatal("JWT_SECRET must be >= 32 bytes", zap.Int("got", len(cfg.JWTSecret)))
  }
  if entropy(cfg.JWTSecret) < 4.0 {  // shannon entropy per byte
      zap.L().Fatal("JWT_SECRET has low entropy; do not reuse common values")
  }
  ```
- **验证**:设 `JWT_SECRET=dev` 启动应 fatal
- **commit**:`feat(server): validate JWT secret strength at startup`

### 0.5 Login / Refresh 独立 IP rate limit

- **位置**:`cmd/server/main.go:339-362` 中间件链
- **改动**:把 login/refresh/ws 从 auth-bypass 列表提取出来,单独挂一个 IP-based limiter:
  ```go
  loginLimiter := middleware.NewIPRateLimiter(5, time.Minute)  // 5 req/min per IP
  v1.POST("/auth/login", loginLimiter.Middleware(), server.Login)
  v1.POST("/auth/refresh", loginLimiter.Middleware(), server.RefreshToken)
  ```
- **验证**:6 次/分钟的 login 失败应返回 429
- **commit**:`feat(auth): add IP rate limit to login/refresh endpoints`

### 0.6 `seedAdmin` 不再把密码 WARN 到日志

- **位置**:`cmd/server/main.go:184-198`
- **改动**:随机密码写到一个 `/tmp/seed-admin-password-$(date).txt` 文件,文件权限 0600,**不进 zap 日志**;只 log 文件路径
- **验证**:grep 启动日志不应出现明文密码
- **commit**:`fix(server): write seeded admin password to file instead of log`

### 0.7 ingestion-engine dev key fallback 加强 guard

- **位置**:`ingestion-engine/app/config.py:11,22-29`
- **改动**:
  ```python
  # 显式读取 env, 不设默认值
  INGESTION_DEPLOY_MODE = os.environ["INGESTION_DEPLOY_MODE"]  # 未设即崩
  if INGESTION_DEPLOY_MODE not in ("development", "staging", "production"):
      raise ValueError(f"invalid INGESTION_DEPLOY_MODE={INGESTION_DEPLOY_MODE}")
  if INGESTION_DEPLOY_MODE != "development" and credential_encryption_key == "0" * 64:
      raise ValueError("zero key only allowed in development")
  ```
- **验证**:`INGESTION_DEPLOY_MODE=production` 且 key 全零应启动失败
- **commit**:`fix(ingestion): require explicit deploy_mode, block zero key in prod`

---

## Phase 1:Tenant 隔离基线(2-3 周)

**目标**:把"每条 SQL 手写 `WHERE tenant_id`"这个脆弱约定替换成**结构性不可绕过**的机制。剩余 HIGH 级 IDOR 一次性修完。

**入场条件**:Phase 0 完成。

**出场条件**:
- 所有 dbgen 层查询必须带 tenant 参数(否则编译失败或 runtime fail-closed)
- 全仓库 grep 裸 `pool.Exec(` / `pool.Query(` 的写操作均经过 tenant helper
- 每个 HIGH 级 IDOR 有对应 regression test

### 1.1 选方案:Postgres RLS vs. Repo-layer helper

**推荐 Repo-layer helper**(阻力小、可增量迁移、不影响 sqlc)。

- **新增**:`cmdb-core/internal/platform/database/tenant.go`
  ```go
  type TenantScoped struct {
      pool     *pgxpool.Pool
      tenantID uuid.UUID
  }
  
  func Scope(pool *pgxpool.Pool, tenantID uuid.UUID) *TenantScoped {
      if tenantID == uuid.Nil {
          panic("tenant scope requires non-nil tenant")
      }
      return &TenantScoped{pool, tenantID}
  }
  
  // Exec/Query/QueryRow 自动把 tenantID 作为第一个参数注入, SQL 必须以 $1 引用 tenant_id
  func (s *TenantScoped) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
      if !strings.Contains(sql, "tenant_id") {
          return pgconn.CommandTag{}, errors.New("tenant-scoped query must reference tenant_id")
      }
      return s.pool.Exec(ctx, sql, append([]any{s.tenantID}, args...)...)
  }
  ```
- handler 内统一用 `db.Scope(s.pool, tenantID).Exec(...)`,消灭裸 pool 调用
- **linter**:加 `go vet` 自定义 analyzer,在 `impl_*.go` 文件里禁用 `s.pool.Exec` / `s.pool.Query` 的直接调用(例外白名单走注解)
- **commit**:`feat(platform): add tenant-scoped db wrapper; enforce via linter`

### 1.2 `user_roles` 加 `tenant_id` 列

- **Migration 000040**:
  ```sql
  ALTER TABLE user_roles ADD COLUMN tenant_id UUID;
  UPDATE user_roles ur SET tenant_id = u.tenant_id FROM users u WHERE ur.user_id = u.id;
  ALTER TABLE user_roles ALTER COLUMN tenant_id SET NOT NULL;
  ALTER TABLE user_roles ADD CONSTRAINT user_roles_tenant_match
      CHECK (tenant_id IS NOT NULL);
  -- 触发器校验 user/role 同 tenant
  CREATE OR REPLACE FUNCTION user_roles_tenant_check() RETURNS trigger AS $$
  DECLARE utenant UUID; rtenant UUID;
  BEGIN
      SELECT tenant_id INTO utenant FROM users WHERE id = NEW.user_id;
      SELECT tenant_id INTO rtenant FROM roles WHERE id = NEW.role_id;
      IF rtenant IS NOT NULL AND utenant != rtenant THEN
          RAISE EXCEPTION 'cross-tenant role assignment: user=% role=%', NEW.user_id, NEW.role_id;
      END IF;
      NEW.tenant_id := utenant;
      RETURN NEW;
  END $$ LANGUAGE plpgsql;
  CREATE TRIGGER trg_user_roles_tenant_check BEFORE INSERT OR UPDATE ON user_roles
      FOR EACH ROW EXECUTE FUNCTION user_roles_tenant_check();
  ```
- **Handler 校验**:`identity/service.go:103-109` `AssignRole` 先 SELECT `user.tenant_id` 和 `role.tenant_id`,不一致 400
- **验证**:admin 把 tenant-A role 赋给 tenant-B user → 400;系统 role(`tenant_id IS NULL`)仍可赋给任何用户
- **commit**:`feat(identity): enforce same-tenant user-role assignment`

### 1.3 `users.username` 唯一性改 `(tenant_id, username)`

- **Migration 000041**:
  ```sql
  ALTER TABLE users DROP CONSTRAINT users_username_key;
  CREATE UNIQUE INDEX users_tenant_username_unique ON users (tenant_id, username);
  ```
- **注意**:跨租户重名冲突会在 login 时需要带 `tenant_slug`。API `POST /auth/login` 需要加一个 `tenant_slug` 字段:
  ```go
  type LoginRequest struct {
      TenantSlug string `json:"tenant_slug"`  // 新增
      Username   string
      Password   string
  }
  ```
- **兼容**:如果 `tenant_slug` 为空,回落到"全局唯一 username → 自动定位 tenant"的旧行为,但加 deprecation 日志
- **验证**:两个 tenant 各建一个 `admin` 不冲突;旧 tenant 的 `admin` 仍可登录
- **commit**:`feat(identity): scope username uniqueness per tenant`

### 1.4 `checkShadowIT` / 后台扫描加 tenant 循环

- **位置**:`workflows/auto_workorders.go:405-565`(`checkShadowIT` / `checkDuplicateSerials` / `checkMissingLocation` 等)
- **改动**:不再跨 tenant 做一次大查询,改为:
  ```go
  tenants, _ := w.queries.ListActiveTenants(ctx)
  for _, tenant := range tenants {
      if err := w.checkShadowITForTenant(ctx, tenant.ID); err != nil {
          zap.L().Warn("shadow IT check failed", zap.String("tenant", tenant.ID.String()), zap.Error(err))
      }
  }
  ```
- **`ListDuePullAdapters`** 维持跨租户(后台任务设计如此),但在调用链加明确注释 + 指标 label `tenant_id`
- **commit**:`fix(workflows): per-tenant loops for scheduled governance scans`

### 1.5 `DeleteAssetDependency` 补 tenant 校验

- 已在 Phase 0.2 处理,此处不重复

### 1.6 Inventory task 状态 flip 加 tenant 过滤

- **位置**:`impl_inventory.go:199,311`、`inventory_resolve_endpoint.go:74`
- **改动**:所有 `UPDATE inventory_tasks SET status=... WHERE id=$1` 加 `AND tenant_id=$2`
- **commit**:`fix(inventory): enforce tenant scope on task state transitions`

### 1.7 SSRF 防御

- **新增**:`cmdb-core/internal/platform/netguard/netguard.go`
  ```go
  var blockedCIDRs = []string{
      "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
      "169.254.0.0/16", "::1/128", "fe80::/10", "fc00::/7",
  }
  
  func ValidateOutboundURL(raw string) error {
      u, err := url.Parse(raw)
      if err != nil { return err }
      if u.Scheme != "https" && u.Scheme != "http" { return errors.New("only http/https allowed") }
      ips, err := net.LookupIP(u.Hostname())
      if err != nil { return err }
      for _, ip := range ips {
          for _, cidr := range blockedCIDRs {
              _, network, _ := net.ParseCIDR(cidr)
              if network.Contains(ip) { return fmt.Errorf("blocked network: %s", ip) }
          }
      }
      return nil
  }
  ```
- **注意**:DNS rebinding 防御需在 HTTP client 的 `Dial` 钩子里再验一次(避免 DNS 查询到 IP1,连接到 IP2):
  ```go
  transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
      host, _, _ := net.SplitHostPort(addr)
      if err := netguard.CheckHost(host); err != nil { return nil, err }
      return (&net.Dialer{}).DialContext(ctx, network, addr)
  }
  ```
- **埋点**:`impl_integration.go` 的 CreateAdapter / UpdateAdapter 校验 endpoint;`webhook_dispatcher.go` 初始化 client 时替换 Dialer;`adapter_custom_rest.go` 的 `cfg.URL` 覆盖路径也要过
- **可配置**:admin 可以配白名单 override(审批流走 `system:write`)
- **验证**:`POST /integration/adapters {"endpoint":"http://169.254.169.254"}` → 400
- **commit**:`feat(netguard): SSRF defense for outbound adapter and webhook URLs`

### 1.8 JWT 加 `jti` + Redis blacklist + 登出接口

- **新增 Claims 字段**:`jti` (uuid)、`iat`
- **新增 endpoint**:`POST /auth/logout` — 把当前 access token 的 `jti` 写入 Redis `jti_blacklist:<jti>`,TTL = token 剩余生命
- **新增**:改密时所有该用户的 refresh token 撤销(`DEL refresh:*` scan by user 或 Redis sorted set 索引)
- **Auth middleware**:`middleware/auth.go:60-108` 多查一次 Redis `GET jti_blacklist:<jti>`,命中则 401
- **验证**:登出后旧 access token 请求返回 401;改密后所有 refresh 失效
- **commit**:`feat(auth): add jti, blacklist, logout endpoint, token revocation on password change`

---

## Phase 2:核心功能补齐(4-6 周)

**目标**:修"看起来能用但实际不工作"的功能链;把所有半成品要么补完,要么明确标记为 disabled / coming soon。

**入场条件**:Phase 1 完成,tenant 基线稳定。

**出场条件**:每条"功能断链"有对应 E2E 测试通过。

### 2.1 Monitoring alert evaluator(**最大的洞**)

两条路,择一:

#### 方案 A(推荐):内部 evaluator

- **新增**:`cmdb-core/internal/domain/monitoring/evaluator.go`
- 每 60s 扫 `alert_rules WHERE enabled=true`,对每条规则:
  1. 按 `rule.metric_name` + 时间窗(rule.condition 里的 `window_seconds`)查 TimescaleDB `metrics` 表的聚合(avg/max/p95)
  2. 按 `rule.condition` 里的 `operator` + `threshold` 判断
  3. 命中且无 firing alert_event → `CreateAlertEvent`
  4. 已 firing 但值已回落且持续 N 次低于 → `status=resolved`
- **`alert_rules.condition` JSONB schema**:
  ```json
  {
    "operator": ">",
    "threshold": 85,
    "window_seconds": 300,
    "aggregation": "avg",
    "consecutive_triggers": 2
  }
  ```
- **幂等**:同 rule 同 asset 已 firing → 不再 insert,只更 `trigger_value` + `updated_at`
- **dedup**:新增 `alert_events.dedup_key = rule_id:asset_id:hour`,UNIQUE
- **启动**:`cmd/server/main.go` 加一行 `monitoringSvc.StartEvaluator(ctx)`
- **metrics**:`monitoring_evaluator_runs_total`、`monitoring_rule_evaluation_duration_seconds`
- **commit**:`feat(monitoring): implement alert_rules evaluator`

#### 方案 B(备选):Alertmanager webhook 接收

- 加 `POST /monitoring/alertmanager/webhook`,解析 Prometheus Alertmanager payload,映射到 `alert_events`
- 需要协调外部 Alertmanager 配置,部署摩擦更大

### 2.2 Discovery Approve 真正创建 Asset

- **位置**:`impl_discovery.go` 的 `ApproveDiscoveredAsset`
- **改动**:在事务里:
  1. `UPDATE discovered_assets SET status='approved'`
  2. **INSERT INTO assets**(从 discovered_assets 的字段映射)— 当前缺这步
  3. publish `SubjectAssetCreated`
  4. 写 audit
- **幂等**:如果 `discovered_assets.approved_asset_id` 已非 null,跳过 INSERT(支持重试)
- **验证**:E2E:discover → approve → `GET /assets` 应包含新行
- **commit**:`fix(discovery): create asset row on approval`

### 2.3 BIA dependency 变更触发 propagate

- **位置**:`impl_bia.go:CreateBIADependency` / `DeleteBIADependency`
- **改动**:两个 handler 末尾都调 `queries.PropagateBIALevelByAssessment(ctx, assessmentID)`
- **性能顾虑**:dependency 表大时 propagate 会扫全表;可以异步(放 eventbus)
- **commit**:`fix(bia): propagate tier on dependency create/delete`

### 2.4 RCA 上下文真正填充

- **位置**:`domain/prediction/service.go:83-96` `CreateRCA`
- **改动**:
  ```go
  alerts, _ := s.queries.ListAlertEventsByIncident(ctx, incidentID)
  assets, _ := s.queries.ListAssetsForIncident(ctx, incidentID)
  req := ai.RCARequest{
      IncidentID:     incidentID,
      TenantID:       tenantID,
      RelatedAlerts:  alerts,
      AffectedAssets: assets,
      Context:        userContext,
  }
  ```
- 新增两个 sqlc query
- **commit**:`fix(prediction): populate RCA request with alerts and affected assets`

### 2.5 Webhook 熔断 + DLQ + retention

- **位置**:`domain/integration/webhook_dispatcher.go`
- **新增字段**(migration 000042):`webhook_subscriptions.consecutive_failures INT DEFAULT 0`、`last_failure_at TIMESTAMPTZ`、`disabled_at TIMESTAMPTZ`
- **逻辑**(镜像 adapter):3 次失败 → `disabled_at=now()`,发 ops-admin 通知
- **DLQ 表**(migration 000042):
  ```sql
  CREATE TABLE webhook_deliveries_dlq (
      id UUID PRIMARY KEY,
      subscription_id UUID,
      event_type TEXT,
      payload JSONB,
      last_error TEXT,
      attempt_count INT,
      created_at TIMESTAMPTZ DEFAULT now()
  );
  ```
- **Retention**:`cleanup.go` 加每天清理 `webhook_deliveries` 30 天前的行 + `webhook_deliveries_dlq` 90 天前的行
- **attempt_number** 列加到 `webhook_deliveries`,记录每次尝试而非覆盖
- **commit**:`feat(webhook): add circuit breaker, DLQ, retention, attempt tracking`

### 2.6 Webhook HMAC 加时间戳防重放

- **位置**:`webhook_dispatcher.go:123-135`
- **改动**:
  ```go
  timestamp := time.Now().UTC().Format(time.RFC3339)
  signed := timestamp + "." + string(body)
  mac := hmac.New(sha256.New, []byte(secret))
  mac.Write([]byte(signed))
  req.Header.Set("X-Webhook-Timestamp", timestamp)
  req.Header.Set("X-Webhook-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
  ```
- **文档更新**:接收方验证需包含 timestamp + `abs(now - timestamp) < 5 分钟` 拒绝
- **commit**:`feat(webhook): include timestamp in HMAC to prevent replay`

### 2.7 Context 生命周期修正

- **位置**:`cmd/server/main.go:67`
- **改动**:
  ```go
  ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer cancel()
  // 传 ctx 给所有 StartXxx
  // shutdown 时:
  <-ctx.Done()  // 等 signal
  shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer shutdownCancel()
  srv.Shutdown(shutdownCtx)  // 先停接受新请求
  // goroutine 自己通过主 ctx 退出
  ```
- **Webhook dispatcher** 用主 ctx 而非 `context.Background()`
- **commit**:`fix(server): propagate cancellation to background workers on shutdown`

### 2.8 SLA checker 原子化

- **位置**:`workflows/sla.go:30-55`
- **改动**:
  ```go
  rows, err := w.pool.Query(ctx, `
      UPDATE work_orders SET sla_breached=true, updated_at=now()
      WHERE tenant_id IS NOT NULL
        AND status IN ('approved','in_progress')
        AND sla_deadline < now()
        AND NOT sla_breached
      RETURNING id, code, assignee_id, tenant_id
  `)
  ```
- 用 `RETURNING` 一步拿到要通知的行,不再 SELECT → UPDATE
- **commit**:`fix(workflows): atomic SLA breach update with RETURNING`

### 2.9 `checkFirmwareOutdated` 用 semver

- **位置**:`workflows/auto_workorders.go:569-617`
- **改动**:新增 `domain/workflows/semver.go`,实现 `compareSemver(a, b string) int` 拆成 major/minor/patch 比较;非 semver 字符串回落 lexicographical + Warn 日志
- 用 `golang.org/x/mod/semver` 包(stdlib 生态)
- **commit**:`fix(workflows): use semver comparison for firmware version check`

### 2.10 Emergency 工单 transition 幂等 + 补偿

- **位置**:`notifications.go:131-156`
- **改动**:两次 transition 改为**一次 SQL**:`UPDATE work_orders SET governance_status='approved', execution_status='in_progress' WHERE id=?`,或用数据库 function 保证原子
- 如果确实需要两次分开:失败时把工单状态回滚到 `submitted` 或写 `order_anomaly` 事件,禁止"半停留在 approved"
- **commit**:`fix(workflows): atomic emergency work order state transition`

### 2.11 Quality 加定时扫描

- **新增**:`workflows/quality.go` 加 `StartQualityScanner(ctx)`,每天凌晨扫一次所有 tenant
- **可选**:tenant 级配置 `quality_scan_cron` 字段
- **commit**:`feat(quality): add scheduled full-tenant quality scan`

### 2.12 `PredictFailure` 接上调用 / 下掉接口

两条路择一:

- **接上**:加 `StartFailurePredictor` ticker(每小时),对每个 tenant 高风险资产调 `PredictFailure`,结果写 `prediction_results`,前端 `/prediction/results/ci/{id}` 改读这张表
- **下掉**:删除 `PredictFailure` 接口 + 三个 provider 的空实现,避免"看起来已接"的假象

推荐**先下掉**(YAGNI),等真有业务需要再接。

- **commit**:`refactor(prediction): remove unused PredictFailure interface`(方案二)

### 2.13 Sync conflict 写入路径:补 or 删

- **位置**:`sync/agent.go` 的 applyX 函数
- **决策**:当前策略是 last-write-wins 静默覆盖。如果业务需要保留冲突,补 insert 逻辑:版本 gap 触发 `INSERT INTO sync_conflicts`;如果不需要,删除 conflict 相关 endpoint + 表
- **推荐**:保留 schema,但标注 `sync_conflicts` 为"手动冲突通道"(只对手动介入的场景使用),自动同步明确走 LWW
- **commit**:`docs(sync): clarify LWW vs manual conflict resolution semantics`

### 2.14 Alert dedup 收紧

- **位置**:`notifications.go:117-128`
- **改动**:critical alert 的 dedup 只看**同类型** open 工单(`type='emergency'`),不再看所有 open 工单
- **commit**:`fix(workflows): dedup emergency orders only against same type`

### 2.15 `checkShadowIT` / `checkDuplicateSerials` 去掉 LIKE dedup

- **位置**:`auto_workorders.go:413, 476`
- **改动**:新增 `work_order_dedup` 表 `(tenant_id, work_order_id, dedup_kind, dedup_key, UNIQUE(tenant_id, dedup_kind, dedup_key))`,建工单时写一行,查重时 SELECT 这张表
- **Migration 000043**:建表 + 历史数据 backfill
- **commit**:`refactor(workflows): replace LIKE dedup with explicit dedup table`

---

## Phase 3:代码质量(6-8 周)

**目标**:消除"静默吞错 + 测试倒挂 + 巨型文件"三大债。让未来的 refactor 成本可控。

**入场条件**:Phase 2 完成。

**出场条件**:
- 无 `_ = pool.Exec(...)` 形式的 fire-and-forget(linter 禁用)
- 前端 `*.test.tsx` 覆盖 20 个核心组件
- 3 个 >800 行文件全部拆分
- Track B handler 数量 < 10(从 60 减到 < 10)

### 3.1 统一错误处理约定

**文档化**:`docs/ERROR_HANDLING.md` 规定三档:

| 场景 | 处理 |
|------|------|
| 用户请求路径 | `response.Err(c, status, code, msg)`;**必须 return** |
| 后台任务 / cron | `zap.L().Warn(...) + metric counter inc`;**继续循环** |
| 关键操作(audit、金额、加密) | `zap.L().Error + metric + notify ops-admin`;**上抛** |

**工具**:写一个自定义 go vet analyzer,禁止:
- `_, _ = pool.Exec(ctx, ...)` 在生产代码中
- `json.Unmarshal` 忽略 err
- `rows.Scan` 不检查 err

**位置**:所有 `workflows/*.go`、`impl_*.go`、`domain/*/service.go` 逐个修:

- `cleanup.go:30,34,38,102,107,122` — 6 处 `_` 改为 Warn + metric
- `notifications.go:47-57,166-168` — `createNotification` 必须检查 err
- `sla.go:33-35,60-62` — Warn + metric
- `auto_workorders.go` — `rows.Scan continue` 加 Warn + 累计 metric
- `adapter_*.go` — 去掉 5 处 `//nolint:errcheck`,改为 400 / skip 行 + Warn
- `maintenanceSvc.Create` 失败从 Debug 提到 Warn

**commit 策略**:每个文件一个 commit(15-20 个 commits)
**commit prefix**:`fix(errors): stop swallowing errors in <module>`

### 3.2 拆 3 个巨型前端 page

#### 3.2.1 `PredictiveHub.tsx` (1416 行, 6 tab 一锅)

- 拆成 7 个文件:
  ```
  pages/predictive/
    PredictiveHub.tsx           # 主页 + Tabs 切换 (<200 行)
    tabs/OverviewTab.tsx        # 原 overview tab 逻辑
    tabs/AlertsTab.tsx
    tabs/InsightsTab.tsx
    tabs/RecommendationsTab.tsx
    tabs/TimelineTab.tsx
    tabs/ForecastTab.tsx
  ```
- tab 之间共享的 hook 抽到 `hooks/usePredictiveContext.ts`
- **commit**:`refactor(predictive): split PredictiveHub by tab`

#### 3.2.2 `RackDetailUnified.tsx` (1227 行)

- 按 section 拆:`RackHeader` / `RackSlotGrid` / `RackPowerPanel` / `RackNetworkPanel` / `RackAlertsPanel` / `RackMaintenanceHistory`
- **commit**:`refactor(racks): split RackDetailUnified into section components`

#### 3.2.3 `SensorConfiguration.tsx` (902 行)

- 按配置分类拆:`SensorList` / `SensorEditor` / `SensorThresholds` / `SensorMapping`
- **commit**:`refactor(sensors): split SensorConfiguration into focused components`

### 3.3 补前端组件测试(20 个核心)

**优先级 20 个**(按业务关键度):
1. `AuthGuard` — 跳转逻辑
2. `Login` — 表单校验
3. `authStore` 已测,跳过
4. `AssetManagementUnified` 表格 + 过滤
5. `AssetDetailUnified` 每个 tab 独立
6. `MaintenanceHub` 工单列表 + 状态筛选
7. `WorkOrder` 创建表单
8. `MonitoringAlerts` 列表 + ack/resolve
9. `QualityDashboard` 维度展示
10. `RackDetailUnified` 拆分后的 slot grid
11. `StatusBadge` / `StatCard` 等 atoms
12. `Create*Modal` 共性(见 3.5)
13. `useWebSocket` — 订阅 + invalidation
14. `LocationContext` — 已测,跳过
15. 4 个 location overview 页
16. `BIA` 5 个页
17. `AuditHistory` — 分页 + 过滤
18. `SystemSettings`
19. `RolesPermissions`
20. `InventoryItemDetail`

**测试模板**(`*.test.tsx`):
```tsx
import { render, screen, userEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

describe('ComponentX', () => {
  it('renders expected state', async () => {
    // Arrange: mock API via msw
    // Act: render + user interaction
    // Assert: DOM + API call assertions
  });
});
```

**工具**:加 `msw` 做 API mock,每个 `use*.ts` hook 配对的 handler

- **commit 策略**:一个测试一个 commit
- **CI**:加 `vitest run --coverage`,阈值 50% 起步,每季度上调 10%

### 3.4 E2E 深化

- **位置**:`e2e/critical/*.spec.ts`
- **`asset-crud.spec.ts`** 从 9 行扩到真正的 CRUD:
  ```ts
  test('full asset lifecycle', async ({ page }) => {
    await login(page);
    await page.goto('/assets');
    await page.click('button:has-text("Create")');
    await page.fill('[name=name]', 'test-asset');
    await page.fill('[name=asset_tag]', 'T-001');
    // ... 所有必填
    await page.click('button:has-text("Save")');
    await expect(page.locator('text=test-asset')).toBeVisible();
    // edit, delete 同样真做
  });
  ```
- **配 Playwright 多浏览器**:
  ```ts
  projects: [
    { name: 'chromium', use: devices['Desktop Chrome'] },
    { name: 'firefox',  use: devices['Desktop Firefox'] },
    { name: 'webkit',   use: devices['Desktop Safari'] },
    { name: 'mobile',   use: devices['iPhone 14'] },
  ]
  ```
- **响应式测试**:320/768/1024/1440 断点
- **commit**:`test(e2e): deepen critical flows and add cross-browser`

### 3.5 Modal 统一 primitive

- **新增**:`components/ui/Modal.tsx` 基础 primitive(headless,a11y 完整)
- 14 个 `Create*Modal` / `Edit*Modal` 迁移到新 primitive,共性部分(header / footer button / 关闭快捷键 / focus trap)沉到 primitive
- **commit**:`refactor(ui): unify Modal primitive across feature modals`

### 3.6 URL state 落地

- **新增**:`hooks/useUrlState.ts` 封装 `useSearchParams`,支持类型安全的 get/set:
  ```tsx
  const [filters, setFilters] = useUrlState<AssetFilters>('asset-filters', defaultFilters);
  ```
- **迁移**:所有带过滤器 / 分页 / 排序的 list 页优先:Assets / Racks / Monitoring / Audit / Inventory / Maintenance
- **commit**:每页一个 commit

### 3.7 Track B handler 迁移到 ServerInterface — **DONE (2026-04-20)**

> 详见 `track-b-audit.md`。`dd6bae9` 时 168 个 operation 全部走 `RegisterHandlers`,
> `oapi-codegen.yaml` 的 `exclude-operation-ids` 已空,`main.go` v1 组只剩 3 条手动路由
> (`/auth/logout` 是历史遗留重复需独立 bugfix 清理;`/ws`、`/admin/migrate-statuses`
> 由 `UndocumentedAllowlist` 白名单,属基础设施路由,保持不变)。`make check-api-routes` 通过。

- **原现状**:60 个手写 `*gin.Context` handler 在 `main.go:349-449` 直接挂
- **历史策略**:分批迁移,每次 5-10 个。每迁一个:
  1. 在 `openapi.yaml` 补 operation + schemas
  2. `make generate-api` 生成新方法签名
  3. 把原 handler 函数签名改为 `ServerInterface` 要求的形式
  4. 从 `oapi-codegen.yaml` 的 `exclude-operation-ids` 移除该 operationId
  5. 从 `main.go` 删除手动路由注册
  6. CI 的 `check-api-routes` 自动验证
- **已落地 commits**:`d36a6cf` `52b935d` `66f2df8` `eb1ddbd` `ba54db4` `9a91826`

### 3.8 sqlc 覆盖补齐

- **现状**:52 张表,sqlc 覆盖 22
- **优先**:`sync_*`、`webhook_*`、`notifications`、`sensors`、`user_sessions`、`rack_network_connections`、`work_order_comments`、`work_order_logs`、`asset_dependencies`、`asset_location_history`
- **策略**:每迁一张表:
  1. `db/queries/xxx.sql` 写 query
  2. `sqlc generate` 产出 `internal/dbgen/xxx.sql.go`
  3. 所有引用该表的裸 SQL 替换为 sqlc 方法调用
  4. 顺手修 tenant 过滤
- **commit**:每张表一个 commit

### 3.9 handler 命名统一 — DONE 2026-04-20

- 决策:全部用 `impl_*.go`(因为 ServerInterface 迁移后都会收敛到 impl)
- 批量 rename:`sync_endpoints.go` → `impl_sync.go` 等
- 更新 import、git mv 保留历史
- **commit**:`refactor(api): unify handler file naming to impl_*.go`

**Outcome (2026-04-20)**: All 20 `*_endpoints.go` / `*_endpoint.go` handler
files under `internal/api/` now follow the `impl_<domain>.go` convention
and are discoverable with a single glob. Split across 7 commits on
master: `92f90ed` (small merges — maintenance comments, rack network,
import template), `e75ad29` (single-endpoint merges — inventory resolve,
location stats, fleet metrics), `54ba403` (custom_endpoints.go split
into impl_rack_stats / impl_asset_lifecycle / impl_alerts), `7457fb7`
(activity / capacity_planning / energy / notifications / qr renames),
`611687d` (inventory / location_detect / prediction_upgrades / sensors
renames), `6bd722f` (sessions / sync / topology renames + their test
files), `1923f76` (comment-reference cleanup in tenant-isolation and
`*_test.go` files). Verified with `make build`, `make lint`, and
`make test` — all green. The `*_service.go` narrow-interface files were
intentionally left in place (they define DI boundaries, not handler
routes).

### 3.10 消除装饰性硬编码数据

- **位置**:`Dashboard.tsx:15-70`、`src/data/fallbacks/*.ts`、`src/data/locationMockData.ts`
- **改动**:UI 里所有 "mock" / "fallback" 字样的数据块:
  - 有真实 API 的 → 直接改接 API
  - 暂无 API 的 → 显示"Coming soon"空态,不显示假数据
- **commit**:每页一个 commit

### 3.11 ingestion-engine Celery 重试配置

- **位置**:`ingestion-engine/app/tasks/*.py`
- **改动**:
  ```python
  @celery_app.task(bind=True, autoretry_for=(httpx.HTTPError, asyncpg.PostgresError),
                   retry_backoff=True, retry_backoff_max=600, max_retries=5)
  def import_excel_task(self, job_id):
      ...
  ```
- **位置**:`processor.py:145-146` 批量 import 错误上报:
  ```python
  stats["errors"].append({"row": row_idx, "reason": str(e), "field": ...})
  ```
- **commit**:`fix(ingestion): add retry config and structured error reporting`

---

## Phase 4:架构级整顿(季度级)

**目标**:解决"长期积累会越来越难"的结构性问题。这些不紧急但不做会逐步劣化。

**入场条件**:Phase 3 完成。

**出场条件**:
- WorkflowSubscriber 不再是神类
- Audit 表有 retention
- ingestion-engine 支持多副本
- Migration 序号有统一 registry

### 4.1 `WorkflowSubscriber` 拆分

- 新结构:
  ```
  internal/domain/workflows/
    notifications/    # 独立 struct + Register + NewNotificationSubscriber
    autoworkorders/   # 独立 struct
    metrics/          # 独立 struct + StartMetricsPuller
    sla/
    cleanup/
    divergence/
    start.go          # workflows.StartAll(ctx, deps) 串联
  ```
- `main.go` 从 8 个 `Start*` 改为 `workflows.StartAll(ctx, deps)`
- **commit**:每子系统一个 commit

### 4.2 Audit 分区 + 归档

- **Migration 000050**:`audit_events` 改为 Postgres declarative partitioning,按 `created_at` 月分区
- **Cron**:每月把 12 个月前的分区 detach + 导出 parquet 到 S3 + drop
- **新增**:`cmdb-core/cmd/audit-archive/main.go` CLI
- **commit**:`feat(audit): monthly partitioning and cold storage archival`

### 4.3 Sync envelope HMAC 签名

- **位置**:`sync/envelope.go:14-47`
- **新增字段**:`Signature` (HMAC-SHA256 over `ID|Source|TenantID|EntityType|EntityID|Version|Timestamp|Diff`)
- **密钥**:新 env `CMDB_SYNC_HMAC_KEY`,所有 node 共享(KeyRing 式轮换)
- **接收端**:校验签名失败 → 写 `sync_envelope_rejected_total{reason=bad_signature}` + log + drop
- **commit**:`feat(sync): HMAC-sign envelopes to prevent NATS forgery`

### 4.4 ingestion-engine 多副本化

- **MAC 扫描 leader election**:用 Redis `SET lock:mac-scan <nodeID> NX EX 600`
- **Celery broker**:已用 Redis,加 `task_acks_late=True` + `task_reject_on_worker_lost=True` 保证任务不丢
- **`CMDB_CORE_URL` 从 env 读**,支持 service discovery
- **进程内周期任务去掉**,改为 Celery beat schedule
- **commit**:`feat(ingestion): support horizontal scaling`

### 4.5 Migration 序号统一 registry

- **新增**:`docs/MIGRATIONS.md`,顶层表格记录每个序号的**归属服务**和**主题**
- **CI 守卫**:`.github/workflows/migration-check.yml` 扫描 `cmdb-core/db/migrations/` 和 `ingestion-engine/db/migrations/` 的所有数字前缀,撞号则 fail
- **commit**:`feat(ci): prevent migration number collision across services`

### 4.6 OTel 全链路 trace

- **pgx**:`pgxpool.Config.ConnConfig.Tracer = otelpgx.NewTracer()`(需引 `github.com/exaring/otelpgx`)
- **NATS**:publish 时把 span context 注入 NATS header,subscriber 提取并 continue span
- **Redis**:`otelredis` 包装 client
- **commit**:`feat(observability): end-to-end tracing across DB/NATS/Redis`

### 4.7 Dashboard 扩充 + 主动失效

- **位置**:`domain/dashboard/service.go`
- **扩字段**:energy_current_kw, rack_utilization_pct, pending_work_orders, avg_quality_score
- **失效**:订阅 asset.created / alert.fired / order.transitioned 等事件,DEL Redis cache
- **commit**:`feat(dashboard): expand stats and invalidate on domain events`

### 4.8 `operator_id = uuid.Nil` 解决 FK 违反

- **当前推测**:`'00000000-0000-0000-0000-000000000000'` 不在 `users` 表,FK 应 reject。需验证是否真的在 insert 成功。
- **决策**:
  - 方案 A:在 `users` 表插入一行 `id=uuid.Nil, username='system'` 作为哨兵,FK 合法
  - 方案 B:`operator_id` 改为 NULLABLE + 去 FK,`NULL = 系统操作`
- **推荐 A**,好查询(WHERE operator_id = uuid.Nil)
- **Migration 000051**:
  ```sql
  INSERT INTO users (id, tenant_id, username, password_hash, status, source)
  SELECT '00000000-0000-0000-0000-000000000000', t.id, 'system', '', 'active', 'system'
  FROM tenants t
  ON CONFLICT DO NOTHING;
  ```
- 但这又撞了 tenant_id 的多租户约束 — 详细方案需设计后再定。标记为 `TODO: 专题设计`
- **commit**:`fix(audit): resolve uuid.Nil operator FK violation`

### 4.9 `publicPaths` / `resourceMap` 配置化

- 两份硬编码表合并成一份 `config/rbac.yaml`,启动时加载
- **commit**:`refactor(rbac): externalize public paths and resource map`

### 4.10 pgxpool hook:慢查询 + observability

- **位置**:`platform/database/postgres.go`
- **改动**:注入 `tracer` + `on_slow_query` hook(阈值 500ms → Warn)
- **commit**:`feat(db): slow query logging and OTel tracer`

---

## 工作量 & 团队配置估算

| Phase | 工作量(人周) | 最小团队 | 可并行? |
|-------|-------------|---------|---------|
| 0 | 1-2 | 1 senior(全栈) | 7 条可并行 |
| 1 | 8-12 | 1 Go + 1 DB/migration | 8 条基本可并行,1.1 和 1.4/1.6 有顺序依赖 |
| 2 | 20-30 | 1-2 Go + 1 前端 | 15 条大部分可并行;2.1 最重 |
| 3 | 30-40 | 1 Go + 2 前端 + 1 QA | 11 条大部分可并行 |
| 4 | 20-30(持续) | 1 senior + 1 平台工程 | 10 条可并行 |
| **合计** | **80-115 人周** | **3-4 人 / 6 个月** |

**建议节奏**:
- 周 1:Phase 0(所有人 all-hands)
- 月 1-2:Phase 1(tenant 基线)
- 月 2-4:Phase 2(功能补齐)
- 月 3-6:Phase 3(代码质量,和 Phase 2 最后 1 个月重叠)
- 月 5-8:Phase 4(架构,和 Phase 3 末段重叠)

---

## 验收与退出标准(全部完成的定义)

1. `docs/reports/audit-2026-07-19/00-summary.md`(下一次季度审计)中:
   - 0 个 CRITICAL
   - ≤ 2 个 HIGH(必须有明确的 Phase 4+ 计划)
   - "功能断链"清单为空
2. **自动化门禁**:
   - CI 的 `check-api-routes` 通过
   - CI 的 migration number collision check 通过
   - Go vet + 自定义 analyzer(无 swallowed error)通过
   - 前端 vitest coverage ≥ 60%
   - E2E 跨三浏览器通过
3. **运行时指标**(上线后 30 天观察):
   - `integration_decrypt_fallback_total{reason=decrypt_failed}` = 0
   - `integration_dual_write_divergence_total` = 0
   - `cmdb_sync_envelope_applied_total` ≥ `cmdb_sync_envelope_failed_total` × 100
   - `monitoring_evaluator_runs_total` 持续递增(evaluator 在跑)
   - `http_requests_total{status=~"5..", path!~"/metrics"}` < 0.1% 总流量
4. **审计日志审计**:每周抽样 100 条 `audit_events`,操作和 diff 对应 tenant,无 cross-tenant 污染

---

## 需要专题决策的事项

这些在 roadmap 里只给了占位方案,需要产品 + 架构评审单独讨论:

1. **Monitoring 方案 A vs B**(内部 evaluator vs Alertmanager webhook)— 看客户部署环境里是否已经有 Prometheus Alertmanager
2. **PredictFailure**:彻底下掉 vs 接上业务流程 — 取决于 AI 预测是否是产品承诺的核心卖点
3. **Sync conflict**:LWW 做底 vs 保留手动冲突通道 — 看客户是否需要审计冲突历史
4. **`users.username` 多租户改造**:login 加 tenant_slug 是 breaking change — 是否需要兼容期 + 双写
5. **Audit `uuid.Nil` FK**:全局 system 用户 vs NULL operator_id — 看现有查询习惯

---

**维护人**:audit orchestrator
**创建时间**:2026-04-19
**预计完成时间**:2026-10-19(6 个月滚动)
**下一次 checkpoint**:2026-05-19(Phase 0+1 完成度 review)
