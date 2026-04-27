# CMDB 项目评审报告 — 8 维度

**日期**：2026-04-21
**评审方式**：Harness Design generator-reviewer 模式，6 个并行 Explore sub-agent 独立验证
**整体判定**：**NOT DONE**（D4、D7 低于硬门槛）
**均分**：6.5/10

---

## Step 2: 多维评审（Scorecard）

| # | 维度 | 得分 | 硬门槛 | 判定 |
|---|------|------|--------|------|
| D1 | 数据模型与 CI 本体 | 7/10 | ≥6 | ✅ PASS |
| D2 | CI 关系与拓扑 | 6/10 | ≥6 | ⚠️ PASS（边缘） |
| D3 | 自动发现 | 6/10 | ≥5 | ✅ PASS |
| D4 | 数据质量 | 5/10 | ≥6 | ❌ **FAIL** |
| D5 | API 一致性 | 8/10 | ≥6 | ✅ PASS |
| D6 | 集成能力 | 6.5/10 | ≥5 | ✅ PASS |
| D7 | 工程质量 | 5/10 | ≥6 | ❌ **FAIL** |
| D8 | 安全 | 8.5/10 | ≥6 | ✅ PASS |

---

### D1 数据模型与 CI 本体 — 7/10（PASS）

**证据**
- `db/migrations/000004_assets_and_racks.up.sql:33` — `attributes JSONB NOT NULL DEFAULT '{}'`，支持扩展属性无需改表
- `internal/domain/assets/` — 清晰的领域分层（asset/rack/location 分治）
- 多租户 `(tenant_id, *)` 唯一约束贯穿全表

**缺口**
- 缺少 CI 本体字典（CI type registry）与属性 schema 校验 → JSONB 可写任意键
- 软删除语义不一致（部分表 `deleted_at`，部分表硬删）
- 缺少属性级别 audit trail（仅整行 `audit_log`）

**改进建议**
1. 新增 `ci_type_schemas` 表 + JSON Schema 校验 middleware
2. 统一软删除（全量 `deleted_at TIMESTAMPTZ`）
3. 属性变更写入 `attribute_change_log`

---

### D2 CI 关系与拓扑 — 6/10（边缘 PASS）

**证据**
- `db/migrations/000008_ci_relations.up.sql` — 关系表已建（type/source/target）
- `internal/domain/ci_relations/` — 基础 CRUD 完整
- `internal/api/impl_ci_relations.go:112-189` — 支持 1 跳邻居查询

**缺口**
- 缺 **递归 CTE 拓扑遍历**：无法 "给定 asset，返回 N 跳影响面"
- 无环路检测（可建 A→B→A）
- 无关系类型本体（containment / dependency / communication 未区分）

**改进建议**
1. 新增 `/ci_relations/topology?root=X&depth=N` 端点，内部用 `WITH RECURSIVE`
2. 插入时检测环路（SQL trigger 或应用层 DFS）
3. 关系类型表 + 方向性约束

---

### D3 自动发现 — 6/10（PASS）

**证据**
- `internal/domain/discovery/` — SNMP/SSH/Agent 三模式骨架
- `discovered_assets` 表 + approval workflow（`impl_discovery.go:233-298`）
- 去重 hash 字段存在

**缺口**
- 去重键只基于 IP+hostname，同一设备换 IP 会重复入库
- 无 "最后发现时间" 衰减策略（下线设备永不过期）
- Agent 心跳 TTL 未实现

**改进建议**
1. 去重加入 `serial_number` / MAC / BMC UUID 作为强标识
2. `last_seen_at` + 超过 N 天标记 stale
3. Agent heartbeat endpoint + TTL 清理 cron

---

### D4 数据质量 — 5/10（**FAIL**，差 1 分到门槛）

**证据**
- `internal/domain/location_detect/detector.go:80-86` — 幂等性 bug：缺 `external_id` 字段匹配，同一导入批重跑会复制
- 缺少 **数据校验规则引擎**（只在 OpenAPI 层做类型校验）
- 跨表一致性检查缺位：asset 可指向已删除的 location 而不报错（仅 FK `ON DELETE SET NULL`）
- 导入失败无 dead letter / 原因回写

**改进建议（详见 Step 3）**

---

### D5 API 一致性 — 8/10（PASS）

**证据**
- OpenAPI 作为 single source of truth，生成 handler stubs
- `make check-api-routes` 保证注册/spec 一致
- 统一错误信封 `{code, message, details}`（`internal/api/errors.go`）
- 50/51 `as any` cast 已消除（commit `380c773`）

**缺口**
- 分页语义不统一：部分端点 `offset/limit`，部分 `page/page_size`
- 批量端点缺幂等键（Idempotency-Key header）

**改进建议**
1. 统一为 cursor-based pagination（`next_cursor`）
2. 批量写入端点强制 `Idempotency-Key`

---

### D6 集成能力 — 6.5/10（PASS）

**证据**
- `internal/domain/integration/` — 适配器注册机制
- Webhook HMAC-SHA256 v2 + timestamp replay 防御（`webhook_signer.go`）
- Circuit breaker（`sony/gobreaker`）包裹外呼

**缺口（HIGH）**
- `db/queries/integration.sql:36` — **`direction='inbound'` 硬编码**，双向同步实际不可用
- 缺 rate limiter per integration
- 无 backfill / replay 机制

**改进建议**
1. 移除 inbound 硬编码，direction 入参化
2. token-bucket per integration
3. `integration_events` 表 + replay endpoint

---

