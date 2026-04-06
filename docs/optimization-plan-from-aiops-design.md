# CMDB 平台優化實施計劃

> 來源：`全球化 CMDB + AIOps 一體化平臺.md` 設計文檔對比分析
> 日期：2026-04-04
> 範圍：10 個優化項，分 3 個 Sprint 執行
> **修訂版：v2（2026-04-04）— 修復 4 個 BLOCKING 衝突**

---

## 衝突修訂摘要

經架構審查發現 4 個 BLOCKING 衝突，已在本版修訂：

| # | 原始問題 | 修訂方案 | 涉及章節 |
|---|---------|---------|---------|
| **B1** | BIA 自動繼承：最後寫入覆蓋更高等級 | 改用 `MAX(tier)` 聚合 SQL，取所有關聯評估中最高等級 | §1.2 |
| **B2** | ITSM 審批：返回 `pending_approval` 打破前端契約 | 改為「事後審計」模式：正常更新 + 額外建工單，返回標準 Asset | §3.3 |
| **B3** | CIType 驗證：CreateAssetModal 不送 attributes 被拒 | 改為 soft validation（warning 不阻斷），先不拒絕請求 | §2.2 |
| **B4** | Webhook deliver() 呼叫簽名不匹配（3 vs 2 參數）| 修正為 `d.deliver(sub, event)` 2 參數 + BIA 查詢移入 goroutine | §1.3 |

另有 9 個 WARNING 已在相應章節加入處理邏輯（GetAsset 錯誤記錄、LEFT JOIN 建議等）。

---

## 需求總覽

| # | 優化項 | 優先級 | 類型 | 預估工時 |
|---|--------|--------|------|---------|
| 1 | U 位衝突檢測 API | Sprint 1 | 後端邏輯 | 1h |
| 2 | BIA 自動繼承（業務系統→資產） | Sprint 1 | 後端邏輯 | 2h |
| 3 | Webhook BIA 等級過濾 | Sprint 1 | 後端+DB | 2h |
| 4 | 機櫃視圖接真實 rack_slots 數據 | Sprint 1 | 前端 | 3h |
| 5 | 數據質量治理引擎 | Sprint 2 | 全棧新模組 | 1-2d |
| 6 | CIType Schema 驗證 | Sprint 2 | 後端邏輯 | 4h |
| 7 | BIA 遞歸影響追溯 API | Sprint 2 | 後端+前端 | 4h |
| 8 | 自動發現 Staging Area | Sprint 3 | 全棧新模組 | 1-2w |
| 9 | Excel 盤點匯入匹配 | Sprint 3 | 後端+前端 | 2-3d |
| 10 | ITSM 審批流整合 | Sprint 3 | 後端+前端 | 1w |

---

## Sprint 1：快速優化（高 ROI，1-2 天）

### 1.1 U 位衝突檢測 API

**需求：** 當資產被放置到機架的某個 U 位時（通過 rack_slots），API 層應檢測是否與現有設備衝突，返回友好錯誤而非 DB 約束報錯。

**現狀：**
- `rack_slots` 表有 `UNIQUE(rack_id, start_u, side)` DB 約束
- 目前沒有 rack_slots 的 CRUD API（rack_slots 只通過 seed.sql 填充）
- 衝突時 DB 報 unique violation 錯誤，前端收到 500

**實施方案：**

#### 1.1.1 新增 rack_slots CRUD（3 個端點）

**sqlc 查詢** — 新建 `db/queries/rack_slots.sql`：

```sql
-- name: ListRackSlots :many
SELECT rs.*, a.name as asset_name, a.asset_tag, a.type as asset_type, a.bia_level
FROM rack_slots rs
JOIN assets a ON rs.asset_id = a.id
WHERE rs.rack_id = $1
ORDER BY rs.start_u;

-- name: CreateRackSlot :one
INSERT INTO rack_slots (rack_id, asset_id, start_u, end_u, side)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteRackSlot :exec
DELETE FROM rack_slots WHERE id = $1;

-- name: CheckSlotConflict :one
SELECT count(*) FROM rack_slots
WHERE rack_id = $1
  AND side = $2
  AND start_u <= $3  -- new end_u
  AND end_u >= $4;   -- new start_u
```

