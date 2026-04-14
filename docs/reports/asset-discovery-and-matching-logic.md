# 資產發現、更新與交叉匹配完整邏輯

> 日期：2026-04-14
> 適用：IronGrid CMDB Platform

---

## 一、資產從哪來

```
┌─────────┐  ┌──────────┐  ┌──────────┐  ┌────────────────┐
│ 手動錄入  │  │ Excel 匯入│  │ SNMP/SSH │  │ IPMI 帶外掃描  │
│ 前端表單  │  │ 批量匯入  │  │ 自動發現  │  │ 硬體資訊       │
└────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬─────────┘
     │             │             │               │
     ▼             ▼             ▼               ▼
┌────────────────────────────────────────────────────────────┐
│              Ingestion Pipeline（統一入口）                  │
│                                                            │
│  ① 正規化 → ② 去重比對 → ③ 權威檢查 → ④ 入庫/衝突         │
└────────────────────────────────────────────────────────────┘
```

另有一條**位置偵測路徑**，不建立新資產，只更新位置：

```
┌────────────────┐
│ SNMP MAC 表     │
│ CDP 鄰居表      │
│ 交換機每5分鐘掃描│
└──────┬─────────┘
       ▼
  偵測位置變化
  → 不走 pipeline
  → 只更新 rack_id
```

---

## 二、四個來源的詳細路徑

### 來源 1：手動錄入

```
用戶在前端填表單
  │
  ▼
POST /api/v1/assets
  │
  ▼
品質閘門檢查（分數 < 40 → 拒絕建立，HTTP 422）
  │
  ▼
API 權威檢查（更新時：低優先級來源無法覆蓋高優先級欄位）
  │
  ▼
寫入 assets 表
  │
  ▼
稽核記錄 + 發布事件 asset.created → NATS
  │
  ▼
WebSocket 推送 → 前端即時更新
通知 ops-admin
```

**特點**：
- 直接寫 CMDB，不經過 ingestion pipeline
- 只靠 asset_tag UNIQUE 約束防重複
- 受品質閘門保護（完整性/準確性檢查）
- 更新時受權威矩陣約束

### 來源 2：Excel 批量匯入

```
用戶上傳 Excel/CSV
  │
  ▼
前端解析（支援中英文表頭）
  ├─ asset_tag / 資產編號 / Asset Tag
  ├─ serial_number / 序號 / Serial Number
  ├─ property_number / 財產編號 / Property Number
  ├─ control_number / 管制編號 / Control Number
  └─ expected_location / 預期位置 / Expected Location
  │
  ▼
POST /api/v1/ingestion/import/upload
  │
  ▼
ingestion-engine 解析 → 生成預覽
  │
  ▼
用戶確認 → POST /import/{job_id}/confirm
  │
  ▼
Celery Worker 非同步處理每一筆：
  │
  ├─ ① 正規化
  │   └─ 統一欄位名稱和格式
  │
  ├─ ② 去重比對
  │   ├─ serial_number + asset_tag → 找到？
  │   ├─ property_number → 找到？
  │   └─ control_number → 找到？
  │
  ├─ ③ 路由決定
  │   ├─ 新資產（沒找到）→ 驗證必要欄位 → 建立
  │   └─ 已有資產（找到）→ 進入權威檢查
  │
  ├─ ④ 權威檢查（已有資產）
  │   └─ 對每個要更新的欄位：
  │       ├─ 查 asset_field_authorities 表
  │       ├─ 匯入來源優先級 >= 當前最高 → 自動合併
  │       └─ 匯入來源優先級 < 當前最高 → 建立衝突記錄
  │
  └─ ⑤ 結果統計
      └─ matched / not_found / conflict / error
```

### 來源 3：SNMP/SSH/IPMI 自動發現

