# IronGrid Enterprise CMDB -- 位置階層鑽取系統設計報告

> 文件版本: v1.0
> 日期: 2026-03-28
> 狀態: Proposed
> 範圍: 位置階層導航 (Country → Region → City → Campus → IDC → Module → Rack → U-position)

---

## 目錄

1. [現狀分析](#1-現狀分析)
2. [設計目標與約束](#2-設計目標與約束)
3. [頁面清冊 (Page Inventory)](#3-頁面清冊)
4. [路由結構 (Routing Structure)](#4-路由結構)
5. [導航流程 (Navigation Flow)](#5-導航流程)
6. [各層級頁面內容設計](#6-各層級頁面內容設計)
7. [麵包屑行為 (Breadcrumb Behavior)](#7-麵包屑行為)
8. [資料聚合邏輯 (Data Aggregation)](#8-資料聚合邏輯)
9. [對既有頁面的影響](#9-對既有頁面的影響)
10. [i18n 策略](#10-i18n-策略)
11. [架構決策紀錄 (ADR)](#11-架構決策紀錄)
12. [實作分期建議](#12-實作分期建議)

---

## 1. 現狀分析

### 1.1 既有路由結構

目前系統共 52 個頁面，所有頁面均在 IDC 層級運作（硬編碼為 IDC-01）。路由為扁平結構，無位置參數：

```
/dashboard          -- IDC 儀表板（含不可點擊的麵包屑）
/racks              -- 機櫃管理（Module 層級）
/racks/3d           -- 3D 機房（含左側位置樹狀結構，但僅為展示）
/racks/visualization -- 機櫃視覺化（U-position 層級）
/racks/facility-map  -- 設施地圖（Module → Rack 平面圖）
/racks/console       -- 機櫃控制台
/racks/2d            -- 2D 正面圖
...其餘 46 頁均為 IDC 固定範圍
```

### 1.2 核心問題

```
                     目前系統邊界
                          |
    Country → Region → City → Campus → [IDC] → Module → Rack → U
    ~~~~~~   ~~~~~~   ~~~~   ~~~~~~     ^^^
           完全缺失                   所有頁面固定在此
```

- Dashboard 上的麵包屑 `CHINA > EAST > SHANGHAI > PUDONG > IDC-01` 為純文字，不可點擊
- DataCenter3D 左側面板有 locationTree，但僅為靜態展示用的 mock 資料
- 不存在任何 URL 參數化機制——無法透過 URL 切換位置上下文
- 所有 KPI、告警、資產數據均為 IDC 範圍，無法向上聚合

### 1.3 既有設計語言

- 深色 Industrial Brutalism 主題
- Material Symbols Outlined 圖標
- Tailwind CSS + 自訂色彩 token（`surface`, `surface-container`, `on-surface-variant` 等）
- `font-headline` 用於標題，全大寫 `tracking-wider` 用於標籤
- 卡片為 `rounded-lg bg-surface-container p-5`
- react-router-dom v6（`<Routes>` / `<Route>` / `lazy()`）
- react-i18next 搭配 `zh-TW` 為 fallback 語言

---

## 2. 設計目標與約束

### 2.1 設計目標

| 目標 | 描述 |
|------|------|
| G1 | 使用者可從全球總覽逐步鑽取至單一 U-position |
| G2 | 每一層級提供有意義的聚合洞察，而非僅作為路過列表 |
| G3 | 全球層級具備「指揮中心」氛圍 |
| G4 | URL 完整反映位置上下文，可書籤、可分享 |
| G5 | 既有 52 個頁面的改動最小化 |
| G6 | 向下鑽取與向上返回皆流暢 |

### 2.2 約束

| 約束 | 原因 |
|------|------|
| C1: 維持 react-router-dom v6 | 技術棧一致性 |
| C2: 維持 lazy() 分頁載入 | 效能 |
| C3: i18n 支援 EN / zh-CN / zh-TW | 國際化需求 |
| C4: Dark Industrial Brutalism 視覺風格 | 品牌一致性 |
| C5: 不改變 MainLayout sidebar 結構 | 降低風險 |

---

## 3. 頁面清冊

### 3.1 新增頁面（共 4 個）

| # | 頁面名稱 | 檔案路徑 | 對應層級 | 主要用途 |
|---|---------|---------|---------|---------|
| 1 | GlobalOverview | `src/pages/locations/GlobalOverview.tsx` | Country (L0) | 全球指揮中心：世界地圖 + 各國 KPI 彙總 |
| 2 | RegionOverview | `src/pages/locations/RegionOverview.tsx` | Region (L1) | 區域地圖 + 區域內城市 KPI 比較 |
| 3 | CityOverview | `src/pages/locations/CityOverview.tsx` | City (L2) | 城市地圖 + 園區清單與 KPI |
| 4 | CampusOverview | `src/pages/locations/CampusOverview.tsx` | Campus (L3) | 園區配置圖 + IDC 清單與 KPI |

### 3.2 既有頁面角色重新定義

| 既有頁面 | 原角色 | 新角色 | 改動程度 |
|---------|-------|-------|---------|
| Dashboard | IDC 固定儀表板 | IDC 層級儀表板（參數化） | 中 |
| DataCenter3D | 靜態 3D 展示 | Module 層級 3D 鑽取入口 | 小 |
| FacilityMap | 靜態設施地圖 | Module 層級平面圖 | 小 |
| RackManagement | 機櫃列表 | Rack 層級管理（參數化） | 小 |
| RackVisualization | U-position 視覺化 | U-position 層級（參數化） | 小 |

### 3.3 不需要新增頁面的層級

- **IDC (L4)**: 既有 Dashboard 重新參數化即可
- **Module (L5)**: 既有 DataCenter3D + FacilityMap 已涵蓋
- **Rack (L6)**: 既有 RackManagement / RackDetail 已涵蓋
- **U-position (L7)**: 既有 RackVisualization / RackFrontView2D 已涵蓋

---

## 4. 路由結構

### 4.1 位置階層路由（新增）

採用巢狀路由搭配 URL slug 參數，每一層級用其 slug 作為路徑段：

```
/locations                                          → GlobalOverview
/locations/:countrySlug                             → RegionOverview
/locations/:countrySlug/:regionSlug                 → CityOverview
/locations/:countrySlug/:regionSlug/:citySlug       → CampusOverview
/locations/:countrySlug/:regionSlug/:citySlug/:campusSlug
                                                    → IDC 列表（CampusOverview 子視圖）
/locations/:countrySlug/:regionSlug/:citySlug/:campusSlug/:idcSlug
                                                    → Dashboard（IDC 儀表板）
```

### 4.2 具體 URL 範例

```
/locations                                    全球總覽
/locations/china                              中國各區域總覽
/locations/china/east                         華東區各城市
/locations/china/east/shanghai                上海各園區
/locations/china/east/shanghai/pudong         浦東園區各 IDC
/locations/china/east/shanghai/pudong/idc-01  IDC-01 儀表板
```

### 4.3 IDC 層級以下（既有路由改造）

IDC 確定後，進入既有功能頁面。既有路由需加上位置前綴：

```
方案 A: 巢狀路由（推薦）
/locations/.../idc-01/racks                     機櫃管理
/locations/.../idc-01/racks/3d                  3D 機房
/locations/.../idc-01/racks/:rackId/visualization  機櫃視覺化
/locations/.../idc-01/assets                    資產管理
/locations/.../idc-01/monitoring                監控告警
...

方案 B: 全域位置上下文 + 既有路由不變（備選）
保留既有 /racks、/assets 等路由不變，
透過 React Context 或 URL query 參數 ?idc=idc-01 傳遞位置上下文。
```

### 4.4 路由方案決策

**採用方案 B（全域位置上下文）作為第一階段實作，方案 A 作為長期目標。**

理由：
- 方案 A 需要改動全部 52 個頁面的路由，風險極高
- 方案 B 只需引入一個 `LocationContext`，既有頁面透過 context 讀取當前位置
- 方案 B 可在不破壞既有路由的前提下逐步遷移

```
第一階段路由結構:

新增:
  /locations                                      → GlobalOverview
  /locations/:countrySlug                         → RegionOverview
  /locations/:countrySlug/:regionSlug             → CityOverview
  /locations/:countrySlug/:regionSlug/:citySlug   → CampusOverview
  /locations/.../:campusSlug/:idcSlug             → 重導向至 /dashboard 並設定 context

保持不變:
  /dashboard        (讀取 LocationContext)
  /racks            (讀取 LocationContext)
  /assets           (讀取 LocationContext)
  /monitoring       (讀取 LocationContext)
  ...其餘 48 頁
```

### 4.5 App.tsx 路由宣告（概念）

```
<Route element={<MainLayout />}>
  {/* 新增: 位置階層頁面 */}
  <Route path="/locations" element={<GlobalOverview />} />
  <Route path="/locations/:countrySlug" element={<RegionOverview />} />
  <Route path="/locations/:countrySlug/:regionSlug" element={<CityOverview />} />
  <Route path="/locations/:countrySlug/:regionSlug/:citySlug" element={<CampusOverview />} />
  <Route path="/locations/:countrySlug/:regionSlug/:citySlug/:campusSlug/:idcSlug"
         element={<IdcRedirect />} />

  {/* 既有路由不變 */}
  <Route path="/dashboard" element={<Dashboard />} />
  <Route path="/racks" element={<RackManagement />} />
  ...
</Route>
```

---

## 5. 導航流程

### 5.1 向下鑽取流程

```
  ┌─────────────────┐
  │  GlobalOverview  │  /locations
  │  (世界地圖)       │
  └────────┬────────┘
           │ 點擊國家卡片/地圖區域
           ▼
  ┌─────────────────┐
  │  RegionOverview  │  /locations/china
  │  (區域地圖)       │
  └────────┬────────┘
           │ 點擊區域卡片/地圖標記
           ▼
  ┌─────────────────┐
  │  CityOverview    │  /locations/china/east
  │  (城市地圖)       │
  └────────┬────────┘
           │ 點擊城市卡片
           ▼
  ┌─────────────────┐
  │  CampusOverview  │  /locations/china/east/shanghai
  │  (園區配置圖)     │
  └────────┬────────┘
           │ 點擊 IDC 卡片
           ▼
  ┌─────────────────┐
  │  Dashboard       │  /dashboard (context = IDC-01)
  │  (IDC 儀表板)     │
  └────────┬────────┘
           │ 既有流程: 點擊 3D 機房 / 設施地圖 等
           ▼
  ┌─────────────────┐
  │  DataCenter3D    │  /racks/3d
  │  FacilityMap     │  /racks/facility-map
  │  (Module 層級)    │
  └────────┬────────┘
           │ 點擊機櫃
           ▼
  ┌─────────────────┐
  │  RackDetail      │  /racks/detail
  │  RackConsole     │  /racks/console
  │  (Rack 層級)      │
  └────────┬────────┘
           │ 點擊 U-position
           ▼
  ┌─────────────────┐
  │ RackVisualization│  /racks/visualization
  │ RackFrontView2D  │  /racks/2d
  │ (U-position)     │
  └─────────────────┘
```

### 5.2 向上返回流程

三種向上返回機制：

1. **麵包屑 (Breadcrumb)** -- 點擊任意上層節點直接跳轉
2. **瀏覽器返回鍵** -- 正常 history 堆疊
3. **Sidebar 導航** -- 新增「位置總覽」頂層入口，直接回到 `/locations`

### 5.3 跨層級快速跳轉

在 MainLayout 頂部工具列的搜尋框（CMD+K）中支援位置搜尋：

```
搜尋: "pudong" → 結果: [CHINA > EAST > SHANGHAI > PUDONG] → 點擊直接進入 CampusOverview
搜尋: "idc-01" → 結果: [CHINA > EAST > SHANGHAI > PUDONG > IDC-01] → 點擊進入 Dashboard
```

---

## 6. 各層級頁面內容設計

### 6.1 Level 0 -- GlobalOverview（全球指揮中心）

**定位**: 集團高管的第一進入點，「一眼掌握全球資料中心版圖」。

**版面配置**:
```
┌────────────────────────────────────────────────────────────┐
│ IRONGRID GLOBAL COMMAND CENTER                             │
│ 3 Countries  │  7 Regions  │  12 Cities  │  18 IDCs       │
├────────────────────────────────────────────────────────────┤
│                                                            │
│   ┌──────────────────────────────────┐ ┌────────────────┐  │
│   │                                  │ │ 全球 KPI 彙總    │  │
│   │       世界地圖                     │ │                │  │
│   │       (標記各國 IDC 數量/狀態)      │ │ 總資產: 38,420  │  │
│   │                                  │ │ 總機櫃: 2,840   │  │
│   │   ● 中國 (12 IDC)                │ │ 平均 PUE: 1.28  │  │
│   │              ● 日本 (4 IDC)       │ │ 告警: 12        │  │
│   │                                  │ │                │  │
│   │         ● 新加坡 (2 IDC)          │ │ [能耗趨勢圖]     │  │
│   │                                  │ │                │  │
│   └──────────────────────────────────┘ └────────────────┘  │
│                                                            │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐                    │
│ │ 中國      │ │ 日本      │ │ 新加坡    │  ← 國家卡片      │
│ │ 12 IDC   │ │ 4 IDC    │ │ 2 IDC    │    (可點擊鑽取)   │
│ │ 28,000資產│ │ 6,800資產 │ │ 3,620資產 │                   │
│ │ PUE 1.25 │ │ PUE 1.32 │ │ PUE 1.18 │                   │
│ │ ■■■■□ 84%│ │ ■■■□□ 72%│ │ ■■■■□ 78%│  ← 機櫃使用率     │
│ └──────────┘ └──────────┘ └──────────┘                    │
│                                                            │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ 全球即時告警流 (最新 10 筆嚴重告警, 跨所有 IDC)         │   │
│ └──────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────┘
```

**KPI 聚合指標**:

| 指標 | 聚合方式 | 展示形式 |
|------|---------|---------|
| IDC 總數 | COUNT(idcs) | 數字 |
| 機櫃總數 | SUM(racks) | 數字 |
| 資產總數 | SUM(assets) | 數字 + 月增率 |
| 全球平均 PUE | WEIGHTED_AVG(pue, power) | 數字 + 趨勢線 |
| 總能耗 (kW) | SUM(power_consumption) | 數字 + 趨勢圖 |
| 機櫃平均使用率 | AVG(rack_occupancy) | 百分比 + 長條圖 |
| 嚴重告警數 | COUNT(alerts WHERE severity=CRITICAL) | 數字 (紅色) |
| 維護中 IDC 數 | COUNT(idcs WHERE status=MAINTENANCE) | 數字 (橘色) |

**視覺化組件**:
- 世界地圖（SVG 或 Canvas）: 標記各國位置，圓圈大小表示 IDC 規模，顏色表示健康狀態
- 國家卡片網格: 每國一張卡片，顯示核心 KPI，點擊進入 RegionOverview
- 全球告警時間線: 橫向捲動，顯示跨 IDC 的最新嚴重告警
- 能耗趨勢折線圖: 過去 7 天/30 天全球總能耗

**互動行為**:
- 地圖上 hover 國家 → 彈出 tooltip 顯示核心 KPI
- 點擊國家標記或國家卡片 → 導航至 `/locations/:countrySlug`
- 地圖支援縮放與平移


### 6.2 Level 1 -- RegionOverview（區域總覽）

**定位**: 展示單一國家內各區域的比較分析。

**版面配置**:
```
┌────────────────────────────────────────────────────────────┐
│ [麵包屑] GLOBAL > CHINA                                    │
│ 中國資料中心總覽                                             │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────┐  │
│  │ 總 IDC: 12  │ │ 機櫃: 1840  │ │ 資產: 28k  │ │告警: 8 │  │
│  └────────────┘ └────────────┘ └────────────┘ └────────┘  │
│                                                            │
│  ┌───────────────────────────┐ ┌──────────────────────────┐│
│  │                           │ │ 區域比較 (橫向長條圖)      ││
│  │   國家地圖                  │ │                          ││
│  │   (標記各區域)              │ │ 華東 ■■■■■■■□□ 78%      ││
│  │                           │ │ 華南 ■■■■■■□□□ 65%      ││
│  │   ● 華東 (5 IDC)          │ │ 華北 ■■■■■□□□□ 55%      ││
│  │   ● 華南 (4 IDC)          │ │ 西南 ■■■□□□□□□ 32%      ││
│  │   ● 華北 (2 IDC)          │ │                          ││
│  │   ● 西南 (1 IDC)          │ │ [PUE 比較]               ││
│  │                           │ │ [能耗比較]                ││
│  └───────────────────────────┘ └──────────────────────────┘│
│                                                            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│  │ 華東      │ │ 華南      │ │ 華北      │ │ 西南      │     │
│  │ 5 IDC    │ │ 4 IDC    │ │ 2 IDC    │ │ 1 IDC    │     │
│  │ 820 racks│ │ 560 racks│ │ 340 racks│ │ 120 racks│     │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘     │
└────────────────────────────────────────────────────────────┘
```

**KPI 聚合指標**:

| 指標 | 聚合範圍 | 展示形式 |
|------|---------|---------|
| 區域內 IDC 總數 | 該國家所有區域 | 統計卡 |
| 各區域機櫃使用率 | 按區域分組 | 橫向長條圖比較 |
| 各區域 PUE | 按區域分組 | 排名列表 |
| 區域能耗占比 | 按區域分組 | 環形圖 |
| 區域告警分佈 | 按區域分組 | 堆疊長條圖 |

**視覺化組件**:
- 國家地圖（簡化 SVG）: 標記各區域位置
- 區域比較長條圖: 橫向排列，比較各區域核心指標
- 區域卡片網格: 每區域一張，點擊進入 CityOverview
- PUE 排名: 區域由優至劣排序

**互動行為**:
- 切換比較維度: 機櫃使用率 / PUE / 能耗 / 告警數
- 點擊區域卡片或地圖標記 → 導航至 `/locations/:countrySlug/:regionSlug`


### 6.3 Level 2 -- CityOverview（城市總覽）

**定位**: 展示單一區域內各城市的園區分佈與運營狀態。

**版面配置**:
```
┌────────────────────────────────────────────────────────────┐
│ [麵包屑] GLOBAL > CHINA > EAST                             │
│ 華東區資料中心總覽                                           │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐  │
│  │IDC: 5  │ │園區: 8  │ │機櫃:820│ │PUE:1.25│ │告警: 3 │  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘  │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ 城市卡片 (可切換: 卡片視圖 / 列表視圖)                    │  │
│  │                                                      │  │
│  │ ┌─────────────────┐  ┌─────────────────┐             │  │
│  │ │ 上海              │  │ 南京              │             │  │
│  │ │ 3 Campus, 4 IDC  │  │ 2 Campus, 1 IDC  │             │  │
│  │ │ 480 Racks        │  │ 120 Racks        │             │  │
│  │ │ PUE 1.22         │  │ PUE 1.31         │             │  │
│  │ │ ■■■■■■□□ 84%     │  │ ■■■■□□□□ 56%     │             │  │
│  │ │ [迷你能耗趨勢]     │  │ [迷你能耗趨勢]     │             │  │
│  │ │ 🔴 1 Critical     │  │ ✅ All Normal     │             │  │
│  │ └─────────────────┘  └─────────────────┘             │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ 城市間 KPI 比較雷達圖 (PUE / 使用率 / 能耗 / 可靠度)    │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

**KPI 聚合指標**:

| 指標 | 聚合範圍 | 展示形式 |
|------|---------|---------|
| 城市內園區數 | 按城市分組 | 統計卡 |
| 城市內 IDC 數 | 按城市分組 | 統計卡 |
| 城市機櫃使用率 | 按城市加權平均 | 進度條 |
| 城市 PUE | 按城市加權平均 | 數字 |
| 城市告警摘要 | 按城市分組 | 狀態徽章 |
| 城市能耗趨勢 | 按城市時間序列 | 迷你折線圖 (sparkline) |

**互動行為**:
- 卡片視圖 / 列表視圖 切換 toggle
- 排序: 按名稱 / 按 IDC 數量 / 按使用率 / 按告警數
- 點擊城市卡片 → 導航至 `/locations/:countrySlug/:regionSlug/:citySlug`


### 6.4 Level 3 -- CampusOverview（園區總覽）

**定位**: 展示單一城市內各園區的佈局，以及園區內 IDC 清單。此為進入 IDC 操作層的最後一個「總覽」層級。

**版面配置**:
```
┌────────────────────────────────────────────────────────────┐
│ [麵包屑] GLOBAL > CHINA > EAST > SHANGHAI                  │
│ 上海資料中心園區                                             │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐             │
│  │園區: 3  │ │IDC: 4  │ │機櫃:480│ │告警: 1 │             │
│  └────────┘ └────────┘ └────────┘ └────────┘             │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                                                      │  │
│  │  園區 A: 浦東園區                                      │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  地址: 上海市浦東新區張江高科技園區                    │  │  │
│  │  │                                                │  │  │
│  │  │  IDC 列表:                                      │  │  │
│  │  │  ┌──────────┐ ┌──────────┐ ┌──────────┐       │  │  │
│  │  │  │ IDC-01   │ │ IDC-02   │ │ IDC-03   │       │  │  │
│  │  │  │ 12 Mod   │ │ 8 Mod    │ │ 6 Mod    │       │  │  │
│  │  │  │ 240 Rack │ │ 160 Rack │ │ 80 Rack  │       │  │  │
│  │  │  │ PUE 1.22 │ │ PUE 1.19 │ │ PUE 1.28 │       │  │  │
│  │  │  │ 84% used │ │ 72% used │ │ 45% used │       │  │  │
│  │  │  │ 🔴 1 告警 │ │ ✅ 正常   │ │ ✅ 正常   │       │  │  │
│  │  │  └──────────┘ └──────────┘ └──────────┘       │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  │                                                      │  │
│  │  園區 B: 嘉定園區                                      │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  IDC 列表:                                      │  │  │
│  │  │  ┌──────────┐                                  │  │  │
│  │  │  │ IDC-04   │                                  │  │  │
│  │  │  │ ...      │                                  │  │  │
│  │  │  └──────────┘                                  │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ 園區間容量規劃比較 (堆疊長條圖: 已用/可用/預留)          │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

**KPI 聚合指標**:

| 指標 | 聚合範圍 | 展示形式 |
|------|---------|---------|
| 園區內 IDC 數 | 按園區分組 | 統計卡 |
| 園區內 Module 數 | 按園區→IDC 分組 | IDC 卡片內顯示 |
| 園區內機櫃數 / 使用率 | 按園區→IDC→Rack 聚合 | 進度條 |
| IDC 層級 PUE | 各 IDC 獨立 | 數字 |
| IDC 電力容量 vs 使用量 | 各 IDC 獨立 | 堆疊長條圖 |
| IDC 告警摘要 | 各 IDC 獨立 | 狀態徽章 |

**互動行為**:
- 園區以手風琴或分組卡片展示
- 點擊 IDC 卡片 → 設定 LocationContext 並導航至 `/dashboard`
- 園區卡片可展開/收合顯示 IDC 列表


### 6.5 Level 4 -- Dashboard / IDC 儀表板（既有改造）

既有 Dashboard 頁面。改造重點：

- 麵包屑從硬編碼改為動態讀取 LocationContext
- KPI 數據從硬編碼改為根據 LocationContext 中的 IDC ID 載入
- 新增「返回園區」按鈕

**不需改變的部分**: 版面配置、視覺風格、卡片結構、heatmap 等均保持不變。


### 6.6 Level 5-7 -- Module / Rack / U-position（既有頁面微調）

既有的 DataCenter3D、FacilityMap、RackManagement、RackVisualization、RackFrontView2D、RackConsole 等頁面。

改造重點：
- 左側 locationTree（DataCenter3D 中已有）改為讀取真實資料
- 各頁面讀取 LocationContext 以確定資料範圍
- 麵包屑動態化

---

## 7. 麵包屑行為

### 7.1 麵包屑結構

麵包屑反映使用者在位置階層中的完整路徑，每一節點皆可點擊。

```
位置階層頁面:
  GLOBAL > CHINA > EAST > SHANGHAI

IDC 及以下頁面:
  GLOBAL > CHINA > EAST > SHANGHAI > PUDONG > IDC-01 [> 3D 機房]
  └─────────── 位置階層 ──────────────────┘   └ 功能頁面 ┘
```

### 7.2 麵包屑組件設計

建立 `LocationBreadcrumb` 共用組件，取代目前 Dashboard 和 FacilityMap 中各自硬編碼的麵包屑。

**資料來源**: LocationContext 提供完整的位置路徑陣列。

**行為規則**:

| 點擊節點 | 導航目標 |
|---------|---------|
| GLOBAL | `/locations` |
| CHINA | `/locations/china` |
| EAST | `/locations/china/east` |
| SHANGHAI | `/locations/china/east/shanghai` |
| PUDONG | `/locations/china/east/shanghai/pudong` |
| IDC-01 | `/dashboard` (設定 context) |
| 3D 機房 | `/racks/3d` (既有路由) |

### 7.3 麵包屑顯示名稱

每個節點的顯示名稱根據語言設定動態切換：

| 層級 | EN | zh-CN | zh-TW |
|------|-----|-------|-------|
| Country | CHINA | 中国 | 中國 |
| Region | EAST | 华东 | 華東 |
| City | SHANGHAI | 上海 | 上海 |
| Campus | PUDONG | 浦东 | 浦東 |
| IDC | IDC-01 | IDC-01 | IDC-01 |

技術名稱（如 IDC-01）不需翻譯，地理名稱需 i18n 支援。

### 7.4 視覺風格

沿用既有設計：
```
text-xs uppercase tracking-widest text-on-surface-variant
分隔符: › (text-[10px] opacity-40)
hover: hover:text-primary
當前節點: text-on-surface font-semibold (不可點擊)
```

---

## 8. 資料聚合邏輯

### 8.1 聚合層級模型

```
                    聚合方向 (自底向上)
                         ↑
  ┌─────────────────────────────────────────────┐
  │ Country                                     │
  │   assets = SUM(region.assets)               │
  │   racks  = SUM(region.racks)                │
  │   pue    = WEIGHTED_AVG(region.pue,         │
  │                         region.power)       │
  │   alerts = SUM(region.alerts)               │
  │   power  = SUM(region.power)                │
  ├─────────────────────────────────────────────┤
  │ Region                                      │
  │   assets = SUM(city.assets)                 │
  │   racks  = SUM(city.racks)                  │
  │   pue    = WEIGHTED_AVG(city.pue,           │
  │                         city.power)         │
  │   alerts = SUM(city.alerts)                 │
  │   power  = SUM(city.power)                  │
  ├─────────────────────────────────────────────┤
  │ City                                        │
  │   assets = SUM(campus.assets)               │
  │   ...同上模式...                              │
  ├─────────────────────────────────────────────┤
  │ Campus                                      │
  │   assets = SUM(idc.assets)                  │
  │   ...同上模式...                              │
  ├─────────────────────────────────────────────┤
  │ IDC (基礎層 -- 直接查詢)                      │
  │   assets = COUNT(assets WHERE idc_id = ?)   │
  │   racks  = COUNT(racks WHERE idc_id = ?)    │
  │   pue    = 直接量測值                         │
  │   alerts = COUNT(active alerts)             │
  │   power  = 直接量測值                         │
  └─────────────────────────────────────────────┘
```

### 8.2 關鍵聚合規則

| 指標 | 聚合方法 | 備註 |
|------|---------|------|
| 資產總數 | SUM | 可加性指標，直接加總 |
| 機櫃總數 | SUM | 可加性指標 |
| 機櫃使用率 | WEIGHTED_AVG(使用率, 機櫃數) | 不可直接平均，需按機櫃數加權 |
| PUE | WEIGHTED_AVG(pue, 總功耗) | 不可直接平均，需按功耗加權 |
| 總功耗 (kW) | SUM | 可加性指標 |
| 告警數 | SUM | 可加性指標 |
| 溫度 | MAX / AVG (視場景) | 告警用 MAX，統計用 AVG |
| 維護中設備數 | SUM | 可加性指標 |

### 8.3 聚合實作策略

**前端 Mock 階段**: 在位置階層頁面中使用靜態 mock 資料（與既有頁面一致的 mock 模式）。

**後端 API 階段（未來）**: 建議後端提供兩種 API：

```
1. 概覽 API（預計算/快取）:
   GET /api/locations/:level/:id/summary
   回傳: { assets, racks, pue, power, alerts, occupancy_rate }

2. 子節點列表 API:
   GET /api/locations/:level/:id/children
   回傳: [ { id, name, slug, summary: {...} } ]
```

使用預計算方式避免深度遞迴查詢。建議每 5 分鐘由後台排程更新各層級的聚合快取。

### 8.4 聚合需注意的陷阱

| 問題 | 解法 |
|------|------|
| PUE 不可直接平均 | 以功耗為權重做加權平均 |
| 百分比不可直接平均 | 以基數（機櫃數/U 數）為權重 |
| 告警可能重複計算 | 告警應繫結至最低層級（IDC），向上僅 COUNT |
| 時間序列聚合粒度 | 上層用日/週粒度，下層用分鐘/小時粒度 |

---

## 9. 對既有頁面的影響

### 9.1 影響矩陣

| 頁面 | 影響程度 | 改動內容 |
|------|---------|---------|
| **App.tsx** | 中 | 新增 4 條 location 路由，引入 LocationProvider |
| **MainLayout.tsx** | 中 | sidebar 新增「位置總覽」頂層導航項；topTabs 新增入口 |
| **Dashboard.tsx** | 中 | 麵包屑動態化；KPI 讀取 LocationContext；移除硬編碼 |
| **DataCenter3D.tsx** | 小 | locationTree 改讀真實資料；麵包屑動態化 |
| **FacilityMap.tsx** | 小 | 麵包屑動態化；讀取 LocationContext |
| **RackManagement.tsx** | 小 | 讀取 LocationContext 篩選資料 |
| **RackVisualization.tsx** | 小 | 讀取 LocationContext |
| **RackDetail.tsx** | 小 | 讀取 LocationContext |
| **其餘 44 頁** | 極小 | 僅需確保可從 LocationContext 讀取當前位置（漸進式） |
| **i18n JSON x 3** | 中 | 新增位置階層相關翻譯鍵 |

### 9.2 MainLayout 改動明細

**Sidebar 導航**: 在最頂部（Dashboard 之前）新增「位置總覽」項目。

```
現狀 navSections:
  [Dashboard] [Assets] [Locations/Racks] [Inventory] ...

改造後:
  [位置總覽]  ← 新增，連結至 /locations
  [Dashboard] ← 保持不變（改為「IDC 儀表板」）
  [Assets] [Locations/Racks] [Inventory] ...
```

**TopTabs**: 新增 "LOCATIONS" tab 在最左側。

### 9.3 Dashboard.tsx 改動明細

```
現狀 (硬編碼):
  {["CHINA", "EAST", "SHANGHAI", "PUDONG", "IDC-01"].map(...)}

改造後:
  <LocationBreadcrumb />    ← 共用組件，從 LocationContext 讀取路徑
```

```
現狀 (硬編碼 KPI):
  <p>12,842</p>  ← 直接寫死

改造後:
  const { currentIdc } = useLocationContext()
  <p>{currentIdc.summary.totalAssets}</p>  ← 從 context 讀取
```

### 9.4 新增共用元件

| 元件 | 用途 |
|------|------|
| `LocationContext` / `LocationProvider` | 全域位置狀態管理 |
| `LocationBreadcrumb` | 動態位置麵包屑 |
| `LocationCard` | 各層級子節點卡片（國家卡片、區域卡片、城市卡片、IDC 卡片共用） |
| `KpiSummaryBar` | 頂部 KPI 統計列（各層級共用） |
| `MiniSparkline` | 迷你趨勢圖（用於卡片內） |

---

## 10. i18n 策略

### 10.1 新增翻譯鍵結構

```json
{
  "locations": {
    "global_overview": "全球指揮中心",
    "region_overview": "區域總覽",
    "city_overview": "城市總覽",
    "campus_overview": "園區總覽",
    "total_idcs": "IDC 總數",
    "total_campuses": "園區總數",
    "total_regions": "區域總數",
    "total_countries": "國家總數",
    "total_racks": "機櫃總數",
    "total_assets": "資產總數",
    "avg_pue": "平均 PUE",
    "total_power": "總功耗",
    "active_alerts": "啟用告警",
    "rack_occupancy": "機櫃使用率",
    "drill_down": "鑽取查看",
    "back_to_parent": "返回上層",
    "view_map": "地圖檢視",
    "view_cards": "卡片檢視",
    "view_list": "列表檢視",
    "compare": "比較",
    "capacity_planning": "容量規劃",
    "sort_by": "排序方式"
  }
}
```

### 10.2 地理名稱 i18n

地理名稱（國家、區域、城市、園區）需要獨立的翻譯表，建議結構：

```json
{
  "geo": {
    "country": {
      "china": { "en": "China", "zh-CN": "中国", "zh-TW": "中國" },
      "japan": { "en": "Japan", "zh-CN": "日本", "zh-TW": "日本" }
    },
    "region": {
      "east": { "en": "East", "zh-CN": "华东", "zh-TW": "華東" },
      "south": { "en": "South", "zh-CN": "华南", "zh-TW": "華南" }
    }
  }
}
```

或者，地理名稱的多語言文字直接由後端 API 回傳（較佳做法，因地理實體為動態資料）。

---

## 11. 架構決策紀錄

### ADR-001: 位置上下文傳遞方式

**Status**: Proposed

**Context**:
系統需要在 52 個既有頁面中傳遞「當前所選位置」的上下文。有三種方案可選：
(A) 將位置參數嵌入所有路由 URL 中
(B) 使用 React Context + URL query 參數
(C) 使用全域狀態管理（如 Zustand）

**Decision**:
採用方案 B -- React Context (`LocationProvider`) 搭配 URL 持久化（`?idc=idc-01` 或 localStorage fallback）。

位置階層頁面 (`/locations/...`) 使用路由參數；
既有功能頁面 (`/dashboard`, `/racks`, ...) 從 LocationContext 讀取。

**Consequences**:
- 正面: 既有 52 個頁面路由完全不變，降低重構風險
- 正面: 位置切換只需更新 context，所有頁面自動響應
- 負面: 功能頁面 URL 不包含完整位置路徑（無法僅從 URL 恢復位置）
- 緩解: 透過 localStorage 持久化最近一次選取的位置，刷新頁面時自動恢復
- 未來: 第二階段可將位置參數遷入路由 URL（方案 A）

---

### ADR-002: 新增頁面的層級粒度

**Status**: Proposed

**Context**:
位置階層共 8 層（Country → Region → City → Campus → IDC → Module → Rack → U-position）。
需決定哪些層級需要獨立的新頁面，哪些可複用既有頁面。

**Decision**:
新增 4 個層級頁面（Country, Region, City, Campus），其餘 4 個層級（IDC, Module, Rack, U-position）複用既有頁面。

```
新增:  GlobalOverview, RegionOverview, CityOverview, CampusOverview
複用:  Dashboard(IDC), DataCenter3D/FacilityMap(Module),
       RackManagement/RackDetail(Rack), RackVisualization(U-pos)
```

**Consequences**:
- 正面: 新增頁面數量最少（4 個），開發量可控
- 正面: IDC 以下的既有頁面已具備完整功能，無需重做
- 負面: 「進入 IDC」時從 `/locations/...` 跳轉至 `/dashboard` 有 URL 斷裂感
- 緩解: 透過 LocationBreadcrumb 維持位置感知的連續性

---

### ADR-003: 地圖視覺化技術選型

**Status**: Proposed

**Context**:
GlobalOverview 和 RegionOverview 需要地圖元件。可選方案：
(A) 互動式地圖庫（如 react-simple-maps + D3）
(B) 靜態 SVG 地圖（手繪/設計師出圖）
(C) 第三方地圖服務（如 Mapbox/高德地圖）

**Decision**:
採用方案 A -- `react-simple-maps` 搭配 TopoJSON。

**Consequences**:
- 正面: 無外部服務依賴，完全離線可用（符合企業內網場景）
- 正面: 可深度客製化樣式，與 Industrial Brutalism 主題一致
- 正面: 輕量，bundle 體積可控
- 負面: 不支援街景/衛星圖（但此場景不需要）
- 負面: 需自行處理 TopoJSON 資料與投影

---

### ADR-004: 聚合資料的快取策略

**Status**: Proposed

**Context**:
各層級的 KPI 需要從底層（IDC/Rack）逐層向上聚合。即時遞迴查詢會導致上層頁面載入緩慢。

**Decision**:
由後端每 5 分鐘預計算各層級的聚合摘要，存入 `location_summary` 快取表。前端直接查詢該表。

**Consequences**:
- 正面: 上層頁面載入速度恆定（不受下層資料量影響）
- 正面: 前端邏輯簡單，無需在瀏覽器端做複雜聚合
- 負面: 資料有最多 5 分鐘的延遲
- 緩解: 在 UI 上顯示「最後更新時間」標記；告警類即時推送不走快取

---

## 12. 實作分期建議

### Phase 1: 基礎建設（2 週）

- 建立 `LocationContext` / `LocationProvider`
- 建立 `LocationBreadcrumb` 共用組件
- 改造 Dashboard 麵包屑為動態
- 新增 `/locations` 路由入口
- 建立 GlobalOverview 頁面（使用 mock 資料）

### Phase 2: 鑽取鏈路（3 週）

- 建立 RegionOverview、CityOverview、CampusOverview
- 串接完整鑽取流程: Global → Region → City → Campus → IDC
- MainLayout sidebar 新增「位置總覽」入口
- 各頁面整合 LocationContext
- i18n 翻譯鍵新增

### Phase 3: 既有頁面整合（2 週）

- Dashboard KPI 改為根據 LocationContext 載入
- DataCenter3D 的 locationTree 改為真實資料
- FacilityMap、RackManagement 等讀取 LocationContext
- 完整測試所有鑽取/返回路徑

### Phase 4: 地圖與視覺化強化（2 週）

- GlobalOverview 接入 react-simple-maps 世界地圖
- RegionOverview 接入國家地圖
- 各層級 KPI 比較圖表（recharts 既已引入）
- 迷你趨勢圖組件 (sparkline)

### Phase 5: 後端整合（待後端就緒）

- 串接真實 location API
- 串接聚合 summary API
- 移除 mock 資料
- 效能測試與調優

---

## 附錄 A: 完整導航結構圖

```
/locations (GlobalOverview)
├── /locations/china (RegionOverview)
│   ├── /locations/china/east (CityOverview)
│   │   ├── /locations/china/east/shanghai (CampusOverview)
│   │   │   ├── [IDC-01] → /dashboard (context: IDC-01)
│   │   │   │   ├── /racks/3d          (DataCenter3D)
│   │   │   │   ├── /racks/facility-map (FacilityMap)
│   │   │   │   ├── /racks             (RackManagement)
│   │   │   │   │   ├── /racks/detail  (RackDetail)
│   │   │   │   │   ├── /racks/visualization (RackVisualization)
│   │   │   │   │   ├── /racks/2d      (RackFrontView2D)
│   │   │   │   │   └── /racks/console (RackConsole)
│   │   │   │   ├── /assets            (AssetManagement)
│   │   │   │   ├── /monitoring        (MonitoringAlerts)
│   │   │   │   ├── /maintenance       (MaintenanceSchedule)
│   │   │   │   ├── /predictive        (PredictiveMaintenance)
│   │   │   │   └── ...其餘功能頁面
│   │   │   ├── [IDC-02] → /dashboard (context: IDC-02)
│   │   │   └── [IDC-03] → /dashboard (context: IDC-03)
│   │   ├── /locations/china/east/nanjing (CampusOverview)
│   │   └── /locations/china/east/hangzhou (CampusOverview)
│   ├── /locations/china/south (CityOverview)
│   ├── /locations/china/north (CityOverview)
│   └── /locations/china/southwest (CityOverview)
├── /locations/japan (RegionOverview)
└── /locations/singapore (RegionOverview)
```

## 附錄 B: LocationContext 介面定義（概念）

```
LocationPath:
  country?:  { id, slug, name }
  region?:   { id, slug, name }
  city?:     { id, slug, name }
  campus?:   { id, slug, name }
  idc?:      { id, slug, name }
  module?:   { id, slug, name }
  rack?:     { id, slug, name }

LocationContextValue:
  path: LocationPath
  setPath: (path: LocationPath) => void
  navigateToLevel: (level: string, slug: string) => void
  breadcrumbs: Array<{ label: string, to: string }>
  currentLevel: 'global' | 'country' | 'region' | 'city' | 'campus' | 'idc' | 'module' | 'rack'
```

## 附錄 C: 各層級 KPI 匯總表

```
                    Global  Country  Region  City  Campus  IDC
                    ──────  ───────  ──────  ────  ──────  ───
子節點數量           ●       ●        ●       ●     ●       ●
機櫃總數             ●       ●        ●       ●     ●       ●
資產總數             ●       ●        ●       ●     ●       ●
機櫃使用率(加權)      ●       ●        ●       ●     ●       ●
PUE(加權)           ●       ●        ●       ●     ●       ●
總功耗              ●       ●        ●       ●     ●       ●
嚴重告警數           ●       ●        ●       ●     ●       ●
能耗趨勢圖           ●       ●        ●       ●     ●       ●
子節點比較圖          ●       ●        ●       ●     ●       -
地圖                ●       ●        -       -     -       -
容量規劃             -       -        -       ●     ●       ●
溫度 heatmap        -       -        -       -     -       ●
3D 視圖             -       -        -       -     -       ●

● = 顯示   - = 不顯示
```

---

*文件結束*
