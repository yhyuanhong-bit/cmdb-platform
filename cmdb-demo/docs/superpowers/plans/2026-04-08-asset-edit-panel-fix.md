# Asset Edit Panel Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the asset edit panel to support all editable fields with proper input controls, validation, and i18n.

**Architecture:** Frontend-only changes to AssetDetailUnified.tsx. No backend changes needed — PUT /assets/:id already supports all required fields (name, status, bia_level, vendor, model, location_id, rack_id, tags, attributes, ip_address). Only need to add serial_number and ip_address to the backend.

**Tech Stack:** React/TypeScript, existing hooks (useAsset, useUpdateAsset, useTopology).

**Spec:** Based on page-by-page analysis of AssetDetailUnified.tsx vs backend PUT /assets/:id capabilities.

---

## File Structure

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-demo/src/pages/AssetDetailUnified.tsx` | Rebuild edit panel with proper controls |
| `cmdb-demo/src/i18n/locales/en.json` | Add asset edit translation keys |
| `cmdb-demo/src/i18n/locales/zh-CN.json` | Same |
| `cmdb-demo/src/i18n/locales/zh-TW.json` | Same |
| `cmdb-core/internal/api/impl.go` | Add serial_number + ip_address to UpdateAsset handler |

---

## Problems to Fix (7 items)

| # | Problem | Fix |
|---|---------|-----|
| 1 | `status` is free-text input | Change to `<select>` dropdown with valid values |
| 2 | `bia_level` is free-text input | Change to `<select>` dropdown with valid values |
| 3 | `ip_address` not editable | Add to edit panel |
| 4 | `serial_number` not editable | Add to edit panel |
| 5 | `location_id` / `rack_id` not editable | Add location + rack dropdowns |
| 6 | `tags` not editable | Add tag input (comma-separated) |
| 7 | Edit panel i18n — hardcoded "Edit Asset", "Save Changes", "Cancel", "Delete", field labels | Use t() calls |

---

## Task 1: Backend — Add serial_number + ip_address to UpdateAsset

**Files:**
- Modify: `cmdb-core/internal/api/impl.go`

- [ ] **Step 1: Read the UpdateAsset handler** (around line 345-415)

Find the section that checks `req.Model != nil` and add two more fields after it:

```go
	if req.SerialNumber != nil {
		params.SerialNumber = pgtype.Text{String: *req.SerialNumber, Valid: true}
	}
	if req.IpAddress != nil {
		params.IpAddress = pgtype.Text{String: *req.IpAddress, Valid: true}
	}
```

Also add to the diff map:
```go
	if req.SerialNumber != nil {
		diff["serial_number"] = *req.SerialNumber
	}
	if req.IpAddress != nil {
		diff["ip_address"] = *req.IpAddress
	}
```

Note: Check if `UpdateAssetParams` and `UpdateAssetJSONRequestBody` already have these fields. If not, they need to be added to the SQL query in `db/queries/assets.sql` and the OpenAPI spec. If sqlc is not available, add the fields via raw SQL in a custom endpoint instead.

- [ ] **Step 2: Verify the SQL update query supports these columns**

Read `cmdb-core/db/queries/assets.sql` and find the UpdateAsset query. Check if it includes `serial_number` and `ip_address` in the SET clause. If not, the simplest fix is to handle these two fields in the existing handler using a supplementary raw SQL update.

- [ ] **Step 3: Build and verify**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/api/impl.go
git commit -m "feat: add serial_number and ip_address to UpdateAsset handler"
```

---

## Task 2: Add i18n Keys for Edit Panel

**Files:**
- Modify: `cmdb-demo/src/i18n/locales/en.json`
- Modify: `cmdb-demo/src/i18n/locales/zh-CN.json`
- Modify: `cmdb-demo/src/i18n/locales/zh-TW.json`

- [ ] **Step 1: Add keys to all 3 locale files**

Add to the `asset_detail` namespace:

**en.json:**
```json
"edit_title": "Edit Asset",
"edit_save": "Save Changes",
"edit_saving": "Saving...",
"edit_cancel": "Cancel",
"btn_delete": "Delete",
"btn_deleting": "Deleting...",
"confirm_delete": "Are you sure you want to delete this asset?",
"field_name": "Name",
"field_status": "Status",
"field_vendor": "Vendor",
"field_model": "Model",
"field_bia_level": "BIA Level",
"field_serial_number": "Serial Number",
"field_ip_address": "IP Address",
"field_location": "Location",
"field_rack": "Rack",
"field_tags": "Tags",
"tags_hint": "Comma-separated",
"status_inventoried": "Inventoried",
"status_operational": "Operational",
"status_deployed": "Deployed",
"status_maintenance": "Maintenance",
"status_retired": "Retired",
"bia_critical": "Critical",
"bia_important": "Important",
"bia_normal": "Normal",
"bia_minor": "Minor",
"select_location": "Select location...",
"select_rack": "Select rack...",
"no_rack": "No rack"
```