**OpenAPI 端點：**

```
GET  /racks/{id}/slots        → 列出機架所有 slot（含資產資訊）
POST /racks/{id}/slots        → 新增 slot（含衝突檢測）
DELETE /racks/{id}/slots/{slotId} → 移除 slot
```

**衝突檢測邏輯（impl.go CreateRackSlot handler）：**

```go
func (s *APIServer) CreateRackSlot(c *gin.Context, rackId IdPath) {
    var req CreateRackSlotJSONRequestBody
    // ... bind JSON

    // 1. 檢查衝突
    conflictCount, err := s.queries.CheckSlotConflict(ctx, dbgen.CheckSlotConflictParams{
        RackID:  uuid.UUID(rackId),
        Side:    req.Side,
        Column3: req.EndU,   // new end_u <= existing start_u check
        Column4: req.StartU, // new start_u >= existing end_u check
    })
    if conflictCount > 0 {
        response.BadRequest(c, fmt.Sprintf("U position conflict: U%d-U%d on %s side is occupied", req.StartU, req.EndU, req.Side))
        return
    }

    // 2. 建立 slot
    slot, err := s.queries.CreateRackSlot(ctx, ...)
    // 3. 記錄審計 + 回應
}
```

**前端接入：**
- `useRackSlots(rackId)` hook — 替換 RackDetail 的 hardcoded equipment
- `useCreateRackSlot()` mutation — AddNewRack 頁面建立 slot

#### 1.1.2 改動清單

| 檔案 | 改動 |
|------|------|
| `db/queries/rack_slots.sql` | 新建，4 queries |
| `api/openapi.yaml` | 加 3 端點 + RackSlot schema |
| `internal/api/impl.go` | 加 3 handler（含衝突檢測邏輯）|
| `internal/api/convert.go` | 加 toAPIRackSlot converter |
| `cmdb-demo/src/hooks/useTopology.ts` | 加 useRackSlots + useCreateRackSlot |
| `cmdb-demo/src/pages/RackDetailUnified.tsx` | slot 視圖改用 API 數據 |

---

### 1.2 BIA 自動繼承（業務系統→資產）

**需求：** 當 `bia_assessments` 的 `tier` 變更時，自動更新其通過 `bia_dependencies` 關聯的所有 `assets` 的 `bia_level` 欄位。

**現狀：**
- `bia_assessments` 有 tier 欄位
- `bia_dependencies` 有 assessment_id → asset_id 關聯
- `assets` 有 bia_level 欄位
- 三者之間沒有自動同步邏輯

**實施方案：**

#### 1.2.1 新增 sqlc 查詢

追加到 `db/queries/bia.sql`：

```sql
-- name: PropagateBIALevelByAssessment :exec
-- 使用 MAX(tier) 策略：取資產關聯的所有評估中最高等級
-- 避免低等級評估覆蓋高等級（衝突分析 BLOCKING #1 修復）
UPDATE assets SET
    bia_level = sub.max_tier,
    updated_at = now()
FROM (
    SELECT bd.asset_id,
        CASE
            WHEN 'critical' = ANY(array_agg(ba.tier)) THEN 'critical'
            WHEN 'important' = ANY(array_agg(ba.tier)) THEN 'important'
            WHEN 'normal' = ANY(array_agg(ba.tier)) THEN 'normal'
            ELSE 'minor'
        END as max_tier
    FROM bia_dependencies bd
    JOIN bia_assessments ba ON ba.id = bd.assessment_id
    WHERE bd.asset_id IN (
        SELECT asset_id FROM bia_dependencies WHERE assessment_id = $1
    )
    GROUP BY bd.asset_id
) sub
WHERE assets.id = sub.asset_id;
```

> **衝突修復說明：** 原方案直接 SET `bia_level = $2`，當資產屬於多個業務系統時，
> 最後寫入的 tier 會覆蓋更高等級。修訂後使用 `MAX(tier)` 聚合，
> 確保資產始終保持其關聯評估中的最高等級。

