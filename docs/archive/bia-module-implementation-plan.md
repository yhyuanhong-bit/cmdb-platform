# BIA 模組完整實施計劃

> 目標：為 CMDB 平台新增業務影響分析（BIA）模組
> 設計參考：`stitch_remix_of_cmdb_platform_BIA/screen.png`（暗色模式 BIA Modeler）
> UI 規範：統一使用平台現有設計系統（非 DESIGN.md 的 Navy 配色）
> 預估改動量：~2,500 行新代碼

---

## 一、模組概覽

### 截圖功能拆解（對應 screen.png）

```
┌──────────────────────────────────────────────────────────┐
│ Header: BIA Modeler + 導出/報告按鈕                        │
├────────────┬─────────────────────────────────────────────┤
│            │  ┌───────────┐ ┌───────────┐ ┌───────────┐ │
│ Left Nav   │  │ BIA 分級  │ │ 系統分級  │ │ 資產依賴  │ │
│            │  │ 規則      │ │ 概覽      │ │ 清單      │ │
│ • Overview │  └───────────┘ └───────────┘ └───────────┘ │
│ • Grading  │                                             │
│ • RTO/RPO  │  ┌─────────────────────────────────────────┐│
│ • Rules    │  │         BIA 評分矩陣（表格）              ││
│ • Dep Map  │  │  業務系統 | BIA分數 | RTO | RPO | 達標   ││
│            │  └─────────────────────────────────────────┘│
│ [Run]      │                                             │
│ [Logs]     │                                             │
│ [Export]   │                                             │
└────────────┴─────────────────────────────────────────────┘
```

### 新增頁面（5 頁）

| 頁面 | 路由 | 功能 |
|------|------|------|
| BIA Overview | `/bia` | 分級規則 + 系統概覽 + 資產依賴 + 評分矩陣 |
| System Grading | `/bia/grading` | 業務系統列表 + BIA 分數分佈 + 分級統計 |
| RTO/RPO Matrices | `/bia/rto-rpo` | RTO/RPO 目標 vs 實際達成率矩陣 |
| Scoring Rules | `/bia/rules` | 分級規則 CRUD（Tier 1-4 定義）|
| Dependency Map | `/bia/dependencies` | 業務系統 → 資產依賴關係圖 |

---

## 二、後端設計

### 2.1 資料庫表

#### `bia_assessments` — 業務系統 BIA 評估

```sql
CREATE TABLE bia_assessments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    system_name     VARCHAR(255) NOT NULL,
    system_code     VARCHAR(100) NOT NULL,
    owner           VARCHAR(255),
    bia_score       INT          NOT NULL DEFAULT 0,       -- 0-100
    tier            VARCHAR(20)  NOT NULL DEFAULT 'normal', -- critical/important/normal/minor
    rto_hours       NUMERIC(10,2),                         -- Recovery Time Objective
    rpo_minutes     NUMERIC(10,2),                         -- Recovery Point Objective
    mtpd_hours      NUMERIC(10,2),                         -- Max Tolerable Period of Disruption
    data_compliance BOOLEAN      DEFAULT false,
    asset_compliance BOOLEAN     DEFAULT false,
    audit_compliance BOOLEAN     DEFAULT false,
    description     TEXT,
    last_assessed   TIMESTAMPTZ  DEFAULT now(),
    assessed_by     UUID         REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_bia_assessments_tenant ON bia_assessments(tenant_id);
CREATE INDEX idx_bia_assessments_tier ON bia_assessments(tenant_id, tier);
```

#### `bia_scoring_rules` — 分級規則定義

