package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Webhook retention windows (in days). Exported as constants so tests
// don't drift from production values.
const (
	WebhookDeliveriesRetentionDays int32 = 30
	WebhookDLQRetentionDays        int32 = 90
)

// Source labels for telemetry.ErrorsSuppressedTotal. Kept as consts so
// the label-value spelling cannot drift between call sites.
const (
	sourceCleanupSessions    = "workflows.cleanup.sessions"
	sourceCleanupConflicts   = "workflows.cleanup.conflicts"
	sourceCleanupDiscoveries = "workflows.cleanup.discoveries"
)

func (w *WorkflowSubscriber) StartSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.cleanupSessions(ctx)
			}
		}
	}()
	zap.L().Info("Session cleanup started (1h interval)")
}

func (w *WorkflowSubscriber) cleanupSessions(ctx context.Context) {
	// Each UPDATE/DELETE below is best-effort: a transient DB error
	// must not kill the hourly ticker. We therefore propagate a Warn
	// + suppressed-error counter increment and zero-out the
	// unavailable RowsAffected() so the info summary at the bottom
	// still reports the successful stages.
	var expired, deleted, trimmed int64

	// 1. Mark sessions inactive for 7+ days as expired
	if n, err := w.queries.ExpireIdleUserSessions(ctx); err != nil {
		zap.L().Warn("session cleanup: expire stage failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupSessions, telemetry.ReasonDBExecFailed).Inc()
	} else {
		expired = n
	}

	// 2. Delete sessions older than 30 days
	if n, err := w.queries.DeleteOldUserSessions(ctx); err != nil {
		zap.L().Warn("session cleanup: delete-old stage failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupSessions, telemetry.ReasonDBExecFailed).Inc()
	} else {
		deleted = n
	}

	// 3. Keep only latest 20 sessions per user (delete excess)
	if n, err := w.queries.TrimUserSessionsPerUser(ctx); err != nil {
		zap.L().Warn("session cleanup: trim-per-user stage failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupSessions, telemetry.ReasonDBExecFailed).Inc()
	} else {
		trimmed = n
	}

	if expired+deleted+trimmed > 0 {
		zap.L().Info("session cleanup completed",
			zap.Int64("expired", expired),
			zap.Int64("deleted", deleted),
			zap.Int64("trimmed", trimmed))
	}
}

// StartConflictAndDiscoveryCleanup runs a background ticker for conflict SLA and discovery TTL.
func (w *WorkflowSubscriber) StartConflictAndDiscoveryCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.autoResolveStaleConflicts(ctx)
				w.expireStaleDiscoveries(ctx)
			}
		}
	}()
	zap.L().Info("Conflict SLA + Discovery TTL checker started (1h interval)")
}

