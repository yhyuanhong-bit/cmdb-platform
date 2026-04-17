# 監控頁面剩餘 8 個問題修復計劃

## 概覽

| # | 原編號 | 嚴重度 | 問題 | 預估工作量 | 依賴 |
|---|--------|--------|------|-----------|------|
| A | #10 | HIGH | LocationDetection 硬編碼文字 + `as any` | 小 | 無 |
| B | #19 | MEDIUM | API response 無 runtime 驗證 | 中 | 無 |
| C | #22 | MEDIUM | Energy 長查詢無 timeout | 小 | 無 |
| D | #23 | MEDIUM | Fallback data i18n 混用 | 小 | 無 |
| E | #24 | MEDIUM | Sensor alert rule 去重 | 小 | 無 |
| F | #25 | LOW | Sensor save 缺 loading 狀態 | 小 | 無 |
| G | #26 | LOW | Sensor polling 間隔 "Coming Soon" | 中 | 後端 API |
| H | #30 | LOW | i18n key 命名不一致 | 大 | 全局影響 |

---

## A. LocationDetection 硬編碼文字 + `as any`

**文件**: `src/pages/LocationDetection.tsx`

**問題**:
- Line 29: `apiClient.post('/ingestion/mac-scan', {}) as any` — 無型別
- 頁面內可能有未經 `t()` 的中英文混雜

**修復**:
1. 定義 `MacScanResult` interface: `{ scanned_ips: number; entries_collected: number }`
2. 替換 `as any` → `as MacScanResult`
3. 全文掃描所有 JSX 文字，未包在 `t()` 的加上 i18n key
4. 三語 locale 各補缺失的 key

**驗證**: `npx tsc --noEmit` + 切換語言看頁面

---

## B. API Response Runtime 驗證

**文件**: 多個監控頁面

**問題**: API 回傳的 JSON shape 只在編譯時假設正確，runtime 沒有驗證。如果後端改了 schema，前端不會報錯而是 crash 或顯示 undefined。

**修復方案（漸進式，不一步到位）**:
1. 在 `src/lib/api/client.ts` 的 `request<T>()` 方法中，加一個 dev-only 的 shape 檢查：
   ```ts
   if (import.meta.env.DEV && json.data === undefined && json.error === undefined) {
     console.warn('[API] Unexpected response shape:', path, json)
   }
   ```
2. 對關鍵的 monitoring endpoints（`/monitoring/alerts`, `/energy/summary`）加 zod schema 驗證在 hook 層：
   ```ts
   const parsed = alertResponseSchema.safeParse(resp)
   if (!parsed.success) console.warn('Alert response mismatch', parsed.error)
   ```
3. 不阻斷 UI — parse 失敗時 fallback 到空數據 + 顯示 warning

**不做**: 不在所有 API 加 zod（工作量太大），只針對核心 monitoring 的 3-4 個端點

**驗證**: 手動修改後端回傳格式，看前端是否 console.warn 而非 crash

---

## C. Energy 長查詢無 Timeout

**文件**: `cmdb-core/internal/api/energy_endpoints.go`

**問題**: `GetEnergyTrend` 聚合可能跨 168 小時數據，JOIN metrics + assets，無 query timeout。慢 client 可佔住 DB connection。

**修復**:
1. 在 query context 上加 timeout：
   ```go
   ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
   defer cancel()
   ```
2. 同樣處理 `GetEnergyBreakdown` 和 `GetEnergySummary`
3. Timeout 後回傳 504 Gateway Timeout 而非 500

**驗證**: `go test` + 手動測試大時間範圍 query

---

## D. Fallback Data i18n 混用

**文件**: `src/data/fallbacks/energy.ts`

**問題**: `FALLBACK_BOTTOM_STATS` 使用 `t()` 做 i18n（正確），但 `FALLBACK_POWER_TREND` 的 time labels 是硬編碼 `"00:00"` 格式（可接受）。`RACK_HEATMAP` 用硬編碼 rack names（不影響，因為是 ID）。`POWER_EVENTS` 已正確使用 `titleKey`/`descKey`。

**實際需要修的**: 
- `FALLBACK_BOTTOM_STATS` 的 `value` 欄位 (`'842.1 kW'` 等) 是硬編碼英文單位 — 但 kW 是國際單位，不需要翻譯
- 結論：**此項實際風險極低**，只需確認 fallback 使用時有 DEMO 標記即可

