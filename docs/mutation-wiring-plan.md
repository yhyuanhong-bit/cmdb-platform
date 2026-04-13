# Mutation Hook 接線執行計劃

> 目標：將 18 個已存在的 mutation hook 接到對應的 UI 按鈕，提升寫入操作覆蓋率從 25% → 90%

---

## A 類：接線已有按鈕（6 項）

每項改動 < 20 行，只需 import hook + 加 onClick。

### A1. UserProfile — "Update Profile" 按鈕

**檔案**: `src/pages/UserProfile.tsx`
**行號**: line 105（按鈕無 onClick）
**現有 state**: `displayName`, `email`(未定義需加), `phone`(未定義需加) — line 16-21

**改動**:
```tsx
// 1. 加 import
import { useUpdateUser } from '../hooks/useIdentity'

// 2. 在 component 內加 hook
const user = useAuthStore(s => s.user)
const updateUser = useUpdateUser()

// 3. 接按鈕
<button onClick={() => {
  if (user?.id) {
    updateUser.mutate({ id: user.id, data: { display_name: displayName } })
  }
}}>
  {updateUser.isPending ? 'Saving...' : t('user_profile.btn_update_profile')}
</button>
```

**依賴**: `useUpdateUser` hook (已存在) → `PUT /users/{id}` (已實現)

---

### A2. MaintenanceTaskView — "Complete" 按鈕

**檔案**: `src/pages/MaintenanceTaskView.tsx`
**行號**: line 139-142（按鈕無 onClick）
**現有 data**: `taskId` from useParams (line 62), `workOrder` from useWorkOrder (line 63)

**改動**:
```tsx
// 1. 加 import
import { useTransitionWorkOrder } from '../hooks/useMaintenance'

// 2. 在 component 內加 hook
const transitionWO = useTransitionWorkOrder()

// 3. 接 Complete 按鈕 (line 139)
<button onClick={() => {
  if (taskId) {
    transitionWO.mutate({
      id: taskId,
      data: { status: 'completed', comment: 'Task completed' }
    })
  }
}} disabled={transitionWO.isPending}>
  {transitionWO.isPending ? 'Completing...' : t('maintenance_task.complete_action')}
</button>
```

**依賴**: `useTransitionWorkOrder` hook (已存在) → `POST /maintenance/orders/{id}/transition` (已實現)

---

### A3. MaintenanceTaskView — 加載工單操作日誌

**檔案**: `src/pages/MaintenanceTaskView.tsx`
**現狀**: 時間線 (timeline) 數據 hardcoded（約 line 75-90）

**改動**:
```tsx
// 1. 加 import
import { useWorkOrderLogs } from '../hooks/useMaintenance'

// 2. 用真實日誌替換 hardcoded timeline
const { data: logsData } = useWorkOrderLogs(taskId)
const logs = logsData?.data || []

// 3. 在 timeline 渲染中，用 logs 替換靜態 TIMELINE_DATA
// 每條 log 有: action, from_status, to_status, operator_id, comment, created_at
```

**依賴**: `useWorkOrderLogs` hook (已存在) → `GET /maintenance/orders/{id}/logs` (已實現)

---

### A4. SystemSettings — "New User" 按鈕

**檔案**: `src/pages/SystemSettings.tsx`
**行號**: line 65-68（按鈕無 onClick）

**改動**: 需加一個簡單的 inline 表單 modal：
```tsx
// 1. 加 imports
import { useCreateUser } from '../hooks/useIdentity'

// 2. 加 state
const [showUserForm, setShowUserForm] = useState(false)
const [newUser, setNewUser] = useState({ username: '', display_name: '', email: '', password: '' })
const createUser = useCreateUser()

// 3. 接按鈕
<button onClick={() => setShowUserForm(true)}>
  {t('system_settings.btn_new_user')}
</button>

// 4. 加 modal (在 return JSX 末尾)
{showUserForm && (
  <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
    <div className="bg-[#1a1f2e] p-6 rounded-xl w-96 space-y-4">
      <h3 className="text-lg font-bold">Create User</h3>
      <input placeholder="Username" value={newUser.username}
        onChange={e => setNewUser(p => ({...p, username: e.target.value}))}
        className="w-full p-2 bg-[#0d1117] rounded border border-gray-700" />
      <input placeholder="Display Name" value={newUser.display_name}
        onChange={e => setNewUser(p => ({...p, display_name: e.target.value}))}
        className="w-full p-2 bg-[#0d1117] rounded border border-gray-700" />
      <input placeholder="Email" value={newUser.email}
        onChange={e => setNewUser(p => ({...p, email: e.target.value}))}
        className="w-full p-2 bg-[#0d1117] rounded border border-gray-700" />
      <input type="password" placeholder="Password" value={newUser.password}
        onChange={e => setNewUser(p => ({...p, password: e.target.value}))}
        className="w-full p-2 bg-[#0d1117] rounded border border-gray-700" />
      <div className="flex gap-2 justify-end">
        <button onClick={() => setShowUserForm(false)}
          className="px-4 py-2 rounded bg-gray-700">Cancel</button>
        <button onClick={() => {
          createUser.mutate(newUser, { onSuccess: () => setShowUserForm(false) })
        }} disabled={createUser.isPending}
          className="px-4 py-2 rounded bg-blue-600">
          {createUser.isPending ? 'Creating...' : 'Create'}
        </button>
      </div>
    </div>
  </div>
)}
```

