package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// Adapter failure policy. Centralized so callers and tests share the same
// numbers and we do not accidentally drift them across files.
const (
	// adapterDisableThreshold is the number of consecutive failures that
	// triggers auto-disable. Matches the prior in-memory behavior but is
	// now evaluated against the persisted counter, so service restarts
	// cannot silently reset progress toward disable.
	adapterDisableThreshold = 3

	// adapterFailureReasonMaxLen caps how much of an error message we
	// persist in last_failure_reason. Adapter errors often include full
	// HTTP bodies; truncating protects the column and audit log size.
	adapterFailureReasonMaxLen = 500
)

// Source label for telemetry.ErrorsSuppressedTotal on the metrics
// INSERT path. A broken `metrics` hypertable used to be invisible
// because the pool.Exec error was discarded; route it through the
// shared errors-suppressed counter so dashboards surface it.
const sourceMetricsInsert = "workflows.metrics.insert"

// computeAdapterBackoff returns the delay to wait before the next pull
// attempt after n consecutive failures. Schedule: 30s, 2m, 10m, 30m cap.
//
// Exposed (lower-cased, package-private but called from tests in the same
// package) so the escalation schedule can be asserted without exercising
// the full DB + workflow stack.
func computeAdapterBackoff(n int32) time.Duration {
	switch {
	case n <= 1:
		return 30 * time.Second
	case n == 2:
		return 2 * time.Minute
	case n == 3:
		return 10 * time.Minute
	default:
		return 30 * time.Minute
	}
}

// truncateReason shortens err text to adapterFailureReasonMaxLen bytes.
// Kept as a pure function so tests can verify the boundary behavior.
func truncateReason(s string) string {
	if len(s) <= adapterFailureReasonMaxLen {
		return s
	}
	return s[:adapterFailureReasonMaxLen]
}

func (w *WorkflowSubscriber) StartMetricsPuller(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.pullMetricsFromAdapters(ctx)
			}
		}
	}()
	zap.L().Info("Metrics puller started (5m interval)")
}

// pullMetricsFromAdapters iterates due-to-pull adapters. Backoff gating
// happens in SQL (see ListDuePullAdapters) so adapters in a cool-down
// window are not even returned here.
//
// IMPORTANT — cross-tenant by design: unlike the governance scans
// (checkShadowIT / checkDuplicateSerials / checkMissingLocation), which
// were converted to per-tenant loops in Phase 1.4, this puller is the
// *scheduler itself*. `ListDuePullAdapters` runs a single global query
// so the backoff predicate (`next_attempt_at <= now()`) is evaluated
// atomically across the whole fleet and the 5-minute tick does not
// multiply by tenant-count. Every downstream query, work order, and
// metric emitted by this loop is still scoped by a.TenantID — the
// cross-tenant read here is a read-only scheduler fan-out, not a
// tenant-mixing data path. Do NOT naively convert this to a per-tenant
// loop without redesigning the backoff schedule.
//
// Per-tenant observability is preserved via
// telemetry.AdapterPullAttemptsTotal with the tenant_id label, so an
// operator can still spot one tenant dominating the batch or flapping.
func (w *WorkflowSubscriber) pullMetricsFromAdapters(ctx context.Context) {
	adapters, err := w.queries.ListDuePullAdapters(ctx)
	if err != nil {
		zap.L().Warn("metrics puller: failed to query adapters", zap.Error(err))
		return
	}

	for _, a := range adapters {
		if !a.Endpoint.Valid || a.Endpoint.String == "" {
			continue
		}

		tenantLabel := a.TenantID.String()

		plainConfig, err := integration.DecryptConfigWithFallback(w.cipher, a.ConfigEncrypted, a.Config)
		if err != nil {
			zap.L().Warn("metrics puller: decrypt config failed",
				zap.String("tenant_id", tenantLabel),
				zap.String("adapter", a.Name), zap.Error(err))
			telemetry.AdapterPullAttemptsTotal.WithLabelValues(tenantLabel, "failure").Inc()
			continue
		}

		pullErr := w.pullFromAdapter(ctx, a.TenantID, a.Name, a.Type, a.Endpoint.String, plainConfig)
		if pullErr != nil {
			telemetry.AdapterPullAttemptsTotal.WithLabelValues(tenantLabel, "failure").Inc()
			w.handleAdapterFailure(ctx, a.ID, a.TenantID, a.Name, a.Type, pullErr)
			continue
		}

		telemetry.AdapterPullAttemptsTotal.WithLabelValues(tenantLabel, "success").Inc()

		if err := w.queries.RecordAdapterSuccess(ctx, dbgen.RecordAdapterSuccessParams{
			ID:       a.ID,
			TenantID: a.TenantID,
		}); err != nil {
			zap.L().Warn("metrics puller: failed to record success",
				zap.String("tenant_id", tenantLabel),
				zap.String("adapter", a.Name), zap.Error(err))
		}
	}
}

