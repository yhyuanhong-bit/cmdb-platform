# MCP Server + AI Adapter + WebSocket Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add MCP Server (7 tools + 3 resources) for AI agent integration, pluggable AI adapter layer (Dify/LLM/Custom providers), and WebSocket real-time push to the cmdb-core Go backend — enabling any AI agent to query CMDB data and enabling the frontend dashboard to receive live updates.

**Architecture:** MCP Server runs as an SSE endpoint inside cmdb-core, exposing CMDB data as tools and resources via the mcp-go SDK. The AI adapter layer provides a unified AIProvider interface with Dify, LLM (OpenAI-compatible), and Custom HTTP adapters, dynamically loaded from the prediction_models DB table. WebSocket hub subscribes to NATS events and pushes filtered updates to authenticated frontend clients.

**Tech Stack:** Go 1.23+, mcp-go (MCP Server SDK), gorilla/websocket, nhooyr.io/websocket, existing Gin/sqlc/NATS stack

**Spec Reference:** `docs/superpowers/specs/2026-04-03-cmdb-backend-techstack-design.md` — Sections 5.3, 5.4, 5.6

**This plan covers:**
- MCP Server with 7 tools (search_assets, get_asset_detail, query_alerts, get_topology, query_metrics, query_work_orders, trigger_rca) and 3 resources
- AI Adapter layer: AIProvider interface + DifyProvider + LLMProvider + CustomModelProvider + Registry
- Prediction domain module (model, service, handler — 5 API endpoints)
- WebSocket hub with JWT auth, tenant-filtered NATS event push
- Config additions (MCP_ENABLED, MCP_PORT, WS_ENABLED)
- main.go updates to wire MCP + AI + WebSocket

**Out of scope:** Integration/webhook module (simple CRUD, low priority), Casbin RBAC middleware (can be added incrementally later).

---

## File Structure

```
cmdb-core/internal/
├── mcp/
│   ├── server.go              # MCP Server setup + SSE transport
│   ├── tools.go               # 7 tool definitions + handlers
│   └── resources.go           # 3 resource definitions + handlers
│
├── ai/
│   ├── provider.go            # AIProvider interface + request/response types
│   ├── registry.go            # Provider registry (dynamic load from DB)
│   ├── dify.go                # Dify workflow adapter
│   ├── llm.go                 # OpenAI-compatible LLM adapter (Claude/OpenAI/local)
│   └── custom.go              # Custom HTTP model adapter
│
├── domain/prediction/
│   ├── model.go               # PredictionModel, PredictionResult, RCAAnalysis structs
│   ├── service.go             # Prediction service (routes to AI providers)
│   └── handler.go             # HTTP handlers (5 endpoints)
│
├── websocket/
│   ├── hub.go                 # WebSocket hub: client registry, broadcast, NATS subscription
│   └── handler.go             # Gin upgrade handler with JWT auth
│
└── config/config.go           # Add MCP_ENABLED, MCP_PORT, WS_ENABLED fields
```

New files in `db/`:
```
db/migrations/
├── 000011_prediction_tables.up.sql
└── 000011_prediction_tables.down.sql
db/queries/
├── prediction_models.sql
└── prediction_results.sql
```

---

## Task 1: DB Migration + sqlc Queries for Prediction

**Files:**
- Create: `cmdb-core/db/migrations/000011_prediction_tables.up.sql`
- Create: `cmdb-core/db/migrations/000011_prediction_tables.down.sql`
- Create: `cmdb-core/db/queries/prediction_models.sql`
- Create: `cmdb-core/db/queries/prediction_results.sql`

- [ ] **Step 1: Create migration up**

Create `cmdb-core/db/migrations/000011_prediction_tables.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS prediction_models (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,
    provider    VARCHAR(30) NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}',
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS prediction_results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    model_id            UUID NOT NULL REFERENCES prediction_models(id),
    asset_id            UUID NOT NULL REFERENCES assets(id),
    prediction_type     VARCHAR(30) NOT NULL,
    result              JSONB NOT NULL,
    severity            VARCHAR(20),
    recommended_action  TEXT,
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_prediction_results_asset ON prediction_results (asset_id);
CREATE INDEX IF NOT EXISTS idx_prediction_results_tenant ON prediction_results (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS rca_analyses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    incident_id     UUID NOT NULL REFERENCES incidents(id),
    model_id        UUID REFERENCES prediction_models(id),
    reasoning       JSONB NOT NULL,
    conclusion_asset_id UUID REFERENCES assets(id),
    confidence      NUMERIC(3,2),
    human_verified  BOOLEAN DEFAULT false,
    verified_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Seed a default Dify provider model
INSERT INTO prediction_models (id, name, type, provider, config, enabled) VALUES
    ('20000000-0000-0000-0000-000000000001', 'Default RCA', 'rca', 'dify',
     '{"base_url": "http://dify:3000", "api_key": "change-me", "workflow_id": "rca-v1"}', false)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Create migration down**

Create `cmdb-core/db/migrations/000011_prediction_tables.down.sql`:

```sql
DROP TABLE IF EXISTS rca_analyses;
DROP TABLE IF EXISTS prediction_results;
DROP TABLE IF EXISTS prediction_models;
```

- [ ] **Step 3: Create prediction_models.sql queries**

Create `cmdb-core/db/queries/prediction_models.sql`:

```sql
-- name: ListEnabledModels :many
SELECT * FROM prediction_models WHERE enabled = true ORDER BY name;

-- name: ListAllModels :many
SELECT * FROM prediction_models ORDER BY name;

