# Backend Fixes Plan — 分步实施计划

> 日期: 2026-04-09
> Spec: `specs/2026-04-09-backend-fixes-spec.md`
> 修复项: P0-3, P1-7a/b/c, P1-6b/c

---

## 实施阶段总览

```
Phase 1: DB Migration (30 min)          ← 无依赖
Phase 2: OpenAPI Spec 更新 (30 min)     ← 无依赖
Phase 3: SQL + sqlc codegen (1 h)       ← 依赖 Phase 1
Phase 4: Go Service 层 (2 h)            ← 依赖 Phase 3
Phase 5: Go API Handler 层 (1 h)        ← 依赖 Phase 2 + 4
Phase 6: WebSocket Auth (1 h)           ← 独立于 Phase 3-5
Phase 7: Frontend 对接 (2 h)            ← 依赖 Phase 5 + 6
Phase 8: 集成测试 (1 h)                 ← 依赖全部
```

总估时: **~9 小时** (可并行则 ~6 小时)

---

## Phase 1: 数据库 Migration

**目标**: 添加 `deleted_at` 列支持软删除

**文件**: 新增 `cmdb-core/migrations/XXX_add_soft_delete.sql`

```sql
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE inventory_tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_work_orders_deleted_at 
  ON work_orders (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_tasks_deleted_at 
  ON inventory_tasks (deleted_at) WHERE deleted_at IS NULL;
```

**验证**:
- [ ] Migration 执行成功
- [ ] 现有数据的 `deleted_at` 为 NULL
- [ ] 索引创建成功

---

## Phase 2: OpenAPI Spec 更新

**目标**: 添加 3 个新端点 + 2 个查询参数

**文件**: `api/openapi.yaml`

### 2.1 添加 DELETE `/maintenance/orders/{id}` (line ~970)

在 `put` 方法之后添加 `delete` 方法：
- operationId: `deleteWorkOrder`
- 响应: 204 成功, 400 状态不允许, 404 未找到
- 详见 Spec 第 2 节

### 2.2 添加 PUT + DELETE `/inventory/tasks/{id}` (line ~1490)

在现有 `get` 方法之后添加:
- `put` — operationId: `updateInventoryTask`
- `delete` — operationId: `deleteInventoryTask`
- 详见 Spec 第 3、4 节

### 2.3 添加 location 查询参数

**`GET /maintenance/orders`** (line ~824):
```yaml
- name: location_id
  in: query
  schema:
    type: string
    format: uuid
```

**`GET /inventory/tasks`** (line ~1408):
```yaml
- name: scope_location_id
  in: query
  schema:
    type: string
    format: uuid
```

**验证**:
- [ ] `openapi-generator validate` 通过
- [ ] 运行 `oapi-codegen` 生成新的 Go server interface

---

## Phase 3: SQL 查询 + sqlc Codegen

**目标**: 添加新查询并重新生成 Go 代码

### 3.1 Work Order 查询

**文件**: `cmdb-core/queries/work_orders.sql`

```sql
-- name: SoftDeleteWorkOrder :exec
UPDATE work_orders
SET deleted_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;
```

修改 `ListWorkOrders`:
```sql
-- name: ListWorkOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::text IS NULL OR status = $2)
  AND ($3::uuid IS NULL OR asset_id = $3)
  AND ($4::uuid IS NULL OR location_id = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;
```

修改 `CountWorkOrders` 同步添加 `deleted_at IS NULL` 和 `location_id` 过滤。

### 3.2 Inventory Task 查询

**文件**: `cmdb-core/queries/inventory_tasks.sql`

```sql
-- name: UpdateInventoryTask :one
UPDATE inventory_tasks
SET name = COALESCE(NULLIF($3, ''), name),
    planned_date = COALESCE($4, planned_date),
    assigned_to = COALESCE($5, assigned_to)
WHERE id = $1 AND tenant_id = $2 AND status != 'completed' AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteInventoryTask :exec
UPDATE inventory_tasks
SET deleted_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND status = 'pending' AND deleted_at IS NULL;
```

