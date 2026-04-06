# Phase A：獨立頁面修復方案

> 9 個頁面，每個可獨立修復，零跨頁依賴
> 按修復難度排序：簡單 → 複雜

---

## A1. VideoPlayer（3/10 → 6/10）

**現狀：** 不讀 URL 參數，所有影片顯示同一內容

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | 不讀 `?v=` 參數 | — | 加 `useSearchParams` 讀 video ID，從 VideoLibrary 的 `VIDEOS` 靜態列表中找對應影片 | +8 |
| 2 | chapters hardcoded | 11-17 | 依照選中影片動態顯示（每個影片可有不同 chapters 結構）| +5 |
| 3 | 進度 31% hardcoded | 70, 178 | 改為 0%（無播放功能時顯示 "未開始"）或移除 | ~2 |
| 4 | Download SOP 按鈕 placeholder | 111 | 改為 `alert('Coming Soon')` 已處理 | 0 |

**不需後端改動。改動量：~15 行，僅改 1 檔案。**

---

## A2. Welcome（5/10 → 7/10）

**現狀：** 5 個 onboarding tab 顯示相同內容

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | tabs 不切換內容 | 71-86 | 每個 tab 對應不同的 feature card 子集或描述文字 | +30 |
| 2 | 配色與全站不一致 | 61 | 可保留（onboarding 用淺色主題作為差異化設計）| 0 |

**不需後端改動。改動量：~30 行，僅改 1 檔案。**

**具體方案：**
```tsx
// 定義每個 tab 的內容
const TAB_CONTENT: Record<string, { title: string; description: string; icon: string }[]> = {
  welcome: FEATURE_CARDS,
  connect: [
    { title: '連接數據源', description: '整合 VMware vCenter、SNMP、Zabbix 等數據來源', icon: 'cable' },
    { title: 'API 整合', description: '使用 REST API 或 Webhook 連接外部系統', icon: 'api' },
  ],
  analyze: [
    { title: 'BIA 影響分析', description: '自動評估業務系統重要性等級', icon: 'assessment' },
    { title: '數據質量治理', description: '四維度自動評分確保資料準確性', icon: 'verified' },
  ],
  secure: [
    { title: 'RBAC 權限控制', description: '基於角色的精細權限管理', icon: 'admin_panel_settings' },
    { title: '審計追蹤', description: '所有操作自動記錄可追溯', icon: 'history' },
  ],
  finish: [
    { title: '開始使用', description: '您的 CMDB 平台已就緒', icon: 'rocket_launch' },
  ],
}

// 渲染時用 activeTab 選擇內容
{TAB_CONTENT[activeTab]?.map(card => (
  <FeatureCard key={card.title} {...card} />
))}
```

---

## A3. UserProfile（5/10 → 7/10）

**現狀：** 只能更新 display_name，其他全 hardcoded

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | sessions hardcoded | 7-10 | 改為從 `user` object 衍生（顯示 "Current Session" + login time）| ~5 |
| 2 | department 不送 API | 99 | 加到 `updateUser.mutate` payload：`{ display_name, department }` — 但後端 UpdateUser 可能不支持 department 欄位。檢查 users 表有 `dept_id` 欄位，但 API 未暴露。**暫改為用 attributes JSON 存 department** | ~3 |
| 3 | Change Password placeholder | 123 | 保留 placeholder（需後端密碼更新端點，超出獨立修復範圍）| 0 |
| 4 | Reset 2FA placeholder | 143 | 保留 placeholder | 0 |
| 5 | Revoke Session placeholder | 164 | 保留 placeholder | 0 |
| 6 | "84.3%" connectivity hardcoded | 216 | 改用 `useSystemHealth` 的 DB latency 衍生：latency < 50ms → "Good"，否則 "Degraded" | ~5 |
| 7 | IP hardcoded | 59 | 改為 `—`（無法從前端取 client IP）| ~1 |
| 8 | notification toggles 不持久化 | 194 | 保留 local state（需後端 user preferences 端點，超出範圍）| 0 |

**不需後端改動。改動量：~15 行，僅改 1 檔案。**

---

## A4. AuditEventDetail（6/10 → 7/10）