```sql
CREATE TABLE bia_scoring_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    tier_name       VARCHAR(20)  NOT NULL,  -- critical/important/normal/minor
    tier_level      INT          NOT NULL,  -- 1/2/3/4
    display_name    VARCHAR(100) NOT NULL,  -- "Tier 1 - CRITICAL"
    min_score       INT          NOT NULL,  -- 分數下限
    max_score       INT          NOT NULL,  -- 分數上限
    rto_threshold   NUMERIC(10,2),          -- RTO 上限（小時）
    rpo_threshold   NUMERIC(10,2),          -- RPO 上限（分鐘）
    description     TEXT,
    color           VARCHAR(20),            -- UI 顯示顏色 (#ff6b6b)
    icon            VARCHAR(50),            -- Material icon name
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
```

#### `bia_dependencies` — 系統依賴關係

```sql
CREATE TABLE bia_dependencies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    assessment_id   UUID         NOT NULL REFERENCES bia_assessments(id) ON DELETE CASCADE,
    asset_id        UUID         NOT NULL REFERENCES assets(id),
    dependency_type VARCHAR(50)  NOT NULL DEFAULT 'runs_on', -- runs_on/depends_on/backed_by
    criticality     VARCHAR(20)  DEFAULT 'high',             -- high/medium/low
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE(assessment_id, asset_id)
);
CREATE INDEX idx_bia_deps_assessment ON bia_dependencies(assessment_id);
CREATE INDEX idx_bia_deps_asset ON bia_dependencies(asset_id);
```

### 2.2 Migration 文件

**檔案**: `cmdb-core/db/migrations/000013_bia_tables.up.sql`

包含以上 3 張表 + 索引。

**Down migration**: `000013_bia_tables.down.sql`
```sql
DROP TABLE IF EXISTS bia_dependencies;
DROP TABLE IF EXISTS bia_scoring_rules;
DROP TABLE IF EXISTS bia_assessments;
```

### 2.3 Seed 數據

**追加到** `cmdb-core/db/seed/seed.sql`：

```sql
-- BIA Scoring Rules (4 tiers)
INSERT INTO bia_scoring_rules (id, tenant_id, tier_name, tier_level, display_name, min_score, max_score, rto_threshold, rpo_threshold, description, color, icon) VALUES
    ('90000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'critical', 1, 'Tier 1 - CRITICAL', 85, 100, 4, 15,
     '核心支付系統、棟宇監控等，停機即產生重大財務或安全影響', '#ff6b6b', 'error'),
    ('90000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'important', 2, 'Tier 2 - IMPORTANT', 60, 84, 12, 60,
     '核心系統群組（CRM、ERP），停機影響業務運作效率', '#ffa94d', 'warning'),
    ('90000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'normal', 3, 'Tier 3 - NORMAL', 30, 59, 24, 240,
     '一般業務系統，停機可使用替代方案', '#9ecaff', 'info'),
    ('90000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'minor', 4, 'Tier 4 - MINOR', 0, 29, 72, null,
     '測試、沙箱環境，停機無業務衝擊', '#8e9196', 'expand_circle_down')
ON CONFLICT DO NOTHING;

-- BIA Assessments (4 business systems matching screenshot)
INSERT INTO bia_assessments (id, tenant_id, system_name, system_code, owner, bia_score, tier, rto_hours, rpo_minutes, mtpd_hours, data_compliance, asset_compliance, audit_compliance, description, assessed_by) VALUES
    ('91000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     '核心支付閘道器 (Payment Gateway)', 'SYS-PROD-PAY-001', '運大文',
     98, 'critical', 4, 15, 8,
     true, true, true,
     '處理所有線上支付交易，連接銀行API和清算系統',
     'b0000000-0000-0000-0000-000000000001'),
    ('91000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     '客戶關係中心 (CRM Core)', 'SYS-PROD-CRM-001', '林昇',
     85, 'important', 12, 120, 24,
     true, true, false,
     '管理客戶資料、服務歷史和溝通記錄',
     'b0000000-0000-0000-0000-000000000002'),
    ('91000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     '內部管理系統 (Admin Panel)', 'SYS-CORP-ADM-001', '王志',
     62, 'normal', 24, 240, 48,
     true, false, false,
     '員工入職、請假、報銷等內部流程管理',
     'b0000000-0000-0000-0000-000000000001'),
    ('91000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'QA 測試環境 (QA Sandbox)', 'SYS-TEST-QA-001', null,
     15, 'minor', 72, null, null,
     false, false, false,
     'QA 團隊測試用環境',
     'b0000000-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;

-- BIA Dependencies (link business systems to infrastructure assets)
INSERT INTO bia_dependencies (tenant_id, assessment_id, asset_id, dependency_type, criticality) VALUES
    -- Payment Gateway → servers + network
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'runs_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002', 'runs_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000005', 'depends_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003', 'depends_on', 'medium'),
    -- CRM → servers + storage
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000006', 'runs_on', 'high'),
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000004', 'depends_on', 'medium'),
    -- Admin Panel → app server
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000007', 'runs_on', 'low'),
    -- QA Sandbox → dev server
    ('a0000000-0000-0000-0000-000000000001', '91000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000008', 'runs_on', 'low')
ON CONFLICT DO NOTHING;
```

