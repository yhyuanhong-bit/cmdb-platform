# 03 — Integration / Webhooks / 加密层

## 模块总览

集成层是**双向**的:

- **入站(inbound)**:`StartMetricsPuller` 每 5 分钟轮询 `direction='inbound'` 且未进入 backoff 窗口的 `integration_adapters`,调用注册的 `MetricsAdapter` 从 Prometheus / Zabbix / Custom REST 等监控系统拉取 `MetricPoint`,写入 `metrics` hypertable(见 `cmdb-core/internal/domain/workflows/metrics.go:59-111`)。
- **出站(outbound)**:`WebhookDispatcher` 订阅 eventbus 上 `asset.>` / `maintenance.>` / `alert.>` / `prediction.>` 四个前缀,把事件 payload 通过 HMAC-SHA256 签名后 POST 给 `webhook_subscriptions` 配置的外部 URL(`cmdb-core/cmd/server/main.go:446-453`、`cmdb-core/internal/domain/integration/webhook_dispatcher.go`)。

出站 adapter(`direction='outbound'` 或 `'bidirectional'`)在代码里没有主动推送实现——`ListDuePullAdapters` SQL 里硬编码 `direction = 'inbound'`(`cmdb-core/db/queries/integration.sql:36`),所以表里 seed 的 `'bidirectional'` 适配器(例如 ServiceNow,`000012_integration_tables.up.sql:37`)实际不会被轮询,也没有别的背景任务处理出站同步。**出站 adapter 目前是未实现的占位**,真正的"把 CMDB 事件推给外部"只走 webhook 链路。

加密层是最近(迁移 000038)加的:给 `integration_adapters.config` 和 `webhook_subscriptions.secret` 加 at-rest 加密,正处于 dual-write 阶段(密文列和明文列共存,新写同时落两列,读优先密文、按需回落明文)。

---

## 1. Integration Adapter

### 数据模型

`integration_adapters` 表经过三次迁移叠加(`000012_integration_tables.up.sql`、`000038_encrypt_integration_secrets.up.sql`、`000039_adapter_failure_state.up.sql`),当前列:

| 列 | 来源迁移 | 语义 |
|---|---|---|
| `id`, `tenant_id`, `name`, `type`, `direction`, `endpoint`, `enabled`, `created_at` | 000012 | 基本元数据 |
| `config JSONB` | 000012 | 明文配置(查询/凭据) |
| `config_encrypted BYTEA` | 000038 | dual-write 目标列,ciphertext = `v{N}:nonce‖aead.Seal(plaintext)` |
| `consecutive_failures INT NOT NULL DEFAULT 0` | 000039 | 连续失败计数,持久化失败状态 |
| `last_failure_at TIMESTAMPTZ` | 000039 | 最近一次失败时间 |
| `last_failure_reason TEXT` | 000039 | 截断到 500 字节的错误消息(`metrics.go:28`) |
| `next_attempt_at TIMESTAMPTZ` | 000039 | SQL 层 backoff gate |

配套有一个部分索引 `idx_integration_adapters_next_attempt ON (next_attempt_at) WHERE enabled=true AND direction='inbound'`,专供 puller 热路径使用(`000039_adapter_failure_state.up.sql:11-14`)。

dbgen 映射见 `cmdb-core/internal/dbgen/models.go:205-220`。

### 生命周期(Create → Enable → Sync → Fail → Backoff → Recover)

1. **Create**:`POST /integration/adapters` → `impl_integration.go:34-86`。Handler 本地 `json.Marshal(req.Config)` → `cipher.Encrypt(configBytes)` 同时写 `config` 和 `config_encrypted`(dual-write)。没有 `enabled` 字段默认 `true`。
2. **Enable / Disable**:PATCH 修改 `enabled`(`impl_integration.go:229-232`)。无额外副作用,下一个 puller tick 就会 pick up。
3. **Sync(一次拉取)**:`pullMetricsFromAdapters` → `ListDuePullAdapters` → `DecryptConfigWithFallback` → `GetAdapter(type).Fetch` → 命中 IP 的点写入 `metrics` 表(`metrics.go:162-177`,直接 raw `INSERT`,没走 sqlc)。
4. **Fail**:`handleAdapterFailure` 调用 `RecordAdapterFailure` 一条 SQL 原子地 `consecutive_failures += 1`、写 `last_failure_reason`、计算 `next_attempt_at`(schedule 在 SQL CASE 里,和 Go 的 `computeAdapterBackoff` 保持手工同步:30s / 2m / 10m / 30m 封顶,见 `integration.sql:59-64` 和 `metrics.go:37-48`)。
5. **Backoff**:下一次 puller tick 时 `ListDuePullAdapters` 的 SQL 过滤 `(next_attempt_at IS NULL OR next_attempt_at <= now())`,在冷却窗口内的 adapter 直接不被返回 —— 退避在 DB 而不是内存里做,服务重启不会意外重试。
6. **Auto-disable**:`row.ConsecutiveFailures >= 3`(`adapterDisableThreshold`,`metrics.go:23`)触发 `disableAdapter`,`enabled=false` + 写 `audit_events` (`action='adapter_auto_disabled'`) + 对该租户所有 ops-admin 发一条应用内 notification(`metrics.go:190-219`)。
7. **Recover**:成功一次拉取 → `RecordAdapterSuccess` 把 `consecutive_failures / last_failure_at / last_failure_reason / next_attempt_at` 全清零(`integration.sql:40-47`)。注意:auto-disable 把 `enabled` 置为 false 后,除非人工重新 `PATCH enabled=true`,puller 再也不会去看它,因此恢复路径是"人工介入"而非"自动愈合"。

