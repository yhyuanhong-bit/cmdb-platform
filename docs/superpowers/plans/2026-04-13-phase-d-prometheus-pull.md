# v1.2 Phase D: Prometheus Metrics Pull Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing MetricsPuller fetch real metrics from Prometheus, write them to the TimescaleDB metrics table, and auto-disable failing adapters with ops-admin notification.

**Architecture:** Extend `pullMetricsFromAdapters()` in subscriber.go to do actual HTTP GET to Prometheus `/api/v1/query`, parse the response, map IP→asset, and INSERT into metrics. Prometheus client logic extracted to its own file. Frontend CreateAdapterModal gets conditional fields for query list and interval.

**Tech Stack:** Go 1.22+, net/http, Prometheus HTTP API, TimescaleDB, sqlc, React

**Design Spec:** `docs/superpowers/specs/2026-04-13-phase-d-prometheus-pull-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-core/internal/domain/workflows/prometheus.go` | Prometheus HTTP client + response parser |
| `cmdb-core/internal/domain/workflows/prometheus_test.go` | Parser unit tests |

### Modified Files

| File | Change |
|------|--------|
| `cmdb-core/db/queries/metrics.sql` | Add InsertMetric query |
| `cmdb-core/internal/dbgen/*` | Regenerated |
| `cmdb-core/internal/domain/workflows/subscriber.go` | Rewrite pullMetricsFromAdapters with real pull logic |
| `cmdb-demo/src/components/CreateAdapterModal.tsx` | Add queries textarea + interval dropdown for inbound REST |

---

## Task 1: sqlc Query — InsertMetric

**Files:**
- Modify: `cmdb-core/db/queries/metrics.sql`

- [ ] **Step 1: Add InsertMetric query**

Append to `cmdb-core/db/queries/metrics.sql`:

```sql
-- name: InsertMetric :exec
INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels)
VALUES ($1, $2, $3, $4, $5, $6);
```

- [ ] **Step 2: Run sqlc generate**

```bash
cd /cmdb-platform/cmdb-core && sqlc generate
```

Verify: `grep -n 'InsertMetric' internal/dbgen/metrics.sql.go`
Expected: Function `InsertMetric` exists.

- [ ] **Step 3: Fix any build errors from sqlc regeneration**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

If build errors occur in unrelated packages (type mismatches from regen), fix them.

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/db/queries/metrics.sql cmdb-core/internal/dbgen/
git commit -m "feat(metrics): add InsertMetric sqlc query for Prometheus pull"
```

---

## Task 2: Prometheus Client + Parser

**Files:**
- Create: `cmdb-core/internal/domain/workflows/prometheus.go`
- Create: `cmdb-core/internal/domain/workflows/prometheus_test.go`

- [ ] **Step 1: Write parser tests**

Create `cmdb-core/internal/domain/workflows/prometheus_test.go`:

```go
package workflows

import (
	"testing"
)

func TestParsePromResponse(t *testing.T) {
	raw := []byte(`{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"__name__": "node_cpu_seconds_total", "instance": "10.0.1.5:9100", "mode": "idle"},
					"value": [1713000000, "42.5"]
				},
				{
					"metric": {"__name__": "node_cpu_seconds_total", "instance": "10.0.1.6:9100", "mode": "idle"},
					"value": [1713000000, "38.2"]
				}
			]
		}
	}`)

	results, err := parsePromResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].MetricName != "node_cpu_seconds_total" {
		t.Errorf("name = %q, want node_cpu_seconds_total", results[0].MetricName)
	}
	if results[0].IP != "10.0.1.5" {
		t.Errorf("ip = %q, want 10.0.1.5", results[0].IP)
	}
	if results[0].Value != 42.5 {
		t.Errorf("value = %f, want 42.5", results[0].Value)
	}
	if results[1].IP != "10.0.1.6" {
		t.Errorf("second ip = %q, want 10.0.1.6", results[1].IP)
	}
}

func TestParsePromResponse_NoInstance(t *testing.T) {
	raw := []byte(`{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"__name__": "up"},
					"value": [1713000000, "1"]
				}
			]
		}
	}`)

	results, err := parsePromResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IP != "" {
		t.Errorf("ip should be empty for metrics without instance, got %q", results[0].IP)
	}
}

