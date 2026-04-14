# IronGrid CMDB 平台升級提案：SNMP 自動偵測驅動的全面改造

> 日期：2026-04-14
> 版本：v1.0
> 目標讀者：管理層決策 + 技術團隊執行

---

## 一、為什麼要做這件事

### 1.1 當前困境

CMDB 平台已建立 12 個業務模組、85+ API、50+ 頁面，但有一個根本問題：

> **資料準確性依賴人的自覺。人不更新，系統就是錯的。**

| 現狀 | 後果 |
|------|------|
| 設備搬遷後不更新 CMDB | 位置錯誤率 ~30% |
| 盤點靠人力，每月投入 2-3 人天 | 成本高、覆蓋率低 |
| 告警時不知道設備在哪 | 維修人員滿樓找，MTTR 長 |
| 容量規劃靠猜 | 有的機櫃以為滿了其實不滿 |
| 合規稽核前臨時抱佛腳 | 風險高 |

### 1.2 一個功能解決所有問題

**SNMP MAC 表自動偵測**——每 5 分鐘掃描所有交換機，自動知道每台設備在哪。

```
不是新增一個獨立功能
而是給整個平台裝上「眼睛」

之前：平台靠人「告訴」它設備在哪（被動）
之後：平台自己「看到」設備在哪（主動）
```

---

## 二、全平台價值鏈分析

### 2.1 一個資料源反哺 12 個模組

```
SNMP MAC 表（每 5 分鐘，全自動，零人力）
│
│  產出：每台設備的「實際網路位置」
│
├──→ ① 資產管理    位置自動準確，不靠人更新
├──→ ② 拓撲管理    機櫃佔用率即時準確，容量規劃有據
├──→ ③ 維運工單    搬遷工單自動閉環，不用人確認
├──→ ④ 監控告警    告警時直奔正確機櫃，MTTR 減半
├──→ ⑤ 盤點管理    從每月人工數 → 每 5 分鐘自動驗
├──→ ⑥ BIA 分析    關鍵設備位置可信，衝擊分析結果可靠
├──→ ⑦ 品質治理    一致性維度自動滿分
├──→ ⑧ 自動發現    未登記設備即時暴露（新 MAC 出現）
├──→ ⑨ 能源管理    每機櫃實際設備數準確，功耗分攤精準
├──→ ⑩ Edge 同步   遠端位置變更即時同步到總部
├──→ ⑪ 合規稽核    資產清冊隨時可查且準確
└──→ ⑫ 預測分析    AI 有正確的環境上下文
```

### 2.2 量化效益

| 指標 | 當前 | 改善後 | 提升幅度 |
|------|------|--------|---------|
| 位置準確率 | ~70% | >98% | +40% |
| 盤點人力投入 | 每月 2-3 人天 | 每季 0.5 人天 | **節省 90%** |
| 故障定位時間 | 15-30 分鐘 | <1 分鐘 | **縮短 95%** |
| 未授權設備發現時間 | 1-3 個月（下次盤點） | 5 分鐘 | **快 1 萬倍** |
| 合規稽核準備時間 | 2 週突擊 | 0（隨時可查） | **100% 節省** |
| 容量規劃準確度 | 靠猜 | 即時精準 | **避免重複採購** |
| CMDB 整體可信度 | 「參考就好」 | 「唯一真相」 | **質變** |

### 2.3 成本

| 項目 | 費用 |
|------|------|
| 硬體 | 0 元（交換機已有 SNMP） |
| QR 標籤列印機 | ~3,000 元 |
| QR 標籤紙 | ~250 元 |
| 開發 | 已有團隊，見改造計劃 |
| **總計** | **~3,250 元** |

**投入 3,250 元，每月節省 2-3 人天人力 + 系統可信度質變。**

---

## 三、平台改造計劃

### 3.1 改造總覽

