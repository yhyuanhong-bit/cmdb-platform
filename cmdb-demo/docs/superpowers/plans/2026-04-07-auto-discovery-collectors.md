# Auto Discovery Collectors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement SNMP/SSH/IPMI collectors with credential management, scan target CRUD, mode-based result routing, and frontend integration.

**Architecture:** Three collectors in `ingestion-engine` driven by Celery tasks. Results routed by mode (auto/review/smart) through existing pipeline or cmdb-core discovery staging. Credentials encrypted with AES-256-GCM in PostgreSQL. Frontend adds credential tab in SystemSettings and scan management tab in AutoDiscovery.

**Tech Stack:** Python 3.12, FastAPI, pysnmp>=7.0, asyncssh>=2.14, pyghmi>=1.5, cryptography, Celery/Redis, asyncpg, React/TypeScript, TanStack Query.

**Spec:** `docs/superpowers/specs/2026-04-07-auto-discovery-collectors-design.md`

---

## File Structure

### New Files (ingestion-engine)

| File | Responsibility |
|------|---------------|
| `app/credentials/__init__.py` | Package init |
| `app/credentials/encryption.py` | AES-256-GCM encrypt/decrypt for credential params |
| `app/credentials/provider.py` | `DBCredentialProvider` — load + decrypt credentials from DB |
| `app/collectors/snmp.py` | SNMP collector — pysnmp async, OID queries, vendor detection |
| `app/collectors/ssh.py` | SSH collector — asyncssh, command execution + parsing |
| `app/collectors/ipmi.py` | IPMI collector — pyghmi sync wrapped in asyncio.to_thread |
| `app/routes/credentials.py` | CRUD endpoints for `/credentials` |
| `app/routes/scan_targets.py` | CRUD endpoints for `/scan-targets` |
| `app/routes/discovery.py` | `/discovery/scan`, `/discovery/tasks`, `/discovery/tasks/{id}` |
| `app/tasks/discovery_task.py` | Celery task: orchestrates collector + mode routing |
| `tests/test_encryption.py` | Tests for AES encrypt/decrypt |
| `tests/test_snmp_collector.py` | Tests for SNMP collector (mocked pysnmp) |
| `tests/test_ssh_collector.py` | Tests for SSH collector (mocked asyncssh) |
| `tests/test_ipmi_collector.py` | Tests for IPMI collector (mocked pyghmi) |
| `tests/test_discovery_task.py` | Tests for mode routing logic |

### New Files (cmdb-core)

| File | Responsibility |
|------|---------------|
| `db/migrations/000017_discovery_collectors.up.sql` | credentials + scan_targets tables + assets.ip_address |
| `db/migrations/000017_discovery_collectors.down.sql` | Rollback migration |

### New Files (cmdb-demo frontend)

| File | Responsibility |
|------|---------------|
| `src/lib/api/ingestion.ts` | API client for ingestion-engine endpoints |
| `src/hooks/useCredentials.ts` | React Query hooks for credentials CRUD |
| `src/hooks/useScanTargets.ts` | React Query hooks for scan targets + discovery tasks |
| `src/components/CreateCredentialModal.tsx` | Modal for creating/editing credentials |
| `src/components/CreateScanTargetModal.tsx` | Modal for creating/editing scan targets |
| `src/components/ScanManagementTab.tsx` | Scan targets list + task history (AutoDiscovery tab 2) |

### Modified Files

| File | Changes |
|------|---------|
| `ingestion-engine/pyproject.toml` | Add pysnmp, asyncssh, pyghmi, cryptography deps |
| `ingestion-engine/app/config.py` | Add `credential_encryption_key` setting |
| `ingestion-engine/app/main.py` | Register 3 new routers |
| `ingestion-engine/app/collectors/__init__.py` | Register 3 collectors |
| `ingestion-engine/app/routes/collectors.py` | Modify test endpoint to accept credential_id |
| `cmdb-core/db/queries/discovery.sql` | Fix FindAssetByIP to use ip_address column |
| `cmdb-demo/vite.config.ts` | Add ingestion proxy |
| `cmdb-demo/src/pages/SystemSettings.tsx` | Add credentials tab |
| `cmdb-demo/src/pages/AutoDiscovery.tsx` | Restructure into 2 tabs |

---

## Task 1: Database Migration

**Files:**
- Create: `cmdb-core/db/migrations/000017_discovery_collectors.up.sql`
- Create: `cmdb-core/db/migrations/000017_discovery_collectors.down.sql`
- Modify: `cmdb-core/db/queries/discovery.sql`

- [ ] **Step 1: Write the up migration**

Create `cmdb-core/db/migrations/000017_discovery_collectors.up.sql`:

```sql
-- Add ip_address column to assets
ALTER TABLE assets ADD COLUMN IF NOT EXISTS ip_address VARCHAR(50);
CREATE INDEX IF NOT EXISTS idx_assets_ip_address ON assets(tenant_id, ip_address);

-- Migrate existing IP data from attributes JSONB
UPDATE assets SET ip_address = attributes->>'ip_address'
WHERE attributes->>'ip_address' IS NOT NULL AND ip_address IS NULL;

-- Credentials table (ingestion-engine manages, but schema lives here for single DB)
CREATE TABLE IF NOT EXISTS credentials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,
    params      BYTEA NOT NULL,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);

-- Scan targets table
CREATE TABLE IF NOT EXISTS scan_targets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(200) NOT NULL,
    cidrs           TEXT[] NOT NULL,
    collector_type  VARCHAR(30) NOT NULL,
    credential_id   UUID NOT NULL REFERENCES credentials(id),
    mode            VARCHAR(20) NOT NULL DEFAULT 'smart',
    location_id     UUID REFERENCES locations(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scan_targets_tenant ON scan_targets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_credentials_tenant ON credentials(tenant_id);
```

- [ ] **Step 2: Write the down migration**

Create `cmdb-core/db/migrations/000017_discovery_collectors.down.sql`:

```sql
DROP TABLE IF EXISTS scan_targets;
DROP TABLE IF EXISTS credentials;
DROP INDEX IF EXISTS idx_assets_ip_address;
ALTER TABLE assets DROP COLUMN IF EXISTS ip_address;
```

- [ ] **Step 3: Fix FindAssetByIP query**

In `cmdb-core/db/queries/discovery.sql`, replace:

```sql
-- name: FindAssetByIP :one
SELECT * FROM assets WHERE tenant_id = $1 AND serial_number = $2 LIMIT 1;
```

With:

```sql
-- name: FindAssetByIP :one
SELECT * FROM assets WHERE tenant_id = $1 AND ip_address = $2 LIMIT 1;
```

- [ ] **Step 4: Run the migration**

```bash
cd /cmdb-platform
psql "$DATABASE_URL" -f cmdb-core/db/migrations/000017_discovery_collectors.up.sql
```

Expected: Tables created, no errors.

- [ ] **Step 5: Verify migration**

```bash
psql "$DATABASE_URL" -c "\d credentials"
psql "$DATABASE_URL" -c "\d scan_targets"
psql "$DATABASE_URL" -c "\d assets" | grep ip_address
```

Expected: All three show correct columns.

- [ ] **Step 6: Regenerate sqlc (if applicable)**

```bash
cd /cmdb-platform/cmdb-core && sqlc generate
```

- [ ] **Step 7: Commit**

```bash
git add cmdb-core/db/migrations/000017_discovery_collectors.up.sql \
       cmdb-core/db/migrations/000017_discovery_collectors.down.sql \
       cmdb-core/db/queries/discovery.sql
git commit -m "feat: add credentials, scan_targets tables + assets.ip_address column"
```

---

## Task 2: Add Python Dependencies

**Files:**
- Modify: `ingestion-engine/pyproject.toml`

- [ ] **Step 1: Add dependencies to pyproject.toml**

In `ingestion-engine/pyproject.toml`, add to `dependencies` list:

```toml
    "pysnmp>=7.0",
    "asyncssh>=2.14",
    "pyghmi>=1.5",
    "cryptography>=42.0",
```

- [ ] **Step 2: Install dependencies**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pip install -e ".[dev]"
```

Expected: All packages install successfully.

- [ ] **Step 3: Verify imports work**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/python -c "import pysnmp; import asyncssh; import pyghmi; import cryptography; print('OK')"
```

Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add ingestion-engine/pyproject.toml
git commit -m "feat: add pysnmp, asyncssh, pyghmi, cryptography dependencies"
```

---

## Task 3: Credential Encryption Module

**Files:**
- Create: `ingestion-engine/app/credentials/__init__.py`
- Create: `ingestion-engine/app/credentials/encryption.py`
- Modify: `ingestion-engine/app/config.py`
- Create: `ingestion-engine/tests/test_encryption.py`

- [ ] **Step 1: Write the failing test**

Create `ingestion-engine/tests/test_encryption.py`:

```python
"""Tests for credential encryption."""

import json
import os

import pytest


def test_encrypt_decrypt_roundtrip():
    """Encrypting then decrypting returns original data."""
    from app.credentials.encryption import decrypt_params, encrypt_params

    original = {"username": "admin", "password": "secret123"}
    key = os.urandom(32)

    encrypted = encrypt_params(original, key)
    assert isinstance(encrypted, bytes)
    assert encrypted != json.dumps(original).encode()

    decrypted = decrypt_params(encrypted, key)
    assert decrypted == original


def test_decrypt_with_wrong_key_fails():
    """Decrypting with wrong key raises ValueError."""
    from app.credentials.encryption import decrypt_params, encrypt_params

    original = {"community": "public"}
    key1 = os.urandom(32)
    key2 = os.urandom(32)

    encrypted = encrypt_params(original, key1)
    with pytest.raises(ValueError, match="decrypt"):
        decrypt_params(encrypted, key2)


def test_encrypt_different_each_time():
    """Each encryption produces different ciphertext (random nonce)."""
    from app.credentials.encryption import encrypt_params

    data = {"username": "test"}
    key = os.urandom(32)

    enc1 = encrypt_params(data, key)
    enc2 = encrypt_params(data, key)
    assert enc1 != enc2
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_encryption.py -v
```

Expected: FAIL — `ModuleNotFoundError: No module named 'app.credentials'`

- [ ] **Step 3: Add credential_encryption_key to config**

In `ingestion-engine/app/config.py`, add to `Settings`:

```python
    credential_encryption_key: str = "0" * 64  # 32 bytes hex, override in production
```

- [ ] **Step 4: Write encryption module**

Create `ingestion-engine/app/credentials/__init__.py` (empty file).

Create `ingestion-engine/app/credentials/encryption.py`:

```python
"""AES-256-GCM encryption for credential params."""

import json
import os

from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def encrypt_params(params: dict, key: bytes) -> bytes:
    """Encrypt a dict as JSON using AES-256-GCM. Returns nonce + ciphertext."""
    nonce = os.urandom(12)
    aesgcm = AESGCM(key)
    plaintext = json.dumps(params).encode("utf-8")
    ciphertext = aesgcm.encrypt(nonce, plaintext, None)
    return nonce + ciphertext


def decrypt_params(data: bytes, key: bytes) -> dict:
    """Decrypt AES-256-GCM encrypted params. Raises ValueError on failure."""
    if len(data) < 12:
        raise ValueError("Invalid encrypted data: too short")
    nonce = data[:12]
    ciphertext = data[12:]
    aesgcm = AESGCM(key)
    try:
        plaintext = aesgcm.decrypt(nonce, ciphertext, None)
    except Exception as e:
        raise ValueError(f"Failed to decrypt credential params: {e}") from e
    return json.loads(plaintext.decode("utf-8"))


def get_key_from_hex(hex_key: str) -> bytes:
    """Convert a 64-char hex string to 32 bytes."""
    return bytes.fromhex(hex_key)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_encryption.py -v
```

Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add ingestion-engine/app/credentials/ ingestion-engine/app/config.py \
       ingestion-engine/tests/test_encryption.py
git commit -m "feat: add AES-256-GCM credential encryption module"
```

