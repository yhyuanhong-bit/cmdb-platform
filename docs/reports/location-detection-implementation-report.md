# SNMP/CDP 自動位置偵測：實作邏輯與改動路線報告

> 日期：2026-04-14
> 狀態：Phase 1-3 已完成，SNMP/CDP 資料管線待接入
> 目標：讓 CMDB 每 5 分鐘自動知道每台設備在哪，不依賴人工更新

---

## 一、核心邏輯

### 1.1 一句話

**交換機不動（錨點），伺服器插在交換機上。CDP 告訴我們誰插在哪，CMDB 告訴我們交換機在哪。兩者結合 = 自動知道伺服器在哪。**

### 1.2 原理圖

```
思科交換機（不動，位置已知）
  │
  ├─ CDP 協議自動廣播：
  │   Port 1 → SRV-001 (MAC: AA:BB:CC:11)
  │   Port 2 → SRV-002 (MAC: AA:BB:CC:22)
  │   Port 3 → SRV-003 (MAC: AA:BB:CC:33)
  │
  ├─ CMDB 已登記：
  │   Switch-A01 在 RACK-A01（3 樓）
  │
  └─ 推導：
      Port 1 的 SRV-001 在 RACK-A01
      Port 2 的 SRV-002 在 RACK-A01
      Port 3 的 SRV-003 在 RACK-A01
```

### 1.3 變更偵測邏輯

```
上一次掃描：SRV-001 在 Switch-A01 Port 1 → RACK-A01
這一次掃描：SRV-001 在 Switch-B01 Port 3 → RACK-B01

→ 結論：SRV-001 從 RACK-A01 搬到了 RACK-B01
→ 不需要任何人告訴系統，系統自己發現了
```

---

## 二、完整的 5 分鐘偵測循環

```
每 5 分鐘自動執行
  │
  ▼
┌─────────────────────────────────────────────────────┐
│ ① 資料採集（ingestion-engine，Python）                │
│                                                      │
│   SNMP 掃描所有思科交換機                              │
│   ├─ 讀 CDP 鄰居表（cdpCacheTable）                   │
│   │   → 每個端口連著的設備名稱、MAC、IP                 │
│   ├─ 讀 MAC 表（dot1dTpFdbTable）                     │
│   │   → 補充 CDP 沒覆蓋到的端口                        │
│   └─ 打包成 NATS 事件推送                              │
│       Subject: "mac_table.updated"                    │
│       Payload: [{switch_id, port, mac, vlan}, ...]    │
└────────────────────────┬────────────────────────────┘
                         │ NATS JetStream
                         ▼
┌─────────────────────────────────────────────────────┐
│ ② 資料更新（cmdb-core，Go）                           │
│                                                      │
│   訂閱 "mac_table.updated" 事件                       │
│   └─ UpdateMACCache()                                │
│       ├─ 每個 MAC → 查 switch_port_mapping            │
│       │   → 交換機端口 → 交換機的 rack_id → 實際機櫃   │
│       ├─ 每個 MAC → 查 assets.attributes.mac_address  │
│       │   → 匹配到 CMDB 中的哪台設備                   │
│       └─ 寫入 mac_address_cache 表                    │
└────────────────────────┬────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│ ③ 位置比對（cmdb-core）                               │
│                                                      │
│   CompareLocations()                                 │
│   對每台設備比較：                                      │
│     CMDB 記錄的 rack_id  vs  MAC 快取推算的 rack_id    │
│                                                      │
│   ┌──────────────────────────────────────────┐       │
│   │ 結果         │ 數量   │ 處理              │       │
│   ├──────────────┼───────┼─────────────────┤        │
│   │ consistent   │ ~95%  │ 靜默              │       │
│   │ relocated    │ ~3%   │ 有工單→自動確認    │       │
│   │              │       │ 沒工單→告警        │       │
│   │ missing      │ ~1%   │ 有維護工單→靜默    │       │
│   │              │       │ 沒工單→告警        │       │
│   │ new_device   │ ~1%   │ 建 Discovery +告警│       │
│   └──────────────┴───────┴─────────────────┘        │
└────────────────────────┬────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│ ④ 異常模式檢測                                        │
│                                                      │
│   ├─ 同一設備 30 天搬 3+ 次 → 頻繁搬遷告警            │
│   ├─ 凌晨 22:00-06:00 偵測到搬遷 → 非工作時間告警     │
│   └─ 同一機櫃 1 小時內 3+ 台消失 → 批量異常告警        │
└────────────────────────┬────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│ ⑤ 結果輸出                                           │
│                                                      │
│   ├─ 告警 → alert_events → WebSocket → 前端即時通知   │
│   ├─ CMDB 自動更新 → assets.rack_id（有工單的情況）    │
│   ├─ 位置歷史 → asset_location_history（所有變更）     │
│   ├─ 品質分數 → 位置不一致的資產一致性扣 50 分          │
│   ├─ 發現候選 → discovered_assets（未登記設備）         │
│   └─ 月報 → /location-detect/report API               │
└─────────────────────────────────────────────────────┘
```

