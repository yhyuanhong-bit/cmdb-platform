# Ingestion Engine Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Python ingestion engine with FastAPI management API, Excel/CSV import pipeline, transform pipeline with authority-based conflict resolution, and collector framework — enabling manual and semi-automatic data import into the CMDB.

**Architecture:** Standalone Python service communicating with cmdb-core (Go) via direct PostgreSQL access for reads and NATS for event publishing. FastAPI serves the management API (upload, preview, confirm, conflict resolution). Celery handles async import processing. The transform pipeline (normalize → deduplicate → validate → authority check → merge/conflict) is the central processing unit shared by all import modes.

**Tech Stack:** Python 3.12+, FastAPI 0.115+, Celery 5.4+, asyncpg, nats-py, openpyxl, pydantic 2.x, Redis (broker), pytest

**Spec Reference:** `docs/superpowers/specs/2026-04-03-cmdb-backend-techstack-design.md` — Section 4

**This plan covers:**
- Transform pipeline (normalize, deduplicate, validate, authority check, conflict resolution)
- Manual mode: Excel/CSV upload, parse, preview, confirm, async processing
- Management API: 17 FastAPI endpoints for import/conflict/collector management
- Collector framework: base protocol + collector manager (stubs for IPMI/SNMP — actual collectors in a future plan)
- DB migrations for ingestion-specific tables (import_conflicts, discovery_tasks, discovery_candidates, import_jobs, asset_field_authorities)
- NATS event publishing (asset.updated, import.completed)

**Out of scope (Plan 2b):** Actual IPMI/SNMP/vCenter/K8s collector implementations, Prometheus bridge, APScheduler cron integration.

---

## File Structure

```
ingestion-engine/
├── pyproject.toml
├── Dockerfile
├── app/
│   ├── __init__.py
│   ├── main.py                              # FastAPI app factory + lifespan
│   ├── config.py                            # Settings from env vars (pydantic-settings)
│   ├── database.py                          # asyncpg pool management
│   ├── events.py                            # NATS client wrapper (publish only)
│   ├── dependencies.py                      # FastAPI dependency injection
│   │
│   ├── models/                              # Pydantic models (shared)
│   │   ├── __init__.py
│   │   ├── common.py                        # RawAssetData, FieldMapping, CollectTarget
│   │   ├── import_job.py                    # ImportJob, ImportRow, ParseResult
│   │   └── conflict.py                      # ImportConflict, ConflictResolution
│   │
│   ├── pipeline/                            # Transform pipeline
│   │   ├── __init__.py
│   │   ├── normalize.py                     # Field mapping + type conversion
│   │   ├── deduplicate.py                   # Match by serial_number / asset_tag
│   │   ├── validate.py                      # Required fields, format, referential
│   │   ├── authority.py                     # Authority check + auto-merge logic
│   │   └── processor.py                     # Orchestrates full pipeline
│   │
│   ├── importers/                           # Manual import handlers
│   │   ├── __init__.py
│   │   ├── excel_parser.py                  # Parse Excel/CSV into RawAssetData
│   │   └── templates.py                     # Generate download templates
│   │
│   ├── collectors/                          # Collector framework
│   │   ├── __init__.py
│   │   ├── base.py                          # CollectorProtocol + CollectorRegistry
│   │   └── manager.py                       # CollectorManager (start/stop/status)
│   │
│   ├── routes/                              # FastAPI routers
│   │   ├── __init__.py
│   │   ├── imports.py                       # /ingestion/import/* endpoints
│   │   ├── conflicts.py                     # /ingestion/conflicts/* endpoints
│   │   └── collectors.py                    # /ingestion/collectors/* endpoints
│   │
│   └── tasks/                               # Celery tasks
│       ├── __init__.py
│       ├── celery_app.py                    # Celery app config
│       └── import_task.py                   # Async import processing task
│
├── db/
│   └── migrations/
│       ├── 000010_ingestion_tables.up.sql   # import_conflicts, discovery_*, import_jobs, asset_field_authorities
│       └── 000010_ingestion_tables.down.sql
│
├── tests/
│   ├── __init__.py
│   ├── conftest.py                          # Fixtures: test DB, async client
│   ├── test_normalize.py
│   ├── test_deduplicate.py
│   ├── test_validate.py
│   ├── test_authority.py
│   ├── test_processor.py
│   ├── test_excel_parser.py
│   └── test_import_routes.py
│
└── templates/
    └── asset_import_template.xlsx           # Pre-built Excel template
```

---

## Task 1: Project Scaffold + Dependencies

**Files:**
- Create: `ingestion-engine/pyproject.toml`
- Create: `ingestion-engine/Dockerfile`
- Create: `ingestion-engine/app/__init__.py`
- Create: `ingestion-engine/app/config.py`
- Create: `ingestion-engine/app/main.py`

- [ ] **Step 1: Create pyproject.toml**

Create `ingestion-engine/pyproject.toml`:

```toml
[project]
name = "cmdb-ingestion-engine"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
    "fastapi>=0.115.0",
    "uvicorn[standard]>=0.32.0",
    "asyncpg>=0.30.0",
    "pydantic>=2.10.0",
    "pydantic-settings>=2.7.0",
    "nats-py>=2.9.0",
    "openpyxl>=3.1.0",
    "celery[redis]>=5.4.0",
    "redis>=5.2.0",
    "python-multipart>=0.0.18",
    "bcrypt>=4.2.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=8.3.0",
    "pytest-asyncio>=0.24.0",
    "httpx>=0.28.0",
    "ruff>=0.8.0",
]

[tool.pytest.ini_options]
asyncio_mode = "auto"
testpaths = ["tests"]

[tool.ruff]
target-version = "py312"
line-length = 120
```

- [ ] **Step 2: Create config.py**

Create `ingestion-engine/app/__init__.py` (empty file).

Create `ingestion-engine/app/config.py`:

```python
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    database_url: str = "postgresql://cmdb:cmdb_secret@localhost:5432/cmdb"
    redis_url: str = "redis://localhost:6379/1"
    nats_url: str = "nats://localhost:4222"
    celery_broker_url: str = "redis://localhost:6379/2"
    upload_dir: str = "/tmp/cmdb-uploads"
    max_upload_size_mb: int = 50
    deploy_mode: str = "central"
    tenant_id: str = ""

    model_config = {"env_prefix": "INGESTION_"}


settings = Settings()
```

- [ ] **Step 3: Create FastAPI app with lifespan**

Create `ingestion-engine/app/main.py`:

```python
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.config import settings
from app.database import create_pool, close_pool
from app.events import connect_nats, close_nats


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    app.state.db_pool = await create_pool(settings.database_url)
    app.state.nats_client = await connect_nats(settings.nats_url)
    yield
    # Shutdown
    await close_nats(app.state.nats_client)
    await close_pool(app.state.db_pool)


def create_app() -> FastAPI:
    app = FastAPI(
        title="CMDB Ingestion Engine",
        version="0.1.0",
        lifespan=lifespan,
    )

    from app.routes.imports import router as imports_router
    from app.routes.conflicts import router as conflicts_router
    from app.routes.collectors import router as collectors_router

    app.include_router(imports_router, prefix="/ingestion")
    app.include_router(conflicts_router, prefix="/ingestion")
    app.include_router(collectors_router, prefix="/ingestion")

    @app.get("/healthz")
    async def healthz():
        return {"status": "ok"}

    return app


app = create_app()
```

- [ ] **Step 4: Create Dockerfile**

Create `ingestion-engine/Dockerfile`:

```dockerfile
FROM python:3.12-slim

WORKDIR /app
COPY pyproject.toml .
RUN pip install --no-cache-dir .
COPY . .
EXPOSE 8081
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8081"]
```

- [ ] **Step 5: Create stub route files so app imports don't fail**

Create `ingestion-engine/app/routes/__init__.py` (empty).
Create `ingestion-engine/app/routes/imports.py`:

```python
from fastapi import APIRouter

router = APIRouter(tags=["imports"])
```

Create `ingestion-engine/app/routes/conflicts.py`:

```python
from fastapi import APIRouter

router = APIRouter(tags=["conflicts"])
```

Create `ingestion-engine/app/routes/collectors.py`:

```python
from fastapi import APIRouter

router = APIRouter(tags=["collectors"])
```

- [ ] **Step 6: Create database.py and events.py stubs**

Create `ingestion-engine/app/database.py`:

```python
import asyncpg


async def create_pool(database_url: str) -> asyncpg.Pool:
    return await asyncpg.create_pool(
        database_url,
        min_size=5,
        max_size=20,
    )


async def close_pool(pool: asyncpg.Pool | None):
    if pool:
        await pool.close()
```

Create `ingestion-engine/app/events.py`:

```python
import json
import nats
from nats.aio.client import Client as NATSClient


async def connect_nats(url: str) -> NATSClient | None:
    try:
        nc = await nats.connect(url)
        return nc
    except Exception as e:
        print(f"WARN: NATS not available: {e}")
        return None


async def close_nats(nc: NATSClient | None):
    if nc:
        await nc.close()


async def publish_event(nc: NATSClient | None, subject: str, tenant_id: str, payload: dict):
    if nc is None:
        return
    full_subject = f"{subject}.{tenant_id}" if tenant_id else subject
    await nc.publish(full_subject, json.dumps(payload).encode())
```

- [ ] **Step 7: Install dependencies and verify**

```bash
cd /cmdb-platform/ingestion-engine
pip install -e ".[dev]"
python -c "from app.main import app; print('OK')"
```

Expected: "OK" printed, no import errors.

- [ ] **Step 8: Commit**

```bash
git add ingestion-engine/
git commit -m "feat: scaffold ingestion-engine with FastAPI, config, DB pool, NATS client"
```

---

## Task 2: Database Migration for Ingestion Tables

**Files:**
- Create: `ingestion-engine/db/migrations/000010_ingestion_tables.up.sql`
- Create: `ingestion-engine/db/migrations/000010_ingestion_tables.down.sql`

- [ ] **Step 1: Create migration up**

Create `ingestion-engine/db/migrations/000010_ingestion_tables.up.sql`:

```sql
-- Asset field authority source configuration
CREATE TABLE IF NOT EXISTS asset_field_authorities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    field_name  VARCHAR(50) NOT NULL,
    source_type VARCHAR(30) NOT NULL,
    priority    INT NOT NULL DEFAULT 0,
    UNIQUE (tenant_id, field_name, source_type)
);

-- Import conflict approval queue
CREATE TABLE IF NOT EXISTS import_conflicts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    asset_id        UUID NOT NULL REFERENCES assets(id),
    source_type     VARCHAR(30) NOT NULL,
    field_name      VARCHAR(50) NOT NULL,
    current_value   TEXT,
    incoming_value  TEXT,
    status          VARCHAR(20) DEFAULT 'pending',
    resolved_by     UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_conflicts_pending ON import_conflicts (tenant_id, status)
    WHERE status = 'pending';

-- Discovery tasks (semi-auto scans)
CREATE TABLE IF NOT EXISTS discovery_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    type            VARCHAR(30) NOT NULL,
    status          VARCHAR(20) DEFAULT 'running',
    config          JSONB NOT NULL,
    stats           JSONB DEFAULT '{}',
    triggered_by    UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS discovery_candidates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES discovery_tasks(id) ON DELETE CASCADE,
    raw_data        JSONB NOT NULL,
    matched_asset_id UUID REFERENCES assets(id),
    status          VARCHAR(20) DEFAULT 'pending',
    reviewed_by     UUID REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ
);

-- Import jobs (Excel/CSV batch import tracking)
CREATE TABLE IF NOT EXISTS import_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    type            VARCHAR(20) NOT NULL,
    filename        VARCHAR(200),
    status          VARCHAR(20) DEFAULT 'parsing',
    total_rows      INT,
    processed_rows  INT DEFAULT 0,
    stats           JSONB DEFAULT '{}',
    error_details   JSONB DEFAULT '[]',
    uploaded_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_tenant ON import_jobs (tenant_id, status);

-- Seed default field authorities for the tw tenant
INSERT INTO asset_field_authorities (tenant_id, field_name, source_type, priority) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'ipmi', 100),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'snmp', 80),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'vendor', 'ipmi', 100),
    ('a0000000-0000-0000-0000-000000000001', 'vendor', 'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'model', 'ipmi', 100),
    ('a0000000-0000-0000-0000-000000000001', 'model', 'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'name', 'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'status', 'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'bia_level', 'manual', 100)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Create migration down**

Create `ingestion-engine/db/migrations/000010_ingestion_tables.down.sql`:

```sql
DELETE FROM asset_field_authorities WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001';
DROP TABLE IF EXISTS import_jobs;
DROP TABLE IF EXISTS discovery_candidates;
DROP TABLE IF EXISTS discovery_tasks;
DROP TABLE IF EXISTS import_conflicts;
DROP TABLE IF EXISTS asset_field_authorities;
```

- [ ] **Step 3: Run migration**

```bash
cd /cmdb-platform/cmdb-core
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" \
  go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -database "$DATABASE_URL" -path db/migrations up
# Then apply ingestion migration
psql "postgres://cmdb:cmdb_secret@localhost:5432/cmdb" \
  -f /cmdb-platform/ingestion-engine/db/migrations/000010_ingestion_tables.up.sql
```

Expected: Tables created successfully.

- [ ] **Step 4: Commit**

```bash
git add ingestion-engine/db/
git commit -m "feat: add ingestion tables migration - conflicts, discovery, import_jobs, field_authorities"
```

---

## Task 3: Pydantic Models (Shared Data Types)

**Files:**
- Create: `ingestion-engine/app/models/__init__.py`
- Create: `ingestion-engine/app/models/common.py`
- Create: `ingestion-engine/app/models/import_job.py`
- Create: `ingestion-engine/app/models/conflict.py`

- [ ] **Step 1: Create common models**

Create `ingestion-engine/app/models/__init__.py` (empty).

Create `ingestion-engine/app/models/common.py`:

```python
from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class RawAssetData(BaseModel):
    """A single asset record from any source (collector, Excel, API)."""
    source: str                                  # "ipmi", "snmp", "excel", "manual"
    unique_key: str                              # serial_number or asset_tag for matching
    fields: dict[str, str | None]                # flat key-value pairs
    attributes: dict[str, any] | None = None     # flexible JSONB fields
    collected_at: datetime | None = None


class FieldMapping(BaseModel):
    """Declares which fields a source can provide and whether it's authoritative."""
    field_name: str
    authority: bool = False


class CollectTarget(BaseModel):
    """Connection target for a collector."""
    endpoint: str
    credentials: dict[str, str] | None = None
    options: dict[str, any] | None = None


class ConnectionResult(BaseModel):
    success: bool
    message: str = ""
    latency_ms: float | None = None


class PipelineResult(BaseModel):
    """Result of processing one RawAssetData through the transform pipeline."""
    asset_id: UUID | None = None
    action: str                                  # "created", "updated", "skipped", "conflict"
    conflicts: list[dict] | None = None          # conflict details if action == "conflict"
    errors: list[str] | None = None              # validation errors
```

- [ ] **Step 2: Create import job models**

Create `ingestion-engine/app/models/import_job.py`:

```python
from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class ImportJobCreate(BaseModel):
    tenant_id: UUID
    type: str                                    # "excel" | "csv"
    filename: str
    uploaded_by: UUID | None = None


class ImportJob(BaseModel):
    id: UUID
    tenant_id: UUID
    type: str
    filename: str | None
    status: str
    total_rows: int | None
    processed_rows: int
    stats: dict
    error_details: list[dict]
    uploaded_by: UUID | None
    created_at: datetime
    completed_at: datetime | None


class ParsedRow(BaseModel):
    row_num: int
    data: dict[str, str | None]
    errors: list[str] | None = None


class ParseResult(BaseModel):
    total_rows: int
    valid_rows: list[ParsedRow]
    error_rows: list[ParsedRow]
    preview: list[ParsedRow]                     # first 20 valid rows
```

- [ ] **Step 3: Create conflict models**

Create `ingestion-engine/app/models/conflict.py`:

```python
from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class ImportConflict(BaseModel):
    id: UUID
    tenant_id: UUID
    asset_id: UUID
    source_type: str
    field_name: str
    current_value: str | None
    incoming_value: str | None
    status: str
    resolved_by: UUID | None
    resolved_at: datetime | None
    created_at: datetime


class ConflictResolution(BaseModel):
    action: str                                  # "approve" | "reject"
    resolved_by: UUID


class BatchConflictResolution(BaseModel):
    conflict_ids: list[UUID]
    action: str                                  # "approve" | "reject"
    resolved_by: UUID