```
Phase 1（2 週）          Phase 2（2 週）          Phase 3（1 週）
┌────────────────┐    ┌────────────────┐    ┌────────────────┐
│ 基礎設施層      │    │ 業務整合層      │    │ 智能化層       │
│                │    │                │    │                │
│ MAC 表採集     │    │ 工單自動閉環    │    │ 位置預測       │
│ 位置比對引擎    │    │ 盤點模式改造    │    │ 異常模式學習    │
│ 交換機-機櫃映射 │    │ 告警位置增強    │    │ 報表輸出       │
│ QR 掃碼系統    │    │ 品質分數聯動    │    │                │
└────────────────┘    └────────────────┘    └────────────────┘
```

### 3.2 Phase 1：基礎設施層（第 1-2 週）

#### Task 1.1：交換機-機櫃映射表

**目的**：建立「交換機端口 → 機櫃位置」的對應關係。

```sql
-- 新增 migration
CREATE TABLE switch_port_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    switch_asset_id UUID NOT NULL REFERENCES assets(id),  -- 交換機本身也是 CMDB 資產
    port_name VARCHAR(50) NOT NULL,                        -- "GigabitEthernet0/1"
    connected_rack_id UUID REFERENCES racks(id),           -- 這個端口連到哪個機櫃
    connected_u_position INT,                              -- 連到哪個 U 位（可選）
    description TEXT,
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(tenant_id, switch_asset_id, port_name)
);

CREATE INDEX idx_switch_port_mapping_rack ON switch_port_mapping(connected_rack_id);
```

**資料來源**：
- 初始：人工配置或從網管系統匯入
- 後續：LLDP/CDP 協議自動維護

#### Task 1.2：MAC 表採集服務

**目的**：定期從所有交換機抓取 MAC 表。

```go
// internal/domain/location_detect/collector.go

type MACTableEntry struct {
    SwitchAssetID uuid.UUID
    PortName      string
    MACAddress    string
    VLAN          int
    CollectedAt   time.Time
}

// CollectMACTables 掃描所有交換機的 MAC 表
func (s *Service) CollectMACTables(ctx context.Context, tenantID uuid.UUID) ([]MACTableEntry, error) {
    // 1. 查詢所有 type='network' 且 sub_type='switch' 的資產
    // 2. 取得每台交換機的 SNMP 憑證
    // 3. SNMP GET ifTable + dot1dTpFdbTable
    // 4. 回傳所有 MAC-port 對應
}
```

**SNMP OID**：
- MAC 表：`1.3.6.1.2.1.17.4.3.1.2` (dot1dTpFdbPort)
- 端口名稱：`1.3.6.1.2.1.31.1.1.1.1` (ifName)
- LLDP 鄰居：`1.0.8802.1.1.2.1.4` (lldpRemTable)

#### Task 1.3：位置比對引擎

**目的**：比對 CMDB 記錄 vs MAC 表實際位置，產出差異。

```go
// internal/domain/location_detect/comparator.go

type LocationDiff struct {
    AssetID       uuid.UUID
    AssetTag      string
    MACAddress    string
    CMDBRackID    uuid.UUID   // CMDB 記錄的位置
    CMDBRackName  string
    ActualRackID  uuid.UUID   // MAC 表推算的位置
    ActualRackName string
    DiffType      string     // "relocated" | "missing" | "new_device" | "consistent"
    RelocationWO  *uuid.UUID // 對應的搬遷工單（如有）
    DetectedAt    time.Time
}

func (s *Service) CompareLocations(ctx context.Context, tenantID uuid.UUID) ([]LocationDiff, error) {
    // 1. 採集 MAC 表
    // 2. 查 CMDB 中每台設備的 MAC 地址（assets.attributes->'mac_address'）
    // 3. 透過 switch_port_mapping 將 MAC 的交換機端口轉換為機櫃位置
    // 4. 與 CMDB 記錄比對
    // 5. 產出差異列表
}
```

**比對邏輯**：