### 每种类型的适配器

注册表见 `adapters.go:27-34`,API 层通过 `GetAdapter(type)` 按 `type` 字符串分发:

| Type | 实现 | 读取 | 写 | 鉴权 | 缺口 |
|---|---|---|---|---|---|
| `prometheus` | `adapter_prometheus.go:16-41` → `fetchPromMetrics` (in `prometheus.go`) | 配置里 `queries` 数组,逐个 `/api/v1/query` | 无(只拉) | 无(假设内网可达);不支持 bearer / basic auth header | 按 query 字符串串行拉取,没有并发、没有单 query 超时 |
| `zabbix` | `adapter_zabbix.go` | JSON-RPC `host.get` + `item.get` | 无 | `api_token` 优先,回落 `username/password` 走 `user.login`;明文存在 `config` JSONB 里 | 单个 item 失败被 `continue` 吃掉(line 76),无 per-item 错误上报;`json.Unmarshal` 结果都 `//nolint:errcheck` |
| `custom_rest` | `adapter_custom_rest.go` | 任意 REST + 可配 JSONPath dot 提取 | 无 | 整个 `Headers` map 任意字符串(包括 `Authorization`),所以 bearer / basic / API key 都能放这里 | `ResultPath` 点号导航不支持数组索引;只认 `status == 200`(301/302/204 都算错);15s timeout 硬编码 |
| `snmp` / `datadog` / `nagios` | `adapter_placeholder.go` | 全是 `return nil, fmt.Errorf("not yet implemented ...")` | — | — | 这三类在 `SupportedAdapterTypes()` 里**对外可见**,但 Fetch 必然失败;配 snmp adapter 三次就会被 auto-disable;SNMP 文档让走 ingestion-engine,但 API 校验不拒绝该 type |

**频率**:全局一个 `time.NewTicker(5 * time.Minute)`(`metrics.go:60`),所有 adapter 共享这一个节奏,配置里 `prometheus_config.PullIntervalSeconds` 字段**被定义但从未读取**(`adapter_prometheus.go:13` 仅在结构体里)—— 文档或 UI 允许配置"每个 adapter 的拉取间隔"就是**未实现**。

**Endpoint 信任模型**:puller 直接拿 `a.Endpoint.String` 去 `http.NewRequestWithContext`,**没有 SSRF 防御**。租户 admin 能创建一个 `endpoint=http://169.254.169.254/latest/meta-data/` 的 prometheus adapter,服务端进程身份凭据、IMDS、内网服务都可能被拉回来返给该租户(metric_name/ip/labels 会落到 `metrics` 表)。

### PATCH / DELETE 的约束与授权

**授权**:路由挂了 `middleware.RBAC` 中间件(`middleware/rbac.go:77`),资源名通过 `resourceMap` 把 `/api/v1/integration/*` 映射到 `integration` 资源(`rbac.go:44`),动作由 HTTP method 推出(`POST/PATCH = write`,`DELETE = delete`)。具体某个 role 是否拥有 `integration:write` / `integration:delete` 看 `roles.permissions` JSONB(`rbac.go:170-180`)—— 没有"adapter owner vs viewer"的更细粒度分权。

**租户边界**:handler 先做 `GetAdapterByID(ctx, adapterID, tenantID)`,不属于当前租户时返 404(`impl_integration.go:186-189`,`impl_integration.go:250-254`);UPDATE / DELETE 的 SQL 也带 `WHERE id=$1 AND tenant_id=$2`(`integration.sql:23`、`27`),防止跨租户越权。

