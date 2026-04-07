# Auto Discovery Collectors Design

**Date:** 2026-04-07
**Scope:** SNMP / SSH / IPMI collectors + credential management + scan target management + frontend integration

---

## 1. Architecture Overview

Three collectors (SNMP, SSH, IPMI) implemented inside `ingestion-engine`, driven by Celery async tasks, sharing the existing pipeline infrastructure.

```
Frontend (AutoDiscovery page)
  │
  ├─ /api/v1/ingestion/credentials       → ingestion-engine :8000
  ├─ /api/v1/ingestion/scan-targets       → ingestion-engine :8000
  ├─ /api/v1/ingestion/discovery/scan     → ingestion-engine :8000
  ├─ /api/v1/ingestion/discovery/tasks    → ingestion-engine :8000
  │
  ├─ /api/v1/discovery/pending            → cmdb-core :8080 (existing)
  ├─ /api/v1/discovery/{id}/approve       → cmdb-core :8080 (existing)
  ├─ /api/v1/discovery/{id}/ignore        → cmdb-core :8080 (existing)
  └─ /api/v1/discovery/stats              → cmdb-core :8080 (existing)

ingestion-engine internal flow:
  POST /discovery/scan
    → Create discovery_tasks record
    → Dispatch Celery task: process_discovery_task.delay()
    → Celery worker:
        1. Load credential from DB (decrypt)
        2. Expand CIDRs to IP list
        3. asyncio.Semaphore for concurrency control
        4. Collector.collect(target) → list[RawAssetData]
        5. Route results by mode:
           - auto   → process_batch() → assets table directly
           - review → POST cmdb-core /discovery/ingest → discovered_assets staging
           - smart  → deduplicate check:
                       matched → process_batch()
                       new     → POST /discovery/ingest
        6. Update discovery_tasks stats
        7. Publish NATS event (import.completed)
```

## 2. Data Model

### 2.1 New Tables

#### `credentials`
```sql
CREATE TABLE credentials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,  -- snmp_v2c / snmp_v3 / ssh_password / ssh_key / ipmi
    params      BYTEA NOT NULL,        -- AES-256 encrypted JSON
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);
```

Credential `params` JSON structure by type:

| Type | Fields |
|------|--------|
| snmp_v2c | `{community}` |
| snmp_v3 | `{username, auth_pass, priv_pass, auth_proto (MD5/SHA), priv_proto (DES/AES)}` |
| ssh_password | `{username, password}` |
| ssh_key | `{username, private_key, passphrase?}` |
| ipmi | `{username, password}` |

#### `scan_targets`
```sql
CREATE TABLE scan_targets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(200) NOT NULL,
    cidrs           TEXT[] NOT NULL,          -- ['192.168.1.0/24', '10.0.5.0/24']
    collector_type  VARCHAR(30) NOT NULL,     -- snmp / ssh / ipmi
    credential_id   UUID NOT NULL REFERENCES credentials(id),
    mode            VARCHAR(20) NOT NULL DEFAULT 'smart',  -- auto / review / smart
    location_id     UUID REFERENCES locations(id),          -- nullable, phase 2
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 2.2 Schema Modifications

#### Add `ip_address` to `assets` table
```sql
ALTER TABLE assets ADD COLUMN ip_address VARCHAR(50);
CREATE INDEX idx_assets_ip_address ON assets(tenant_id, ip_address);