**現狀：** 大量 fallback 數據，diffMode 無法切換

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | diffMode 初始化但無 setter 暴露 | ~130 | 加 toggle button：`<button onClick={() => setDiffMode(m => m === 'side' ? 'inline' : 'side')}>` | +3 |
| 2 | Export Log placeholder | 169 | 改為：`onClick={() => { const blob = new Blob([JSON.stringify(event, null, 2)]); const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = 'audit-event.json'; a.click() }}` | +5 |
| 3 | delta line count hardcoded 12 | 321 | 改為 `diffLines.length` | ~1 |
| 4 | metadata 永遠 fallback | — | 從 API event 的 `diff` JSON 提取更多欄位（source IP 等在 audit event 的 diff 中可能有）| ~5 |

**不需後端改動。改動量：~15 行，僅改 1 檔案。**

---

## A5. AuditHistory（6/10 → 8/10）

**現狀：** 3 個 filter 全不生效，breadcrumb hardcoded

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | Search 無 onChange | 221 | 加 `const [search, setSearch] = useState('')` + `onChange={e => setSearch(e.target.value)}` + 在 table 渲染前 filter：`entries.filter(e => e.action.includes(search) \|\| e.module?.includes(search))` | +8 |
| 2 | Event type filter 無 onChange | 229 | 加 `const [eventTypeFilter, setEventTypeFilter] = useState('')` + `onChange` + 傳入 API query 的 `module` 參數 | +8 |
| 3 | User filter 無 onChange | 236 | 加 `const [userFilter, setUserFilter] = useState('')` + client-side filter on `operator_id` | +5 |
| 4 | Date range hardcoded 文字 | 245 | 改為兩個 date input（from/to），client-side filter by `created_at` | +10 |
| 5 | Advanced Filters placeholder | 251 | 保留 placeholder | 0 |
| 6 | Run Report placeholder | 378 | 改為 JSON 導出（同 A4 的 Export 邏輯）| +5 |
| 7 | Breadcrumb hardcoded "SRV-PROD-001" | 131 | 改為 "Audit History"（通用，不綁定特定資產）| ~2 |
| 8 | Stats "47 config changes" hardcoded | — | 從 `auditEvents.length` 衍生 | ~3 |
| 9 | Row expand vs navigate 衝突 | 319 | expand button 加 `e.stopPropagation()` | +1 |

**不需後端改動。改動量：~40 行，僅改 1 檔案。**

---

## A6. SystemHealth（5/10 → 7/10）

**現狀：** 只有 DB latency 和 critical alerts 來自 API，其餘全 hardcoded

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | HEALTH_SEGMENTS donut hardcoded | 13-17 | 從 `useAlerts()` 統計：`critical = alerts.filter(a => a.severity === 'critical').length`，`warning = ...`，`healthy = total_assets - critical - warning` | +10 |
| 2 | TREND_BARS hardcoded | 19-28 | 保留（需 time-series 聚合端點，超出獨立範圍）| 0 |
| 3 | Uptime "99.992%" | 157 | 從 `useSystemHealth` 的 DB status 衍生：DB ok → "99.99%"，error → "Degraded" | ~3 |
| 4 | Managed nodes "2,847" | 225 | 用 `useAssets()` 取 `pagination.total`：`const { data: assetsResp } = useAssets({ page_size: 1 })`，取 `assetsResp?.pagination?.total` | +5 |
| 5 | Resource storage/power/memory | 333-367 | 保留 hardcoded（需後端 resource monitoring 端點）| 0 |
| 6 | Sync progress 76% | 195 | 改為 "N/A"（無對應 API）| ~1 |
| 7 | BIA Tier 列顯示 "--" | 386 | 從 assets 做 lookup：`useAssets()` 取所有資產，建 Map `assetId → bia_level`，在 alert row 中查詢 | +10 |

**不需後端改動。改動量：~30 行，僅改 1 檔案。**

---

## A7. SensorConfiguration（6/10 → 8/10）

**現狀：** 已接通 useAlertRules，但 7 個 placeholder 按鈕 + 不完整的 CRUD

**修復項：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | Add New Rule placeholder | 684 | 改為開啟 inline form（name + metric_name + condition JSON + severity + enabled），submit 用 `useCreateAlertRule`（hook 已存在於 useMonitoring.ts） | +30 |
| 2 | Delete rule placeholder | 672 | 需後端 DELETE /monitoring/rules/{id}。**如不存在，保留 placeholder** | 0 或 +5 |
| 3 | Edit rule placeholder | 664 | 改為 inline edit：click → 該 row 變為 input，Save 用 `useUpdateAlertRule` | +20 |
| 4 | Reset to Defaults | 600 | `setThresholds(THRESHOLDS)` reset 為初始值 | +3 |
| 5 | Export Configuration | 608 | JSON.stringify(thresholds + rules) → Blob download | +5 |
| 6 | Import Configuration | 616 | file input → parse JSON → setThresholds/setRules | +10 |
| 7 | Discover Sensors | 359 | 保留 placeholder（需後端網路掃描功能）| 0 |
| 8 | Polling interval stub | 468 | 保留 placeholder（sensor 為合成實體，無持久化模型）| 0 |
| 9 | Threshold warning < critical 校驗 | — | slider onChange 時校驗：if warning >= critical → 不允許 | +5 |

