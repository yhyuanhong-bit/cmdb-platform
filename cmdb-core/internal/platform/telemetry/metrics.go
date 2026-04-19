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