#### 1.2.2 在 UpdateBIAAssessment handler 中觸發

修改 `impl.go` 的 `UpdateBIAAssessment` handler：

```go
// 原有邏輯：更新 assessment
updated, err := s.biaSvc.UpdateAssessment(ctx, params)

// 新增：如果 tier 變更，重新計算所有關聯資產的 BIA 等級（取 MAX）
if req.Tier != nil {
    if err := s.biaSvc.PropagateBIALevel(c.Request.Context(), updated.ID); err != nil {
        fmt.Printf("BIA propagation error: %v\n", err)
    }
}
```

#### 1.2.3 Service 方法

```go
func (s *Service) PropagateBIALevel(ctx context.Context, assessmentID uuid.UUID) error {
    // 只需傳 assessmentID，SQL 內部會計算 MAX(tier)
    return s.queries.PropagateBIALevelByAssessment(ctx, assessmentID)
}
```

#### 1.2.4 改動清單

| 檔案 | 改動 |
|------|------|
| `db/queries/bia.sql` | 加 UpdateAssetsBIAByAssessment query |
| `internal/domain/bia/service.go` | 加 PropagateBIALevel method |
| `internal/api/impl.go` | UpdateBIAAssessment handler 加 propagation 調用 |
| `internal/dbgen/bia.sql.go` | 重新生成 |

---

### 1.3 Webhook BIA 等級過濾

**需求：** webhook_subscriptions 支持按 BIA 等級過濾，只有 Critical/Important 資產的事件才推送給指定 webhook。

**現狀：**
- `webhook_subscriptions` 有 `events TEXT[]` 欄位（按事件類型過濾）
- webhook_dispatcher.go 只做事件類型匹配
- 沒有 BIA 過濾機制

**實施方案：**

#### 1.3.1 DB 遷移

新建 `db/migrations/000014_webhook_bia_filter.up.sql`：

```sql
ALTER TABLE webhook_subscriptions ADD COLUMN filter_bia TEXT[] DEFAULT '{}';
```

down：
```sql
ALTER TABLE webhook_subscriptions DROP COLUMN IF EXISTS filter_bia;
```

#### 1.3.2 修改 webhook_dispatcher.go

> **衝突修復說明：**
> - 修正 `deliver()` 呼叫簽名：實際方法只接收 2 參數 `(sub, event)`，不接收 `ctx`（BLOCKING #4 修復）
> - `GetAsset` 錯誤不再靜默忽略，改為記錄 warning 並放行（WARNING #6 修復）
> - BIA 查詢移到 goroutine 內部，避免阻塞主事件處理迴圈（WARNING #7 修復）

```go
func (d *WebhookDispatcher) HandleEvent(ctx context.Context, event eventbus.Event) error {
    subs, err := d.queries.ListWebhooksByEvent(ctx, event.Subject)
    if err != nil {
        zap.L().Error("failed to list webhooks", zap.Error(err))
        return nil
    }
    
    for _, sub := range subs {
        sub := sub // capture for goroutine
        
        if len(sub.FilterBia) > 0 {
            // BIA 過濾在 goroutine 內執行，不阻塞主迴圈
            go func() {
                var payload map[string]string
                json.Unmarshal(event.Payload, &payload)
                if assetID, ok := payload["asset_id"]; ok {
                    asset, err := d.queries.GetAsset(ctx, uuid.MustParse(assetID))
                    if err != nil {
                        // 資產查詢失敗（可能已刪除）→ 記錄 warning 但仍投遞
                        zap.L().Warn("BIA filter: asset lookup failed, delivering anyway",
                            zap.String("asset_id", assetID), zap.Error(err))
                    } else if !contains(sub.FilterBia, asset.BiaLevel) {
                        return // BIA 等級不匹配，跳過投遞
                    }
                }
                d.deliver(sub, event) // 正確的 2 參數呼叫
            }()
        } else {
            go d.deliver(sub, event) // 無 BIA 過濾，直接投遞
        }
    }
    return nil
}
```

#### 1.3.3 改動清單