// autoResolveStaleConflicts resolves import conflicts older than 7 days
// by accepting the higher-priority source value. Every DB failure below
// is logged at Warn and counted via telemetry.ErrorsSuppressedTotal so
// the hourly loop keeps making forward progress even when one stage
// transiently fails.
func (w *WorkflowSubscriber) autoResolveStaleConflicts(ctx context.Context) {
	q := dbgen.New(w.pool)

	// Notify ops-admins about conflicts approaching 3-day SLA warning.
	// Cross-tenant sweep by design — the caller then fans out per-tenant
	// notifications through opsAdminUserIDs + createNotification.
	slaRows, err := q.CountPendingConflictsByTenantNearSLA(ctx)
	if err != nil {
		zap.L().Warn("autoResolveStaleConflicts: SLA-warning query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupConflicts, telemetry.ReasonDBQueryFailed).Inc()
	} else {
		for _, row := range slaRows {
			for _, uid := range w.opsAdminUserIDs(ctx, row.TenantID) {
				w.createNotification(ctx, row.TenantID, uid,
					"conflict_sla_warning",
					fmt.Sprintf("%d sync conflicts approaching SLA deadline", row.Count),
					"These conflicts will be auto-resolved in 4 days if not manually addressed.",
					"sync_conflict", uuid.Nil)
			}
		}
	}

	var expired1, expired2 int64

	// Auto-resolve sync_conflicts older than 7 days
	if rowsAffected, err := q.AutoExpireStaleSyncConflicts(ctx); err != nil {
		zap.L().Warn("autoResolveStaleConflicts: sync_conflicts expire failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupConflicts, telemetry.ReasonDBExecFailed).Inc()
	} else {
		expired1 = rowsAffected
	}

	// Also handle import_conflicts if the table exists (created by ingestion-engine)
	if res, err := w.pool.Exec(ctx,
		`UPDATE import_conflicts SET status = 'auto_resolved', resolved_at = now()
		 WHERE status = 'pending' AND created_at < now() - interval '7 days'`,
	); err != nil {
		zap.L().Warn("autoResolveStaleConflicts: import_conflicts expire failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupConflicts, telemetry.ReasonDBExecFailed).Inc()
	} else {
		expired2 = res.RowsAffected()
	}

	if expired1+expired2 > 0 {
		zap.L().Info("auto-resolved stale conflicts",
			zap.Int64("sync_conflicts", expired1),
			zap.Int64("import_conflicts", expired2))
	}
}

// expireStaleDiscoveries marks discovered assets pending for >14 days
// as expired. A transient DB failure emits a Warn + counter and the
// loop tries again on the next tick.
func (w *WorkflowSubscriber) expireStaleDiscoveries(ctx context.Context) {
	res, err := w.pool.Exec(ctx,
		`UPDATE discovered_assets SET status = 'expired'
		 WHERE status = 'pending' AND discovered_at < now() - interval '14 days'`)
	if err != nil {
		zap.L().Warn("expireStaleDiscoveries: exec failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceCleanupDiscoveries, telemetry.ReasonDBExecFailed).Inc()
		return
	}

	expired := res.RowsAffected()
	if expired > 0 {
		zap.L().Info("expired stale discoveries", zap.Int64("count", expired))
	}
}

// StartWebhookRetention runs the daily webhook retention sweep. Separate
// goroutine from the hourly cleanups because the sweep is more expensive
// (two full-table DELETEs) and doesn't need hourly granularity.
//
// We align to ~03:00 UTC on first tick so the sweep runs during the
// quietest part of the global traffic window. Subsequent ticks fire every
// 24 hours.
func (w *WorkflowSubscriber) StartWebhookRetention(ctx context.Context) {
	go func() {
		// First-tick alignment: wake up at the next 03:00 UTC, then
		// tick every 24h. If the service starts at 04:00 UTC we'll
		// wait 23 hours before the first sweep — that's fine, the
		// retention windows are measured in weeks.
		initialDelay := durationUntilNextUTCHour(w.nowUTC(), 3)
		timer := time.NewTimer(initialDelay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		w.runWebhookRetention(ctx)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runWebhookRetention(ctx)
			}
		}
	}()
	zap.L().Info("webhook retention sweep started (daily at 03:00 UTC)",
		zap.Int32("deliveries_retention_days", WebhookDeliveriesRetentionDays),
		zap.Int32("dlq_retention_days", WebhookDLQRetentionDays))
}

// runWebhookRetention executes the two DELETEs and updates the retention
// counter. Errors are logged but never returned — a transient DB blip must
// not stop the next day's sweep.
func (w *WorkflowSubscriber) runWebhookRetention(ctx context.Context) {
	deliveries, err := w.queries.DeleteOldWebhookDeliveries(ctx, WebhookDeliveriesRetentionDays)
	if err != nil {
		zap.L().Error("webhook retention: deliveries sweep failed", zap.Error(err))
	} else if deliveries > 0 {
		telemetry.WebhookRetentionDeletesTotal.WithLabelValues("webhook_deliveries").Add(float64(deliveries))
	}

	dlq, err := w.queries.DeleteOldWebhookDLQ(ctx, WebhookDLQRetentionDays)
	if err != nil {
		zap.L().Error("webhook retention: DLQ sweep failed", zap.Error(err))
	} else if dlq > 0 {
		telemetry.WebhookRetentionDeletesTotal.WithLabelValues("webhook_deliveries_dlq").Add(float64(dlq))
	}

	if deliveries+dlq > 0 {
		zap.L().Info("webhook retention sweep completed",
			zap.Int64("deliveries_deleted", deliveries),
			zap.Int64("dlq_deleted", dlq))
	}
}

// nowUTC is pulled out for test-overridability. Production always returns
// the real wall clock.
func (w *WorkflowSubscriber) nowUTC() time.Time {
	return time.Now().UTC()
}

// durationUntilNextUTCHour returns how long until the next occurrence of
// hourUTC:00:00. If we're already past today's hour, it returns the wait
// until tomorrow's.
func durationUntilNextUTCHour(now time.Time, hourUTC int) time.Duration {
	target := time.Date(now.Year(), now.Month(), now.Day(), hourUTC, 0, 0, 0, time.UTC)
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target.Sub(now)
}
