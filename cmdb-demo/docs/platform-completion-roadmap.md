# CMDB 平台功能完善路線圖

> **基準日期**: 2026-04-04
> **當前完整度**: 72/100
> **目標完整度**: 95/100（生產就緒）

---

## 目錄

1. [第 1 週：安全合規基礎](#第-1-週安全合規基礎)
2. [第 2 週：CRUD 完整 + 數據填充](#第-2-週crud-完整--數據填充)
3. [第 3 週：事件驅動 + 即時推送](#第-3-週事件驅動--即時推送)
4. [第 4 週：驗收加固 + 清理](#第-4-週驗收加固--清理)

---

## 第 1 週：安全合規基礎

### 1.1 RBAC 權限執行中間件

**問題：** 任何登入用戶可執行所有操作，角色權限形同虛設。

**影響模組：** 所有 API 端點（40 個）

**現有基礎：**
- `roles` 表有 3 個角色，`permissions` 欄位存 JSON（如 `{"assets": ["read","write"]}`）
- `/auth/me` 回傳用戶的合併權限
- JWT claims 包含 `user_id`, `tenant_id`

**實現細節：**

| 步驟 | 文件 | 內容 |
|------|------|------|
| 1.1.1 | `cmdb-core/internal/middleware/rbac.go` | 新建 RBAC 中間件 |
| 1.1.2 | `cmdb-core/internal/middleware/rbac.go` | 路由 → 資源+動作 映射表 |
| 1.1.3 | `cmdb-core/cmd/server/main.go` | 在 auth 之後、handler 之前掛載 RBAC |
| 1.1.4 | `cmdb-core/db/seed/seed.sql` | 確認角色權限定義完整 |

**1.1.1 RBAC 中間件邏輯：**

```
請求進入
  → auth middleware 驗證 JWT → 得到 user_id, tenant_id
  → rbac middleware:
      1. 從 gin context 拿 user_id
      2. 查 Redis 快取：user_id → permissions（快取 5 分鐘）
         如果快取未命中：查 DB user_roles → roles → 合併 permissions → 寫快取
      3. 從請求的 Method + Path 推導出 resource + action：
         GET  /api/v1/assets      → resource="assets",  action="read"
         POST /api/v1/assets      → resource="assets",  action="write"
         PUT  /api/v1/assets/{id} → resource="assets",  action="write"
         DELETE /api/v1/assets/{id} → resource="assets", action="delete"
         POST /api/v1/maintenance/orders/{id}/transition → resource="maintenance", action="write"
      4. 檢查 permissions[resource] 是否包含 action
         如果包含 → 放行
         如果 permissions["*"] 包含 "*" → 放行（super-admin）
         否則 → 403 Forbidden
```

**1.1.2 路由 → 資源映射表：**

```go
var routeResourceMap = map[string]string{
    "/api/v1/assets":              "assets",
    "/api/v1/locations":           "topology",
    "/api/v1/racks":               "topology",
    "/api/v1/maintenance/orders":  "maintenance",
    "/api/v1/monitoring/alerts":   "monitoring",
    "/api/v1/monitoring/metrics":  "monitoring",
    "/api/v1/inventory/tasks":     "inventory",
    "/api/v1/audit/events":        "audit",
    "/api/v1/dashboard/stats":     "dashboard",
    "/api/v1/users":               "identity",
    "/api/v1/roles":               "identity",
    "/api/v1/prediction":          "prediction",
    "/api/v1/integration":         "integration",
    "/api/v1/system/health":       "system",
}

var methodActionMap = map[string]string{
    "GET":    "read",
    "POST":   "write",
    "PUT":    "write",
    "DELETE": "delete",
}
```

**1.1.3 免檢路由（公開端點）：**

```go
var publicPaths = map[string]bool{
    "/api/v1/auth/login":   true,
    "/api/v1/auth/refresh": true,
    "/healthz":             true,
    "/metrics":             true,
}
```

**1.1.4 角色權限定義補全：**

```sql
-- super-admin（已有）
UPDATE roles SET permissions = '{"*": ["*"]}' WHERE name = 'super-admin';

-- ops-admin：可讀寫資產、維護、監控，可讀拓撲、盤點、審計
UPDATE roles SET permissions = '{
  "assets": ["read", "write", "delete"],
  "maintenance": ["read", "write"],
  "monitoring": ["read", "write"],
  "topology": ["read"],
  "inventory": ["read", "write"],
  "audit": ["read"],
  "dashboard": ["read"],
  "prediction": ["read"]
}' WHERE name = 'ops-admin';

-- viewer：只能讀
UPDATE roles SET permissions = '{
  "assets": ["read"],
  "topology": ["read"],
  "maintenance": ["read"],
  "monitoring": ["read"],
  "inventory": ["read"],
  "audit": ["read"],
  "dashboard": ["read"]
}' WHERE name = 'viewer';
```

**驗收標準：**
- admin 帳號（super-admin）可以執行所有操作
- sarah.jenkins（ops-admin）可以建立工單，不能刪除角色
- mike.chen（viewer）只能查看，POST/PUT/DELETE 回傳 403

---

### 1.2 審計日誌自動記錄

**問題：** 所有寫操作（Create/Update/Delete/Transition）不記錄審計日誌。

**影響模組：** asset, maintenance, monitoring, inventory, identity, integration, prediction

**現有基礎：**
- `audit_events` 表 schema 完整
- `internal/domain/audit/service.go` 有 Query 方法
- `internal/eventbus/subjects.go` 定義了 `SubjectAuditRecorded`
- seed 有 10 條示範數據

**實現細節：**

| 步驟 | 文件 | 內容 |
|------|------|------|
| 1.2.1 | `cmdb-core/internal/domain/audit/service.go` | 加 Record() 方法 |
| 1.2.2 | `cmdb-core/internal/domain/audit/collector.go` | 新建：NATS 訂閱 → 異步批量寫入 |
| 1.2.3 | `cmdb-core/internal/api/impl.go` | 每個寫操作後調 audit |
| 1.2.4 | `cmdb-core/cmd/server/main.go` | 啟動 audit collector |

**1.2.1 Audit Service Record 方法：**

```go
func (s *Service) Record(ctx context.Context, params dbgen.CreateAuditEventParams) error {
    _, err := s.queries.CreateAuditEvent(ctx, params)
    return err
}
```

**1.2.2 Audit Collector（事件驅動方式）：**

```
方式 A（同步，簡單）：
  impl.go 每個寫方法最後直接調 auditSvc.Record()

方式 B（異步，推薦）：
  impl.go 寫操作後 → eventBus.Publish(audit event)
  collector goroutine 訂閱 audit.* → 批量每 100 條或每 5 秒 flush 到 DB
```

推薦方式 A 先上線（簡單可靠），後續再遷移到方式 B（高吞吐）。

**1.2.3 需要加審計的寫操作清單：**

| impl.go 方法 | audit action | module | target_type |
|--------------|-------------|--------|-------------|
| `CreateAsset` | asset.created | asset | asset |
| `TransitionWorkOrder` | order.transitioned | maintenance | work_order |
| `CreateWorkOrder` | order.created | maintenance | work_order |
| `AcknowledgeAlert` | alert.acknowledged | monitoring | alert |
| `ResolveAlert` | alert.resolved | monitoring | alert |
| `CreateRCA` | rca.created | prediction | rca |
| `VerifyRCA` | rca.verified | prediction | rca |

每個方法在成功後加：

```go
s.auditSvc.Record(ctx, dbgen.CreateAuditEventParams{
    TenantID:   tenantID,
    Action:     "asset.created",
    Module:     "asset",
    TargetType: "asset",
    TargetID:   newAsset.ID,
    OperatorID: userID,  // 從 gin context 取
    Diff:       diffJSON, // {"field": {"old": null, "new": value}}
    Source:     "web",
})
```

**驗收標準：**
- 建立一筆工單 → `GET /audit/events` 能看到新的 `order.created` 記錄
- Ack 一個告警 → 審計日誌出現 `alert.acknowledged`
- 審計頁面顯示即時操作記錄，不只是 seed 數據

---

## 第 2 週：CRUD 完整 + 數據填充

### 2.1 Asset Update/Delete 端點

**問題：** 前端 `assetApi.update()` 和 `assetApi.delete()` 調用會 404。

| 步驟 | 文件 | 內容 |
|------|------|------|
| 2.1.1 | `api/openapi.yaml` | 加 PUT /assets/{id} 和 DELETE /assets/{id} |
| 2.1.2 | 重新生成 | `make generate` 更新 Go + TS 型別 |
| 2.1.3 | `cmdb-core/internal/api/impl.go` | 實現 UpdateAsset 和 DeleteAsset |
| 2.1.4 | `cmdb-core/internal/domain/asset/service.go` | 加 Update 和 Delete 方法 |

**openapi.yaml 新增：**

```yaml
  /assets/{id}:
    put:
      operationId: updateAsset
      tags: [assets]
      parameters:
        - $ref: '#/components/parameters/IdPath'
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
                status: { type: string }
                bia_level: { type: string }
                location_id: { type: string, format: uuid, nullable: true }
                rack_id: { type: string, format: uuid, nullable: true }
                vendor: { type: string }
                model: { type: string }
                attributes: { type: object }
                tags: { type: array, items: { type: string } }
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  data: { $ref: '#/components/schemas/Asset' }
                  meta: { $ref: '#/components/schemas/Meta' }
    delete:
      operationId: deleteAsset
      tags: [assets]
      parameters:
        - $ref: '#/components/parameters/IdPath'
      responses:
        '204':
          description: Asset deleted
```

**sqlc 已有 UpdateAsset 和 DeleteAsset 查詢，無需新增。**

**驗收標準：**
- `PUT /assets/{id}` 可修改資產名稱、狀態、位置
- `DELETE /assets/{id}` 可刪除資產
- 兩個操作都觸發審計日誌

---

### 2.2 Alert Rules CRUD 端點

**問題：** 前端 `monitoringApi.listAlertRules()` 會 404，無法管理告警規則。

| 步驟 | 文件 | 內容 |
|------|------|------|
| 2.2.1 | `api/openapi.yaml` | 加 AlertRule schema + 2 個端點 |
| 2.2.2 | `db/queries/alert_rules.sql` | 新建查詢文件 |
| 2.2.3 | `internal/domain/monitoring/service.go` | 加 ListRules, CreateRule |
| 2.2.4 | `internal/api/impl.go` | 實現端點 |
| 2.2.5 | `db/seed/seed.sql` | 加 alert_rules seed |

**新端點：**

```
GET  /monitoring/rules       → 列出告警規則
POST /monitoring/rules       → 建立告警規則
```

**Seed 數據：**

```sql
INSERT INTO alert_rules (tenant_id, name, metric_name, condition, severity, enabled) VALUES
    ('a0000000-...', 'CPU High', 'cpu_usage', '{"op": ">", "threshold": 85}', 'warning', true),
    ('a0000000-...', 'CPU Critical', 'cpu_usage', '{"op": ">", "threshold": 95}', 'critical', true),
    ('a0000000-...', 'Temp High', 'temperature', '{"op": ">", "threshold": 40}', 'warning', true),
    ('a0000000-...', 'Disk Full', 'disk_usage', '{"op": ">", "threshold": 90}', 'critical', true),
    ('a0000000-...', 'Memory High', 'memory_usage', '{"op": ">", "threshold": 90}', 'warning', true);
```

---

### 2.3 Incidents CRUD 端點

**問題：** 前端 `monitoringApi.listIncidents()` 會 404。

| 步驟 | 文件 | 內容 |
|------|------|------|
| 2.3.1 | `api/openapi.yaml` | 加 Incident schema + 4 個端點 |
| 2.3.2 | `db/queries/incidents.sql` | 新建查詢文件 |
| 2.3.3 | `internal/domain/monitoring/service.go` | 加 incident 方法 |
| 2.3.4 | `internal/api/impl.go` | 實現 4 個端點 |
| 2.3.5 | `db/seed/seed.sql` | 加 incidents seed |

**新端點：**

```
GET    /monitoring/incidents        → 列出事件
POST   /monitoring/incidents        → 建立事件（歸併多個告警）
GET    /monitoring/incidents/{id}   → 事件詳情
PUT    /monitoring/incidents/{id}   → 更新事件（狀態、解決方案）
```

**Seed 數據：**

```sql
INSERT INTO incidents (tenant_id, title, status, severity, started_at) VALUES
    ('a0000000-...', 'Network Core Switch Failure', 'open', 'critical', now() - interval '2 hours'),
    ('a0000000-...', 'Storage Latency Degradation', 'investigating', 'warning', now() - interval '4 hours'),
    ('a0000000-...', 'UPS Battery Alert', 'resolved', 'warning', now() - interval '1 day');
```

---

### 2.4 空表數據填充

**12 張空表的補充策略：**

| 表 | 填充方式 | 數據量 | 依賴 |
|----|---------|--------|------|
| `departments` | seed.sql | 3 筆（運維部、網路部、機房管理） | tenants |
| `rack_slots` | seed.sql | 20 筆（20 個資產各佔 1-4U） | racks + assets |
| `alert_rules` | seed.sql | 5 筆（CPU/Temp/Disk/Memory/PUE 閾值） | tenants |
| `incidents` | seed.sql | 3 筆（各種狀態） | tenants |
| `metrics` | inject-metrics.py --backfill 24h | ~100K 筆 | assets |
| `prediction_models` | seed.sql | 2 筆（Dify RCA + Local Failure） | — |
| `prediction_results` | seed.sql | 5 筆（各種嚴重度） | prediction_models + assets |
| `rca_analyses` | seed.sql | 2 筆（一個已驗證、一個待驗證） | incidents + prediction_models |
| `inventory_items` | seed.sql | 10 筆（含 scanned/discrepancy 狀態） | inventory_tasks + assets + racks |
| `integration_adapters` | 已有 seed（Phase 6 加的） | 3 筆 | — |
| `webhook_subscriptions` | 已有 seed（Phase 6 加的） | 2 筆 | — |
| `webhook_deliveries` | seed.sql | 5 筆 | webhook_subscriptions |

**rack_slots seed 範例（關鍵 — 解鎖 RackDetail 可視化）：**

```sql
INSERT INTO rack_slots (rack_id, asset_id, start_u, end_u, side) VALUES
    -- RACK-A01: 2 servers + 1 PDU
    ('e0000000-...-000001', 'f0000000-...-000001', 1, 2, 'front'),   -- SRV-PROD-001 (2U)
    ('e0000000-...-000001', 'f0000000-...-000002', 3, 4, 'front'),   -- SRV-PROD-002 (2U)
    ('e0000000-...-000001', 'f0000000-...-000013', 40, 42, 'back'),  -- PDU-001 (3U)
    -- RACK-A02: 2 servers
    ('e0000000-...-000002', 'f0000000-...-000005', 1, 2, 'front'),   -- DB-001 (2U)
    ('e0000000-...-000002', 'f0000000-...-000006', 3, 4, 'front'),   -- APP-001 (2U)
    -- ... etc
```

**inventory_items seed 範例：**

```sql
INSERT INTO inventory_items (task_id, asset_id, rack_id, expected, actual, status) VALUES
    ('30000000-...-000001', 'f0000000-...-000001', 'e0000000-...-000001',
     '{"location": "RACK-A01 U1-2", "serial": "SN-DELL-001"}',
     '{"location": "RACK-A01 U1-2", "serial": "SN-DELL-001"}',
     'scanned'),
    ('30000000-...-000001', 'f0000000-...-000003', 'e0000000-...-000002',
     '{"location": "RACK-A02 U1-2", "serial": "SN-CISCO-001"}',
     '{"location": "RACK-B01 U5-6", "serial": "SN-CISCO-001"}',
     'discrepancy'),  -- 位置不符
```

**驗收標準：**
- `SELECT count(*) FROM [table]` 所有 24 張表非空
- RackDetail 頁面能看到設備在哪個 U 位
- InventoryItemDetail 能看到掃描結果和差異
- PredictiveHub 能看到預測結果和 RCA 分析

---

## 第 3 週：事件驅動 + 即時推送

### 3.1 寫操作發布 NATS 事件

**問題：** EventBus 基礎設施完整（NATS JetStream + 14 個主題常量），但沒有代碼在寫操作後發布事件。

**影響：**
- WebSocket 不推送（沒有事件源）
- Webhook 不觸發
- 審計 collector（方式 B）無法運作

| 步驟 | 文件 | 內容 |
|------|------|------|
| 3.1.1 | `internal/api/impl.go` | 每個寫操作後調 eventBus.Publish() |
| 3.1.2 | `internal/eventbus/bus.go` | 確認 Bus 接口在 APIServer 中可用 |

**需要發布事件的操作：**

| 操作 | 事件主題 | Payload |
|------|---------|---------|
| CreateAsset | `asset.created` | `{asset_id, tenant_id, asset_tag, type}` |
| UpdateAsset | `asset.updated` | `{asset_id, tenant_id, changed_fields}` |
| DeleteAsset | `asset.deleted` | `{asset_id, tenant_id}` |
| CreateWorkOrder | `maintenance.order_created` | `{order_id, tenant_id, code, priority}` |
| TransitionWorkOrder | `maintenance.order_transitioned` | `{order_id, from, to}` |
| AcknowledgeAlert | `alert.acknowledged` | `{alert_id, tenant_id}` |
| ResolveAlert | `alert.resolved` | `{alert_id, tenant_id}` |
| CreateRCA | `prediction.rca_created` | `{rca_id, incident_id}` |

**impl.go 模式：**

```go
func (s *APIServer) CreateWorkOrder(c *gin.Context) {
    // ... 建立工單 ...

    // 發布事件
    if s.eventBus != nil {
        payload, _ := json.Marshal(map[string]string{
            "order_id": order.ID.String(),
            "code":     order.Code,
            "priority": order.Priority,
        })
        s.eventBus.Publish(ctx, eventbus.Event{
            Subject:  eventbus.SubjectOrderCreated,
            TenantID: tenantID.String(),
            Payload:  payload,
        })
    }

    response.Created(c, toAPIWorkOrder(order))
}
```

---

### 3.2 前端 WebSocket 客戶端

**問題：** 後端 Hub 運行中，但前端沒有連接邏輯，頁面不消費即時事件。

| 步驟 | 文件 | 內容 |
|------|------|------|
| 3.2.1 | `cmdb-demo/src/hooks/useWebSocket.ts` | 新建 WS hook |
| 3.2.2 | `cmdb-demo/src/providers/WebSocketProvider.tsx` | 新建全局 WS 連接 provider |
| 3.2.3 | `cmdb-demo/src/pages/MonitoringAlerts.tsx` | 消費 alert.fired 事件即時更新列表 |
| 3.2.4 | `cmdb-demo/src/pages/Dashboard.tsx` | 消費事件即時更新統計 |

**useWebSocket hook：**

```typescript
function useWebSocket() {
  const token = useAuthStore(s => s.accessToken)
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!token) return
    const ws = new WebSocket(`ws://${location.host}/api/v1/ws?token=${token}`)

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      // 根據事件類型自動刷新對應的 React Query cache
      switch (msg.type) {
        case 'alert.fired':
        case 'alert.resolved':
          queryClient.invalidateQueries({ queryKey: ['alerts'] })
          queryClient.invalidateQueries({ queryKey: ['dashboardStats'] })
          break
        case 'asset.created':
        case 'asset.updated':
          queryClient.invalidateQueries({ queryKey: ['assets'] })
          break
        case 'maintenance.order_created':
        case 'maintenance.order_transitioned':
          queryClient.invalidateQueries({ queryKey: ['workOrders'] })
          break
      }
    }

    ws.onclose = () => setTimeout(() => /* reconnect */, 3000)

    return () => ws.close()
  }, [token])
}
```

**效果：**
- 運維人員 A 在台北確認告警 → 運維人員 B 在上海的 Dashboard 即時刷新
- 新資產從 Excel 導入 → 資產列表自動出現新資產
- 工單狀態變更 → 維護頁面即時更新

---

### 3.3 Webhook 投遞實現

**問題：** `webhook_subscriptions` 有配置，但沒有代碼在事件發生時投遞 Webhook。

| 步驟 | 文件 | 內容 |
|------|------|------|
| 3.3.1 | `internal/domain/integration/webhook_dispatcher.go` | 新建 dispatcher |
| 3.3.2 | `cmdb-core/cmd/server/main.go` | 訂閱事件 → 觸發 dispatcher |

**Webhook Dispatcher 邏輯：**

```
NATS 事件到達
  → 查 webhook_subscriptions 中 events 包含該事件類型的訂閱
  → 對每個訂閱：
      1. 構建 payload（事件內容 + 時間戳 + 簽名）
      2. POST 到 subscription.url
      3. 記錄結果到 webhook_deliveries（status_code, response_body）
      4. 如果失敗：重試 3 次（指數退避：1s, 5s, 30s）