**UPDATE 约束**:

- `UpdateAdapter` SQL 用 `COALESCE(sqlc.narg('x'), x)` 做 partial update(`integration.sql:15-23`);handler 里只有在 `req.Config != nil` 时才**同时**设 `params.Config` 和 `params.ConfigEncrypted`,保持 dual-write 两列一致(`impl_integration.go:213-228`)。SQL 注释也写明了这条约束(`integration.sql:13-14`)。
- 没有 `type` / `direction` 变更的合法性校验 —— 可以 PATCH 一个 `prometheus` adapter 把 `type` 改成 `nagios`(placeholder),auto-disable 会接上兜底,但没有"type 变更后 config schema 校验"。
- 没有 concurrency token / ETag;同时两个 PATCH 后写覆盖先写。

**DELETE**:纯 `DELETE WHERE id=$1 AND tenant_id=$2`(`integration.sql:27`),不检查是否有正在 in-flight 的 pull(puller 已经把行 `SELECT` 出来、调到第三方前,另一个 admin 可以把行 DELETE 掉,后续 `RecordAdapterFailure` 的 `WHERE` 就命不中;不是数据错误,只是指标丢失)。没有 soft-delete,没有级联清理 `metrics` 表中该 adapter 之前写的数据(metrics 只以 `asset_id` 关联)。

**审计**:create / update / delete 都走 `recordAudit`(`impl_integration.go:80-84`、`239`、`260-264`),`diff` 里记字段 key 和新值。**注意 create 时不记 `config` 明细**(可能包含凭据),但 **create 时的 `adapter.created` diff 也不掩码 `type` / `direction` 这些元数据**;update 的 `diff["config"] = "updated"` 刻意不落具体内容,避免审计日志泄密 —— 这一点是正确的。

---

## 2. Webhook

### 数据模型 (`webhook_subscriptions`)

来源迁移:`000012_integration_tables.up.sql:13-22` + `000014_webhook_bia_filter.up.sql`(新增 `filter_bia TEXT[]`)+ `000038_encrypt_integration_secrets.up.sql`(新增 `secret_encrypted BYTEA`)。

当前列(见 `cmdb-core/internal/dbgen/models.go:595-606`):`id`, `tenant_id`, `name`, `url`, `secret pgtype.Text`,`secret_encrypted []byte`,`events []string`, `enabled`, `filter_bia []string`, `created_at`。

投递流水表 `webhook_deliveries`(`000012:24-32`)列:`subscription_id`, `event_type`, `payload`, `status_code`, `response_body`(截断到 1024 字节,`webhook_dispatcher.go:146`),`delivered_at`。**没有 `attempt_number` / `retry_of` / `error_kind` 这些字段**,所以一个事件即便重试 3 次,UI 里只能看到"最后一次"的结果;且**没有 retention 清理**(`cleanup.go` 只清 sessions / conflicts / discoveries,webhook_deliveries 无限增长,`Grep` `DELETE FROM webhook_deliveries` 0 命中)。

### 发送流程

订阅见 `cmd/server/main.go:446-453`:

```go
webhookSubjects := []string{"asset.>", "maintenance.>", "alert.>", "prediction.>"}
for _, subj := range webhookSubjects {
    _ = bus.Subscribe(subj, dispatcher.HandleEvent)
}
```

**触发事件**:以上四个前缀下的所有事件。`webhook_subscriptions.events` 字段做精确匹配(`= ANY(events)`,`integration.sql:110`)—— 即 webhook 必须显式订阅 `asset.created` 而不是 `asset.*`。

**BIA 过滤**:如果 `filter_bia` 非空,dispatcher 会在子 goroutine 里把事件 payload 反序列化取 `asset_id`,再 `GetAsset` 查出 `bia_level` 做过滤(`webhook_dispatcher.go:57-88`)—— 注意这条路径每个事件要打一次额外 DB 查询。

**Payload 结构**(`webhook_dispatcher.go:97-103`):

```json
{
  "event_type": "<subject>",
  "tenant_id":  "<uuid>",
  "payload":    <原始事件 payload 原样嵌入>,
  "timestamp":  "<RFC3339 UTC>"
}
```

**HMAC 签名**(`webhook_dispatcher.go:123-135`):请求头 `X-Webhook-Signature: sha256=<hex>`,secret 从 `DecryptSecretWithFallback(cipher, sub.SecretEncrypted, sub.Secret.String)` 取。解密失败时 `secret=""` 并 skip 签名,继续投递;这是**降级行为**(投递仍会发出,接收方能看到 `X-Webhook-Event` 但无法校验),而没有抛告警阻止发送 —— 某种意义上是静默降级(见下文"缺口")。

