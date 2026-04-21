package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// CustomRESTAdapter implements MetricsAdapter for any REST API with configurable JSON path.
// It consults the package-level SSRF guard (configured via SetNetGuard at
// startup) to reject loopback/metadata/private targets — both at URL-parse
// time and again at DialContext to defeat DNS rebinding.
type CustomRESTAdapter struct{}

type customRESTConfig struct {
	URL            string            `json:"url"`             // Full URL (overrides endpoint if set)
	Headers        map[string]string `json:"headers"`         // Custom headers (e.g., Authorization)
	Method         string            `json:"method"`          // GET or POST, default GET
	Body           string            `json:"body"`            // Request body for POST
	ResultPath     string            `json:"result_path"`     // JSONPath-like dot notation to results array, e.g., "data.metrics"
	NameField      string            `json:"name_field"`      // Field name for metric name, default "name"
	ValueField     string            `json:"value_field"`     // Field name for value, default "value"
	TimestampField string            `json:"timestamp_field"` // Field name for timestamp, default "timestamp"
	IPField        string            `json:"ip_field"`        // Field name for IP, default "ip"
}

func (a *CustomRESTAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	var cfg customRESTConfig
	// An invalid config JSON must reject the fetch rather than silently
	// fall back to an all-zero struct — the pre-fix code happily issued
	// the request with empty headers/body/URL override, masking operator
	// mistakes. Return the parse error so the caller can surface it and
	// the adapter failure counter bumps for the right reason.
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("custom_rest: parse config: %w", err)
		}
	}

	targetURL := endpoint
	if cfg.URL != "" {
		targetURL = cfg.URL
	}
	method := "GET"
	if cfg.Method != "" {
		method = strings.ToUpper(cfg.Method)
	}

	// SSRF defense — refuse loopback/metadata/private targets before dial.
	guard := GetNetGuard()
	if err := guard.ValidateURL(targetURL); err != nil {
		return nil, fmt.Errorf("custom_rest: target rejected by netguard: %w", err)
	}

	var bodyReader *strings.Reader
	if cfg.Body != "" {
		bodyReader = strings.NewReader(cfg.Body)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, targetURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	// Guarded transport re-checks the resolved IP at dial time (defeats
	// DNS rebinding between ValidateURL and Do()). otelhttp wraps the
	// guarded transport so each outbound call produces a client span
	// stitched under the puller's parent context.
	client := http.Client{
		Transport: otelhttp.NewTransport(
			guard.SafeTransport(nil),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return "adapter.fetch " + r.Method
			}),
		),
		Timeout: 15 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response. A malformed response body is a real adapter
	// failure — the pre-fix `//nolint:errcheck` swallowed it, leaving
	// `raw` as nil and the downstream ResultPath walk silently
	// producing zero metrics. Propagate it so the adapter failure
	// counter ticks and the ops team sees the broken upstream.
	var raw interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("custom_rest: parse response: %w", err)
	}

	// Navigate to result path
	data := raw
	if cfg.ResultPath != "" {
		for _, key := range strings.Split(cfg.ResultPath, ".") {
			if m, ok := data.(map[string]interface{}); ok {
				data = m[key]
			}
		}
	}

	// Extract metric points
	arr, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("result_path did not resolve to an array")
	}

	nameField := cfg.NameField
	if nameField == "" {
		nameField = "name"
	}
	valueField := cfg.ValueField
	if valueField == "" {
		valueField = "value"
	}
	timestampField := cfg.TimestampField
	if timestampField == "" {
		timestampField = "timestamp"
	}
	ipField := cfg.IPField
	if ipField == "" {
		ipField = "ip"
	}

	var points []MetricPoint
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m[nameField].(string)
		val := 0.0
		switch v := m[valueField].(type) {
		case float64:
			val = v
		case string:
			val, _ = strconv.ParseFloat(v, 64)
		}
		ip, _ := m[ipField].(string)

		ts := time.Now()
		if tsRaw, ok := m[timestampField]; ok {
			switch v := tsRaw.(type) {
			case float64:
				ts = time.Unix(int64(v), 0)
			case string:
				if parsed, err := time.Parse(time.RFC3339, v); err == nil {
					ts = parsed
				}
			}
		}

		points = append(points, MetricPoint{
			Name:      name,
			Value:     val,
			Timestamp: ts,
			IP:        ip,
			Labels:    map[string]string{},
		})
	}

	return points, nil
}