```

---

## 第 4 週：驗收加固 + 清理

### 4.1 前端死代碼清理

**問題：** 前端定義了後端不存在的 API 函數，調用會 404。

| 文件 | 死代碼 | 處理 |
|------|--------|------|
| `src/lib/api/assets.ts` | `update()`, `delete()` | 第 2 週加了端點後變活碼 |
| `src/lib/api/monitoring.ts` | `listAlertRules()`, `createAlertRule()` | 第 2 週加了端點後變活碼 |
| `src/lib/api/monitoring.ts` | `listIncidents()`, `createIncident()` | 第 2 週加了端點後變活碼 |
| `src/lib/api/monitoring.ts` | `getIncident()`, `updateIncident()` | 第 2 週加了端點後變活碼 |
| `src/lib/api/monitoring.ts` | `ingestMetrics()` | 保留但標記為 ingestion-engine 專用 |

**結論：** 第 2 週完成 Alert Rules + Incidents 端點後，死代碼自動變為活碼。無需刪除。

---

### 4.2 MCP Server 認證

**問題：** MCP Server 在 port 3001 對外開放，任何人可查詢 CMDB 數據。

| 步驟 | 文件 | 內容 |
|------|------|------|
| 4.2.1 | `internal/mcp/server.go` | 加 API Key 驗證或 mTLS |
| 4.2.2 | `internal/config/config.go` | 加 MCP_API_KEY 配置項 |

**方案：** SSE 連接時驗證 `Authorization: Bearer <MCP_API_KEY>` header。

---

### 4.3 端到端冒煙測試腳本

| 步驟 | 文件 | 內容 |
|------|------|------|
| 4.3.1 | `scripts/smoke-test.sh` | Bash 腳本，測試核心業務流程 |

**測試內容：**

```bash
#!/bin/bash
# 1. 登入
TOKEN=$(curl -s -X POST .../auth/login ...)
assert_status 200