**依賴**: `useCreateUser` hook (已存在) → `POST /users` (已實現)

---

### A5. RolesPermissions — "Add New Role" 按鈕

**檔案**: `src/pages/RolesPermissions.tsx`
**行號**: line 212-219（按鈕無 onClick）

**改動**: 類似 A4 的 modal 模式：
```tsx
// 1. 加 imports
import { useCreateRole, useDeleteRole } from '../hooks/useIdentity'

// 2. 加 state
const [showRoleForm, setShowRoleForm] = useState(false)
const [newRole, setNewRole] = useState({ name: '', description: '' })
const createRole = useCreateRole()
const deleteRole = useDeleteRole()

// 3. 接 "Add New Role" 按鈕 (line 212)
<button onClick={() => setShowRoleForm(true)}>
  {t('roles.add_new_role')}
</button>

// 4. Modal with name + description fields → createRole.mutate(newRole)

// 5. 每個非系統角色卡片加 Delete 按鈕
{!role.is_system && (
  <button onClick={() => {
    if (confirm('Delete this role?')) deleteRole.mutate(role.id)
  }} className="text-red-400 text-xs">Delete</button>
)}
```

**依賴**: `useCreateRole` + `useDeleteRole` hooks → `POST /roles` + `DELETE /roles/{id}`

---

### A6. RolesPermissions — "Save Changes" 按鈕

**檔案**: `src/pages/RolesPermissions.tsx`
**行號**: line 295-300（按鈕無 onClick）
**現有 state**: `permOverrides` (line 149) 存本地權限切換

**改動**:
```tsx
// Save Changes 目前沒有直接對應的 "updateRole" mutation
// 但可以用現有的 permissions 結構：
// permOverrides 是 { [scope]: { view: bool, edit: bool, delete: bool, export: bool } }
// 需要轉換成後端格式 { [resource]: ["read", "write", "delete"] }

// 暫時標記為 placeholder — 需要後端 PUT /roles/{id} 端點
// 該端點目前不存在，但可以用 createRole 重建
<button onClick={() => {
  alert('Permission changes saved locally. Backend sync not yet available.')
}}>
  {t('roles.save_changes')}
</button>
```

**注意**: 後端沒有 `PUT /roles/{id}` 端點。此項需要先加後端端點，或改用 workaround。標記為 **BLOCKED**。

---

## B 類：加編輯/刪除模式到現有詳情頁（6 項）

每項改動 30-60 行，需要加 editing state + form fields + save/cancel。

### B1. AssetDetailUnified — Edit Asset

**檔案**: `src/pages/AssetDetailUnified.tsx`
**行號**: line 866-869（"Edit Asset" 按鈕無 onClick）
**現有 data**: `apiAsset` from `useAsset(assetId)` (line 781)

**改動**:
```tsx
// 1. 加 imports
import { useUpdateAsset } from '../hooks/useAssets'

// 2. 加 state
const [editing, setEditing] = useState(false)
const [editData, setEditData] = useState<Record<string, any>>({})
const updateAsset = useUpdateAsset()

// 3. 接 "Edit Asset" 按鈕 (line 866)
<button onClick={() => {
  setEditing(true)
  setEditData({
    name: apiAsset?.name || '',
    status: apiAsset?.status || '',
    vendor: apiAsset?.vendor || '',
    model: apiAsset?.model || '',
    bia_level: apiAsset?.bia_level || '',
  })
}}>Edit Asset</button>

// 4. editing 模式下，把顯示欄位換成 input
// 5. 加 Save/Cancel 按鈕
{editing && (
  <div className="flex gap-2">
    <button onClick={() => {
      updateAsset.mutate({ id: assetId!, data: editData }, {
        onSuccess: () => setEditing(false)
      })
    }} disabled={updateAsset.isPending}>
      {updateAsset.isPending ? 'Saving...' : 'Save'}
    </button>
    <button onClick={() => setEditing(false)}>Cancel</button>
  </div>
)}
```

---

### B2. AssetDetailUnified — Delete Asset

**同一檔案**: `src/pages/AssetDetailUnified.tsx`

