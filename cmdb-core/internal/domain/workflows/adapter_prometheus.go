package workflows

import (
	"context"
	"encoding/json"
	"fmt"
)

// PrometheusAdapter implements MetricsAdapter for Prometheus.
type PrometheusAdapter struct{}

type prometheusConfig struct {
	Queries             []string `json:"queries"`
	PullIntervalSeconds int      `json:"pull_interval_seconds"`
}

func (a *PrometheusAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	var cfg prometheusConfig
	// A malformed config JSON must short-circuit with a real error so
	// the adapter-failure counter records the cause — the pre-fix code
	// silently fell through to an empty-queries path and Fetch returned
	// (nil, nil), making a broken config look identical to a healthy
	// idle adapter.
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("prometheus: parse config: %w", err)
		}
	}

	if len(cfg.Queries) == 0 {
		return nil, nil
	}

	var allPoints []MetricPoint
	for _, query := range cfg.Queries {
		results, err := fetchPromMetrics(ctx, endpoint, query)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			allPoints = append(allPoints, MetricPoint{
				Name:      r.MetricName,
				Value:     r.Value,
				Timestamp: r.Timestamp,
				IP:        r.IP,
				Labels:    r.Labels,
			})
		}
	}
	return allPoints, nil
}
