# Inventory Excel Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Demo Import button in HighSpeedInventory with a real Excel/CSV file upload that parses on the frontend and calls the existing inventory import API.

**Architecture:** Frontend-only. SheetJS (xlsx) library parses Excel/CSV in the browser. Extracted rows sent to existing `POST /inventory/tasks/{id}/import` API. Backend already supports 5 fields: asset_tag, serial_number, property_number, control_number, expected_location.

**Tech Stack:** SheetJS (xlsx npm package), React, existing inventory API hooks.

**Brainstorm decisions:**
- Excel parsing: frontend (SheetJS) — no backend changes needed
- Upload flow: direct import, no preview (Phase 1; preview can be added later)
- Column format: fixed column names + downloadable template
- 5 columns: asset_tag, serial_number, property_number, control_number, expected_location

---

## File Structure

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-demo/package.json` | Add `xlsx` dependency |
| `cmdb-demo/src/pages/HighSpeedInventory.tsx` | Replace Demo button with file upload + download template |
| `cmdb-demo/src/i18n/locales/en.json` | Add inventory import i18n keys |
| `cmdb-demo/src/i18n/locales/zh-CN.json` | Same |
| `cmdb-demo/src/i18n/locales/zh-TW.json` | Same |

### No New Files

Everything fits in the existing HighSpeedInventory page. No new components needed.

---

## Task 1: Install SheetJS

- [ ] **Step 1: Add xlsx package**

```bash
cd /cmdb-platform/cmdb-demo
npm install xlsx
```

- [ ] **Step 2: Verify import works**

```bash
node -e "const XLSX = require('xlsx'); console.log('SheetJS version:', XLSX.version)"
```

- [ ] **Step 3: Commit**

```bash
git add package.json package-lock.json
git commit -m "feat: add SheetJS (xlsx) for Excel parsing in inventory import"
```

---

## Task 2: Add i18n Keys

- [ ] **Step 1: Add keys to all 3 locale files**

Add to `inventory` namespace:

**en.json:**
```json
"btn_upload_excel": "Upload Excel / CSV",
"btn_uploading": "Importing...",
"btn_download_template": "Download Template",
"import_success": "Import complete: {{matched}} matched, {{not_found}} not found, {{total}} total",
"import_error": "Import failed. Please check the file format.",
"import_no_task": "No active inventory task. Create a task first.",
"import_no_data": "No valid data rows found in the file.",
"import_columns_hint": "Required columns: asset_tag, serial_number, property_number, control_number, expected_location"
```

**zh-CN.json:**
```json
"btn_upload_excel": "上传 Excel / CSV",
"btn_uploading": "导入中...",
"btn_download_template": "下载模板",
"import_success": "导入完成：{{matched}} 匹配，{{not_found}} 未找到，共 {{total}} 条",
"import_error": "导入失败，请检查文件格式。",
"import_no_task": "没有进行中的盘点任务，请先创建任务。",
"import_no_data": "文件中未找到有效数据行。",
"import_columns_hint": "所需列：资产编号、序列号、财产编号、管制编号、预期位置"
```

**zh-TW.json:**
```json
"btn_upload_excel": "上傳 Excel / CSV",
"btn_uploading": "匯入中...",
"btn_download_template": "下載範本",
"import_success": "匯入完成：{{matched}} 匹配，{{not_found}} 未找到，共 {{total}} 筆",
"import_error": "匯入失敗，請檢查檔案格式。",
"import_no_task": "沒有進行中的盤點任務，請先建立任務。",
"import_no_data": "檔案中未找到有效資料列。",
"import_columns_hint": "所需欄位：資產編號、序號、財產編號、管制編號、預期位置"
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/i18n/locales/*.json
git commit -m "feat: add inventory Excel import i18n keys (8 keys x 3 languages)"
```

---

## Task 3: Replace Demo Button with Excel Upload

This is the main task. Modify `cmdb-demo/src/pages/HighSpeedInventory.tsx`.

- [ ] **Step 1: Read the current Demo Import section**

Find the Demo Import button (around lines 241-268). The current code:
```tsx
<button onClick={() => {
  const demo = [
    { asset_tag: 'SRV-PROD-001', serial_number: 'SN-DELL-001', expected_location: 'RACK-A01' },
    // ...
  ]
  if (currentTask) {
    importItems.mutate({ taskId: currentTask.id, items: demo }, { ... })
  }
}} ...>
  {importItems.isPending ? 'Importing...' : 'Run Demo Import'}
</button>
```

- [ ] **Step 2: Add SheetJS import at the top of the file**

```tsx
import * as XLSX from 'xlsx'
```

Also add `useRef` to the React import if not already there.

- [ ] **Step 3: Add file input ref and handler**

Inside the component, add:

```tsx
const fileInputRef = useRef<HTMLInputElement>(null)

const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
  const file = e.target.files?.[0]
  if (!file) return
  if (!currentTask) {
    alert(t('inventory.import_no_task'))
    return
  }

  const reader = new FileReader()
  reader.onload = (evt) => {
    try {
      const data = evt.target?.result
      const workbook = XLSX.read(data, { type: 'array' })
      const firstSheet = workbook.Sheets[workbook.SheetNames[0]]
      const rows: any[] = XLSX.utils.sheet_to_json(firstSheet)

      if (rows.length === 0) {
        alert(t('inventory.import_no_data'))
        return
      }

      // Map Excel rows to API format
      const items = rows.map((row: any) => ({
        asset_tag: row.asset_tag || row['Asset Tag'] || row['资产编号'] || row['資產編號'] || undefined,
        serial_number: row.serial_number || row['Serial Number'] || row['序列号'] || row['序號'] || undefined,
        property_number: row.property_number || row['Property Number'] || row['财产编号'] || row['財產編號'] || undefined,
        control_number: row.control_number || row['Control Number'] || row['管制编号'] || row['管制編號'] || undefined,
        expected_location: row.expected_location || row['Expected Location'] || row['预期位置'] || row['預期位置'] || undefined,
      })).filter((item: any) =>
        item.asset_tag || item.serial_number || item.property_number || item.control_number
      )

      if (items.length === 0) {
        alert(t('inventory.import_no_data'))
        return
      }

      importItems.mutate({ taskId: currentTask.id, items }, {
        onSuccess: (resp: any) => {
          const d = resp?.data ?? {}
          alert(t('inventory.import_success', {
            matched: d.matched ?? 0,
            not_found: d.not_found ?? 0,
            total: d.total ?? 0,
          }))
        },
        onError: () => alert(t('inventory.import_error')),
      })
    } catch {
      alert(t('inventory.import_error'))
    }
  }
  reader.readAsArrayBuffer(file)

  // Reset input so same file can be re-uploaded
  e.target.value = ''
}
```

Note on column name mapping: The handler accepts BOTH English column names (asset_tag) AND Chinese column names (資產編號/资产编号), so the template works regardless of language.

- [ ] **Step 4: Add template download handler**

```tsx
const handleDownloadTemplate = () => {
  const headers = ['asset_tag', 'serial_number', 'property_number', 'control_number', 'expected_location']
  const exampleRow = ['SRV-PROD-001', 'SN-DELL-001', 'P-2025-0001', 'CTRL-TW-A-0001', 'RACK-A01']

  const ws = XLSX.utils.aoa_to_sheet([headers, exampleRow])
  // Set column widths
  ws['!cols'] = headers.map(() => ({ wch: 20 }))
  const wb = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(wb, ws, 'Inventory')
  XLSX.writeFile(wb, 'inventory_import_template.xlsx')
}
```

- [ ] **Step 5: Replace the Demo button HTML**

Remove the entire Demo Import button block (lines ~241-268). Replace with:

```tsx
<div className="mt-auto pt-4 flex flex-col gap-2">
  {/* Column hint */}
  <p className="text-[10px] text-on-surface-variant">
    {t('inventory.import_columns_hint')}
  </p>

  {/* Hidden file input */}
  <input
    ref={fileInputRef}
    type="file"
    accept=".xlsx,.xls,.csv"
    onChange={handleFileUpload}
    hidden
  />

  {/* Upload button */}
  <button
    onClick={() => fileInputRef.current?.click()}
    disabled={importItems.isPending || !currentTask}
    className="bg-primary hover:opacity-90 text-on-primary px-3 py-1.5 rounded-lg text-xs font-label font-bold flex items-center gap-2 transition-opacity w-fit disabled:opacity-50"
  >
    <Icon name="cloud_upload" className="text-sm" />
    {importItems.isPending ? t('inventory.btn_uploading') : t('inventory.btn_upload_excel')}
  </button>

  {/* Download template button */}
  <button
    onClick={handleDownloadTemplate}
    className="text-xs text-on-surface-variant hover:text-primary flex items-center gap-1 transition-colors"
  >
    <Icon name="download" className="text-sm" />
    {t('inventory.btn_download_template')}
  </button>