---

## Task 4: Credential Provider

**Files:**
- Create: `ingestion-engine/app/credentials/provider.py`

- [ ] **Step 1: Write the credential provider**

Create `ingestion-engine/app/credentials/provider.py`:

```python
"""Database credential provider — load and decrypt credentials."""

from uuid import UUID

import asyncpg

from app.config import settings
from app.credentials.encryption import decrypt_params, get_key_from_hex


class DBCredentialProvider:
    """Reads credentials from DB and decrypts params."""

    def __init__(self, pool: asyncpg.Pool):
        self._pool = pool
        self._key = get_key_from_hex(settings.credential_encryption_key)

    async def get(self, credential_id: UUID) -> dict:
        """Load and decrypt a credential by ID. Returns full row + decrypted params."""
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT id, tenant_id, name, type, params, created_by, created_at, updated_at "
                "FROM credentials WHERE id = $1",
                credential_id,
            )
        if not row:
            raise ValueError(f"Credential {credential_id} not found")

        decrypted = decrypt_params(bytes(row["params"]), self._key)
        return {
            "id": str(row["id"]),
            "tenant_id": str(row["tenant_id"]),
            "name": row["name"],
            "type": row["type"],
            "params": decrypted,
        }
```

- [ ] **Step 2: Commit**

```bash
git add ingestion-engine/app/credentials/provider.py
git commit -m "feat: add DB credential provider with decryption"
```

---

## Task 5: Credentials CRUD API

**Files:**
- Create: `ingestion-engine/app/routes/credentials.py`
- Modify: `ingestion-engine/app/main.py`

- [ ] **Step 1: Write the credentials route**

Create `ingestion-engine/app/routes/credentials.py`:

```python
"""Credentials CRUD routes."""

import uuid
from datetime import datetime, timezone

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.config import settings
from app.credentials.encryption import encrypt_params, get_key_from_hex
from app.dependencies import get_db_pool

router = APIRouter(tags=["credentials"])

VALID_TYPES = {"snmp_v2c", "snmp_v3", "ssh_password", "ssh_key", "ipmi"}


class CreateCredentialRequest(BaseModel):
    tenant_id: str
    name: str
    type: str
    params: dict
    created_by: str | None = None


class UpdateCredentialRequest(BaseModel):
    name: str | None = None
    type: str | None = None
    params: dict | None = None


@router.get("/credentials")
async def list_credentials(
    tenant_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List credentials for a tenant. Params are masked."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT id, tenant_id, name, type, created_by, created_at, updated_at "
            "FROM credentials WHERE tenant_id = $1 ORDER BY created_at DESC",
            uuid.UUID(tenant_id),
        )
    return {"credentials": [dict(r) for r in rows]}


@router.post("/credentials")
async def create_credential(
    body: CreateCredentialRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Create a new credential with encrypted params."""
    if body.type not in VALID_TYPES:
        raise HTTPException(status_code=400, detail=f"Invalid type: {body.type}. Must be one of {VALID_TYPES}")

    key = get_key_from_hex(settings.credential_encryption_key)
    encrypted = encrypt_params(body.params, key)

    cred_id = uuid.uuid4()
    async with pool.acquire() as conn:
        try:
            await conn.execute(
                """INSERT INTO credentials (id, tenant_id, name, type, params, created_by)
                   VALUES ($1, $2, $3, $4, $5, $6)""",
                cred_id,
                uuid.UUID(body.tenant_id),
                body.name,
                body.type,
                encrypted,
                uuid.UUID(body.created_by) if body.created_by else None,
            )
        except asyncpg.UniqueViolationError:
            raise HTTPException(status_code=409, detail=f"Credential name '{body.name}' already exists")

    return {"id": str(cred_id), "name": body.name, "type": body.type}


@router.put("/credentials/{credential_id}")
async def update_credential(
    credential_id: str,
    body: UpdateCredentialRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Update a credential. Omit params to keep existing encrypted params."""
    async with pool.acquire() as conn:
        existing = await conn.fetchrow(
            "SELECT * FROM credentials WHERE id = $1",
            uuid.UUID(credential_id),
        )
    if not existing:
        raise HTTPException(status_code=404, detail="Credential not found")

    updates = []
    values = []
    idx = 1

    if body.name is not None:
        updates.append(f"name = ${idx}")
        values.append(body.name)
        idx += 1

    if body.type is not None:
        if body.type not in VALID_TYPES:
            raise HTTPException(status_code=400, detail=f"Invalid type: {body.type}")
        updates.append(f"type = ${idx}")
        values.append(body.type)
        idx += 1

    if body.params is not None:
        key = get_key_from_hex(settings.credential_encryption_key)
        encrypted = encrypt_params(body.params, key)
        updates.append(f"params = ${idx}")
        values.append(encrypted)
        idx += 1

    if not updates:
        return {"message": "No changes"}

    updates.append(f"updated_at = ${idx}")
    values.append(datetime.now(timezone.utc))
    idx += 1

    values.append(uuid.UUID(credential_id))

    async with pool.acquire() as conn:
        await conn.execute(
            f"UPDATE credentials SET {', '.join(updates)} WHERE id = ${idx}",
            *values,
        )

    return {"id": credential_id, "message": "Updated"}


@router.delete("/credentials/{credential_id}")
async def delete_credential(
    credential_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Delete a credential. Rejects if referenced by scan_targets."""
    cred_uuid = uuid.UUID(credential_id)
    async with pool.acquire() as conn:
        ref_count = await conn.fetchval(
            "SELECT count(*) FROM scan_targets WHERE credential_id = $1",
            cred_uuid,
        )
        if ref_count > 0:
            raise HTTPException(
                status_code=409,
                detail=f"Credential is used by {ref_count} scan target(s). Remove them first.",
            )
        deleted = await conn.execute(
            "DELETE FROM credentials WHERE id = $1", cred_uuid
        )
    if deleted == "DELETE 0":
        raise HTTPException(status_code=404, detail="Credential not found")
    return {"message": "Deleted"}
```

- [ ] **Step 2: Register router in main.py**

In `ingestion-engine/app/main.py`, add import and include:

```python
from app.routes.credentials import router as credentials_router
```

In `create_app()`, add:

```python
    application.include_router(credentials_router)
```

- [ ] **Step 3: Commit**

```bash
git add ingestion-engine/app/routes/credentials.py ingestion-engine/app/main.py
git commit -m "feat: add credentials CRUD API with encryption"
```

---

## Task 6: Scan Targets CRUD API

**Files:**
- Create: `ingestion-engine/app/routes/scan_targets.py`
- Modify: `ingestion-engine/app/main.py`

- [ ] **Step 1: Write the scan targets route**

Create `ingestion-engine/app/routes/scan_targets.py`:

```python
"""Scan targets CRUD routes."""

import uuid
from datetime import datetime, timezone

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.dependencies import get_db_pool

router = APIRouter(tags=["scan-targets"])

VALID_COLLECTOR_TYPES = {"snmp", "ssh", "ipmi"}
VALID_MODES = {"auto", "review", "smart"}


class CreateScanTargetRequest(BaseModel):
    tenant_id: str
    name: str
    cidrs: list[str]
    collector_type: str
    credential_id: str
    mode: str = "smart"


class UpdateScanTargetRequest(BaseModel):
    name: str | None = None
    cidrs: list[str] | None = None
    collector_type: str | None = None
    credential_id: str | None = None
    mode: str | None = None


@router.get("/scan-targets")
async def list_scan_targets(
    tenant_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List scan targets for a tenant, including credential name."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """SELECT st.*, c.name as credential_name
               FROM scan_targets st
               LEFT JOIN credentials c ON c.id = st.credential_id
               WHERE st.tenant_id = $1
               ORDER BY st.created_at DESC""",
            uuid.UUID(tenant_id),
        )
    return {"scan_targets": [dict(r) for r in rows]}


@router.post("/scan-targets")
async def create_scan_target(
    body: CreateScanTargetRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Create a new scan target."""
    if body.collector_type not in VALID_COLLECTOR_TYPES:
        raise HTTPException(status_code=400, detail=f"Invalid collector_type: {body.collector_type}")
    if body.mode not in VALID_MODES:
        raise HTTPException(status_code=400, detail=f"Invalid mode: {body.mode}")
    if not body.cidrs:
        raise HTTPException(status_code=400, detail="cidrs must not be empty")

    # Verify credential exists and matches type
    async with pool.acquire() as conn:
        cred = await conn.fetchrow(
            "SELECT id, type FROM credentials WHERE id = $1",
            uuid.UUID(body.credential_id),
        )
    if not cred:
        raise HTTPException(status_code=404, detail="Credential not found")

    target_id = uuid.uuid4()
    async with pool.acquire() as conn:
        await conn.execute(
            """INSERT INTO scan_targets (id, tenant_id, name, cidrs, collector_type, credential_id, mode)
               VALUES ($1, $2, $3, $4, $5, $6, $7)""",
            target_id,
            uuid.UUID(body.tenant_id),
            body.name,
            body.cidrs,
            body.collector_type,
            uuid.UUID(body.credential_id),
            body.mode,
        )

    return {"id": str(target_id), "name": body.name}


@router.put("/scan-targets/{target_id}")
async def update_scan_target(
    target_id: str,
    body: UpdateScanTargetRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Update a scan target."""
    target_uuid = uuid.UUID(target_id)
    async with pool.acquire() as conn:
        existing = await conn.fetchrow("SELECT * FROM scan_targets WHERE id = $1", target_uuid)
    if not existing:
        raise HTTPException(status_code=404, detail="Scan target not found")

    updates = []
    values = []
    idx = 1

    if body.name is not None:
        updates.append(f"name = ${idx}")
        values.append(body.name)
        idx += 1
    if body.cidrs is not None:
        if not body.cidrs:
            raise HTTPException(status_code=400, detail="cidrs must not be empty")
        updates.append(f"cidrs = ${idx}")
        values.append(body.cidrs)
        idx += 1
    if body.collector_type is not None:
        if body.collector_type not in VALID_COLLECTOR_TYPES:
            raise HTTPException(status_code=400, detail=f"Invalid collector_type: {body.collector_type}")
        updates.append(f"collector_type = ${idx}")
        values.append(body.collector_type)
        idx += 1
    if body.credential_id is not None:
        updates.append(f"credential_id = ${idx}")
        values.append(uuid.UUID(body.credential_id))
        idx += 1
    if body.mode is not None:
        if body.mode not in VALID_MODES:
            raise HTTPException(status_code=400, detail=f"Invalid mode: {body.mode}")
        updates.append(f"mode = ${idx}")
        values.append(body.mode)
        idx += 1

    if not updates:
        return {"message": "No changes"}

    updates.append(f"updated_at = ${idx}")
    values.append(datetime.now(timezone.utc))
    idx += 1
    values.append(target_uuid)

    async with pool.acquire() as conn:
        await conn.execute(
            f"UPDATE scan_targets SET {', '.join(updates)} WHERE id = ${idx}",
            *values,
        )

    return {"id": target_id, "message": "Updated"}


@router.delete("/scan-targets/{target_id}")
async def delete_scan_target(
    target_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Delete a scan target."""
    result = None
    async with pool.acquire() as conn:
        result = await conn.execute(
            "DELETE FROM scan_targets WHERE id = $1", uuid.UUID(target_id)
        )
    if result == "DELETE 0":
        raise HTTPException(status_code=404, detail="Scan target not found")
    return {"message": "Deleted"}
```

- [ ] **Step 2: Register router in main.py**

In `ingestion-engine/app/main.py`, add:

```python
from app.routes.scan_targets import router as scan_targets_router
```

In `create_app()`:

```python
    application.include_router(scan_targets_router)
```

- [ ] **Step 3: Commit**

```bash
git add ingestion-engine/app/routes/scan_targets.py ingestion-engine/app/main.py
git commit -m "feat: add scan targets CRUD API"
```

---

## Task 7: SNMP Collector

