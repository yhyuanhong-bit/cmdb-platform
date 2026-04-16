# P1-5 + P1-15 詳細實施計劃

---

## P1-5: SensorConfiguration 改用 useAlertRules

### 目標

將 SensorConfiguration 頁面從 hardcoded `INITIAL_RULES` 改為使用後端 `alert_rules` 表的真實數據，實現：
- 顯示真實告警規則
- 閾值修改持久化到 DB
- 與 MonitoringAlerts 頁面數據一致

### 數據結構映射

**後端 alert_rules 表：**
```
id: UUID
name: string            → 直接映射到 rule.name
metric_name: string     → 映射到 threshold 的 metric 欄位
condition: JSONB        → { "op": ">", "threshold": 85 } → 映射到 warning/critical 值
severity: string        → "warning" | "critical" → 決定是 warning 還是 critical 閾值
enabled: boolean        → 直接映射到 rule.enabled
```

**前端現有結構（需廢棄）：**
```typescript
// INITIAL_RULES — 完全不同的結構
{ id: "RULE-001", name: "...", condition: "Temperature > Critical for 5 min", action: "...", enabled: true }

// THRESHOLDS — 按 metric 組織
{ metric: "Temperature", warning: 38, critical: 42, min: 20, max: 60, step: 1 }
```

**映射方案：**

seed 中每個 metric 有兩條規則（warning + critical），需要合併為一個 ThresholdConfig：

```
alert_rules 表:
  CPU High    (metric_name: cpu_usage,    severity: warning,  threshold: 85)
  CPU Critical(metric_name: cpu_usage,    severity: critical, threshold: 95)
  Temp High   (metric_name: temperature,  severity: warning,  threshold: 40)
  Disk Full   (metric_name: disk_usage,   severity: critical, threshold: 90)
  Memory High (metric_name: memory_usage, severity: warning,  threshold: 90)

→ 合併為 ThresholdConfig[]:
  { metric: "cpu_usage",    warning: 85, critical: 95,  ruleIds: { warning: "4000...01", critical: "4000...02" } }
  { metric: "temperature",  warning: 40, critical: null, ruleIds: { warning: "4000...03" } }
  { metric: "disk_usage",   warning: null, critical: 90, ruleIds: { critical: "4000...04" } }
  { metric: "memory_usage", warning: 90, critical: null, ruleIds: { warning: "4000...05" } }
```

### 改動清單

#### 1. 加 useCreateAlertRule mutation hook（如不存在）

**檔案**: `src/hooks/useMonitoring.ts`

```typescript
export function useCreateAlertRule() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: any) => monitoringApi.createAlertRule(data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['alertRules'] }) }
  })
}
```

#### 2. 重構 SensorConfiguration.tsx 數據層

**檔案**: `src/pages/SensorConfiguration.tsx`

**Step 2a: 改 import + 加 hook**
```typescript
// 移除 INITIAL_RULES 常量
// 加 import
import { useAlertRules } from '../hooks/useMonitoring'

// 在 component 內
const { data: rulesResp, isLoading: rulesLoading } = useAlertRules()
const apiRules = rulesResp?.data || []
```

**Step 2b: 從 apiRules 建構 rules 和 thresholds state**

替換 `useState(INITIAL_RULES)` 和 `useState(THRESHOLDS)`：