```
POST /discovery/scan 觸發（或 Celery Beat 定時）
  │
  ▼
收集器掃描目標 CIDR 網段：
  │
  ├─ SNMP 收集器
  │   ├─ sysDescr → OS 版本
  │   ├─ sysName → 主機名
  │   ├─ sysObjectID → 廠商識別（Cisco/HP/Dell/Huawei/Juniper）
  │   └─ 廠商專用 OID → 序號
  │
  ├─ SSH 收集器
  │   ├─ hostname → 主機名
  │   ├─ dmidecode → 序號、廠商、型號
  │   ├─ /etc/os-release → OS 類型和版本
  │   ├─ nproc / free / lsblk → CPU、記憶體、磁碟
  │   ├─ ip addr → IP 地址
  │   └─ dmidecode product_name → VM 自動偵測
  │       ├─ 含 VMware/KVM/Hyper-V → sub_type = "virtual"
  │       └─ 不含 → sub_type = "physical"
  │
  └─ IPMI 收集器
      ├─ FRU Inventory → 序號、廠商、型號
      ├─ Chassis Status → 電源狀態
      ├─ BMC Network → BMC IP/MAC
      └─ Sensor Data → 溫度、電壓
  │
  ▼
對每台發現的設備：
  │
  ├─ 去重比對（同 Excel 匯入邏輯）
  │
  ├─ 路由決定（取決於 scan_target 的 mode）：
  │   │
  │   ├─ auto 模式
  │   │   └─ 全部進 pipeline → 自動合併或建衝突
  │   │
  │   ├─ review 模式
  │   │   └─ 全部進 staging → 等人審批
  │   │
  │   └─ smart 模式（推薦）
  │       ├─ 已有資產 → 進 pipeline → 自動合併
  │       └─ 新資產 → 進 staging → 等人審批
  │
  ├─ 進 pipeline 的：
  │   └─ 權威檢查 → 自動合併或建衝突（同 Excel）
  │
  └─ 進 staging 的：
      └─ 寫入 discovered_assets 表（status=pending）
          │
          ├─ 人工 approve → 建立資產進 CMDB
          ├─ 人工 ignore → 標記忽略
          └─ 14 天無人處理 → 自動標記 expired
```

### 來源 4：SNMP MAC 表位置偵測

```
ingestion-engine 每 5 分鐘自動掃描交換機
  │
  ├─ 讀 CDP 鄰居表（思科交換機）
  │   → 每個端口連著的設備名稱 + MAC
  │
  ├─ 讀 MAC 地址表（補充）
  │   → 每個端口上的 MAC 地址
  │
  └─ NATS 推送 → cmdb-core
  │
  ▼
cmdb-core 收到後：
  │
  ├─ 更新 mac_address_cache
  │   ├─ MAC → 交換機端口 → 交換機的 rack_id → 偵測到的位置
  │   └─ MAC → 匹配 CMDB 中的資產
  │
  ├─ 立即比對 CompareLocations()
  │   ├─ CMDB 位置 vs 偵測位置
  │   │
  │   ├─ 一致（95%）→ 靜默
  │   ├─ 位置變了 →
  │   │   ├─ 有搬遷工單 → 自動更新 CMDB + 關閉工單
  │   │   └─ 沒有工單 → 告警「未授權搬遷」
  │   ├─ 設備消失 → 告警「設備失聯」
  │   └─ 陌生 MAC → 建立 Discovery 候選 + 告警
  │
  └─ 異常模式檢測
      ├─ 30 天內搬 3+ 次 → 頻繁搬遷告警
      ├─ 凌晨 22:00-06:00 搬遷 → 非工作時間告警
      └─ 同機櫃 1 小時消失 3+ 台 → 批量異常告警
```

**特點**：不建立新資產，不走 ingestion pipeline，只更新位置。

---

## 三、去重比對（交叉匹配）邏輯

### 3.1 四級 Fallback 匹配

不管資料從哪來，去重比對都用同一套邏輯：

```
一筆資料進來
  │
  ▼
Level 1：serial_number + asset_tag 聯合查詢
  SELECT id FROM assets 
  WHERE (serial_number = $1 OR asset_tag = $2) 
  AND tenant_id = $3 AND deleted_at IS NULL
  → 找到 ✓ → 匹配成功
  │
  ▼ 沒找到
Level 2：property_number 查詢
  SELECT id FROM assets WHERE property_number = $1 ...
  → 找到 ✓ → 匹配成功
  │
  ▼ 沒找到
Level 3：control_number 查詢
  SELECT id FROM assets WHERE control_number = $1 ...
  → 找到 ✓ → 匹配成功
  │
  ▼ 沒找到
Level 4：hostname + IP 查詢（自動發現場景）
  SELECT id FROM assets 
  WHERE attributes->>'hostname' = $1 
  OR attributes->>'ip_address' = $2 ...
  → 找到 ✓ → 匹配成功
  │
  ▼ 全都沒找到
結論：新資產 → 建立 or 進 staging
```

### 3.2 為什麼用四級 Fallback

| 級別 | 識別符 | 可靠度 | 為什麼需要 |
|------|--------|--------|-----------|
| Level 1 | serial_number + asset_tag | 最高 | 全球唯一標識，最可靠 |
| Level 2 | property_number | 高 | 公司內部財產編號，序號沒填時的備案 |
| Level 3 | control_number | 中 | 政府/法規管制編號，某些行業必須有 |
| Level 4 | hostname + IP | 低 | 自動發現場景，可能沒有其他識別符 |

### 3.3 匹配到已有資產後的更新流程

