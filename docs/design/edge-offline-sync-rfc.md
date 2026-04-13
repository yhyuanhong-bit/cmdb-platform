# RFC: Edge 離線同步系統設計

> 狀態：Approved（開放問題已全部決定）
> 作者：CMDB Platform Team
> 日期：2026-04-11
> 目標版本：v1.2.0

---

## 1. 背景與動機

### 1.1 問題描述

CMDB Platform 支援 Cloud（中央）和 Edge（邊緣）兩種部署模式。Edge 節點部署在分公司、遠端機房等網路不穩定的環境中。當 Edge 與 Central 之間網路中斷時：

- 現場人員無法查看/更新資產
- 盤點掃描結果無法上傳
- 告警無法同步到總部
- 工單流轉卡住
- 稽核記錄斷裂

### 1.2 目標

| 目標 | 描述 |
|------|------|
| 離線可用 | Edge 斷網後仍可完整讀寫所有 CMDB 功能 |
| 自動同步 | 恢復連線後自動同步增量變更 |
| 衝突解決 | 同一筆資料在離線期間被兩端修改時，有明確的解決策略 |
| 最終一致性 | 所有節點最終資料一致，允許短暫不一致 |
| 最小頻寬 | 只傳輸差異，不全量複製 |

### 1.3 非目標

- 即時強一致性（CAP 理論中選擇 AP）
- 跨 Edge 節點直接同步（所有同步經由 Central）
- Edge 節點自動部署/升級（獨立的 DevOps 議題）

---

## 2. 現有基礎設施分析

### 2.1 已具備的能力

| 能力 | 現狀 | 可利用程度 |
|------|------|-----------|
| NATS JetStream | Central 1GB / Edge 256MB，7 天訊息保留 | 高 — 直接作為同步傳輸層 |
| Leafnode 聯邦 | Edge NATS 配置為 Central 的 leafnode client | 高 — 斷線時本地排隊，恢復後自動推送 |
| 事件匯流排 | 15+ 事件主題，所有寫操作發布事件 | 高 — 變更事件已覆蓋所有模組 |
| 稽核軌跡 | `audit_events` 表記錄所有寫操作的 diff (JSONB) | 中 — 可作為變更日誌的補充 |
| `updated_at` 欄位 | 幾乎所有表都有 | 高 — 增量同步的基礎 |
| 軟刪除 | assets / work_orders / inventory_tasks 有 `deleted_at` | 中 — 需要同步刪除操作為 tombstone |
| 發現衝突處理 | `discovered_assets.diff_details` JSONB | 中 — 衝突 schema 可複用 |
| 租戶隔離 | 所有表有 `tenant_id`，Edge 強制單租戶 | 高 — 同步天然限定範圍 |

### 2.2 缺失的能力

| 能力 | 說明 |
|------|------|
| 變更向量時鐘 | 無法判斷兩端變更的因果順序 |
| 同步狀態追蹤 | 無 sync cursor/checkpoint 機制 |
| 衝突佇列 | 無人工介入的衝突解決 UI |
| 冪等套用 | 事件重複消費未做去重 |
| 批量差異查詢 | 無 `WHERE updated_at > $last_sync` 的專用端點 |

---

## 3. 架構設計

### 3.1 整體架構

```
┌─ Central (Cloud) ─────────────────────────────────┐
│                                                     │
│  ┌─────────┐   ┌──────────┐   ┌───────────────┐   │
│  │ cmdb-core│──▶│ NATS     │──▶│ Sync Service  │   │
│  │ (API)    │   │ JetStream│   │ (新增)         │   │
│  └─────────┘   │ Central  │   └───────┬───────┘   │
│                 │ Stream:  │           │            │
│                 │ CMDB     │     ┌─────▼─────┐     │
│                 └──┬───────┘     │ Conflict  │     │
│                    │ leafnode    │ Queue     │     │
│                    │             └───────────┘     │
└────────────────────┼──────────────────────────────┘
                     │ (可中斷)
┌────────────────────┼──────────────────────────────┐
│ Edge Node          │                               │
│                 ┌──▼───────┐                       │
│  ┌─────────┐   │ NATS     │   ┌───────────────┐   │
│  │ cmdb-core│──▶│ JetStream│──▶│ Sync Agent    │   │
│  │ (API)    │   │ Edge     │   │ (新增)         │   │
│  └─────────┘   │ Local Q  │   └───────────────┘   │
│                 └──────────┘                       │
│  ┌──────────┐                                      │
│  │ PostgreSQL│ ◀── 完整資料副本                      │
│  │ (Local)   │                                      │
│  └──────────┘                                      │
└────────────────────────────────────────────────────┘
```