**Request 头**:只设 `Content-Type: application/json`、`X-Webhook-Event: <subject>`、`X-Webhook-Signature`。**没有 `X-Webhook-Timestamp` / `X-Webhook-Id` / nonce**,接收方无法防重放;payload 本身的 `timestamp` 字段可以被伪造者换掉,因为它在签名 body 内侧而 HMAC 只签 body —— 实际上是够用的,但如果中间人能在**签名生成之后**篡改 body,会被 HMAC 捕获;风险在**重放**:攻击者把旧请求一字不差地回放,签名仍然 valid,接收方没有办法拒绝。

### 重试、死信、失败处理

**重试**:硬编码 `delays := []time.Duration{0, 1 * time.Second, 5 * time.Second}`(`webhook_dispatcher.go:106`),即 0s / 1s / 5s 三次尝试,非 2xx 或网络错继续下一次,2xx 提前 break。

**HTTP 超时**:`&http.Client{Timeout: 10 * time.Second}`(`webhook_dispatcher.go:35`)—— 单次 request 最多 10 秒,注意这个 timeout 不跟随调用方 ctx,用的是 `context.Background()`(`webhook_dispatcher.go:94`),所以**服务 shutdown 时正在投递的 webhook 不会被 cancel**,会跑完全部 3 次尝试。

**死信 / DLQ**:**不存在**。非 2xx + 3 次失败之后就是 `CreateDelivery` 记一条结果(`webhook_dispatcher.go:158-165`),没有 retry queue、没有 max-backoff、没有 disable webhook 的机制。一个一直 500 的订阅每次事件都会浪费 3 次 HTTP round-trip,无任何阈值熔断 —— 与 adapter 侧的 `consecutive_failures` 机制**不对称**。

**并发**:每次 `HandleEvent` 为每个订阅起一个 `go d.deliver(sub, event)`(`webhook_dispatcher.go:84-88`),**没有并发上限**;突发事件峰值时可能瞬间起几百上千个 goroutine 同时发 HTTP。

### 鉴权 secret 的使用

接收方侧:文档 / schema 里没有说明,接收方需要自己用同一个 `secret` 对 body 做 HMAC-SHA256 比较 `X-Webhook-Signature` 的 `sha256=<hex>` 部分。

发送方侧:`secret` 作为字段 PATCH 时,handler `impl_integration.go:305-314` 同样 dual-write 到 `secret`(pgtype.Text)和 `secret_encrypted`(BYTEA)。`diff["secret"] = "updated"` 不落明文到审计日志。轮换路径(下一节密钥)不影响 webhook secret 本身的值,只重加密密文。

---

## 3. 加密层 (at-rest secret 保护)

### 为什么需要(威胁模型)

保护的资产:`integration_adapters.config`(可能含 ServiceNow / Zabbix API token、REST Authorization header)和 `webhook_subscriptions.secret`(HMAC 签名密钥)。

对抗场景:

1. **数据库备份泄露**:`pg_dump` 文件被偷,无密钥时密文不可读。
2. **DBA / 第三方只读访问**:能看表内容但没 `CMDB_SECRET_KEY` 就读不到凭据。
3. **SQL 注入读表**:利用漏洞把这两列 `SELECT` 出来也只拿到密文。

**不保护**的场景(文档坦诚说明,`integration-encryption-deployment.md:27`):应用进程被攻破时密钥就在内存;dual-write 期间明文列还在,上述三条统统回落到"能读明文"。

### 实现层次

#### 3.1 `crypto.Cipher`(单密钥接口)—— `crypto/crypto.go`

- 算法:AES-256-GCM,12 字节 random nonce 前置,GCM 内置 16 字节 tag。wire 格式 `nonce(12) || aead.Seal(plaintext)`(`crypto.go:7-8`)。
- 接口 `Cipher` 只有 `Encrypt` / `Decrypt` 两个方法(`crypto.go:46-56`);`aesGCMCipher` 底层 `cipher.AEAD` 并发安全。
- 工厂 `NewAESGCMCipher(key)` 强制 `len(key)==32`;`CipherFromEnv(envVar)` 读 hex string 解码出 key。
- 设计决策:单密钥 Cipher 不内置轮换;**多密钥由 `KeyRing` 在上层叠**(`crypto.go:18-22` 注释)。

#### 3.2 `crypto.KeyRing`(多版本)—— `crypto/keyring.go`

