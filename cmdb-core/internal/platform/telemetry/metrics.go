package telemetry

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	}, []string{"method", "path"})

	// ActiveWSConnections tracks the number of active WebSocket connections.
	ActiveWSConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ws_active_connections",
		Help: "Number of active WebSocket connections.",
	})

	// NATSMessagesPublished counts NATS messages published by subject.
	NATSMessagesPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nats_messages_published_total",
		Help: "Total NATS messages published.",
	}, []string{"subject"})

	// DBQueryDuration tracks database query durations.
	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "db_query_duration_seconds",
		Help: "Duration of database queries in seconds.",
	}, []string{"query"})

	// Sync metrics
	SyncEnvelopeApplied = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_applied_total",
		Help: "Successfully applied sync envelopes.",
	}, []string{"entity_type"})

	SyncEnvelopeSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_skipped_total",
		Help: "Skipped sync envelopes (version gate or duplicate).",
	}, []string{"entity_type"})

	SyncEnvelopeFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_failed_total",
		Help: "Failed sync envelope applications.",
	}, []string{"entity_type"})

	// SyncEnvelopeRejected counts envelopes rejected BEFORE apply for
	// integrity / authorization reasons. Distinct from SyncEnvelopeFailed,
	// which covers apply-time DB errors on envelopes we believed were
	// legitimate. Reasons currently emitted:
	//
	//   tenant_mismatch — env.TenantID did not match the tenant segment
	//                     of the NATS subject it arrived on (cross-tenant
	//                     replay attempt or publisher bug).
	//   bad_checksum    — SHA-256 fingerprint mismatch (payload tampered
	//                     or corrupted in transit).
	//
	// Phase 4.3 HMAC signing will add sig_missing / sig_bad_alg /
	// sig_unknown_kid / bad_signature reasons on top of this counter.
	SyncEnvelopeRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_sync_envelope_rejected_total",
		Help: "Sync envelopes rejected before apply (tenant_mismatch|bad_checksum|...).",
	}, []string{"entity_type", "reason"})

	SyncReconciliationRuns = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cmdb_sync_reconciliation_runs_total",
		Help: "Total reconciliation job executions.",
	})

	// IntegrationDecryptFallbackTotal counts times the dual-read path for
	// integration secrets fell back from ciphertext to plaintext (or failed
	// to decrypt). Observational only — does not change read semantics.
	//
	// table:  integration_adapters | webhook_subscriptions
	// reason: ciphertext_null | decrypt_failed
	IntegrationDecryptFallbackTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "integration_decrypt_fallback_total",
		Help: "Times the integration-secrets read path fell back to the plaintext column or hit a decrypt failure.",
	}, []string{"table", "reason"})

	// IntegrationDualWriteDivergenceTotal counts rows where the plaintext
	// column and decrypted ciphertext column disagree. Populated by the
	// periodic divergence sampling job; any non-zero value is an operator
	// alert signal.
	//
	// table: integration_adapters | webhook_subscriptions
	IntegrationDualWriteDivergenceTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "integration_dual_write_divergence_total",
		Help: "Rows detected where the encrypted and plaintext integration-secret columns disagree.",
	}, []string{"table"})

	// MonitoringEvaluatorRunsTotal counts full evaluator tick outcomes. One
	// tick = one scan of ListEnabledAlertRules and evaluation of every rule.
	// outcome: ok | error
	MonitoringEvaluatorRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "monitoring_evaluator_runs_total",
		Help: "Total alert-evaluator tick executions by outcome (ok|error).",
	}, []string{"outcome"})

	// MonitoringRuleEvaluationDuration measures how long a single rule takes
	// to evaluate (condition parse + metric aggregation + comparison +
	// optional emit). Label cardinality: operator (~6) × aggregation (~5) =
	// at most 30 series. We deliberately do NOT label by rule_id — a noisy
	// tenant with thousands of rules would blow up the timeseries budget.
	// See pattern in telemetry.DBQueryDuration / httpRequestDuration above.
	MonitoringRuleEvaluationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "monitoring_rule_evaluation_duration_seconds",
		Help:    "Duration of a single alert rule evaluation, bucketed by operator and aggregation.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
	}, []string{"operator", "aggregation"})

	// MonitoringAlertsEmittedTotal counts emissions from the evaluator. We
	// label by severity + status + action rather than rule_id for the same
	// cardinality reason above. action: inserted | updated (dedup upsert)
	// so operators can distinguish new firings from repeated same-hour hits.
	MonitoringAlertsEmittedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "monitoring_alerts_emitted_total",
		Help: "Total alert_event rows emitted by the evaluator, by status (firing|resolved), severity, and action (inserted|updated).",
	}, []string{"status", "severity", "action"})

	// AdapterPullAttemptsTotal counts metric-puller attempts per tenant and
	// outcome. The puller's `ListDuePullAdapters` query is intentionally
	// cross-tenant (it IS the scheduler), but every emitted observation is
	// still labelled with `tenant_id` so operators can see at a glance
	// whether one tenant's adapters dominate the batch or are failing.
	//
	// outcome: success | failure
	AdapterPullAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "adapter_pull_attempts_total",
		Help: "Total adapter pull attempts by tenant and outcome (success|failure).",
	}, []string{"tenant_id", "outcome"})

	// WebhookRetentionDeletesTotal counts rows pruned by the daily retention
	// sweep. Exposed so operators can confirm the cron is actually running —
	// a counter that flatlines for >24h means the retention goroutine has
	// died and the tables are growing unbounded.
	//
	// table: webhook_deliveries | webhook_deliveries_dlq
	WebhookRetentionDeletesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_retention_deletes_total",
		Help: "Total rows deleted by the webhook retention sweep, by table.",
	}, []string{"table"})

	// WebhookCircuitBreakerTripsTotal counts subscription trips — i.e.
	// transitions from enabled-but-failing to disabled_at IS NOT NULL.
	// Separate from ordinary failures because each trip requires an
	// ops-admin to manually re-enable.
	WebhookCircuitBreakerTripsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "webhook_circuit_breaker_trips_total",
		Help: "Total times a webhook subscription was auto-disabled after consecutive failures.",
	})

	// WebhookDLQRowsTotal counts DLQ inserts. One row per tripped delivery
	// attempt whose payload was parked for operator replay.
	WebhookDLQRowsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "webhook_dlq_rows_total",
		Help: "Total webhook payloads parked in the DLQ after a circuit breaker trip.",
	})

	// QualityScannerRunsTotal counts full-tenant quality scan executions
	// by outcome. One increment per tenant scanned by the scheduled
	// quality.Service.ScanTenant call (Phase 2.11). The daily loop
	// iterates ListActiveTenants and emits one observation per tenant so
	// operators can see at a glance whether a single tenant is
	// consistently failing the scan while the rest succeed.
	//
	// outcome: ok | error
	QualityScannerRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quality_scanner_runs_total",
		Help: "Total scheduled per-tenant quality scan executions by outcome (ok|error).",
	}, []string{"outcome"})
)

// Label values for the integration_* metrics. Exported so callers don't
// drift on the spelling.
const (
	IntegrationTableAdapters = "integration_adapters"
	IntegrationTableWebhooks = "webhook_subscriptions"

	IntegrationFallbackReasonCiphertextNull = "ciphertext_null"
	IntegrationFallbackReasonDecryptFailed  = "decrypt_failed"
)

// PrometheusMiddleware returns a Gin middleware that records HTTP request
// duration and total count, labelled by method, path, and status.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

// MetricsHandler returns a Gin handler that serves Prometheus metrics.
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
