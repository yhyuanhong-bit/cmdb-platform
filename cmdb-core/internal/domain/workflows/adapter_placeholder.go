package workflows

import (
	"context"
	"encoding/json"
	"fmt"
)

// SNMPAdapter placeholder — SNMP metrics are currently handled by ingestion-engine.
type SNMPAdapter struct{}

func (a *SNMPAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	return nil, fmt.Errorf("SNMP adapter: not yet implemented in Go — use ingestion-engine SNMP collector instead")
}

// DatadogAdapter placeholder for future Datadog integration.
type DatadogAdapter struct{}

func (a *DatadogAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	return nil, fmt.Errorf("Datadog adapter: not yet implemented — configure API key and site in adapter config")
}

// NagiosAdapter placeholder for Nagios/Icinga integration.
type NagiosAdapter struct{}

func (a *NagiosAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	return nil, fmt.Errorf("Nagios adapter: not yet implemented — use Custom REST adapter with Livestatus API as workaround")
}