| 檔案 | 改動 |
|------|------|
| `db/migrations/000014_*.sql` | 新建 migration |
| `internal/domain/integration/webhook_dispatcher.go` | 加 BIA filter 邏輯 |
| `api/openapi.yaml` | WebhookSubscription schema 加 filter_bia 欄位 |
| `cmdb-demo/src/components/CreateWebhookModal.tsx` | 加 BIA filter 多選 |

---

### 1.4 機櫃視圖接真實 rack_slots 數據

**需求：** RackDetail 頁面的 U 位可視化從 hardcoded 設備列表改為使用真實 rack_slots + assets 數據，且按 BIA 等級顯示不同顏色。

**現狀：**
- RackDetail 有 FRONT/REAR 視圖切換
- `useRackAssets(rackId)` 已接通（返回資產列表）
- rack_slots seed 有 20 條數據
- 但渲染仍使用 hardcoded `equipment` 數組

**實施方案：**

#### 1.4.1 使用 1.1 的 useRackSlots hook

依賴 1.1 完成後的 `GET /racks/{id}/slots` 端點。

#### 1.4.2 修改 RackDetail 可視化組件

```tsx
// 建立 42U slot 數組
const totalU = rack?.total_u || 42
const uSlots = Array.from({ length: totalU }, (_, i) => totalU - i)

// 從 API 取 slot 數據
const { data: slotsResp } = useRackSlots(rackId)
const slots = slotsResp?.data || []

// 渲染每個 U 位
{uSlots.map(u => {
  const slot = slots.find(s => u >= s.start_u && u <= s.end_u && s.side === view)
  const isStartU = slot && u === slot.end_u // 頂部 U 位顯示設備名
  
  return (
    <div key={u} className="flex h-6 border-b border-outline-variant/20">
      <div className="w-8 text-center text-[10px] text-on-surface-variant">{u}</div>
      <div className="flex-1 relative">
        {slot ? (
          isStartU && (
            <div
              className={`absolute inset-x-0 z-10 m-px rounded flex items-center justify-center text-xs font-bold
                ${slot.bia_level === 'critical' ? 'bg-error-container text-on-error-container' :
                  slot.bia_level === 'important' ? 'bg-[#92400e] text-[#fbbf24]' :
                  'bg-[#1e3a5f] text-on-primary-container'}`}
              style={{ height: `${(slot.end_u - slot.start_u + 1) * 24 - 4}px` }}
            >
              {slot.asset_name || slot.asset_tag}
            </div>
          )
        ) : (
          <span className="text-[10px] text-on-surface-variant/30 ml-2">Empty</span>
        )}
      </div>
    </div>
  )
})}

{/* 圖例 */}
<div className="flex gap-4 mt-3 text-xs text-on-surface-variant">
  <div className="flex items-center gap-1">
    <span className="w-3 h-3 rounded bg-error-container" /> Critical
  </div>
  <div className="flex items-center gap-1">
    <span className="w-3 h-3 rounded bg-[#92400e]" /> Important
  </div>
  <div className="flex items-center gap-1">
    <span className="w-3 h-3 rounded bg-[#1e3a5f]" /> Normal
  </div>
</div>
```

#### 1.4.3 改動清單

| 檔案 | 改動 |
|------|------|
| `cmdb-demo/src/hooks/useTopology.ts` | 加 useRackSlots hook |
| `cmdb-demo/src/lib/api/topology.ts` | 加 listRackSlots method |
| `cmdb-demo/src/pages/RackDetailUnified.tsx` | 重構 VisualizationTab 用 slot 數據 |

---

## Sprint 2：中級優化（中 ROI，3-5 天）

### 2.1 數據質量治理引擎

**需求：** 實現四維度（完整性/準確性/時效性/一致性）自動評分系統，定期掃描所有資產，生成質量報告。

**新增 DB 表（2 張）：**

#### quality_rules — 質量規則定義

