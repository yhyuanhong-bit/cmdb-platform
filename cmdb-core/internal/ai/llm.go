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

// LLMProvider uses an OpenAI-compatible chat-completions API for root-cause
// analysis. Works with OpenAI, Claude (via compatible proxy), and local LLM
// servers (e.g. vLLM, Ollama with OpenAI compat).
type LLMProvider struct {
	name     string
	provider string // "openai", "claude", "local_llm"
	endpoint string
	apiKey   string
	model    string
	client   http.Client
}

// NewLLMProvider creates an LLMProvider with a 120-second HTTP timeout.
func NewLLMProvider(name, provider, endpoint, apiKey, model string) *LLMProvider {
	return &LLMProvider{
		name:     name,
		provider: provider,
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		client:   http.Client{Timeout: 120 * time.Second},
	}
}

func (l *LLMProvider) Name() string { return l.name }
func (l *LLMProvider) Type() string { return "llm" }

// AnalyzeRootCause builds a prompt with alerts and assets and calls the LLM.
func (l *LLMProvider) AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error) {
	alertsJSON, _ := json.Marshal(req.RelatedAlerts)
	assetsJSON, _ := json.Marshal(req.AffectedAssets)

	prompt := fmt.Sprintf(`You are a root-cause analysis AI for a CMDB platform.
Given the incident details below, determine the most likely root cause.

Incident ID: %s
Tenant ID: %s
Additional Context: %s

Related Alerts (JSON):
%s

Affected Assets (JSON):
%s

Respond with JSON:
{
  "reasoning": "<step-by-step explanation>",
  "conclusion_asset_id": "<UUID of the root-cause asset or null>",
  "confidence": <0.0-1.0>
}`, req.IncidentID, req.TenantID, req.Context, string(alertsJSON), string(assetsJSON))

	raw, err := l.chatCompletion(ctx, prompt)
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

// HealthCheck verifies the LLM backend is reachable by listing models.
func (l *LLMProvider) HealthCheck(ctx context.Context) error {
	url := l.endpoint + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("llm health: %w", err)
	}
	if l.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("llm health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("llm health: status %d", resp.StatusCode)
	}
	return nil
}

// chatCompletion sends a single-turn prompt to the OpenAI-compatible
// /chat/completions endpoint and returns the assistant message content.
func (l *LLMProvider) chatCompletion(ctx context.Context, prompt string) (json.RawMessage, error) {
	payload := map[string]any{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	url := l.endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: call chat completions: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("llm: chat completions returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse OpenAI response: {"choices": [{"message": {"content": "..."}}]}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return respBody, nil
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("llm: no choices in response")
	}

	content := envelope.Choices[0].Message.Content

	// Try to parse the content as JSON; if it's valid JSON return it directly.
	if json.Valid([]byte(content)) {
		return json.RawMessage(content), nil
	}
	// Otherwise wrap the text content as a JSON string.
	wrapped, _ := json.Marshal(content)
	return wrapped, nil
}
