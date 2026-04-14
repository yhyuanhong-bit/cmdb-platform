package workflows

import (
	"context"
	"encoding/json"
)

// PrometheusAdapter implements MetricsAdapter for Prometheus.
type PrometheusAdapter struct{}

type prometheusConfig struct {
	Queries             []string `json:"queries"`
	PullIntervalSeconds int      `json:"pull_interval_seconds"`
}

func (a *PrometheusAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	var cfg prometheusConfig
	json.Unmarshal(config, &cfg) //nolint:errcheck

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
