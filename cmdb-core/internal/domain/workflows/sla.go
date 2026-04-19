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

// breachedRow is the minimal tuple RETURNING surface of the atomic
// SLA-breach update. We collect these into a slice first so that the
// notification side-effects (DB insert + event bus publish) run AFTER
// the UPDATE cursor is drained — this keeps the pgx row scanner from
// holding a connection while notifications fan out.
type breachedRow struct {
	id         uuid.UUID
	tenantID   uuid.UUID
	code       string
	assigneeID pgtype.UUID
}

// checkSLABreaches atomically flips sla_breached=true for every
// eligible work order and returns the rows that actually transitioned,
// in a single round-trip. The previous implementation did a SELECT
// followed by per-row UPDATE, which leaves a TOCTOU window where two
// scheduler instances (or a restart mid-loop) could double-publish the
// same breach notification. By pushing the guard (`NOT sla_breached`)
// into the UPDATE's WHERE clause, the row-level lock Postgres takes
// during the UPDATE ensures exactly one tick ever sees a given row in
// RETURNING — the second concurrent call finds the already-flipped
// flag and skips.
//
// Cross-tenant scheduler: this runs outside any tenant context, so the
// query intentionally has no tenant filter. RETURNING carries
// tenant_id per row and every downstream notification is scoped to
// that exact tenant — never an ambient value.
//
// Notification side-effects (createNotification) are cheap: a single
// INSERT + an in-process eventbus publish. We still drain the rows
// cursor first, then fan out — this keeps the connection short-held
// and leaves room to swap in a slower notifier later without having
// to restructure.
func (w *WorkflowSubscriber) checkSLABreaches(ctx context.Context) {
	rows, err := w.pool.Query(ctx, `
		UPDATE work_orders
		SET sla_breached = true, updated_at = now()
		WHERE tenant_id IS NOT NULL
		  AND status IN ('approved','in_progress')
		  AND sla_deadline IS NOT NULL
		  AND sla_deadline < now()
		  AND NOT sla_breached
		  AND deleted_at IS NULL
		RETURNING id, tenant_id, code, assignee_id
	`)
	if err != nil {
		zap.L().Error("sla breach update failed", zap.Error(err))
		return
	}

	breached := make([]breachedRow, 0, 16)
	for rows.Next() {
		var r breachedRow
		if err := rows.Scan(&r.id, &r.tenantID, &r.code, &r.assigneeID); err != nil {
			zap.L().Warn("sla breach scan failed", zap.Error(err))
			continue
		}
		breached = append(breached, r)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		zap.L().Error("sla breach rows error", zap.Error(rowsErr))
		// still fan out whatever we successfully scanned — the UPDATE
		// itself committed, so the DB state is consistent
	}

	for _, r := range breached {
		if r.assigneeID.Valid {
			w.createNotification(ctx, r.tenantID, uuid.UUID(r.assigneeID.Bytes),
				"sla_breach",
				fmt.Sprintf("SLA Breached: %s", r.code),
				fmt.Sprintf("Work order %s has exceeded its SLA deadline.", r.code),
				"work_order", r.id)
		}
		zap.L().Warn("SLA breached",
			zap.String("order", r.code),
			zap.String("tenant_id", r.tenantID.String()))
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