-- Migrate existing data from attributes JSONB
UPDATE assets SET ip_address = attributes->>'ip_address'
WHERE attributes->>'ip_address' IS NOT NULL;
```

#### Fix `FindAssetByIP` query
```sql
-- name: FindAssetByIP :one
SELECT * FROM assets WHERE tenant_id = $1 AND ip_address = $2 LIMIT 1;
```

### 2.3 Existing Tables (reused as-is)

- `discovery_tasks` — tracks each scan execution (type, status, config JSONB, stats JSONB)
- `discovered_assets` (cmdb-core) — staging area for review/smart mode
- `asset_field_authorities` — authority matrix (already seeded for ipmi/snmp/manual)
- `import_conflicts` — field-level conflicts from pipeline

## 3. Collector Implementations

### 3.1 SNMP Collector (`collectors/snmp.py`)

**Library:** `pysnmp>=7.0` (native async via `pysnmp.hlapi.asyncio`)

**Target:** Network devices (switches, routers, firewalls)

**Concurrency:** `asyncio.Semaphore(50)` — SNMP is lightweight UDP

**Collection flow:**
1. Expand CIDR to IP list
2. Probe each IP with `sysDescr` (1.3.6.1.2.1.1.1.0) for liveness
3. For responding IPs, collect:

| OID | Field |
|-----|-------|
| sysName (1.3.6.1.2.1.1.5.0) | hostname |
| sysDescr (1.3.6.1.2.1.1.1.0) | model / os_version (parsed) |
| sysObjectID (1.3.6.1.2.1.1.2.0) | vendor (via OID prefix mapping) |
| entPhysicalSerialNum (1.3.6.1.2.1.47.1.1.1.1.11) | serial_number (standard ENTITY-MIB, try first) |
| ifTable (1.3.6.1.2.1.2.2.1.*) | network interfaces (stored in attributes) |

**Serial number fallback OIDs** (when ENTITY-MIB not supported):

| Vendor | OID |
|--------|-----|
| HP | 1.3.6.1.4.1.11.2.36.1.1.2.9.0 |
| Dell | 1.3.6.1.4.1.674.10895.3000.1.2.100.8.1.4.1 |
| Huawei | 1.3.6.1.4.1.2011.5.25.188.1.1 (hwDeviceEsn) |
| Juniper | 1.3.6.1.4.1.2636.3.1.3.0 (jnxBoxSerialNo) |

**Vendor detection:** Map `sysObjectID` prefix to vendor name:
- `1.3.6.1.4.1.9.*` → Cisco
- `1.3.6.1.4.1.11.*` → HP
- `1.3.6.1.4.1.674.*` → Dell
- `1.3.6.1.4.1.2011.*` → Huawei
- `1.3.6.1.4.1.2636.*` → Juniper

### 3.2 SSH Collector (`collectors/ssh.py`)

**Library:** `asyncssh` (native async)

**Target:** Linux/Unix servers

**Concurrency:** `asyncio.Semaphore(10)` — SSH is heavyweight TCP

**Collection flow:**
1. Connect to each target IP via SSH (password or key auth)
2. Execute commands and parse output:

| Command | Field |
|---------|-------|
| `hostname` | hostname |
| `dmidecode -s system-serial-number` | serial_number |
| `dmidecode -s system-manufacturer` | vendor |
| `dmidecode -s system-product-name` | model |
| `cat /etc/os-release` | os_type, os_version |
| `nproc` | cpu_cores |
| `free -m \| awk '/Mem:/{print $2}'` | memory_mb |
| `lsblk -dbn -o SIZE \| awk '{s+=$1}END{print int(s/1073741824)}'` | disk_gb |
| `ip -4 addr show \| grep inet \| grep -v 127.0.0.1` | ip_address |

**Notes:**
- `dmidecode` requires root or sudo; handle permission errors gracefully
- Parse `/etc/os-release` for `ID` and `VERSION_ID` fields

### 3.3 IPMI Collector (`collectors/ipmi.py`)

**Library:** `pyghmi` (synchronous — wrap with `asyncio.to_thread()`)

**Target:** Server BMC (iDRAC, iLO, Supermicro IPMI)

**Concurrency:** `asyncio.Semaphore(20)` — UDP but BMC has limited processing

**Collection flow:**
1. Expand CIDR to BMC IP list
2. Connect via `pyghmi.ipmi.command.Command(bmc=ip, userid=..., password=...)`
3. Collect:

| pyghmi API | Field |
|------------|-------|
| `get_inventory()` → product_serial | serial_number |
| `get_inventory()` → manufacturer | vendor |
| `get_inventory()` → product_name | model |
| `get_inventory()` → product_part_number | part_number (attributes) |
| `get_net_configuration()` | bmc_ip, bmc_mac (attributes) |
| `get_chassis_status()` | power_state (attributes) |
| `get_sensor_data()` | temperature, fan speed (attributes) |
| `get_firmware()` | firmware_version (attributes) |

**Async wrapper pattern:**
```python
async def collect(self, target: CollectTarget) -> list[RawAssetData]:
    return await asyncio.to_thread(self._collect_sync, target)
