package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
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
	// 1. Mark sessions inactive for 7+ days as expired
	res1, _ := w.pool.Exec(ctx,
		"UPDATE user_sessions SET expired_at = now() WHERE expired_at IS NULL AND last_active_at < now() - interval '7 days'")

	// 2. Delete sessions older than 30 days
	res2, _ := w.pool.Exec(ctx,
		"DELETE FROM user_sessions WHERE created_at < now() - interval '30 days'")

	// 3. Keep only latest 20 sessions per user (delete excess)
	res3, _ := w.pool.Exec(ctx,
		`DELETE FROM user_sessions WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) AS rn
				FROM user_sessions
			) ranked WHERE rn > 20
		)`)

	expired := res1.RowsAffected()
	deleted := res2.RowsAffected()
	trimmed := res3.RowsAffected()

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
// by accepting the higher-priority source value.
func (w *WorkflowSubscriber) autoResolveStaleConflicts(ctx context.Context) {
	// Notify ops-admins about conflicts approaching 3-day SLA warning
	rows, _ := w.pool.Query(ctx,
		`SELECT tenant_id, count(*) FROM sync_conflicts
		 WHERE resolution = 'pending' AND created_at < now() - interval '3 days' AND created_at >= now() - interval '4 days'
		 GROUP BY tenant_id`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tid uuid.UUID
			var cnt int
			if rows.Scan(&tid, &cnt) == nil {
				for _, uid := range w.opsAdminUserIDs(ctx, tid) {
					w.createNotification(ctx, tid, uid,
						"conflict_sla_warning",
						fmt.Sprintf("%d sync conflicts approaching SLA deadline", cnt),
						"These conflicts will be auto-resolved in 4 days if not manually addressed.",
						"sync_conflict", uuid.Nil)
				}
			}
		}
	}

	// Auto-resolve sync_conflicts older than 7 days
	res1, _ := w.pool.Exec(ctx,
		`UPDATE sync_conflicts SET resolution = 'auto_expired', resolved_at = now()
		 WHERE resolution = 'pending' AND created_at < now() - interval '7 days'`)

	// Also handle import_conflicts if the table exists (created by ingestion-engine)
	res2, _ := w.pool.Exec(ctx,
		`UPDATE import_conflicts SET status = 'auto_resolved', resolved_at = now()
		 WHERE status = 'pending' AND created_at < now() - interval '7 days'`)

	expired1 := res1.RowsAffected()
	expired2 := res2.RowsAffected()
	if expired1+expired2 > 0 {
		zap.L().Info("auto-resolved stale conflicts",
			zap.Int64("sync_conflicts", expired1),
			zap.Int64("import_conflicts", expired2))
	}
}

// expireStaleDiscoveries marks discovered assets pending for >14 days as expired.
func (w *WorkflowSubscriber) expireStaleDiscoveries(ctx context.Context) {
	res, _ := w.pool.Exec(ctx,
		`UPDATE discovered_assets SET status = 'expired'
		 WHERE status = 'pending' AND discovered_at < now() - interval '14 days'`)

	expired := res.RowsAffected()
	if expired > 0 {
		zap.L().Info("expired stale discoveries", zap.Int64("count", expired))
	}
}