**Files:**
- Create: `ingestion-engine/app/collectors/snmp.py`
- Create: `ingestion-engine/tests/test_snmp_collector.py`

- [ ] **Step 1: Write the failing test**

Create `ingestion-engine/tests/test_snmp_collector.py`:

```python
"""Tests for SNMP collector."""

from unittest.mock import AsyncMock, patch

import pytest

from app.models.common import CollectTarget, RawAssetData


@pytest.fixture
def snmp_target():
    return CollectTarget(
        endpoint="192.168.1.1",
        credentials={"community": "public", "version": "2c"},
        options={"port": 161, "timeout": 5},
    )


def test_snmp_collector_implements_protocol():
    """SNMPCollector satisfies the Collector protocol."""
    from app.collectors.base import Collector
    from app.collectors.snmp import SNMPCollector

    collector = SNMPCollector()
    assert isinstance(collector, Collector)
    assert collector.name == "snmp"
    assert collector.collect_type == "snmp"


def test_snmp_supported_fields():
    """SNMPCollector declares expected supported fields."""
    from app.collectors.snmp import SNMPCollector

    collector = SNMPCollector()
    field_names = [f.field_name for f in collector.supported_fields()]
    assert "serial_number" in field_names
    assert "vendor" in field_names
    assert "model" in field_names
    assert "name" in field_names


def test_snmp_vendor_detection():
    """sysObjectID prefix maps to correct vendor."""
    from app.collectors.snmp import detect_vendor

    assert detect_vendor("1.3.6.1.4.1.9.1.2066") == "Cisco"
    assert detect_vendor("1.3.6.1.4.1.2011.2.999") == "Huawei"
    assert detect_vendor("1.3.6.1.4.1.2636.1.1.1") == "Juniper"
    assert detect_vendor("1.3.6.1.4.1.11.2.3.7") == "HP"
    assert detect_vendor("1.3.6.1.4.1.674.999") == "Dell"
    assert detect_vendor("1.3.6.1.4.1.99999.1") == "Unknown"


def test_snmp_parse_sysdescr():
    """sysDescr parsing extracts model/os info."""
    from app.collectors.snmp import parse_sysdescr

    result = parse_sysdescr("Cisco IOS Software, C2960 Software (C2960-LANBASEK9-M), Version 15.0(2)SE")
    assert "C2960" in result.get("model", "")


def test_snmp_expand_cidrs():
    """CIDR expansion produces correct IP list."""
    from app.collectors.snmp import expand_cidrs

    ips = expand_cidrs(["192.168.1.0/30"])
    # /30 = 4 addresses, 2 usable hosts (exclude network + broadcast)
    assert "192.168.1.1" in ips
    assert "192.168.1.2" in ips
    assert "192.168.1.0" not in ips  # network address
    assert "192.168.1.3" not in ips  # broadcast address
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_snmp_collector.py -v
```

Expected: FAIL — `ModuleNotFoundError: No module named 'app.collectors.snmp'`

- [ ] **Step 3: Write SNMP collector**

Create `ingestion-engine/app/collectors/snmp.py`:

```python
"""SNMP collector — queries network devices via SNMP v2c/v3."""

import asyncio
import ipaddress
import logging
from datetime import datetime, timezone

from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData

logger = logging.getLogger(__name__)

# Standard MIB-2 OIDs
OID_SYS_DESCR = "1.3.6.1.2.1.1.1.0"
OID_SYS_OBJECT_ID = "1.3.6.1.2.1.1.2.0"
OID_SYS_NAME = "1.3.6.1.2.1.1.5.0"

# Standard ENTITY-MIB serial number (preferred)
OID_ENT_SERIAL = "1.3.6.1.2.1.47.1.1.1.1.11"

# Vendor-specific serial number fallbacks
VENDOR_SERIAL_OIDS = {
    "HP": "1.3.6.1.4.1.11.2.36.1.1.2.9.0",
    "Dell": "1.3.6.1.4.1.674.10895.3000.1.2.100.8.1.4.1",
    "Huawei": "1.3.6.1.4.1.2011.5.25.188.1.1",
    "Juniper": "1.3.6.1.4.1.2636.3.1.3.0",
}

# sysObjectID prefix → vendor name
VENDOR_OID_PREFIXES = {
    "1.3.6.1.4.1.9": "Cisco",
    "1.3.6.1.4.1.11": "HP",
    "1.3.6.1.4.1.674": "Dell",
    "1.3.6.1.4.1.2011": "Huawei",
    "1.3.6.1.4.1.2636": "Juniper",
}


def detect_vendor(sys_object_id: str) -> str:
    """Map sysObjectID to vendor name by longest prefix match."""
    for prefix, vendor in sorted(VENDOR_OID_PREFIXES.items(), key=lambda x: -len(x[0])):
        if sys_object_id.startswith(prefix):
            return vendor
    return "Unknown"


def parse_sysdescr(descr: str) -> dict:
    """Extract model and OS info from sysDescr string."""
    result = {}
    if descr:
        result["os_version"] = descr[:200]
        # Try to extract model from common patterns like "C2960", "Catalyst 9300"
        parts = descr.split(",")
        if len(parts) >= 2:
            result["model"] = parts[1].strip().split("Software")[0].strip()
            if not result["model"]:
                result["model"] = parts[0].strip()[:100]
        else:
            result["model"] = descr[:100]
    return result


def expand_cidrs(cidrs: list[str]) -> list[str]:
    """Expand a list of CIDR strings to individual host IP strings."""
    ips = []
    for cidr in cidrs:
        try:
            network = ipaddress.ip_network(cidr, strict=False)
            if network.prefixlen == 32:
                ips.append(str(network.network_address))
            else:
                ips.extend(str(ip) for ip in network.hosts())
        except ValueError:
            logger.warning("Invalid CIDR: %s, skipping", cidr)
    return ips


async def _snmp_get(ip: str, oids: list[str], credentials: dict, options: dict) -> dict[str, str]:
    """Perform SNMP GET for a list of OIDs on a single host. Returns {oid: value}."""
    from pysnmp.hlapi.asyncio import (
        CommunityData,
        ContextData,
        ObjectIdentity,
        ObjectType,
        SnmpEngine,
        UdpTransportTarget,
        getCmd,
    )

    version = credentials.get("version", "2c")
    community = credentials.get("community", "public")
    port = options.get("port", 161)
    timeout_s = options.get("timeout", 5)
    retries = options.get("retries", 1)

    if version in ("1", "2c"):
        auth_data = CommunityData(community, mpModel=0 if version == "1" else 1)
    else:
        # v3 support can be extended here
        from pysnmp.hlapi.asyncio import UsmUserData
        auth_data = UsmUserData(
            credentials.get("username", ""),
            credentials.get("auth_pass", ""),
            credentials.get("priv_pass", ""),
        )

    transport = UdpTransportTarget((ip, port), timeout=timeout_s, retries=retries)
    engine = SnmpEngine()

    results = {}
    for oid in oids:
        try:
            error_indication, error_status, _error_index, var_binds = await getCmd(
                engine,
                auth_data,
                transport,
                ContextData(),
                ObjectType(ObjectIdentity(oid)),
            )
            if error_indication or error_status:
                continue
            for _oid, val in var_binds:
                val_str = str(val)
                if val_str and val_str != "No Such Object" and val_str != "No Such Instance":
                    results[oid] = val_str
        except Exception as e:
            logger.debug("SNMP GET %s on %s failed: %s", oid, ip, e)
    return results


async def _collect_single(ip: str, credentials: dict, options: dict) -> RawAssetData | None:
    """Collect asset data from a single IP via SNMP."""
    # Step 1: Probe with sysDescr
    probe = await _snmp_get(ip, [OID_SYS_DESCR], credentials, options)
    if not probe:
        return None

    # Step 2: Full collection
    oids = [OID_SYS_DESCR, OID_SYS_OBJECT_ID, OID_SYS_NAME]
    data = await _snmp_get(ip, oids, credentials, options)
    if not data:
        return None

    # Detect vendor
    sys_object_id = data.get(OID_SYS_OBJECT_ID, "")
    vendor = detect_vendor(sys_object_id)

    # Parse sysDescr
    descr_info = parse_sysdescr(data.get(OID_SYS_DESCR, ""))

    # Try ENTITY-MIB serial first
    serial_data = await _snmp_get(ip, [OID_ENT_SERIAL], credentials, options)
    serial_number = serial_data.get(OID_ENT_SERIAL)

    # Fallback to vendor-specific OID
    if not serial_number and vendor in VENDOR_SERIAL_OIDS:
        fallback_data = await _snmp_get(ip, [VENDOR_SERIAL_OIDS[vendor]], credentials, options)
        serial_number = fallback_data.get(VENDOR_SERIAL_OIDS[vendor])

    hostname = data.get(OID_SYS_NAME, "")

    fields = {
        "name": hostname or ip,
        "vendor": vendor if vendor != "Unknown" else None,
        "model": descr_info.get("model"),
        "serial_number": serial_number,
    }
    # Remove None values
    fields = {k: v for k, v in fields.items() if v is not None}

    attributes = {
        "ip_address": ip,
        "sys_object_id": sys_object_id,
    }
    if descr_info.get("os_version"):
        attributes["os_version"] = descr_info["os_version"]

    return RawAssetData(
        source="snmp",
        unique_key=serial_number or ip,
        fields=fields,
        attributes=attributes,
        collected_at=datetime.now(timezone.utc),
    )


class SNMPCollector:
    """SNMP collector for network devices."""

    name = "snmp"
    collect_type = "snmp"

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        """Collect asset data from target endpoint(s) via SNMP."""
        credentials = target.credentials or {}
        options = target.options or {}
        concurrency = options.get("concurrency", 50)

        # Expand CIDRs if the endpoint looks like a CIDR
        ips = expand_cidrs([target.endpoint])

        results: list[RawAssetData] = []
        semaphore = asyncio.Semaphore(concurrency)

        async def _scan(ip: str):
            async with semaphore:
                asset = await _collect_single(ip, credentials, options)
                if asset:
                    results.append(asset)

        await asyncio.gather(*[_scan(ip) for ip in ips], return_exceptions=True)
        return results

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        """Test SNMP connectivity to the target."""
        credentials = target.credentials or {}
        options = target.options or {}
        ip = target.endpoint.split("/")[0]

        try:
            import time
            start = time.monotonic()
            data = await _snmp_get(ip, [OID_SYS_DESCR], credentials, options)
            latency = (time.monotonic() - start) * 1000

            if data:
                return ConnectionResult(
                    success=True,
                    message=f"SNMP OK: {data.get(OID_SYS_DESCR, 'responded')[:100]}",
                    latency_ms=round(latency, 1),
                )
            return ConnectionResult(success=False, message="No SNMP response")
        except Exception as e:
            return ConnectionResult(success=False, message=str(e)[:200])

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="serial_number", authority=True),
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
            FieldMapping(field_name="name", authority=False),
        ]
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_snmp_collector.py -v
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/collectors/snmp.py ingestion-engine/tests/test_snmp_collector.py
git commit -m "feat: implement SNMP collector with vendor detection and OID queries"
```

---

## Task 8: SSH Collector

**Files:**
- Create: `ingestion-engine/app/collectors/ssh.py`
- Create: `ingestion-engine/tests/test_ssh_collector.py`

- [ ] **Step 1: Write the failing test**

Create `ingestion-engine/tests/test_ssh_collector.py`:

```python
"""Tests for SSH collector."""

import pytest

from app.models.common import CollectTarget


@pytest.fixture
def ssh_target():
    return CollectTarget(
        endpoint="192.168.1.10",
        credentials={"username": "root", "password": "secret"},
        options={"port": 22, "timeout": 10},
    )


def test_ssh_collector_implements_protocol():
    """SSHCollector satisfies the Collector protocol."""
    from app.collectors.base import Collector
    from app.collectors.ssh import SSHCollector

    collector = SSHCollector()
    assert isinstance(collector, Collector)
    assert collector.name == "ssh"
    assert collector.collect_type == "ssh"


def test_ssh_supported_fields():
    """SSHCollector declares expected supported fields."""
    from app.collectors.ssh import SSHCollector

    collector = SSHCollector()
    field_names = [f.field_name for f in collector.supported_fields()]
    assert "serial_number" in field_names
    assert "vendor" in field_names
    assert "model" in field_names
    assert "name" in field_names


def test_parse_os_release():
    """Parsing /etc/os-release extracts ID and VERSION_ID."""
    from app.collectors.ssh import parse_os_release

    content = 'NAME="Ubuntu"\nVERSION="22.04.3 LTS"\nID=ubuntu\nVERSION_ID="22.04"\n'
    result = parse_os_release(content)
    assert result["os_type"] == "ubuntu"
    assert result["os_version"] == "22.04"


def test_parse_os_release_centos():
    """Parsing CentOS /etc/os-release."""
    from app.collectors.ssh import parse_os_release

    content = 'NAME="CentOS Linux"\nVERSION="7 (Core)"\nID="centos"\nVERSION_ID="7"\n'
    result = parse_os_release(content)
    assert result["os_type"] == "centos"
    assert result["os_version"] == "7"


def test_parse_os_release_empty():
    """Empty string returns empty dict."""
    from app.collectors.ssh import parse_os_release

    assert parse_os_release("") == {}
    assert parse_os_release("GARBAGE\nDATA\n") == {}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_ssh_collector.py -v
```

Expected: FAIL — `ModuleNotFoundError: No module named 'app.collectors.ssh'`

- [ ] **Step 3: Write SSH collector**

Create `ingestion-engine/app/collectors/ssh.py`:

```python
"""SSH collector — connects to Linux/Unix servers, runs commands, parses output."""

import asyncio
import logging
from datetime import datetime, timezone

import asyncssh

from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData

logger = logging.getLogger(__name__)

# Commands to collect system information
COLLECT_COMMANDS = {
    "hostname": "hostname",
    "serial_number": "dmidecode -s system-serial-number 2>/dev/null || echo ''",
    "vendor": "dmidecode -s system-manufacturer 2>/dev/null || echo ''",
    "model": "dmidecode -s system-product-name 2>/dev/null || echo ''",
    "os_release": "cat /etc/os-release 2>/dev/null || echo ''",
    "cpu_cores": "nproc 2>/dev/null || echo ''",
    "memory_mb": "free -m 2>/dev/null | awk '/Mem:/{print $2}' || echo ''",
    "disk_gb": "lsblk -dbn -o SIZE 2>/dev/null | awk '{s+=$1}END{print int(s/1073741824)}' || echo ''",
    "ip_addr": "ip -4 addr show 2>/dev/null | grep 'inet ' | grep -v '127.0.0.1' | awk '{print $2}' | head -1 || echo ''",
}


def parse_os_release(content: str) -> dict:
    """Parse /etc/os-release content into {os_type, os_version}."""
    result = {}
    for line in content.strip().splitlines():
        line = line.strip()
        if line.startswith("ID="):
            result["os_type"] = line.split("=", 1)[1].strip('"').strip()
        elif line.startswith("VERSION_ID="):
            result["os_version"] = line.split("=", 1)[1].strip('"').strip()
    return result


async def _connect_and_collect(ip: str, credentials: dict, options: dict) -> RawAssetData | None:
    """SSH into a host, run commands, and return RawAssetData."""
    port = options.get("port", 22)
    timeout = options.get("timeout", 10)
    username = credentials.get("username", "root")
    password = credentials.get("password")
    private_key = credentials.get("private_key")
    passphrase = credentials.get("passphrase")

    connect_kwargs = {
        "host": ip,
        "port": port,
        "username": username,
        "known_hosts": None,  # Accept any host key for automated scanning
        "login_timeout": timeout,
    }

    if private_key:
        connect_kwargs["client_keys"] = [asyncssh.import_private_key(private_key, passphrase)]
    elif password:
        connect_kwargs["password"] = password

    try:
        async with asyncssh.connect(**connect_kwargs) as conn:
            collected = {}
            for key, cmd in COLLECT_COMMANDS.items():
                try:
                    result = await asyncio.wait_for(conn.run(cmd, check=False), timeout=timeout)
                    collected[key] = result.stdout.strip() if result.stdout else ""
                except asyncio.TimeoutError:
                    collected[key] = ""
                except Exception as e:
                    logger.debug("Command '%s' failed on %s: %s", cmd, ip, e)
                    collected[key] = ""

            # Parse results
            hostname = collected.get("hostname", "") or ip
            serial_number = collected.get("serial_number", "")
            vendor = collected.get("vendor", "")
            model = collected.get("model", "")
            os_info = parse_os_release(collected.get("os_release", ""))

            # Clean up empty/placeholder values from dmidecode
            for val_name in ("serial_number", "vendor", "model"):
                val = collected.get(val_name, "")
                if val.lower() in ("", "not specified", "to be filled by o.e.m.", "default string", "none"):
                    collected[val_name] = ""

            serial_number = collected["serial_number"]
            vendor = collected["vendor"]
            model = collected["model"]

            ip_addr = collected.get("ip_addr", "").split("/")[0]  # Remove CIDR suffix

            fields = {
                "name": hostname,
                "serial_number": serial_number or None,
                "vendor": vendor or None,
                "model": model or None,
            }
            fields = {k: v for k, v in fields.items() if v is not None}

            attributes = {
                "ip_address": ip_addr or ip,
            }
            if os_info.get("os_type"):
                attributes["os_type"] = os_info["os_type"]
            if os_info.get("os_version"):
                attributes["os_version"] = os_info["os_version"]
            cpu = collected.get("cpu_cores", "")
            if cpu:
                attributes["cpu_cores"] = cpu
            mem = collected.get("memory_mb", "")
            if mem:
                attributes["memory_mb"] = mem
            disk = collected.get("disk_gb", "")
            if disk and disk != "0":
                attributes["disk_gb"] = disk

            return RawAssetData(
                source="ssh",
                unique_key=serial_number or hostname or ip,
                fields=fields,
                attributes=attributes,
                collected_at=datetime.now(timezone.utc),
            )

    except Exception as e:
        logger.warning("SSH collection failed for %s: %s", ip, e)
        return None


class SSHCollector:
    """SSH collector for Linux/Unix servers."""

    name = "ssh"
    collect_type = "ssh"

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        """Collect asset data from target via SSH."""
        credentials = target.credentials or {}
        options = target.options or {}
        concurrency = options.get("concurrency", 10)

        from app.collectors.snmp import expand_cidrs
        ips = expand_cidrs([target.endpoint])

        results: list[RawAssetData] = []
        semaphore = asyncio.Semaphore(concurrency)

        async def _scan(ip: str):
            async with semaphore:
                asset = await _connect_and_collect(ip, credentials, options)
                if asset:
                    results.append(asset)

        await asyncio.gather(*[_scan(ip) for ip in ips], return_exceptions=True)
        return results

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        """Test SSH connectivity."""
        credentials = target.credentials or {}
        options = target.options or {}
        ip = target.endpoint.split("/")[0]

        try:
            import time
            start = time.monotonic()

            port = options.get("port", 22)
            username = credentials.get("username", "root")
            password = credentials.get("password")
            private_key = credentials.get("private_key")
            passphrase = credentials.get("passphrase")

            connect_kwargs = {
                "host": ip, "port": port, "username": username,
                "known_hosts": None, "login_timeout": options.get("timeout", 10),
            }
            if private_key:
                connect_kwargs["client_keys"] = [asyncssh.import_private_key(private_key, passphrase)]
            elif password:
                connect_kwargs["password"] = password

            async with asyncssh.connect(**connect_kwargs) as conn:
                result = await conn.run("hostname", check=False)
                latency = (time.monotonic() - start) * 1000
                hostname = result.stdout.strip() if result.stdout else "unknown"
                return ConnectionResult(
                    success=True,
                    message=f"SSH OK: {hostname}",
                    latency_ms=round(latency, 1),
                )
        except Exception as e:
            return ConnectionResult(success=False, message=str(e)[:200])

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="serial_number", authority=True),
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
            FieldMapping(field_name="name", authority=False),
        ]
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_ssh_collector.py -v
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/collectors/ssh.py ingestion-engine/tests/test_ssh_collector.py
git commit -m "feat: implement SSH collector with command parsing"
```

---

## Task 9: IPMI Collector

**Files:**
- Create: `ingestion-engine/app/collectors/ipmi.py`
- Create: `ingestion-engine/tests/test_ipmi_collector.py`

- [ ] **Step 1: Write the failing test**

Create `ingestion-engine/tests/test_ipmi_collector.py`:

```python
"""Tests for IPMI collector."""

import pytest

from app.models.common import CollectTarget


def test_ipmi_collector_implements_protocol():
    """IPMICollector satisfies the Collector protocol."""
    from app.collectors.base import Collector
    from app.collectors.ipmi import IPMICollector

    collector = IPMICollector()
    assert isinstance(collector, Collector)
    assert collector.name == "ipmi"
    assert collector.collect_type == "ipmi"


def test_ipmi_supported_fields():
    """IPMICollector declares expected supported fields."""
    from app.collectors.ipmi import IPMICollector

    collector = IPMICollector()
    field_names = [f.field_name for f in collector.supported_fields()]
    assert "serial_number" in field_names
    assert "vendor" in field_names
    assert "model" in field_names


def test_parse_fru_inventory():
    """FRU inventory dict is correctly parsed into fields and attributes."""
    from app.collectors.ipmi import parse_fru_inventory

    fru = {
        "product_serial": "SN12345",
        "product_manufacturer": "Dell Inc.",
        "product_name": "PowerEdge R740",
        "product_part_number": "P/N-001",
    }
    fields, attrs = parse_fru_inventory(fru)
    assert fields["serial_number"] == "SN12345"
    assert fields["vendor"] == "Dell Inc."
    assert fields["model"] == "PowerEdge R740"
    assert attrs["part_number"] == "P/N-001"


def test_parse_fru_inventory_missing_fields():
    """Missing FRU fields return empty dict entries gracefully."""
    from app.collectors.ipmi import parse_fru_inventory

    fru = {"product_name": "Unknown Server"}
    fields, attrs = parse_fru_inventory(fru)
    assert "serial_number" not in fields
    assert "vendor" not in fields
    assert fields["model"] == "Unknown Server"
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_ipmi_collector.py -v
```

Expected: FAIL — `ModuleNotFoundError: No module named 'app.collectors.ipmi'`

- [ ] **Step 3: Write IPMI collector**

Create `ingestion-engine/app/collectors/ipmi.py`:

```python
"""IPMI collector — connects to BMC via pyghmi, reads FRU and sensor data."""

import asyncio
import logging
from datetime import datetime, timezone

from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData

logger = logging.getLogger(__name__)


def parse_fru_inventory(fru: dict) -> tuple[dict, dict]:
    """Parse pyghmi FRU inventory dict into (fields, attributes).

    Args:
        fru: Dict from pyghmi get_inventory(), typically has keys like
             product_serial, product_manufacturer, product_name, product_part_number.

    Returns:
        Tuple of (fields dict for RawAssetData.fields, attributes dict).
    """
    fields = {}
    attributes = {}

    serial = fru.get("product_serial") or fru.get("serial_number") or fru.get("Serial Number")
    if serial and serial.strip():
        fields["serial_number"] = serial.strip()

    vendor = fru.get("product_manufacturer") or fru.get("manufacturer") or fru.get("Manufacturer")
    if vendor and vendor.strip():
        fields["vendor"] = vendor.strip()

    model = fru.get("product_name") or fru.get("Product Name")
    if model and model.strip():
        fields["model"] = model.strip()

    part = fru.get("product_part_number") or fru.get("Part Number")
    if part and part.strip():
        attributes["part_number"] = part.strip()

    return fields, attributes


def _collect_single_sync(ip: str, credentials: dict, options: dict) -> RawAssetData | None:
    """Synchronous IPMI collection for a single BMC. Called via asyncio.to_thread."""
    from pyghmi.ipmi.command import Command

    username = credentials.get("username", "admin")
    password = credentials.get("password", "")
    port = options.get("port", 623)

    try:
        conn = Command(bmc=ip, userid=username, password=password, port=port)

        # FRU inventory
        fru = {}
        try:
            inventory = conn.get_inventory()
            if isinstance(inventory, dict):
                fru = inventory
            elif isinstance(inventory, list) and len(inventory) > 0:
                fru = inventory[0] if isinstance(inventory[0], dict) else {}
        except Exception as e:
            logger.debug("FRU read failed on %s: %s", ip, e)

        fields, attributes = parse_fru_inventory(fru)
        attributes["ip_address"] = ip

        # Chassis status
        try:
            chassis = conn.get_chassis_status()
            if isinstance(chassis, dict):
                attributes["power_state"] = "on" if chassis.get("power_on") else "off"
        except Exception as e:
            logger.debug("Chassis status failed on %s: %s", ip, e)

        # BMC network config
        try:
            net = conn.get_net_configuration()
            if isinstance(net, dict):
                if net.get("ipv4_address"):
                    attributes["bmc_ip"] = str(net["ipv4_address"])
                if net.get("mac_address"):
                    attributes["bmc_mac"] = str(net["mac_address"])
        except Exception as e:
            logger.debug("Net config failed on %s: %s", ip, e)

        # Sensor data (store summary)
        try:
            sensors = conn.get_sensor_data()
            if sensors:
                sensor_summary = {}
                for name, data in sensors.items():
                    if hasattr(data, "value") and data.value is not None:
                        sensor_summary[name] = str(data.value)
                if sensor_summary:
                    attributes["sensors"] = sensor_summary
        except Exception as e:
            logger.debug("Sensor read failed on %s: %s", ip, e)

        # Firmware version
        try:
            fw = conn.get_firmware()
            if isinstance(fw, list) and len(fw) > 0:
                attributes["firmware_version"] = str(fw[0].get("version", ""))
            elif isinstance(fw, dict):
                attributes["firmware_version"] = str(fw.get("version", ""))
        except Exception as e:
            logger.debug("Firmware read failed on %s: %s", ip, e)

        if not fields.get("serial_number"):
            fields["name"] = fields.get("name") or ip

        return RawAssetData(
            source="ipmi",
            unique_key=fields.get("serial_number") or ip,
            fields={k: v for k, v in fields.items() if v},
            attributes=attributes,
            collected_at=datetime.now(timezone.utc),
        )

    except Exception as e:
        logger.warning("IPMI collection failed for %s: %s", ip, e)
        return None


class IPMICollector:
    """IPMI collector for server BMC (iDRAC, iLO, Supermicro)."""

    name = "ipmi"
    collect_type = "ipmi"

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        """Collect asset data from target BMC(s) via IPMI."""
        credentials = target.credentials or {}
        options = target.options or {}
        concurrency = options.get("concurrency", 20)

        from app.collectors.snmp import expand_cidrs
        ips = expand_cidrs([target.endpoint])

        results: list[RawAssetData] = []
        semaphore = asyncio.Semaphore(concurrency)

        async def _scan(ip: str):
            async with semaphore:
                asset = await asyncio.to_thread(_collect_single_sync, ip, credentials, options)
                if asset:
                    results.append(asset)

        await asyncio.gather(*[_scan(ip) for ip in ips], return_exceptions=True)
        return results

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        """Test IPMI connectivity to the target BMC."""
        credentials = target.credentials or {}
        options = target.options or {}
        ip = target.endpoint.split("/")[0]

        try:
            import time
            start = time.monotonic()

            def _test_sync():
                from pyghmi.ipmi.command import Command
                conn = Command(
                    bmc=ip,
                    userid=credentials.get("username", "admin"),
                    password=credentials.get("password", ""),
                    port=options.get("port", 623),
                )
                return conn.get_chassis_status()

            result = await asyncio.to_thread(_test_sync)
            latency = (time.monotonic() - start) * 1000
            power = "on" if isinstance(result, dict) and result.get("power_on") else "unknown"
            return ConnectionResult(
                success=True,
                message=f"IPMI OK: power={power}",
                latency_ms=round(latency, 1),
            )
        except Exception as e:
            return ConnectionResult(success=False, message=str(e)[:200])

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="serial_number", authority=True),
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
        ]
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_ipmi_collector.py -v
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/collectors/ipmi.py ingestion-engine/tests/test_ipmi_collector.py
git commit -m "feat: implement IPMI collector with FRU/sensor parsing"
```

---

## Task 10: Register Collectors

**Files:**
- Modify: `ingestion-engine/app/collectors/__init__.py`

- [ ] **Step 1: Register all three collectors**

Replace `ingestion-engine/app/collectors/__init__.py` with:

```python
"""Register all collectors on import."""

from app.collectors.base import registry
from app.collectors.ipmi import IPMICollector
from app.collectors.snmp import SNMPCollector
from app.collectors.ssh import SSHCollector

registry.register(SNMPCollector())
registry.register(SSHCollector())
registry.register(IPMICollector())
```

- [ ] **Step 2: Verify registration**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/python -c "
from app.collectors import registry
print([c['name'] for c in registry.list_all()])
"
```

Expected: `['snmp', 'ssh', 'ipmi']`

- [ ] **Step 3: Commit**

```bash
git add ingestion-engine/app/collectors/__init__.py
git commit -m "feat: register SNMP, SSH, IPMI collectors in registry"
```

---

## Task 11: Discovery Celery Task + Mode Routing

**Files:**
- Create: `ingestion-engine/app/tasks/discovery_task.py`
- Create: `ingestion-engine/tests/test_discovery_task.py`

- [ ] **Step 1: Write the failing test for mode routing**

Create `ingestion-engine/tests/test_discovery_task.py`:

```python
"""Tests for discovery task mode routing logic."""

from app.models.common import RawAssetData


def test_route_auto_returns_pipeline():
    """Mode 'auto' routes all assets to pipeline."""
    from app.tasks.discovery_task import determine_routing

    raw = RawAssetData(source="snmp", unique_key="SN1", fields={"serial_number": "SN1"})
    action = determine_routing("auto", raw, existing_asset_id=None)
    assert action == "pipeline"


def test_route_review_returns_staging():
    """Mode 'review' routes all assets to staging."""
    from app.tasks.discovery_task import determine_routing

    raw = RawAssetData(source="snmp", unique_key="SN1", fields={"serial_number": "SN1"})
    action = determine_routing("review", raw, existing_asset_id=None)
    assert action == "staging"


def test_route_smart_new_asset_returns_staging():
    """Mode 'smart' routes new (unmatched) assets to staging."""
    from app.tasks.discovery_task import determine_routing

    raw = RawAssetData(source="snmp", unique_key="SN1", fields={"serial_number": "SN1"})
    action = determine_routing("smart", raw, existing_asset_id=None)
    assert action == "staging"


def test_route_smart_existing_asset_returns_pipeline():
    """Mode 'smart' routes matched assets to pipeline."""
    from app.tasks.discovery_task import determine_routing

    raw = RawAssetData(source="snmp", unique_key="SN1", fields={"serial_number": "SN1"})
    action = determine_routing("smart", raw, existing_asset_id="some-uuid")
    assert action == "pipeline"
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_discovery_task.py -v
```

Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Write the discovery task**

Create `ingestion-engine/app/tasks/discovery_task.py`:

```python
"""Celery task for discovery scanning with mode-based result routing."""

import asyncio
import json
import logging
from datetime import datetime, timezone
from uuid import UUID

import asyncpg
import httpx

from app.collectors.base import registry
from app.config import settings
from app.credentials.provider import DBCredentialProvider
from app.events import connect_nats, publish_event, close_nats
from app.models.common import CollectTarget, RawAssetData
from app.pipeline.deduplicate import deduplicate
from app.pipeline.processor import process_single
from app.tasks.celery_app import celery_app

logger = logging.getLogger(__name__)

# cmdb-core URL for discovery ingest (review/smart mode)
CMDB_CORE_URL = "http://localhost:8080/api/v1"


def determine_routing(mode: str, raw: RawAssetData, existing_asset_id: str | None) -> str:
    """Determine where to route a collected asset based on mode.

    Returns 'pipeline' (process_batch) or 'staging' (POST /discovery/ingest).
    """
    if mode == "auto":
        return "pipeline"
    elif mode == "review":
        return "staging"
    else:  # smart
        return "pipeline" if existing_asset_id else "staging"


def _run_async(coro):
    """Create a new event loop, run the coroutine, and close the loop."""
    loop = asyncio.new_event_loop()
    try:
        return loop.run_until_complete(coro)
    finally:
        loop.close()


@celery_app.task(bind=True, name="ingestion.process_discovery")
def process_discovery_task(
    self,
    task_id: str,
    tenant_id: str,
    collector_type: str,
    cidrs: list[str],
    credential_id: str,
    mode: str,
):
    """Celery task that runs a discovery scan."""
    return _run_async(
        _process_discovery(self, task_id, tenant_id, collector_type, cidrs, credential_id, mode)
    )


async def _process_discovery(
    task,
    task_id: str,
    tenant_id: str,
    collector_type: str,
    cidrs: list[str],
    credential_id: str,
    mode: str,
):
    """Async implementation of discovery scanning."""
    pool = await asyncpg.create_pool(settings.database_url, min_size=2, max_size=10)
    nc = await connect_nats(settings.nats_url)

    stats = {
        "total_ips": 0,
        "responded": 0,
        "created": 0,
        "updated": 0,
        "conflicts": 0,
        "skipped": 0,
        "staging": 0,
        "errors": 0,
    }

    try:
        # Update task status to running
        await _update_task_status(pool, task_id, "running")

        # Load credential
        provider = DBCredentialProvider(pool)
        cred = await provider.get(UUID(credential_id))

        # Get collector
        collector = registry.get(collector_type)
        if not collector:
            raise ValueError(f"Collector '{collector_type}' not found in registry")

        # Collect from each CIDR
        all_assets: list[RawAssetData] = []
        for cidr in cidrs:
            target = CollectTarget(
                endpoint=cidr,
                credentials=cred["params"],
                options={},
            )
            try:
                assets = await collector.collect(target)
                all_assets.extend(assets)
            except Exception as e:
                logger.error("Collection failed for CIDR %s: %s", cidr, e)
                stats["errors"] += 1

        stats["responded"] = len(all_assets)

        # Route each asset by mode
        for raw in all_assets:
            try:
                # Check if asset exists (for smart mode)
                dedup_result = await deduplicate(pool, UUID(tenant_id), raw)
                existing_id = str(dedup_result.existing_asset_id) if not dedup_result.is_new else None

                routing = determine_routing(mode, raw, existing_id)

                if routing == "pipeline":
                    result = await process_single(pool, UUID(tenant_id), raw)
                    if result.action == "created":
                        stats["created"] += 1
                    elif result.action == "updated":
                        stats["updated"] += 1
                    elif result.action == "conflict":
                        stats["conflicts"] += 1
                    else:
                        stats["skipped"] += 1
                else:
                    # Send to cmdb-core staging
                    await _send_to_staging(tenant_id, raw)
                    stats["staging"] += 1

            except Exception as e:
                logger.error("Routing failed for asset %s: %s", raw.unique_key, e)
                stats["errors"] += 1

            # Progress update
            task.update_state(
                state="PROGRESS",
                meta={"processed": stats["responded"], "stats": stats},
            )

        # Mark task complete
        await _update_task_completed(pool, task_id, stats)

        # Publish NATS event
        await publish_event(nc, "import.completed", tenant_id, {
            "task_id": task_id,
            "collector": collector_type,
            "stats": stats,
        })

        return {"status": "completed", "stats": stats}

    except Exception as e:
        logger.exception("Discovery task %s failed", task_id)
        await _update_task_status(pool, task_id, "failed")
        raise
    finally:
        await close_nats(nc)
        await pool.close()


