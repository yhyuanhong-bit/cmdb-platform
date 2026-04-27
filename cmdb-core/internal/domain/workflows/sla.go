package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

//tenantlint:allow-direct-pool — cross-tenant SLA breach scheduler

// Source labels for telemetry.ErrorsSuppressedTotal.
const (
	sourceSLABreach  = "workflows.sla.checkSLABreaches"
	sourceSLAWarning = "workflows.sla.checkSLAWarnings"
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
				tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.sla")
				w.checkSLABreaches(tickCtx)
				w.checkSLAWarnings(tickCtx)
				end()
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
			w.warnNotify(ctx, sourceSLABreach, r.tenantID, uuid.UUID(r.assigneeID.Bytes),
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

// checkSLAWarnings atomically flips sla_warning_sent=true for every
// work order whose remaining time has dropped below 25% of the original
// approved-to-deadline window, returning the rows that actually
// transitioned. Same TOCTOU-safe pattern as checkSLABreaches above —
// the row-level lock Postgres takes during the UPDATE plus the
// `NOT sla_warning_sent` guard ensure exactly one tick ever sees a
// given row in RETURNING. Replaces the previous SELECT-then-per-row-
// UPDATE flow which let two scheduler instances (or a restart mid-loop)
// double-publish the same warning notification.
func (w *WorkflowSubscriber) checkSLAWarnings(ctx context.Context) {
	rows, err := w.pool.Query(ctx, `
		UPDATE work_orders
		SET sla_warning_sent = true, updated_at = now()
		WHERE status IN ('approved','in_progress')
		  AND sla_deadline IS NOT NULL
		  AND approved_at IS NOT NULL
		  AND NOT sla_warning_sent
		  AND sla_deadline - (sla_deadline - approved_at) * 0.25 < now()
		  AND deleted_at IS NULL
		RETURNING id, tenant_id, code, assignee_id
	`)
	if err != nil {
		zap.L().Warn("checkSLAWarnings: update failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSLAWarning, telemetry.ReasonDBExecFailed).Inc()
		return
	}

	// Drain the RETURNING cursor into a slice before fanning out
	// notifications so the connection isn't held during the side-effects.
	warned := make([]breachedRow, 0, 16)
	for rows.Next() {
		var r breachedRow
		if scanErr := rows.Scan(&r.id, &r.tenantID, &r.code, &r.assigneeID); scanErr != nil {
			zap.L().Warn("checkSLAWarnings: scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSLAWarning, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		warned = append(warned, r)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		zap.L().Warn("checkSLAWarnings: rows iter failed", zap.Error(rowsErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSLAWarning, telemetry.ReasonRowsIterFailed).Inc()
		// still fan out whatever we successfully scanned — the UPDATE
		// committed, so DB state is consistent.
	}

	for _, r := range warned {
		if r.assigneeID.Valid {
			w.warnNotify(ctx, sourceSLAWarning, r.tenantID, uuid.UUID(r.assigneeID.Bytes),
				"sla_warning",
				fmt.Sprintf("SLA Warning: %s", r.code),
				fmt.Sprintf("Work order %s is approaching its SLA deadline.", r.code),
				"work_order", r.id)
		}
	}
}
