package monitoring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Wave 5.4: alert → incident bridge.
//
// When the evaluator emits a high-signal firing alert (severity ∈
// {critical, high, warning}) for a known asset, we tie it to an
// incident so operators have a single place to coordinate response.
// The rules are:
//
//   1. Look for an existing open / acknowledged / investigating incident
//      on the same asset within the last 24 hours.  Found → attach this
//      alert to it (UPDATE alert_events.incident_id) and append a system
//      comment to the timeline.  This collapses follow-up alerts of the
//      same fault onto one incident, which is what an operator wants
//      during an active page.
//   2. No open incident → create a new one (status='open', severity from
//      alert, affected_asset_id from alert), then attach + comment.
//   3. Low-severity alerts and alerts without an asset are skipped — we
//      don't want noise piling up incidents that nobody will act on.
//
// All three steps run in one tx so the timeline cannot drift from the
// alert↔incident link.  The bridge is OPTIONAL (the evaluator works
// without it for unit tests that don't care about ITSM) — if the pool
// passed to NewEvaluator via WithIncidentBridge is nil, this code is a
// no-op.

// bridgeable severities. Keep this in sync with the incidents.severity
// CHECK constraint (000065). Anything outside this set is ignored — we
// don't auto-page for "info" alerts.
var bridgeableSeverity = map[string]bool{
	"critical": true,
	"high":     true,
	"warning":  true,
}

// dedupeWindow is how far back we look for an existing open incident on
// the same asset. 24h matches the auto-workorder cadence so an incident
// that's been ignored for a day starts fresh rather than piling all
// recurring alerts onto one ancient row.
const incidentDedupeWindow = 24 * time.Hour

// incidentBridge owns the alert→incident translation. Held by the
// Evaluator and called from emit().
type incidentBridge struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
	now    func() time.Time
}

// WithIncidentBridge enables the alert→incident auto-link (Wave 5.4).
// pool MUST be the same DB the alert_events table lives on; we open
// a tx for each linkage so the comment write and the FK update commit
// atomically. If pool is nil this option is a no-op.
func WithIncidentBridge(pool *pgxpool.Pool) Option {
	return func(e *Evaluator) {
		if pool == nil {
			return
		}
		e.bridge = &incidentBridge{
			pool:   pool,
			logger: e.logger,
			now:    e.now,
		}
	}
}

// onAlertEmitted is called by emit() after a successful upsert, with the
// alert's id, status, severity and affected asset. Errors are logged but
// not propagated — a bridge failure must never lose the alert itself.
func (b *incidentBridge) onAlertEmitted(
	ctx context.Context,
	tenantID, alertID, assetID uuid.UUID,
	severity, status, message string,
) {
	if b == nil {
		return
	}

	// Only the firing edge spawns / attaches incidents. Resolved alerts
	// just append a follow-up comment to whatever incident they were
	// already tied to.
	switch status {
	case "firing":
		if !bridgeableSeverity[severity] {
			return
		}
		if assetID == uuid.Nil {
			return
		}
		if err := b.linkOrCreate(ctx, tenantID, alertID, assetID, severity, message); err != nil {
			b.logger.Warn("incident bridge: link/create failed",
				zap.String("alert_id", alertID.String()),
				zap.String("tenant_id", tenantID.String()),
				zap.Error(err))
		}
	case "resolved":
		if err := b.commentOnResolved(ctx, tenantID, alertID, message); err != nil {
			b.logger.Warn("incident bridge: resolved-comment failed",
				zap.String("alert_id", alertID.String()),
				zap.Error(err))
		}
	}
}

