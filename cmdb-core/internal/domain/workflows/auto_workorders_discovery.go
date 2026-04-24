package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// --- Auto Work Order 11: Discovery unreviewed > 24h → Governance review ---
//
// Wave 3 promise: if a discovery sits in pending/conflict status long
// enough that a human should have looked at it, open a governance work
// order so the data-governance team sees it in their queue instead of
// letting it rot in the discovered_assets table.
//
// Dedup: one WO per discovered_asset.id. Once the discovery is approved
// or ignored, the row drops out of our LEFT JOIN and we stop generating
// follow-up WOs for it even if the dedup row is still in place.

const (
	// discoveryReviewThresholdHours drives when a pending/conflict
	// discovery becomes a governance ticket. Matches the spec and the
	// review-gate SLA operators are signed up for.
	discoveryReviewThresholdHours = 24

	// discoveryReviewBatchLimit caps how many WOs a single scheduler
	// tick can open so a fresh deployment with years of backlog does
	// not blast through tens of thousands of tickets in one run.
	discoveryReviewBatchLimit = 200
)

// checkUnreviewedDiscoveries scans discovered_assets across all tenants,
// opens a governance-review WO for anything stuck in pending/conflict
// past the threshold. Cross-tenant by design — governance audit is not
// tenant-specific; the WO itself is written with the correct tenant_id.
func (w *WorkflowSubscriber) checkUnreviewedDiscoveries(ctx context.Context) {
	rows, err := w.queries.ListUnreviewedOverdue(ctx, dbgen.ListUnreviewedOverdueParams{
		Hours: discoveryReviewThresholdHours,
		Limit: int32(discoveryReviewBatchLimit),
	})
	if err != nil {
		zap.L().Warn("discovery unreviewed: list query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDiscoveryUnreviewed, telemetry.ReasonDBExecFailed).Inc()
		return
	}

	for _, r := range rows {
		daysPending := int(time.Since(r.DiscoveredAt).Hours() / 24)
		if err := w.createUnreviewedDiscoveryWO(ctx, r, daysPending); err != nil {
			zap.L().Warn("discovery unreviewed: WO creation skipped",
				zap.String("tenant_id", r.TenantID.String()),
				zap.String("discovered_asset_id", r.ID.String()),
				zap.String("status", r.Status),
				zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDiscoveryUnreviewed, reasonWOCreateFailed).Inc()
			continue
		}
		zap.L().Info("discovery unreviewed: opened governance WO",
			zap.String("tenant_id", r.TenantID.String()),
			zap.String("discovered_asset_id", r.ID.String()),
			zap.String("status", r.Status),
			zap.Int("days_pending", daysPending))
	}
}

// createUnreviewedDiscoveryWO opens a single governance-review WO under
// transaction so the dedup row and the WO land atomically. If dedup
// reports 0 rows (a concurrent scan beat us), the rollback keeps us
// from leaving orphan tickets.
func (w *WorkflowSubscriber) createUnreviewedDiscoveryWO(
	ctx context.Context,
	row dbgen.ListUnreviewedOverdueRow,
	daysPending int,
) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sysUID, ok := w.resolveSystemUser(ctx, row.TenantID, sourceDiscoveryUnreviewed)
	if !ok {
		return fmt.Errorf("resolve system user: skipped")
	}

	hostname := "unknown"
	if row.Hostname.Valid {
		hostname = row.Hostname.String
	}
	ipStr := "unknown"
	if row.IpAddress.Valid {
		ipStr = row.IpAddress.String
	}

	// Title + description mention the discovery ID so the governance
	// team can find the row in the review queue UI.
	priority := "medium"
	if row.Status == "conflict" {
		// Conflicts already have a known collision with an existing CI
		// — they're more time-sensitive than simple unreviewed
		// "pending" rows.
		priority = "high"
	}

	order, err := w.maintenanceSvc.CreateTx(ctx, tx, row.TenantID, sysUID, maintenance.CreateOrderRequest{
		Title: fmt.Sprintf("Discovery review overdue: %s (%s) — %d days", hostname, ipStr, daysPending),
		Type:  "discovery_review",
		Priority: priority,
		Description: fmt.Sprintf(
			"A discovered asset has been in status '%s' for %d days without review.\n\n"+
				"Source: %s\nHostname: %s\nIP: %s\nDiscovered: %s\nDiscovery ID: %s\n\n"+
				"Action: open the Discovery Review page, inspect the match (if any), "+
				"and either approve (creates a CI) or ignore (rejects the row) with a reason.",
			row.Status, daysPending, row.Source, hostname, ipStr,
			row.DiscoveredAt.Format(time.RFC3339), row.ID.String(),
		),
	})
	if err != nil {
		return fmt.Errorf("create discovery review WO: %w", err)
	}

	// Dedup key is the discovered_asset_id — we only want one open WO
	// per unreviewed row, not a new one per scan tick.
	n, err := w.queries.WithTx(tx).InsertWorkOrderDedup(ctx, dbgen.InsertWorkOrderDedupParams{
		TenantID:    row.TenantID,
		WorkOrderID: order.ID,
		DedupKind:   dedupKindDiscoveryUnreviewed,
		DedupKey:    row.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("insert dedup: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("dedup race: discovery_unreviewed key %s already exists", row.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	w.maintenanceSvc.BumpSyncVersionAfterCreate(ctx, order.ID, row.TenantID)
	return nil
}

// StartDiscoveryReviewChecker launches the periodic scan that opens
// governance work orders for long-unreviewed discoveries. Hourly cadence
// is frequent enough that overdue rows surface within the same shift
// they crossed the 24h threshold without spamming the scheduler.
func (w *WorkflowSubscriber) StartDiscoveryReviewChecker(ctx context.Context) {
	// Run once immediately so a restart does not leave overdue rows
	// invisible until the first tick.
	go w.checkUnreviewedDiscoveries(ctx)

	ticker := time.NewTicker(time.Hour)
	zap.L().Info("discovery review checker started", zap.Int("interval_hours", 1))
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.checkUnreviewedDiscoveries(ctx)
			}
		}
	}()
	_ = uuid.Nil // keep uuid import used even if future edits drop direct refs
}