```typescript
// 從 API 規則合併為閾值配置
useEffect(() => {
  if (apiRules.length === 0) return

  // 按 metric_name 分組
  const grouped: Record<string, { warning?: number; critical?: number; warningId?: string; criticalId?: string; enabled: boolean }> = {}
  
  apiRules.forEach(rule => {
    if (!grouped[rule.metric_name]) {
      grouped[rule.metric_name] = { enabled: true }
    }
    const threshold = (rule.condition as any)?.threshold ?? 0
    if (rule.severity === 'warning') {
      grouped[rule.metric_name].warning = threshold
      grouped[rule.metric_name].warningId = rule.id
    } else if (rule.severity === 'critical') {
      grouped[rule.metric_name].critical = threshold
      grouped[rule.metric_name].criticalId = rule.id
    }
    if (!rule.enabled) grouped[rule.metric_name].enabled = false
  })

  // metric 元數據（顯示名、圖標、單位、範圍）
  const META: Record<string, { icon: string; unit: string; min: number; max: number; step: number }> = {
    cpu_usage:    { icon: 'memory', unit: '%', min: 0, max: 100, step: 5 },
    temperature:  { icon: 'thermostat', unit: '°C', min: 10, max: 60, step: 1 },
    disk_usage:   { icon: 'storage', unit: '%', min: 0, max: 100, step: 5 },
    memory_usage: { icon: 'memory', unit: '%', min: 0, max: 100, step: 5 },
    power_kw:     { icon: 'bolt', unit: 'kW', min: 0, max: 10, step: 0.5 },
  }

  const newThresholds = Object.entries(grouped).map(([metric, data]) => ({
    metric,
    icon: META[metric]?.icon || 'sensors',
    unit: META[metric]?.unit || '',
    warning: data.warning ?? 0,
    critical: data.critical ?? 0,
    min: META[metric]?.min ?? 0,
    max: META[metric]?.max ?? 100,
    step: META[metric]?.step ?? 1,
    warningId: data.warningId,
    criticalId: data.criticalId,
  }))

  setThresholds(newThresholds)

  // 規則列表也從 API 渲染
  setRules(apiRules.map(r => ({
    id: r.id,
    name: r.name,
    condition: `${r.metric_name} ${(r.condition as any)?.op || '>'} ${(r.condition as any)?.threshold || 0}`,
    action: r.severity === 'critical' ? 'Page on-call + Escalate' : 'Notify Team',
    enabled: r.enabled,
  })))
}, [apiRules])
```

**Step 2c: Save Configuration 按鈕邏輯**

Save 需要做的事：對每個修改過的 threshold，更新對應的 alert_rule。

但目前後端沒有 `PUT /monitoring/rules/{id}` 端點。兩個選擇：

- **方案 A（簡單）**: 不加新端點。Save 時刪舊規則 + 建新規則（需要後端 DELETE /monitoring/rules/{id}）
- **方案 B（正規）**: 加 `PUT /monitoring/rules/{id}` 端點

**推薦方案 B**，需要：
1. 後端加 `UpdateAlertRule` sqlc query + endpoint
2. 前端加 `useUpdateAlertRule` hook
3. Save 按鈕遍歷 thresholds，對每個有變更的規則呼叫 update

```typescript
// Save onClick 邏輯偽代碼
const handleSave = async () => {
  for (const t of thresholds) {
    if (t.warningId) {
      await updateAlertRule.mutateAsync({
        id: t.warningId,
        data: { condition: { op: '>', threshold: t.warning }, enabled: rules.find(r => r.id === t.warningId)?.enabled ?? true }
      })
    }
    if (t.criticalId) {
      await updateAlertRule.mutateAsync({
        id: t.criticalId,
        data: { condition: { op: '>', threshold: t.critical }, enabled: rules.find(r => r.id === t.criticalId)?.enabled ?? true }
      })
    }
  }
}
```

### 需要新增的後端端點

| 端點 | 用途 |
|------|------|
| `PUT /monitoring/rules/{id}` | 更新單條告警規則（condition, severity, enabled）|

sqlc query:
```sql
-- name: UpdateAlertRule :one
UPDATE alert_rules SET
    name        = COALESCE(sqlc.narg('name'), name),
    metric_name = COALESCE(sqlc.narg('metric_name'), metric_name),
    condition   = COALESCE(sqlc.narg('condition'), condition),
    severity    = COALESCE(sqlc.narg('severity'), severity),
    enabled     = COALESCE(sqlc.narg('enabled'), enabled)
WHERE id = sqlc.arg('id')
RETURNING *;
```

### 改動檔案清單

