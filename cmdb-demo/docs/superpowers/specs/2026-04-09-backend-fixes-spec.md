# Backend Fixes Spec — API 契约定义

> 日期: 2026-04-09
> 来源: full-project-review-report.md + fix-impact-analysis-report.md
> 范围: 6 项需后端配合的修复

---

## 目录

1. [P0-3: WebSocket Token 传递方式](#1-p0-3-websocket-token-传递方式)
2. [P1-7a: Work Order 软删除](#2-p1-7a-work-order-软删除)
3. [P1-7b: Inventory Task 更新](#3-p1-7b-inventory-task-更新)
4. [P1-7c: Inventory Task 软删除](#4-p1-7c-inventory-task-软删除)
5. [P1-6b: Work Order Location 过滤](#5-p1-6b-work-order-location-过滤)
6. [P1-6c: Inventory Task Location 过滤](#6-p1-6c-inventory-task-location-过滤)

---

## 1. P0-3: WebSocket Token 传递方式

### 现状分析

**后端**: WebSocket handler (`internal/websocket/handler.go`) 通过 Gin auth middleware 认证，从 `c.Get("tenant_id")` 和 `c.Get("user_id")` 读取凭证。后端本身**不从 URL 读取 token**。

**前端**: `hooks/useWebSocket.ts:51` 将 JWT 附加到 URL: `ws?token=${token}`。这个 token 实际上被后端的 auth middleware 读取（作为 query parameter fallback）。

### 问题

Token 出现在 URL 中 → 浏览器历史、代理日志、CDN 日志均可见。

### 方案

由于后端 WS handler 已依赖 Gin middleware（不直接处理 token），修改集中在 **middleware 层**：

#### 方案 A: 协议升级前使用 Authorization Header（推荐）

浏览器 WebSocket API 不支持自定义 header，但可通过 **Sec-WebSocket-Protocol** header 传递 token：

**前端变更** (`hooks/useWebSocket.ts`):
```typescript
// BEFORE:
const wsUrl = `${baseUrl}/ws?token=${token}`
const ws = new WebSocket(wsUrl)

// AFTER:
const wsUrl = `${baseUrl}/ws`
const ws = new WebSocket(wsUrl, [`access_token.${token}`])
```

**后端变更** (`internal/middleware/auth.go` 或新增 WS auth middleware):
```go
func WebSocketAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 先尝试标准 Authorization header
        token := extractBearerToken(c.GetHeader("Authorization"))
        
        // 2. 回退: 从 Sec-WebSocket-Protocol 提取
        if token == "" {
            for _, proto := range strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",") {
                proto = strings.TrimSpace(proto)
                if strings.HasPrefix(proto, "access_token.") {
                    token = strings.TrimPrefix(proto, "access_token.")
                    // 回传 subprotocol 让浏览器接受连接
                    c.Header("Sec-WebSocket-Protocol", proto)
                    break
                }
            }
        }
        
        // 3. 验证 token (复用现有 JWT 验证逻辑)
        claims, err := validateJWT(token)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        
        c.Set("tenant_id", claims.TenantID)
        c.Set("user_id", claims.UserID)
        c.Next()
    }
}
```

**路由变更** (`internal/api/routes.go` 或 `cmd/server/main.go`):
```go
// 移除 WS 路由上的 query-param token 支持
wsGroup := router.Group("/api/v1/ws")
wsGroup.Use(WebSocketAuthMiddleware())
wsGroup.GET("", hub.ServeWS)
```

#### 方案 B: 首条消息认证

如果 Sec-WebSocket-Protocol 方案不可行：

**前端**: 连接后立即发送 `{ type: "auth", token: "xxx" }`
**后端**: WS handler 在 `OnConnect` 中等待首条消息，超时 5s 未收到 auth 则断开

此方案更复杂（需要 handler 层改动），建议**优先采用方案 A**。

### 兼容性

- 方案 A: 所有现代浏览器支持 `Sec-WebSocket-Protocol`
- 需要同时支持旧 token-in-URL 模式一段时间（渐进迁移）

---

## 2. P1-7a: Work Order 软删除

### 现状

- OpenAPI: `/maintenance/orders/{id}` 仅有 GET + PUT，无 DELETE
- Go service: `maintenance.Service` 无 Delete 方法
- SQL: `work_orders.sql.go` 无 DELETE 查询
- 状态机: `draft → pending → approved → in_progress → completed → closed`

### 设计决策

**采用软删除**（设置 `deleted_at` 时间戳）而非硬删除，原因：
1. 工单关联审计日志 (`work_order_logs`)、评论 (`work_order_comments`)
2. CMDB 合规要求保留操作痕迹
3. 已完成工单可能被其他系统引用

### 数据库变更

```sql
-- Migration: add deleted_at column
ALTER TABLE work_orders ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_work_orders_deleted_at ON work_orders (deleted_at) WHERE deleted_at IS NULL;
```

### SQL 查询 (`queries/work_orders.sql`)

```sql
-- name: SoftDeleteWorkOrder :exec
UPDATE work_orders
SET deleted_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- 修改现有 ListWorkOrders: 添加 AND deleted_at IS NULL
-- name: ListWorkOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND deleted_at IS NULL  -- 新增
  AND ($2::text IS NULL OR status = $2)
  AND ($3::uuid IS NULL OR asset_id = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;
```

### OpenAPI 变更 (`api/openapi.yaml`)

在 `/maintenance/orders/{id}` 路径下添加：

```yaml
    delete:
      operationId: deleteWorkOrder
      tags: [maintenance]
      summary: Soft-delete a work order
      description: |
        Marks a work order as deleted. Only work orders in 'draft' or 'rejected' 
        status can be deleted. Completed/closed orders are preserved for audit.
      parameters:
        - $ref: '#/components/parameters/IdPath'
      responses:
        '204':
          description: Work order deleted
        '400':
          description: Cannot delete work order in current status
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
```

### Go Service (`internal/domain/maintenance/service.go`)

```go
// Delete soft-deletes a work order. Only draft/rejected orders can be deleted.
func (s *Service) Delete(ctx context.Context, tenantID, orderID string) error {
    order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{
        ID:       uuid.MustParse(orderID),
        TenantID: uuid.MustParse(tenantID),
    })
    if err != nil {
        return ErrNotFound
    }
    
    // 业务规则: 仅 draft 和 rejected 状态可删除
    if order.Status != "draft" && order.Status != "rejected" {
        return fmt.Errorf("cannot delete work order in '%s' status", order.Status)
    }
    
    return s.queries.SoftDeleteWorkOrder(ctx, dbgen.SoftDeleteWorkOrderParams{
        ID:       uuid.MustParse(orderID),
        TenantID: uuid.MustParse(tenantID),
    })
}
```

### Go Handler (`internal/api/impl.go`)

```go
func (s *APIServer) DeleteWorkOrder(c *gin.Context, id openapi_types.UUID) {
    tenantID := c.GetString("tenant_id")
    err := s.maintenanceSvc.Delete(c.Request.Context(), tenantID, id.String())
    if err != nil {
        if errors.Is(err, maintenance.ErrNotFound) {
            response.NotFound(c)
            return
        }
        response.BadRequest(c, err.Error())
        return
    }
    c.Status(204)
}
```

### 前端变更

**API layer** (`lib/api/maintenance.ts`):
```typescript
delete: (id: string) => apiClient.del(`/maintenance/orders/${id}`),
```

**Hook** (`hooks/useMaintenance.ts`):
```typescript
export function useDeleteWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => maintenanceApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['workOrders'] }),
  })
}
```

**UI** (`pages/WorkOrder.tsx`): 添加删除按钮（仅 draft/rejected 状态显示）。

---

## 3. P1-7b: Inventory Task 更新

### 现状

- OpenAPI: `/inventory/tasks/{id}` 仅有 GET，无 PUT
- Go service: 无 Update 方法（仅有 Complete）
- 数据模型有 `name`, `method`, `planned_date`, `assigned_to`, `scope_location_id`

### 业务规则

- 仅 `pending` 和 `in_progress` 状态的任务可编辑
- `completed` 任务不可修改
- 可编辑字段: `name`, `planned_date`, `assigned_to`
- 不可编辑字段: `method` (一旦创建不可修改，因为已有扫描记录依赖此字段), `scope_location_id` (同理)

### SQL 查询

```sql
-- name: UpdateInventoryTask :one
UPDATE inventory_tasks
SET name = COALESCE($3, name),
    planned_date = COALESCE($4, planned_date),
    assigned_to = COALESCE($5, assigned_to)
WHERE id = $1 AND tenant_id = $2 AND status != 'completed'
RETURNING *;
```

### OpenAPI 变更

在 `/inventory/tasks/{id}` 路径下添加：

```yaml
    put:
      operationId: updateInventoryTask
      tags: [inventory]
      summary: Update an inventory task
      description: |
        Update editable fields of an inventory task. Only pending and in_progress
        tasks can be modified. Method and scope_location_id cannot be changed.
      parameters:
        - $ref: '#/components/parameters/IdPath'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                planned_date:
                  type: string
                assigned_to:
                  type: string
                  format: uuid
      responses:
        '200':
          description: Inventory task updated
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    $ref: '#/components/schemas/InventoryTask'
                  meta:
                    $ref: '#/components/schemas/Meta'
        '400':
          $ref: '#/components/responses/BadRequest'
        '404':
          $ref: '#/components/responses/NotFound'
```

### Go Service

```go
func (s *Service) Update(ctx context.Context, tenantID, taskID string, input UpdateInput) (*InventoryTask, error) {
    task, err := s.queries.GetInventoryTask(ctx, ...)
    if err != nil {
        return nil, ErrNotFound
    }
    if task.Status == "completed" {
        return nil, fmt.Errorf("cannot update completed task")
    }
    
    row, err := s.queries.UpdateInventoryTask(ctx, dbgen.UpdateInventoryTaskParams{
        ID:          uuid.MustParse(taskID),
        TenantID:    uuid.MustParse(tenantID),
        Name:        pgtype.Text{String: input.Name, Valid: input.Name != ""},
        PlannedDate: ...,
        AssignedTo:  ...,
    })
    return mapTask(row), err
}
```

### 前端变更

**API**: `update: (id, data) => apiClient.put('/inventory/tasks/${id}', data)`
**Hook**: `useUpdateInventoryTask()`
**UI**: 复用 `CreateInventoryTaskModal` 组件添加编辑模式

---

## 4. P1-7c: Inventory Task 软删除

### 业务规则

- 仅 `pending` 状态可删除
- `in_progress` 和 `completed` 任务不可删除（已有扫描记录）
- 软删除（同 Work Order 模式）

### SQL 查询

```sql
-- Migration
ALTER TABLE inventory_tasks ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_inventory_tasks_deleted_at ON inventory_tasks (deleted_at) WHERE deleted_at IS NULL;

-- name: SoftDeleteInventoryTask :exec
UPDATE inventory_tasks
SET deleted_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND status = 'pending' AND deleted_at IS NULL;

-- 修改 ListInventoryTasks: 添加 AND deleted_at IS NULL
```

### OpenAPI 变更

```yaml
    delete:
      operationId: deleteInventoryTask
      tags: [inventory]
      summary: Soft-delete an inventory task
      description: Only pending tasks can be deleted.
      parameters:
        - $ref: '#/components/parameters/IdPath'
      responses:
        '204':
          description: Inventory task deleted
        '400':
          description: Cannot delete task in current status
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '404':
          $ref: '#/components/responses/NotFound'
```

### 前端变更

**API**: `delete: (id) => apiClient.del('/inventory/tasks/${id}')`
**Hook**: `useDeleteInventoryTask()`
**UI**: 在 `HighSpeedInventory.tsx` 任务卡片上添加删除按钮（仅 pending 状态）

---

## 5. P1-6b: Work Order Location 过滤

### 现状

- `listWorkOrders` 参数: `page`, `page_size`, `status`, `asset_id`
- 缺少 `location_id` 参数
- Work Order 创建时已有 `location_id` 字段（OpenAPI line 872-874）
- 数据库 `work_orders` 表已有 `location_id` 列

### OpenAPI 变更

在 `/maintenance/orders` GET 添加参数：

```yaml
        - name: location_id
          in: query
          description: Filter by location (returns orders for this location and descendants)
          schema:
            type: string
            format: uuid
```

### SQL 变更

```sql
-- name: ListWorkOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::text IS NULL OR status = $2)
  AND ($3::uuid IS NULL OR asset_id = $3)
  AND ($4::uuid IS NULL OR location_id = $4)  -- 新增
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;
```

### Go Service 变更

```go
// List 方法签名扩展
func (s *Service) List(ctx context.Context, tenantID string, status *string, 
    locationID *string, limit, offset int) ([]WorkOrder, int, error)
```

### 前端变更

**`pages/MaintenanceHub.tsx`**:
```typescript
const { path } = useLocationContext()
const locationId = path.idc?.id || path.campus?.id || ...
const { data } = useWorkOrders(locationId ? { location_id: locationId } : undefined)
```

---

## 6. P1-6c: Inventory Task Location 过滤

### 现状

- `listInventoryTasks` 参数: `page`, `page_size`
- 缺少 location 过滤
- 创建时有 `scope_location_id` 字段
- 数据库已有 `scope_location_id` 列

### OpenAPI 变更

```yaml
        - name: scope_location_id
          in: query
          description: Filter tasks by scope location
          schema:
            type: string
            format: uuid
```

### SQL 变更

```sql
-- name: ListInventoryTasks :many
SELECT * FROM inventory_tasks
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::uuid IS NULL OR scope_location_id = $2)  -- 新增
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### 前端变更

**`pages/HighSpeedInventory.tsx`**:
```typescript
const { path } = useLocationContext()
const locationId = path.idc?.id || path.campus?.id || ...
const { data } = useInventoryTasks(locationId ? { scope_location_id: locationId } : undefined)
```

---

## Schema 变更汇总

### 数据库 Migration

```sql
-- 001_add_soft_delete.sql
ALTER TABLE work_orders ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE inventory_tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_work_orders_deleted_at 
  ON work_orders (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_tasks_deleted_at 
  ON inventory_tasks (deleted_at) WHERE deleted_at IS NULL;
```

### OpenAPI 新增 Operations

| Method | Path | OperationId | 描述 |
|--------|------|-------------|------|
| DELETE | `/maintenance/orders/{id}` | deleteWorkOrder | 软删除工单 |
| PUT | `/inventory/tasks/{id}` | updateInventoryTask | 更新盘点任务 |
| DELETE | `/inventory/tasks/{id}` | deleteInventoryTask | 软删除盘点任务 |

### OpenAPI 参数扩展

| Endpoint | 新参数 |
|----------|--------|
| `GET /maintenance/orders` | `location_id` (uuid, optional) |
| `GET /inventory/tasks` | `scope_location_id` (uuid, optional) |

### WebSocket

| 变更 | 描述 |
|------|------|
| Auth middleware | 从 `Sec-WebSocket-Protocol` header 读取 token |
| 前端 | 移除 URL 中的 `?token=` |