```sql
CREATE TABLE quality_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    ci_type         VARCHAR(50),           -- 適用的資產類型（server/network/storage/NULL=全部）
    dimension       VARCHAR(20)  NOT NULL, -- completeness/accuracy/timeliness/consistency
    field_name      VARCHAR(50)  NOT NULL, -- 檢查的欄位名
    rule_type       VARCHAR(20)  NOT NULL, -- required/regex/range/foreign_key
    rule_config     JSONB        DEFAULT '{}', -- {"regex": "^10\\.", "min": 0, "max": 100}
    weight          INT          DEFAULT 10,   -- 該規則在維度中的權重
    enabled         BOOLEAN      DEFAULT true,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
```

#### quality_scores — 掃描結果

```sql
CREATE TABLE quality_scores (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    asset_id        UUID         NOT NULL REFERENCES assets(id),
    completeness    NUMERIC(5,2) DEFAULT 0,
    accuracy        NUMERIC(5,2) DEFAULT 0,
    timeliness      NUMERIC(5,2) DEFAULT 0,
    consistency     NUMERIC(5,2) DEFAULT 0,
    total_score     NUMERIC(5,2) DEFAULT 0,
    issue_details   JSONB        DEFAULT '[]',
    scan_date       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_quality_scores_asset ON quality_scores(asset_id);
CREATE INDEX idx_quality_scores_date ON quality_scores(scan_date DESC);
```

**API 端點（5 個）：**

| 方法 | 路徑 | 用途 |
|------|------|------|
| GET | `/quality/rules` | 列出規則 |
| POST | `/quality/rules` | 建立規則 |
| GET | `/quality/dashboard` | 質量看板（全域平均分 + 維度分 + 最差 Top 5）|
| GET | `/quality/scores/{assetId}` | 單資產質量歷史 |
| POST | `/quality/scan` | 手動觸發掃描（或定時任務）|

**評分引擎邏輯（Go service）：**

```go
func (s *QualityService) ScanAllAssets(ctx context.Context, tenantID uuid.UUID) error {
    assets, _, _ := s.assetQueries.ListAssets(ctx, ...)
    rules, _ := s.queries.ListQualityRules(ctx, tenantID)
    
    for _, asset := range assets {
        score := s.evaluateAsset(asset, rules)
        s.queries.CreateQualityScore(ctx, score)
    }
    return nil
}

func (s *QualityService) evaluateAsset(asset dbgen.Asset, rules []dbgen.QualityRule) QualityScore {
    scores := map[string]float64{"completeness": 100, "accuracy": 100, "timeliness": 100, "consistency": 100}
    issues := []map[string]string{}
    
    for _, rule := range rules {
        if rule.CiType.Valid && rule.CiType.String != asset.Type { continue }
        
        value := getAssetField(asset, rule.FieldName)
        
        switch rule.Dimension {
        case "completeness":
            if rule.RuleType == "required" && (value == "" || value == nil) {
                scores["completeness"] -= float64(rule.Weight)
                issues = append(issues, map[string]string{"field": rule.FieldName, "error": "必填項缺失"})
            }
        case "accuracy":
            if rule.RuleType == "regex" {
                pattern := rule.RuleConfig["regex"]
                if !regexp.MustCompile(pattern).MatchString(value) {
                    scores["accuracy"] -= float64(rule.Weight)
                }
            }
        }
    }
    
    // 時效性：90 天未更新 → 降級
    if time.Since(asset.UpdatedAt) > 90*24*time.Hour {
        scores["timeliness"] = 60
    }
    
    // 一致性：物理服務器應有 rack_id
    if asset.Type == "server" && !asset.RackID.Valid {
        scores["consistency"] -= 50
    }
    
    total := scores["completeness"]*0.4 + scores["accuracy"]*0.3 + scores["timeliness"]*0.1 + scores["consistency"]*0.2
    
    return QualityScore{Completeness: scores["completeness"], ..., Total: total, Issues: issues}
}
```

**前端頁面：** `/quality` — 質量看板

| 區域 | 內容 |
|------|------|
| 全域得分卡 | 總體質量分（圓環）+ 4 維度分數 |
| 維度趨勢圖 | 近 7 天每日掃描分數折線 |
| 問題最多 Top 5 | 最低分資產列表 + 問題詳情 |
| 規則管理 tab | 質量規則 CRUD |

**Seed 數據：** 5 條質量規則

