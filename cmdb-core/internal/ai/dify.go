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

// DifyProvider calls a Dify.ai workflow backend for prediction and RCA.
type DifyProvider struct {
	name       string
	baseURL    string
	apiKey     string
	workflowID string
	client     http.Client
}

// NewDifyProvider creates a DifyProvider with a 60-second HTTP timeout.
func NewDifyProvider(name, baseURL, apiKey, workflowID string) *DifyProvider {
	return &DifyProvider{
		name:       name,
		baseURL:    baseURL,
		apiKey:     apiKey,
		workflowID: workflowID,
		client:     http.Client{Timeout: 60 * time.Second},
	}
}

func (d *DifyProvider) Name() string { return d.name }
func (d *DifyProvider) Type() string { return "workflow" }

// PredictFailure invokes the Dify workflow with asset metrics.
func (d *DifyProvider) PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error) {
	metricsJSON, err := json.Marshal(req.Metrics)
	if err != nil {
		return nil, fmt.Errorf("dify: marshal metrics: %w", err)
	}

	inputs := map[string]any{
		"task":       "predict_failure",
		"asset_id":   req.AssetID.String(),
		"asset_type": req.AssetType,
		"metrics":    string(metricsJSON),
		"context":    req.Context,
	}

	raw, err := d.callWorkflow(ctx, inputs)
	if err != nil {
		return nil, err
	}

	var result PredictionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		// If the workflow returns a non-structured answer, wrap it.
		result = PredictionResult{
			PredictionType: "workflow_raw",
			Result:         raw,
			Confidence:     0.5,
		}
	}
	return &result, nil
}

// AnalyzeRootCause invokes the Dify workflow with incident context.
func (d *DifyProvider) AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error) {
	alertsJSON, _ := json.Marshal(req.RelatedAlerts)
	assetsJSON, _ := json.Marshal(req.AffectedAssets)

	inputs := map[string]any{
		"task":        "root_cause_analysis",
		"incident_id": req.IncidentID.String(),
		"tenant_id":   req.TenantID.String(),
		"alerts":      string(alertsJSON),
		"assets":      string(assetsJSON),
		"context":     req.Context,
	}

	raw, err := d.callWorkflow(ctx, inputs)
	if err != nil {
		return nil, err
	}

	var result RCAResult
	if err := json.Unmarshal(raw, &result); err != nil {
		result = RCAResult{
			Reasoning:  raw,
			Confidence: 0.5,
		}
	}
	return &result, nil
}

// HealthCheck verifies the Dify backend is reachable.
func (d *DifyProvider) HealthCheck(ctx context.Context) error {
	url := d.baseURL + "/v1/parameters"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("dify health: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("dify health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("dify health: status %d", resp.StatusCode)
	}
	return nil
}

// callWorkflow POSTs a workflow run request and returns the output JSON.
func (d *DifyProvider) callWorkflow(ctx context.Context, inputs map[string]any) (json.RawMessage, error) {
	payload := map[string]any{
		"inputs":        inputs,
		"response_mode": "blocking",
		"user":          "cmdb-platform",
	}
	if d.workflowID != "" {
		payload["workflow_id"] = d.workflowID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("dify: marshal payload: %w", err)
	}

	url := d.baseURL + "/v1/workflows/run"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("dify: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("dify: call workflow: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dify: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("dify: workflow returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Dify wraps output in {"data": {"outputs": {...}}}
	var envelope struct {
		Data struct {
			Outputs json.RawMessage `json:"outputs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		// If we can't parse the envelope, return the raw body.
		return respBody, nil
	}
	if len(envelope.Data.Outputs) > 0 {
		return envelope.Data.Outputs, nil
	}
	return respBody, nil
}
