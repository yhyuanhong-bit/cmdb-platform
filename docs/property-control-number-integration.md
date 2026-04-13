# Property Number + Control Number Integration Report

**Date:** 2026-04-08
**Status:** Fields exist in DB/API but completely unused in frontend
**Risk Level:** Zero — all changes are additive, no existing logic modified

---

## 1. Current State Analysis

### What Already Works (No Changes Needed)

| Layer | Component | Status | Evidence |
|-------|-----------|--------|----------|
| **DB Schema** | `assets.property_number VARCHAR(100)` | ✅ Exists | Migration 000004 |
| **DB Schema** | `assets.control_number VARCHAR(100)` | ✅ Exists | Migration 000004 |
| **Go Create** | `CreateAsset` writes both fields | ✅ Works | impl.go:294-295 |
| **Go Update** | `UpdateAsset` supports COALESCE update | ✅ Works | assets.sql:43-44 |
| **Go Query** | All SELECT queries include both fields | ✅ Returns | assets.sql.go:74,157,197,228,259 |
| **API Response** | JSON fields `property_number`, `control_number` | ✅ Returns | generated.go:49,55 |
| **TS Types** | `property_number?: string \| null` | ✅ Typed | api-types.ts:651-652 |
| **Pipeline Normalize** | Aliases: `prop_number` → `property_number` | ✅ Works | normalize.py:17-18 |
| **Pipeline Create** | `_create_asset()` writes both fields | ✅ Works | processor.py:179-180 |
| **Pipeline Dedup** | SELECT includes both in existing_fields | ✅ Reads | deduplicate.py:39,52 |

### What's Missing (Changes Needed)

| Layer | Component | Status | Impact |
|-------|-----------|--------|--------|
| **Seed Data** | 20 assets all have NULL values | ❌ Empty | No demo data |
| **Asset List Page** | Table doesn't show these columns | ❌ Missing | Users can't see |
| **Asset Detail Page** | Overview doesn't display them | ❌ Missing | Users can't see |
| **Asset Edit Panel** | editData doesn't include them | ❌ Missing | Users can't edit |
| **Create Asset Modal** | Form has no input for them | ❌ Missing | Users can't set on create |
| **Inventory Import API** | Only accepts asset_tag, serial_number, expected_location | ❌ Missing | Can't match by property/control |
| **Inventory Match Logic** | `FindBySerialOrTag` only checks serial + asset_tag | ❌ Missing | Can't match by property/control |
| **i18n Keys** | No translation keys for field labels | ❌ Missing | Labels untranslatable |

---

## 2. Business Logic: 1:1:1 Relationship

In enterprise IT asset management, every physical hardware asset has three identifiers:

```
┌─────────────────────────┐
│  Physical Hardware      │
│  (e.g., Dell R750)      │
├─────────────────────────┤
│ serial_number           │ ← Manufacturer assigns (printed on device)
│ e.g., CN0RKVY6SDE001   │   Unique globally, immutable
├─────────────────────────┤
│ property_number         │ ← Finance/accounting assigns (asset register)
│ e.g., P-2024-00123     │   Unique within organization, for depreciation tracking
├─────────────────────────┤
│ control_number          │ ← IT asset management assigns (CMDB register)
│ e.g., CTRL-TW-A-0456   │   Unique within CMDB, for operational tracking
├─────────────────────────┤
│ asset_tag               │ ← System auto-generates (internal reference)
│ e.g., SRV-PROD-001     │   Unique in system, for daily operations
└─────────────────────────┘

Relationship: 1 serial_number : 1 property_number : 1 control_number : 1 asset_tag
```

**Why all four matter:**
- `serial_number` — Hardware identity (vendor support, warranty lookup)
- `property_number` — Financial identity (depreciation, tax, insurance)
- `control_number` — Administrative identity (audit, compliance, regulatory)
- `asset_tag` — Operational identity (daily management, work orders)

**Inventory use case:** During physical inventory (盤點), the auditor scans any of these numbers. The system must be able to match from ANY of them.

---

## 3. Implementation Plan

### Task 1: Seed Data — Add property_number + control_number to 20 assets

**File:** Run SQL directly

```sql
UPDATE assets SET property_number = 'P-2025-0001', control_number = 'CTRL-TW-A-0001' WHERE asset_tag = 'SRV-PROD-001';
UPDATE assets SET property_number = 'P-2025-0002', control_number = 'CTRL-TW-A-0002' WHERE asset_tag = 'SRV-DB-001';
UPDATE assets SET property_number = 'P-2025-0003', control_number = 'CTRL-TW-A-0003' WHERE asset_tag = 'SRV-APP-001';
-- ... all 20 assets
```

Pattern: `P-{year}-{4-digit seq}` for property, `CTRL-{region}-{rack_row}-{4-digit seq}` for control.

**Effort:** 0.5h | **Risk:** Zero — only adds data, doesn't change schema

---

### Task 2: i18n — Add translation keys

**Files:** 3 locale files

```
asset_detail.field_property_number:
  EN: "Property Number"
  CN: "财产编号"
  TW: "財產編號"

asset_detail.field_control_number:
  EN: "Control Number"
  CN: "管制编号"
  TW: "管制編號"

assets.table_property_number:
  EN: "Property No."
  CN: "财产编号"
  TW: "財產編號"

assets.table_control_number:
  EN: "Control No."
  CN: "管制编号"
  TW: "管制編號"
```

**Effort:** 0.5h | **Risk:** Zero

---

### Task 3: Asset List Page — Add 2 columns

**File:** `cmdb-demo/src/pages/AssetManagementUnified.tsx`

Add two columns to the table between "Asset No" and "Name":

