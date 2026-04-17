package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func (w *WorkflowSubscriber) StartSLAChecker(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.checkSLABreaches(ctx)
				w.checkSLAWarnings(ctx)
			}
		}
	}()
	zap.L().Info("SLA checker started (60s interval)")
}

func (w *WorkflowSubscriber) checkSLABreaches(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		"SELECT id, tenant_id, code, assignee_id FROM work_orders WHERE status IN ('approved','in_progress') AND sla_deadline IS NOT NULL AND sla_deadline < now() AND sla_breached = false AND deleted_at IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tenantID uuid.UUID
		var code string
		var assigneeID pgtype.UUID
		if rows.Scan(&id, &tenantID, &code, &assigneeID) != nil {
			continue
		}
		w.pool.Exec(ctx, "UPDATE work_orders SET sla_breached = true WHERE id = $1 AND tenant_id = $2", id, tenantID)
		if assigneeID.Valid {
			w.createNotification(ctx, tenantID, uuid.UUID(assigneeID.Bytes),
				"sla_breach",
				fmt.Sprintf("SLA Breached: %s", code),
				fmt.Sprintf("Work order %s has exceeded its SLA deadline.", code),
				"work_order", id)
		}
		zap.L().Warn("SLA breached", zap.String("order", code))
	}
}

func (w *WorkflowSubscriber) checkSLAWarnings(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		"SELECT id, tenant_id, code, assignee_id FROM work_orders WHERE status IN ('approved','in_progress') AND sla_deadline IS NOT NULL AND sla_warning_sent = false AND sla_deadline - (sla_deadline - approved_at) * 0.25 < now() AND approved_at IS NOT NULL AND deleted_at IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tenantID uuid.UUID
		var code string
		var assigneeID pgtype.UUID
		if rows.Scan(&id, &tenantID, &code, &assigneeID) != nil {
			continue
		}
		w.pool.Exec(ctx, "UPDATE work_orders SET sla_warning_sent = true WHERE id = $1 AND tenant_id = $2", id, tenantID)
		if assigneeID.Valid {
			w.createNotification(ctx, tenantID, uuid.UUID(assigneeID.Bytes),
				"sla_warning",
				fmt.Sprintf("SLA Warning: %s", code),
				fmt.Sprintf("Work order %s is approaching its SLA deadline.", code),
				"work_order", id)
		}
	}
}
