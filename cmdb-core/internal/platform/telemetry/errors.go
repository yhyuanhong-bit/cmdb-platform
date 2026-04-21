package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ErrorsSuppressedTotal counts the times a non-nil error was observed
// in a background / cron / cleanup code path and intentionally dropped
// so the outer loop could continue. Every increment corresponds to a
// Warn-level log line — the counter exists so operators can trigger
// an alert on a sustained uptick without having to log-scrape for
// textual patterns.
//
// Two-label cardinality budget:
//
//	source — the calling module's short name ("workflows.cleanup",
//	         "workflows.sla", "workflows.notifications", etc.).
//	         Bounded by the number of call sites (currently ~20).
//	reason — a short tag for why the error was suppressed
//	         ("db_exec_failed", "db_query_failed", "row_scan_failed",
//	         "json_unmarshal_failed", "wo_creation_failed", etc.).
//	         Also bounded, ~10 values.
//
// Cross product ≈ 200 series in the worst case, well inside the
// per-metric budget we hold elsewhere. Do NOT add a tenant_id label —
// these are operational signals, not per-tenant billing.
var ErrorsSuppressedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "errors_suppressed_total",
	Help: "Total non-nil errors suppressed by a background/cron path, labelled by source module and reason tag. Every increment should correspond to a zap.Warn line.",
}, []string{"source", "reason"})

// Well-known reason tags. Exported so callers share one spelling —
// drift here defeats operator dashboards that match on exact strings.
const (
	ReasonDBExecFailed         = "db_exec_failed"
	ReasonDBQueryFailed        = "db_query_failed"
	ReasonDBTxFailed           = "db_tx_failed"
	ReasonRowScanFailed        = "row_scan_failed"
	ReasonRowsIterFailed       = "rows_iter_failed"
	ReasonJSONUnmarshal        = "json_unmarshal_failed"
	ReasonNotificationFailed   = "notification_insert_failed"
	ReasonWOCreationFailed     = "wo_creation_failed"
	ReasonAdapterConfig        = "adapter_config_invalid"
	ReasonSystemUserUnresolved = "system_user_unresolved"
)