### 3.2 核心元件

#### Sync Agent（Edge 側，新增）

運行在每個 Edge 節點上的後台服務，職責：
- 監聽本地 NATS 事件（本地變更）→ 標記為待同步
- 接收 Central 推送的遠端變更 → 套用到本地 DB
- 維護同步狀態（最後同步時間戳、pending 佇列）
- 偵測衝突並記錄

#### Sync Service（Central 側，新增）

運行在 Central 的後台服務，職責：
- 接收 Edge 推送的變更事件 → 套用到中央 DB
- 回應 Edge 的差異查詢（增量拉取）
- 管理衝突佇列
- 追蹤每個 Edge 節點的同步狀態

---

## 4. 同步協議設計

### 4.1 同步模式：推拉結合

```
正常連線時:
  Edge 寫入 → 本地 DB → NATS event → leafnode → Central NATS → Sync Service → Central DB
  Central 寫入 → Central DB → NATS event → leafnode → Edge NATS → Sync Agent → Edge DB

斷線時:
  Edge 寫入 → 本地 DB → NATS event → 本地 JetStream 佇列（最多 7 天）
  
恢復連線時:
  1. Edge NATS leafnode 自動重連
  2. 本地佇列中的事件自動推送到 Central
  3. Sync Agent 向 Central 拉取離線期間的增量變更
```

### 4.2 同步單元：Sync Envelope

所有同步訊息使用統一信封格式：

```go
type SyncEnvelope struct {
    ID          string    `json:"id"`           // 全局唯一 (UUID v7，含時間戳)
    Source      string    `json:"source"`       // "central" | "edge:{node_id}"
    TenantID    string    `json:"tenant_id"`
    EntityType  string    `json:"entity_type"`  // "asset" | "work_order" | ...
    EntityID    string    `json:"entity_id"`    // UUID
    Action      string    `json:"action"`       // "create" | "update" | "delete"
    Version     int64     `json:"version"`      // 遞增版本號
    Timestamp   time.Time `json:"timestamp"`    // 來源端的寫入時間
    Diff        json.RawMessage `json:"diff"`   // 欄位級差異
    Checksum    string    `json:"checksum"`     // SHA256(EntityID + Version + Diff)
}
```

### 4.3 版本控制：實體版本號

在需要同步的核心表上新增 `sync_version BIGINT DEFAULT 0` 欄位：

```sql
ALTER TABLE assets ADD COLUMN sync_version BIGINT DEFAULT 0;
ALTER TABLE work_orders ADD COLUMN sync_version BIGINT DEFAULT 0;
ALTER TABLE locations ADD COLUMN sync_version BIGINT DEFAULT 0;
ALTER TABLE racks ADD COLUMN sync_version BIGINT DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN sync_version BIGINT DEFAULT 0;
ALTER TABLE inventory_tasks ADD COLUMN sync_version BIGINT DEFAULT 0;
```

每次寫入時自動遞增：`sync_version = sync_version + 1`

### 4.4 同步狀態追蹤

新增 `sync_state` 表：

```sql
CREATE TABLE sync_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id VARCHAR(100) NOT NULL UNIQUE,  -- "central" | "edge:{hostname}"
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    entity_type VARCHAR(50) NOT NULL,      -- "asset" | "work_order" | ...
    last_sync_version BIGINT DEFAULT 0,    -- 該節點已同步到的版本
    last_sync_at TIMESTAMPTZ,
    status VARCHAR(20) DEFAULT 'active',   -- active | paused | error
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(node_id, entity_type)
);
```

### 4.5 增量拉取 API

新增同步專用端點：

```
GET /api/v1/sync/changes?entity_type=asset&since_version=1234&limit=100
```

回應：
```json
{
  "data": [
    {
      "entity_id": "uuid",
      "entity_type": "asset",
      "action": "update",
      "version": 1235,
      "diff": {"name": ["old", "new"], "status": ["operational", "maintenance"]},
      "timestamp": "2026-04-11T10:00:00Z"
    }
  ],
  "meta": {
    "has_more": true,
    "latest_version": 1300
  }
}
```