- `KeyRing` 本身也实现 `Cipher` 接口(`keyring.go:47`),所以下游代码(`webhook_dispatcher`、`impl_integration`、两个 CLI)都拿 `crypto.Cipher` 就够,不需要关心是否在轮换中。
- **Encrypt**:用 `activeVersion` 的 key seal,前缀 `"v{N}:"`(`keyring.go:102-116`)。
- **Decrypt**:`parseVersionPrefix` 识别前缀;未知前缀会 fail loud(`keyring.go:126-128`)。**关键行为**:无前缀的旧密文(KeyRing 引入之前写的)被**强制路由到 v1**(`keyring.go:145-163`,`minPrefix=3`)—— 这是 backward compat 的保证。
- **Active 版本解析**(`keyring.go:228-246`):`CMDB_SECRET_KEY_ACTIVE` > 最高配置版本。强烈的隐式行为:"加了 V2 但忘改 ACTIVE,新加密也会自动跳到 V2";文档 §8.2 建议先把 ACTIVE 钉死在 V1 再加 V2,避免这个"友好"行为绊人。
- **Legacy fallback**(`keyring.go:199-208`):**没有任何** `CMDB_SECRET_KEY_V{N}` 时,`CMDB_SECRET_KEY` 被当成 V1 用。一旦设了任何 `_V{N}`,legacy 别名被**彻底忽略**。文档 §8.2 warning 说明了这点,代码注释也说了(`keyring.go:473-475`)。

#### 3.3 Dual-write 阶段 fallback —— `domain/integration/secrets.go`

- `DecryptConfigWithFallback(cipher, ciphertext, plaintext)`(`secrets.go:19-43`):密文非空就解密,解密失败 → bump `IntegrationDecryptFallbackTotal{reason=decrypt_failed}` 并**返错**(注意:这里不回落明文);密文为空 → bump `{reason=ciphertext_null}` 返回明文。
- `DecryptSecretWithFallback` 同理,只是 plaintext 类型是 `string`(webhook secret 的明文列类型)。
- **重要语义细节**:**解密失败不 fallback 到明文列**,只在"密文列为 NULL"时 fallback。这可能不符合直觉—— 部署时 `CMDB_SECRET_KEY` 配错、所有密文解密失败,读路径会直接失败而不是降级到明文。puller 里用 `continue` 跳过那一行(`metrics.go:92-95`),webhook dispatcher 里 `secret=""` 跳过签名继续投递(`webhook_dispatcher.go:125-129`),两种不同降级策略,都没有"回落到明文列"这条救急路径。

### 密钥配置

| Env | 说明 |
|---|---|
| `CMDB_SECRET_KEY` | 老单密钥;仅当**没有**任何 `_V{N}` 时作为 V1 生效 |
| `CMDB_SECRET_KEY_V1`, `V2`, ... | 版本化密钥,64 字符 hex / 32 字节 AES-256;扫描上限 `maxScannedVersion=32`(`keyring.go:30`) |
| `CMDB_SECRET_KEY_ACTIVE` | 整数,指明 Encrypt 用哪个版本;未设则取最高配置版本 |

**启动期校验**:`cmd/server/main.go:82-91` 直接 `zap.L().Fatal` 如果 `KeyRingFromEnv()` 返错 —— fail-closed,不会静默无加密运行。

### Migration 000038 / 000039 关系

- **000038**(`encrypt_integration_secrets.up.sql`):只加两个 `BYTEA` 列,都 nullable。**下迁移**只 `DROP COLUMN`,不会触碰明文列 —— 也就是说回滚到 000037 之后,dual-write 期间写过的新 adapter 里的**新字段**丢失但**明文**仍在,业务不中断(文档 §6.2 显式说了这点)。
- **000039**(`adapter_failure_state.up.sql`):4 列 + 1 索引。**独立于加密**,但为什么绑在一起?对照 git log 推测:两项都是"从内存状态搬到 DB 持久化"的一次集中改动,因此打包成同一次发布;关系上没有强耦合,000039 单独上线也能工作。

**文档与代码 gap**(对比 `integration-encryption-deployment.md` 发现):

- 文档 §5.1 预期启动日志包含 `migration: applied version=38`。搜过 `cmd/server/main.go` 启动路径,没看到显式的 `version=38` 格式日志;真正的迁移 runner 在 `cmd/server/main.go` 之外(没搜到专门 log 行),这条预期对不上实际日志输出的概率较高 —— 文档和代码的实际 log 格式我没交叉验证,留作一个小 gap。
- 文档 §9.1 错误消息 "failed to load at-rest encryption key (set `CMDB_SECRET_KEY`)"。实际代码是 "`failed to load at-rest encryption key ring (set CMDB_SECRET_KEY or CMDB_SECRET_KEY_V{N})`"(`cmd/server/main.go:84`)。错误文案不匹配,运维 grep 文档里的字符串搜不到。
- 文档 §11 快速 checklist 仍只写"校验 `echo ${#CMDB_SECRET_KEY}` 输出 `64`",没补 `_V{N}` 变种的检查清单。

