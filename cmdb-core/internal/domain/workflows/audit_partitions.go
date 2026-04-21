package workflows

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"go.uber.org/zap"
)

// sourceAuditPartitionSample is the ErrorsSuppressedTotal label for a
// failed partition-count sample. Using a dedicated label keeps the
// partition-sampler failures distinct from workflow-level DB errors.
const sourceAuditPartitionSample = "workflows.audit.partition_sample"

// StartAuditPartitionSampler polls pg_inherits every 5 minutes and
// publishes the current child count of audit_events through
// telemetry.AuditPartitionCount. The metric is what the alertmanager
// rule `cmdb_audit_partition_count < 3 for 1h` watches to catch a
// missed CronJob (= no next-month partition was created, so writes
// will start failing the moment the month rolls over).
//
// The sampler is deliberately read-only and uses a short Exec-style
// QueryRow, so it is safe to run in every server replica. Running
// multiple samplers concurrently does not distort the metric because
// the gauge is set to an absolute value, not incremented.
func (w *WorkflowSubscriber) StartAuditPartitionSampler(ctx context.Context) {
	const interval = 5 * time.Minute
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		// Sample once immediately so freshly-deployed servers report
		// the current value without waiting 5 minutes for the first
		// scrape to have data.
		w.sampleAuditPartitionCount(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.audit_partition_sample")
				w.sampleAuditPartitionCount(tickCtx)
				end()
			}
		}
	}()
	zap.L().Info("audit partition sampler started", zap.Duration("interval", interval))
}

func (w *WorkflowSubscriber) sampleAuditPartitionCount(ctx context.Context) {
	const q = `
SELECT count(*)::int
FROM pg_inherits i
JOIN pg_class p ON p.oid = i.inhparent
WHERE p.relname = 'audit_events'`

	var n int
	if err := w.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		// A transient DB error must not kill the ticker. Surface via
		// the shared errors-suppressed counter so dashboards notice.
		zap.L().Warn("audit partition sample failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceAuditPartitionSample, telemetry.ReasonDBExecFailed).Inc()
		return
	}
	telemetry.AuditPartitionCount.Set(float64(n))
}