**修復**:
1. 在 EnergyMonitor 使用 fallback 數據時加 DEMO badge（類似 AlertTopology 的做法）
2. 不改 fallback 文件本身

**驗證**: 切斷 API，確認 DEMO badge 出現

---

## E. Sensor Alert Rule 去重

**文件**: `src/pages/SensorConfiguration.tsx` (lines 241-255)

**問題**: 同一 `metric_name` 如果有多條 severity=warning 的規則，後加入的會覆蓋前一條的 `warningId` 和閾值，靜默丟失數據。

**修復**:
1. 在 grouping 邏輯中，如果同一 metric+severity 已有值，保留 **最新建立的** 那條（按 `created_at` 排序）
2. 或者合併為陣列，UI 顯示多條規則時展開
3. 最簡單方案：先 sort by `created_at DESC`，grouping 時跳過已有值的 severity：
   ```ts
   const sorted = [...apiRules].sort((a, b) => 
     new Date(b.created_at ?? 0).getTime() - new Date(a.created_at ?? 0).getTime()
   );
   sorted.forEach(rule => {
     // ...existing logic but add:
     if (rule.severity === 'warning' && grouped[key].warningId) return; // skip older
   });
   ```

**驗證**: 建立兩條同 metric 同 severity 的規則，確認只顯示最新的

---

## F. Sensor Save 缺 Loading 狀態

**文件**: `src/pages/SensorConfiguration.tsx`

**問題**: 「Save Thresholds」按鈕在 `updateAlertRule.mutateAsync()` 執行期間沒有 loading 指示，使用者可能重複點擊。

**修復**:
1. 加 `const [saving, setSaving] = useState(false)` 狀態
2. Save handler 開始時 `setSaving(true)`，finally `setSaving(false)`
3. 按鈕加 `disabled={saving}` + spinner icon

**驗證**: 點 Save，確認按鈕顯示 loading 且不可重複點擊

---

## G. Sensor Polling 間隔功能實作

**文件**: 
- 前端: `src/pages/SensorConfiguration.tsx` (line 502)
- 後端: `cmdb-core/internal/api/sensor_endpoints.go`

**問題**: Polling interval dropdown 的 `onChange` 只顯示 "Coming Soon"，沒有實際更新。

**修復**:
1. 後端 `UpdateSensor` endpoint 已支持 `polling_interval` 欄位 — 確認
2. 前端移除 `toast.info('Coming Soon')`，改為：
   ```ts
   onChange={(e) => {
     const interval = parseInt(e.target.value);
     setGlobalPolling(interval);
     // Batch update all enabled sensors
     sensors.filter(s => s.enabled).forEach(s => {
       updateSensor.mutate({ id: s.id, data: { polling_interval: interval } });
     });
   }}
   ```
3. 加 toast 成功/失敗提示

**風險**: 批量更新多個 sensor 可能產生大量 API 呼叫 — 可考慮後端加 batch endpoint

**驗證**: 選擇不同 polling 間隔，確認 sensor 列表更新

---

## H. i18n Key 命名不一致（長期）

**問題**: 
- `monitoring.xxx` vs `system_health.xxx` vs `sensors.xxx` 混用不同命名空間
- 部分用 snake_case（`alert_trend_24h`），部分用 camelCase
- 部分 key 有前綴 `label_`、`btn_`，部分沒有

**修復策略（不建議一次性改）**:
1. 制定 i18n key 命名規範文檔
2. 新增的 key 必須遵循規範
3. 舊 key 在觸碰該頁面時順帶重命名
4. 規範建議：
   - namespace = page/feature name（`monitoring`、`sensors`、`energy`）
   - key = `{noun}_{descriptor}`（`alert_count`、`btn_export`）
   - 動作前綴：`btn_`、`label_`、`toast_`、`error_`
   - 全部 snake_case

**不建議**: 一次性 rename 所有 key — 影響面太大，容易遺漏

**驗證**: code review 新 PR 時檢查 i18n key 是否符合規範

---

## 建議執行順序

```
A (小, HIGH)  →  快速修完
C (小, MEDIUM) →  3 行改動
E (小, MEDIUM) →  5 行改動  
F (小, LOW)    →  10 行改動
D (小, MEDIUM) →  加 DEMO badge
G (中, LOW)    →  需確認後端 + 前端
B (中, MEDIUM) →  加 dev-only 檢查
H (大, LOW)    →  持續改善，不一次做
```

A → C → E → F → D 可以在一個 commit 內完成（都是小改動）。
G 和 B 各自一個 commit。H 長期執行。