func TestParsePromResponse_ErrorStatus(t *testing.T) {
	raw := []byte(`{"status": "error", "errorType": "bad_data", "error": "invalid query"}`)
	_, err := parsePromResponse(raw)
	if err == nil {
		t.Fatal("expected error for error status")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		instance string
		want     string
	}{
		{"10.0.1.5:9100", "10.0.1.5"},
		{"10.0.1.5", "10.0.1.5"},
		{"hostname:9100", "hostname"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.instance, func(t *testing.T) {
			if got := extractIP(tt.instance); got != tt.want {
				t.Errorf("extractIP(%q) = %q, want %q", tt.instance, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /cmdb-platform/cmdb-core && go test ./internal/domain/workflows/ -run 'TestParseProm|TestExtractIP' -v
```
Expected: FAIL — functions not defined.

- [ ] **Step 3: Implement Prometheus client**

Create `cmdb-core/internal/domain/workflows/prometheus.go`:

```go
package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// promResult represents a single parsed metric data point from Prometheus.
type promResult struct {
	MetricName string
	IP         string            // extracted from instance label
	Value      float64
	Timestamp  time.Time
	Labels     map[string]string // all original labels
}

// promQueryResponse models the Prometheus /api/v1/query JSON response.
type promQueryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// fetchPromMetrics queries a Prometheus endpoint and returns parsed results.
func fetchPromMetrics(ctx context.Context, endpoint, query string) ([]promResult, error) {
	client := http.Client{Timeout: 10 * time.Second}

	u := fmt.Sprintf("%s/query?query=%s", strings.TrimRight(endpoint, "/"), url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, string(body))
	}

	return parsePromResponse(body)
}

// parsePromResponse parses raw Prometheus JSON into promResult slice.
func parsePromResponse(raw []byte) ([]promResult, error) {
	var resp promQueryResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", resp.Error)
	}

	var results []promResult
	for _, r := range resp.Data.Result {
		// Parse timestamp
		var ts float64
		if err := json.Unmarshal(r.Value[0], &ts); err != nil {
			continue
		}

		// Parse value
		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		name := r.Metric["__name__"]
		instance := r.Metric["instance"]

		results = append(results, promResult{
			MetricName: name,
			IP:         extractIP(instance),
			Value:      val,
			Timestamp:  time.Unix(int64(ts), 0),
			Labels:     r.Metric,
		})
	}

	return results, nil
}

// extractIP strips the port from a Prometheus instance label (e.g., "10.0.1.5:9100" → "10.0.1.5").
func extractIP(instance string) string {
	if instance == "" {
		return ""
	}
	if idx := strings.LastIndex(instance, ":"); idx > 0 {
		return instance[:idx]
	}
	return instance
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd /cmdb-platform/cmdb-core && go test ./internal/domain/workflows/ -run 'TestParseProm|TestExtractIP' -v
```
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/domain/workflows/prometheus.go cmdb-core/internal/domain/workflows/prometheus_test.go
git commit -m "feat(metrics): add Prometheus HTTP client + response parser with tests"
```

---

## Task 3: Rewrite pullMetricsFromAdapters

**Files:**
- Modify: `cmdb-core/internal/domain/workflows/subscriber.go`

- [ ] **Step 1: Add failure tracker field to WorkflowSubscriber**

At the top of `subscriber.go`, add to the struct:

```go
type WorkflowSubscriber struct {
	pool             *pgxpool.Pool
	queries          *dbgen.Queries
	bus              eventbus.Bus
	maintenanceSvc   *maintenance.Service
	adapterFailures  map[uuid.UUID]int // consecutive failure count per adapter
}
```

In the `New()` constructor, initialize the map:

```go
func New(pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus, maintenanceSvc *maintenance.Service) *WorkflowSubscriber {
	return &WorkflowSubscriber{pool: pool, queries: queries, bus: bus, maintenanceSvc: maintenanceSvc, adapterFailures: make(map[uuid.UUID]int)}
}
```

- [ ] **Step 2: Replace pullMetricsFromAdapters**

Replace the entire `pullMetricsFromAdapters` function:

```go
func (w *WorkflowSubscriber) pullMetricsFromAdapters(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT id, tenant_id, name, type, endpoint, config FROM integration_adapters
		 WHERE direction = 'inbound' AND enabled = true`)
	if err != nil {
		zap.L().Warn("metrics puller: failed to query adapters", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tenantID uuid.UUID
		var name, adapterType string
		var endpoint *string
		var configRaw []byte
		if rows.Scan(&id, &tenantID, &name, &adapterType, &endpoint, &configRaw) != nil {
			continue
		}

		if endpoint == nil || *endpoint == "" {
			continue
		}

		// Parse config for query list
		var cfg struct {
			Queries            []string `json:"queries"`
			PullIntervalSeconds int     `json:"pull_interval_seconds"`
		}
		json.Unmarshal(configRaw, &cfg)

		if len(cfg.Queries) == 0 {
			zap.L().Debug("metrics puller: no queries configured", zap.String("adapter", name))
			continue
		}

		pullErr := w.pullFromAdapter(ctx, id, tenantID, name, *endpoint, cfg.Queries)
		if pullErr != nil {
			w.adapterFailures[id]++
			zap.L().Warn("metrics puller: pull failed",
				zap.String("adapter", name),
				zap.Int("consecutive_failures", w.adapterFailures[id]),
				zap.Error(pullErr))

			if w.adapterFailures[id] >= 3 {
				w.disableAdapter(ctx, id, tenantID, name)
			}
		} else {
			w.adapterFailures[id] = 0
		}
	}
}

func (w *WorkflowSubscriber) pullFromAdapter(ctx context.Context, adapterID, tenantID uuid.UUID, name, endpoint string, queries []string) error {
	for _, query := range queries {
		results, err := fetchPromMetrics(ctx, endpoint, query)
		if err != nil {
			return fmt.Errorf("query %q: %w", query, err)
		}

		for _, r := range results {
			// Lookup asset by IP
			var assetID pgtype.UUID
			if r.IP != "" {
				asset, err := w.queries.FindAssetByIP(ctx, dbgen.FindAssetByIPParams{
					TenantID:  tenantID,
					IpAddress: pgtype.Text{String: r.IP, Valid: true},
				})
				if err == nil {
					assetID = pgtype.UUID{Bytes: asset.ID, Valid: true}
				}
			}

			// Store labels as JSONB
			labelsJSON, _ := json.Marshal(r.Labels)

			w.pool.Exec(ctx,
				"INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels) VALUES ($1, $2, $3, $4, $5, $6)",
				r.Timestamp, assetID, tenantID, r.MetricName, r.Value, labelsJSON)
		}

		zap.L().Debug("metrics puller: stored metrics",
			zap.String("adapter", name),
			zap.String("query", query),
			zap.Int("count", len(results)))
	}
	return nil
}

func (w *WorkflowSubscriber) disableAdapter(ctx context.Context, adapterID, tenantID uuid.UUID, name string) {
	_, err := w.pool.Exec(ctx,
		"UPDATE integration_adapters SET enabled = false WHERE id = $1", adapterID)
	if err != nil {
		zap.L().Error("metrics puller: failed to disable adapter", zap.String("adapter", name), zap.Error(err))
		return
	}

	zap.L().Warn("metrics puller: adapter disabled after 3 consecutive failures", zap.String("adapter", name))

	// Notify ops-admin users
	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.createNotification(ctx, tenantID, adminID,
			"adapter_error",
			fmt.Sprintf("Adapter '%s' disabled", name),
			fmt.Sprintf("The inbound adapter '%s' has been automatically disabled after 3 consecutive pull failures.", name),
			"integration_adapter", adapterID)
	}

	delete(w.adapterFailures, adapterID)
}
```

Note: Check the exact signature of `FindAssetByIP` from the sqlc-generated code. The param struct might use `IpAddress` as a `pgtype.Text` or plain `string`. Adjust accordingly.

- [ ] **Step 3: Add tenant_id to the adapter query**

The existing query in `pullMetricsFromAdapters` selects `id, name, type, endpoint, config` but NOT `tenant_id`. The updated version needs it. Make sure the SELECT includes `tenant_id`:

```sql
SELECT id, tenant_id, name, type, endpoint, config FROM integration_adapters
WHERE direction = 'inbound' AND enabled = true
```

This is already done in the replacement code above.

- [ ] **Step 4: Verify build**

```bash
cd /cmdb-platform/cmdb-core && go build ./...
```

- [ ] **Step 5: Run all tests**

```bash
cd /cmdb-platform/cmdb-core && go test ./... -count=1
```

- [ ] **Step 6: Commit**

```bash
git add cmdb-core/internal/domain/workflows/subscriber.go
git commit -m "feat(metrics): rewrite pullMetricsFromAdapters — real Prometheus pull + health check"
```

---

## Task 4: Frontend — Adapter Config UI Enhancement

**Files:**
- Modify: `cmdb-demo/src/components/CreateAdapterModal.tsx`

- [ ] **Step 1: Add config fields to the modal**

Replace the entire `CreateAdapterModal.tsx`:

```tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateAdapter } from '../hooks/useIntegration'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  type: 'rest',
  direction: 'inbound',
  endpoint: '',
  enabled: true,
  queries: '',
  pull_interval: '300',
}

export default function CreateAdapterModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateAdapter()

  if (!open) return null

  const isInboundRest = formData.type === 'rest' && formData.direction === 'inbound'

  const handleCreate = () => {
    const payload: Record<string, unknown> = {
      name: formData.name,
      type: formData.type,
      direction: formData.direction,
      endpoint: formData.endpoint,
      enabled: formData.enabled,
    }

    if (isInboundRest && formData.queries.trim()) {
      payload.config = {
        queries: formData.queries.split('\n').map(q => q.trim()).filter(Boolean),
        pull_interval_seconds: parseInt(formData.pull_interval, 10),
      }
    }

    mutation.mutate(payload as any, {
      onSuccess: () => { onClose(); setFormData({ ...initial }) },
    })
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('adapter_modal.title')}</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('adapter_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.type_label')}</label>
          <select value={formData.type} onChange={e => setFormData(p => ({ ...p, type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="dify">Dify</option>
            <option value="rest">REST</option>
            <option value="grpc">gRPC</option>
            <option value="mqtt">MQTT</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.direction_label')}</label>
          <select value={formData.direction} onChange={e => setFormData(p => ({ ...p, direction: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="inbound">{t('adapter_modal.direction_inbound')}</option>
            <option value="outbound">{t('adapter_modal.direction_outbound')}</option>
            <option value="bidirectional">{t('adapter_modal.direction_bidirectional')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.endpoint_label')}</label>
          <input value={formData.endpoint} onChange={e => setFormData(p => ({ ...p, endpoint: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="https://..." />
        </div>

        {isInboundRest && (
          <>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Metric Queries (one per line)</label>
              <textarea
                value={formData.queries}
                onChange={e => setFormData(p => ({ ...p, queries: e.target.value }))}
                rows={4}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm font-mono"
                placeholder={"node_cpu_seconds_total\nnode_memory_MemAvailable_bytes\npower_kw"}
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Pull Interval</label>
              <select value={formData.pull_interval} onChange={e => setFormData(p => ({ ...p, pull_interval: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
                <option value="60">1 minute</option>
                <option value="300">5 minutes (default)</option>
                <option value="900">15 minutes</option>
                <option value="1800">30 minutes</option>
                <option value="3600">1 hour</option>
              </select>
            </div>
          </>
        )}

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={formData.enabled} onChange={e => setFormData(p => ({ ...p, enabled: e.target.checked }))}
            className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">{t('adapter_modal.enabled_label')}</label>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('adapter_modal.btn_cancel')}</button>
          <button
            onClick={handleCreate}
            disabled={mutation.isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('adapter_modal.btn_creating') : t('adapter_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verify frontend builds**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/components/CreateAdapterModal.tsx
git commit -m "feat(ui): add metric queries + pull interval fields to adapter modal"
```

---

## Task 5: Update Seed Data + Verification

**Files:** Verification only (+ optional seed data update)

- [ ] **Step 1: Update Prometheus adapter seed data with queries config**

The existing seed adapter has no config. Update it:

```bash
cd /cmdb-platform/cmdb-core
psql "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" -c "
UPDATE integration_adapters 
SET config = '{\"queries\": [\"node_cpu_seconds_total\", \"node_memory_MemAvailable_bytes\", \"power_kw\"], \"pull_interval_seconds\": 300}'::jsonb
WHERE name = 'Prometheus Metrics' AND type = 'rest'
"
```

- [ ] **Step 2: Full backend build + tests**

```bash
cd /cmdb-platform/cmdb-core && go build ./... && go test ./... -count=1
```

- [ ] **Step 3: Restart server**

```bash
kill $(lsof -t -i :8080) 2>/dev/null
cd /cmdb-platform/cmdb-core && go build -o cmdb-server ./cmd/server/ && ./cmdb-server &
sleep 2
```

- [ ] **Step 4: Verify metrics puller runs**

Check server logs for metrics puller activity:
```bash
# The puller runs every 5 minutes. Check logs after waiting or trigger manually.
# If Prometheus is not running, expect "pull failed" warnings — this is correct behavior.
# The important thing is that the code runs without panicking.
```

- [ ] **Step 5: Verify adapter modal**

Start frontend dev server and test:
```bash
cd /cmdb-platform/cmdb-demo && npm run dev
```

Navigate to `/system/settings` → Integrations tab → click "Add Adapter":
1. Select type "REST" and direction "Inbound"
2. Verify "Metric Queries" textarea and "Pull Interval" dropdown appear
3. Change type to "Dify" — verify fields disappear
4. Change back to "REST Inbound" — verify fields reappear

- [ ] **Step 6: Frontend build verification**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit && npm run build
```

- [ ] **Step 7: Tag Phase D**

```bash
cd /cmdb-platform && git tag v1.2-phase-d
```