---

## 5. 衝突解決策略

### 5.1 衝突偵測

當 Sync Agent/Service 套用遠端變更時：
1. 比較 `sync_version`：如果本地版本 > 期望的基礎版本 → 衝突
2. 比較欄位級 diff：如果同一欄位被兩端修改 → 衝突

### 5.2 解決規則（分層策略）

| 優先級 | 策略 | 適用場景 |
|--------|------|---------|
| 1 | **Central Wins** | 安全/合規相關欄位（RBAC 角色、BIA 層級、告警規則） |
| 2 | **Last Write Wins (LWW)** | 一般欄位（資產名稱、描述、位置） |
| 3 | **Field-Level Merge** | 不衝突的欄位各自保留最新值 |
| 4 | **Manual Resolution** | 高風險衝突（工單狀態、盤點結果）進入衝突佇列 |

### 5.3 衝突佇列

```sql
CREATE TABLE sync_conflicts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    local_version BIGINT NOT NULL,
    remote_version BIGINT NOT NULL,
    local_diff JSONB NOT NULL,
    remote_diff JSONB NOT NULL,
    resolution VARCHAR(20) DEFAULT 'pending',  -- pending | local_wins | remote_wins | merged | dismissed
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### 5.4 衝突解決 UI

新增前端頁面 `/system/sync`：
- 待解決衝突列表
- 左右對照（本地 vs 遠端）
- 一鍵選擇：保留本地 / 接受遠端 / 手動合併
- 批量操作

---

## 6. 同步範圍定義

### 6.1 需要同步的實體（依優先級）

| 階段 | 實體 | 方向 | 衝突策略 |
|------|------|------|---------|
| Phase 1 | assets | 雙向 | Field-Level Merge |
| Phase 1 | locations | Central → Edge | Central Wins |
| Phase 1 | racks | 雙向 | Field-Level Merge |
| Phase 1 | rack_slots | 雙向 | LWW |
| Phase 2 | work_orders | 雙向 | 雙維度狀態模型（見 5.5） |
| Phase 2 | work_order_logs | Edge → Central | Append Only |
| Phase 2 | alert_events | 雙向 | LWW |
| Phase 2 | alert_rules | Central → Edge | Central Wins |
| Phase 3 | inventory_tasks | 雙向 | Manual |
| Phase 3 | inventory_items | Edge → Central | Append Only |
| Phase 3 | audit_events | Edge → Central | Append Only |

### 6.2 不需要同步的實體

| 實體 | 原因 |
|------|------|
| users / roles | Central 管理，Edge 啟動時全量拉取 |
| tenants | Edge 單租戶，啟動配置 |
| metrics (TimescaleDB) | 時序資料量大，Edge 本地保留即可 |
| webhook_subscriptions | Central 專有功能 |
| prediction_results | Central AI 服務產出，Edge 唯讀 |
| notifications | 各端獨立產生 |

---

## 7. 實作計劃

### 7.1 Phase 1：基礎同步框架（4 週）

**Week 1-2：資料層準備**
- [ ] 新增 migration：sync_version 欄位、sync_state 表、sync_conflicts 表
- [ ] 修改 domain service：寫入時自動遞增 sync_version
- [ ] 新增 sync changes API 端點（增量拉取）

**Week 3-4：Sync Agent & Service**
- [ ] 實作 Sync Agent（Edge 後台 goroutine）
- [ ] 實作 Sync Service（Central 後台 goroutine）
- [ ] NATS 同步通道：`sync.{tenant_id}.{entity_type}`
- [ ] 基礎衝突偵測（版本號比較）

**交付物**：assets + locations + racks 的雙向同步，LWW 衝突解決

### 7.2 Phase 2：維運同步（3 週）

- [ ] work_orders 同步 + 狀態機衝突處理
- [ ] alert_events 同步
- [ ] alert_rules 單向同步（Central → Edge）
- [ ] 衝突佇列 + 前端解決 UI

**交付物**：維運模組完整離線能力

### 7.3 Phase 3：盤點與稽核（2 週）

- [ ] inventory_tasks/items 同步
- [ ] audit_events 單向同步（Edge → Central）
- [ ] 同步監控面板（節點狀態、佇列深度、延遲）

**交付物**：全模組同步 + 運維監控

### 7.4 Phase 4：強化與測試（2 週）

- [ ] 壓力測試：模擬長時間斷線（7 天）後恢復
- [ ] 頻寬測試：量測增量同步的網路消耗
- [ ] 混沌測試：隨機斷線/恢復場景
- [ ] 文件完善

---

## 8. NATS 同步通道設計

### 8.1 新增 JetStream Stream

```
Stream: CMDB_SYNC
Subjects:
  sync.{tenant_id}.asset.>
  sync.{tenant_id}.location.>
  sync.{tenant_id}.rack.>
  sync.{tenant_id}.work_order.>
  sync.{tenant_id}.alert.>
  sync.{tenant_id}.inventory.>
  sync.{tenant_id}.audit.>