```
匹配到 asset_id = XXX
  │
  ▼
比較每個欄位：
  incoming_value vs cmdb_current_value
  │
  ├─ 值相同 → 跳過（不更新）
  │
  └─ 值不同 → 查權威矩陣
      │
      SELECT MAX(priority) FROM asset_field_authorities
      WHERE field_name = '欄位名' AND tenant_id = $1
      │
      ├─ 匯入來源優先級 >= 最高優先級
      │   └─ 自動合併：UPDATE assets SET 欄位 = 新值
      │
      └─ 匯入來源優先級 < 最高優先級
          └─ 建立衝突記錄：
              INSERT INTO import_conflicts (
                asset_id, field_name,
                current_value, incoming_value,
                current_source, incoming_source,
                status = 'pending'
              )
              │
              └─ 衝突處理：
                  ├─ 3 天無人處理 → 通知 ops-admin
                  └─ 7 天無人處理 → 自動採用高優先級來源
```

---

## 四、權威矩陣（誰有權改什麼）

### 4.1 矩陣定義

```
asset_field_authorities 表：

欄位            │ 來源    │ 優先級  │ 含義
───────────────┼────────┼───────┼────────────────
serial_number  │ ipmi   │  100   │ IPMI 直接讀硬體，最可信
serial_number  │ snmp   │   80   │ SNMP 讀交換機上報
serial_number  │ ssh    │   80   │ SSH 讀 dmidecode
serial_number  │ manual │   50   │ 人手動填，可能填錯
               │        │        │
vendor         │ ipmi   │  100   │ 硬體 FRU 資訊
vendor         │ manual │   50   │
               │        │        │
model          │ ipmi   │  100   │
model          │ manual │   50   │
               │        │        │
name           │ manual │  100   │ 名字是人取的，手動最可信
name           │ snmp   │   30   │ SNMP 的 sysName 可能不準
               │        │        │
status         │ manual │  100   │ 狀態由人決定
bia_level      │ manual │  100   │ BIA 等級由人決定
```

### 4.2 設計原則

| 欄位類型 | 最可信來源 | 原因 |
|---------|-----------|------|
| 硬體屬性（序號、廠商、型號） | IPMI（100） | 直接讀硬體 FRU，不可能錯 |
| 網路屬性（IP、hostname） | SSH/SNMP（80） | 機器自己知道自己的 IP |
| 命名屬性（名稱） | 手動（100） | 名字是人的決策 |
| 管理屬性（狀態、BIA 等級） | 手動（100） | 業務決策，只有人能定 |
| API 更新 | 50 | 等同手動，不能覆蓋自動掃描的硬體資訊 |

### 4.3 權威矩陣的保護效果

```
場景：IPMI 掃描到序號 SN-DELL-001

之後有人在前端手動改序號為 SN-WRONG-999
  │
  ▼
API PUT /assets/{id} 觸發權威檢查：
  serial_number 的最高優先級 = 100（IPMI）
  API 來源優先級 = 50（manual）
  50 < 100 → 拒絕修改，回傳 warning

結果：序號保持 SN-DELL-001，不被人為錯誤覆蓋
```

---

## 五、位置偵測的交叉匹配

### 5.1 三源比對

```
        ┌───────────────┐
        │   CMDB 記錄     │ ← assets.rack_id
        └───────┬───────┘
                │
     ┌──────────┼──────────┐
     ▼          ▼          ▼
┌─────────┐ ┌─────────┐ ┌─────────┐
│ 網路層   │ │ 實體層   │ │ 歷史層   │
│          │ │          │ │          │
│ CDP/MAC  │ │ QR 掃碼  │ │ 工單記錄 │
│ 交換機   │ │ 現場確認 │ │ 稽核日誌 │
│ 每5分鐘  │ │ 搬遷時   │ │ 持續     │
└────┬────┘ └────┬────┘ └────┬────┘
     │           │           │
     └───────────┼───────────┘
                 │
           比對結果矩陣
```

### 5.2 比對結果矩陣

| CMDB | 網路層 | 實體層 | 結論 | 處理 |
|------|--------|--------|------|------|
| A01 | A01 | A01 | 完全一致 | 靜默 |
| A01 | B03 | — | 設備搬了 | 有工單→自動更新；無工單→告警 |
| A01 | A01 | B03 | 掃碼位置不符（接延長線？） | 警告，人工確認 |
| A01 | (消失) | — | 設備離線 | 有維護工單→靜默；無→告警 |
| (不存在) | B03 | B03 | 未登記設備 | 建 Discovery 候選 + 告警 |
| A01 | B03 | B03 | 確認搬遷 | 自動修正 CMDB |

---

## 六、品質分數交叉驗證

### 6.1 四維度模型

```
品質分數 = 完整性 × 0.4 + 準確性 × 0.3 + 即時性 × 0.1 + 一致性 × 0.2
```