```

- [ ] **Step 4: Verify imports**

```bash
cd /cmdb-platform/ingestion-engine
python -c "from app.models.common import RawAssetData, PipelineResult; from app.models.import_job import ImportJob, ParseResult; from app.models.conflict import ImportConflict; print('OK')"
```

Expected: "OK"

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/models/
git commit -m "feat: add pydantic models - RawAssetData, ImportJob, Conflict, PipelineResult"
```

---

## Task 4: Transform Pipeline — Normalize + Deduplicate + Validate

**Files:**
- Create: `ingestion-engine/app/pipeline/__init__.py`
- Create: `ingestion-engine/app/pipeline/normalize.py`
- Create: `ingestion-engine/app/pipeline/deduplicate.py`
- Create: `ingestion-engine/app/pipeline/validate.py`
- Create: `ingestion-engine/tests/__init__.py`
- Create: `ingestion-engine/tests/conftest.py`
- Create: `ingestion-engine/tests/test_normalize.py`
- Create: `ingestion-engine/tests/test_deduplicate.py`
- Create: `ingestion-engine/tests/test_validate.py`

- [ ] **Step 1: Create normalize.py**

Create `ingestion-engine/app/pipeline/__init__.py` (empty).

Create `ingestion-engine/app/pipeline/normalize.py`:

```python
from app.models.common import RawAssetData

# Map common source-specific field names to canonical asset field names
FIELD_ALIASES: dict[str, str] = {
    "serial": "serial_number",
    "sn": "serial_number",
    "serialnumber": "serial_number",
    "hostname": "name",
    "host_name": "name",
    "device_name": "name",
    "manufacturer": "vendor",
    "mfg": "vendor",
    "device_type": "type",
    "asset_type": "type",
    "sub_type": "sub_type",
    "subtype": "sub_type",
    "tag": "asset_tag",
    "asset_number": "asset_tag",
    "bia": "bia_level",
}

VALID_ASSET_FIELDS = {
    "asset_tag", "name", "type", "sub_type", "status", "bia_level",
    "vendor", "model", "serial_number", "property_number", "control_number",
}


def normalize(raw: RawAssetData) -> RawAssetData:
    """Normalize field names and strip whitespace from values."""
    normalized_fields: dict[str, str | None] = {}

    for key, value in raw.fields.items():
        # Lowercase key, strip whitespace
        clean_key = key.strip().lower().replace(" ", "_")

        # Resolve aliases
        canonical = FIELD_ALIASES.get(clean_key, clean_key)

        # Only keep known fields (extras go to attributes)
        if canonical in VALID_ASSET_FIELDS:
            normalized_fields[canonical] = value.strip() if isinstance(value, str) else value
        else:
            # Unknown fields go to attributes
            if raw.attributes is None:
                raw.attributes = {}
            raw.attributes[clean_key] = value

    return RawAssetData(
        source=raw.source,
        unique_key=raw.unique_key.strip() if raw.unique_key else raw.unique_key,
        fields=normalized_fields,
        attributes=raw.attributes,
        collected_at=raw.collected_at,
    )
```

- [ ] **Step 2: Create test_normalize.py**

Create `ingestion-engine/tests/__init__.py` (empty).

Create `ingestion-engine/tests/conftest.py`:

```python
# Shared fixtures will go here when we add DB tests
```

Create `ingestion-engine/tests/test_normalize.py`:

```python
from app.models.common import RawAssetData
from app.pipeline.normalize import normalize


def test_normalize_aliases():
    raw = RawAssetData(
        source="excel",
        unique_key="SN-001",
        fields={"Serial": "SN-001", "Manufacturer": "Dell", "hostname": "srv-01"},
    )
    result = normalize(raw)
    assert result.fields["serial_number"] == "SN-001"
    assert result.fields["vendor"] == "Dell"
    assert result.fields["name"] == "srv-01"


def test_normalize_strips_whitespace():
    raw = RawAssetData(
        source="excel",
        unique_key=" SN-002 ",
        fields={"serial_number": "  SN-002  ", "vendor": " HP "},
    )
    result = normalize(raw)
    assert result.unique_key == "SN-002"
    assert result.fields["serial_number"] == "SN-002"
    assert result.fields["vendor"] == "HP"


def test_normalize_unknown_fields_to_attributes():
    raw = RawAssetData(
        source="ipmi",
        unique_key="SN-003",
        fields={"serial_number": "SN-003", "bios_version": "2.1.0"},
    )
    result = normalize(raw)
    assert "serial_number" in result.fields
    assert "bios_version" not in result.fields
    assert result.attributes["bios_version"] == "2.1.0"
```

- [ ] **Step 3: Run normalize tests**

```bash
cd /cmdb-platform/ingestion-engine
pytest tests/test_normalize.py -v
```

Expected: 3 tests pass.

- [ ] **Step 4: Create deduplicate.py**

Create `ingestion-engine/app/pipeline/deduplicate.py`:

```python
from uuid import UUID

import asyncpg

from app.models.common import RawAssetData


class DeduplicateResult:
    def __init__(self, existing_asset_id: UUID | None, existing_fields: dict | None):
        self.existing_asset_id = existing_asset_id
        self.existing_fields = existing_fields

    @property
    def is_new(self) -> bool:
        return self.existing_asset_id is None


async def deduplicate(pool: asyncpg.Pool, tenant_id: UUID, raw: RawAssetData) -> DeduplicateResult:
    """Try to match raw data against existing assets by serial_number or asset_tag."""
    async with pool.acquire() as conn:
        # Try serial_number first (most reliable)
        serial = raw.fields.get("serial_number") or raw.unique_key
        if serial:
            row = await conn.fetchrow(
                "SELECT id, asset_tag, name, type, sub_type, status, bia_level, "
                "vendor, model, serial_number, property_number, control_number "
                "FROM assets WHERE tenant_id = $1 AND serial_number = $2",
                tenant_id, serial,
            )
            if row:
                return DeduplicateResult(
                    existing_asset_id=row["id"],
                    existing_fields=dict(row),
                )

        # Try asset_tag
        tag = raw.fields.get("asset_tag")
        if tag:
            row = await conn.fetchrow(
                "SELECT id, asset_tag, name, type, sub_type, status, bia_level, "
                "vendor, model, serial_number, property_number, control_number "
                "FROM assets WHERE tenant_id = $1 AND asset_tag = $2",
                tenant_id, tag,
            )
            if row:
                return DeduplicateResult(
                    existing_asset_id=row["id"],
                    existing_fields=dict(row),
                )

    return DeduplicateResult(existing_asset_id=None, existing_fields=None)
```

- [ ] **Step 5: Create validate.py**

Create `ingestion-engine/app/pipeline/validate.py`:

```python
from app.models.common import RawAssetData

REQUIRED_FIELDS_FOR_CREATE = {"asset_tag", "name", "type"}

VALID_TYPES = {"server", "network", "storage", "power"}
VALID_STATUSES = {"inventoried", "deployed", "operational", "maintenance", "decommissioned"}
VALID_BIA_LEVELS = {"critical", "important", "normal", "minor"}


def validate_for_create(raw: RawAssetData) -> list[str]:
    """Validate that a new asset has all required fields and valid values."""
    errors = []
    fields = raw.fields

    for f in REQUIRED_FIELDS_FOR_CREATE:
        if not fields.get(f):
            errors.append(f"Missing required field: {f}")

    if fields.get("type") and fields["type"] not in VALID_TYPES:
        errors.append(f"Invalid type '{fields['type']}'. Must be one of: {', '.join(VALID_TYPES)}")

    if fields.get("status") and fields["status"] not in VALID_STATUSES:
        errors.append(f"Invalid status '{fields['status']}'. Must be one of: {', '.join(VALID_STATUSES)}")

    if fields.get("bia_level") and fields["bia_level"] not in VALID_BIA_LEVELS:
        errors.append(f"Invalid bia_level '{fields['bia_level']}'. Must be one of: {', '.join(VALID_BIA_LEVELS)}")

    return errors


def validate_for_update(raw: RawAssetData) -> list[str]:
    """Validate fields for updating an existing asset. Less strict — only checks format."""
    errors = []
    fields = raw.fields

    if fields.get("type") and fields["type"] not in VALID_TYPES:
        errors.append(f"Invalid type '{fields['type']}'")

    if fields.get("status") and fields["status"] not in VALID_STATUSES:
        errors.append(f"Invalid status '{fields['status']}'")

    if fields.get("bia_level") and fields["bia_level"] not in VALID_BIA_LEVELS:
        errors.append(f"Invalid bia_level '{fields['bia_level']}'")

    return errors
```