**改動**: 在 header 加 Delete 按鈕
```tsx
// 1. 加 import (同 B1 一起)
// useAssets.ts 沒有 useDeleteAsset — 需要先加到 hook 檔案

// 2. 加到 useAssets.ts:
export function useDeleteAsset() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => assetApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['assets'] })
  })
}

// 3. 頁面中使用
const deleteAsset = useDeleteAsset()
const navigate = useNavigate()

<button onClick={() => {
  if (confirm('確定刪除此資產？')) {
    deleteAsset.mutate(assetId!, { onSuccess: () => navigate('/assets') })
  }
}} className="text-red-400">Delete</button>
```

**注意**: 需先在 `useAssets.ts` 加 `useDeleteAsset` hook。

---

### B3. RackDetailUnified — Edit Rack

**檔案**: `src/pages/RackDetailUnified.tsx`
**現狀**: 沒有 edit/delete 按鈕，需要在 header 區域添加

**改動**:
```tsx
// 1. 加 imports
import { useUpdateRack, useDeleteRack } from '../hooks/useTopology'

// 2. 加 state
const [editingRack, setEditingRack] = useState(false)
const [rackEdit, setRackEdit] = useState({ name: '', status: '' })
const updateRack = useUpdateRack()
const deleteRack = useDeleteRack()

// 3. 在 rack header 區域加按鈕
<button onClick={() => {
  setEditingRack(true)
  setRackEdit({ name: rack?.name || '', status: rack?.status || '' })
}}>Edit Rack</button>

<button onClick={() => {
  if (confirm('確定刪除此機架？')) {
    deleteRack.mutate(rackId!, { onSuccess: () => navigate('/racks') })
  }
}} className="text-red-400">Delete</button>

// 4. editing 模式 → 顯示 input fields + Save/Cancel
```

---

### B4. PredictiveHub — Run RCA + Verify

**檔案**: `src/pages/PredictiveHub.tsx`
**行號**: line 560（"Run Analysis" 按鈕）, line 1206（"Verify" 按鈕附近）

**改動**:
```tsx
// 1. 加 imports
import { useCreateRCA, useVerifyRCA } from '../hooks/usePrediction'

// 2. 加 hooks
const createRCA = useCreateRCA()
const verifyRCA = useVerifyRCA()

// 3. 接 "Run Analysis" 按鈕 (line 560)
// 需要一個 incident 選擇器 — 用最近的 incidents
<button onClick={() => {
  // 使用第一個 open incident 作為 demo
  createRCA.mutate({
    incident_id: '50000000-0000-0000-0000-000000000001', // 或從 state 選擇
    model_name: 'Dify RCA Analyzer',
  })
}} disabled={createRCA.isPending}>
  {createRCA.isPending ? 'Analyzing...' : 'Run Analysis'}
</button>

// 4. RCA 結果卡片上加 Verify 按鈕
// 需要遍歷 predictions/rca 列表，在 human_verified=false 的項目加按鈕
<button onClick={() => {
  verifyRCA.mutate({ id: rca.id, data: { verified_by: userId } })
}}>Verify</button>
```

**注意**: RCA 建立需要 `incident_id`。理想情況需要一個 incident 選擇 dropdown。簡化版可用最近的 open incident。

---

### B5. InventoryItemDetail — Verify/Flag/Resolve

**檔案**: `src/pages/InventoryItemDetail.tsx`
**行號**: line 187-198（Verify/Flag/Print 按鈕）, line 477-485（Resolved/Escalate）

**改動**:
```tsx
// 1. 加 imports
import { useScanItem } from '../hooks/useInventory'

// 2. 加 hook
const scanItem = useScanItem()
const taskId = searchParams.get('taskId') || ''

// 3. "Verify Asset" 按鈕 (line 187) — 掃描確認
<button onClick={() => {
  const item = items?.[0] // 當前項目
  if (item) {
    scanItem.mutate({
      taskId,
      itemId: item.id,
      data: { actual: item.expected, status: 'scanned' }
    })
  }
}}>Verify Asset</button>

// 4. "Flag Issue" 按鈕 (line 191) — 標記差異
<button onClick={() => {
  const item = items?.[0]
  if (item) {
    scanItem.mutate({
      taskId,
      itemId: item.id,
      data: { actual: item.actual || {}, status: 'discrepancy' }
    })
  }
}}>Flag Issue</button>

// 5. "Mark Resolved" 按鈕 (line 477) — 解決差異
<button onClick={() => {
  const item = items?.[0]
  if (item) {
    scanItem.mutate({
      taskId,
      itemId: item.id,
      data: { actual: item.expected, status: 'scanned' } // 確認為正確
    })
  }
}}>Mark Resolved</button>
```

---

### B6. HighSpeedInventory — Complete Task

**檔案**: `src/pages/HighSpeedInventory.tsx`
**行號**: 無明確 complete 按鈕，但可在 task header 加