```sql
INSERT INTO quality_rules (tenant_id, ci_type, dimension, field_name, rule_type, rule_config, weight) VALUES
    ('a0000000-...', 'server', 'completeness', 'serial_number', 'required', '{}', 15),
    ('a0000000-...', 'server', 'completeness', 'vendor', 'required', '{}', 10),
    ('a0000000-...', NULL, 'accuracy', 'serial_number', 'regex', '{"regex": "^[A-Z0-9\\-]{5,30}$"}', 20),
    ('a0000000-...', 'network', 'completeness', 'serial_number', 'required', '{}', 15),
    ('a0000000-...', NULL, 'consistency', 'rack_id', 'required', '{}', 25);
```

**改動清單：**

| 檔案 | 類型 | 說明 |
|------|------|------|
| `db/migrations/000015_quality_tables.up/down.sql` | 新建 | 2 張表 |
| `db/queries/quality.sql` | 新建 | CRUD + dashboard 聚合 |
| `db/seed/seed.sql` | 追加 | 5 規則 |
| `internal/domain/quality/service.go` | 新建 | 評分引擎 + CRUD |
| `internal/api/impl.go` | 修改 | 5 handler |
| `api/openapi.yaml` | 修改 | 5 端點 + 3 schema |
| `cmd/server/main.go` | 修改 | 接入 qualitySvc |
| `cmdb-demo/src/lib/api/quality.ts` | 新建 | API client |
| `cmdb-demo/src/hooks/useQuality.ts` | 新建 | hooks |
| `cmdb-demo/src/pages/QualityDashboard.tsx` | 新建 | 質量看板頁 |
| `cmdb-demo/src/App.tsx` | 修改 | 路由 |
| `cmdb-demo/src/layouts/MainLayout.tsx` | 修改 | sidebar 加 Data Quality |

---

### 2.2 CIType Schema 驗證

**需求：** 定義每種資產類型的 attributes 必填 schema，CreateAsset 時校驗。

> **衝突修復說明（BLOCKING #3）：**
> 原方案在 CreateAsset 時硬性拒絕缺少 attributes 的請求，
> 但 `CreateAssetModal` 不送 attributes 欄位 → 所有建立都會被拒。
>
> **修訂方案：** 改為 **soft validation**（警告不阻斷）+ 同步更新 CreateAssetModal。

**方案 A：Soft Validation（推薦，先上線）**

不加新表。在 CreateAsset handler 加入 attributes **警告**（不阻斷），並在 API response 的 `meta` 中附帶 warning：

```go
var assetTypeSchemas = map[string][]string{
    "server":  {"cpu", "memory", "storage", "os"},
    "network": {"ports", "firmware"},
    "storage": {"raw_capacity", "protocol"},
    "power":   {"capacity"},
}

// 在 CreateAsset handler 中（建立成功後、回應前）：
warnings := []string{}
if schema, ok := assetTypeSchemas[req.Type]; ok {
    if req.Attributes == nil {
        warnings = append(warnings, fmt.Sprintf("type %s recommends attributes: %v", req.Type, schema))
    } else {
        attrs := *req.Attributes
        for _, field := range schema {
            if _, exists := attrs[field]; !exists {
                warnings = append(warnings, fmt.Sprintf("missing recommended attribute: %s", field))
            }
        }
    }
}

// 回應時附帶 warnings（不影響 201 狀態碼）
if len(warnings) > 0 {
    c.JSON(201, gin.H{"data": toAPIAsset(*created), "meta": gin.H{"warnings": warnings}})
    return
}
response.Created(c, toAPIAsset(*created))
```

**方案 B：嚴格驗證（Phase 2 上線，需先完成 Modal 更新）**

1. 先更新 `CreateAssetModal.tsx` 加入依資產類型動態顯示的 attributes 欄位
2. 再將 soft validation 改為 hard validation（400 拒絕）

**改動（方案 A）：**

| 檔案 | 改動 |
|------|------|
| `internal/api/impl.go` | CreateAsset handler 加 ~25 行 warning 邏輯 |
| `cmdb-demo/src/components/CreateAssetModal.tsx` | （可選）加 attributes 輸入欄位 |