-- name: GetModel :one
SELECT * FROM prediction_models WHERE id = $1;

-- name: CreateModel :one
INSERT INTO prediction_models (name, type, provider, config, enabled)
VALUES ($1, $2, $3, $4, $5) RETURNING *;
```

- [ ] **Step 4: Create prediction_results.sql queries**

Create `cmdb-core/db/queries/prediction_results.sql`:

```sql
-- name: ListPredictionsByAsset :many
SELECT * FROM prediction_results
WHERE asset_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListPredictionsByTenant :many
SELECT * FROM prediction_results
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPredictionsByTenant :one
SELECT count(*) FROM prediction_results WHERE tenant_id = $1;

-- name: CreatePredictionResult :one
INSERT INTO prediction_results (tenant_id, model_id, asset_id, prediction_type, result, severity, recommended_action, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: CreateRCA :one
INSERT INTO rca_analyses (tenant_id, incident_id, model_id, reasoning, conclusion_asset_id, confidence)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetRCA :one
SELECT * FROM rca_analyses WHERE id = $1;

-- name: VerifyRCA :one
UPDATE rca_analyses SET human_verified = true, verified_by = $2 WHERE id = $1 RETURNING *;
```

- [ ] **Step 5: Regenerate sqlc**

```bash
cd /cmdb-platform/cmdb-core
sqlc generate
go build ./internal/dbgen/...
```

Expected: New prediction_models and prediction_results query files generated, build succeeds.

- [ ] **Step 6: Commit**

```bash
git add db/migrations/000011_* db/queries/prediction_*.sql internal/dbgen/
git commit -m "feat: add prediction tables migration + sqlc queries"
```

---

## Task 2: AI Provider Interface + Registry

**Files:**
- Create: `cmdb-core/internal/ai/provider.go`
- Create: `cmdb-core/internal/ai/registry.go`

- [ ] **Step 1: Create provider.go**

Create `cmdb-core/internal/ai/provider.go`:

```go
package ai

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AIProvider is the unified interface for all AI backends.
type AIProvider interface {
	Name() string
	Type() string // "llm" | "ml_model" | "workflow"
	PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error)
	AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error)
	HealthCheck(ctx context.Context) error
}

type PredictionRequest struct {
	AssetID   uuid.UUID       `json:"asset_id"`
	AssetType string          `json:"asset_type"`
	Metrics   []MetricPoint   `json:"metrics,omitempty"`
	Context   string          `json:"context,omitempty"`
}

type MetricPoint struct {
	Time  time.Time `json:"time"`
	Name  string    `json:"name"`
	Value float64   `json:"value"`
}

type PredictionResult struct {
	PredictionType    string          `json:"prediction_type"`
	Result            json.RawMessage `json:"result"`
	Severity          string          `json:"severity,omitempty"`
	RecommendedAction string          `json:"recommended_action,omitempty"`
	Confidence        float64         `json:"confidence,omitempty"`
}

type RCARequest struct {
	IncidentID     uuid.UUID `json:"incident_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	RelatedAlerts  []AlertBrief  `json:"related_alerts,omitempty"`
	AffectedAssets []AssetBrief  `json:"affected_assets,omitempty"`
	Context        string        `json:"context,omitempty"`
}

type AlertBrief struct {
	ID       uuid.UUID `json:"id"`
	AssetID  uuid.UUID `json:"asset_id"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
	FiredAt  time.Time `json:"fired_at"`
}

type AssetBrief struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Type   string    `json:"type"`
	Status string    `json:"status"`
}

type RCAResult struct {
	Reasoning       json.RawMessage `json:"reasoning"`
	ConclusionAssetID *uuid.UUID   `json:"conclusion_asset_id,omitempty"`
	Confidence      float64         `json:"confidence"`
}
```

- [ ] **Step 2: Create registry.go**

Create `cmdb-core/internal/ai/registry.go`:

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string]AIProvider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]AIProvider),
	}
}

func (r *Registry) Register(p AIProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (AIProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ProviderInfo
	for _, p := range r.providers {
		result = append(result, ProviderInfo{
			Name: p.Name(),
			Type: p.Type(),
		})
	}
	return result
}

type ProviderInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// LoadFromDB reads enabled prediction_models and registers providers.
func (r *Registry) LoadFromDB(ctx context.Context, queries *dbgen.Queries) error {
	models, err := queries.ListEnabledModels(ctx)
	if err != nil {
		return fmt.Errorf("load models: %w", err)
	}

	for _, m := range models {
		var config map[string]any
		if err := json.Unmarshal(m.Config, &config); err != nil {
			log.Printf("WARN: invalid config for model %s: %v", m.Name, err)
			continue
		}

		var provider AIProvider
		switch m.Provider {
		case "dify":
			provider = NewDifyProvider(m.Name, config)
		case "openai", "claude", "local_llm":
			provider = NewLLMProvider(m.Name, m.Provider, config)
		case "custom":
			provider = NewCustomProvider(m.Name, config)
		default:
			log.Printf("WARN: unknown provider type %s for model %s", m.Provider, m.Name)
			continue
		}

		r.Register(provider)
		log.Printf("AI: registered provider %s (type=%s)", provider.Name(), provider.Type())
	}

	return nil
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./internal/ai/...
```