async def _send_to_staging(tenant_id: str, raw: RawAssetData):
    """Send a discovered asset to cmdb-core /discovery/ingest."""
    payload = {
        "source": raw.source,
        "hostname": raw.fields.get("name", ""),
        "ip_address": (raw.attributes or {}).get("ip_address", ""),
        "raw_data": {
            "fields": raw.fields,
            "attributes": raw.attributes or {},
        },
    }
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{CMDB_CORE_URL}/discovery/ingest",
            json=payload,
            headers={"X-Tenant-ID": tenant_id},
            timeout=10,
        )
        resp.raise_for_status()


async def _update_task_status(pool: asyncpg.Pool, task_id: str, status: str):
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE discovery_tasks SET status = $1 WHERE id = $2",
            status, UUID(task_id),
        )


async def _update_task_completed(pool: asyncpg.Pool, task_id: str, stats: dict):
    async with pool.acquire() as conn:
        await conn.execute(
            """UPDATE discovery_tasks
               SET status = 'completed', stats = $1, completed_at = $2
               WHERE id = $3""",
            json.dumps(stats),
            datetime.now(timezone.utc),
            UUID(task_id),
        )
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/test_discovery_task.py -v
```

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ingestion-engine/app/tasks/discovery_task.py ingestion-engine/tests/test_discovery_task.py
git commit -m "feat: add discovery Celery task with auto/review/smart mode routing"
```

---

## Task 12: Discovery API Routes

**Files:**
- Create: `ingestion-engine/app/routes/discovery.py`
- Modify: `ingestion-engine/app/main.py`

- [ ] **Step 1: Write the discovery routes**

Create `ingestion-engine/app/routes/discovery.py`:

```python
"""Discovery routes: trigger scan, list tasks, task details."""

import json
import uuid

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.dependencies import get_db_pool
from app.tasks.discovery_task import process_discovery_task

router = APIRouter(tags=["discovery"])


class ScanRequest(BaseModel):
    scan_target_id: str | None = None
    # Inline config (used when scan_target_id is not provided)
    tenant_id: str | None = None
    collector_type: str | None = None
    cidrs: list[str] | None = None
    credential_id: str | None = None
    mode: str | None = None


@router.post("/discovery/scan")
async def trigger_scan(
    body: ScanRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Trigger a discovery scan. Either provide scan_target_id or inline config."""
    if body.scan_target_id:
        # Load from scan_targets table
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT * FROM scan_targets WHERE id = $1",
                uuid.UUID(body.scan_target_id),
            )
        if not row:
            raise HTTPException(status_code=404, detail="Scan target not found")

        tenant_id = str(row["tenant_id"])
        collector_type = row["collector_type"]
        cidrs = list(row["cidrs"])
        credential_id = str(row["credential_id"])
        mode = row["mode"]
    else:
        # Inline config
        if not all([body.tenant_id, body.collector_type, body.cidrs, body.credential_id]):
            raise HTTPException(
                status_code=400,
                detail="Provide scan_target_id or all of: tenant_id, collector_type, cidrs, credential_id",
            )
        tenant_id = body.tenant_id
        collector_type = body.collector_type
        cidrs = body.cidrs
        credential_id = body.credential_id
        mode = body.mode or "smart"

    # Create discovery_tasks record
    task_id = uuid.uuid4()
    config = {
        "cidrs": cidrs,
        "credential_id": credential_id,
        "mode": mode,
    }
    async with pool.acquire() as conn:
        await conn.execute(
            """INSERT INTO discovery_tasks (id, tenant_id, type, status, config)
               VALUES ($1, $2, $3, 'pending', $4)""",
            task_id,
            uuid.UUID(tenant_id),
            collector_type,
            json.dumps(config),
        )

    # Dispatch Celery task
    process_discovery_task.delay(
        str(task_id), tenant_id, collector_type, cidrs, credential_id, mode,
    )

    return {
        "task_id": str(task_id),
        "status": "pending",
        "type": collector_type,
    }


@router.get("/discovery/tasks")
async def list_discovery_tasks(
    tenant_id: str,
    status: str | None = None,
    limit: int = 50,
    offset: int = 0,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List discovery tasks for a tenant."""
    async with pool.acquire() as conn:
        if status:
            rows = await conn.fetch(
                """SELECT * FROM discovery_tasks
                   WHERE tenant_id = $1 AND status = $2
                   ORDER BY created_at DESC LIMIT $3 OFFSET $4""",
                uuid.UUID(tenant_id), status, limit, offset,
            )
            total = await conn.fetchval(
                "SELECT count(*) FROM discovery_tasks WHERE tenant_id = $1 AND status = $2",
                uuid.UUID(tenant_id), status,
            )
        else:
            rows = await conn.fetch(
                """SELECT * FROM discovery_tasks
                   WHERE tenant_id = $1
                   ORDER BY created_at DESC LIMIT $2 OFFSET $3""",
                uuid.UUID(tenant_id), limit, offset,
            )
            total = await conn.fetchval(
                "SELECT count(*) FROM discovery_tasks WHERE tenant_id = $1",
                uuid.UUID(tenant_id),
            )

    tasks = []
    for r in rows:
        task_dict = dict(r)
        # Parse JSONB fields
        if task_dict.get("config") and isinstance(task_dict["config"], str):
            task_dict["config"] = json.loads(task_dict["config"])
        if task_dict.get("stats") and isinstance(task_dict["stats"], str):
            task_dict["stats"] = json.loads(task_dict["stats"])
        tasks.append(task_dict)

    return {"tasks": tasks, "total": total}


@router.get("/discovery/tasks/{task_id}")
async def get_discovery_task(
    task_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Get a single discovery task's details."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT * FROM discovery_tasks WHERE id = $1",
            uuid.UUID(task_id),
        )
    if not row:
        raise HTTPException(status_code=404, detail="Discovery task not found")

    task_dict = dict(row)
    if task_dict.get("config") and isinstance(task_dict["config"], str):
        task_dict["config"] = json.loads(task_dict["config"])
    if task_dict.get("stats") and isinstance(task_dict["stats"], str):
        task_dict["stats"] = json.loads(task_dict["stats"])

    return {"task": task_dict}
```

- [ ] **Step 2: Register router in main.py**

In `ingestion-engine/app/main.py`, add:

```python
from app.routes.discovery import router as discovery_router
```

In `create_app()`:

```python
    application.include_router(discovery_router)
```

- [ ] **Step 3: Commit**

```bash
git add ingestion-engine/app/routes/discovery.py ingestion-engine/app/main.py
git commit -m "feat: add discovery scan trigger and task list API routes"
```

---

## Task 13: Modify Collector Test Endpoint

**Files:**
- Modify: `ingestion-engine/app/routes/collectors.py`

- [ ] **Step 1: Update test endpoint to accept credential_id**

In `ingestion-engine/app/routes/collectors.py`, replace the `test_collector` function:

```python
"""Collector management routes."""

from dataclasses import asdict
from uuid import UUID

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.collectors.manager import manager
from app.credentials.provider import DBCredentialProvider
from app.dependencies import get_db_pool
from app.models.common import CollectTarget

router = APIRouter(tags=["collectors"])


class TestRequest(BaseModel):
    credential_id: str
    endpoint: str


@router.get("/collectors")
async def list_collectors():
    """List all registered collectors with their status."""
    return {"collectors": manager.list_all()}


@router.post("/collectors/{name}/start")
async def start_collector(name: str):
    """Start a collector by name."""
    try:
        status = manager.start(name)
        return asdict(status)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.post("/collectors/{name}/stop")
async def stop_collector(name: str):
    """Stop a collector by name."""
    try:
        status = manager.stop(name)
        return asdict(status)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.post("/collectors/{name}/test")
async def test_collector(
    name: str,
    body: TestRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Test connectivity for a collector using a stored credential."""
    # Load credential
    provider = DBCredentialProvider(pool)
    try:
        cred = await provider.get(UUID(body.credential_id))
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))

    target = CollectTarget(
        endpoint=body.endpoint,
        credentials=cred["params"],
    )
    try:
        result = await manager.test_connection(name, target)
        return result.model_dump()
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
```

- [ ] **Step 2: Commit**

```bash
git add ingestion-engine/app/routes/collectors.py
git commit -m "feat: update collector test endpoint to use stored credentials"
```

---

## Task 14: Frontend Proxy + API Client

**Files:**
- Modify: `cmdb-demo/vite.config.ts`
- Create: `cmdb-demo/src/lib/api/ingestion.ts`

- [ ] **Step 1: Add ingestion proxy to vite config**

In `cmdb-demo/vite.config.ts`, replace the `proxy` section:

```ts
    proxy: {
      '/api/v1/ingestion': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        rewrite: (path: string) => path.replace('/api/v1/ingestion', ''),
      },
      '/api/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
```

- [ ] **Step 2: Create ingestion API client**

Create `cmdb-demo/src/lib/api/ingestion.ts`:

```ts
import { apiClient } from './client'

export const ingestionApi = {
  // Credentials
  listCredentials: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/credentials', params),
  createCredential: (data: any) =>
    apiClient.post('/ingestion/credentials', data),
  updateCredential: (id: string, data: any) =>
    apiClient.put(`/ingestion/credentials/${id}`, data),
  deleteCredential: (id: string) =>
    apiClient.del(`/ingestion/credentials/${id}`),

  // Scan Targets
  listScanTargets: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/scan-targets', params),
  createScanTarget: (data: any) =>
    apiClient.post('/ingestion/scan-targets', data),
  updateScanTarget: (id: string, data: any) =>
    apiClient.put(`/ingestion/scan-targets/${id}`, data),
  deleteScanTarget: (id: string) =>
    apiClient.del(`/ingestion/scan-targets/${id}`),

  // Discovery
  triggerScan: (data: any) =>
    apiClient.post('/ingestion/discovery/scan', data),
  listTasks: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/discovery/tasks', params),
  getTask: (id: string) =>
    apiClient.get(`/ingestion/discovery/tasks/${id}`),

  // Collector test
  testCollector: (name: string, data: { credential_id: string; endpoint: string }) =>
    apiClient.post(`/ingestion/collectors/${name}/test`, data),
}
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/vite.config.ts cmdb-demo/src/lib/api/ingestion.ts
git commit -m "feat: add ingestion proxy and API client"
```

---

## Task 15: Frontend Hooks

**Files:**
- Create: `cmdb-demo/src/hooks/useCredentials.ts`
- Create: `cmdb-demo/src/hooks/useScanTargets.ts`

- [ ] **Step 1: Create credentials hooks**

Create `cmdb-demo/src/hooks/useCredentials.ts`:

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ingestionApi } from '../lib/api/ingestion'
import { useAuthStore } from '../stores/authStore'

function useTenantId() {
  return useAuthStore((s) => s.tenantId) ?? 'a0000000-0000-0000-0000-000000000001'
}

export function useCredentials() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['credentials', tenantId],
    queryFn: () => ingestionApi.listCredentials({ tenant_id: tenantId }),
  })
}

export function useCreateCredential() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ingestionApi.createCredential,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}

export function useUpdateCredential() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) =>
      ingestionApi.updateCredential(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}

