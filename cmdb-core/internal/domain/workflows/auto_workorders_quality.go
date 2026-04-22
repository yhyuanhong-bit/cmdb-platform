package workflows

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// D9-P0 from review-2026-04-21-v2: when an asset's quality score
// stays below the intervention threshold for the whole lookback
// window, open a data-correction work order automatically. Without
// this loop low-quality CIs sit on the worst-assets dashboard
// forever with nobody assigned to fix them.
const (
	lowQualityThreshold   = 40.0
	lowQualityLookbackDays = 7

	dedupKindLowQualityPersistent = "low_quality_persistent"
)

// routingLabelFor maps an asset's nullable owner_team into the routing
// hint baked into the auto-WO title and description. A blank / unset
// owner_team is explicitly surfaced so the triage queue can still filter
// on it — silently falling back to an empty string would make owner-less
// work orders invisible to a "team-less queue" view.
func routingLabelFor(ownerTeam pgtype.Text) string {
	if ownerTeam.Valid && ownerTeam.String != "" {
		return "team " + ownerTeam.String
	}
	return "unassigned team"
}

// checkLowQualityPersistent fans out to every active tenant. A
// failure on one tenant never starves the remaining tenants of
// coverage — the same pattern every other daily scan uses.
func (w *WorkflowSubscriber) checkLowQualityPersistent(ctx context.Context) {
	tenants, err := w.queries.ListActiveTenants(ctx)
	if err != nil {
		zap.L().Warn("low-quality checker: list active tenants failed", zap.Error(err))
		return
	}
	for _, t := range tenants {
		if err := w.checkLowQualityPersistentForTenant(ctx, t.ID); err != nil {
			zap.L().Warn("low-quality checker: tenant scan failed",
				zap.String("tenant_id", t.ID.String()),
				zap.Error(err))
		}
	}
}

// checkLowQualityPersistentForTenant picks up every asset whose
// max-per-day score stayed strictly below lowQualityThreshold across
// every day in the window, and whose scanner has actually run on
// each of those days (the query's days_covered guard keeps an
// outage-induced scoring gap from triggering a work order storm).
func (w *WorkflowSubscriber) checkLowQualityPersistentForTenant(ctx context.Context, tenantID uuid.UUID) error {
	assetIDs, err := w.queries.AssetsLowQualityPersistent(ctx, dbgen.AssetsLowQualityPersistentParams{
		TenantID: tenantID,
		Column2:  lowQualityThreshold,
		Column3:  lowQualityLookbackDays,
	})
	if err != nil {
		return fmt.Errorf("list low-quality persistent: %w", err)
	}

	for _, assetID := range assetIDs {
		if err := w.createLowQualityWorkOrder(ctx, tenantID, assetID); err != nil {
			zap.L().Warn("low-quality checker: WO creation skipped",
				zap.String("tenant_id", tenantID.String()),
				zap.String("asset_id", assetID.String()),
				zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLowQualityCheck, reasonWOCreateFailed).Inc()
			continue
		}
		zap.L().Info("low-quality checker: created data-correction WO",
			zap.String("tenant_id", tenantID.String()),
			zap.String("asset_id", assetID.String()))
	}
	return nil
}

// createLowQualityWorkOrder atomically creates a data-correction WO
// and its matching work_order_dedup row. Asset ID is the dedup key,
// so a subsequent daily tick cannot stamp out duplicates for the
// same asset while the first WO is still open.
func (w *WorkflowSubscriber) createLowQualityWorkOrder(ctx context.Context, tenantID, assetID uuid.UUID) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceLowQualityCheck)
	if !ok {
		return fmt.Errorf("resolve system user: skipped")
	}

	// Resolve asset name/tag/owner_team inside the tx for a stable snapshot
	// and so we never hand the operator a stale tag if the asset was just
	// renamed mid-scan. owner_team is the D9-P1 routing label — baked into
	// the order title and description so the triage queue UI can filter
	// by team without a second lookup.
	var assetName, assetTag string
	var ownerTeam pgtype.Text
	if scanErr := tx.QueryRow(ctx,
		`SELECT name, asset_tag, owner_team FROM assets WHERE id = $1 AND tenant_id = $2`,
		assetID, tenantID).Scan(&assetName, &assetTag, &ownerTeam); scanErr != nil {
		return fmt.Errorf("lookup asset: %w", scanErr)
	}

	routing := routingLabelFor(ownerTeam)

	order, err := w.maintenanceSvc.CreateTx(ctx, tx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:    fmt.Sprintf("Data Quality: %s (%s) score below %.0f for %dd [%s]", assetName, assetTag, lowQualityThreshold, lowQualityLookbackDays, routing),
		Type:     "data_correction",
		Priority: "high",
		AssetID:  &assetID,
		Description: fmt.Sprintf(
			"Asset '%s' (tag: %s) has scored below %.0f on every daily quality scan for the last %d days. "+
				"Route to: %s. "+
				"Investigate the asset's CMDB record, resolve the issues flagged on the quality dashboard, and close this order once the score recovers.",
			assetName, assetTag, lowQualityThreshold, lowQualityLookbackDays, routing),
	})
	if err != nil {
		return fmt.Errorf("create data-correction WO: %w", err)
	}

	n, err := w.queries.WithTx(tx).InsertWorkOrderDedup(ctx, dbgen.InsertWorkOrderDedupParams{
		TenantID:    tenantID,
		WorkOrderID: order.ID,
		DedupKind:   dedupKindLowQualityPersistent,
		DedupKey:    assetID.String(),
	})
	if err != nil {
		return fmt.Errorf("insert dedup: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("dedup race: %s key %s already open", dedupKindLowQualityPersistent, assetID.String())
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