# 2. 列出資產
ASSETS=$(curl -s .../assets -H "Authorization: Bearer $TOKEN")
assert_count $(echo $ASSETS | jq '.pagination.total') -ge 20

# 3. 建立工單
ORDER=$(curl -s -X POST .../maintenance/orders -d '{"title":"Test","type":"inspection","priority":"low"}')
ORDER_ID=$(echo $ORDER | jq -r '.data.id')
assert_not_empty $ORDER_ID

# 4. 驗證審計日誌
AUDIT=$(curl -s ".../audit/events?target_id=$ORDER_ID")
assert_count $(echo $AUDIT | jq '.data | length') -ge 1

# 5. 狀態流轉
curl -s -X POST ".../maintenance/orders/$ORDER_ID/transition" -d '{"status":"pending"}'
assert_status 200

# 6. 列出告警
ALERTS=$(curl -s .../monitoring/alerts)
assert_count $(echo $ALERTS | jq '.pagination.total') -ge 1

# 7. 查指標
METRICS=$(curl -s ".../monitoring/metrics?asset_id=...&metric_name=cpu_usage&time_range=1h")
assert_count $(echo $METRICS | jq '.data | length') -ge 1

# 8. 查系統健康
HEALTH=$(curl -s .../system/health)
assert_field $(echo $HEALTH | jq -r '.data.database.status') "operational"