---

### 2.3 BIA 遞歸影響追溯 API

**需求：** 給定一個資產 ID，向上追溯受影響的所有業務系統（通過 bia_dependencies 反查 bia_assessments）。

**新增端點：**

```
GET /bia/impact/{assetId} → 返回受影響的 BIA 評估列表 + 影響路徑
```

**sqlc 查詢：**

```sql
-- name: GetImpactedAssessments :many
SELECT ba.* FROM bia_assessments ba
JOIN bia_dependencies bd ON bd.assessment_id = ba.id
WHERE bd.asset_id = $1;
```

**前端接入：**
- AssetDetail 頁面加「BIA Impact」區塊，顯示此資產影響的業務系統
- 用 Tier 顏色 badge 顯示嚴重程度

**改動清單：**

| 檔案 | 改動 |
|------|------|
| `db/queries/bia.sql` | 加 GetImpactedAssessments query |
| `api/openapi.yaml` | 加 GET /bia/impact/{assetId} |
| `internal/api/impl.go` | 加 handler |
| `cmdb-demo/src/hooks/useBIA.ts` | 加 useBIAImpact hook |
| `cmdb-demo/src/pages/AssetDetailUnified.tsx` | Overview tab 加 BIA Impact 區塊 |

---

## Sprint 3：大型模組（低 ROI 高工作量，1-2 週）

### 3.1 自動發現 Staging Area

**需求：** 從外部系統（VMware/SNMP/手動匯入）採集資產 → 進入緩衝區 → 差異比對 → 人工審核 → 入庫。

**新增 DB 表：**

```sql
CREATE TABLE discovered_assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    source          VARCHAR(50)  NOT NULL,  -- vmware/snmp/manual/csv
    external_id     VARCHAR(255),           -- 來源系統唯一 ID
    hostname        VARCHAR(255),
    ip_address      VARCHAR(50),
    raw_data        JSONB        NOT NULL DEFAULT '{}',
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending', -- pending/approved/ignored/conflict
    matched_asset_id UUID        REFERENCES assets(id),     -- 如匹配到現有資產
    diff_details    JSONB,                  -- {"name": {"old": "A", "new": "B"}}
    discovered_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    reviewed_by     UUID         REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ
);
```

**API 端點（6 個）：**

| 方法 | 路徑 | 用途 |
|------|------|------|
| GET | `/discovery/pending` | 列出待審核資產 |
| POST | `/discovery/ingest` | 批量匯入發現資產（來源系統調用）|
| POST | `/discovery/{id}/approve` | 審核通過 → 建立/更新 CI |
| POST | `/discovery/{id}/ignore` | 忽略 |
| GET | `/discovery/stats` | 統計（今日發現/待審/衝突/自動匹配率）|
| POST | `/discovery/auto-review` | 執行自動審核規則 |

**自動匹配邏輯：**
```go
// 根據 IP 或 serial_number 匹配現有資產
func (s *DiscoveryService) AutoMatch(ctx context.Context, item dbgen.DiscoveredAsset) {
    // 1. 先用 IP 精確匹配
    // 2. 再用 hostname 模糊匹配
    // 3. 找到 → 設 matched_asset_id + 計算 diff
    // 4. 未找到 → status = 'pending'（新資產）
}
```

**前端：** 重寫 AutoDiscovery 頁面（`/assets/discovery`）為真實審核界面

**改動清單：**

| 類型 | 檔案數 | 說明 |
|------|--------|------|
| 後端 | 6 | migration + queries + service + impl + openapi + main |
| 前端 | 4 | api client + hook + page rewrite + modal |
| Seed | 1 | 模擬 5 條 discovered_assets |

---

### 3.2 Excel 盤點匯入匹配

**需求：** 上傳財務 Excel 清單，自動與 CMDB 資產比對，標記匹配/不匹配/遺失。

**方案：** Go 後端用 `excelize` 解析 Excel。

**新增端點：**

```
POST /inventory/tasks/{id}/import-excel  → multipart form-data 上傳 Excel
```