### 2.4 sqlc 查詢

**新建** `cmdb-core/db/queries/bia.sql`：

```sql
-- name: ListBIAAssessments :many
SELECT * FROM bia_assessments
WHERE tenant_id = $1
ORDER BY bia_score DESC
LIMIT $2 OFFSET $3;

-- name: CountBIAAssessments :one
SELECT count(*) FROM bia_assessments WHERE tenant_id = $1;

-- name: GetBIAAssessment :one
SELECT * FROM bia_assessments WHERE id = $1;

-- name: CreateBIAAssessment :one
INSERT INTO bia_assessments (
    tenant_id, system_name, system_code, owner, bia_score, tier,
    rto_hours, rpo_minutes, mtpd_hours,
    data_compliance, asset_compliance, audit_compliance,
    description, assessed_by
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
RETURNING *;

-- name: UpdateBIAAssessment :one
UPDATE bia_assessments SET
    system_name      = COALESCE(sqlc.narg('system_name'), system_name),
    owner            = COALESCE(sqlc.narg('owner'), owner),
    bia_score        = COALESCE(sqlc.narg('bia_score'), bia_score),
    tier             = COALESCE(sqlc.narg('tier'), tier),
    rto_hours        = COALESCE(sqlc.narg('rto_hours'), rto_hours),
    rpo_minutes      = COALESCE(sqlc.narg('rpo_minutes'), rpo_minutes),
    mtpd_hours       = COALESCE(sqlc.narg('mtpd_hours'), mtpd_hours),
    data_compliance  = COALESCE(sqlc.narg('data_compliance'), data_compliance),
    asset_compliance = COALESCE(sqlc.narg('asset_compliance'), asset_compliance),
    audit_compliance = COALESCE(sqlc.narg('audit_compliance'), audit_compliance),
    description      = COALESCE(sqlc.narg('description'), description),
    last_assessed    = now(),
    updated_at       = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteBIAAssessment :exec
DELETE FROM bia_assessments WHERE id = $1;

-- name: ListBIAScoringRules :many
SELECT * FROM bia_scoring_rules
WHERE tenant_id = $1
ORDER BY tier_level;

-- name: CreateBIAScoringRule :one
INSERT INTO bia_scoring_rules (tenant_id, tier_name, tier_level, display_name, min_score, max_score, rto_threshold, rpo_threshold, description, color, icon)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING *;

-- name: UpdateBIAScoringRule :one
UPDATE bia_scoring_rules SET
    display_name   = COALESCE(sqlc.narg('display_name'), display_name),
    min_score      = COALESCE(sqlc.narg('min_score'), min_score),
    max_score      = COALESCE(sqlc.narg('max_score'), max_score),
    rto_threshold  = COALESCE(sqlc.narg('rto_threshold'), rto_threshold),
    rpo_threshold  = COALESCE(sqlc.narg('rpo_threshold'), rpo_threshold),
    description    = COALESCE(sqlc.narg('description'), description),
    color          = COALESCE(sqlc.narg('color'), color)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListBIADependencies :many
SELECT * FROM bia_dependencies
WHERE assessment_id = $1;

-- name: CreateBIADependency :one
INSERT INTO bia_dependencies (tenant_id, assessment_id, asset_id, dependency_type, criticality)
VALUES ($1,$2,$3,$4,$5)
RETURNING *;

-- name: DeleteBIADependency :exec
DELETE FROM bia_dependencies WHERE id = $1;

-- name: CountBIAByTier :many
SELECT tier, count(*) as count FROM bia_assessments
WHERE tenant_id = $1
GROUP BY tier;

-- name: GetBIAComplianceStats :one
SELECT
    count(*) as total,
    count(*) FILTER (WHERE data_compliance = true) as data_compliant,
    count(*) FILTER (WHERE asset_compliance = true) as asset_compliant,
    count(*) FILTER (WHERE audit_compliance = true) as audit_compliant
FROM bia_assessments
WHERE tenant_id = $1;
```