- [ ] **Step 6: Create test_validate.py**

Create `ingestion-engine/tests/test_validate.py`:

```python
from app.models.common import RawAssetData
from app.pipeline.validate import validate_for_create, validate_for_update


def test_validate_create_missing_required():
    raw = RawAssetData(source="excel", unique_key="SN-001", fields={"serial_number": "SN-001"})
    errors = validate_for_create(raw)
    assert any("asset_tag" in e for e in errors)
    assert any("name" in e for e in errors)
    assert any("type" in e for e in errors)


def test_validate_create_valid():
    raw = RawAssetData(
        source="excel", unique_key="SN-001",
        fields={"asset_tag": "SRV-001", "name": "Server 1", "type": "server", "serial_number": "SN-001"},
    )
    errors = validate_for_create(raw)
    assert errors == []


def test_validate_create_invalid_type():
    raw = RawAssetData(
        source="excel", unique_key="SN-001",
        fields={"asset_tag": "SRV-001", "name": "Server 1", "type": "invalid_type"},
    )
    errors = validate_for_create(raw)
    assert any("Invalid type" in e for e in errors)


def test_validate_update_allows_partial():
    raw = RawAssetData(source="ipmi", unique_key="SN-001", fields={"vendor": "Dell"})
    errors = validate_for_update(raw)
    assert errors == []


def test_validate_update_rejects_invalid_status():
    raw = RawAssetData(source="ipmi", unique_key="SN-001", fields={"status": "broken"})
    errors = validate_for_update(raw)
    assert any("Invalid status" in e for e in errors)
```

- [ ] **Step 7: Run all pipeline tests**

```bash
cd /cmdb-platform/ingestion-engine
pytest tests/test_normalize.py tests/test_validate.py -v
```

Expected: 8 tests pass.

- [ ] **Step 8: Commit**

```bash
git add ingestion-engine/app/pipeline/ ingestion-engine/tests/
git commit -m "feat: add transform pipeline - normalize, deduplicate, validate with tests"
```

---

## Task 5: Authority Check + Conflict Resolution

**Files:**
- Create: `ingestion-engine/app/pipeline/authority.py`
- Create: `ingestion-engine/tests/test_authority.py`

- [ ] **Step 1: Create authority.py**

Create `ingestion-engine/app/pipeline/authority.py`:

```python
from uuid import UUID

import asyncpg


class AuthorityResult:
    def __init__(self):
        self.auto_merge_fields: dict[str, str | None] = {}   # field_name -> new_value
        self.conflict_fields: list[dict] = []                  # fields needing human review
        self.skipped_fields: list[str] = []                    # unchanged values


async def check_authority(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    asset_id: UUID,
    source_type: str,
    incoming_fields: dict[str, str | None],
    existing_fields: dict[str, str | None],
) -> AuthorityResult:
    """Check field authority and determine merge/conflict for each changed field."""
    result = AuthorityResult()

    # Load authority config for this tenant
    authorities = await _load_authorities(pool, tenant_id)

    for field_name, incoming_value in incoming_fields.items():
        existing_value = existing_fields.get(field_name)

        # If values are the same, skip
        if str(incoming_value or "") == str(existing_value or ""):
            result.skipped_fields.append(field_name)
            continue

        # Check if this source is authoritative for this field
        source_priority = authorities.get((field_name, source_type), 0)
        max_priority = _get_max_priority(authorities, field_name)

        if source_priority >= max_priority and source_priority > 0:
            # This source IS the authority — auto-merge
            result.auto_merge_fields[field_name] = incoming_value
        else:
            # Not authoritative — create conflict
            result.conflict_fields.append({
                "field_name": field_name,
                "current_value": str(existing_value) if existing_value else None,
                "incoming_value": str(incoming_value) if incoming_value else None,
                "source_type": source_type,
            })

    return result


async def _load_authorities(pool: asyncpg.Pool, tenant_id: UUID) -> dict[tuple[str, str], int]:
    """Load authority config as {(field_name, source_type): priority}."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT field_name, source_type, priority FROM asset_field_authorities WHERE tenant_id = $1",
            tenant_id,
        )
    return {(row["field_name"], row["source_type"]): row["priority"] for row in rows}


def _get_max_priority(authorities: dict[tuple[str, str], int], field_name: str) -> int:
    """Get the highest priority for a given field across all sources."""
    return max(
        (p for (f, _), p in authorities.items() if f == field_name),
        default=0,
    )


async def apply_auto_merge(
    pool: asyncpg.Pool,
    asset_id: UUID,
    fields: dict[str, str | None],
) -> None:
    """Apply authoritative field updates directly to the asset."""
    if not fields:
        return

    set_clauses = []
    values = []
    for i, (field, value) in enumerate(fields.items(), start=2):
        set_clauses.append(f"{field} = ${i}")
        values.append(value)

    set_clauses.append("updated_at = now()")
    sql = f"UPDATE assets SET {', '.join(set_clauses)} WHERE id = $1"
    async with pool.acquire() as conn:
        await conn.execute(sql, asset_id, *values)


async def create_conflicts(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    asset_id: UUID,
    source_type: str,
    conflict_fields: list[dict],
) -> list[UUID]:
    """Insert conflict records into import_conflicts table."""
    conflict_ids = []
    async with pool.acquire() as conn:
        for cf in conflict_fields:
            row = await conn.fetchrow(
                "INSERT INTO import_conflicts (tenant_id, asset_id, source_type, field_name, current_value, incoming_value) "
                "VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
                tenant_id, asset_id, source_type, cf["field_name"], cf["current_value"], cf["incoming_value"],
            )
            conflict_ids.append(row["id"])
    return conflict_ids
```

- [ ] **Step 2: Create test_authority.py (unit tests for logic)**

Create `ingestion-engine/tests/test_authority.py`:

```python
from app.pipeline.authority import _get_max_priority, AuthorityResult


def test_get_max_priority():
    authorities = {
        ("serial_number", "ipmi"): 100,
        ("serial_number", "snmp"): 80,
        ("serial_number", "manual"): 50,
        ("vendor", "ipmi"): 100,
    }
    assert _get_max_priority(authorities, "serial_number") == 100
    assert _get_max_priority(authorities, "vendor") == 100
    assert _get_max_priority(authorities, "nonexistent") == 0


def test_authority_result_structure():
    result = AuthorityResult()
    result.auto_merge_fields["vendor"] = "Dell"
    result.conflict_fields.append({
        "field_name": "name",
        "current_value": "Old Name",
        "incoming_value": "New Name",
        "source_type": "excel",
    })
    result.skipped_fields.append("serial_number")

    assert result.auto_merge_fields == {"vendor": "Dell"}
    assert len(result.conflict_fields) == 1
    assert result.skipped_fields == ["serial_number"]
```

- [ ] **Step 3: Run tests**

```bash
pytest tests/test_authority.py -v
```

Expected: 2 tests pass.

- [ ] **Step 4: Commit**

```bash
git add ingestion-engine/app/pipeline/authority.py ingestion-engine/tests/test_authority.py
git commit -m "feat: add authority check + conflict resolution with auto-merge logic"
```

---

## Task 6: Pipeline Processor (Orchestrator)

**Files:**
- Create: `ingestion-engine/app/pipeline/processor.py`
- Create: `ingestion-engine/tests/test_processor.py`

- [ ] **Step 1: Create processor.py**

Create `ingestion-engine/app/pipeline/processor.py`:

```python
import json
from uuid import UUID

import asyncpg

from app.models.common import RawAssetData, PipelineResult
from app.pipeline.normalize import normalize
from app.pipeline.deduplicate import deduplicate, DeduplicateResult
from app.pipeline.validate import validate_for_create, validate_for_update
from app.pipeline.authority import check_authority, apply_auto_merge, create_conflicts


async def process_single(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    raw: RawAssetData,
) -> PipelineResult:
    """Process a single RawAssetData through the full transform pipeline."""

    # Step 1: Normalize
    normalized = normalize(raw)

    # Step 2: Deduplicate
    dedup = await deduplicate(pool, tenant_id, normalized)

    if dedup.is_new:
        # New asset — validate for create
        errors = validate_for_create(normalized)
        if errors:
            return PipelineResult(action="skipped", errors=errors)

        # Insert new asset
        asset_id = await _create_asset(pool, tenant_id, normalized)
        return PipelineResult(asset_id=asset_id, action="created")
    else:
        # Existing asset — validate for update
        errors = validate_for_update(normalized)
        if errors:
            return PipelineResult(asset_id=dedup.existing_asset_id, action="skipped", errors=errors)

        # Step 3: Authority check
        auth_result = await check_authority(
            pool, tenant_id, dedup.existing_asset_id,
            normalized.source, normalized.fields, dedup.existing_fields,
        )

        # Auto-merge authoritative fields
        if auth_result.auto_merge_fields:
            await apply_auto_merge(pool, dedup.existing_asset_id, auth_result.auto_merge_fields)

        # Create conflict records for non-authoritative fields
        if auth_result.conflict_fields:
            await create_conflicts(
                pool, tenant_id, dedup.existing_asset_id,
                normalized.source, auth_result.conflict_fields,
            )
            return PipelineResult(
                asset_id=dedup.existing_asset_id,
                action="conflict",
                conflicts=[cf for cf in auth_result.conflict_fields],
            )

        if auth_result.auto_merge_fields:
            return PipelineResult(asset_id=dedup.existing_asset_id, action="updated")

        return PipelineResult(asset_id=dedup.existing_asset_id, action="skipped")


async def process_batch(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    items: list[RawAssetData],
    progress_callback=None,
) -> dict:
    """Process a batch of items. Returns stats dict."""
    stats = {"created": 0, "updated": 0, "skipped": 0, "conflicts": 0, "errors": 0}

    for i, item in enumerate(items):
        try:
            result = await process_single(pool, tenant_id, item)
            if result.action in stats:
                stats[result.action] += 1
            if result.errors:
                stats["errors"] += 1
        except Exception as e:
            stats["errors"] += 1

        if progress_callback and (i + 1) % 50 == 0:
            await progress_callback(i + 1, len(items), stats)

    return stats


async def _create_asset(pool: asyncpg.Pool, tenant_id: UUID, raw: RawAssetData) -> UUID:
    """Insert a new asset from normalized data."""
    fields = raw.fields
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "INSERT INTO assets (tenant_id, asset_tag, name, type, sub_type, status, bia_level, "
            "vendor, model, serial_number, property_number, control_number, attributes) "
            "VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id",
            tenant_id,
            fields.get("asset_tag"),
            fields.get("name"),
            fields.get("type"),
            fields.get("sub_type"),
            fields.get("status", "inventoried"),
            fields.get("bia_level", "normal"),
            fields.get("vendor"),
            fields.get("model"),
            fields.get("serial_number"),
            fields.get("property_number"),
            fields.get("control_number"),
            json.dumps(raw.attributes) if raw.attributes else "{}",
        )
    return row["id"]
```

- [ ] **Step 2: Verify imports**

```bash
cd /cmdb-platform/ingestion-engine
python -c "from app.pipeline.processor import process_single, process_batch; print('OK')"
```

Expected: "OK"

- [ ] **Step 3: Commit**

```bash
git add ingestion-engine/app/pipeline/processor.py
git commit -m "feat: add pipeline processor - orchestrates normalize/dedup/validate/authority"
```

---

## Task 7: Excel/CSV Parser + Templates

**Files:**
- Create: `ingestion-engine/app/importers/__init__.py`
- Create: `ingestion-engine/app/importers/excel_parser.py`
- Create: `ingestion-engine/app/importers/templates.py`
- Create: `ingestion-engine/tests/test_excel_parser.py`

- [ ] **Step 1: Create excel_parser.py**

Create `ingestion-engine/app/importers/__init__.py` (empty).

Create `ingestion-engine/app/importers/excel_parser.py`:

```python
import csv
import io
from pathlib import Path

import openpyxl

from app.models.common import RawAssetData
from app.models.import_job import ParsedRow, ParseResult

EXPECTED_HEADERS = [
    "asset_tag", "name", "type", "sub_type", "status", "bia_level",
    "vendor", "model", "serial_number", "property_number", "control_number",
]


def parse_excel(file_path: str) -> ParseResult:
    """Parse an Excel file into structured rows."""
    wb = openpyxl.load_workbook(file_path, read_only=True)
    ws = wb.active

    rows = list(ws.iter_rows(values_only=True))
    if not rows:
        return ParseResult(total_rows=0, valid_rows=[], error_rows=[], preview=[])

    # First row is header
    headers = [str(h).strip().lower().replace(" ", "_") if h else "" for h in rows[0]]

    valid_rows = []
    error_rows = []

    for i, row_values in enumerate(rows[1:], start=2):
        row_data = {}
        for j, header in enumerate(headers):
            if header and j < len(row_values):
                val = row_values[j]
                row_data[header] = str(val).strip() if val is not None else None

        # Basic row-level validation
        errors = []
        if not row_data.get("asset_tag"):
            errors.append("Missing asset_tag")
        if not row_data.get("name"):
            errors.append("Missing name")
        if not row_data.get("type"):
            errors.append("Missing type")

        parsed = ParsedRow(row_num=i, data=row_data, errors=errors if errors else None)
        if errors:
            error_rows.append(parsed)
        else:
            valid_rows.append(parsed)

    wb.close()

    return ParseResult(
        total_rows=len(valid_rows) + len(error_rows),
        valid_rows=valid_rows,
        error_rows=error_rows,
        preview=valid_rows[:20],
    )


def parse_csv(file_path: str) -> ParseResult:
    """Parse a CSV file into structured rows."""
    with open(file_path, "r", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        # Normalize header names
        if reader.fieldnames:
            reader.fieldnames = [h.strip().lower().replace(" ", "_") for h in reader.fieldnames]

        valid_rows = []
        error_rows = []

        for i, row in enumerate(reader, start=2):
            row_data = {k: v.strip() if v else None for k, v in row.items()}

            errors = []
            if not row_data.get("asset_tag"):
                errors.append("Missing asset_tag")
            if not row_data.get("name"):
                errors.append("Missing name")
            if not row_data.get("type"):
                errors.append("Missing type")

            parsed = ParsedRow(row_num=i, data=row_data, errors=errors if errors else None)
            if errors:
                error_rows.append(parsed)
            else:
                valid_rows.append(parsed)

    return ParseResult(
        total_rows=len(valid_rows) + len(error_rows),
        valid_rows=valid_rows,
        error_rows=error_rows,
        preview=valid_rows[:20],
    )


def rows_to_raw_assets(rows: list[ParsedRow], source: str = "excel") -> list[RawAssetData]:
    """Convert parsed rows to RawAssetData for the pipeline."""
    results = []
    for row in rows:
        unique_key = row.data.get("serial_number") or row.data.get("asset_tag") or ""
        known_fields = {}
        attributes = {}

        for k, v in row.data.items():
            if k in {
                "asset_tag", "name", "type", "sub_type", "status", "bia_level",
                "vendor", "model", "serial_number", "property_number", "control_number",
            }:
                known_fields[k] = v
            elif v:
                attributes[k] = v

        results.append(RawAssetData(
            source=source,
            unique_key=unique_key,
            fields=known_fields,
            attributes=attributes if attributes else None,
        ))
    return results
```

- [ ] **Step 2: Create templates.py**

Create `ingestion-engine/app/importers/templates.py`:

```python
import io

import openpyxl
from openpyxl.styles import Font, PatternFill, Alignment

ASSET_HEADERS = [
    ("asset_tag", "Asset Tag *", "SRV-PROD-001"),
    ("name", "Name *", "Production Server 01"),
    ("type", "Type *", "server"),
    ("sub_type", "Sub Type", "rack_server"),
    ("status", "Status", "operational"),
    ("bia_level", "BIA Level", "critical"),
    ("vendor", "Vendor", "Dell"),
    ("model", "Model", "PowerEdge R750"),
    ("serial_number", "Serial Number", "SN-DELL-001"),
    ("property_number", "Property Number", "PROP-001"),
    ("control_number", "Control Number", "CTRL-001"),
]


def generate_asset_template() -> io.BytesIO:
    """Generate an Excel template for asset import."""
    wb = openpyxl.Workbook()
    ws = wb.active
    ws.title = "Assets"

    header_font = Font(bold=True, color="FFFFFF")
    header_fill = PatternFill(start_color="4472C4", end_color="4472C4", fill_type="solid")

    # Write headers
    for col, (field_name, display_name, example) in enumerate(ASSET_HEADERS, start=1):
        cell = ws.cell(row=1, column=col, value=display_name)
        cell.font = header_font
        cell.fill = header_fill
        cell.alignment = Alignment(horizontal="center")
        ws.column_dimensions[cell.column_letter].width = max(len(display_name) + 4, 15)

    # Write example row
    for col, (field_name, display_name, example) in enumerate(ASSET_HEADERS, start=1):
        ws.cell(row=2, column=col, value=example)

    # Add validation notes sheet
    notes = wb.create_sheet("Notes")
    notes.cell(row=1, column=1, value="Field").font = Font(bold=True)
    notes.cell(row=1, column=2, value="Required").font = Font(bold=True)
    notes.cell(row=1, column=3, value="Valid Values").font = Font(bold=True)
    validations = [
        ("asset_tag", "Yes", "Unique identifier, e.g. SRV-PROD-001"),
        ("name", "Yes", "Human-readable name"),
        ("type", "Yes", "server | network | storage | power"),
        ("sub_type", "No", "rack_server | switch | ups | nas | ..."),
        ("status", "No", "inventoried | deployed | operational | maintenance | decommissioned"),
        ("bia_level", "No", "critical | important | normal | minor"),
    ]
    for i, (field, req, vals) in enumerate(validations, start=2):
        notes.cell(row=i, column=1, value=field)
        notes.cell(row=i, column=2, value=req)
        notes.cell(row=i, column=3, value=vals)

    buf = io.BytesIO()
    wb.save(buf)
    buf.seek(0)
    return buf
```

- [ ] **Step 3: Create test_excel_parser.py**

Create `ingestion-engine/tests/test_excel_parser.py`:

```python
import tempfile
import openpyxl

from app.importers.excel_parser import parse_excel, rows_to_raw_assets
from app.importers.templates import generate_asset_template


def _create_test_excel(rows: list[list]) -> str:
    """Helper: create a temp Excel file with given header + data rows."""
    wb = openpyxl.Workbook()
    ws = wb.active
    for row in rows:
        ws.append(row)
    path = tempfile.mktemp(suffix=".xlsx")
    wb.save(path)
    return path


def test_parse_valid_excel():
    path = _create_test_excel([
        ["asset_tag", "name", "type", "vendor", "serial_number"],
        ["SRV-001", "Server 1", "server", "Dell", "SN-001"],
        ["NET-001", "Switch 1", "network", "Cisco", "SN-002"],
    ])
    result = parse_excel(path)
    assert result.total_rows == 2
    assert len(result.valid_rows) == 2
    assert len(result.error_rows) == 0
    assert result.valid_rows[0].data["asset_tag"] == "SRV-001"


def test_parse_excel_with_errors():
    path = _create_test_excel([
        ["asset_tag", "name", "type"],
        ["SRV-001", "Server 1", "server"],
        [None, "No Tag", "server"],           # Missing asset_tag
        ["NET-001", None, "network"],          # Missing name
    ])
    result = parse_excel(path)
    assert result.total_rows == 3
    assert len(result.valid_rows) == 1
    assert len(result.error_rows) == 2


def test_rows_to_raw_assets():
    path = _create_test_excel([
        ["asset_tag", "name", "type", "serial_number", "custom_field"],
        ["SRV-001", "Server 1", "server", "SN-001", "custom_value"],
    ])
    result = parse_excel(path)
    raw_items = rows_to_raw_assets(result.valid_rows)
    assert len(raw_items) == 1
    assert raw_items[0].source == "excel"
    assert raw_items[0].unique_key == "SN-001"
    assert raw_items[0].fields["asset_tag"] == "SRV-001"
    assert raw_items[0].attributes["custom_field"] == "custom_value"


def test_generate_template():
    buf = generate_asset_template()
    wb = openpyxl.load_workbook(buf)
    ws = wb.active
    assert ws.cell(row=1, column=1).value == "Asset Tag *"
    assert ws.cell(row=2, column=1).value == "SRV-PROD-001"
    assert "Notes" in wb.sheetnames
```

- [ ] **Step 4: Run tests**

```bash
cd /cmdb-platform/ingestion-engine
pytest tests/test_excel_parser.py -v
```

Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/importers/ ingestion-engine/tests/test_excel_parser.py
git commit -m "feat: add Excel/CSV parser + template generator with tests"
```

---

## Task 8: Celery Task for Async Import Processing

**Files:**
- Create: `ingestion-engine/app/tasks/__init__.py`
- Create: `ingestion-engine/app/tasks/celery_app.py`
- Create: `ingestion-engine/app/tasks/import_task.py`

- [ ] **Step 1: Create Celery app**

Create `ingestion-engine/app/tasks/__init__.py` (empty).

Create `ingestion-engine/app/tasks/celery_app.py`:

```python
from celery import Celery

from app.config import settings

celery_app = Celery(
    "ingestion",
    broker=settings.celery_broker_url,
    backend=settings.redis_url,
)

celery_app.conf.update(
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    enable_utc=True,
    task_track_started=True,
    task_acks_late=True,
    worker_prefetch_multiplier=1,
)
```

- [ ] **Step 2: Create import_task.py**

Create `ingestion-engine/app/tasks/import_task.py`:

```python
import asyncio
import json

import asyncpg

from app.config import settings
from app.importers.excel_parser import parse_excel, parse_csv, rows_to_raw_assets
from app.pipeline.processor import process_batch
from app.tasks.celery_app import celery_app


def _run_async(coro):
    """Run an async function in a sync Celery task."""
    loop = asyncio.new_event_loop()
    try:
        return loop.run_until_complete(coro)
    finally:
        loop.close()


@celery_app.task(bind=True, name="ingestion.process_import")
def process_import_task(self, import_job_id: str, tenant_id: str, file_path: str, file_type: str):
    """Celery task: process an uploaded Excel/CSV file through the pipeline."""
    _run_async(_process_import(self, import_job_id, tenant_id, file_path, file_type))


async def _process_import(task, import_job_id: str, tenant_id: str, file_path: str, file_type: str):
    pool = await asyncpg.create_pool(settings.database_url, min_size=2, max_size=10)
    try:
        # Update job status to processing
        await _update_job_status(pool, import_job_id, "processing")

        # Parse file
        if file_type == "csv":
            parse_result = parse_csv(file_path)
        else:
            parse_result = parse_excel(file_path)

        # Convert to RawAssetData
        raw_items = rows_to_raw_assets(parse_result.valid_rows, source=file_type)

        # Update total rows
        await _update_job_total(pool, import_job_id, len(raw_items))

        # Process through pipeline
        from uuid import UUID
        tid = UUID(tenant_id)

        async def progress_callback(processed, total, stats):
            await _update_job_progress(pool, import_job_id, processed, stats)
            task.update_state(state="PROGRESS", meta={"processed": processed, "total": total})

        stats = await process_batch(pool, tid, raw_items, progress_callback=progress_callback)

        # Mark completed
        await _update_job_completed(pool, import_job_id, stats, parse_result.error_rows)

    except Exception as e:
        await _update_job_status(pool, import_job_id, "failed")
        raise
    finally:
        await pool.close()


async def _update_job_status(pool: asyncpg.Pool, job_id: str, status: str):
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET status = $2 WHERE id = $1::uuid",
            job_id, status,
        )


async def _update_job_total(pool: asyncpg.Pool, job_id: str, total: int):
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET total_rows = $2 WHERE id = $1::uuid",
            job_id, total,
        )


async def _update_job_progress(pool: asyncpg.Pool, job_id: str, processed: int, stats: dict):
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET processed_rows = $2, stats = $3::jsonb WHERE id = $1::uuid",
            job_id, processed, json.dumps(stats),
        )


async def _update_job_completed(pool: asyncpg.Pool, job_id: str, stats: dict, error_rows: list):
    error_details = [{"row": r.row_num, "errors": r.errors} for r in error_rows] if error_rows else []
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET status = 'completed', processed_rows = total_rows, "
            "stats = $2::jsonb, error_details = $3::jsonb, completed_at = now() WHERE id = $1::uuid",
            job_id, json.dumps(stats), json.dumps(error_details),
        )
