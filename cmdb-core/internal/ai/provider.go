package ai

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AIProvider defines the interface for all AI/ML prediction backends.
//
// PredictFailure was removed in Phase 2.12 (YAGNI): the endpoint was never
// wired to a scheduler or a real caller, the three provider implementations
// were stubs against external services that no production code ever invoked,
// and the prediction_results table was only populated by seed data. If a
// real predictive-maintenance feature is ever needed, re-introduce a
// dedicated interface at that time rather than resurrecting this one.
type AIProvider interface {
	// Name returns the unique provider instance name.
	Name() string
	// Type returns the provider category: "llm", "ml_model", or "workflow".
	Type() string
	// AnalyzeRootCause performs root-cause analysis on an incident.
	AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error)
	// HealthCheck verifies the provider backend is reachable.
	HealthCheck(ctx context.Context) error
}

// RCARequest carries the input for a root-cause analysis call.
type RCARequest struct {
	IncidentID     uuid.UUID    `json:"incident_id"`
	TenantID       uuid.UUID    `json:"tenant_id"`
	RelatedAlerts  []AlertBrief `json:"related_alerts"`
	AffectedAssets []AssetBrief `json:"affected_assets"`
	Context        string       `json:"context,omitempty"`
}

// AlertBrief is a lightweight alert summary used in RCA requests.
type AlertBrief struct {
	ID       uuid.UUID `json:"id"`
	AssetID  uuid.UUID `json:"asset_id"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
	FiredAt  time.Time `json:"fired_at"`
}

// AssetBrief is a lightweight asset summary used in RCA requests.
type AssetBrief struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Type   string    `json:"type"`
	Status string    `json:"status"`
}

// RCAResult holds the output of a root-cause analysis.
type RCAResult struct {
	Reasoning         json.RawMessage `json:"reasoning"`
	ConclusionAssetID *uuid.UUID      `json:"conclusion_asset_id,omitempty"`
	Confidence        float64         `json:"confidence"`
}