```
對每個已知 MAC 地址：
  │
  ├─ CMDB 機櫃 == MAC 表推算機櫃 → "consistent"（一致）
  │
  ├─ CMDB 機櫃 != MAC 表推算機櫃
  │   ├─ 有搬遷工單 → "relocated"（已授權搬遷）
  │   └─ 無搬遷工單 → "relocated"（未授權搬遷）⚠️
  │
  ├─ CMDB 有記錄但 MAC 表找不到 → "missing"（設備失聯）🔴
  │
  └─ MAC 表有但 CMDB 沒記錄 → "new_device"（未登記設備）🔴
```

#### Task 1.4：定時任務 + 告警

```go
// 在 WorkflowSubscriber 中新增
func (w *WorkflowSubscriber) StartLocationDetection(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    go func() {
        for {
            select {
            case <-ctx.Done():
                ticker.Stop()
                return
            case <-ticker.C:
                w.runLocationDetection(ctx)
            }
        }
    }()
}

func (w *WorkflowSubscriber) runLocationDetection(ctx context.Context) {
    diffs := locationDetectSvc.CompareLocations(ctx, tenantID)
    
    for _, diff := range diffs {
        switch diff.DiffType {
        case "relocated":
            if diff.RelocationWO != nil {
                // 有工單 → 自動確認搬遷，更新 CMDB
                updateAssetLocation(diff.AssetID, diff.ActualRackID)
                closeWorkOrder(diff.RelocationWO)
            } else {
                // 無工單 → 告警
                createAlert("unauthorized_relocation", diff)
                notifyOpsAdmins(diff)
            }
        case "missing":
            createAlert("device_missing", diff)
        case "new_device":
            createDiscoveryCandidate(diff)
        }
    }
}
```

#### Task 1.5：QR 掃碼系統

**前端**：
```typescript
// 新增 /cmdb-demo/src/components/QRScanner.tsx
// 使用 html5-qrcode 函式庫

function QRScanner({ onScan }: { onScan: (data: QRData) => void }) {
  // 啟動相機 → 掃描 QR → 解碼 → 回調
}

// QR 內容格式
type QRData = 
  | { t: 'rack', id: string, name: string, loc: string }
  | { t: 'asset', tag: string, sn: string }
```

**後端**：
```
POST /api/v1/assets/{id}/confirm-location
{
  "rack_id": "uuid-of-new-rack",
  "source": "qr_scan",
  "scanned_by": "user-uuid"
}
```

**QR 生成**：
```
GET /api/v1/assets/{id}/qr-code → 回傳 SVG/PNG
GET /api/v1/racks/{id}/qr-code → 回傳 SVG/PNG
POST /api/v1/assets/batch-qr → 回傳 PDF（多標籤列印）
```

### 3.3 Phase 2：業務整合層（第 3-4 週）

#### Task 2.1：工單自動閉環

```
搬遷工單 type=relocation
  │
  └─ 目標位置：RACK-B03
      │
      ▼
  5 分鐘後 MAC 表偵測到設備出現在 RACK-B03
      │
      ▼
  自動：
  ├─ 更新 assets.rack_id = RACK-B03
  ├─ 更新 rack_slots（釋放舊、分配新）
  ├─ 工單狀態 → completed
  ├─ 稽核記錄：location_auto_confirmed, source=snmp_mac
  └─ 通知申請人：搬遷已自動確認完成
```

**修改檔案**：
- `internal/domain/maintenance/service.go` — 新增 `type=relocation` 處理
- `internal/domain/workflows/subscriber.go` — 位置偵測結果觸發工單閉環

#### Task 2.2：盤點模式改造

從「人工盤點為主」改為「自動偵測為主，人工為輔」：

```
舊模式（每月）：
  建立任務 → 匯入 Excel → 現場掃描 → 處理差異 → 完成
  人力：2-3 人天

新模式（持續 + 每季度校準）：

  持續（自動）：
  ┌─────────────────────────────────────────┐
  │ SNMP 每 5 分鐘 → 位置比對 → 差異自動處理  │
  │ 覆蓋 95% 設備                            │
  │ 人力：零                                  │
  └─────────────────────────────────────────┘

  每季度（人工校準）：
  ┌─────────────────────────────────────────┐
  │ 隨機抽查 10% 機櫃                        │
  │ 目的：驗證自動偵測準確性                   │
  │ 人力：0.5 人天                            │
  └─────────────────────────────────────────┘
```