Retention: WorkQueue (消費後刪除)
MaxAge: 14 days (離線窗口上限，已確認)
Storage: File
Replicas: 1 (Edge) / 3 (Central)
```

### 8.2 Consumer 設計

每個 Edge 節點建立一個 durable consumer：

```
Consumer: edge-{node_id}
FilterSubject: sync.{tenant_id}.>
DeliverPolicy: Last per subject (避免重複)
AckPolicy: Explicit (確認套用成功後才 ACK)
MaxDeliver: 5 (重試上限)
AckWait: 30s
```

---

## 9. 離線窗口與資料量估算

### 9.1 假設條件

| 參數 | 值 |
|------|-----|
| 每日資產變更 | ~50 筆 |
| 每日工單操作 | ~20 筆 |
| 每日告警事件 | ~100 筆 |
| 每日盤點掃描 | ~200 筆 |
| 每筆 SyncEnvelope 大小 | ~500 bytes |
| 最大離線窗口 | 14 天 |

### 9.2 資料量

```
每日同步量 = (50 + 20 + 100 + 200) * 500 bytes = ~185 KB/天
14 天離線佇列 = 185 KB * 14 = ~2.6 MB
恢復同步時間 = 2.6 MB / 1 Mbps = ~20 秒
```

結論：離線同步的**資料量極小**，瓶頸在衝突解決而非頻寬。

---

## 10. 安全考量

| 項目 | 措施 |
|------|------|
| 傳輸加密 | NATS leafnode 啟用 TLS（已在 nats-edge.conf 預留） |
| 訊息簽名 | SyncEnvelope.Checksum 防止篡改 |
| 認證 | Edge 節點使用 NATS credentials 認證 |
| 租戶隔離 | 同步通道包含 tenant_id，無法跨租戶 |
| 防重播 | UUID v7 ID + 版本號去重 |
| 稽核 | 所有同步操作記錄到 audit_events |

---

## 11. 監控與告警

新增 Prometheus 指標：

```
cmdb_sync_pending_events{node_id, entity_type}     — 待同步事件數
cmdb_sync_conflicts_total{node_id, entity_type}     — 衝突總數
cmdb_sync_lag_seconds{node_id}                      — 同步延遲（秒）
cmdb_sync_last_success_timestamp{node_id}           — 最後成功同步時間
cmdb_sync_errors_total{node_id, error_type}         — 同步錯誤計數
```

告警規則：
- `cmdb_sync_lag_seconds > 3600` → Warning：同步延遲超過 1 小時
- `cmdb_sync_lag_seconds > 86400` → Critical：同步延遲超過 1 天
- `cmdb_sync_pending_events > 1000` → Warning：佇列積壓
- `cmdb_sync_errors_total` 增長速率 > 10/min → Critical：同步錯誤激增

---

## 12. 回滾計劃

| 階段 | 回滾方式 |
|------|---------|
| Migration | `migrate down` 移除 sync_version / sync_state / sync_conflicts 表 |
| Sync Agent | 停止 goroutine，Edge 回到獨立運作模式 |
| Sync Service | 停止 goroutine，Central 不再處理同步事件 |
| NATS Stream | 刪除 CMDB_SYNC stream |

回滾後 Edge 節點資料保持最後同步狀態，不會遺失本地資料。

---

## 13. 已決定的設計問題

### Q1：離線窗口上限 — 14 天

Edge NATS 256MB 儲存，14 天同步資料量約 2.6MB，綽綽有餘。多一倍容錯空間應對長假或災難場景。

### Q2：工單衝突 — 雙維度狀態模型

LWW 或 Manual 都不適合工單場景。採用**分離執行與治理**的雙維度模型：

```sql
ALTER TABLE work_orders ADD COLUMN execution_status VARCHAR(20) DEFAULT 'pending';
-- 值: pending → working → done
-- 控制端: Edge（現場人員）