**改動**:
```tsx
// 1. 加 imports
import { useCompleteTask } from '../hooks/useInventory'

// 2. 加 hook
const completeTask = useCompleteTask()

// 3. 在 currentTask header 區域加 Complete 按鈕
{currentTask && currentTask.status === 'in_progress' && (
  <button onClick={() => {
    if (confirm('Mark this inventory task as completed?')) {
      completeTask.mutate(currentTask.id)
    }
  }} disabled={completeTask.isPending}>
    {completeTask.isPending ? 'Completing...' : 'Complete Task'}
  </button>
)}
```

---

## C 類：新建 Modal 組件（6 項）

每項需要一個獨立的 modal 組件 + 接到頁面。

### C1. AssetManagement — Create Asset Modal

**檔案**: 新建 `src/components/CreateAssetModal.tsx`
**掛載**: `src/pages/AssetManagementUnified.tsx` line 254（"Add Asset" 按鈕已有 navigate('/assets/new')）

**方案選擇**:
- **方案 A**: 改為 inline modal（不需要新 route）
- **方案 B**: 建立 `/assets/new` 頁面（需要新 route）
- **推薦方案 A** — 改 navigate 為開 modal

**Modal 欄位**: asset_tag, name, type, sub_type, status, bia_level, vendor, model, serial_number
**Hook**: `useCreateAsset()`
**改動量**: ~80 行新組件 + 10 行頁面改動

---

### C2. HighSpeedInventory — Create Task Modal

**檔案**: 新建 `src/components/CreateInventoryTaskModal.tsx`
**掛載**: `src/pages/HighSpeedInventory.tsx` 加 "New Task" 按鈕

**Modal 欄位**: name, scope_location_id, method (barcode/rfid), planned_date, assigned_to
**Hook**: `useCreateInventoryTask()`
**改動量**: ~70 行新組件 + 15 行頁面改動

---

### C3. SystemSettings — Create Adapter Modal

**檔案**: 新建 `src/components/CreateAdapterModal.tsx`
**掛載**: `src/pages/SystemSettings.tsx` Integrations tab

**Modal 欄位**: name, type (dify/rest/grpc), direction (inbound/outbound), endpoint, enabled
**Hook**: `useCreateAdapter()`
**改動量**: ~60 行新組件 + 10 行頁面改動

---

### C4. SystemSettings — Create Webhook Modal

**檔案**: 新建 `src/components/CreateWebhookModal.tsx`
**掛載**: `src/pages/SystemSettings.tsx` Integrations tab

**Modal 欄位**: name, url, secret, events (multi-select), enabled
**Hook**: `useCreateWebhook()`
**改動量**: ~70 行新組件 + 10 行頁面改動

---

### C5. PredictiveHub — Create RCA Modal

**檔案**: 新建 `src/components/CreateRCAModal.tsx`
**掛載**: `src/pages/PredictiveHub.tsx` line 560 附近

**Modal 欄位**: incident_id (dropdown from incidents), model_name (dropdown from models), context (textarea)
**Hook**: `useCreateRCA()`
**改動量**: ~80 行新組件 + 15 行頁面改動（需要 useIncidents hook 取 incident 列表）

---

### C6. 拓撲頁面 — Create Location Modal

**檔案**: 新建 `src/components/CreateLocationModal.tsx`
**掛載**: GlobalOverview 或 CampusOverview 加 "Add Location" 按鈕

**Modal 欄位**: name, name_en, slug, level (country/region/city/campus), parent_id (dropdown), status
**Hook**: `useCreateLocation()`
**改動量**: ~80 行新組件 + 10 行頁面改動

---

## 阻塞項

| 項目 | 阻塞原因 | 解決方案 |
|------|---------|---------|
| A6 RolesPermissions "Save Changes" | 沒有 `PUT /roles/{id}` 端點 | 需先加後端端點 |
| B2 AssetDetail Delete | `useDeleteAsset` hook 不存在 | 需先加到 useAssets.ts |
| C5 PredictiveHub RCA | Modal 需要 incident 列表 | 需加 `useIncidents()` query hook |

---

## 執行順序

```
Phase 1 (A類): A1 → A2 → A3 → A4 → A5          ≈ 5 個改動
Phase 2 (B類): B1+B2 → B3 → B4 → B5 → B6       ≈ 6 個改動
Phase 3 (C類): C1 → C2 → C3+C4 → C5 → C6       ≈ 6 個組件
```

## 預估改動量

| Phase | 新檔案 | 修改檔案 | 新增行數 |
|-------|--------|---------|---------|
| A 類 | 0 | 5 頁面 | ~150 行 |
| B 類 | 0 | 4 頁面 + 1 hook | ~250 行 |
| C 類 | 6 modal | 6 頁面 | ~500 行 |
| **合計** | **6** | **~12** | **~900 行** |