---

## 三、已完成 vs 待完成

### 3.1 已完成（Phase 1-3）

| 元件 | 檔案 | 功能 |
|------|------|------|
| **資料庫** | migration 000032 | switch_port_mapping + mac_address_cache + asset_location_history 三張表 |
| **比對引擎** | location_detect/service.go | CompareLocations()：CMDB vs MAC 快取比對 |
| **MAC 快取更新** | location_detect/service.go | UpdateMACCache()：接收 MAC 條目，匹配資產，更新快取 |
| **位置歷史** | location_detect/service.go | RecordLocationChange() + GetLocationHistory() |
| **定時偵測** | location_detect/detector.go | 每 5 分鐘執行比對 + 自動確認/告警 |
| **異常偵測** | location_detect/anomaly.go | 頻繁搬遷 + 非工作時間 + 批量消失 |
| **8 個 API** | api/location_detect_endpoints.go + api/qr_endpoints.go | diffs/summary/anomalies/report/history + QR data/confirm |
| **QR 掃碼** | components/QRScanner.tsx | 前端相機掃碼 + 位置確認 API |
| **工單自動閉環** | detector.go | 偵測到搬遷 + 有工單 → 自動 completed |
| **品質分數聯動** | quality/service.go | MAC 位置不一致 → 一致性扣 50 分 |
| **未登記設備** | detector.go | 新 MAC → 自動建 Discovery 候選 |
| **盤點面板** | HighSpeedInventory.tsx | AutoDetectionPanel 顯示偵測統計 |
| **告警增強** | MonitoringAlerts.tsx | 位置驗證標記 |

### 3.2 待完成（資料管線）

**目前缺的就是「從交換機抓資料 → 推送到 cmdb-core」這一段。**

```
已完成的部分：
  mac_address_cache ──→ CompareLocations ──→ 告警/更新/歷史
       ↑
       │ UpdateMACCache()（接口已就緒，等資料進來）
       │
       │ ❌ 這裡是斷的
       │
  NATS 事件 "mac_table.updated"
       ↑
       │ ❌ 沒有人發布這個事件
       │
  ingestion-engine SNMP 掃描
       ↑
       │ ❌ CDP 鄰居表讀取未實作
       │
  思科交換機 CDP
```

### 3.3 需要改動的 3 個點

```
改動 1（Python ~60 行）：
  ingestion-engine 的 SNMP collector
  新增 CDP 鄰居表讀取 + MAC 表讀取
  掃描完成後發布 NATS 事件 "mac_table.updated"

改動 2（Go ~20 行）：
  cmdb-core 訂閱 "mac_table.updated"
  收到事件 → 呼叫 UpdateMACCache()

改動 3（前端 ~100 行）：
  系統設定頁新增「位置偵測」分頁
  ├─ 啟用/停用開關
  ├─ 狀態顯示（追蹤設備數、覆蓋率、上次掃描時間）
  └─ [立即掃描] 按鈕
```

---

## 四、改動路線

### 4.1 Phase 4：資料管線接入（1 週）

#### Task 4.1：SNMP Collector 擴展（Python）

**檔案**：`ingestion-engine/app/collectors/snmp.py`

**改動內容**：

```python
# 新增 CDP 鄰居表讀取
CDP_CACHE_TABLE = "1.3.6.1.4.1.9.9.23.1.2.1"
# cdpCacheDeviceId:   .1.3.6.1.4.1.9.9.23.1.2.1.1.6
# cdpCacheDevicePort: .1.3.6.1.4.1.9.9.23.1.2.1.1.7
# cdpCachePlatform:   .1.3.6.1.4.1.9.9.23.1.2.1.1.8
# cdpCacheAddress:    .1.3.6.1.4.1.9.9.23.1.2.1.1.4

async def collect_cdp_neighbors(self, target) -> list[dict]:
    """讀取 CDP 鄰居表，回傳每個端口連接的設備資訊"""
    # SNMP Walk cdpCacheTable
    # 解析：端口 → 設備名稱 + MAC + IP + 型號
    # 回傳 [{port_name, device_name, device_mac, device_ip, platform}]

async def collect_mac_table(self, target) -> list[dict]:
    """讀取 MAC 地址表"""
    # SNMP Walk dot1dTpFdbTable
    # 回傳 [{port_name, mac_address, vlan_id}]
```