ALTER TABLE work_orders ADD COLUMN governance_status VARCHAR(20) DEFAULT 'submitted';
-- 值: submitted → approved / rejected → verified
-- 控制端: Central（管理者）
```

| 維度 | 控制端 | 值 |
|------|--------|-----|
| execution_status | Edge | `pending → working → done` |
| governance_status | Central | `submitted → approved/rejected → verified` |

兩個維度獨立演進，不互相阻塞：
- Edge 現場可以直接開工、完成（操作 execution_status）
- Central 審批可以後補（操作 governance_status）
- 最終狀態 = 兩個維度的組合
- `execution=done` + `governance=rejected` → 觸發異常流程，通知 ops-admin

**零衝突**——兩端操作的是不同欄位。

### Q3：初始同步 — 自動快照 (方案 D)

Edge 首次啟動時完全自動，無需人工操作：

```
Edge 首次啟動
    │
    ▼
sync_state 為空？──是──▶ 請求 Central /api/v1/sync/snapshot
    │                         │
    否                        ▼
    │                    流式 JSON 分批寫入本地 DB
    ▼                    （每批 500 筆，帶交易）
正常增量同步                   │
                         完成 → 記錄 last_sync_version
                              │
                              ▼
                         自動切換到增量模式
```

- 全程 Edge API 返回 `503 Sync in progress`，防止髒讀
- 快照端點支援認證、gzip 壓縮、斷點續傳
- 典型耗時：~5 秒（5000 資產級別）
- Edge 部署只需 `docker compose up`，零 DBA 操作

### Q4：Edge → Edge 直接同步 — 不需要

- Edge 節點之間通常沒有直接網路連接
- 星型拓撲（所有同步經 Central）避免網狀衝突的指數級複雜度
- Central 高可用由 PostgreSQL replica + NATS cluster 保障
- 未來如有需求，可在兩個 Edge 之間加 NATS leafnode 連線（v2.0+）

### Q5：同步重試 — 分層有序佇列

#### 依賴層級定義

```
Layer 0（無依賴）: locations, users, roles, alert_rules
Layer 1（依賴 L0）: racks, assets
Layer 2（依賴 L1）: rack_slots, asset_dependencies, alert_events
Layer 3（依賴 L2）: work_orders, inventory_tasks
Layer 4（依賴 L3）: work_order_logs, inventory_items, audit_events
```

#### 處理規則

```
1. 按 Layer 順序處理：L0 全部完成 → L1 → L2 → ...
2. 同 Layer 內並行處理，無順序依賴
3. 某 Layer 有失敗 → 重試該 Layer（指數退避，最多 5 次）
4. 5 次仍失敗 → 標記失敗項為 error，其餘繼續進入下一層
5. error 項的下游自動標記 skipped
```

#### 重試策略

| 錯誤類型 | 處理方式 |
|---------|---------|
| 暫態錯誤（網路/鎖） | 指數退避：2s → 4s → 8s → 16s → 32s，共 5 次 |
| 永久錯誤（schema 不相容） | 標記 `error`，跳過，告警 ops-admin |
| 斷網 | NATS leafnode 自動排隊，不觸發告警 |
| 連線正常但同步失敗 | 區分斷網和錯誤，僅錯誤觸發告警 |
| 24 小時仍有 error 項 | 暫停該節點同步，標記節點狀態為 `error`，等人工介入 |

#### 效能影響

- 同步為後台 goroutine，不阻塞 API 請求
- 每次寫入多 ~0.1ms（sync_version 遞增，同一交易內）
- Layer 串行總耗時 ~5 秒（典型資料量），用戶無感

---

## 14. 參考資料

- [NATS JetStream Leafnodes](https://docs.nats.io/running-a-nats-service/configuration/leafnodes)
- [CRDTs and Conflict-Free Replicated Data Types](https://crdt.tech/)
- [Offline-First Design Patterns](https://offlinefirst.org/)
- Martin Kleppmann, *Designing Data-Intensive Applications*, Ch. 5: Replication