### 2.5 API 端點（OpenAPI）

**新增 tag**:
```yaml
  - name: bia
    description: Business Impact Analysis
```

**新增 10 個端點**:

| 方法 | 路徑 | 操作 |
|------|------|------|
| GET | `/bia/assessments` | 列出 BIA 評估（分頁）|
| POST | `/bia/assessments` | 建立 BIA 評估 |
| GET | `/bia/assessments/{id}` | 評估詳情 |
| PUT | `/bia/assessments/{id}` | 更新評估 |
| DELETE | `/bia/assessments/{id}` | 刪除評估 |
| GET | `/bia/rules` | 列出分級規則 |
| PUT | `/bia/rules/{id}` | 更新分級規則 |
| GET | `/bia/assessments/{id}/dependencies` | 列出系統依賴 |
| POST | `/bia/assessments/{id}/dependencies` | 新增依賴 |
| GET | `/bia/stats` | BIA 統計（分級分佈 + 合規率）|

**新增 schemas**:

```yaml
    BIAAssessment:
      type: object
      properties:
        id: { type: string, format: uuid }
        system_name: { type: string }
        system_code: { type: string }
        owner: { type: string }
        bia_score: { type: integer }
        tier: { type: string }
        rto_hours: { type: number }
        rpo_minutes: { type: number }
        mtpd_hours: { type: number }
        data_compliance: { type: boolean }
        asset_compliance: { type: boolean }
        audit_compliance: { type: boolean }
        description: { type: string }
        last_assessed: { type: string, format: date-time }
        assessed_by: { type: string, format: uuid }
        created_at: { type: string, format: date-time }
      required: [id, system_name, system_code, bia_score, tier]

    BIAScoringRule:
      type: object
      properties:
        id: { type: string, format: uuid }
        tier_name: { type: string }
        tier_level: { type: integer }
        display_name: { type: string }
        min_score: { type: integer }
        max_score: { type: integer }
        rto_threshold: { type: number }
        rpo_threshold: { type: number }
        description: { type: string }
        color: { type: string }
        icon: { type: string }
      required: [id, tier_name, tier_level, display_name, min_score, max_score]

    BIADependency:
      type: object
      properties:
        id: { type: string, format: uuid }
        assessment_id: { type: string, format: uuid }
        asset_id: { type: string, format: uuid }
        dependency_type: { type: string }
        criticality: { type: string }
      required: [id, assessment_id, asset_id, dependency_type]

    BIAStats:
      type: object
      properties:
        total: { type: integer }
        by_tier: { type: object, additionalProperties: { type: integer } }
        avg_compliance: { type: number }
        data_compliant: { type: integer }
        asset_compliant: { type: integer }
        audit_compliant: { type: integer }
```

### 2.6 Go 服務層

**新建** `cmdb-core/internal/domain/bia/service.go`：