**zh-CN.json:**
```json
"edit_title": "编辑资产",
"edit_save": "保存更改",
"edit_saving": "保存中...",
"edit_cancel": "取消",
"btn_delete": "删除",
"btn_deleting": "删除中...",
"confirm_delete": "确定要删除此资产吗？",
"field_name": "名称",
"field_status": "状态",
"field_vendor": "供应商",
"field_model": "型号",
"field_bia_level": "BIA 等级",
"field_serial_number": "序列号",
"field_ip_address": "IP 地址",
"field_location": "位置",
"field_rack": "机柜",
"field_tags": "标签",
"tags_hint": "逗号分隔",
"status_inventoried": "已登记",
"status_operational": "运行中",
"status_deployed": "已部署",
"status_maintenance": "维护中",
"status_retired": "已退役",
"bia_critical": "关键",
"bia_important": "重要",
"bia_normal": "一般",
"bia_minor": "次要",
"select_location": "选择位置...",
"select_rack": "选择机柜...",
"no_rack": "无机柜"
```

**zh-TW.json:**
```json
"edit_title": "編輯資產",
"edit_save": "儲存變更",
"edit_saving": "儲存中...",
"edit_cancel": "取消",
"btn_delete": "刪除",
"btn_deleting": "刪除中...",
"confirm_delete": "確定要刪除此資產嗎？",
"field_name": "名稱",
"field_status": "狀態",
"field_vendor": "供應商",
"field_model": "型號",
"field_bia_level": "BIA 等級",
"field_serial_number": "序號",
"field_ip_address": "IP 位址",
"field_location": "位置",
"field_rack": "機櫃",
"field_tags": "標籤",
"tags_hint": "逗號分隔",
"status_inventoried": "已登錄",
"status_operational": "運作中",
"status_deployed": "已部署",
"status_maintenance": "維護中",
"status_retired": "已退役",
"bia_critical": "關鍵",
"bia_important": "重要",
"bia_normal": "一般",
"bia_minor": "次要",
"select_location": "選擇位置...",
"select_rack": "選擇機櫃...",
"no_rack": "無機櫃"
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/i18n/locales/*.json
git commit -m "feat: add asset edit panel i18n keys (30 keys x 3 languages)"
```

---

## Task 3: Rebuild Edit Panel in AssetDetailUnified.tsx

**Files:**
- Modify: `cmdb-demo/src/pages/AssetDetailUnified.tsx`

- [ ] **Step 1: Read the current edit panel code** (lines 1008-1098)

The current edit panel:
```tsx
setEditData({
  name: apiAsset?.name || '',
  status: apiAsset?.status || '',
  vendor: apiAsset?.vendor || '',
  model: apiAsset?.model || '',
  bia_level: apiAsset?.bia_level || '',
})
```
And renders 5 plain text inputs.

- [ ] **Step 2: Add imports for location/rack hooks**

At the top of the file, add:
```tsx
import { useRootLocations, useLocationDescendants, useRacks } from '../hooks/useTopology'
```

- [ ] **Step 3: Add location/rack queries in the main component**

Inside the main `AssetDetailUnified` component, after the existing hooks:
```tsx
const rootLocQ = useRootLocations()
const firstTerritoryId = rootLocQ.data?.data?.[0]?.id ?? ''
const descQ = useLocationDescendants(firstTerritoryId)
const allLocations = descQ.data?.data ?? []
// Filter to only room/idc/module level locations for dropdown
const editableLocations = allLocations.filter((l: any) =>
  ['room', 'module', 'idc', 'campus'].includes(l.level)
)
```

- [ ] **Step 4: Expand editData to include new fields**

Change the edit button onClick:
```tsx
setEditData({
  name: apiAsset?.name || '',
  status: apiAsset?.status || '',
  vendor: apiAsset?.vendor || '',
  model: apiAsset?.model || '',
  bia_level: apiAsset?.bia_level || '',
  serial_number: apiAsset?.serial_number || '',
  ip_address: apiAsset?.ip_address || '',
  location_id: apiAsset?.location_id || '',
  rack_id: apiAsset?.rack_id || '',
  tags: (apiAsset?.tags || []).join(', '),
})
```

- [ ] **Step 5: Replace the edit panel HTML**

Replace the entire `{editing && (...)}` block (lines 1064-1098) with:

```tsx
{editing && (
  <div className="px-8 py-4">
    <div className="bg-surface-container rounded-lg p-5 space-y-4">
      <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wider">
        {t('asset_detail.edit_title')}
      </h3>
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">

        {/* Name — text input */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_name')}
          </label>
          <input value={editData.name ?? ''} onChange={e => setEditData(p => ({ ...p, name: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm" />
        </div>

        {/* Status — dropdown */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_status')}
          </label>
          <select value={editData.status ?? ''} onChange={e => setEditData(p => ({ ...p, status: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm">
            <option value="inventoried">{t('asset_detail.status_inventoried')}</option>
            <option value="operational">{t('asset_detail.status_operational')}</option>
            <option value="deployed">{t('asset_detail.status_deployed')}</option>
            <option value="maintenance">{t('asset_detail.status_maintenance')}</option>
            <option value="retired">{t('asset_detail.status_retired')}</option>
          </select>
        </div>

        {/* BIA Level — dropdown */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_bia_level')}
          </label>
          <select value={editData.bia_level ?? ''} onChange={e => setEditData(p => ({ ...p, bia_level: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm">
            <option value="critical">{t('asset_detail.bia_critical')}</option>
            <option value="important">{t('asset_detail.bia_important')}</option>
            <option value="normal">{t('asset_detail.bia_normal')}</option>
            <option value="minor">{t('asset_detail.bia_minor')}</option>
          </select>
        </div>

        {/* Vendor — text input */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_vendor')}
          </label>
          <input value={editData.vendor ?? ''} onChange={e => setEditData(p => ({ ...p, vendor: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm" />
        </div>

        {/* Model — text input */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_model')}
          </label>
          <input value={editData.model ?? ''} onChange={e => setEditData(p => ({ ...p, model: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm" />
        </div>

        {/* Serial Number — text input */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_serial_number')}
          </label>
          <input value={editData.serial_number ?? ''} onChange={e => setEditData(p => ({ ...p, serial_number: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm" />
        </div>

        {/* IP Address — text input */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_ip_address')}
          </label>
          <input value={editData.ip_address ?? ''} onChange={e => setEditData(p => ({ ...p, ip_address: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
            placeholder="192.168.1.100" />
        </div>

        {/* Location — dropdown */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_location')}
          </label>
          <select value={editData.location_id ?? ''} onChange={e => setEditData(p => ({ ...p, location_id: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm">
            <option value="">{t('asset_detail.select_location')}</option>
            {editableLocations.map((loc: any) => (
              <option key={loc.id} value={loc.id}>{loc.name} ({loc.level})</option>
            ))}
          </select>
        </div>

        {/* Tags — comma-separated text input */}
        <div className="flex flex-col gap-1">
          <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
            {t('asset_detail.field_tags')}
            <span className="ml-1 text-on-surface-variant/50 normal-case">({t('asset_detail.tags_hint')})</span>
          </label>
          <input value={editData.tags ?? ''} onChange={e => setEditData(p => ({ ...p, tags: e.target.value }))}
            className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
            placeholder="production, tier-1" />
        </div>

      </div>

      {/* Buttons */}
      <div className="flex gap-2 mt-4">
        <button onClick={() => {
          // Build the update payload — convert tags string to array
          const payload: Record<string, any> = { ...editData }
          if (typeof payload.tags === 'string') {
            payload.tags = payload.tags.split(',').map((t: string) => t.trim()).filter(Boolean)
          }
          // Remove empty strings to avoid overwriting with empty
          Object.keys(payload).forEach(k => {
            if (payload[k] === '') delete payload[k]
          })
          updateAsset.mutate({ id: assetId!, data: payload }, {
            onSuccess: (resp: any) => {
              setEditing(false)
              if (resp?.meta?.change_order_id) {
                alert(`Critical asset change recorded. Audit order: ${resp.meta.change_order_id}`)
              }
            }
          })
        }} disabled={updateAsset.isPending}
          className="px-4 py-2 rounded bg-blue-600 text-white text-sm font-semibold hover:bg-blue-500 disabled:opacity-50">
          {updateAsset.isPending ? t('asset_detail.edit_saving') : t('asset_detail.edit_save')}
        </button>
        <button onClick={() => setEditing(false)}
          className="px-4 py-2 rounded bg-gray-700 text-white text-sm hover:bg-gray-600">
          {t('asset_detail.edit_cancel')}
        </button>
      </div>
    </div>
  </div>
)}
```

- [ ] **Step 6: Fix the Delete button i18n**

Find the delete button (around line 1022-1029) and replace:
```tsx
// Old:
if (confirm('Are you sure you want to delete this asset?'))
// New:
if (confirm(t('asset_detail.confirm_delete')))

// Old:
{deleteAsset.isPending ? 'Deleting...' : 'Delete'}
// New:
{deleteAsset.isPending ? t('asset_detail.btn_deleting') : t('asset_detail.btn_delete')}
```

- [ ] **Step 7: Commit**

```bash
git add cmdb-demo/src/pages/AssetDetailUnified.tsx
git commit -m "feat: rebuild asset edit panel with dropdowns, new fields, and i18n

- status/bia_level: dropdown instead of free text
- Added: serial_number, ip_address, location, tags
- All labels and buttons use t() translations
- Tags input as comma-separated with array conversion on save"
```

---

## Task 4: Verification

- [ ] **Step 1: TypeScript check**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit 2>&1 | grep "AssetDetail" | head -10
```

- [ ] **Step 2: Go build**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

- [ ] **Step 3: Functional test**

1. Open http://localhost:5175/assets
2. Click any asset to open detail
3. Click "Edit Asset" button
4. Verify:
   - Status shows dropdown with 5 options
   - BIA Level shows dropdown with 4 options
   - Serial Number and IP Address fields appear
   - Location dropdown shows available locations
   - Tags field shows current tags comma-separated
   - All labels are translated based on current language
5. Change status from dropdown, click Save
6. Verify the asset status actually updated
7. Switch language (EN → 中文), verify all labels change
