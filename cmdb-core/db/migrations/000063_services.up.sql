-- ============================================================================
-- 000063: Business Services (Wave 2 — M1 Service entity)
-- ============================================================================
--
-- Spec: db/specs/services.md (Approved 2026-04-23)
-- Decision log: docs/decisions/2026-04-22-day-0.md §D1
-- Business review: docs/reviews/2026-04-22-business-fit-review.md #1
--
-- Creates the first-class Business Service entity and its N:M relationship
-- with assets. Backfills services from existing bia_assessments so the BIA
-- data starts flowing through the new entity immediately.
--
-- Preserves bia_assessments unchanged — only adds an optional service_id
-- FK so legacy BIA queries keep working while new code reads via service.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. services — user-visible business functions
-- ---------------------------------------------------------------------------
CREATE TABLE services (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),

    -- Business identity. Human-readable reference ID; unique per tenant.
    code              VARCHAR(64) NOT NULL,
    name              VARCHAR(255) NOT NULL,
    description       TEXT,

    -- Tier reuses BIA tier_name vocabulary so BIA scoring + services align.
    tier              VARCHAR(20) NOT NULL DEFAULT 'normal',

    -- Ownership. Placeholder VARCHAR until M3 Wave 7 introduces the teams
    -- entity and migrates this to a FK column.
    owner_team        VARCHAR(100),

    -- Optional link to a BIA assessment record (0..1). Deleting the
    -- assessment sets this to NULL rather than cascading — the service
    -- survives, we just lose the RTO/RPO linkage.
    bia_assessment_id UUID REFERENCES bia_assessments(id) ON DELETE SET NULL,

    -- Lifecycle.
    --   active         = in use
    --   deprecated     = still running but no new investment
    --   decommissioned = retired; service_assets preserved per Q2 sign-off
    status            VARCHAR(20) NOT NULL DEFAULT 'active',

    -- Free-form tags mirror the assets.tags pattern.
    tags              TEXT[] NOT NULL DEFAULT '{}',

    -- Metadata.
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID REFERENCES users(id),
    deleted_at        TIMESTAMPTZ,
    sync_version      BIGINT NOT NULL DEFAULT 0,

    UNIQUE (tenant_id, code),
    CHECK (status IN ('active', 'deprecated', 'decommissioned')),
    CHECK (tier IN ('critical', 'important', 'normal', 'low', 'minor')),
    -- Q1 sign-off: code must look like a k8s-safe business ID.
    CHECK (code ~ '^[A-Z][A-Z0-9_-]{1,63}$')
);

CREATE INDEX idx_services_tenant
    ON services(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_tier
    ON services(tenant_id, tier) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_owner_team
    ON services(tenant_id, owner_team)
    WHERE owner_team IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_services_status
    ON services(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_sync_version
    ON services(tenant_id, sync_version);

-- ---------------------------------------------------------------------------
-- 2. service_assets — N:M between services and assets
-- ---------------------------------------------------------------------------
-- Membership of an asset in a service. Critical-path flag distinguishes
-- "service dies if this asset dies" from "one of many redundant components".
CREATE TABLE service_assets (
    service_id    UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    asset_id      UUID NOT NULL REFERENCES assets(id)   ON DELETE CASCADE,
    -- tenant_id is denormalized from services/assets so tenantlint +
    -- per-tenant indexes still work on this relation table.
    tenant_id     UUID NOT NULL REFERENCES tenants(id),

    -- role describes what the asset does inside the service. The 7-value
    -- enum is deliberately small; Q3 sign-off: load_balancer → proxy,
    -- database → primary or storage, firewall → dependency.
    role          VARCHAR(50) NOT NULL DEFAULT 'component',

    -- is_critical drives service health computation:
    --   health = all(is_critical=true assets are healthy)
    is_critical   BOOLEAN NOT NULL DEFAULT false,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    UUID REFERENCES users(id),

    PRIMARY KEY (service_id, asset_id),
    CHECK (role IN ('primary', 'replica', 'cache', 'proxy', 'storage', 'dependency', 'component'))
);

CREATE INDEX idx_service_assets_asset
    ON service_assets(asset_id);
CREATE INDEX idx_service_assets_tenant
    ON service_assets(tenant_id);
CREATE INDEX idx_service_assets_critical
    ON service_assets(service_id) WHERE is_critical = true;

-- ---------------------------------------------------------------------------
-- 3. bia_assessments.service_id reverse FK
-- ---------------------------------------------------------------------------
-- Non-destructive: existing BIA queries by system_code still work. New
-- code prefers joining through services.id. Deleting a service sets
-- bia_assessments.service_id to NULL so assessment history survives.
ALTER TABLE bia_assessments
    ADD COLUMN IF NOT EXISTS service_id UUID REFERENCES services(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_bia_assessments_service
    ON bia_assessments(service_id) WHERE service_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 4. Backfill services from bia_assessments
-- ---------------------------------------------------------------------------
-- Q4 sign-off: DISTINCT ON picks the latest assessment per (tenant, code),
-- which is the one whose tier/RTO/RPO reflects current business intent.
--
-- bia_assessments.system_code is VARCHAR(100). Rows where the code does
-- not match our CHECK regex (lowercase, spaces, Unicode) are skipped
-- rather than coerced — operators can manually create services for those
-- after normalizing the source code.
--
-- ON CONFLICT (tenant_id, code) DO NOTHING means re-runs are safe.
INSERT INTO services (tenant_id, code, name, tier, bia_assessment_id, created_at, updated_at)
SELECT DISTINCT ON (b.tenant_id, b.system_code)
    b.tenant_id,
    b.system_code,
    b.system_name,
    b.tier,
    b.id,
    b.created_at,
    b.updated_at
FROM bia_assessments b
WHERE b.system_code ~ '^[A-Z][A-Z0-9_-]{1,63}$'
ORDER BY b.tenant_id, b.system_code, b.last_assessed DESC NULLS LAST, b.id
ON CONFLICT (tenant_id, code) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 5. Reverse-fill bia_assessments.service_id
-- ---------------------------------------------------------------------------
-- For every assessment whose system_code now maps to a service, point the
-- reverse FK at it. Multi-version assessments all point to the single
-- (tenant, code) service row created above.
UPDATE bia_assessments b
SET service_id = s.id
FROM services s
WHERE s.tenant_id = b.tenant_id
  AND s.code = b.system_code
  AND b.service_id IS NULL;

COMMIT;