---

## 4. 运维工具 (CLI)

### `cmd/backfill-integration-secrets/main.go` (305 行)

**目的**:把历史上(000038 之前)创建、只有明文没有密文的行补回填密文列。这些行 `config_encrypted` / `secret_encrypted` IS NULL,dual-write 代码不会主动回写它们。

**何时跑**:000038 上线后、`clear_integration_plaintext.sql` 之前。文档 §7.1。

**幂等性保证**:

- `SELECT` 和 `UPDATE` 都加 `config_encrypted IS NULL`(或 `secret_encrypted IS NULL`),第二次跑查出 0 候选(`backfill-integration-secrets/main.go:141-161`、`185-189`)。
- 空配置(`config::text = '{}'`)和空 secret 预先过滤,避免把"没有秘密"的行也加密,导致 dry-run 和 apply 计数不一致。
- 逐行独立写:单行 `cipher.Encrypt` 错误只 `stats.errors++` + stderr 一行,不中断整批(`main.go:178-193`)。
- 默认 dry-run,`--apply` 才真写。
- 用 `KeyRingFromEnv` 加载,Encrypt 走 active version —— 所以 backfill 写出来的密文就是 `v{active}:...` 前缀,不会产生任何无前缀 legacy 密文。

**审计事件行为**:apply 结束后,按 `tenant_id` 聚合,每个租户往 `audit_events` 写一条:

- `action='integration_backfill_completed'`
- `module='integration'`, `target_type='system'`
- `source='admin-cli'`
- `diff` JSON 含 `adapters_encrypted`, `webhooks_encrypted`, `tool`, `version`(`main.go:279-305`)

**为什么每个租户一条**:`audit_events.tenant_id` NOT NULL + FK,没有跨租户 sentinel(代码注释 `main.go:276-278`、SQL 脚本 `clear_integration_plaintext.sql:62-65` 也都说了这条约束)。

### `cmd/rotate-integration-secrets/main.go` (406 行)

**目的**:把早期版本(v1 或无前缀)加密的行重新加密到 active 版本,即 key rotation 的最后一步 "重新加密老数据"。对应文档 §8.4。

**何时跑**:在密钥轮换流程里 —— ① 新密钥 V{N+1} 配进环境 → ② `ACTIVE` 切到 V{N+1} → ③ **本 CLI** 重新加密 → ④ 校验 `pre_active=0` → ⑤ 退役老 V{N}。

**幂等性**:

- SELECT / UPDATE 的 where 子句都是 `position($1 IN config_encrypted) <> 1`,其中 `$1 = "v{active}:"`,只挑非 active 版本的行(`main.go:177-196`、`236-247`)。
- `UPDATE` 子句里**再加一次** `position($4 IN ...) <> 1`,防止并发 rotator 或 concurrent writer 把已经 rotate 到 active 的行被我们再次覆盖(防 TOCTOU)。
- `tag.RowsAffected() == 0` 视作 benign skip,不算 error(`main.go:248-252`)。
- 逐行独立,单行失败不中断。
- **用 `KeyRing.Decrypt`** 按前缀 dispatch 到正确版本的 key,因此 v1+v2 共存时都能解开。

**审计事件**:`action='integration_key_rotated'`,`diff` 含 `active_version`(`main.go:394-397`)—— 比 backfill 多了版本号,方便 dashboard 上画轮换时间线。

**一个与实现对齐的细节**:`detectVersion`(`main.go:356-373`)**刻意**和 `parseVersionPrefix` 有一处分歧:KeyRing 的 parser 把"损坏的 v 开头 + 非数字 / 无冒号"当 v1 处理;CLI 的 `detectVersion` 把"成功解析到整数但 <= 0"返回 0 作为"解析失败但不碰撞任何合法版本"的哨兵,只用于 `perVersion` 直方图分类。这个哨兵不参与决策,只做诊断 —— 合理。

### `db/scripts/cleanup/clear_integration_plaintext.sql` (131 行)

**目的**:dual-write 观察期通过之后,把明文列清零,转成"纯密文"的稳态。

