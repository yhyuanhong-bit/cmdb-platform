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

type promResult struct {
	MetricName string
	IP         string
	Value      float64
	Timestamp  time.Time
	Labels     map[string]string
}

type promQueryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string  `json:"metric"`
			Value  [2]json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

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
		var ts float64
		if err := json.Unmarshal(r.Value[0], &ts); err != nil {
			continue
		}
		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		results = append(results, promResult{
			MetricName: r.Metric["__name__"],
			IP:         extractIP(r.Metric["instance"]),
			Value:      val,
			Timestamp:  time.Unix(int64(ts), 0),
			Labels:     r.Metric,
		})
	}
	return results, nil
}

func extractIP(instance string) string {
	if instance == "" {
		return ""
	}
	if idx := strings.LastIndex(instance, ":"); idx > 0 {
		return instance[:idx]
	}
	return instance
}