```

- [ ] **Step 3: Verify imports**

```bash
python -c "from app.tasks.import_task import process_import_task; print('OK')"
```

Expected: "OK"

- [ ] **Step 4: Commit**

```bash
git add ingestion-engine/app/tasks/
git commit -m "feat: add Celery async import task with progress tracking"
```

---

## Task 9: FastAPI Dependencies + Import Routes

**Files:**
- Create: `ingestion-engine/app/dependencies.py`
- Modify: `ingestion-engine/app/routes/imports.py`
- Modify: `ingestion-engine/app/routes/conflicts.py`

- [ ] **Step 1: Create dependencies.py**

Create `ingestion-engine/app/dependencies.py`:

```python
import asyncpg
from fastapi import Depends, Request
from nats.aio.client import Client as NATSClient


async def get_db_pool(request: Request) -> asyncpg.Pool:
    return request.app.state.db_pool


async def get_nats(request: Request) -> NATSClient | None:
    return request.app.state.nats_client
```

- [ ] **Step 2: Rewrite imports router**

Replace `ingestion-engine/app/routes/imports.py`:

```python
import os
import uuid

import asyncpg
from fastapi import APIRouter, Depends, File, Query, UploadFile
from fastapi.responses import StreamingResponse

from app.config import settings
from app.dependencies import get_db_pool
from app.importers.excel_parser import parse_excel, parse_csv
from app.importers.templates import generate_asset_template
from app.tasks.import_task import process_import_task

router = APIRouter(tags=["imports"])


@router.post("/import/upload")
async def upload_file(
    file: UploadFile = File(...),
    tenant_id: str = Query(...),
    uploaded_by: str = Query(None),
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Upload Excel/CSV file, parse it, store job, return preview."""
    # Determine file type
    filename = file.filename or "upload.xlsx"
    file_type = "csv" if filename.endswith(".csv") else "excel"

    # Save uploaded file
    os.makedirs(settings.upload_dir, exist_ok=True)
    file_id = str(uuid.uuid4())
    file_path = os.path.join(settings.upload_dir, f"{file_id}_{filename}")
    content = await file.read()
    with open(file_path, "wb") as f:
        f.write(content)

    # Parse for preview
    parse_result = parse_excel(file_path) if file_type == "excel" else parse_csv(file_path)

    # Create import job record
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "INSERT INTO import_jobs (tenant_id, type, filename, status, total_rows, error_details, uploaded_by) "
            "VALUES ($1::uuid, $2, $3, 'previewing', $4, $5::jsonb, $6::uuid) RETURNING id, created_at",
            tenant_id, file_type, filename, parse_result.total_rows,
            "[]",
            uploaded_by,
        )

    return {
        "job_id": str(row["id"]),
        "filename": filename,
        "status": "previewing",
        "total_rows": parse_result.total_rows,
        "valid_rows": len(parse_result.valid_rows),
        "error_rows": len(parse_result.error_rows),
        "preview": [r.model_dump() for r in parse_result.preview[:20]],
        "errors": [r.model_dump() for r in parse_result.error_rows[:50]],
        "file_path": file_path,
    }


@router.get("/import/{job_id}/preview")
async def get_preview(
    job_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Get import job details and preview."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow("SELECT * FROM import_jobs WHERE id = $1::uuid", job_id)
    if not row:
        return {"error": "Job not found"}, 404
    return dict(row)


@router.post("/import/{job_id}/confirm")
async def confirm_import(
    job_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Confirm and start async processing of the import job."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow("SELECT * FROM import_jobs WHERE id = $1::uuid", job_id)

    if not row:
        return {"error": "Job not found"}, 404
    if row["status"] != "previewing":
        return {"error": f"Job status is '{row['status']}', expected 'previewing'"}, 400

    # Update status to confirmed
    async with pool.acquire() as conn:
        await conn.execute("UPDATE import_jobs SET status = 'confirmed' WHERE id = $1::uuid", job_id)

    # Find the file path (stored during upload in the upload dir)
    file_path = _find_file(job_id, row["filename"])

    # Dispatch Celery task
    process_import_task.delay(job_id, str(row["tenant_id"]), file_path, row["type"])

    return {"job_id": job_id, "status": "confirmed", "message": "Import processing started"}


@router.get("/import/{job_id}/progress")
async def get_progress(
    job_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Query current import progress."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT id, status, total_rows, processed_rows, stats, error_details, completed_at "
            "FROM import_jobs WHERE id = $1::uuid",
            job_id,
        )
    if not row:
        return {"error": "Job not found"}, 404
    return dict(row)


@router.get("/import/templates/{template_type}")
async def download_template(template_type: str):
    """Download an import template Excel file."""
    if template_type != "asset":
        return {"error": f"Unknown template type: {template_type}"}, 404

    buf = generate_asset_template()
    return StreamingResponse(
        buf,
        media_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        headers={"Content-Disposition": f"attachment; filename=cmdb_asset_template.xlsx"},
    )


def _find_file(job_id: str, filename: str) -> str:
    """Find the uploaded file in the upload directory."""
    for f in os.listdir(settings.upload_dir):
        if f.endswith(f"_{filename}"):
            return os.path.join(settings.upload_dir, f)
    # Fallback: search by any file
    for f in os.listdir(settings.upload_dir):
        if job_id[:8] in f:
            return os.path.join(settings.upload_dir, f)
    return os.path.join(settings.upload_dir, filename)
```

- [ ] **Step 3: Rewrite conflicts router**

Replace `ingestion-engine/app/routes/conflicts.py`:

```python
import json
from uuid import UUID

import asyncpg
from fastapi import APIRouter, Depends, Query

from app.dependencies import get_db_pool
from app.models.conflict import ConflictResolution, BatchConflictResolution

router = APIRouter(tags=["conflicts"])


@router.get("/conflicts")
async def list_conflicts(
    tenant_id: str = Query(...),
    status: str = Query("pending"),
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List import conflicts for a tenant."""
    offset = (page - 1) * page_size
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT * FROM import_conflicts WHERE tenant_id = $1::uuid AND status = $2 "
            "ORDER BY created_at DESC LIMIT $3 OFFSET $4",
            tenant_id, status, page_size, offset,
        )
        count = await conn.fetchval(
            "SELECT count(*) FROM import_conflicts WHERE tenant_id = $1::uuid AND status = $2",
            tenant_id, status,
        )
    return {
        "data": [dict(r) for r in rows],
        "pagination": {"page": page, "page_size": page_size, "total": count},
    }