```go
package bia

type Service struct {
    queries *dbgen.Queries
}

func NewService(queries *dbgen.Queries) *Service
func (s *Service) ListAssessments(ctx, tenantID, limit, offset) ([]dbgen.BiaAssessment, int64, error)
func (s *Service) GetAssessment(ctx, id) (*dbgen.BiaAssessment, error)
func (s *Service) CreateAssessment(ctx, params) (*dbgen.BiaAssessment, error)
func (s *Service) UpdateAssessment(ctx, params) (*dbgen.BiaAssessment, error)
func (s *Service) DeleteAssessment(ctx, id) error
func (s *Service) ListRules(ctx, tenantID) ([]dbgen.BiaScoringRule, error)
func (s *Service) UpdateRule(ctx, params) (*dbgen.BiaScoringRule, error)
func (s *Service) ListDependencies(ctx, assessmentID) ([]dbgen.BiaDependency, error)
func (s *Service) CreateDependency(ctx, params) (*dbgen.BiaDependency, error)
func (s *Service) DeleteDependency(ctx, id) error
func (s *Service) GetStats(ctx, tenantID) (*BIAStats, error)
```

### 2.7 impl.go 新增 handler + APIServer 接入

- APIServer 加 `biaSvc *bia.Service` 欄位
- NewAPIServer 加 biaSvc 參數
- main.go 建立 `biaSvc := bia.NewService(queries)` 並傳入
- 10 個 handler 方法實現

---

## 三、前端設計

### 3.1 統一使用平台設計系統（非 DESIGN.md Navy 配色）

