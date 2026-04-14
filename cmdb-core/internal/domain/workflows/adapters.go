package workflows

import (
	"context"
	"encoding/json"
	"time"
)

// MetricPoint is the unified internal format for all metrics adapters.
type MetricPoint struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	IP        string            `json:"ip"`
	Labels    map[string]string `json:"labels"`
}

// MetricsAdapter defines the interface all monitoring system adapters must implement.
type MetricsAdapter interface {
	// Fetch retrieves metrics from the monitoring system.
	// endpoint: the base URL of the monitoring system
	// config: adapter-specific configuration from integration_adapters.config JSONB
	Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error)
}

// adapterRegistry maps adapter type names to implementations.
var adapterRegistry = map[string]MetricsAdapter{
	"prometheus":  &PrometheusAdapter{},
	"zabbix":      &ZabbixAdapter{},
	"custom_rest": &CustomRESTAdapter{},
	"snmp":        &SNMPAdapter{},
	"datadog":     &DatadogAdapter{},
	"nagios":      &NagiosAdapter{},
}

// GetAdapter returns the adapter for a given type, or nil if not supported.
func GetAdapter(adapterType string) MetricsAdapter {
	return adapterRegistry[adapterType]
}

// SupportedAdapterTypes returns all registered adapter type names.
func SupportedAdapterTypes() []string {
	types := make([]string, 0, len(adapterRegistry))
	for k := range adapterRegistry {
		types = append(types, k)
	}
	return types
}