**需檢查：** 後端是否有 `DELETE /monitoring/rules/{id}` 端點。如無，#2 保留 placeholder。

**改動量：~70 行，僅改 1 檔案（+ 可能 useMonitoring.ts 加 useDeleteAlertRule hook）。**

---

## A8. EquipmentHealth（3/10 → 5/10）

**現狀：** 幾乎 100% hardcoded，無對應後端

**修復項（有限度改善）：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | healthScore hardcoded 92.4 | 10 | 從 `useAssets({ type: 'server' })` 計算：`operational / total * 100` | +5 |
| 2 | metrics 4 項 hardcoded | 12-45 | 從 assets 統計衍生：service stability = operational%，hardware lifespan = (total - maintenance) / total * 100 | +10 |
| 3 | sensorReadings hardcoded | 47-66 | 用 `useMetrics` 取第一個 server 的 temperature + power_kw | +8 |
| 4 | warningMessage hardcoded | 68-72 | 用 `useAlerts({ severity: 'warning' })` 取最近一條 warning | +5 |
| 5 | riskAssessment hardcoded | 74-79 | 從 alerts 統計：critical 數量 > 3 → "HIGH"，> 0 → "ELEVATED"，0 → "LOW" | +5 |
| 6 | Last sync hardcoded | 108 | `new Date().toISOString()` | ~1 |
| 7 | Export Full Report null action | 326 | `alert('Coming Soon')` | ~1 |

**不需後端改動（用現有 API 衍生）。改動量：~35 行，僅改 1 檔案。**

**限制：** 此頁需要真正的 equipment health API 才能達到 8+/10。目前的修復是「用已有 API 盡量衍生」，從 3/10 提升到 5/10。

---

## A9. ComponentUpgrade（3/10 → 5/10）

**現狀：** 100% hardcoded，無推薦引擎

**修復項（有限度改善）：**

| # | 問題 | 行號 | 修復方案 | 行數 |
|---|------|------|---------|------|
| 1 | initialCards hardcoded | 42-103 | 保留靜態卡片（無推薦引擎 API），但改為從 assets API 動態計算 impact metrics | +10 |
| 2 | impactMetricKeys hardcoded | 21-24 | 改為：fleet health = `operational assets / total * 100`%，power efficiency 保留 hardcoded | +5 |
| 3 | Learn More placeholder | 285 | 改為展開卡片詳情（toggle 顯示更多描述）| +10 |
| 4 | Request Selection placeholder | 329 | 改為：`navigate('/maintenance/add')` 預帶選中卡片的標題 | +3 |
| 5 | assetCount 計算但未渲染 | 116 | 在頁面 header 加 "Based on {assetCount} assets" | +2 |

**不需後端改動。改動量：~30 行，僅改 1 檔案。**

**限制：** 同 A8，需要推薦引擎 API 才能達到 8+/10。

---

## 修復統計總覽

| 頁面 | 當前分 | 修復後 | 改動行數 | 後端改動 |
|------|--------|--------|---------|---------|
| A1 VideoPlayer | 3 | 6 | ~15 | 無 |
| A2 Welcome | 5 | 7 | ~30 | 無 |
| A3 UserProfile | 5 | 7 | ~15 | 無 |
| A4 AuditEventDetail | 6 | 7 | ~15 | 無 |
| A5 AuditHistory | 6 | 8 | ~40 | 無 |
| A6 SystemHealth | 5 | 7 | ~30 | 無 |
| A7 SensorConfig | 6 | 8 | ~70 | 可能加 DELETE 端點 |
| A8 EquipmentHealth | 3 | 5 | ~35 | 無 |
| A9 ComponentUpgrade | 3 | 5 | ~30 | 無 |
| **合計** | **42** | **60** | **~280** | **0-1 個端點** |

**平台總分影響：** 9 頁平均從 4.7 → 6.7（+2.0），帶動平台總分從 72 → ~76

---

## 執行順序建議

```
最快見效（< 15 行）：A1 → A3 → A4
中等改動（15-40 行）：A2 → A5 → A6 → A8 → A9
最複雜（70 行）：A7
```