#### Task 4.2：定時掃描 + NATS 推送（Python）

**檔案**：`ingestion-engine/app/tasks/mac_scan_task.py`（新建）

**改動內容**：

```python
async def periodic_mac_scan(tenant_id: str):
    """定時掃描所有交換機 MAC/CDP 表，推送到 NATS"""
    
    # 1. 查詢所有 type=network, sub_type=switch 的資產
    switches = await db.fetch(
        "SELECT id, attributes->>'management_ip' as ip FROM assets "
        "WHERE tenant_id=$1 AND type='network' AND deleted_at IS NULL", tenant_id)
    
    # 2. 取得 SNMP 憑證（掃描目標關聯的 credential）
    
    # 3. 對每台交換機：
    #    - collect_cdp_neighbors() → 設備鄰居資訊
    #    - collect_mac_table() → MAC 表
    #    - 合併去重
    
    # 4. 發布 NATS 事件
    await nats.publish("mac_table.updated", {
        "tenant_id": tenant_id,
        "switch_id": switch_id,
        "entries": [
            {"switch_asset_id": "uuid", "port_name": "Gi0/1", 
             "mac_address": "AA:BB:CC:11:22:33", "vlan_id": 100},
            ...
        ]
    })
```

**掃描觸發方式**：
- 自動：Celery Beat 每 5 分鐘觸發
- 手動：`POST /discovery/mac-scan` API

#### Task 4.3：cmdb-core 訂閱事件（Go）

**檔案**：`cmdb-core/cmd/server/main.go` + `location_detect/service.go`

**改動內容**：

```go
// main.go — 訂閱 MAC 表更新事件
if bus != nil && locationDetectSvc != nil {
    bus.Subscribe("mac_table.updated", func(ctx context.Context, event eventbus.Event) error {
        var payload struct {
            TenantID string     `json:"tenant_id"`
            Entries  []MACEntry `json:"entries"`
        }
        if err := json.Unmarshal(event.Payload, &payload); err != nil {
            return nil
        }
        tenantID, _ := uuid.Parse(payload.TenantID)
        return locationDetectSvc.UpdateMACCache(ctx, tenantID, payload.Entries)
    })
}
```

#### Task 4.4：前端管理頁面

**檔案**：`cmdb-demo/src/pages/SystemSettings.tsx`

**改動**：新增「位置偵測」分頁

```
┌─────────────────────────────────────────────────┐
│  系統設定                                        │
│                                                  │
│  [權限] [使用者] [整合器] [Webhook] [位置偵測]     │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │                                            │ │
│  │  位置偵測狀態                                │ │
│  │  ● 已啟用                                   │ │
│  │                                            │ │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────┐│ │
│  │  │追蹤設備 │ │ 覆蓋率  │ │搬遷(24h)│ │ 異常 ││ │
│  │  │  186   │ │ 93%   │ │   2    │ │  0  ││ │
│  │  └────────┘ └────────┘ └────────┘ └──────┘│ │
│  │                                            │ │
│  │  上次掃描：2 分鐘前                          │ │
│  │  交換機數量：8 台                            │ │
│  │  SNMP 憑證：DC-A SNMP v2c [已配置 ✓]        │ │
│  │                                            │ │
│  │  [立即掃描]  [查看偵測結果]  [下載月報]       │ │
│  │                                            │ │
│  └────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

---

## 五、前提條件

### 5.1 一次性準備（運維人員，約 15 分鐘）

| 步驟 | 操作 | 時間 |
|------|------|------|
| 1 | 確認交換機在 CMDB 中有登記（type=network, sub_type=switch）且 rack_id 正確 | 10 分鐘 |
| 2 | 在系統設定 → 整合器頁面配置 SNMP 憑證（community string） | 2 分鐘 |
| 3 | 配置掃描範圍（交換機管理 IP 的 CIDR） | 1 分鐘 |
| 4 | 點擊「啟用位置偵測」 | 1 秒 |

### 5.2 為什麼只需要登記交換機

```
交換機：8-12 台，基本不搬（錨點）→ 一次性登記
伺服器：上百台，會搬 → 系統自動追蹤（就是我們要解決的）
```

### 5.3 自動推導映射（不需要手動配端口）

```
系統自動完成：

CDP 說：Switch-A01 Port 1 連著 SRV-001（MAC: AA:BB:CC:11）
CMDB 說：Switch-A01 在 RACK-A01
→ 推導：SRV-001 在 RACK-A01