修改 `ListInventoryTasks`:
```sql
-- name: ListInventoryTasks :many
SELECT * FROM inventory_tasks
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::uuid IS NULL OR scope_location_id = $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### 3.3 Codegen

```bash
cd cmdb-core && sqlc generate
```

**验证**:
- [ ] `sqlc generate` 无错误
- [ ] 新函数出现在 `internal/dbgen/work_orders.sql.go` 和 `inventory_tasks.sql.go`

---

## Phase 4: Go Domain Service 层

### 4.1 Maintenance Service

**文件**: `internal/domain/maintenance/service.go`

添加 `Delete` 方法:
```go
func (s *Service) Delete(ctx context.Context, tenantID, orderID string) error
```

修改 `List` 方法签名:
```go
func (s *Service) List(ctx context.Context, tenantID string, 
    status *string, locationID *string, limit, offset int) ([]WorkOrder, int, error)
```

**业务规则**:
- Delete: 仅 `draft` / `rejected` 状态可删除
- List: 传递 `locationID` 到 SQL 查询

### 4.2 Inventory Service

**文件**: `internal/domain/inventory/service.go`

添加:
```go
func (s *Service) Update(ctx context.Context, tenantID, taskID string, input UpdateInput) (*InventoryTask, error)
func (s *Service) Delete(ctx context.Context, tenantID, taskID string) error
```

修改 `List` 签名:
```go
func (s *Service) List(ctx context.Context, tenantID string, 
    scopeLocationID *string, limit, offset int) ([]InventoryTask, int, error)
```

添加 `model.go`:
```go
type UpdateInput struct {
    Name        string
    PlannedDate *time.Time
    AssignedTo  *string
}
```

**业务规则**:
- Update: 仅 `pending` / `in_progress` 可编辑; `method` 和 `scope_location_id` 不可改
- Delete: 仅 `pending` 可删除

**验证**:
- [ ] `go build ./...` 通过
- [ ] 单元测试覆盖状态校验逻辑

---

## Phase 5: Go API Handler 层

### 5.1 重新生成 Server Interface

```bash
cd cmdb-core
oapi-codegen -config oapi-config.yaml ../api/openapi.yaml > internal/api/generated.go
```

### 5.2 实现新 Handler

**文件**: `internal/api/impl.go` (或新文件 `maintenance_endpoints.go`)

| 方法 | 对应 Service 调用 |
|------|-------------------|
| `DeleteWorkOrder(c, id)` | `s.maintenanceSvc.Delete(ctx, tenantID, id)` |
| `UpdateInventoryTask(c, id)` | `s.inventorySvc.Update(ctx, tenantID, id, input)` |
| `DeleteInventoryTask(c, id)` | `s.inventorySvc.Delete(ctx, tenantID, id)` |

### 5.3 修改现有 Handler

| 方法 | 变更 |
|------|------|
| `ListWorkOrders` | 解析新 `location_id` 参数，传给 Service |
| `ListInventoryTasks` | 解析新 `scope_location_id` 参数，传给 Service |

**验证**:
- [ ] `go build ./...` 通过
- [ ] 编译时接口检查: `var _ ServerInterface = (*APIServer)(nil)`

---

## Phase 6: WebSocket Auth 改造

**独立于 Phase 3-5，可并行执行**

### 6.1 后端

**文件**: `internal/middleware/auth.go` 或新增 `ws_auth.go`

添加 `Sec-WebSocket-Protocol` token 提取逻辑（详见 Spec 方案 A）。

### 6.2 前端

**文件**: `cmdb-demo/src/hooks/useWebSocket.ts`

```typescript
// BEFORE:
const wsUrl = `${baseUrl}/ws?token=${token}`
const ws = new WebSocket(wsUrl)