echo "All smoke tests passed!"
```

---

### 4.4 AssetDetail 頁面增強

**問題：** 多個詳情欄位使用 fallback 默認值（warranty, uptime, mtbf, cpu, memory 等）。

**方案：** 不加新端點。改用 `asset.attributes` JSONB 擴展欄位：

```sql
-- 在 seed 中給資產加 attributes
UPDATE assets SET attributes = '{
  "cpu": "2x Intel Xeon Gold 6348 (56C/112T)",
  "memory": "512GB DDR4-3200 ECC",
  "storage": "8x 3.84TB NVMe SSD (RAID-10)",
  "network": "4x 25GbE + 2x 100GbE",
  "os": "Rocky Linux 9.3",
  "primary_ip": "10.134.143.101",
  "mgmt_ip": "10.134.144.101",
  "form_factor": "2U Rackmount",
  "warranty_expiry": "2028-03-15",
  "purchase_date": "2025-03-15"
}' WHERE asset_tag = 'SRV-PROD-001';
```

AssetDetail 頁面已經從 `apiAsset.attributes.cpu` 讀取，只要 attributes 有數據就會顯示。

---

## 完成後的目標狀態

| 維度 | 當前 (72) | 第 1 週後 | 第 2 週後 | 第 3 週後 | 第 4 週後 (95) |
|------|-----------|----------|----------|----------|--------------|
| API 端點 | 40 | 40 | 48 (+8) | 48 | 48 |
| RBAC | 0% | 100% | 100% | 100% | 100% |
| 審計記錄 | 0% | 100% | 100% | 100% | 100% |
| DB 數據覆蓋 | 50% | 50% | 100% | 100% | 100% |
| CRUD 完整 | 65% | 65% | 95% | 95% | 95% |
| 事件驅動 | 0% | 0% | 0% | 100% | 100% |
| WebSocket | 0% | 0% | 0% | 100% | 100% |
| 冒煙測試 | 0% | 0% | 0% | 0% | 100% |
| **總分** | **72** | **80** | **88** | **93** | **95** |

---

## 附錄：按模組的完整任務清單

### Asset 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| A1 | PUT /assets/{id} 端點 | W2 | openapi + impl |
| A2 | DELETE /assets/{id} 端點 | W2 | openapi + impl |
| A3 | 建立/更新/刪除後寫審計日誌 | W1 | audit service |
| A4 | 建立/更新/刪除後發 NATS 事件 | W3 | eventBus |
| A5 | seed 補 attributes（CPU, memory, warranty 等） | W4 | seed.sql |

### Maintenance 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| M1 | 建立/流轉後寫審計日誌 | W1 | audit service |
| M2 | 建立/流轉後發 NATS 事件 | W3 | eventBus |
| M3 | PUT /maintenance/orders/{id} 端點（完整更新） | W2 | openapi + impl |
| M4 | GET /maintenance/orders/{id}/logs 端點 | W2 | openapi + impl |

### Monitoring 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| MO1 | Alert Rules CRUD（2 端點） | W2 | openapi + sqlc + impl |
| MO2 | Incidents CRUD（4 端點） | W2 | openapi + sqlc + impl |
| MO3 | Ack/Resolve 後寫審計日誌 | W1 | audit service |
| MO4 | Alert fired/resolved 發 NATS 事件 | W3 | eventBus |
| MO5 | Seed: alert_rules (5), incidents (3) | W2 | seed.sql |

### Inventory 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| I1 | Seed: inventory_items (10) | W2 | seed.sql |
| I2 | POST /inventory/tasks/{id}/items/{itemId}/scan 端點 | W2 | openapi + impl |
| I3 | POST /inventory/tasks/{id}/complete 端點 | W2 | openapi + impl |

### Prediction 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| P1 | Seed: prediction_models (2), prediction_results (5), rca_analyses (2) | W2 | seed.sql |
| P2 | CreateRCA/VerifyRCA 後寫審計日誌 | W1 | audit service |

### Topology 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| T1 | Seed: departments (3) | W2 | seed.sql |
| T2 | Seed: rack_slots (20) | W2 | seed.sql |

### Integration 模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| IG1 | Webhook dispatcher（事件 → HTTP 投遞） | W3 | NATS 訂閱 |
| IG2 | Seed: webhook_deliveries (5) | W2 | seed.sql |

### System / 跨模組

| # | 任務 | 週次 | 依賴 |
|---|------|------|------|
| S1 | RBAC middleware | W1 | middleware |
| S2 | MCP 認證（API Key） | W4 | config |
| S3 | 冒煙測試腳本 | W4 | 全部完成後 |
| S4 | WebSocket 前端客戶端 | W3 | useWebSocket hook |
| S5 | WebSocket Provider（全局連接） | W3 | Provider 組件 |

### 總計

| 週次 | 任務數 | 核心內容 |
|------|--------|---------|
| W1 | 8 | RBAC + Audit（安全合規） |
| W2 | 16 | CRUD 補全 + Seed 填充 |
| W3 | 8 | 事件驅動 + WebSocket + Webhook |
| W4 | 5 | 測試 + MCP 認證 + 清理 |
| **合計** | **37** | |