@router.post("/conflicts/{conflict_id}/resolve")
async def resolve_conflict(
    conflict_id: str,
    body: ConflictResolution,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Resolve a single conflict (approve or reject)."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow("SELECT * FROM import_conflicts WHERE id = $1::uuid", conflict_id)
        if not row:
            return {"error": "Conflict not found"}, 404

        if body.action == "approve":
            # Apply the incoming value to the asset
            await conn.execute(
                f"UPDATE assets SET {row['field_name']} = $2, updated_at = now() WHERE id = $1",
                row["asset_id"], row["incoming_value"],
            )

        await conn.execute(
            "UPDATE import_conflicts SET status = $2, resolved_by = $3, resolved_at = now() "
            "WHERE id = $1::uuid",
            conflict_id,
            "approved" if body.action == "approve" else "rejected",
            str(body.resolved_by),
        )

    return {"id": conflict_id, "status": body.action + "d"}


@router.post("/conflicts/batch-resolve")
async def batch_resolve(
    body: BatchConflictResolution,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Batch resolve multiple conflicts."""
    results = []
    for cid in body.conflict_ids:
        resolution = ConflictResolution(action=body.action, resolved_by=body.resolved_by)
        result = await resolve_conflict(str(cid), resolution, pool)
        results.append(result)
    return {"resolved": len(results), "results": results}
```

- [ ] **Step 4: Verify app starts**

```bash
cd /cmdb-platform/ingestion-engine
python -c "from app.main import app; print('Routes:', [r.path for r in app.routes])"
```

Expected: Routes listed including /ingestion/import/upload, /ingestion/conflicts, etc.

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/dependencies.py ingestion-engine/app/routes/
git commit -m "feat: add import routes (upload/preview/confirm/progress/template) + conflict resolution"
```

---

## Task 10: Collector Framework + Collectors Route

**Files:**
- Create: `ingestion-engine/app/collectors/__init__.py`
- Create: `ingestion-engine/app/collectors/base.py`
- Create: `ingestion-engine/app/collectors/manager.py`
- Modify: `ingestion-engine/app/routes/collectors.py`

- [ ] **Step 1: Create collector base protocol**

Create `ingestion-engine/app/collectors/__init__.py` (empty).

Create `ingestion-engine/app/collectors/base.py`:

```python
from typing import Protocol, runtime_checkable

from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData


@runtime_checkable
class Collector(Protocol):
    name: str
    collect_type: str  # "full_sync" | "incremental" | "event_driven"

    async def collect(self, target: CollectTarget) -> list[RawAssetData]: ...
    async def test_connection(self, target: CollectTarget) -> ConnectionResult: ...
    def supported_fields(self) -> list[FieldMapping]: ...


class CollectorRegistry:
    """Registry of available collectors."""

    def __init__(self):
        self._collectors: dict[str, Collector] = {}

    def register(self, collector: Collector):
        self._collectors[collector.name] = collector

    def get(self, name: str) -> Collector | None:
        return self._collectors.get(name)

    def list_all(self) -> list[dict]:
        return [
            {
                "name": c.name,
                "collect_type": c.collect_type,
                "supported_fields": [f.model_dump() for f in c.supported_fields()],
            }
            for c in self._collectors.values()
        ]


# Global registry
registry = CollectorRegistry()
```

- [ ] **Step 2: Create collector manager**

Create `ingestion-engine/app/collectors/manager.py`:

```python
from dataclasses import dataclass, field
from datetime import datetime

from app.collectors.base import registry, Collector
from app.models.common import CollectTarget, ConnectionResult


@dataclass
class CollectorStatus:
    name: str
    running: bool = False
    last_run: datetime | None = None
    last_error: str | None = None
    items_collected: int = 0


class CollectorManager:
    """Manages collector lifecycle (start/stop/status)."""

    def __init__(self):
        self._status: dict[str, CollectorStatus] = {}

    def get_status(self, name: str) -> CollectorStatus | None:
        collector = registry.get(name)
        if not collector:
            return None
        if name not in self._status:
            self._status[name] = CollectorStatus(name=name)
        return self._status[name]

    def list_all(self) -> list[dict]:
        result = []
        for info in registry.list_all():
            status = self._status.get(info["name"], CollectorStatus(name=info["name"]))
            result.append({
                **info,
                "running": status.running,
                "last_run": status.last_run.isoformat() if status.last_run else None,
                "last_error": status.last_error,
                "items_collected": status.items_collected,
            })
        return result

    async def start(self, name: str) -> bool:
        status = self.get_status(name)
        if not status:
            return False
        status.running = True
        return True

    async def stop(self, name: str) -> bool:
        status = self.get_status(name)
        if not status:
            return False
        status.running = False
        return True

    async def test_connection(self, name: str, target: CollectTarget) -> ConnectionResult:
        collector = registry.get(name)
        if not collector:
            return ConnectionResult(success=False, message=f"Unknown collector: {name}")
        return await collector.test_connection(target)


# Global manager
manager = CollectorManager()
```

- [ ] **Step 3: Rewrite collectors route**

Replace `ingestion-engine/app/routes/collectors.py`:

```python
from fastapi import APIRouter

from app.collectors.manager import manager
from app.models.common import CollectTarget, ConnectionResult

router = APIRouter(tags=["collectors"])


@router.get("/collectors")
async def list_collectors():
    """List all registered collectors and their status."""
    return {"collectors": manager.list_all()}


@router.post("/collectors/{name}/start")
async def start_collector(name: str):
    """Start a collector."""
    ok = await manager.start(name)
    if not ok:
        return {"error": f"Unknown collector: {name}"}, 404
    return {"name": name, "status": "started"}


@router.post("/collectors/{name}/stop")
async def stop_collector(name: str):
    """Stop a collector."""
    ok = await manager.stop(name)
    if not ok:
        return {"error": f"Unknown collector: {name}"}, 404
    return {"name": name, "status": "stopped"}


@router.post("/collectors/{name}/test")
async def test_collector(name: str, target: CollectTarget):
    """Test collector connectivity."""
    result = await manager.test_connection(name, target)
    return result.model_dump()
```

- [ ] **Step 4: Verify**

```bash
cd /cmdb-platform/ingestion-engine
python -c "from app.collectors.base import registry; from app.collectors.manager import manager; print('OK')"
```

Expected: "OK"

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/collectors/ ingestion-engine/app/routes/collectors.py
git commit -m "feat: add collector framework - protocol, registry, manager + routes"
```

---

## Task 11: Docker Compose Integration + Smoke Test

**Files:**
- Modify: `cmdb-core/deploy/docker-compose.yml` (add ingestion-engine service)
- Create: `ingestion-engine/run.sh`

- [ ] **Step 1: Add ingestion-engine to docker-compose**

Append to `cmdb-core/deploy/docker-compose.yml` services section:

```yaml
  ingestion-engine:
    build: ../../ingestion-engine
    environment:
      INGESTION_DATABASE_URL: postgresql://cmdb:${DB_PASS:-cmdb_secret}@postgres:5432/cmdb
      INGESTION_REDIS_URL: redis://redis:6379/1
      INGESTION_NATS_URL: nats://nats:4222
      INGESTION_CELERY_BROKER_URL: redis://redis:6379/2
    ports:
      - "8081:8081"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  ingestion-worker:
    build: ../../ingestion-engine
    command: celery -A app.tasks.celery_app worker -l info -c 2
    environment:
      INGESTION_DATABASE_URL: postgresql://cmdb:${DB_PASS:-cmdb_secret}@postgres:5432/cmdb
      INGESTION_REDIS_URL: redis://redis:6379/1
      INGESTION_CELERY_BROKER_URL: redis://redis:6379/2
    depends_on:
      - redis
```

- [ ] **Step 2: Create run.sh for local dev**

Create `ingestion-engine/run.sh`:

```bash
#!/bin/bash
set -e
cd "$(dirname "$0")"
export INGESTION_DATABASE_URL="${INGESTION_DATABASE_URL:-postgresql://cmdb:cmdb_secret@localhost:5432/cmdb}"
export INGESTION_REDIS_URL="${INGESTION_REDIS_URL:-redis://localhost:6379/1}"
export INGESTION_NATS_URL="${INGESTION_NATS_URL:-nats://localhost:4222}"
export INGESTION_CELERY_BROKER_URL="${INGESTION_CELERY_BROKER_URL:-redis://localhost:6379/2}"
uvicorn app.main:app --host 0.0.0.0 --port 8081 --reload
```

```bash
chmod +x ingestion-engine/run.sh
```

- [ ] **Step 3: Run all tests**

```bash
cd /cmdb-platform/ingestion-engine
pytest tests/ -v
```

Expected: All tests pass (normalize: 3, validate: 5, authority: 2, excel_parser: 4 = 14 tests).

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/deploy/docker-compose.yml ingestion-engine/run.sh
git commit -m "feat: add ingestion-engine to docker-compose + local dev runner"
```

---

## Endpoint Summary

After all 11 tasks, the ingestion engine provides these 17 API endpoints:

| # | Method | Path | Purpose | Task |
|---|--------|------|---------|------|
| 1 | GET | `/healthz` | Health check | 1 |
| 2 | POST | `/ingestion/import/upload` | Upload Excel/CSV file | 9 |
| 3 | GET | `/ingestion/import/{id}/preview` | Preview parsed results | 9 |
| 4 | POST | `/ingestion/import/{id}/confirm` | Confirm & start processing | 9 |
| 5 | GET | `/ingestion/import/{id}/progress` | Query import progress | 9 |
| 6 | GET | `/ingestion/import/templates/{type}` | Download template | 9 |
| 7 | GET | `/ingestion/conflicts` | List pending conflicts | 9 |
| 8 | POST | `/ingestion/conflicts/{id}/resolve` | Resolve single conflict | 9 |
| 9 | POST | `/ingestion/conflicts/batch-resolve` | Batch resolve | 9 |
| 10 | GET | `/ingestion/collectors` | List collectors + status | 10 |
| 11 | POST | `/ingestion/collectors/{name}/start` | Start collector | 10 |
| 12 | POST | `/ingestion/collectors/{name}/stop` | Stop collector | 10 |
| 13 | POST | `/ingestion/collectors/{name}/test` | Test connectivity | 10 |