| 檔案 | 改動 | 行數估計 |
|------|------|---------|
| `cmdb-core/db/queries/alert_rules.sql` | 加 UpdateAlertRule query | +8 |
| `cmdb-core/internal/domain/monitoring/service.go` | 加 UpdateRule method | +8 |
| `cmdb-core/internal/api/impl.go` | 加 UpdateAlertRule handler | +30 |
| `api/openapi.yaml` | 加 PUT /monitoring/rules/{id} | +25 |
| `cmdb-core/internal/api/generated.go` | 重新生成 | 自動 |
| `cmdb-core/internal/dbgen/alert_rules.sql.go` | 重新生成 | 自動 |
| `cmdb-demo/src/hooks/useMonitoring.ts` | 加 useCreateAlertRule + useUpdateAlertRule | +20 |
| `cmdb-demo/src/pages/SensorConfiguration.tsx` | 重構數據層 + Save 邏輯 | ~80 行改動 |

**總改動量：~170 行**

---

## P1-15: DataCenter3D 位置樹改用 API

### 目標

將 DataCenter3D 左側位置樹從 hardcoded Germany/Frankfurt 改為使用真實的 Taiwan 位置層級，並且：
- 點擊樹節點切換 locationId → useRacks 自動載入對應機架
- 樹結構與 GlobalOverview → CampusOverview 一致

### 數據結構映射

**後端 Location 型別（API 返回）：**
```typescript
interface Location {
  id: string       // UUID
  name: string     // "台灣"
  name_en: string  // "Taiwan"
  slug: string     // "tw"
  level: string    // "country" | "region" | "city" | "campus"
  parent_id: string | null
  path: string     // "tw.north.taipei.neihu"
  status: string
  metadata: Record<string, any>
  sort_order: number
}
```

**前端現有 TreeNode 結構：**
```typescript
interface TreeNode {
  id: string        // "de"
  label: string     // "Germany (DE)"
  children?: TreeNode[]
  active?: boolean
}
```

**映射方案：**
```typescript
// Location → TreeNode 轉換
function locationToTreeNode(loc: Location, children: TreeNode[]): TreeNode {
  return {
    id: loc.id,                            // 用真實 UUID
    label: `${loc.name_en} (${loc.name})`, // "Taiwan (台灣)"
    children: children.length > 0 ? children : undefined,
    active: false,
  }
}
```

### 載入策略

位置層級最多 4 層（country → region → city → campus）。需要遞迴載入。

**方案 A（逐層懶載入）：** 點擊展開時才載入子節點 — 需要改 TreeItem 組件加 loading 狀態
**方案 B（一次性載入全部）：** 用 `GET /locations/{id}/descendants` 一次取所有後代 — API 已存在

**推薦方案 B** — 一次 API 呼叫取所有位置，前端組裝樹結構：

```typescript
// 1. 取根位置
const { data: rootResp } = useRootLocations()
const country = rootResp?.data?.[0] // Taiwan

// 2. 取所有後代
const { data: descResp } = useLocationDescendants(country?.id || '')
const allLocations = descResp?.data || []

// 3. 組裝樹
function buildTree(locations: Location[], parentId: string | null): TreeNode[] {
  return locations
    .filter(loc => loc.parent_id === parentId)
    .sort((a, b) => a.sort_order - b.sort_order)
    .map(loc => ({
      id: loc.id,
      label: `${loc.name_en || loc.name}`,
      children: buildTree(locations, loc.id),
      active: loc.id === selectedLocationId,
    }))
    .map(node => ({
      ...node,
      children: node.children && node.children.length > 0 ? node.children : undefined,
    }))
}

// 4. 建樹（把 country 作為根）
const tree: TreeNode[] = country
  ? [{ id: country.id, label: country.name_en || country.name,
       children: buildTree([...allLocations], country.id) }]
  : []
```

### 節點點擊 → 切換 rack 視圖

現在 `toggleTreeNode` 只做展開/收合。需要增加：點擊葉節點（campus 層級）時更新 `locationId` → `useRacks` 自動重新取數據。