</div>
```

- [ ] **Step 6: Remove the hardcoded file name display**

Find `IDC01_Q3_assets.xlsx` hardcoded string (around line 244). Remove or replace with dynamic file info display if needed.

- [ ] **Step 7: Commit**

```bash
git add cmdb-demo/src/pages/HighSpeedInventory.tsx
git commit -m "feat: replace Demo Import with real Excel/CSV upload

- SheetJS parses Excel/CSV in browser
- Supports 5 columns: asset_tag, serial_number, property_number, control_number, expected_location
- Accepts both English and Chinese column names
- Download template button generates .xlsx with headers + example row
- All UI text uses i18n translations"
```

---

## Task 4: Verification

- [ ] **Step 1: TypeScript check**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit 2>&1 | grep "HighSpeedInventory" | head -5
```

- [ ] **Step 2: Create a test Excel file**

```bash
cd /tmp
python3 -c "
import json
# Create a simple CSV for testing
with open('test_inventory.csv', 'w') as f:
    f.write('asset_tag,serial_number,property_number,control_number,expected_location\n')
    f.write('SRV-PROD-001,TEST-SN-001,P-2025-0001,CTRL-TW-A-0001,RACK-A01\n')
    f.write('UNKNOWN-999,SN-FAKE-999,P-9999-0099,CTRL-XX-X-9999,RACK-X99\n')
print('Test CSV created at /tmp/test_inventory.csv')
"
```

- [ ] **Step 3: Functional test**

1. Open http://localhost:5175/inventory
2. Create a task (or use existing)
3. Click "Upload Excel / CSV" button
4. Select the test CSV file
5. Verify alert shows: "Import complete: 1 matched, 1 not found, 2 total"
6. Click "Download Template" button
7. Verify .xlsx file downloads with correct headers
8. Switch language to Chinese
9. Verify all buttons and messages display in Chinese