```

### 3.4 Common Output

All collectors produce `list[RawAssetData]`:

```python
RawAssetData(
    source="snmp",           # or "ssh", "ipmi"
    unique_key="SN123456",   # serial_number for dedup
    fields={
        "serial_number": "SN123456",
        "vendor": "Cisco",
        "model": "Catalyst 9300",
        "name": "core-sw-01",      # from hostname
    },
    attributes={
        "ip_address": "192.168.1.1",
        "os_version": "IOS-XE 17.6",
        "interfaces": [...],
    },
    collected_at=datetime.now(timezone.utc),
)
```

## 4. Result Routing (Mode)

Each scan target has a `mode` field:

| Mode | Behavior |
|------|----------|
| `auto` | All results go through `process_batch()` → directly into `assets` table. Authority system handles conflicts. |
| `review` | All results sent to cmdb-core `POST /discovery/ingest` → `discovered_assets` staging → manual approve/ignore. |
| `smart` | Pipeline `deduplicate()` check: if matched to existing asset → `process_batch()`. If new → `POST /discovery/ingest` for manual review. |

## 5. Credential Management

### 5.1 Encryption

- AES-256-GCM encryption for credential params
- Encryption key from environment variable `CREDENTIAL_ENCRYPTION_KEY`
- Stored as BYTEA in PostgreSQL

### 5.2 Provider Interface

```python
class CredentialProvider(Protocol):
    async def get(self, credential_id: UUID) -> dict: ...

class DBCredentialProvider:
    """Reads from credentials table, decrypts params."""
    async def get(self, credential_id: UUID) -> dict: ...
```

Phase 1: DB storage only (configured via frontend).
Interface allows future extension (e.g., HashiCorp Vault).

## 6. API Endpoints

### 6.1 ingestion-engine (new)

**Credentials:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/credentials?tenant_id=` | List (password fields masked as `***`) |
| POST | `/credentials` | Create |
| PUT | `/credentials/{id}` | Update (omit password fields to keep existing) |
| DELETE | `/credentials/{id}` | Delete (reject if referenced by scan_targets) |

**Scan Targets:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/scan-targets?tenant_id=` | List (includes credential name) |
| POST | `/scan-targets` | Create |
| PUT | `/scan-targets/{id}` | Update |
| DELETE | `/scan-targets/{id}` | Delete |

**Discovery Execution:**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/discovery/scan` | Trigger scan (by scan_target_id or inline config) |
| GET | `/discovery/tasks?tenant_id=` | List tasks (paginated, filterable by status) |
| GET | `/discovery/tasks/{id}` | Task detail (progress + stats) |

