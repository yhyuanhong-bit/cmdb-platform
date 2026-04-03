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

// CustomProvider is a generic HTTP adapter that forwards prediction and RCA
// requests to a user-managed model service.
type CustomProvider struct {
	name     string
	endpoint string
	client   http.Client
}

// NewCustomProvider creates a CustomProvider with a 30-second HTTP timeout.
func NewCustomProvider(name, endpoint string) *CustomProvider {
	return &CustomProvider{
		name:     name,
		endpoint: endpoint,
		client:   http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CustomProvider) Name() string { return c.name }
func (c *CustomProvider) Type() string { return "ml_model" }

// PredictFailure POSTs the prediction request to /predict.
func (c *CustomProvider) PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error) {
	raw, err := c.post(ctx, "/predict", req)
	if err != nil {
		return nil, err
	}

	var result PredictionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		result = PredictionResult{
			PredictionType: "custom_raw",
			Result:         raw,
			Confidence:     0.5,
		}
	}
	if result.Result == nil {
		result.Result = raw
	}
	return &result, nil
}

// AnalyzeRootCause POSTs the RCA request to /rca.
func (c *CustomProvider) AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error) {
	raw, err := c.post(ctx, "/rca", req)
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
	if result.Reasoning == nil {
		result.Reasoning = raw
	}
	return &result, nil
}

// HealthCheck verifies the custom model service is reachable.
func (c *CustomProvider) HealthCheck(ctx context.Context) error {
	url := c.endpoint + "/health"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("custom health: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("custom health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("custom health: status %d", resp.StatusCode)
	}
	return nil
}

// post sends a JSON POST request to the given path and returns the response body.
func (c *CustomProvider) post(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("custom: marshal request: %w", err)
	}

	url := c.endpoint + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("custom: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("custom: call %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("custom: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("custom: %s returned %d: %s", path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