func (b *incidentBridge) linkOrCreate(
	ctx context.Context,
	tenantID, alertID, assetID uuid.UUID,
	severity, message string,
) error {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Step 1: find an existing open incident on the same asset within
	// the dedupe window. Lock the row FOR UPDATE so two concurrent
	// emits on the same asset don't both decide there's no incident
	// and each create their own.
	var existingID uuid.UUID
	cutoff := b.now().Add(-incidentDedupeWindow)
	err = tx.QueryRow(ctx, `
		SELECT id FROM incidents
		WHERE tenant_id = $1
		  AND affected_asset_id = $2
		  AND status IN ('open', 'acknowledged', 'investigating')
		  AND started_at >= $3
		ORDER BY started_at DESC
		LIMIT 1
		FOR UPDATE
	`, tenantID, assetID, cutoff).Scan(&existingID)

	var (
		incidentID uuid.UUID
		commentBody string
	)

	switch {
	case err == nil:
		// Attach to the existing incident.
		incidentID = existingID
		commentBody = "alert attached: " + message
	case errors.Is(err, pgx.ErrNoRows):
		// No open incident — create one.
		title := buildIncidentTitle(message)
		if err := tx.QueryRow(ctx, `
			INSERT INTO incidents (tenant_id, title, status, severity, affected_asset_id)
			VALUES ($1, $2, 'open', $3, $4)
			RETURNING id
		`, tenantID, title, severity, assetID).Scan(&incidentID); err != nil {
			return fmt.Errorf("create incident: %w", err)
		}
		commentBody = "incident opened from alert: " + message
	default:
		return fmt.Errorf("look up open incident: %w", err)
	}

	// Step 2: stamp the alert_events row with the incident_id pointer.
	if _, err := tx.Exec(ctx, `
		UPDATE alert_events SET incident_id = $1
		WHERE id = $2 AND tenant_id = $3
	`, incidentID, alertID, tenantID); err != nil {
		return fmt.Errorf("link alert to incident: %w", err)
	}

	// Step 3: write a system comment to the incident's timeline.  We
	// pass author_id NULL because alerts come from the evaluator, not
	// a human user; the comment kind='system' makes that explicit in
	// the UI.
	if _, err := tx.Exec(ctx, `
		INSERT INTO incident_comments (tenant_id, incident_id, author_id, kind, body)
		VALUES ($1, $2, NULL, 'system', $3)
	`, tenantID, incidentID, commentBody); err != nil {
		return fmt.Errorf("write timeline comment: %w", err)
	}

	return tx.Commit(ctx)
}

// commentOnResolved appends a "alert resolved" line to the incident
// timeline if the alert was previously linked. We deliberately do NOT
// auto-resolve the incident — the alert's recovery doesn't always mean
// the underlying problem is fixed (e.g. a flapping CPU spike); the
// operator decides when to flip the incident.
func (b *incidentBridge) commentOnResolved(ctx context.Context, tenantID, alertID uuid.UUID, message string) error {
	var incidentID *uuid.UUID
	if err := b.pool.QueryRow(ctx, `
		SELECT incident_id FROM alert_events
		WHERE id = $1 AND tenant_id = $2
	`, alertID, tenantID).Scan(&incidentID); err != nil {
		return fmt.Errorf("look up alert linkage: %w", err)
	}
	if incidentID == nil {
		// Alert wasn't linked — resolved comment is just noise.
		return nil
	}

	if _, err := b.pool.Exec(ctx, `
		INSERT INTO incident_comments (tenant_id, incident_id, author_id, kind, body)
		VALUES ($1, $2, NULL, 'system', $3)
	`, tenantID, *incidentID, "alert resolved: "+message); err != nil {
		return fmt.Errorf("write resolved comment: %w", err)
	}
	return nil
}

// buildIncidentTitle takes the alert message and produces a concise
// incident title. Alert messages are deliberately verbose ("rule X
// firing: avg(cpu_usage) = 92.3 > 80"); the incident title just needs
// to convey the gist. We trim to first 120 chars to keep it inside
// the VARCHAR(255) bound and leave room for prefixing if a future
// caller wants to add e.g. a runbook tag.
func buildIncidentTitle(message string) string {
	const cap = 120
	if len(message) > cap {
		return message[:cap-1] + "…"
	}
	return message
}