**Connection Test (modify existing):**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/collectors/{name}/test` | Accept `credential_id` + `endpoint` instead of raw credentials |

### 6.2 cmdb-core (existing, no changes except bug fix)

- `GET /discovery/pending` — list discovered assets
- `POST /discovery/ingest` — ingest from collector
- `POST /discovery/{id}/approve` — approve
- `POST /discovery/{id}/ignore` — ignore
- `GET /discovery/stats` — 24h statistics
- **Fix:** `FindAssetByIP` query to use `ip_address` column instead of `serial_number`

### 6.3 Frontend Proxy

Add second proxy rule in `vite.config.ts`:

```ts
proxy: {
  '/api/v1/ingestion': {
    target: 'http://localhost:8000',
    changeOrigin: true,
    rewrite: (path) => path.replace('/api/v1/ingestion', ''),
  },
  '/api/v1': {
    target: 'http://localhost:8080',
    changeOrigin: true,
  },
}
```

Order matters: `/api/v1/ingestion` must come before `/api/v1`.

## 7. Frontend Changes

### 7.1 SystemSettings — New "Credentials" Tab

Add fourth tab after `permissions` / `security` / `integrations`.

**List view:** Table with name, type, created_by, created_at, edit/delete actions.

**Create/Edit Modal:**
- Name (text input)
- Type (dropdown: SNMP v2c / SNMP v3 / SSH Password / SSH Key / IPMI)
- Dynamic fields based on type selection
- Edit mode: password fields show `••••••••` placeholder; leave empty to keep existing

### 7.2 AutoDiscovery — Tab Restructure

Replace current single-page layout with two tabs:

**Tab 1: Discovery Review (existing content)**
- Stats cards (total, pending, conflict, approved, ignored, matched)
- Source + status filters
- Discovered assets table with approve/ignore actions
- Remove bottom "Smart Insights" and "Schedule" panels

**Tab 2: Scan Management (new)**
- Scan targets table: name, type (SNMP/SSH/IPMI), CIDRs, credential name, mode, actions
- Actions per row: "Scan Now" button, "Test Connection" button, edit, delete
- "Add Scan Target" button → Modal (name, type, CIDRs multi-line, credential dropdown filtered by type, mode dropdown)
- Below targets: Scan task history table (type, status, target CIDRs, stats summary, triggered_at, completed_at)

### 7.3 Source Icon Addition

Add SSH to `sourceIcon` mapping:
```ts
SSH: { icon: 'terminal', bg: 'bg-[#1a365d]' }
```

Add SSH option to source filter dropdown.

### 7.4 New API Client Functions

```ts
// lib/api/ingestion.ts
export const ingestionApi = {
  // Credentials
  listCredentials: () => apiClient.get('/ingestion/credentials', { tenant_id: ... }),
  createCredential: (data) => apiClient.post('/ingestion/credentials', data),
  updateCredential: (id, data) => apiClient.put(`/ingestion/credentials/${id}`, data),
  deleteCredential: (id) => apiClient.del(`/ingestion/credentials/${id}`),

  // Scan Targets
  listScanTargets: () => apiClient.get('/ingestion/scan-targets', { tenant_id: ... }),
  createScanTarget: (data) => apiClient.post('/ingestion/scan-targets', data),
  updateScanTarget: (id, data) => apiClient.put(`/ingestion/scan-targets/${id}`, data),
  deleteScanTarget: (id) => apiClient.del(`/ingestion/scan-targets/${id}`),

  // Discovery
  triggerScan: (data) => apiClient.post('/ingestion/discovery/scan', data),
  listTasks: () => apiClient.get('/ingestion/discovery/tasks', { tenant_id: ... }),
  getTask: (id) => apiClient.get(`/ingestion/discovery/tasks/${id}`),
}
```

## 8. Dependencies

Add to `ingestion-engine/pyproject.toml`:

```toml
pysnmp = ">=7.0"        # SNMP v1/v2c/v3 (async native)
asyncssh = ">=2.14"     # SSH client (async native)
pyghmi = ">=1.5"        # IPMI BMC access (sync, wrapped with asyncio.to_thread)
cryptography = ">=42.0"  # AES-256-GCM for credential encryption
```

## 9. File Structure (New/Modified)

```
ingestion-engine/
├── app/
│   ├── collectors/
│   │   ├── __init__.py          ← MODIFY: register all 3 collectors
│   │   ├── base.py              ← existing (no change)
│   │   ├── manager.py           ← existing (no change for phase 1)
│   │   ├── snmp.py              ← NEW
│   │   ├── ssh.py               ← NEW
│   │   └── ipmi.py              ← NEW
│   ├── credentials/
│   │   ├── __init__.py          ← NEW
│   │   ├── encryption.py        ← NEW: AES-256-GCM encrypt/decrypt
│   │   └── provider.py          ← NEW: DBCredentialProvider
│   ├── routes/
│   │   ├── credentials.py       ← NEW: CRUD endpoints
│   │   ├── scan_targets.py      ← NEW: CRUD endpoints
│   │   └── discovery.py         ← NEW: scan trigger + task list
│   ├── tasks/
│   │   └── discovery_task.py    ← NEW: Celery task
│   └── main.py                  ← MODIFY: register new routers

cmdb-core/
├── db/
│   ├── migrations/
│   │   └── 000017_*.up.sql      ← NEW: credentials + scan_targets + assets.ip_address
│   └── queries/
│       └── discovery.sql         ← MODIFY: fix FindAssetByIP

cmdb-demo/src/
├── lib/api/
│   └── ingestion.ts             ← NEW: API client for ingestion endpoints
├── hooks/
│   ├── useCredentials.ts        ← NEW
│   └── useScanTargets.ts        ← NEW
├── pages/
│   ├── AutoDiscovery.tsx        ← MODIFY: add tabs, scan management
│   └── SystemSettings.tsx       ← MODIFY: add credentials tab
├── components/
│   ├── CreateCredentialModal.tsx ← NEW
│   └── CreateScanTargetModal.tsx← NEW
└── vite.config.ts               ← MODIFY: add ingestion proxy
```

## 10. Phase Plan

**Phase 1 (this implementation):**
- 3 collectors (SNMP, SSH, IPMI)
- Credential DB storage + frontend management
- Scan target CRUD + manual trigger
- Mode routing (auto/review/smart)
- Fix FindAssetByIP + add assets.ip_address

**Phase 2 (future):**
- Scan target ↔ Location binding (multi-CIDR per location)
- Scheduled scanning (APScheduler / Celery Beat with per-target cron)
- Real-time notification (WebSocket / polling for scan completion)
- Collector start/stop lifecycle (long-running collectors)
