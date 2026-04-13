# v1.2 Phase D: Prometheus Metrics Pull Design Spec

> Date: 2026-04-13
> Status: Draft
> Prereqs: Phase A-C complete, StartMetricsPuller skeleton exists
> Scope: HTTP pull from Prometheus, write to metrics table, adapter config UI, health check + notification

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Asset mapping | Auto-match via ip_address | assets table has ip_address; Prometheus instance label contains IP; no manual mapping needed |
| Pull frequency | Configurable per adapter via config JSONB, default 5 min | Already runs on 5-min ticker; adapter config can override |
| Query list | Stored in adapter config JSONB | No new table needed; config already exists as JSONB |
| Retry | 3 consecutive failures → mark status=error + notify | Matches milestone plan exactly |

---

## D1: Prometheus HTTP Pull

### Current State

`pullMetricsFromAdapters()` in `workflows/subscriber.go` already:
- Runs every 5 minutes
- Queries `integration_adapters WHERE direction='inbound' AND enabled=true`
- Logs found adapters but does nothing else

### What to Add

Replace the log-only body with actual pull logic:

```
For each inbound adapter:
  1. Read adapter.config JSONB for query list:
     config = {"queries": ["node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "power_kw"]}
  2. For each query expr:
     GET {adapter.endpoint}/query?query={expr}
  3. Parse Prometheus response (standard JSON format):
     {"status":"success","data":{"resultType":"vector","result":[{"metric":{"instance":"10.0.1.5:9100",...},"value":[timestamp, "value"]}]}}
  4. For each result:
     - Extract IP from instance label (strip :port)
     - Lookup asset_id via: SELECT id FROM assets WHERE ip_address = $ip AND tenant_id = $tenant
     - If no match: asset_id = NULL
     - INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels) VALUES (...)
  5. Track consecutive failures per adapter:
     - Success → reset failure count
     - Failure → increment count
     - Count >= 3 → UPDATE adapter SET enabled=false, notify ops-admin
```

### Metrics Insert

Need new sqlc query (doesn't exist yet):

```sql
-- name: InsertMetric :exec
INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels)
VALUES ($1, $2, $3, $4, $5, $6);
```

### Asset IP Lookup

Need new sqlc query:

```sql
-- name: FindAssetByIP :one
SELECT id FROM assets WHERE ip_address = $1 AND tenant_id = $2 AND deleted_at IS NULL LIMIT 1;
```

### Prometheus Response Parsing

Standard Go struct:

```go
type PromQueryResponse struct {
    Status string `json:"status"`
    Data   struct {
        ResultType string `json:"resultType"`
        Result     []struct {
            Metric map[string]string `json:"metric"`
            Value  [2]interface{}    `json:"value"` // [timestamp, "value_string"]
        } `json:"result"`
    } `json:"data"`
}
```

### Error Tracking

Per-adapter failure counter in memory (map[uuid.UUID]int). Not persisted — resets on server restart. This is fine because:
- 3 failures at 5-min intervals = 15 minutes to detect
- Server restart clears the counter, giving the adapter a fresh chance

When count reaches 3:
- `UPDATE integration_adapters SET enabled = false WHERE id = $1`
- Find ops-admin users, create notification: "Adapter {name} disabled after 3 consecutive failures"
- Log error

---

## D2: Adapter Config UI Enhancement

### Current State

CreateAdapterModal has: name, type, direction, endpoint, enabled. No fields for Prometheus-specific config.

### What to Add

When `type == 'rest'` and `direction == 'inbound'`, show additional fields:

- **Metric Queries** — textarea, one PromQL expression per line
- **Pull Interval** — dropdown: 1min, 5min (default), 15min, 30min, 1h

These are stored in the adapter's `config` JSONB:

```json
{
  "queries": ["node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "power_kw"],
  "pull_interval_seconds": 300
}
```

### Frontend Changes

Modify `CreateAdapterModal.tsx`:
- Add conditional fields when type=rest + direction=inbound
- Serialize to config JSONB before API call

No new components needed — just conditional fields in the existing modal.

### Existing Adapters Tab Display

Add a "Metrics" column showing query count: "3 queries, 5m interval" for inbound REST adapters.

---

## D3: Energy Dashboard Integration

### Current State

The Energy dashboard (`EnergyMonitor.tsx`) exists but uses fallback/hardcoded data.

### What to Change

After Prometheus pull is working, the metrics table will have real data. The existing `useMetrics` hook already reads from the metrics API. No frontend changes needed — once real metrics flow into the table, the Energy dashboard will automatically show them.

**Verification only:** After implementing the pull, confirm Energy dashboard shows non-zero values.

---

## Files Changed Summary

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/db/queries/metrics.sql` | InsertMetric + FindAssetByIP queries |
| `cmdb-core/internal/domain/workflows/prometheus.go` | Prometheus HTTP client + response parser |
| `cmdb-core/internal/domain/workflows/prometheus_test.go` | Parser tests |

### Modified Files

| File | Change |
|------|--------|
| `cmdb-core/internal/domain/workflows/subscriber.go` | Replace pullMetricsFromAdapters log-only with real pull |
| `cmdb-core/internal/dbgen/*` | Regenerated (new queries) |
| `cmdb-demo/src/components/CreateAdapterModal.tsx` | Add metric queries + interval fields for inbound REST |

---

## Acceptance Criteria (from milestone plan)

- [ ] Configure Prometheus adapter → metrics table auto-updates every 5 minutes
- [ ] Adapter down 3 times → auto-disabled + ops-admin notified
- [ ] Energy dashboard shows real Prometheus metrics