export function useDeleteCredential() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => ingestionApi.deleteCredential(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}
```

- [ ] **Step 2: Create scan targets + discovery hooks**

Create `cmdb-demo/src/hooks/useScanTargets.ts`:

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ingestionApi } from '../lib/api/ingestion'
import { useAuthStore } from '../stores/authStore'

function useTenantId() {
  return useAuthStore((s) => s.tenantId) ?? 'a0000000-0000-0000-0000-000000000001'
}

export function useScanTargets() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['scanTargets', tenantId],
    queryFn: () => ingestionApi.listScanTargets({ tenant_id: tenantId }),
  })
}

export function useCreateScanTarget() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ingestionApi.createScanTarget,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useUpdateScanTarget() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) =>
      ingestionApi.updateScanTarget(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useDeleteScanTarget() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => ingestionApi.deleteScanTarget(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useTriggerScan() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ingestionApi.triggerScan,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['discoveryTasks'] }),
  })
}

export function useDiscoveryTasks() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['discoveryTasks', tenantId],
    queryFn: () => ingestionApi.listTasks({ tenant_id: tenantId }),
    refetchInterval: 10000,
  })
}

export function useTestCollector() {
  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: { credential_id: string; endpoint: string } }) =>
      ingestionApi.testCollector(name, data),
  })
}
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/hooks/useCredentials.ts cmdb-demo/src/hooks/useScanTargets.ts
git commit -m "feat: add React Query hooks for credentials and scan targets"
```

---

## Task 16: Credential Management Modal + SystemSettings Tab

**Files:**
- Create: `cmdb-demo/src/components/CreateCredentialModal.tsx`
- Modify: `cmdb-demo/src/pages/SystemSettings.tsx`

- [ ] **Step 1: Create credential modal**

Create `cmdb-demo/src/components/CreateCredentialModal.tsx`:

```tsx
import { useState, useEffect } from 'react'
import { useCreateCredential, useUpdateCredential } from '../hooks/useCredentials'
import { useAuthStore } from '../stores/authStore'

interface Props {
  open: boolean
  onClose: () => void
  editing?: { id: string; name: string; type: string } | null
}

const TYPES = [
  { value: 'snmp_v2c', label: 'SNMP v2c' },
  { value: 'snmp_v3', label: 'SNMP v3' },
  { value: 'ssh_password', label: 'SSH Password' },
  { value: 'ssh_key', label: 'SSH Key' },
  { value: 'ipmi', label: 'IPMI' },
]

const initialParams: Record<string, Record<string, string>> = {
  snmp_v2c: { community: '' },
  snmp_v3: { username: '', auth_pass: '', priv_pass: '', auth_proto: 'SHA', priv_proto: 'AES' },
  ssh_password: { username: '', password: '' },
  ssh_key: { username: '', private_key: '', passphrase: '' },
  ipmi: { username: '', password: '' },
}

export default function CreateCredentialModal({ open, onClose, editing }: Props) {
  const [name, setName] = useState('')
  const [type, setType] = useState('snmp_v2c')
  const [params, setParams] = useState<Record<string, string>>({ community: '' })
  const createMutation = useCreateCredential()
  const updateMutation = useUpdateCredential()
  const tenantId = useAuthStore((s) => s.tenantId) ?? 'a0000000-0000-0000-0000-000000000001'

  useEffect(() => {
    if (editing) {
      setName(editing.name)
      setType(editing.type)
      setParams(initialParams[editing.type] || {})
    } else {
      setName('')
      setType('snmp_v2c')
      setParams({ community: '' })
    }
  }, [editing, open])

  if (!open) return null

  const handleTypeChange = (newType: string) => {
    setType(newType)
    setParams({ ...(initialParams[newType] || {}) })
  }

  const handleParamChange = (key: string, value: string) => {
    setParams((p) => ({ ...p, [key]: value }))
  }

  const handleSubmit = () => {
    // Filter out empty optional params
    const cleanParams: Record<string, string> = {}
    for (const [k, v] of Object.entries(params)) {
      if (v) cleanParams[k] = v
    }

    if (editing) {
      updateMutation.mutate(
        { id: editing.id, data: { name, type, params: Object.keys(cleanParams).length > 0 ? cleanParams : undefined } },
        { onSuccess: onClose },
      )
    } else {
      createMutation.mutate(
        { tenant_id: tenantId, name, type, params: cleanParams },
        { onSuccess: onClose },
      )
    }
  }

  const inputCls = 'w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm'
  const labelCls = 'block text-sm text-gray-400 mb-1'

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{editing ? 'Edit Credential' : 'Create Credential'}</h3>

        <div>
          <label className={labelCls}>Name *</label>
          <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} placeholder="e.g. IDC-A Switches" />
        </div>

        <div>
          <label className={labelCls}>Type *</label>
          <select value={type} onChange={(e) => handleTypeChange(e.target.value)} className={inputCls}>
            {TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
        </div>

        {/* Dynamic param fields */}
        {type === 'snmp_v2c' && (
          <div>
            <label className={labelCls}>Community String *</label>
            <input value={params.community || ''} onChange={(e) => handleParamChange('community', e.target.value)}
              className={inputCls} placeholder={editing ? '••••••••' : 'public'} />
          </div>
        )}

        {type === 'snmp_v3' && (
          <>
            <div><label className={labelCls}>Username *</label>
              <input value={params.username || ''} onChange={(e) => handleParamChange('username', e.target.value)} className={inputCls} /></div>
            <div><label className={labelCls}>Auth Password *</label>
              <input type="password" value={params.auth_pass || ''} onChange={(e) => handleParamChange('auth_pass', e.target.value)}
                className={inputCls} placeholder={editing ? '••••••••' : ''} /></div>
            <div><label className={labelCls}>Priv Password</label>
              <input type="password" value={params.priv_pass || ''} onChange={(e) => handleParamChange('priv_pass', e.target.value)}
                className={inputCls} placeholder={editing ? '••••••••' : ''} /></div>
            <div className="grid grid-cols-2 gap-3">
              <div><label className={labelCls}>Auth Protocol</label>
                <select value={params.auth_proto || 'SHA'} onChange={(e) => handleParamChange('auth_proto', e.target.value)} className={inputCls}>
                  <option value="MD5">MD5</option><option value="SHA">SHA</option>
                </select></div>
              <div><label className={labelCls}>Priv Protocol</label>
                <select value={params.priv_proto || 'AES'} onChange={(e) => handleParamChange('priv_proto', e.target.value)} className={inputCls}>
                  <option value="DES">DES</option><option value="AES">AES</option>
                </select></div>
            </div>
          </>
        )}

        {type === 'ssh_password' && (
          <>
            <div><label className={labelCls}>Username *</label>
              <input value={params.username || ''} onChange={(e) => handleParamChange('username', e.target.value)} className={inputCls} placeholder="root" /></div>
            <div><label className={labelCls}>Password *</label>
              <input type="password" value={params.password || ''} onChange={(e) => handleParamChange('password', e.target.value)}
                className={inputCls} placeholder={editing ? '••••••••' : ''} /></div>
          </>
        )}

        {type === 'ssh_key' && (
          <>
            <div><label className={labelCls}>Username *</label>
              <input value={params.username || ''} onChange={(e) => handleParamChange('username', e.target.value)} className={inputCls} placeholder="root" /></div>
            <div><label className={labelCls}>Private Key *</label>
              <textarea value={params.private_key || ''} onChange={(e) => handleParamChange('private_key', e.target.value)}
                className={`${inputCls} h-32 font-mono text-xs`} placeholder={editing ? '••••••••' : '-----BEGIN OPENSSH PRIVATE KEY-----'} /></div>
            <div><label className={labelCls}>Passphrase</label>
              <input type="password" value={params.passphrase || ''} onChange={(e) => handleParamChange('passphrase', e.target.value)}
                className={inputCls} placeholder="Optional" /></div>
          </>
        )}

        {type === 'ipmi' && (
          <>
            <div><label className={labelCls}>Username *</label>
              <input value={params.username || ''} onChange={(e) => handleParamChange('username', e.target.value)} className={inputCls} placeholder="admin" /></div>
            <div><label className={labelCls}>Password *</label>
              <input type="password" value={params.password || ''} onChange={(e) => handleParamChange('password', e.target.value)}
                className={inputCls} placeholder={editing ? '••••••••' : ''} /></div>
          </>
        )}

        <div className="flex justify-end gap-3 pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-gray-300 text-sm hover:bg-gray-600">Cancel</button>
          <button onClick={handleSubmit} disabled={!name || createMutation.isPending || updateMutation.isPending}
            className="px-4 py-2 rounded bg-sky-600 text-white text-sm font-semibold hover:bg-sky-500 disabled:opacity-50">
            {editing ? 'Update' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Add credentials tab to SystemSettings**

In `cmdb-demo/src/pages/SystemSettings.tsx`, add the imports at the top:

```tsx
import { useCredentials, useDeleteCredential } from '../hooks/useCredentials'
import CreateCredentialModal from '../components/CreateCredentialModal'
```

Add state for the modal:

```tsx
const [showCreateCredential, setShowCreateCredential] = useState(false)
const [editingCredential, setEditingCredential] = useState<any>(null)
const { data: credentialsResp } = useCredentials()
const deleteCredential = useDeleteCredential()
const apiCredentials = (credentialsResp as any)?.credentials ?? []
```

Add `'credentials'` to the tabs array and add the tab content panel (matching existing tab pattern in the file — see the `activeTab === 'integrations'` section for the pattern). The credentials tab renders a table with name, type, created_at columns, edit/delete buttons, and the create modal trigger.

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/components/CreateCredentialModal.tsx cmdb-demo/src/pages/SystemSettings.tsx
git commit -m "feat: add credential management UI in SystemSettings"
```

---

## Task 17: Scan Management Tab + AutoDiscovery Restructure

**Files:**
- Create: `cmdb-demo/src/components/CreateScanTargetModal.tsx`
- Create: `cmdb-demo/src/components/ScanManagementTab.tsx`
- Modify: `cmdb-demo/src/pages/AutoDiscovery.tsx`

- [ ] **Step 1: Create scan target modal**

Create `cmdb-demo/src/components/CreateScanTargetModal.tsx`:

```tsx
import { useState, useEffect } from 'react'
import { useCreateScanTarget, useUpdateScanTarget } from '../hooks/useScanTargets'
import { useCredentials } from '../hooks/useCredentials'
import { useAuthStore } from '../stores/authStore'

interface Props {
  open: boolean
  onClose: () => void
  editing?: any | null
}

export default function CreateScanTargetModal({ open, onClose, editing }: Props) {
  const [name, setName] = useState('')
  const [collectorType, setCollectorType] = useState('snmp')
  const [cidrs, setCidrs] = useState('')
  const [credentialId, setCredentialId] = useState('')
  const [mode, setMode] = useState('smart')
  const createMutation = useCreateScanTarget()
  const updateMutation = useUpdateScanTarget()
  const { data: credResp } = useCredentials()
  const allCredentials = (credResp as any)?.credentials ?? []
  const tenantId = useAuthStore((s) => s.tenantId) ?? 'a0000000-0000-0000-0000-000000000001'

  // Filter credentials by compatible type
  const typeMap: Record<string, string[]> = {
    snmp: ['snmp_v2c', 'snmp_v3'],
    ssh: ['ssh_password', 'ssh_key'],
    ipmi: ['ipmi'],
  }
  const filteredCredentials = allCredentials.filter((c: any) =>
    typeMap[collectorType]?.includes(c.type),
  )

  useEffect(() => {
    if (editing) {
      setName(editing.name)
      setCollectorType(editing.collector_type)
      setCidrs((editing.cidrs || []).join('\n'))
      setCredentialId(editing.credential_id || '')
      setMode(editing.mode || 'smart')
    } else {
      setName('')
      setCollectorType('snmp')
      setCidrs('')
      setCredentialId('')
      setMode('smart')
    }
  }, [editing, open])

  if (!open) return null

  const handleSubmit = () => {
    const cidrList = cidrs.split('\n').map((s) => s.trim()).filter(Boolean)
    const payload = { name, collector_type: collectorType, cidrs: cidrList, credential_id: credentialId, mode }

    if (editing) {
      updateMutation.mutate({ id: editing.id, data: payload }, { onSuccess: onClose })
    } else {
      createMutation.mutate({ ...payload, tenant_id: tenantId }, { onSuccess: onClose })
    }
  }

  const inputCls = 'w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm'
  const labelCls = 'block text-sm text-gray-400 mb-1'

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[32rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{editing ? 'Edit Scan Target' : 'Add Scan Target'}</h3>

        <div><label className={labelCls}>Name *</label>
          <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} placeholder="e.g. IDC-A Server BMCs" /></div>

        <div><label className={labelCls}>Collector Type *</label>
          <select value={collectorType} onChange={(e) => { setCollectorType(e.target.value); setCredentialId('') }} className={inputCls}>
            <option value="snmp">SNMP</option>
            <option value="ssh">SSH</option>
            <option value="ipmi">IPMI</option>
          </select></div>

        <div><label className={labelCls}>CIDRs (one per line) *</label>
          <textarea value={cidrs} onChange={(e) => setCidrs(e.target.value)}
            className={`${inputCls} h-24 font-mono`} placeholder="192.168.1.0/24&#10;10.0.5.0/24" /></div>

        <div><label className={labelCls}>Credential *</label>
          <select value={credentialId} onChange={(e) => setCredentialId(e.target.value)} className={inputCls}>
            <option value="">-- Select --</option>
            {filteredCredentials.map((c: any) => (
              <option key={c.id} value={c.id}>{c.name} ({c.type})</option>
            ))}
          </select>
          {filteredCredentials.length === 0 && (
            <p className="text-xs text-amber-400 mt-1">No compatible credentials found. Create one in System Settings first.</p>
          )}
        </div>

        <div><label className={labelCls}>Mode *</label>
          <select value={mode} onChange={(e) => setMode(e.target.value)} className={inputCls}>
            <option value="auto">Auto (direct to pipeline)</option>
            <option value="review">Review (manual approval)</option>
            <option value="smart">Smart (new→review, existing→auto)</option>
          </select></div>

        <div className="flex justify-end gap-3 pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-gray-300 text-sm hover:bg-gray-600">Cancel</button>
          <button onClick={handleSubmit} disabled={!name || !cidrs || !credentialId || createMutation.isPending}
            className="px-4 py-2 rounded bg-sky-600 text-white text-sm font-semibold hover:bg-sky-500 disabled:opacity-50">
            {editing ? 'Update' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Create ScanManagementTab component**

Create `cmdb-demo/src/components/ScanManagementTab.tsx`:

```tsx
import { useState } from 'react'
import Icon from './Icon'
import { useScanTargets, useDeleteScanTarget, useTriggerScan, useDiscoveryTasks, useTestCollector } from '../hooks/useScanTargets'
import CreateScanTargetModal from './CreateScanTargetModal'