不需要人配「Port 1 → RACK-A01」
只需要 CMDB 裡交換機的位置正確就行
```

---

## 六、四種偵測結果的處理方式

### 6.1 位置一致（consistent）— 95% 的情況

```
CMDB: SRV-001 → RACK-A01
CDP:  SRV-001 → Switch-A01 → RACK-A01
→ 一致 → 靜默，什麼都不做
```

### 6.2 位置變更（relocated）— 有人搬了設備

```
CMDB: SRV-001 → RACK-A01
CDP:  SRV-001 → Switch-B01 → RACK-B01
→ 不一致！

  ├─ 有搬遷工單？
  │   └─ 是 → 自動處理：
  │       ├─ CMDB rack_id 更新為 RACK-B01
  │       ├─ 工單自動 completed
  │       ├─ 記錄位置歷史
  │       └─ 通知申請人
  │
  └─ 沒有工單？
      └─ 告警：「未授權搬遷」
          ├─ 通知 ops-admin + 機房管理員
          └─ 等人確認或建補工單
```

### 6.3 設備失聯（missing）— 設備從網路消失

```
CMDB: SRV-001 → RACK-A01
CDP:  SRV-001 在所有交換機上都找不到
→ 設備失聯！

  ├─ 有維護工單（設備在修）？ → 正常，靜默
  ├─ 有搬遷工單（在路上）？ → 正常，靜默
  └─ 什麼都沒有？ → 告警：「設備失聯」
```

### 6.4 陌生設備（new_device）— 未登記設備出現

```
CDP: Switch-A01 Port 4 連著未知 MAC: XX:XX:XX:XX:XX:XX
CMDB: 沒有這個 MAC 對應的資產
→ 未登記設備！

  → 告警：「未登記設備出現在 RACK-A01」
  → 自動建立 Discovery 候選（等人審批入庫）
  → 如果在 BIA Critical 機櫃 → severity=critical
```

---

## 七、與現有模組的整合關係

```
                    SNMP/CDP 掃描結果
                          │
          ┌───────────────┼───────────────────────┐
          ▼               ▼                       ▼
     mac_address     比對引擎                  異常偵測
     _cache 更新     CompareLocations()        DetectAnomalies()
          │               │                       │
    ┌─────┴────┐    ┌─────┴─────┐           ┌─────┴─────┐
    ▼          ▼    ▼           ▼           ▼           ▼
 資產管理   品質治理  維運工單    監控告警    盤點管理    合規稽核
                                                    
 rack_id   一致性   自動閉環   位置驗證    自動偵測    位置歷史
 自動更新  扣分/加分 搬遷工單   標記       面板       完整軌跡
                   completed                         月報
    │                  │          │          │          │
    ▼                  ▼          ▼          ▼          ▼
 BIA 分析          通知模組    WebSocket   Discovery  稽核日誌
 衝擊分析準確     ops-admin    即時推送    未登記審批  append-only
                  收到通知     前端更新
```

---

## 八、技術架構選擇

### 8.1 為什麼選方案 C（NATS 事件驅動）

| 對比項 | A: Go 直接 SNMP | B: HTTP 呼叫 | **C: NATS 推送** |
|--------|----------------|-------------|-----------------|
| 憑證管理 | 需要共享加密金鑰 | 不用 | **不用** |
| 能力重複 | Go 重寫 SNMP | 無 | **無** |
| 依賴關係 | cmdb-core 獨立 | 強依賴 ingestion | **鬆耦合** |
| 服務掛了 | 不影響 | 偵測停止 | **用快取繼續** |
| Edge 模式 | 可用 | 不可用 | **可用** |
| 符合現有架構 | 否 | 否 | **是（事件驅動）** |
| 改動量 | 大 | 中 | **小（~80 行）** |

### 8.2 為什麼用 CDP 而非 LLDP

| 對比項 | CDP | LLDP |
|--------|-----|------|
| 思科支援 | **1994 年起全系列支援** | 2009 年後才有 |
| 預設狀態 | **預設開啟** | 可能需要手動開啟 |
| 10 年以上老設備 | **100% 支援** | 不一定 |
| OID | `1.3.6.1.4.1.9.9.23` | `1.0.8802.1.1.2` |

### 8.3 降級策略

```
最優：CDP 鄰居表（思科設備，直接知道端口連的是誰）
  ↓ CDP 不可用時
次優：MAC 地址表（知道端口上有哪些 MAC，間接推斷）
  ↓ SNMP 不可用時