**前端改造**：
- 盤點頁面新增「自動偵測結果」分頁
- 顯示最近 24h/7d/30d 的位置變化歷史
- 按機櫃顯示一致/不一致狀態

#### Task 2.3：告警位置增強

```
現在：
  告警：伺服器 SRV-001 CPU 溫度過高
  維修人員：SRV-001 在哪？→ 查 CMDB → 可能是錯的

改造後：
  告警：伺服器 SRV-001 CPU 溫度過高
       位置：3F / Room-4A / RACK-A01 / U12-14（即時驗證：✓ 已確認）
  維修人員：直奔 3 樓 RACK-A01
```

**修改**：
- 告警 API 回傳中增加 `location_verified: true/false`
- 前端告警卡片顯示驗證狀態
- 位置未驗證的告警標記警告

#### Task 2.4：品質分數聯動

```
品質掃描引擎擴展：

一致性維度新增規則：
  IF asset.rack_id != location_detect.actual_rack_id
  THEN consistency_score -= 50
  
  IF asset.mac_address NOT IN recent_mac_table
  THEN consistency_score -= 30（設備可能已不存在）
```

**修改**：
- `internal/domain/quality/service.go` — `evaluateAsset` 增加位置一致性規則
- 需要傳入最近的 MAC 表比對結果

#### Task 2.5：安全增強 — 未授權設備偵測

```
MAC 表中出現新 MAC
  │
  ├─ CMDB 中無對應資產
  │
  ▼
自動：
  ├─ 建立 Discovery candidate（staging）
  ├─ 告警：severity=warning「未登記設備出現在 RACK-B03」
  ├─ 通知機房管理員 + 安全團隊
  └─ 如果在 BIA Critical 機櫃 → severity=critical
```

### 3.4 Phase 3：智能化層（第 5 週）

#### Task 3.1：位置變更歷史與趨勢

```sql
-- 位置變更歷史表
CREATE TABLE asset_location_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    asset_id UUID NOT NULL REFERENCES assets(id),
    from_rack_id UUID REFERENCES racks(id),
    to_rack_id UUID REFERENCES racks(id),
    detected_by VARCHAR(20) NOT NULL,  -- 'snmp' | 'qr_scan' | 'manual' | 'import'
    work_order_id UUID REFERENCES work_orders(id),
    detected_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_location_history_asset ON asset_location_history(asset_id, detected_at DESC);
```

#### Task 3.2：異常模式識別

```
分析歷史資料，識別異常模式：

模式 1：頻繁搬遷
  設備 A 在 30 天內搬了 3 次 → 告警：可能是臨時借調未歸還

模式 2：夜間搬遷
  凌晨 2 點偵測到位置變化 → 告警：非工作時間搬遷

模式 3：跨區域搬遷
  設備從 3 樓搬到 5 樓 → 正常
  設備從台北搬到高雄 → 告警：重大搬遷，檢查是否有審批

模式 4：批量消失
  同一機櫃 5 台設備同時消失 → 告警：可能是機櫃級故障或搬遷
```

#### Task 3.3：管理報表

```
月度報表自動生成：

┌─────────────────────────────────────────────┐
│           資產位置治理月報 — 2026 年 4 月       │
├─────────────────────────────────────────────┤
│                                              │
│  位置準確率：98.5%（上月 97.2% ↑1.3%）        │
│                                              │
│  本月位置變更：23 次                           │
│  ├─ 授權搬遷（有工單）：18 次                   │
│  ├─ 未授權搬遷（自動偵測）：3 次                 │
│  └─ 自動修正：2 次                             │
│                                              │
│  未登記設備發現：2 台（已處理）                  │
│  設備失聯事件：1 次（已確認報廢）                │
│                                              │
│  盤點人力投入：0.5 人天（季度校準）               │
│  節省人力：2.5 人天/月                          │
│                                              │
│  自動偵測準確率：99.1%（季度校準結果）            │
└─────────────────────────────────────────────┘
```