| Asset No | Property No. | Control No. | Name | ... |
|----------|-------------|-------------|------|-----|
| SRV-PROD-001 | P-2025-0001 | CTRL-TW-A-0001 | Production Server 01 | ... |

Card view: add property_number and control_number to the card info section.

**Effort:** 0.5h | **Risk:** Zero — additive column, doesn't affect existing columns

---

### Task 4: Asset Detail Page — Display + Edit

**File:** `cmdb-demo/src/pages/AssetDetailUnified.tsx`

#### Display (Overview tab):
Add 2 rows to the asset info section:
```
Property Number: P-2025-0001
Control Number:  CTRL-TW-A-0001
```

#### Edit panel:
Add to `editData` initialization:
```tsx
setEditData({
  ...existing fields,
  property_number: apiAsset?.property_number || '',
  control_number: apiAsset?.control_number || '',
})
```

Add 2 text input fields to the edit grid.

**Effort:** 1h | **Risk:** Zero — edit panel already handles partial updates via COALESCE

---

### Task 5: Create Asset Modal — Add 2 fields

**File:** `cmdb-demo/src/components/CreateAssetModal.tsx`

Add property_number and control_number text inputs to the form. Both optional.

**Effort:** 0.5h | **Risk:** Zero — Go CreateAsset already accepts these fields

---

### Task 6: Inventory Import API — Add 2 fields

**File:** `cmdb-core/internal/api/impl.go` + `cmdb-core/internal/api/generated.go`

#### 6a: Extend import request body

Add to `ImportInventoryItemsJSONBody.Items`:
```go
PropertyNumber   *string `json:"property_number,omitempty"`
ControlNumber    *string `json:"control_number,omitempty"`
```

#### 6b: Extend matching logic

Current `FindBySerialOrTag` matches by:
1. serial_number
2. asset_tag

Extend to also match by:
3. property_number
4. control_number

```go
// In ImportInventoryItems handler, after checking serial_number and asset_tag:
if asset == nil && item.PropertyNumber != nil && *item.PropertyNumber != "" {
    asset, _ = findAssetByPropertyNumber(ctx, tenantID, *item.PropertyNumber)
}
if asset == nil && item.ControlNumber != nil && *item.ControlNumber != "" {
    asset, _ = findAssetByControlNumber(ctx, tenantID, *item.ControlNumber)
}
```

Add raw SQL queries:
```sql
SELECT * FROM assets WHERE tenant_id = $1 AND property_number = $2 LIMIT 1
SELECT * FROM assets WHERE tenant_id = $1 AND control_number = $2 LIMIT 1
```

**Effort:** 1h | **Risk:** Low — extends existing match logic, doesn't change current behavior

---

### Task 7: Inventory Excel Import — Update template + parser

**File:** `cmdb-demo/src/pages/HighSpeedInventory.tsx`

Excel template columns (5 columns):
| asset_tag | serial_number | property_number | control_number | expected_location |
|-----------|--------------|----------------|----------------|-------------------|

Parser extracts all 5 fields from Excel rows and sends to import API.

**Effort:** 0.5h (part of the Excel import feature) | **Risk:** Zero

---

## 4. Impact Analysis

### Affected Pages

| Page | Change | Risk |
|------|--------|------|
| AssetManagementUnified | Add 2 table columns | Zero — additive |
| AssetDetailUnified | Add 2 display rows + 2 edit fields | Zero — additive |
| CreateAssetModal | Add 2 form inputs | Zero — additive |
| HighSpeedInventory | Excel template + parser includes 2 more fields | Zero — additive |
| InventoryItemDetail | Display property/control in asset info | Zero — additive |

### NOT Affected (Zero Changes)

| Component | Reason |
|-----------|--------|
| All Go domain services | No logic changes, fields already in CRUD |
| All SQL queries | Already SELECT/INSERT/UPDATE these columns |
| Pipeline (normalize, dedup, validate, authority) | Already handles these fields |
| All other pages (Dashboard, BIA, Monitoring, etc.) | Don't interact with these fields |
| DB schema | Columns already exist |
| API response types | Already include these fields |

### Data Integrity

| Concern | Assessment |
|---------|-----------|
| Uniqueness | DB has no UNIQUE constraint on property_number or control_number. **Should consider adding** `UNIQUE(tenant_id, property_number)` and `UNIQUE(tenant_id, control_number)` to prevent duplicates. |
| NULL handling | Both columns are nullable. Existing assets with NULL values are unaffected. |
| Backward compatibility | 100% backward compatible. No existing API consumers will break. |

---

## 5. Execution Summary

| # | Task | Backend | Frontend | i18n | Effort |
|---|------|---------|----------|------|--------|
| 1 | Seed data | SQL UPDATE | — | — | 0.5h |
| 2 | i18n keys | — | — | 3 files | 0.5h |
| 3 | Asset list columns | — | AssetManagement | — | 0.5h |
| 4 | Asset detail display + edit | — | AssetDetail | — | 1h |
| 5 | Create asset modal | — | CreateAssetModal | — | 0.5h |
| 6 | Inventory import API | impl.go + generated.go | — | — | 1h |
| 7 | Excel import template | — | HighSpeedInventory | — | 0.5h |
| **Total** | | **1h** | **2.5h** | **0.5h** | **4h** |

### Optional: Add UNIQUE Constraints

```sql
-- Migration 000021 (optional but recommended)
CREATE UNIQUE INDEX idx_assets_property_number ON assets(tenant_id, property_number) WHERE property_number IS NOT NULL;
CREATE UNIQUE INDEX idx_assets_control_number ON assets(tenant_id, control_number) WHERE control_number IS NOT NULL;
```

This ensures no two assets in the same tenant can share the same property_number or control_number, enforcing the 1:1:1 relationship at the database level.