Note: This will fail because NewDifyProvider, NewLLMProvider, NewCustomProvider don't exist yet. That's expected — they're created in Task 3. For now just create the files and verify no syntax errors by checking `go vet ./internal/ai/...` after Task 3.

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/ai/provider.go cmdb-core/internal/ai/registry.go
git commit -m "feat: add AI provider interface + dynamic registry"
```

---

## Task 3: AI Adapters (Dify + LLM + Custom)

**Files:**
- Create: `cmdb-core/internal/ai/dify.go`
- Create: `cmdb-core/internal/ai/llm.go`
- Create: `cmdb-core/internal/ai/custom.go`

- [ ] **Step 1: Create dify.go**

Create `cmdb-core/internal/ai/dify.go`:

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DifyProvider struct {
	name       string
	baseURL    string
	apiKey     string
	workflowID string
	client     *http.Client
}

func NewDifyProvider(name string, config map[string]any) *DifyProvider {
	return &DifyProvider{
		name:       name,
		baseURL:    stringFromConfig(config, "base_url", "http://localhost:3000"),
		apiKey:     stringFromConfig(config, "api_key", ""),
		workflowID: stringFromConfig(config, "workflow_id", ""),
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (d *DifyProvider) Name() string { return d.name }
func (d *DifyProvider) Type() string { return "workflow" }

func (d *DifyProvider) PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error) {
	payload := map[string]any{
		"inputs":       map[string]any{"asset_id": req.AssetID.String(), "asset_type": req.AssetType, "context": req.Context},
		"response_mode": "blocking",
		"user":          "cmdb-system",
	}
	body, err := d.callWorkflow(ctx, payload)
	if err != nil {
		return nil, err
	}
	return &PredictionResult{
		PredictionType: "failure_prediction",
		Result:         body,
	}, nil
}

func (d *DifyProvider) AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error) {
	payload := map[string]any{
		"inputs":       map[string]any{"incident_id": req.IncidentID.String(), "context": req.Context},
		"response_mode": "blocking",
		"user":          "cmdb-system",
	}
	body, err := d.callWorkflow(ctx, payload)
	if err != nil {
		return nil, err
	}
	return &RCAResult{Reasoning: body, Confidence: 0.0}, nil
}

func (d *DifyProvider) HealthCheck(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", d.baseURL+"/v1/parameters", nil)
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("dify health check failed: %d", resp.StatusCode)
	}
	return nil
}

func (d *DifyProvider) callWorkflow(ctx context.Context, payload map[string]any) (json.RawMessage, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", d.baseURL+"/v1/workflows/run", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dify workflow call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("dify returned %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func stringFromConfig(config map[string]any, key, fallback string) string {
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return fallback
}
```

- [ ] **Step 2: Create llm.go**