// AFTER:
const wsUrl = `${baseUrl}/ws`
const ws = new WebSocket(wsUrl, [`access_token.${token}`])
```

### 6.3 兼容期

暂时保留 URL query token 作为 fallback，2 周后移除。

**验证**:
- [ ] 新方式可连接 WS
- [ ] 旧方式仍可连接（兼容期）
- [ ] 浏览器 Network tab 中 URL 不含 token

---

## Phase 7: Frontend 对接

### 7.1 Work Order 删除 (依赖 Phase 5)

| 层级 | 文件 | 改动 |
|------|------|------|
| API | `lib/api/maintenance.ts` | 添加 `delete()` |
| Hook | `hooks/useMaintenance.ts` | 添加 `useDeleteWorkOrder()` |
| UI | `pages/WorkOrder.tsx` | 添加删除按钮 (draft/rejected 状态) |

### 7.2 Inventory Task 编辑/删除 (依赖 Phase 5)

| 层级 | 文件 | 改动 |
|------|------|------|
| API | `lib/api/inventory.ts` | 添加 `update()`, `delete()` |
| Hook | `hooks/useInventory.ts` | 添加 `useUpdateInventoryTask()`, `useDeleteInventoryTask()` |
| UI | `pages/HighSpeedInventory.tsx` | 编辑按钮 + 复用 Modal; 删除按钮 (pending 状态) |

### 7.3 Location 过滤 (依赖 Phase 5)

| 页面 | 文件 | 改动 |
|------|------|------|
| MaintenanceHub | `pages/MaintenanceHub.tsx` | 从 LocationContext 传 `location_id` |
| HighSpeedInventory | `pages/HighSpeedInventory.tsx` | 从 LocationContext 传 `scope_location_id` |

前端改动模式与已完成的 `AssetManagementUnified` location 过滤一致。

**验证**:
- [ ] `tsc --noEmit` 无新错误
- [ ] 删除按钮仅在允许状态显示
- [ ] Location 切换后数据正确过滤

---

## Phase 8: 集成测试

### 手动测试清单

#### Work Order 删除
- [ ] 创建 draft 工单 → 删除成功 → 列表中不显示
- [ ] 创建 → approve → 尝试删除 → 返回 400
- [ ] 删除已删除工单 → 返回 404
- [ ] 删除后重新查询 → 确认软删除 (数据库 deleted_at 非空)

#### Inventory Task 编辑
- [ ] 创建 pending 任务 → 编辑 name → 成功
- [ ] 开始扫描 (in_progress) → 编辑 planned_date → 成功
- [ ] 完成任务 → 尝试编辑 → 返回 400
- [ ] 编辑 method → 返回 400 (不可修改)

#### Inventory Task 删除
- [ ] 创建 pending 任务 → 删除成功
- [ ] 开始扫描 → 尝试删除 → 返回 400
- [ ] 完成任务 → 尝试删除 → 返回 400

#### Location 过滤
- [ ] MaintenanceHub: 选择 campus → 仅显示该 campus 工单
- [ ] MaintenanceHub: 全局级别 → 显示所有工单
- [ ] HighSpeedInventory: 选择 IDC → 仅显示该 IDC 任务

#### WebSocket
- [ ] 新方式 (Sec-WebSocket-Protocol) 连接成功
- [ ] 实时事件推送正常
- [ ] 旧方式 (URL token) 仍可用 (兼容期)
- [ ] 浏览器 Network tab 确认 URL 无 token

---

## 优先级与依赖图

```
                    ┌──────────────┐
                    │  Phase 1:    │
                    │  DB Migration│
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐     ┌──────────────┐
                    │  Phase 3:    │     │  Phase 2:    │
                    │  SQL + sqlc  │     │  OpenAPI     │
                    └──────┬───────┘     └──────┬───────┘
                           │                    │
                    ┌──────▼───────┐            │
                    │  Phase 4:    │◄───────────┘
                    │  Go Service  │
                    └──────┬───────┘
                           │
┌──────────────┐    ┌──────▼───────┐
│  Phase 6:    │    │  Phase 5:    │
│  WS Auth     │    │  Go Handler  │
│  (并行)      │    └──────┬───────┘
└──────┬───────┘           │
       │            ┌──────▼───────┐
       └───────────►│  Phase 7:    │
                    │  Frontend    │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Phase 8:    │
                    │  集成测试     │
                    └──────────────┘
```

---

## 风险与降级方案

| 风险 | 概率 | 降级方案 |
|------|------|---------|
| sqlc codegen 与现有查询冲突 | 低 | 手写 Go 查询代码绕过 sqlc |
| Sec-WebSocket-Protocol 被企业代理拦截 | 中 | 回退到方案 B (首条消息认证) |
| `deleted_at` 列影响现有查询性能 | 低 | Partial index 已包含在 migration 中 |
| 软删除后 work_order_logs 孤立 | 无 | 软删除不删除关联数据，logs 仍可通过直接查询访问 |
| inventory task 编辑与正在进行的扫描冲突 | 中 | 限制 in_progress 仅可编辑 name 和 assigned_to |