**處理邏輯：**
```go
// 1. 解析 Excel（asset_no, serial_number, location 欄位）
// 2. 逐行比對 assets 表（by serial_number 或 asset_tag）
// 3. 匹配成功 → 建立 inventory_item status='scanned'
// 4. 不匹配 → 建立 inventory_item status='discrepancy' + diff
// 5. Excel 有但 CMDB 無 → status='missing'
// 6. 回傳統計（matched/mismatch/missing counts）
```

**改動：** 4 檔案（endpoint + service + frontend upload）

---

### 3.3 ITSM 審批流整合

**需求：** 關鍵資產（Critical BIA）的變更需經審批才能生效。

> **衝突修復說明（BLOCKING #2）：**
> 原方案在 UpdateAsset 返回 `{"status": "pending_approval"}`，
> 但前端 `updateAsset.mutate` 預期收到 Asset 物件 → 導致 runtime error。
>
> **修訂方案：**
> 1. 資產仍正常更新（先執行變更）
> 2. Critical 資產變更後**額外**自動建立審計工單（不阻斷更新）
> 3. 返回標準 200 + Asset 物件 + 額外 `meta.change_order_id` 欄位
> 4. 前端可選擇性顯示「已建立變更審計工單」提示

**方案：** 利用現有 work_orders 表，**事後審計**而非事前阻斷。

**邏輯：**
```
資產變更請求 → 正常執行更新
  → Critical: 更新成功 + 自動建立 work_order (type='change_audit') → 通知主管覆核
  → Important: 更新成功 + 記錄審計日誌（現有 recordAudit 已覆蓋）
  → Normal/Minor: 更新成功
```

**修改 UpdateAsset handler：**
```go
// 原有邏輯：正常更新資產（不阻斷）
updated, err := s.assetSvc.Update(c.Request.Context(), params)
if err != nil { ... }

// 記錄審計
s.recordAudit(c, "asset.updated", "asset", "asset", updated.ID, diff)

// 新增：Critical 資產自動建立變更審計工單
var changeOrderID *uuid.UUID
if updated.BiaLevel == "critical" {
    order, err := s.maintenanceSvc.Create(c.Request.Context(), tenantID, userID,
        maintenance.CreateOrderRequest{
            Title:       fmt.Sprintf("Change Audit: %s", updated.Name),
            Type:        "change_audit",
            Description: fmt.Sprintf("Critical asset modified. Changes: %v", diff),
            Priority:    "high",
        })
    if err == nil {
        changeOrderID = &order.ID
    }
}

// 回應：標準 Asset 物件 + 可選 meta（前端向後兼容）
apiAsset := toAPIAsset(*updated)
if changeOrderID != nil {
    c.JSON(200, gin.H{
        "data": apiAsset,
        "meta": gin.H{"change_order_id": changeOrderID.String(), "request_id": c.GetString("request_id")},
    })
    return
}
response.OK(c, apiAsset)
```

**前端處理（可選增強）：**
```tsx
// updateAsset.mutate 的 onSuccess 回調
onSuccess: (resp) => {
  setEditing(false)
  // 可選：檢查是否有變更工單
  if (resp?.meta?.change_order_id) {
    alert(`Critical asset change recorded. Audit order: ${resp.meta.change_order_id}`)
  }
}
```

**改動：**

| 檔案 | 改動 |
|------|------|
| `internal/api/impl.go` | UpdateAsset handler 加 ~15 行（Critical 建工單邏輯）|
| `cmdb-demo/src/pages/AssetDetailUnified.tsx` | （可選）onSuccess 加 toast 提示 |

> **關鍵差異：** 原方案阻斷更新（前端壞掉），修訂後不阻斷更新（事後審計），
> 前端完全向後兼容。

---

## 改動量總估計

| Sprint | 新檔案 | 修改檔案 | 新增行數 | 工時 |
|--------|--------|---------|---------|------|
| Sprint 1 | 3 | 12 | ~400 | 1-2 天 |
| Sprint 2 | 8 | 8 | ~800 | 3-5 天 |
| Sprint 3 | 10 | 8 | ~1,200 | 1-2 週 |
| **合計** | **21** | **28** | **~2,400** | **2-3 週** |
