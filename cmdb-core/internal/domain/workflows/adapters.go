package workflows

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
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

// Package-level SSRF guard shared by outbound adapters. Initialised from
// main.go via SetNetGuard at startup. If the application forgets to
// configure one, GetNetGuard returns a Guard backed by DefaultBlockedCIDRs —
// this keeps us secure-by-default even when wiring is missed.
var (
	netGuardMu      sync.RWMutex
	netGuardCurrent *netguard.Guard
)

// SetNetGuard installs the process-wide Guard used by outbound adapters.
// Call this once at startup; subsequent calls replace the previous guard
// (useful for tests).
func SetNetGuard(g *netguard.Guard) {
	netGuardMu.Lock()
	defer netGuardMu.Unlock()
	netGuardCurrent = g
}

// GetNetGuard returns the active Guard. Falls back to a defaults-only Guard
// if none was configured, so adapters always have SSRF protection.
func GetNetGuard() *netguard.Guard {
	netGuardMu.RLock()
	g := netGuardCurrent
	netGuardMu.RUnlock()
	if g != nil {
		return g
	}
	// Lazily initialise a default guard — the CIDRs are static, so this
	// cannot fail. If it somehow does, fall back to Permissive rather than
	// panicking the request path; a startup log warning in main.go makes
	// the misconfiguration visible.
	fallback, err := netguard.New(nil, nil)
	if err != nil {
		return netguard.Permissive()
	}
	netGuardMu.Lock()
	if netGuardCurrent == nil {
		netGuardCurrent = fallback
	}
	g = netGuardCurrent
	netGuardMu.Unlock()
	return g
}