export default function ScanManagementTab() {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<any>(null)
  const { data: targetsResp, isLoading: targetsLoading } = useScanTargets()
  const { data: tasksResp, isLoading: tasksLoading } = useDiscoveryTasks()
  const deleteMutation = useDeleteScanTarget()
  const scanMutation = useTriggerScan()
  const testMutation = useTestCollector()

  const targets = (targetsResp as any)?.scan_targets ?? []
  const tasks = (tasksResp as any)?.tasks ?? []

  const handleScan = (target: any) => {
    scanMutation.mutate({ scan_target_id: target.id })
  }

  const handleTest = (target: any) => {
    const firstCidr = target.cidrs?.[0] || ''
    const firstIp = firstCidr.split('/')[0]
    testMutation.mutate({ name: target.collector_type, data: { credential_id: target.credential_id, endpoint: firstIp } })
  }

  const collectorIcon: Record<string, string> = { snmp: 'router', ssh: 'terminal', ipmi: 'developer_board' }
  const statusColor: Record<string, string> = {
    completed: 'text-emerald-400', running: 'text-blue-400', pending: 'text-gray-400', failed: 'text-red-400',
  }

  return (
    <div className="space-y-6">
      {/* Scan Targets */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-headline font-bold text-lg text-on-surface">Scan Targets</h2>
          <button onClick={() => { setEditing(null); setShowModal(true) }}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-sky-600 text-white text-sm font-semibold hover:bg-sky-500">
            <Icon name="add" className="text-[18px]" /> Add Target
          </button>
        </div>

        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">Name</th>
                <th className="px-4 py-3 text-left font-semibold">Type</th>
                <th className="px-4 py-3 text-left font-semibold">CIDRs</th>
                <th className="px-4 py-3 text-left font-semibold">Credential</th>
                <th className="px-4 py-3 text-left font-semibold">Mode</th>
                <th className="px-4 py-3 text-right font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody>
              {targetsLoading && (
                <tr><td colSpan={6} className="py-8 text-center">
                  <div className="inline-block animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {targets.map((t: any) => (
                <tr key={t.id} className="border-t border-surface-container-high hover:bg-surface-container-high transition-colors">
                  <td className="px-4 py-3 font-medium text-on-surface">{t.name}</td>
                  <td className="px-4 py-3">
                    <span className="flex items-center gap-1.5">
                      <Icon name={collectorIcon[t.collector_type] || 'device_hub'} className="text-[16px] text-on-surface-variant" />
                      {t.collector_type.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-on-surface-variant">{(t.cidrs || []).join(', ')}</td>
                  <td className="px-4 py-3 text-on-surface-variant">{t.credential_name || '—'}</td>
                  <td className="px-4 py-3">
                    <span className="px-2 py-0.5 rounded text-xs font-semibold bg-surface-container-highest text-on-surface-variant uppercase">
                      {t.mode}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-1">
                      <button onClick={() => handleScan(t)} disabled={scanMutation.isPending}
                        className="p-1.5 rounded-md hover:bg-sky-500/20 transition-colors" title="Scan Now">
                        <Icon name="play_arrow" className="text-[18px] text-sky-400" />
                      </button>
                      <button onClick={() => handleTest(t)} disabled={testMutation.isPending}
                        className="p-1.5 rounded-md hover:bg-amber-500/20 transition-colors" title="Test Connection">
                        <Icon name="lan" className="text-[18px] text-amber-400" />
                      </button>
                      <button onClick={() => { setEditing(t); setShowModal(true) }}
                        className="p-1.5 rounded-md hover:bg-surface-container-highest transition-colors" title="Edit">
                        <Icon name="edit" className="text-[18px] text-on-surface-variant" />
                      </button>
                      <button onClick={() => deleteMutation.mutate(t.id)}
                        className="p-1.5 rounded-md hover:bg-error-container/40 transition-colors" title="Delete">
                        <Icon name="delete" className="text-[18px] text-error" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {!targetsLoading && targets.length === 0 && (
                <tr><td colSpan={6} className="py-8 text-center text-on-surface-variant text-sm">
                  No scan targets configured. Click "Add Target" to get started.
                </td></tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Test result toast */}
        {testMutation.isSuccess && (
          <div className="mt-2 p-3 rounded-lg bg-surface-container-high text-sm">
            {(testMutation.data as any)?.success
              ? <span className="text-emerald-400">Connected: {(testMutation.data as any)?.message}</span>
              : <span className="text-red-400">Failed: {(testMutation.data as any)?.message}</span>}
          </div>
        )}
      </div>

      {/* Task History */}
      <div>
        <h2 className="font-headline font-bold text-lg text-on-surface mb-4">Scan History</h2>
        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">Type</th>
                <th className="px-4 py-3 text-left font-semibold">Status</th>
                <th className="px-4 py-3 text-left font-semibold">Stats</th>
                <th className="px-4 py-3 text-left font-semibold">Started</th>
                <th className="px-4 py-3 text-left font-semibold">Completed</th>
              </tr>
            </thead>
            <tbody>
              {tasksLoading && (
                <tr><td colSpan={5} className="py-8 text-center">
                  <div className="inline-block animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {tasks.map((t: any) => {
                const stats = t.stats || {}
                return (
                  <tr key={t.id} className="border-t border-surface-container-high">
                    <td className="px-4 py-3 font-medium text-on-surface uppercase">{t.type}</td>
                    <td className={`px-4 py-3 font-semibold ${statusColor[t.status] || 'text-on-surface-variant'}`}>{t.status}</td>
                    <td className="px-4 py-3 text-xs text-on-surface-variant font-mono">
                      {stats.responded != null ? `${stats.responded} found` : '—'}
                      {stats.created ? ` / ${stats.created} new` : ''}
                      {stats.updated ? ` / ${stats.updated} updated` : ''}
                      {stats.conflicts ? ` / ${stats.conflicts} conflicts` : ''}
                    </td>
                    <td className="px-4 py-3 text-on-surface-variant text-xs">{t.created_at ? new Date(t.created_at).toLocaleString() : '—'}</td>
                    <td className="px-4 py-3 text-on-surface-variant text-xs">{t.completed_at ? new Date(t.completed_at).toLocaleString() : '—'}</td>
                  </tr>
                )
              })}
              {!tasksLoading && tasks.length === 0 && (
                <tr><td colSpan={5} className="py-8 text-center text-on-surface-variant text-sm">No scan tasks yet.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <CreateScanTargetModal open={showModal} onClose={() => setShowModal(false)} editing={editing} />
    </div>
  )
}
```

- [ ] **Step 3: Restructure AutoDiscovery into tabs**

In `cmdb-demo/src/pages/AutoDiscovery.tsx`, add imports and restructure:

1. Add import: `import ScanManagementTab from '../components/ScanManagementTab'`
2. Add state: `const [activeTab, setActiveTab] = useState<'review' | 'scan'>('review')`
3. Add SSH to sourceIcon: `SSH: { icon: 'terminal', bg: 'bg-[#1a365d]' }`
4. Add SSH option to source filter dropdown
5. Wrap existing content (stats + filters + table) in `{activeTab === 'review' && (...)}`
6. Add `{activeTab === 'scan' && <ScanManagementTab />}`
7. Add tab switcher after the header section
8. Remove the bottom "Smart Insights" and "Schedule" panels

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/components/CreateScanTargetModal.tsx \
       cmdb-demo/src/components/ScanManagementTab.tsx \
       cmdb-demo/src/pages/AutoDiscovery.tsx
git commit -m "feat: add scan management UI and restructure AutoDiscovery into tabs"
```

---

## Task 18: Add httpx Dependency

**Files:**
- Modify: `ingestion-engine/pyproject.toml`

The discovery task uses `httpx` to call cmdb-core's `/discovery/ingest` endpoint for review/smart mode.

- [ ] **Step 1: Add httpx to dependencies**

In `ingestion-engine/pyproject.toml`, add to `dependencies`:

```toml
    "httpx>=0.28.0",
```

Note: `httpx` is already in `[project.optional-dependencies] dev`, move it to main dependencies.

- [ ] **Step 2: Install**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pip install -e ".[dev]"
```

- [ ] **Step 3: Commit**

```bash
git add ingestion-engine/pyproject.toml
git commit -m "feat: add httpx as runtime dependency for discovery staging"
```

---

## Task 19: Integration Smoke Test

**Files:** No new files — manual verification.

- [ ] **Step 1: Run all Python tests**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/pytest tests/ -v
```

Expected: All tests pass.

- [ ] **Step 2: Start ingestion-engine and verify endpoints**

```bash
cd /cmdb-platform/ingestion-engine
.venv/bin/uvicorn app.main:app --port 8000 --reload &
sleep 3

# List collectors
curl -s http://localhost:8000/collectors | python3 -m json.tool

# Create a credential
curl -s -X POST http://localhost:8000/credentials \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"a0000000-0000-0000-0000-000000000001","name":"test-snmp","type":"snmp_v2c","params":{"community":"public"}}' \
  | python3 -m json.tool

# List credentials
curl -s "http://localhost:8000/credentials?tenant_id=a0000000-0000-0000-0000-000000000001" | python3 -m json.tool
```

Expected: All 3 collectors listed; credential created and listed without params.

- [ ] **Step 3: Verify frontend compiles**

```bash
cd /cmdb-platform/cmdb-demo
npx tsc --noEmit
```

Expected: No type errors.

- [ ] **Step 4: Verify frontend loads**

Open `http://localhost:5175/assets/discovery` — should show two tabs.
Open `http://localhost:5175/system` — should show credentials tab.

- [ ] **Step 5: Commit any fixes needed**

If any fixes were required during smoke testing, commit them.