**何时跑**:文档 §7.2,建议 ≥ 1 个完整发布周期无报错。**不是**自动迁移,刻意放在 `db/scripts/cleanup/` 避免 `main.go` 迁移 scanner 误扫(脚本顶部和 `cleanup/README.md` 都反复强调)。

**幂等性**:

- 整个脚本一个 `BEGIN; ... COMMIT;`。
- Pre-flight 块(`DO $$ ... $$`):统计还有多少行"有明文但无密文",任一 > 0 就 `RAISE EXCEPTION` 回滚,不动任何数据。
- 主 CTE 里 `UPDATE` 的 where 是 `config_encrypted IS NOT NULL AND config::text <> '{}'`(webhook 同理 `secret IS NOT NULL`),第二次跑在已清空的表上匹配 0 行,无副作用。

**审计事件**:按 CTE `RETURNING tenant_id`,FULL OUTER JOIN 聚合出 per-tenant 两个计数,INSERT 一条 `action='integration_plaintext_cleared'`,`module='integration'`,`source='admin-script'`。

---

## 5. 监控指标

### `integration_decrypt_fallback_total`

- 定义:`telemetry/metrics.go:70-73`,`CounterVec` labels `(table, reason)`。
- 埋点:只在 `DecryptConfigWithFallback` / `DecryptSecretWithFallback`(`domain/integration/secrets.go`)里 inc。
- **什么情况下会涨**:
  - `reason=ciphertext_null`:读到了明文列但密文列是 NULL,即命中的是"历史未回填行"。dual-write 正常跑了以后新行都同时写,所以这个计数稳定在一个上限(= 未 backfill 的老行数 × 每行被读到的次数)。**backfill 跑完后**应接近 0。
  - `reason=decrypt_failed`:密文非空但解密报错。通常意味着 `CMDB_SECRET_KEY` 错了 / 丢了 / 密文被改过 / 密钥版本不在 KeyRing 里。**任何非零**都该报警。
- 注意:名字叫 `_total` + `reason=ciphertext_null` 会让监控看板觉得"有 fallback 是坏事";实际稳态下 backfill 前它是正常流量指标,只有"没跑 backfill 但声称跑完了"才异常。需要运维手册指明这点。

### `integration_dual_write_divergence_total`

- 定义:`telemetry/metrics.go:81-84`,`CounterVec` label `(table)`。
- 埋点:只在 `domain/workflows/divergence.go` 的 `checkAdapterDivergence` / `checkWebhookDivergence` 里 inc —— 明文 JSON ≠ 解密后的 JSON(adapter)或明文字符串 ≠ 解密字符串(webhook)时 +1。
- **什么情况下会涨**:
  - 某一写路径只写了两列中的一列(比如手改 DB、或新加的 code path 漏了 dual-write)。
  - 解密失败(密钥问题,两种都算 divergence +1,见 `divergence.go:125`、`185`)。
- **任何非零值** = 该行的密文和明文不一致,回到文档 §6/§9 做诊断。
- **门控**:整个 divergence checker 受 `CMDB_INTEGRATION_DIVERGENCE_CHECK=1` 控制,**默认关闭**(`divergence.go:44-49`),所以不开 flag 就永远是 0。这是个 opt-in observability 工具,不是强制检查 —— 生产环境要主动开启。
- 采样:15 分钟一次,每张表 500 行/次(`divergence.go:27-31`),大表上收敛到"迟早全部扫过"。如果要抓某一条具体行的 divergence 窗口需要等。

---

## 整体评估

### Dual-write → cleanup 的完整 rollout 路径是否闭环

**闭环**。可追踪的阶段:

1. 000038 迁移 → `config_encrypted` / `secret_encrypted` 列创建,nullable,旧行 NULL。
2. 服务部署 → dual-write(create/update handler 同步写两列),读路径 `DecryptXxxWithFallback` 密文优先、NULL 才回落明文。
3. 可选运行 `StartDivergenceChecker`(feature-flag) → 15 分钟采样一次,发散时 `integration_dual_write_divergence_total` +1 + error log。
4. `cmd/backfill-integration-secrets --apply` → 填补历史无密文行;audit_events 留痕。
5. 观察期 → `integration_decrypt_fallback_total{reason=ciphertext_null}` 应归零,`{reason=decrypt_failed}` 和 divergence 指标都应保持 0。
6. `db/scripts/cleanup/clear_integration_plaintext.sql` → 带 pre-flight guard,不满足全 0 就 `RAISE EXCEPTION`;满足则清空明文列 + audit_events。
7. 轮换(可选)→ `cmd/rotate-integration-secrets` 把老版本密文升到 active 版本。