| 設計元素 | 使用平台現有 token |
|---------|-------------------|
| 背景 | `bg-surface` (#0a151a) |
| 卡片 | `bg-surface-container` (#162127), `rounded-lg p-5` |
| 表格 header | `bg-surface-container-high` (#202b32) |
| 文字 | `text-on-surface` (#d8e4ec) |
| 次要文字 | `text-on-surface-variant` (#c4c6cc) |
| 標題 | `font-headline font-bold` (Manrope) |
| 小標籤 | `text-[0.6875rem] uppercase tracking-wider` |
| 主色調 | `text-primary` (#9ecaff) / `machined-gradient` |
| 危險色 | `text-error` (#ffb4ab) |
| 成功色 | `text-[#34d399]` |

### 3.2 BIA Tier 顏色映射

```typescript
const TIER_COLORS: Record<string, { bg: string; text: string; icon: string }> = {
  critical:  { bg: 'bg-error-container',            text: 'text-on-error-container', icon: 'error' },
  important: { bg: 'bg-[#92400e]',                  text: 'text-[#fbbf24]',         icon: 'warning' },
  normal:    { bg: 'bg-[#1e3a5f]',                  text: 'text-on-primary-container', icon: 'info' },
  minor:     { bg: 'bg-surface-container-highest',  text: 'text-on-surface-variant', icon: 'expand_circle_down' },
}
```

### 3.3 頁面佈局

#### BIA Overview（主頁面，對應截圖）

採用 **Left Nav + Right Content** 佈局（與 RolesPermissions 相同模式）：

```tsx
<div className="grid grid-cols-1 gap-4 lg:grid-cols-[240px_1fr]">
  {/* Left Nav Panel */}
  <div className="space-y-2">
    <NavItem icon="dashboard" label="BIA Overview" active />
    <NavItem icon="grade" label="System Grading" />
    <NavItem icon="timer" label="RTO/RPO Matrices" />
    <NavItem icon="rule" label="Scoring Rules" />
    <NavItem icon="device_hub" label="Dependency Map" />
    <div className="my-4 border-t border-outline-variant/20" />
    <ActionButton icon="play_arrow" label="Run New Analysis" gradient />
    <ActionButton icon="history" label="Audit Logs" />
    <ActionButton icon="download" label="Export" />
  </div>

  {/* Right Content */}
  <div className="space-y-5">
    {/* Row 1: 3 stat cards */}
    <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
      <TierRulesCard />
      <SystemOverviewCard />
      <AssetDependencyCard />
    </div>

    {/* Row 2: Assessment matrix table */}
    <AssessmentMatrixTable />
  </div>
</div>
```

#### TierRulesCard（BIA 自動分級規則）

```tsx
<div className="rounded-lg bg-surface-container p-5">
  <div className="mb-4 flex items-center gap-2">
    <Icon name="tune" className="text-primary text-xl" />
    <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
      BIA 自動分級規則
    </h3>
  </div>
  {rules.map(rule => (
    <div key={rule.id} className="flex items-center gap-3 rounded-lg bg-surface-container-low p-3 mb-2">
      <Icon name={rule.icon} className={TIER_COLORS[rule.tier_name].text} />
      <div className="flex-1">
        <p className="text-sm font-semibold text-on-surface">{rule.display_name}</p>
        <p className="text-xs text-on-surface-variant">{rule.description}</p>
      </div>
    </div>
  ))}
</div>
```

#### SystemOverviewCard（系統分級概覽）

```tsx
<div className="rounded-lg bg-surface-container p-5">
  {/* Donut chart: tier distribution */}
  <DonutChart segments={tierDistribution} />
  <div className="mt-3 text-center">
    <p className="font-headline text-3xl font-bold text-on-surface">{stats.total}</p>
    <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">TOTAL MONITORED</p>
  </div>
  <div className="mt-2 text-center">
    <p className="text-sm text-on-surface-variant">AVG COMPLIANCE</p>
    <p className="font-headline text-xl font-bold text-[#34d399]">{stats.avg_compliance}%</p>
  </div>
</div>
```

#### AssessmentMatrixTable（BIA 評分矩陣）

```tsx
<div className="rounded-lg bg-surface-container overflow-x-auto">
  <div className="flex items-center justify-between p-5 pb-3">
    <div className="flex items-center gap-2">
      <Icon name="assessment" className="text-primary text-xl" />
      <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
        BIA 評分矩陣
      </h3>
    </div>
    <div className="flex items-center gap-2">
      {/* Tier 1 / Tier 2 filter badges */}
    </div>
  </div>
  <table className="w-full">
    <thead>
      <tr className="bg-surface-container-high">
        <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">業務系統</th>
        <th>負責人</th>
        <th>BIA 名稱</th>
        <th>BIA 分數</th>
        <th>RTO (小時)</th>
        <th>RPO (分鐘)</th>
        <th>資產</th>
        <th>數據</th>
        <th>合規</th>
      </tr>
    </thead>
    <tbody>
      {assessments.map(a => (
        <tr key={a.id} className="hover:bg-surface-container-low cursor-pointer transition-colors">
          <td className="px-5 py-3.5">
            <p className="text-sm font-semibold text-on-surface">{a.system_name}</p>
            <p className="text-xs text-on-surface-variant">{a.system_code}</p>
          </td>
          <td className="px-5 py-3.5 text-sm">{a.owner}</td>
          <td className="px-5 py-3.5">
            <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase ${TIER_COLORS[a.tier].bg} ${TIER_COLORS[a.tier].text}`}>
              {a.tier}
            </span>
          </td>
          <td className="px-5 py-3.5">
            <span className="font-headline text-lg font-bold">{a.bia_score}</span>
            <BiaScoreArrow score={a.bia_score} />
          </td>
          <td className="px-5 py-3.5 text-sm">{a.rto_hours}</td>
          <td className="px-5 py-3.5 text-sm">{a.rpo_minutes ?? 'N/A'}</td>
          <td><ComplianceIcon ok={a.asset_compliance} /></td>
          <td><ComplianceIcon ok={a.data_compliance} /></td>
          <td><ComplianceIcon ok={a.audit_compliance} /></td>
        </tr>
      ))}
    </tbody>
  </table>
</div>
```

### 3.4 前端檔案結構

```
cmdb-demo/src/
├── pages/
│   └── bia/
│       ├── BIAOverview.tsx           ← 主頁面（截圖對應）
│       ├── SystemGrading.tsx         ← 系統分級詳情
│       ├── RtoRpoMatrices.tsx        ← RTO/RPO 矩陣
│       ├── ScoringRules.tsx          ← 分級規則管理
│       └── DependencyMap.tsx         ← 依賴關係圖
├── hooks/
│   └── useBIA.ts                    ← React Query hooks
├── lib/api/
│   └── bia.ts                       ← API client
└── components/
    ├── CreateAssessmentModal.tsx     ← 新建評估表單
    └── BIAComplianceIcon.tsx        ← 合規達標圖標（✓/✗）
```

### 3.5 React Query Hooks

```typescript
// hooks/useBIA.ts
export function useBIAAssessments(params?)       // GET /bia/assessments
export function useBIAAssessment(id)             // GET /bia/assessments/{id}
export function useCreateBIAAssessment()         // POST /bia/assessments
export function useUpdateBIAAssessment()         // PUT /bia/assessments/{id}
export function useDeleteBIAAssessment()         // DELETE /bia/assessments/{id}
export function useBIAScoringRules()             // GET /bia/rules
export function useUpdateBIAScoringRule()         // PUT /bia/rules/{id}
export function useBIADependencies(assessmentId) // GET /bia/assessments/{id}/dependencies
export function useCreateBIADependency()         // POST /bia/assessments/{id}/dependencies
export function useBIAStats()                    // GET /bia/stats
```

### 3.6 路由定義

```tsx
// App.tsx 新增
<Route path="/bia" element={<BIAOverview />} />
<Route path="/bia/grading" element={<SystemGrading />} />
<Route path="/bia/rto-rpo" element={<RtoRpoMatrices />} />
<Route path="/bia/rules" element={<ScoringRules />} />
<Route path="/bia/dependencies" element={<DependencyMap />} />
```

### 3.7 側邊欄導航

在 MainLayout.tsx 的導航列表加入 BIA 區塊：

```tsx
{
  section: 'IMPACT ANALYSIS',
  items: [
    { icon: 'assessment', label: 'BIA Modeler', path: '/bia' },
  ]
}
```

---

## 四、執行順序

```
Phase 1: 後端基礎 (1 migration + seed + sqlc + service + 10 endpoints)
  ├── 1a. Migration + seed data
  ├── 1b. sqlc queries + generate
  ├── 1c. Service layer
  ├── 1d. OpenAPI spec + generate
  └── 1e. impl.go handlers + main.go wiring

Phase 2: 前端 BIA Overview 主頁面
  ├── 2a. API client + hooks
  ├── 2b. BIAOverview.tsx (截圖主視圖)
  ├── 2c. CreateAssessmentModal
  ├── 2d. 路由 + 側邊欄導航
  └── 2e. BIAComplianceIcon 組件

Phase 3: 其餘 4 個子頁面
  ├── 3a. SystemGrading.tsx
  ├── 3b. RtoRpoMatrices.tsx
  ├── 3c. ScoringRules.tsx (規則 CRUD)
  └── 3d. DependencyMap.tsx

Phase 4: 與現有模組整合
  ├── 4a. Dashboard 加 BIA 統計卡
  ├── 4b. 告警嚴重度根據 BIA tier 自動升級
  └── 4c. 工單建立時自動設定 priority 依據 BIA
```

---

## 五、改動量估計

| Phase | 新檔案 | 改動檔案 | 新增行數 |
|-------|--------|---------|---------|
| Phase 1 (後端) | 4 (migration, queries, service, seed) | 4 (openapi, generated, impl, main) | ~600 |
| Phase 2 (主頁面) | 5 (overview, hooks, api, modal, icon) | 2 (App.tsx, MainLayout) | ~800 |
| Phase 3 (子頁面) | 4 (grading, rto, rules, depmap) | 0 | ~700 |
| Phase 4 (整合) | 0 | 3 (Dashboard, impl, alert logic) | ~100 |
| **合計** | **13** | **9** | **~2,200** |