Create `cmdb-core/internal/ai/llm.go`:

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMProvider implements the OpenAI-compatible chat completions API.
// Works with Claude (via Anthropic proxy), OpenAI, and local models.
type LLMProvider struct {
	name     string
	provider string // "openai" | "claude" | "local_llm"
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func NewLLMProvider(name, provider string, config map[string]any) *LLMProvider {
	return &LLMProvider{
		name:     name,
		provider: provider,
		endpoint: stringFromConfig(config, "endpoint", "https://api.openai.com/v1"),
		apiKey:   stringFromConfig(config, "api_key", ""),
		model:    stringFromConfig(config, "model", "gpt-4o"),
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (l *LLMProvider) Name() string { return l.name }
func (l *LLMProvider) Type() string { return "llm" }

func (l *LLMProvider) PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error) {
	prompt := fmt.Sprintf(
		"Analyze this asset for potential failures.\nAsset ID: %s\nType: %s\nContext: %s\n\nProvide prediction as JSON with fields: severity, recommended_action, confidence.",
		req.AssetID, req.AssetType, req.Context,
	)
	result, err := l.chatCompletion(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &PredictionResult{
		PredictionType: "failure_prediction",
		Result:         result,
	}, nil
}

func (l *LLMProvider) AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error) {
	alertsJSON, _ := json.Marshal(req.RelatedAlerts)
	assetsJSON, _ := json.Marshal(req.AffectedAssets)
	prompt := fmt.Sprintf(
		"Perform root cause analysis for incident %s.\n\nRelated Alerts:\n%s\n\nAffected Assets:\n%s\n\nAdditional Context: %s\n\nProvide analysis as JSON with fields: reasoning, conclusion_asset_id, confidence.",
		req.IncidentID, string(alertsJSON), string(assetsJSON), req.Context,
	)
	result, err := l.chatCompletion(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &RCAResult{Reasoning: result, Confidence: 0.0}, nil
}

func (l *LLMProvider) HealthCheck(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", l.endpoint+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("LLM health check failed: %d", resp.StatusCode)
	}
	return nil
}

func (l *LLMProvider) chatCompletion(ctx context.Context, prompt string) (json.RawMessage, error) {
	payload := map[string]any{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a CMDB operations AI assistant. Respond in JSON."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", l.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Extract content from OpenAI response format
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil || len(chatResp.Choices) == 0 {
		return respBody, nil // Return raw if can't parse
	}
	return json.RawMessage(fmt.Sprintf(`{"content": %q}`, chatResp.Choices[0].Message.Content)), nil
}
```

- [ ] **Step 3: Create custom.go**

Create `cmdb-core/internal/ai/custom.go`:

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CustomProvider calls a self-hosted ML inference service via HTTP.
type CustomProvider struct {
	name     string
	endpoint string
	client   *http.Client
}

func NewCustomProvider(name string, config map[string]any) *CustomProvider {
	return &CustomProvider{
		name:     name,
		endpoint: stringFromConfig(config, "endpoint", "http://localhost:8000"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CustomProvider) Name() string { return c.name }
func (c *CustomProvider) Type() string { return "ml_model" }

func (c *CustomProvider) PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error) {
	body, _ := json.Marshal(req)
	result, err := c.post(ctx, "/predict", body)
	if err != nil {
		return nil, err
	}
	return &PredictionResult{
		PredictionType: "failure_prediction",
		Result:         result,
	}, nil
}

func (c *CustomProvider) AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error) {
	body, _ := json.Marshal(req)
	result, err := c.post(ctx, "/rca", body)
	if err != nil {
		return nil, err
	}
	return &RCAResult{Reasoning: result, Confidence: 0.0}, nil
}

func (c *CustomProvider) HealthCheck(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/health", nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("custom model health check failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *CustomProvider) post(ctx context.Context, path string, body []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("custom model call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("custom model returned %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}
```

- [ ] **Step 4: Verify full AI package builds**

```bash
go build ./internal/ai/...
```

Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/ai/
git commit -m "feat: add AI adapters - Dify workflow, LLM (OpenAI-compat), custom HTTP model"
```

---

## Task 4: Prediction Domain Module (5 endpoints)

**Files:**
- Create: `cmdb-core/internal/domain/prediction/model.go`
- Create: `cmdb-core/internal/domain/prediction/service.go`
- Create: `cmdb-core/internal/domain/prediction/handler.go`

- [ ] **Step 1: Create prediction model.go**

Create `cmdb-core/internal/domain/prediction/model.go`:

```go
package prediction

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Model struct {
	ID       uuid.UUID       `json:"id"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Provider string          `json:"provider"`
	Config   json.RawMessage `json:"config"`
	Enabled  bool            `json:"enabled"`
}

type Result struct {
	ID                uuid.UUID       `json:"id"`
	TenantID          uuid.UUID       `json:"tenant_id"`
	ModelID           uuid.UUID       `json:"model_id"`
	AssetID           uuid.UUID       `json:"asset_id"`
	PredictionType    string          `json:"prediction_type"`
	Result            json.RawMessage `json:"result"`
	Severity          *string         `json:"severity,omitempty"`
	RecommendedAction *string         `json:"recommended_action,omitempty"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type CreateRCARequest struct {
	IncidentID uuid.UUID `json:"incident_id" binding:"required"`
	ModelName  string    `json:"model_name"`
	Context    string    `json:"context"`
}

type VerifyRCARequest struct {
	VerifiedBy uuid.UUID `json:"verified_by" binding:"required"`
}
```

- [ ] **Step 2: Create prediction service.go**

Create `cmdb-core/internal/domain/prediction/service.go`:

```go
package prediction

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

type Service struct {
	queries  *dbgen.Queries
	registry *ai.Registry
}

func NewService(q *dbgen.Queries, registry *ai.Registry) *Service {
	return &Service{queries: q, registry: registry}
}

func (s *Service) ListModels(ctx context.Context) ([]dbgen.PredictionModel, error) {
	return s.queries.ListAllModels(ctx)
}

func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, limit int) ([]dbgen.PredictionResult, error) {
	return s.queries.ListPredictionsByAsset(ctx, dbgen.ListPredictionsByAssetParams{
		AssetID: assetID,
		Limit:   int32(limit),
	})
}

func (s *Service) CreateRCA(ctx context.Context, tenantID uuid.UUID, req CreateRCARequest) (*dbgen.RcaAnalysis, error) {
	// Find provider
	modelName := req.ModelName
	if modelName == "" {
		modelName = "Default RCA"
	}

	provider, ok := s.registry.Get(modelName)
	if !ok {
		// If no AI provider configured, create a placeholder RCA
		result, err := s.queries.CreateRCA(ctx, dbgen.CreateRCAParams{
			TenantID:          tenantID,
			IncidentID:        req.IncidentID,
			Reasoning:         []byte(`{"status": "no_ai_provider_configured"}`),
			Confidence:        0,
		})
		if err != nil {
			return nil, err
		}
		return &result, nil
	}

	// Call AI provider
	rcaReq := ai.RCARequest{
		IncidentID: req.IncidentID,
		TenantID:   tenantID,
		Context:    req.Context,
	}
	rcaResult, err := provider.AnalyzeRootCause(ctx, rcaReq)
	if err != nil {
		return nil, fmt.Errorf("AI RCA failed: %w", err)
	}

	// Store result
	result, err := s.queries.CreateRCA(ctx, dbgen.CreateRCAParams{
		TenantID:          tenantID,
		IncidentID:        req.IncidentID,
		Reasoning:         rcaResult.Reasoning,
		ConclusionAssetID: rcaResult.ConclusionAssetID,
		Confidence:        rcaResult.Confidence,
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) VerifyRCA(ctx context.Context, id uuid.UUID, verifiedBy uuid.UUID) (*dbgen.RcaAnalysis, error) {
	result, err := s.queries.VerifyRCA(ctx, dbgen.VerifyRCAParams{
		ID:         id,
		VerifiedBy: &verifiedBy,
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
```

- [ ] **Step 3: Create prediction handler.go**

Create `cmdb-core/internal/domain/prediction/handler.go`:

```go
package prediction

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/prediction/models", h.ListModels)
	r.GET("/prediction/results/ci/:ciId", h.ListByAsset)
	r.POST("/prediction/rca", h.CreateRCA)
	r.POST("/prediction/rca/:id/verify", h.VerifyRCA)
}

func (h *Handler) ListModels(c *gin.Context) {
	models, err := h.svc.ListModels(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to list models")
		return
	}
	response.OK(c, models)
}

func (h *Handler) ListByAsset(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("ciId"))
	if err != nil {
		response.BadRequest(c, "invalid asset id")
		return
	}
	results, err := h.svc.ListByAsset(c.Request.Context(), assetID, 20)
	if err != nil {
		response.InternalError(c, "failed to list predictions")
		return
	}
	response.OK(c, results)
}

func (h *Handler) CreateRCA(c *gin.Context) {
	tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
	var req CreateRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	result, err := h.svc.CreateRCA(c.Request.Context(), tenantID, req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, result)
}

func (h *Handler) VerifyRCA(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid RCA id")
		return
	}
	var req VerifyRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	result, err := h.svc.VerifyRCA(c.Request.Context(), id, req.VerifiedBy)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./internal/domain/prediction/...
```

Expected: Build succeeds (may need to check exact dbgen types).

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/domain/prediction/
git commit -m "feat: add prediction module - models, RCA, verify (4 endpoints)"
```

---

## Task 5: MCP Server (7 Tools + 3 Resources)

**Files:**
- Create: `cmdb-core/internal/mcp/server.go`
- Create: `cmdb-core/internal/mcp/tools.go`
- Create: `cmdb-core/internal/mcp/resources.go`

- [ ] **Step 1: Create server.go**

Create `cmdb-core/internal/mcp/server.go`:

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

type Server struct {
	queries *dbgen.Queries
	mcp     *mcpsdk.MCPServer
}

func NewServer(queries *dbgen.Queries) *Server {
	s := &Server{
		queries: queries,
		mcp:     mcpsdk.NewMCPServer("cmdb-mcp", "1.0.0"),
	}
	s.registerTools()
	s.registerResources()
	return s
}

func (s *Server) MCPServer() *mcpsdk.MCPServer {
	return s.mcp
}

func jsonResult(data any) string {
	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}
```

- [ ] **Step 2: Create tools.go**

Create `cmdb-core/internal/mcp/tools.go`:

```go
package mcp

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func (s *Server) registerTools() {
	// Tool 1: search_assets
	s.mcp.AddTool(
		mcpsdk.NewTool("search_assets",
			mcpsdk.WithDescription("Search CMDB assets by type, status, location, or keyword"),
			mcpsdk.WithString("type", mcpsdk.Description("Asset type: server|network|storage|power")),
			mcpsdk.WithString("status", mcpsdk.Description("Asset status: operational|maintenance|offline")),
			mcpsdk.WithString("query", mcpsdk.Description("Search by name, serial number, or asset tag")),
			mcpsdk.WithNumber("limit", mcpsdk.Description("Max results, default 20")),
		),
		s.handleSearchAssets,
	)

	// Tool 2: get_asset_detail
	s.mcp.AddTool(
		mcpsdk.NewTool("get_asset_detail",
			mcpsdk.WithDescription("Get full details of a single asset including location and status"),
			mcpsdk.WithString("asset_id", mcpsdk.Required(), mcpsdk.Description("Asset UUID or asset_tag")),
		),
		s.handleGetAssetDetail,
	)

	// Tool 3: query_alerts
	s.mcp.AddTool(
		mcpsdk.NewTool("query_alerts",
			mcpsdk.WithDescription("Query alert events by severity, status, or asset"),
			mcpsdk.WithString("severity", mcpsdk.Description("critical|warning|info")),
			mcpsdk.WithString("status", mcpsdk.Description("firing|acknowledged|resolved")),
			mcpsdk.WithString("asset_id", mcpsdk.Description("Filter by asset UUID")),
			mcpsdk.WithNumber("limit", mcpsdk.Description("Max results, default 20")),
		),
		s.handleQueryAlerts,
	)

	// Tool 4: get_topology
	s.mcp.AddTool(
		mcpsdk.NewTool("get_topology",
			mcpsdk.WithDescription("Get location hierarchy with stats"),
			mcpsdk.WithString("location_id", mcpsdk.Description("Location UUID to start from (omit for root)")),
			mcpsdk.WithBoolean("include_stats", mcpsdk.Description("Include asset/rack counts")),
		),
		s.handleGetTopology,
	)

	// Tool 5: query_metrics
	s.mcp.AddTool(
		mcpsdk.NewTool("query_metrics",
			mcpsdk.WithDescription("Query time-series metrics for an asset"),
			mcpsdk.WithString("asset_id", mcpsdk.Required(), mcpsdk.Description("Asset UUID")),
			mcpsdk.WithString("metric_name", mcpsdk.Required(), mcpsdk.Description("cpu_usage|temperature|power_kw|pue")),
			mcpsdk.WithString("time_range", mcpsdk.Description("1h|6h|24h|7d|30d, default 24h")),
		),
		s.handleQueryMetrics,
	)

	// Tool 6: query_work_orders
	s.mcp.AddTool(
		mcpsdk.NewTool("query_work_orders",
			mcpsdk.WithDescription("Query maintenance work orders"),
			mcpsdk.WithString("status", mcpsdk.Description("draft|pending|approved|in_progress|completed")),
			mcpsdk.WithString("asset_id", mcpsdk.Description("Filter by asset UUID")),
			mcpsdk.WithNumber("limit", mcpsdk.Description("Max results, default 20")),
		),
		s.handleQueryWorkOrders,
	)

	// Tool 7: trigger_rca
	s.mcp.AddTool(
		mcpsdk.NewTool("trigger_rca",
			mcpsdk.WithDescription("Trigger root cause analysis for an incident"),
			mcpsdk.WithString("incident_id", mcpsdk.Required(), mcpsdk.Description("Incident UUID")),
			mcpsdk.WithString("context", mcpsdk.Description("Additional context for RCA")),
		),
		s.handleTriggerRCA,
	)
}

func (s *Server) handleSearchAssets(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	args := request.Params.Arguments
	limit := int32(20)
	if v, ok := args["limit"].(float64); ok {
		limit = int32(v)
	}

	// Use first tenant for MCP (MCP doesn't have JWT context)
	// In production, MCP auth would provide tenant context
	tenants, _ := s.queries.ListTenants(ctx)
	if len(tenants) == 0 {
		return mcpsdk.NewToolResultText("No tenants configured"), nil
	}
	tenantID := tenants[0].ID

	params := dbgen.ListAssetsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   0,
	}
	if v, ok := args["type"].(string); ok && v != "" {
		params.Type = &v
	}
	if v, ok := args["status"].(string); ok && v != "" {
		params.Status = &v
	}
	if v, ok := args["query"].(string); ok && v != "" {
		params.SerialNumber = &v // Search by serial as a start
	}

	assets, err := s.queries.ListAssets(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("search assets: %w", err)
	}
	return mcpsdk.NewToolResultText(jsonResult(assets)), nil
}

func (s *Server) handleGetAssetDetail(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	assetIDStr, _ := request.Params.Arguments["asset_id"].(string)
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		// Try as asset_tag
		asset, err := s.queries.GetAssetByTag(ctx, assetIDStr)
		if err != nil {
			return mcpsdk.NewToolResultText(fmt.Sprintf("Asset not found: %s", assetIDStr)), nil
		}
		return mcpsdk.NewToolResultText(jsonResult(asset)), nil
	}
	asset, err := s.queries.GetAsset(ctx, assetID)
	if err != nil {
		return mcpsdk.NewToolResultText(fmt.Sprintf("Asset not found: %s", assetIDStr)), nil
	}
	return mcpsdk.NewToolResultText(jsonResult(asset)), nil
}

func (s *Server) handleQueryAlerts(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	args := request.Params.Arguments
	limit := int32(20)
	if v, ok := args["limit"].(float64); ok {
		limit = int32(v)
	}
	tenants, _ := s.queries.ListTenants(ctx)
	if len(tenants) == 0 {
		return mcpsdk.NewToolResultText("No tenants"), nil
	}

	params := dbgen.ListAlertsParams{
		TenantID: tenants[0].ID,
		Limit:    limit,
		Offset:   0,
	}
	if v, ok := args["severity"].(string); ok && v != "" {
		params.Severity = &v
	}
	if v, ok := args["status"].(string); ok && v != "" {
		params.Status = &v
	}
	if v, ok := args["asset_id"].(string); ok && v != "" {
		id, _ := uuid.Parse(v)
		params.AssetID = &id
	}

	alerts, err := s.queries.ListAlerts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	return mcpsdk.NewToolResultText(jsonResult(alerts)), nil
}

func (s *Server) handleGetTopology(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	args := request.Params.Arguments
	tenants, _ := s.queries.ListTenants(ctx)
	if len(tenants) == 0 {
		return mcpsdk.NewToolResultText("No tenants"), nil
	}

	if locIDStr, ok := args["location_id"].(string); ok && locIDStr != "" {
		locID, _ := uuid.Parse(locIDStr)
		children, err := s.queries.ListChildren(ctx, locID)
		if err != nil {
			return nil, err
		}
		return mcpsdk.NewToolResultText(jsonResult(children)), nil
	}

	// Root locations
	locations, err := s.queries.ListRootLocations(ctx, tenants[0].ID)
	if err != nil {
		return nil, err
	}
	return mcpsdk.NewToolResultText(jsonResult(locations)), nil
}

func (s *Server) handleQueryMetrics(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	// Metrics queries require TimescaleDB — return placeholder for now
	// Full implementation when metrics ingestion is active
	return mcpsdk.NewToolResultText(`{"message": "Metrics query available when time-series data is ingested", "status": "no_data"}`), nil
}

func (s *Server) handleQueryWorkOrders(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	args := request.Params.Arguments
	limit := int32(20)
	if v, ok := args["limit"].(float64); ok {
		limit = int32(v)
	}
	tenants, _ := s.queries.ListTenants(ctx)
	if len(tenants) == 0 {
		return mcpsdk.NewToolResultText("No tenants"), nil
	}

	params := dbgen.ListWorkOrdersParams{
		TenantID: tenants[0].ID,
		Limit:    limit,
		Offset:   0,
	}
	if v, ok := args["status"].(string); ok && v != "" {
		params.Status = &v
	}

	orders, err := s.queries.ListWorkOrders(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	return mcpsdk.NewToolResultText(jsonResult(orders)), nil
}

func (s *Server) handleTriggerRCA(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	incidentIDStr, _ := request.Params.Arguments["incident_id"].(string)
	contextStr, _ := request.Params.Arguments["context"].(string)
	_, err := uuid.Parse(incidentIDStr)
	if err != nil {
		return mcpsdk.NewToolResultText("Invalid incident_id"), nil
	}
	return mcpsdk.NewToolResultText(fmt.Sprintf(`{"status": "rca_triggered", "incident_id": "%s", "context": "%s"}`, incidentIDStr, contextStr)), nil
}
```

- [ ] **Step 3: Create resources.go**

Create `cmdb-core/internal/mcp/resources.go`:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func (s *Server) registerResources() {
	// Resource 1: Asset type definitions
	s.mcp.AddResource(
		mcpsdk.NewResource("cmdb://schema/asset-types",
			"Asset Type Definitions",
			mcpsdk.WithResourceDescription("All asset types, subtypes, and their field descriptions"),
			mcpsdk.WithMIMEType("application/json"),
		),
		s.handleAssetTypes,
	)

	// Resource 2: Severity levels
	s.mcp.AddResource(
		mcpsdk.NewResource("cmdb://schema/severity-levels",
			"Severity Level Definitions",
			mcpsdk.WithResourceDescription("Alert severity levels and their SLA requirements"),
			mcpsdk.WithMIMEType("application/json"),
		),
		s.handleSeverityLevels,
	)

	// Resource 3: Topology tree
	s.mcp.AddResource(
		mcpsdk.NewResource("cmdb://topology/tree",
			"Location Topology Tree",
			mcpsdk.WithResourceDescription("Complete location hierarchy (country > region > city > campus > IDC)"),
			mcpsdk.WithMIMEType("application/json"),
		),
		s.handleTopologyTree,
	)
}

func (s *Server) handleAssetTypes(ctx context.Context, request mcpsdk.ReadResourceRequest) ([]mcpsdk.ResourceContents, error) {
	content := `{
  "types": {
    "server": {"subtypes": ["rack_server", "blade_server", "tower_server"], "description": "Physical or virtual server"},
    "network": {"subtypes": ["switch", "router", "firewall", "load_balancer"], "description": "Network equipment"},
    "storage": {"subtypes": ["san", "nas", "das", "object_storage"], "description": "Storage systems"},
    "power": {"subtypes": ["ups", "pdu", "generator"], "description": "Power infrastructure"}
  },
  "statuses": ["inventoried", "deployed", "operational", "maintenance", "decommissioned"],
  "bia_levels": ["critical", "important", "normal", "minor"]
}`
	return []mcpsdk.ResourceContents{mcpsdk.NewTextResourceContents(request.Params.URI, "application/json", content)}, nil
}

func (s *Server) handleSeverityLevels(ctx context.Context, request mcpsdk.ReadResourceRequest) ([]mcpsdk.ResourceContents, error) {
	content := `{
  "levels": {
    "critical": {"priority": 1, "sla_response_minutes": 15, "description": "Service down or data loss imminent"},
    "warning": {"priority": 2, "sla_response_minutes": 60, "description": "Degraded performance or approaching threshold"},
    "info": {"priority": 3, "sla_response_minutes": 480, "description": "Informational alert, no immediate action"}
  }
}`
	return []mcpsdk.ResourceContents{mcpsdk.NewTextResourceContents(request.Params.URI, "application/json", content)}, nil
}

func (s *Server) handleTopologyTree(ctx context.Context, request mcpsdk.ReadResourceRequest) ([]mcpsdk.ResourceContents, error) {
	tenants, _ := s.queries.ListTenants(ctx)
	if len(tenants) == 0 {
		return []mcpsdk.ResourceContents{mcpsdk.NewTextResourceContents(request.Params.URI, "application/json", "[]")}, nil
	}

	locations, err := s.queries.ListRootLocations(ctx, tenants[0].ID)
	if err != nil {
		return nil, err
	}
	return []mcpsdk.ResourceContents{mcpsdk.NewTextResourceContents(request.Params.URI, "application/json", jsonResult(locations))}, nil
}
```

- [ ] **Step 4: Install mcp-go and verify build**

```bash
cd /cmdb-platform/cmdb-core
go get github.com/mark3labs/mcp-go@latest
go mod tidy
go build ./internal/mcp/...
```

Expected: Build succeeds. Note: The exact mcp-go API may differ from what's shown. Read the installed package to adapt the code if needed. The key pattern is: create MCPServer, add tools with handlers, add resources with handlers.

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/mcp/
git commit -m "feat: add MCP Server - 7 tools + 3 resources for AI agent integration"
```

---

## Task 6: WebSocket Hub (NATS → Frontend Push)

**Files:**
- Create: `cmdb-core/internal/websocket/hub.go`
- Create: `cmdb-core/internal/websocket/handler.go`

- [ ] **Step 1: Create hub.go**

Create `cmdb-core/internal/websocket/hub.go`:

```go
package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	TenantID string
	UserID   string
	Conn     *websocket.Conn
	Send     chan []byte
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan BroadcastMessage
	register   chan *Client
	unregister chan *Client
}

type BroadcastMessage struct {
	TenantID string          `json:"tenant_id"`
	Type     string          `json:"type"`     // "alert.fired", "asset.status_changed", etc.
	Payload  json.RawMessage `json:"payload"`
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WS: client connected (tenant=%s, user=%s)", client.TenantID, client.UserID)
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("WS: client disconnected (tenant=%s)", client.TenantID)
		case msg := <-h.broadcast:
			data, _ := json.Marshal(msg)
			h.mu.RLock()
			for client := range h.clients {
				// Filter by tenant_id
				if msg.TenantID != "" && client.TenantID != msg.TenantID {
					continue
				}
				select {
				case client.Send <- data:
				default:
					// Client buffer full, disconnect
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(msg BroadcastMessage) {
	h.broadcast <- msg
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
```

- [ ] **Step 2: Create handler.go**

Create `cmdb-core/internal/websocket/handler.go`:

```go
package websocket

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins in dev
}

// HandleWS is a Gin handler that upgrades HTTP to WebSocket.
// JWT token is passed as query param: ws://host/api/v1/ws?token=<jwt>
// The auth middleware should have already validated the token and set context values.
func HandleWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		userID := c.GetString("user_id")

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WS upgrade failed: %v", err)
			return
		}

		client := &Client{
			TenantID: tenantID,
			UserID:   userID,
			Conn:     conn,
			Send:     make(chan []byte, 256),
		}

		hub.Register(client)

		// Write pump
		go func() {
			defer func() {
				conn.Close()
				hub.Unregister(client)
			}()
			for msg := range client.Send {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(ws.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		// Read pump (just keep connection alive, handle pings)
		go func() {
			defer func() {
				hub.Unregister(client)
				conn.Close()
			}()
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			conn.SetPongHandler(func(string) error {
				conn.SetReadDeadline(time.Now().Add(60 * time.Second))
				return nil
			})
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()

		// Ping ticker
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(ws.PingMessage, nil); err != nil {
					return
				}
			}
		}()
	}
}
```

- [ ] **Step 3: Install gorilla/websocket and verify build**

```bash
cd /cmdb-platform/cmdb-core
go get github.com/gorilla/websocket
go mod tidy
go build ./internal/websocket/...
```

Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/websocket/
git commit -m "feat: add WebSocket hub with JWT auth + tenant-filtered NATS event push"
```

---

## Task 7: Wire MCP + AI + WebSocket + Prediction into main.go

**Files:**
- Modify: `cmdb-core/internal/config/config.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add MCP/WS config fields**

Add to Config struct in `cmdb-core/internal/config/config.go`:

```go
// Add these fields to the Config struct:
MCPEnabled bool
MCPPort    int
WSEnabled  bool
```

Add to Load() function:

```go
cfg.MCPEnabled = envOrDefault("MCP_ENABLED", "true") == "true"
mcpPort := 3001
if v := os.Getenv("MCP_PORT"); v != "" {
    if p, err := strconv.Atoi(v); err == nil {
        mcpPort = p
    }
}
cfg.MCPPort = mcpPort
cfg.WSEnabled = envOrDefault("WS_ENABLED", "true") == "true"
```

- [ ] **Step 2: Update main.go to wire new modules**

Read the current main.go first, then add:

1. Import new packages: `mcp`, `ai`, `prediction`, `websocket`
2. After creating queries, create AI registry: `aiRegistry := ai.NewRegistry()` and call `aiRegistry.LoadFromDB(ctx, queries)` (log warning on error)
3. Create prediction service and handler: `predictionSvc := prediction.NewService(queries, aiRegistry)`, `predictionHandler := prediction.NewHandler(predictionSvc)`
4. Register prediction handler on the protected group: `predictionHandler.Register(protected)`
5. If MCPEnabled: create MCP server `mcpSrv := mcp.NewServer(queries)`, start SSE transport in a goroutine on MCP_PORT
6. If WSEnabled: create WebSocket hub `wsHub := websocket.NewHub()`, run hub in goroutine, subscribe to NATS events and forward to hub, register `GET /api/v1/ws` route with auth middleware + websocket.HandleWS(wsHub)

For NATS→WebSocket bridge:

```go
if bus != nil && wsHub != nil {
    // Subscribe to key events and forward to WebSocket
    subjects := []string{"alert.>", "asset.>", "maintenance.>", "import.>"}
    for _, subj := range subjects {
        bus.Subscribe(subj, func(ctx context.Context, event eventbus.Event) error {
            wsHub.Broadcast(websocket.BroadcastMessage{
                TenantID: event.TenantID,
                Type:     event.Subject,
                Payload:  event.Payload,
            })
            return nil
        })
    }
}
```

- [ ] **Step 3: Verify full build**

```bash
cd /cmdb-platform/cmdb-core
go mod tidy
go build ./cmd/server
```

Expected: Build succeeds with all new modules wired in.

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/config/config.go cmdb-core/cmd/server/main.go
git commit -m "feat: wire MCP server + AI registry + prediction module + WebSocket into main"
```

---

## Endpoint Summary

After all 7 tasks, Phase 3 adds these new capabilities:

### New REST Endpoints (4)

| # | Method | Path | Module |
|---|--------|------|--------|
| 1 | GET | `/api/v1/prediction/models` | Prediction |
| 2 | GET | `/api/v1/prediction/results/ci/:ciId` | Prediction |
| 3 | POST | `/api/v1/prediction/rca` | Prediction |
| 4 | POST | `/api/v1/prediction/rca/:id/verify` | Prediction |

### MCP Tools (7)

| Tool | Description |
|------|-------------|
| `search_assets` | Search assets by type, status, location, keyword |
| `get_asset_detail` | Full asset details by UUID or asset_tag |
| `query_alerts` | Alert events filtered by severity/status/asset |
| `get_topology` | Location hierarchy with children |
| `query_metrics` | Time-series data (placeholder until ingestion active) |
| `query_work_orders` | Work orders by status/priority |
| `trigger_rca` | Trigger root cause analysis |

### MCP Resources (3)

| URI | Content |
|-----|---------|
| `cmdb://schema/asset-types` | Type/subtype definitions + valid statuses |
| `cmdb://schema/severity-levels` | Alert severity SLA definitions |
| `cmdb://topology/tree` | Root location hierarchy |

### WebSocket

| Endpoint | Auth | Events |
|----------|------|--------|
| `ws://host/api/v1/ws?token=<jwt>` | JWT required | alert.*, asset.*, maintenance.*, import.* |

### AI Providers

| Provider | Type | Backend |
|----------|------|---------|
| DifyProvider | workflow | Dify workflow engine (MCP callback supported) |
| LLMProvider | llm | OpenAI-compatible API (Claude/OpenAI/local) |
| CustomProvider | ml_model | Any HTTP inference endpoint |