最後：QR 掃碼（人工掃碼確認，6 秒/台）
```

---

## 九、改動工作量估算

| 改動 | 位置 | 行數 | 時間 |
|------|------|------|------|
| SNMP collector 加 CDP 讀取 | ingestion-engine/collectors/snmp.py | ~40 行 | 2 小時 |
| MAC 掃描定時任務 + NATS 推送 | ingestion-engine/tasks/mac_scan_task.py | ~30 行 | 1 小時 |
| 手動掃描 API | ingestion-engine/routes/ | ~15 行 | 30 分鐘 |
| cmdb-core 訂閱 NATS 事件 | cmd/server/main.go | ~20 行 | 30 分鐘 |
| 前端位置偵測設定分頁 | cmdb-demo/src/pages/SystemSettings.tsx | ~100 行 | 2 小時 |
| **合計** | | **~205 行** | **~6 小時** |

---

## 十、運維人員操作手冊

### 10.1 首次啟用（一次性，15 分鐘）

```
步驟 1：確認交換機已在 CMDB 登記（10 分鐘）
  → 資產管理 → 篩選 type=network
  → 確認每台交換機的 rack_id 正確
  → 如果沒有，Excel 匯入批量登記

步驟 2：配置 SNMP 憑證（2 分鐘）
  → 系統設定 → 整合器 → 新增 SNMP adapter
  → 填入 community string（或 v3 帳密）

步驟 3：配置掃描範圍（1 分鐘）
  → 填入交換機管理 IP 的 CIDR 範圍
  → 例如 10.0.0.0/24

步驟 4：啟用（1 秒）
  → 系統設定 → 位置偵測 → 開啟
```

### 10.2 日常操作（什麼都不用做）

```
系統每 5 分鐘自動掃描
  → 99% 的時候結果是「全部一致」
  → 運維人員不需要任何操作
```

### 10.3 收到告警時（偶爾，1 分鐘處理）

```
手機/瀏覽器通知：「SRV-001 位置異動」
  → 打開 CMDB → 看差異詳情
  ├─ 確實搬了 → 點「確認」→ CMDB 自動更新
  └─ 誤報     → 點「忽略」
```

### 10.4 每季度校準（10 分鐘）

```
查看月報 → 位置準確率 98%+
抽查 2-3 個機櫃 → 確認偵測準確
```

---

## 十一、預期效果

| 指標 | 改造前 | 改造後 |
|------|--------|--------|
| 位置準確率 | ~70% | **>98%** |
| 位置異動發現時間 | 1-3 個月（下次盤點） | **5 分鐘** |
| 盤點人力 | 每月 2-3 人天 | **每季 0.5 人天** |
| 故障定位時間 | 15-30 分鐘找設備 | **<1 分鐘** |
| 未授權設備暴露時間 | 1-3 個月 | **5 分鐘** |
| 運維日常操作 | 手動更新 CMDB | **零操作** |
| CMDB 可信度 | 「參考就好」 | **「唯一真相」** |

---

## 十二、風險與緩解

| 風險 | 機率 | 緩解 |
|------|------|------|
| 交換機 SNMP 被防火牆擋 | 中 | 開放 cmdb 伺服器 → 交換機 161/udp |
| CDP 被管理員關閉 | 低 | 思科設備預設開啟；退回 MAC 表比對 |
| 首次推導不準（CMDB 位置本身不對） | 中 | 多數決校準 + 人工 10 分鐘確認 |
| NATS 故障 | 低 | JetStream 持久化，恢復後補發 |
| 虛擬機偵測不到 | 預期內 | VM 走 Hypervisor API，不走 SNMP |

---

## 十三、總結

### 改了什麼

```
ingestion-engine（Python）:
  + CDP 鄰居表讀取
  + MAC 表讀取
  + NATS 事件推送
  + 手動掃描 API
  = ~85 行新程式碼

cmdb-core（Go）:
  + 訂閱 NATS 事件 → UpdateMACCache()
  = ~20 行新程式碼
  （比對引擎、偵測器、告警、歷史、異常、API 已在 Phase 1-3 完成）

前端（TypeScript）:
  + 系統設定「位置偵測」分頁
  = ~100 行新程式碼

合計：~205 行，打通最後一環
```

### 為什麼值得做

```
投入：205 行程式碼 + 運維 15 分鐘初始化
產出：
  ├─ 位置準確率 70% → 98%
  ├─ 每月省 2.5 人天人力
  ├─ 故障定位快 95%
  ├─ 未授權設備 5 分鐘暴露
  └─ CMDB 從「參考」變「真相」
```