```typescript
const [selectedLocationId, setSelectedLocationId] = useState(locationId)

const handleTreeNodeClick = (nodeId: string) => {
  // 找到該位置
  const loc = allLocations.find(l => l.id === nodeId) || (country?.id === nodeId ? country : null)
  if (!loc) return

  // 展開/收合
  setTreeExpanded(prev => ({ ...prev, [nodeId]: prev[nodeId] === false }))

  // 如果是 campus 層級（有 rack 的層級），切換 rack 視圖
  if (loc.level === 'campus') {
    setSelectedLocationId(nodeId)
  }
}

// useRacks 改用 selectedLocationId
const { data: racksResponse, isLoading } = useRacks(selectedLocationId)
```

### 改動清單

**不需要任何後端改動** — 所有 API 端點已存在。

| 檔案 | 改動 | 行數估計 |
|------|------|---------|
| `cmdb-demo/src/pages/DataCenter3D.tsx` | 替換 hardcoded locationTree + 加 API hooks + 改 toggleTreeNode | ~60 行改動 |

### 具體改動步驟

#### Step 1: 加 imports
```typescript
import { useRootLocations, useLocationDescendants } from '../hooks/useTopology'
```

#### Step 2: 替換 hardcoded locationTree

刪除 lines 22-54 的 `const locationTree: TreeNode[] = [...]`

加入動態建樹邏輯（見上方 buildTree 函數）

#### Step 3: 改 state

```typescript
// 原本：hardcoded expanded
const [treeExpanded, setTreeExpanded] = useState<Record<string, boolean>>({
  de: true, "de-south": true, frankfurt: true, "campus-alpha": true, "idc-alpha": true
})

// 改為：動態初始化（全部展開）
const [treeExpanded, setTreeExpanded] = useState<Record<string, boolean>>({})

useEffect(() => {
  if (allLocations.length > 0 && country) {
    const expanded: Record<string, boolean> = { [country.id]: true }
    allLocations.forEach(loc => { expanded[loc.id] = true })
    setTreeExpanded(expanded)
  }
}, [allLocations, country])
```

#### Step 4: 加 selectedLocationId state + 改 useRacks 參數

```typescript
const [selectedLocationId, setSelectedLocationId] = useState(locationId)

// 原本：const { data: racksResponse } = useRacks(locationId)
// 改為：
const { data: racksResponse, isLoading } = useRacks(selectedLocationId)
```

#### Step 5: 改 toggleTreeNode → handleTreeNodeClick

```typescript
const handleTreeNodeClick = (nodeId: string) => {
  setTreeExpanded(prev => ({ ...prev, [nodeId]: prev[nodeId] === false }))
  
  const loc = allLocations.find(l => l.id === nodeId)
  if (loc && loc.level === 'campus') {
    setSelectedLocationId(nodeId)
  }
}
```

#### Step 6: TreeItem 渲染改用動態 tree

```tsx
// 原本：{locationTree.map(node => <TreeItem ... />)}
// 改為：
{tree.map(node => (
  <TreeItem key={node.id} node={node} depth={0} expanded={treeExpanded} onToggle={handleTreeNodeClick} />
))}
```

### 預期效果

修復前的樹：
```
▼ Germany (DE)
  ▼ DE-South Region
    ▼ Frankfurt
      ▼ Campus Alpha
        ▼ IDC Alpha
          ● Module 1 [ACTIVE]
```

修復後的樹：
```
▼ Taiwan (台灣)
  ▼ North (北部)
    ▼ Taipei (台北)
      ● Neihu Campus (内湖園區)     ← 點擊載入 6 個機架
    ▼ Hsinchu (新竹)
      ● HSIP Campus (竹科園區)       ← 點擊載入 2 個機架
  ▼ South (南部)
    ▼ Kaohsiung (高雄)
      ● Qianzhen Campus (前鎮園區)  ← 點擊載入 2 個機架
```

---

## 執行順序

```
1. P1-5 後端：加 PUT /monitoring/rules/{id} 端點     (~40 行)
2. P1-5 前端：重構 SensorConfiguration 數據層          (~80 行)
3. P1-15 前端：重構 DataCenter3D 位置樹                (~60 行)
```

Step 1 和 3 互相獨立。Step 2 依賴 Step 1。

**總改動量：~230 行**
**改動檔案：8 個**
**新增後端端點：1 個（PUT /monitoring/rules/{id}）**