| 維度 | 權重 | 驗證什麼 | 資料來源 |
|------|------|---------|---------|
| 完整性 | 40% | 必要欄位是否填寫 | CMDB 自身 |
| 準確性 | 30% | 欄位格式是否正確（正則驗證） | CMDB 自身 |
| 即時性 | 10% | 資料是否超過 90 天未更新 | CMDB updated_at |
| 一致性 | 20% | 位置是否與網路偵測一致；伺服器是否有 rack_id | MAC 快取 + CMDB |

### 6.2 一致性維度的交叉驗證

```
品質掃描時：

  ├─ 檢查 1：伺服器有沒有 rack_id？
  │   └─ 沒有 → 一致性 -50 分
  │
  ├─ 檢查 2：MAC 快取中的位置 vs CMDB 記錄的位置
  │   └─ 不一致 → 一致性 -50 分
  │
  └─ 兩項都通過 → 一致性 100 分
```

---

## 七、衝突處理機制

### 7.1 衝突來源

| 來源 | 場景 | 衝突類型 |
|------|------|---------|
| Excel 匯入 | 匯入的序號和 CMDB 不同 | 欄位值衝突 |
| 自動發現 | 掃描到的廠商和 CMDB 不同 | 欄位值衝突 |
| 位置偵測 | MAC 表位置和 CMDB 位置不同 | 位置衝突 |
| Edge 同步 | 兩端同時修改同一筆資料 | 同步衝突 |

### 7.2 衝突處理流程

```
衝突產生
  │
  ├─ 欄位衝突（import_conflicts 表）
  │   ├─ 3 天 → 通知 ops-admin
  │   ├─ 7 天 → 自動採用高優先級來源
  │   └─ 人工處理 → 選擇保留哪個值
  │
  ├─ 位置衝突
  │   ├─ 有搬遷工單 → 自動確認
  │   └─ 無工單 → 告警 → 人工確認或忽略
  │
  └─ 同步衝突（sync_conflicts 表）
      ├─ 工單：雙維度模型（零衝突）
      ├─ 其他：LWW 或 Central Wins
      └─ 無法自動解決 → 衝突 UI 人工處理
```

---

## 八、資產生命週期全景

```
                    發現                入庫              運維              退役
                     │                  │                │                │
  ┌──────────────────▼──────────────────▼────────────────▼────────────────▼─────┐
  │                                                                            │
  │  SNMP/SSH 掃描 ──→ staging ──→ approve ──→ CMDB ──→ 維護/搬遷 ──→ 報廢     │
  │  Excel 匯入 ─────→ pipeline ─→ 自動合併 ─┘   │                              │
  │  手動建立 ────────────────────────────────────┘   │                          │
  │                                                   │                         │
  │  位置偵測 ─────── 持續追蹤位置 ───────────────────────┘                        │
  │  盤點核對 ─────── 定期驗證帳實 ────────────────────────────────────────────────┘│
  │  品質掃描 ─────── 持續驗證資料品質 ─────────────────────────────────────────────┘│
  │                                                                               │
  │  狀態流轉：procurement → deploying → operational → maintenance → decommission  │
  │                                                                               │
  │  全程稽核：每一步都記錄到 audit_events（append-only，不可竄改）                   │
  │                                                                               │
  └───────────────────────────────────────────────────────────────────────────────┘
```

---

## 九、總結

### 資料流入

| 來源 | 路徑 | 去重 | 權威檢查 | 品質閘門 |
|------|------|------|---------|---------|
| 手動錄入 | API 直接寫入 | asset_tag UNIQUE | 更新時有 | 建立時有 |
| Excel 匯入 | ingestion pipeline | 四級 fallback | 有 | 由 pipeline 驗證 |
| 自動發現 | ingestion pipeline | 四級 fallback | 有 | 由 pipeline 驗證 |
| 位置偵測 | MAC 快取比對 | MAC 地址匹配 | 不適用（只改位置） | 品質分數聯動 |

### 交叉驗證

| 驗證維度 | 資料來源 A | 資料來源 B | 不一致時 |
|---------|-----------|-----------|---------|
| 欄位準確性 | CMDB 記錄 | 最新掃描結果 | 權威矩陣決定保留哪個 |
| 位置準確性 | CMDB rack_id | MAC 表偵測位置 | 告警或自動更新 |
| 帳實相符 | CMDB 資產清單 | 盤點掃碼結果 | 差異記錄 + 超過 5 筆建工單 |
| 設備存在性 | CMDB 記錄 | MAC 表（有/無） | 消失→告警，新出現→Discovery |

### SSOT 保障

```
外部資料 ──→ 統一管線 ──→ 權威檢查 ──→ CMDB（唯一真相）
                                         │
                              ┌──────────┼──────────┐
                              ▼          ▼          ▼
                          位置偵測    盤點核對    品質掃描
                          （持續）    （觸發式）  （持續）
                              │          │          │
                              └──────────┼──────────┘
                                         │
                                    發現不一致
                                         │
                                    告警 + 修正
```