各阶段**幂等**、**有 audit trail**、**dry-run** 默认、**SQL guard** 防意外覆盖。整个路径在代码层、SQL 层、部署文档层都是一致的,算是做得相当完整的一次至 rest 加密 rollout。

### 还有哪些口子没补

**加密/轮换侧**:

- **Key rotation 自动通知缺失**:active 版本变更(启动日志 `active_version=2 available=[1,2]`)只在服务启动日志里出现,**不会**产生 `audit_events` 或 notification。审计上只能从轮换 CLI 的 `integration_key_rotated` 事件间接看到,但"服务启动配置了某个 active 版本"这条信息没留痕。
- **密钥轮换 dashboard 缺失**:`telemetry/metrics.go` 没有 `integration_encryption_active_version` gauge;运维靠日志反查。加一个 gauge vec (label by `version`) 很便宜,值就是"该版本下有多少行"。
- **Divergence checker 默认关闭**:rollout 质量最关键的探针要显式 `CMDB_INTEGRATION_DIVERGENCE_CHECK=1`。文档没在 checklist 里强调。

**Adapter 侧**:

- **出站(outbound/bidirectional) adapter 是占位**:`ListDuePullAdapters` 硬编码 `direction='inbound'`,`000012` seed 的 ServiceNow / 任何 bidirectional adapter 永不被处理。要么代码补实现,要么表里禁止这两种 direction。
- **`snmp` / `datadog` / `nagios` 注册了但永远失败**:API 层不拒绝这种 type 的创建,配置后必然 3 次失败被 auto-disable。应该在 `CreateAdapter` 里用 `SupportedAdapterTypes()` + 每类型的"实际 Fetch 可用"白名单做 400。
- **SSRF 无防御**:`endpoint` 任意 URL,puller 直连。至少应该校验 host 不落在 `127.0.0.0/8`、`169.254.0.0/16`、`::1` 等元数据/loopback 网段,或可配白名单。
- **Adapter 健康看板缺失**:有 `consecutive_failures` 列但没 Prometheus metric(`integration_adapter_consecutive_failures{adapter,tenant}` gauge、`integration_adapter_pull_duration_seconds` histogram 都没有)。看板只能直接查 DB。
- **Puller 并发度固定**:串行遍历 `ListDuePullAdapters` 返回的所有 adapter(`metrics.go:85-110`),单个慢 adapter 会阻塞后面所有 adapter 的这一轮 tick(5 分钟内可能全 miss)。
- **每 adapter pull interval 未实现**:`prometheusConfig.PullIntervalSeconds` 字段定义但未读。
- **Type 变更无 schema 校验**:PATCH 时能把 `type` 从 `prometheus` 改成 `zabbix` 但原来的 `config` 不匹配。

**Webhook 侧**:

- **没有熔断**:一个一直 500 的 webhook 每次事件都花 0+1+5s 三次重试,无任何 `webhook_consecutive_failures` 或 auto-disable(和 adapter 不对称)。
- **无 DLQ**:3 次失败之后事件丢失。
- **重试用 `context.Background()`**:关服时正在投递的事件仍跑到结束,优雅关闭的 10s timeout 可能不够覆盖(一个事件最长 = 3×10s request timeout + 0+1+5s delays ≈ 36s)。
- **`webhook_deliveries` 无 retention**:表无限增长,`cleanup.go` 里没清理逻辑;Grep `DELETE FROM webhook_deliveries` 0 命中。应加个 "30 天清一次" 的定时任务。
- **无 `attempt_number` 字段**:`deliveries` 表记录的是"最后一次"的 status_code,看不到重试轨迹。
- **HMAC 无时间戳**:只签 body,没有 `X-Webhook-Timestamp` 头 / nonce,理论上可被重放。
- **Secret 解密失败静默投递**:`webhook_dispatcher.go:125-129` 走 `secret=""` 跳过签名继续发 —— 虽然这是为了可用性,但**接收方会因为没收到签名直接拒收**,或更糟 —— 接收方没校验签名时收到了"未签名"payload。应该把这条事件 emit 成 `alert.fired` 或至少 Warn 级别日志 + 计数器,不该只是一行 error log。
- **Dispatcher 对每事件 spawn goroutine 无上限**:峰值下内存/连接数爆炸;应加 worker pool。
- **BIA 过滤 N+1**:每个订阅 + 每条事件都多一次 `GetAsset` DB 查询(`webhook_dispatcher.go:65`);频繁事件(例如 `asset.updated` 大批量导入)时放大成百次。

---

**维护人**:audit
**最后更新**:2026-04-18