// handleAdapterFailure persists the failure, triggers auto-disable when
// the threshold is reached, and always emits an audit event so operators
// can reconstruct the escalation. Errors from the persistence path are
// logged but never propagate — a DB hiccup should not crash the puller.
func (w *WorkflowSubscriber) handleAdapterFailure(
	ctx context.Context,
	adapterID, tenantID uuid.UUID,
	name, adapterType string,
	pullErr error,
) {
	reason := truncateReason(pullErr.Error())

	row, err := w.queries.RecordAdapterFailure(ctx, dbgen.RecordAdapterFailureParams{
		ID:                adapterID,
		TenantID:          tenantID,
		LastFailureReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		zap.L().Error("metrics puller: failed to record failure",
			zap.String("adapter", name), zap.Error(err))
		return
	}

	zap.L().Warn("metrics puller: pull failed",
		zap.String("adapter", name),
		zap.String("type", adapterType),
		zap.Int32("consecutive_failures", row.ConsecutiveFailures),
		zap.Error(pullErr))

	if row.ConsecutiveFailures >= adapterDisableThreshold {
		w.disableAdapter(ctx, adapterID, tenantID, name, row.ConsecutiveFailures, reason)
	}
}

func (w *WorkflowSubscriber) pullFromAdapter(ctx context.Context, tenantID uuid.UUID, name, adapterType, endpoint string, configRaw []byte) error {
	adapter := GetAdapter(adapterType)
	if adapter == nil {
		// Fallback: try as prometheus for backward compat with type="rest"
		adapter = GetAdapter("prometheus")
	}
	if adapter == nil {
		return fmt.Errorf("unsupported adapter type: %s", adapterType)
	}

	points, err := adapter.Fetch(ctx, endpoint, configRaw)
	if err != nil {
		return err
	}

	for _, pt := range points {
		var assetID pgtype.UUID
		if pt.IP != "" {
			asset, err := w.queries.FindAssetByIP(ctx, dbgen.FindAssetByIPParams{
				TenantID:  tenantID,
				IpAddress: pgtype.Text{String: pt.IP, Valid: true},
			})
			if err == nil {
				assetID = pgtype.UUID{Bytes: asset.ID, Valid: true}
			}
		}
		labelsJSON, marshalErr := json.Marshal(pt.Labels)
		if marshalErr != nil {
			// A label map that can't be JSON-encoded is a bug in the
			// adapter, not a transient failure — emit the counter and
			// skip the row so the rest of the batch still lands.
			zap.L().Warn("metrics puller: labels marshal failed",
				zap.String("adapter", name),
				zap.String("metric", pt.Name),
				zap.Error(marshalErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceMetricsInsert, telemetry.ReasonJSONUnmarshal).Inc()
			continue
		}
		if _, execErr := w.pool.Exec(ctx,
			"INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels) VALUES ($1, $2, $3, $4, $5, $6)",
			pt.Timestamp, assetID, tenantID, pt.Name, pt.Value, labelsJSON,
		); execErr != nil {
			// The caller loops over an in-memory point slice, so a
			// single failed INSERT must not abort the batch. Emit
			// Warn + counter and continue advancing — the adapter
			// failure counter on the outer path catches the pattern
			// if writes stop succeeding entirely.
			zap.L().Warn("metrics puller: insert failed",
				zap.String("adapter", name),
				zap.String("metric", pt.Name),
				zap.Error(execErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceMetricsInsert, telemetry.ReasonDBExecFailed).Inc()
		}
	}

	zap.L().Debug("metrics puller: stored metrics",
		zap.String("adapter", name),
		zap.String("type", adapterType),
		zap.Int("count", len(points)))

	return nil
}

// disableAdapter flips enabled=false, emits an audit event, and notifies
// the tenant's ops-admin users. Each step fails-soft so a single error
// does not skip the subsequent operator-visible signal.
func (w *WorkflowSubscriber) disableAdapter(
	ctx context.Context,
	adapterID, tenantID uuid.UUID,
	name string,
	failureCount int32,
	reason string,
) {
	if err := w.queries.DisableAdapter(ctx, dbgen.DisableAdapterParams{
		ID:       adapterID,
		TenantID: tenantID,
	}); err != nil {
		zap.L().Error("metrics puller: failed to disable adapter",
			zap.String("adapter", name), zap.Error(err))
		return
	}
	zap.L().Warn("metrics puller: adapter auto-disabled",
		zap.String("adapter", name),
		zap.Int32("consecutive_failures", failureCount))

	w.emitAdapterDisabledAudit(ctx, tenantID, adapterID, name, failureCount, reason)

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.createNotification(ctx, tenantID, adminID,
			"adapter_error",
			fmt.Sprintf("Adapter '%s' disabled", name),
			fmt.Sprintf("The inbound adapter '%s' has been automatically disabled after %d consecutive pull failures. Last error: %s", name, failureCount, reason),
			"integration_adapter", adapterID)
	}
}

// emitAdapterDisabledAudit records an audit_events row so the escalation
// is visible in the audit UI. Uses the workflow pseudo-operator (zero
// UUID) since no human triggered the disable.
func (w *WorkflowSubscriber) emitAdapterDisabledAudit(
	ctx context.Context,
	tenantID, adapterID uuid.UUID,
	name string,
	failureCount int32,
	reason string,
) {
	diff := map[string]any{
		"adapter_name":         name,
		"consecutive_failures": failureCount,
		"last_failure_reason":  reason,
		"auto_disabled":        true,
	}
	diffJSON, _ := json.Marshal(diff)

	_, err := w.queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		TenantID:   tenantID,
		Action:     "adapter_auto_disabled",
		Module:     pgtype.Text{String: "integration", Valid: true},
		TargetType: pgtype.Text{String: "integration_adapter", Valid: true},
		TargetID:   pgtype.UUID{Bytes: adapterID, Valid: true},
		// operator_id left invalid: system action, not a user.
		OperatorID: pgtype.UUID{Valid: false},
		Diff:       diffJSON,
		Source:     "workflow",
	})
	if err != nil {
		zap.L().Warn("metrics puller: failed to emit audit event",
			zap.String("adapter", name), zap.Error(err))
	}
}