---

## 四、改造涉及的檔案清單

### 新增檔案

| 檔案 | 用途 |
|------|------|
| `db/migrations/000030_location_detect.up.sql` | switch_port_mapping + asset_location_history 表 |
| `internal/domain/location_detect/collector.go` | SNMP MAC 表採集 |
| `internal/domain/location_detect/comparator.go` | 位置比對引擎 |
| `internal/domain/location_detect/service.go` | 偵測服務主邏輯 |
| `internal/api/location_detect_endpoints.go` | 偵測結果 API |
| `internal/api/qr_endpoints.go` | QR 生成/掃碼 API |
| `cmdb-demo/src/components/QRScanner.tsx` | QR 掃碼元件 |
| `cmdb-demo/src/pages/LocationDetection.tsx` | 位置偵測儀表板 |
| `cmdb-demo/src/hooks/useLocationDetect.ts` | 偵測結果 hook |

### 修改檔案

| 檔案 | 改動 |
|------|------|
| `internal/domain/workflows/subscriber.go` | 新增 StartLocationDetection 定時任務 |
| `internal/domain/maintenance/service.go` | 搬遷工單自動閉環 |
| `internal/domain/quality/service.go` | 一致性規則增加位置驗證 |
| `internal/domain/monitoring/service.go` | 告警附加位置驗證狀態 |
| `cmd/server/main.go` | 初始化位置偵測服務 |
| `cmdb-demo/src/pages/HighSpeedInventory.tsx` | 新增自動偵測結果分頁 |
| `cmdb-demo/src/pages/MonitoringAlerts.tsx` | 告警卡片增加位置驗證 |
| `cmdb-demo/src/pages/RackDetailUnified.tsx` | 機櫃詳情增加偵測狀態 |

---

## 五、實施時間線

```
Week 1                    Week 2                    Week 3
├─ 交換機映射表           ├─ 位置比對引擎            ├─ 工單自動閉環
├─ MAC 表採集服務         ├─ 定時任務 + 告警          ├─ 告警位置增強
├─ 資料庫 migration       ├─ QR 掃碼前端             ├─ 品質分數聯動
└─ 交換機端口映射初始化    └─ QR 生成 API             └─ 安全：未授權偵測

Week 4                    Week 5
├─ 盤點模式改造           ├─ 位置歷史 + 趨勢
├─ 自動偵測結果頁面       ├─ 異常模式識別
├─ 前端整合               ├─ 管理報表
└─ 整合測試               └─ 文件 + 培訓
```

---

## 六、風險與緩解

| 風險 | 機率 | 緩解 |
|------|------|------|
| 交換機不支援 SNMP | 低 | 企業級交換機 99% 支援；不支援的用 QR 掃碼覆蓋 |
| MAC 表太大影響效能 | 低 | 每台交換機幾百條 MAC，5 分鐘掃一輪很輕鬆 |
| 端口映射初始化工作量大 | 中 | 用 LLDP/CDP 自動發現鄰居關係，減少手動配置 |
| VLAN 導致 MAC 表不準 | 中 | 採集時記錄 VLAN ID，按 VLAN 分組比對 |
| 虛擬機 MAC 地址動態變化 | 中 | 虛擬機不在 MAC 偵測範圍（由 Hypervisor API 管理） |

---

## 七、總結

### 核心觀點

```
投入                        產出

3,250 元硬體         →    位置準確率 70% → 98%
5 週開發             →    盤點人力節省 90%
                    →    故障定位時間縮短 95%
                    →    未授權設備 5 分鐘暴露
                    →    合規稽核隨時可查
                    →    CMDB 從「參考」變「真相」
```

### 三句話給老闆

1. **花 3,250 元買設備 + 5 週開發，每月省 2.5 人天人力，一年省 30 人天。**

2. **不是新建一個功能，是讓已有的 12 個模組全部從「靠人填資料」升級為「系統自動準確」。**

3. **這是 CMDB 從「有用但不可信」變成「唯一權威真相」的最後一塊拼圖。**