### D7 工程质量 — 5/10（**FAIL**，差 1 分到门槛）

**证据**
- `internal/domain/workflows/auto_workorders.go` — **1079 行**单文件（违反 <800 行硬约束）
- `tools/tenantlint/` 自定义 analyzer **未接入 CI**（仅本地可跑）
- 测试覆盖率不均：domain 层 ~70%，api handler 层 ~35%
- `go vet` 有 5 处 shadow warning 未修

**改进建议（详见 Step 3）**

---

### D8 安全 — 8.5/10（PASS）

**证据**
- `internal/platform/netguard/netguard.go:30-43, 124-141` — 完整 SSRF 防御（私网/DNS rebinding）
- `internal/platform/crypto/keyring.go:49-76, 132-162` — KeyRing 多版本加密 + 自动轮换
- `internal/middleware/rbac.go` — 细粒度 resource:action 权限模型
- bcrypt cost=12，JWT jti + Redis blacklist，熵校验 ≥4.0 bits/byte
- **重要更正**：audit-2026-04-19 所称的 1 CRITICAL + 6 HIGH IDOR **已被回归测试覆盖**（`TestTenantIsolation_*`）；`impl_sync.go:19-115` 的 `SyncResolveConflict` 通过 `pgx.Identifier.Sanitize()` + 白名单 + 位置占位符，无 SQL 注入风险

**缺口**
- `CMDB_SECRET_KEY` 仍是环境变量，未接 KMS
- 审计日志未签名（可被篡改）

**改进建议**
1. 接入 AWS KMS / HashiCorp Vault
2. 审计日志 Merkle chain / HMAC 链

---

## Step 3: 最小修复清单（仅针对 FAIL 维度）

### D4 数据质量 5 → 6（+1 分达标）

| P | 任务 | 文件 | 估时 |
|---|------|------|------|
| **P0** | 修复 location 幂等性：以 `(tenant_id, external_id)` 为冲突键，`ON CONFLICT UPDATE` | `internal/domain/location_detect/detector.go:80-86` | 0.5d |
| **P0** | Asset → Location FK 改为 `ON DELETE RESTRICT`，并在删除路径加预检 | `db/migrations/000004_*.up.sql`（新增 000020） | 0.5d |
| **P1** | 导入失败入 `import_errors` 表，UI 暴露"查看原因" | 新建 migration + `impl_import.go` | 1d |

**通过验证**：重跑同一 CSV 两次，DB 无重复；删除被引用 location 返回 409。

### D7 工程质量 5 → 6（+1 分达标）

| P | 任务 | 文件 | 估时 |
|---|------|------|------|
| **P0** | 拆分 `auto_workorders.go`（1079 行）按阶段：trigger / planner / executor / reconciler | `internal/domain/workflows/auto_workorders.go` | 1d |
| **P0** | 将 `tenantlint` 接入 CI：`.github/workflows/ci.yml` 新增 step `go vet -vettool=$(which tenantlint) ./...` | CI yaml | 0.5d |
| **P1** | 修 5 处 shadow warning（`go vet ./...` 零警告） | 逐个修 | 0.5d |
| **P2** | API handler 层测试覆盖率 35% → 60%（补 rack/location/ci_relations handler 测试） | `internal/api/*_test.go` | 2d |

**通过验证**：`wc -l` 最大文件 < 800；CI pipeline 包含 tenantlint；`go vet` 干净。

---

## Step 4: 可验证性说明

| 结论 | 验证方法 |
|------|---------|
| D1 得分 | Explore agent 读取 `db/migrations/*.up.sql`（21 个迁移文件）+ `internal/domain/assets/*.go`，统计 JSONB/唯一约束/软删除分布 |
| D2 得分 | Grep `WITH RECURSIVE` → 0 命中；读 `impl_ci_relations.go` 全文确认仅 1-hop |
| D3 得分 | 读 `internal/domain/discovery/` 全目录 + `discovered_assets` 表 schema；grep `last_seen` → 字段不存在 |
| D4 得分 | 读 `detector.go:80-86` 源码确认 INSERT 缺 `external_id` 冲突处理；读 FK 定义确认 `SET NULL` |
| D5 得分 | 运行 `make check-api-routes` 通过；grep `as any` 剩余 1 处（已知白名单） |
| D6 得分 | 读 `db/queries/integration.sql:36` 确认 `direction='inbound'` literal；读 `webhook_signer.go` 确认 v2 签名实现 |
| D7 得分 | `find internal -name '*.go' \| xargs wc -l \| sort -n \| tail` 得最大 1079；读 `.github/workflows/*.yml` 未见 tenantlint；`go vet ./... 2>&1 \| grep -c shadow` = 5 |
| D8 得分 | 读 `netguard.go:30-43, 124-141` 确认 private-CIDR + DNS 双校验；读 `keyring.go` + `impl_sync.go:19-115` 确认白名单 + Sanitize()；读 `internal/middleware/*_test.go` 确认 TenantIsolation 回归测试 |

每个维度由独立 Explore sub-agent 执行，返回结构化 JSON（score/verdict/evidence[]/improvements[]），本报告为其汇总。

---

## 总判定

**NOT DONE** — D4（数据质量 5/6）、D7（工程质量 5/6）低于硬门槛。

**亮点**：安全（8.5）、API 一致性（8）显著强于业界平均；SSRF/加密/多租户隔离工程化完成度高。

**关键短板**：幂等性 bug + 文件超长 + lint 未入 CI，均为低成本可修项。预计 **3–4 人日** 可使 D4/D7 双双达标 → 整体通过。
