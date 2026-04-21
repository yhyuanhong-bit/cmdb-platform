package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Dedup kinds written to the work_order_dedup table by the auto-WO
// scans (Phase 2.15). Keep these in sync with the backfill in
// 000049_work_order_dedup.up.sql — a typo here silently defeats dedup.
const (
	dedupKindShadowIT        = "shadow_it"
	dedupKindDuplicateSerial = "duplicate_serial"
)

// Source labels for telemetry.ErrorsSuppressedTotal emitted from the
// auto-work-order scans. Each scan gets its own label so a broken
// fixture (e.g. a renamed column) lights up the exact source instead
// of being aggregated into a single unactionable bucket.
const (
	sourceWarrantyCheck         = "workflows.auto_wo.warranty"
	sourceAssetVerification     = "workflows.auto_wo.asset_verification"
	sourceDataCorrection        = "workflows.auto_wo.data_correction"
	sourceEOLCheck              = "workflows.auto_wo.eol"
	sourceLifespanCheck         = "workflows.auto_wo.lifespan"
	sourceShadowITCheck         = "workflows.auto_wo.shadow_it"
	sourceDuplicateSerialCheck  = "workflows.auto_wo.duplicate_serial"
	sourceMissingLocationCheck  = "workflows.auto_wo.missing_location"
	sourceFirmwareCheck         = "workflows.auto_wo.firmware"
	sourceBMCSecurityCheck      = "workflows.auto_wo.bmc_security"
	sourceScanDiffEventHandler  = "workflows.auto_wo.scan_diff_event"
	sourceBMCDefaultEventParse  = "workflows.auto_wo.bmc_default_event"
)

// Reason used when maintenanceSvc.Create fails in an auto-WO scan.
// These failures were previously Debug-logged, which made a broken
// maintenance service invisible in production dashboards. Promote to
// Warn + counter so a spike shows up on the errors_suppressed panel.
const reasonWOCreateFailed = telemetry.ReasonWOCreationFailed

// --- Auto Work Order 1: Warranty Expiry → Renewal Evaluation ---

// StartWarrantyChecker runs daily to check for assets approaching warranty expiry.
func (w *WorkflowSubscriber) StartWarrantyChecker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		func() {
			tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.warranty_daily")
			defer end()
			w.runDailyChecks(tickCtx)
		}()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.warranty_daily")
				w.runDailyChecks(tickCtx)
				end()
			}
		}
	}()
	zap.L().Info("Daily data governance checker started (24h interval)")
}

// runDailyChecks executes all daily data governance work order checks in sequence.
func (w *WorkflowSubscriber) runDailyChecks(ctx context.Context) {
	w.checkWarrantyExpiry(ctx)
	w.checkEOLReached(ctx)
	w.checkOverLifespan(ctx)
	w.checkFirmwareOutdated(ctx)
}

func (w *WorkflowSubscriber) checkWarrantyExpiry(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.warranty_end, a.warranty_vendor
		 FROM assets a
		 WHERE a.warranty_end IS NOT NULL
		   AND a.warranty_end > now()
		   AND a.warranty_end <= now() + interval '30 days'
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'warranty_renewal'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("warranty checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var warrantyEnd time.Time
		var warrantyVendor *string
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &warrantyEnd, &warrantyVendor); scanErr != nil {
			zap.L().Warn("warranty checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		daysLeft := int(time.Until(warrantyEnd).Hours() / 24)
		vendor := "N/A"
		if warrantyVendor != nil {
			vendor = *warrantyVendor
		}

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceWarrantyCheck)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Warranty Renewal: %s (%s)", name, assetTag),
			Type:        "warranty_renewal",
			Priority:    "medium",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' warranty expires in %d days (vendor: %s, expiry: %s). Evaluate: renew warranty, plan replacement, or accept risk.", name, daysLeft, vendor, warrantyEnd.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Warn("warranty checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceWarrantyCheck, tenantID, adminID,
				"warranty_expiry",
				fmt.Sprintf("Warranty expiring: %s", assetTag),
				fmt.Sprintf("Asset '%s' warranty expires in %d days. Work order created.", name, daysLeft),
				"work_order", order.ID)
		}

		zap.L().Info("warranty checker: created renewal WO",
			zap.String("asset", assetTag),
			zap.Int("days_left", daysLeft))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("warranty checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, telemetry.ReasonRowsIterFailed).Inc()
	}
}

// --- Auto Work Order 2: CMDB record not seen by scan → Asset Verification ---

// StartAssetVerificationChecker runs weekly to find assets not detected by any network scan.
func (w *WorkflowSubscriber) StartAssetVerificationChecker(ctx context.Context) {
	ticker := time.NewTicker(7 * 24 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.asset_verification_weekly")
				w.runWeeklyChecks(tickCtx)
				end()
			}
		}
	}()
	zap.L().Info("Weekly data governance checker started (7d interval)")
}

// runWeeklyChecks executes all weekly data governance work order checks in sequence.
func (w *WorkflowSubscriber) runWeeklyChecks(ctx context.Context) {
	w.checkMissingAssets(ctx)
	w.checkShadowIT(ctx)
	w.checkDuplicateSerials(ctx)
	w.checkMissingLocation(ctx)
}

func (w *WorkflowSubscriber) checkMissingAssets(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.ip_address, a.bmc_ip
		 FROM assets a
		 WHERE a.deleted_at IS NULL
		   AND a.status NOT IN ('disposed', 'decommission', 'procurement')
		   AND (a.ip_address IS NOT NULL OR a.bmc_ip IS NOT NULL)
		   AND NOT EXISTS (
		     SELECT 1 FROM discovered_assets da
		     WHERE da.tenant_id = a.tenant_id
		     AND (da.ip_address = a.ip_address OR da.ip_address = a.bmc_ip)
		     AND da.created_at > now() - interval '30 days'
		   )
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'asset_verification'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("asset verification checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceAssetVerification, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var ipAddress, bmcIP *string
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &ipAddress, &bmcIP); scanErr != nil {
			zap.L().Warn("asset verification checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceAssetVerification, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		ip := "N/A"
		if ipAddress != nil {
			ip = *ipAddress
		} else if bmcIP != nil {
			ip = *bmcIP
		}

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceAssetVerification)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Asset Verification: %s (%s)", name, assetTag),
			Type:        "asset_verification",
			Priority:    "low",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' (IP: %s) has not been detected by any network scan in the last 30 days. Please verify: is the asset still physically present? Has it been relocated? Is it powered off?", name, ip),
		})
		if err != nil {
			zap.L().Warn("asset verification checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceAssetVerification, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceAssetVerification, tenantID, adminID,
				"asset_verification",
				fmt.Sprintf("Asset not detected: %s", assetTag),
				fmt.Sprintf("Asset '%s' not seen by scans in 30 days. Work order created for verification.", name),
				"work_order", order.ID)
		}

		zap.L().Info("asset verification checker: created WO",
			zap.String("asset", assetTag))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("asset verification checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceAssetVerification, telemetry.ReasonRowsIterFailed).Inc()
	}
}

// --- Auto Work Order 3: Scan data differs from CMDB → Data Correction ---

// scanDifferencesPayload is the expected event payload for scan differences.
type scanDifferencesPayload struct {
	AssetID   string                 `json:"asset_id"`
	AssetTag  string                 `json:"asset_tag"`
	AssetName string                 `json:"asset_name"`
	Diffs     map[string]interface{} `json:"diffs"`
}

// onScanDifferencesDetected handles SubjectScanDifferencesDetected events published by the
// IPMI collector or discovery pipeline when field values diverge from CMDB records.
func (w *WorkflowSubscriber) onScanDifferencesDetected(ctx context.Context, event eventbus.Event) error {
	var payload scanDifferencesPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		zap.L().Warn("workflow: failed to parse scan differences payload", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceScanDiffEventHandler, telemetry.ReasonJSONUnmarshal).Inc()
		return nil
	}

	tenantID, _ := uuid.Parse(event.TenantID)
	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil || tenantID == uuid.Nil {
		return nil
	}

	w.checkScanDifferences(ctx, tenantID, assetID, payload.AssetTag, payload.AssetName, payload.Diffs)
	return nil
}

// checkScanDifferences creates a data correction WO when scan results differ from CMDB.
// It can be called directly from the IPMI collector or via the SubjectScanDifferencesDetected event.
func (w *WorkflowSubscriber) checkScanDifferences(ctx context.Context, tenantID, assetID uuid.UUID, assetTag, assetName string, diffs map[string]interface{}) {
	if len(diffs) == 0 {
		return
	}

	var existingCount int
	if scanErr := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM work_orders
		 WHERE asset_id = $1 AND type = 'data_correction'
		 AND status NOT IN ('completed','verified','rejected')
		 AND deleted_at IS NULL`,
		assetID).Scan(&existingCount); scanErr != nil {
		// If the dedup probe fails we prefer to skip rather than
		// double-create — a broken probe re-issuing a WO every scan
		// is strictly worse than a missed cycle.
		zap.L().Warn("data correction: dedup probe failed", zap.String("asset", assetTag), zap.Error(scanErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDataCorrection, telemetry.ReasonRowScanFailed).Inc()
		return
	}
	if existingCount > 0 {
		return
	}

	diffLines := make([]string, 0, len(diffs))
	for field, val := range diffs {
		if m, ok := val.(map[string]interface{}); ok {
			diffLines = append(diffLines, fmt.Sprintf("- %s: CMDB='%v' → Scanned='%v'", field, m["cmdb"], m["scanned"]))
		}
	}
	if len(diffLines) == 0 {
		return
	}

	description := fmt.Sprintf(
		"Network scan detected data inconsistencies for asset '%s' (%s):\n\n%s\n\nPlease verify and update CMDB records.",
		assetName, assetTag, strings.Join(diffLines, "\n"))

	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceDataCorrection)
	if !ok {
		return
	}
	order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Data Correction: %s (%s)", assetName, assetTag),
		Type:        "data_correction",
		Priority:    "low",
		AssetID:     &assetID,
		Description: description,
	})
	if err != nil {
		zap.L().Warn("data correction: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDataCorrection, reasonWOCreateFailed).Inc()
		return
	}

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.warnNotify(ctx, sourceDataCorrection, tenantID, adminID,
			"data_correction",
			fmt.Sprintf("Data mismatch: %s", assetTag),
			fmt.Sprintf("%d field(s) differ between scan and CMDB for '%s'.", len(diffs), assetName),
			"work_order", order.ID)
	}

	zap.L().Info("data correction WO created",
		zap.String("asset", assetTag),
		zap.Int("diff_count", len(diffs)))
}

// --- Auto Work Order 4: EOL Reached → Decommission ---

func (w *WorkflowSubscriber) checkEOLReached(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.eol_date
		 FROM assets a
		 WHERE a.eol_date IS NOT NULL
		   AND a.eol_date <= now()
		   AND a.status NOT IN ('disposed', 'decommission')
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'decommission'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("eol checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var eolDate time.Time
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &eolDate); scanErr != nil {
			zap.L().Warn("eol checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		daysPast := int(time.Since(eolDate).Hours() / 24)

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceEOLCheck)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Decommission: %s (%s)", name, assetTag),
			Type:        "decommission",
			Priority:    "high",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' reached end-of-life %d days ago (EOL: %s). Action required: data migration, service failover, physical removal, and CMDB status update to 'disposed'.", name, daysPast, eolDate.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Warn("eol checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceEOLCheck, tenantID, adminID, "eol_reached",
				fmt.Sprintf("EOL reached: %s", assetTag),
				fmt.Sprintf("Asset '%s' has passed its end-of-life date. Decommission work order created.", name),
				"work_order", order.ID)
		}
		zap.L().Info("eol checker: created decommission WO", zap.String("asset", assetTag))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("eol checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, telemetry.ReasonRowsIterFailed).Inc()
	}
}

// --- Auto Work Order 5: Over Expected Lifespan → Lifespan Evaluation ---

func (w *WorkflowSubscriber) checkOverLifespan(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.expected_lifespan_months, a.created_at
		 FROM assets a
		 WHERE a.expected_lifespan_months IS NOT NULL
		   AND a.created_at + (a.expected_lifespan_months || ' months')::interval < now()
		   AND a.status NOT IN ('disposed', 'decommission')
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'lifespan_evaluation'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("lifespan checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var lifespanMonths int
		var createdAt time.Time
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &lifespanMonths, &createdAt); scanErr != nil {
			zap.L().Warn("lifespan checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		actualMonths := int(time.Since(createdAt).Hours() / 24 / 30)

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceLifespanCheck)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Lifespan Evaluation: %s (%s)", name, assetTag),
			Type:        "lifespan_evaluation",
			Priority:    "medium",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' has been in service for %d months, exceeding the expected lifespan of %d months. Evaluate: continue operation, plan replacement, or schedule decommission.", name, actualMonths, lifespanMonths),
		})
		if err != nil {
			zap.L().Warn("lifespan checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceLifespanCheck, tenantID, adminID, "lifespan_exceeded",
				fmt.Sprintf("Lifespan exceeded: %s", assetTag),
				fmt.Sprintf("Asset '%s' exceeded expected %d-month lifespan.", name, lifespanMonths),
				"work_order", order.ID)
		}
		zap.L().Info("lifespan checker: created evaluation WO", zap.String("asset", assetTag))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("lifespan checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, telemetry.ReasonRowsIterFailed).Inc()
	}
}

// --- Auto Work Order 6: Shadow IT — Discovered but not in CMDB ---

// checkShadowIT iterates active tenants and runs the shadow-IT scan per
// tenant. A failure in one tenant is logged and does not abort the batch,
// so a single tenant's bad data cannot starve the remaining tenants of
// scan coverage.
func (w *WorkflowSubscriber) checkShadowIT(ctx context.Context) {
	tenants, err := w.queries.ListActiveTenants(ctx)
	if err != nil {
		zap.L().Warn("shadow IT checker: list active tenants failed", zap.Error(err))
		return
	}
	for _, t := range tenants {
		if err := w.checkShadowITForTenant(ctx, t.ID); err != nil {
			zap.L().Warn("shadow IT checker: tenant scan failed",
				zap.String("tenant_id", t.ID.String()),
				zap.Error(err))
			// Continue to next tenant — one failure must not abort the batch.
		}
	}
}

// checkShadowITForTenant runs the shadow-IT scan for a single tenant.
// Both the discovered_assets source and the dedup predicate are scoped
// to tenantID, so a WO seeded under tenant A cannot suppress a WO
// needed under tenant B.
//
// Dedup is enforced via the `work_order_dedup` table (Phase 2.15): the
// prior `wo.description LIKE '%IP:xxx%'` probe was replaced with an
// indexed (tenant_id, dedup_kind, dedup_key) lookup, and the WO insert
// + dedup insert now share one transaction so a crash between them can
// never produce an orphan WO or a ghost dedup entry.
func (w *WorkflowSubscriber) checkShadowITForTenant(ctx context.Context, tenantID uuid.UUID) error {
	// Drive-by correctness: the previous SQL referenced `da.created_at`,
	// which does not exist on discovered_assets — the real column is
	// `discovered_at` (see 000016_discovered_assets.up.sql). The
	// pre-refactor query therefore errored on every run and the scan
	// never actually emitted a WO. Switching to `discovered_at`
	// preserves the intended semantics ("discovered >7 days ago and
	// still unreviewed") and is required for this per-tenant refactor
	// to produce any observable behavior.
	rows, err := w.pool.Query(ctx,
		`SELECT da.id, da.hostname, da.ip_address, da.source, da.discovered_at
		 FROM discovered_assets da
		 WHERE da.tenant_id = $1
		   AND da.status = 'pending'
		   AND da.matched_asset_id IS NULL
		   AND da.discovered_at < now() - interval '7 days'
		   AND NOT EXISTS (
		     SELECT 1 FROM work_order_dedup wod
		     WHERE wod.tenant_id  = $1
		       AND wod.dedup_kind = 'shadow_it'
		       AND wod.dedup_key  = da.ip_address
		   )`, tenantID)
	if err != nil {
		return fmt.Errorf("query discovered_assets: %w", err)
	}

	type shadowITCandidate struct {
		daID         uuid.UUID
		hostname     string
		ipAddress    string
		source       string
		discoveredAt time.Time
	}
	var candidates []shadowITCandidate
	for rows.Next() {
		var c shadowITCandidate
		if err := rows.Scan(&c.daID, &c.hostname, &c.ipAddress, &c.source, &c.discoveredAt); err != nil {
			zap.L().Warn("shadow IT checker: row scan failed",
				zap.String("tenant_id", tenantID.String()), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceShadowITCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		candidates = append(candidates, c)
	}
	iterErr := rows.Err()
	// Close the read cursor BEFORE starting per-candidate write txs so
	// pgxpool can reuse the same connection — otherwise each iteration
	// would hold two connections (cursor + tx) and a busy scan could
	// starve the pool.
	rows.Close()
	if iterErr != nil {
		return fmt.Errorf("iterate discovered_assets: %w", iterErr)
	}

	for _, c := range candidates {
		daysPending := int(time.Since(c.discoveredAt).Hours() / 24)

		if err := w.createShadowITWorkOrder(ctx, tenantID, c.hostname, c.ipAddress, c.source, daysPending); err != nil {
			zap.L().Warn("shadow IT checker: WO creation skipped",
				zap.String("tenant_id", tenantID.String()),
				zap.String("ip", c.ipAddress), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceShadowITCheck, reasonWOCreateFailed).Inc()
			continue
		}
		zap.L().Info("shadow IT checker: created registration WO",
			zap.String("tenant_id", tenantID.String()),
			zap.String("ip", c.ipAddress),
			zap.String("hostname", c.hostname))
	}
	return nil
}

// createShadowITWorkOrder atomically creates a shadow-IT WO and its
// matching work_order_dedup row. If another scan raced us to the same
// (tenant, shadow_it, ip) key, the dedup INSERT ... ON CONFLICT DO
// NOTHING reports 0 rows and we roll the tx back so no orphan WO lands.
func (w *WorkflowSubscriber) createShadowITWorkOrder(ctx context.Context, tenantID uuid.UUID, hostname, ipAddress, source string, daysPending int) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // safe no-op after Commit.

	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceShadowITCheck)
	if !ok {
		return fmt.Errorf("resolve system user: skipped")
	}
	order, err := w.maintenanceSvc.CreateTx(ctx, tx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Shadow IT: Unregistered device %s (%s)", hostname, ipAddress),
		Type:        "shadow_it_registration",
		Priority:    "medium",
		Description: fmt.Sprintf("Network scan (%s) discovered device '%s' (IP: %s) %d days ago but it has no matching CMDB record. This is potential shadow IT. Action: register as new asset, or mark as ignored in discovery.", source, hostname, ipAddress, daysPending),
	})
	if err != nil {
		return fmt.Errorf("create shadow IT WO: %w", err)
	}

	n, err := w.queries.WithTx(tx).InsertWorkOrderDedup(ctx, dbgen.InsertWorkOrderDedupParams{
		TenantID:    tenantID,
		WorkOrderID: order.ID,
		DedupKind:   dedupKindShadowIT,
		DedupKey:    ipAddress,
	})
	if err != nil {
		return fmt.Errorf("insert dedup: %w", err)
	}
	if n == 0 {
		// A concurrent scan inserted the same (tenant, shadow_it, ip)
		// between our NOT EXISTS probe and this insert. Rolling back
		// via the deferred Rollback keeps the WO out of the table.
		return fmt.Errorf("dedup race: shadow_it key %s already exists", ipAddress)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	w.maintenanceSvc.BumpSyncVersionAfterCreate(ctx, order.ID)
	return nil
}

// --- Auto Work Order 7: Duplicate Serial Number → Dedup Audit ---

// checkDuplicateSerials iterates active tenants and runs the duplicate-
// serial scan per tenant so WOs are always tagged with the right
// tenant_id. One tenant's failure must not abort the batch.
func (w *WorkflowSubscriber) checkDuplicateSerials(ctx context.Context) {
	tenants, err := w.queries.ListActiveTenants(ctx)
	if err != nil {
		zap.L().Warn("dedup checker: list active tenants failed", zap.Error(err))
		return
	}
	for _, t := range tenants {
		if err := w.checkDuplicateSerialsForTenant(ctx, t.ID); err != nil {
			zap.L().Warn("dedup checker: tenant scan failed",
				zap.String("tenant_id", t.ID.String()),
				zap.Error(err))
			// Continue to next tenant — one failure must not abort the batch.
		}
	}
}

// checkDuplicateSerialsForTenant finds serial-number duplicates within a
// single tenant. The GROUP BY drops `tenant_id` since the outer WHERE
// already pins a single tenant.
//
// Dedup is enforced via the `work_order_dedup` table (Phase 2.15): the
// prior `description LIKE '%serial%'` probe was replaced with an
// indexed (tenant_id, dedup_kind, dedup_key) lookup, and the WO insert
// + dedup insert now share one transaction. The HAVING filter drops
// serials already recorded in work_order_dedup so we don't even scan
// them.
func (w *WorkflowSubscriber) checkDuplicateSerialsForTenant(ctx context.Context, tenantID uuid.UUID) error {
	rows, err := w.pool.Query(ctx,
		`SELECT serial_number, array_agg(id) AS asset_ids, array_agg(asset_tag) AS asset_tags
		 FROM assets
		 WHERE tenant_id = $1
		   AND serial_number IS NOT NULL
		   AND serial_number != ''
		   AND deleted_at IS NULL
		 GROUP BY serial_number
		 HAVING count(*) > 1
		    AND NOT EXISTS (
		      SELECT 1 FROM work_order_dedup wod
		      WHERE wod.tenant_id  = $1
		        AND wod.dedup_kind = 'duplicate_serial'
		        AND wod.dedup_key  = serial_number
		    )`, tenantID)
	if err != nil {
		return fmt.Errorf("query assets: %w", err)
	}

	type dupSerialCandidate struct {
		serial    string
		assetIDs  []uuid.UUID
		assetTags []string
	}
	var candidates []dupSerialCandidate
	for rows.Next() {
		var c dupSerialCandidate
		if err := rows.Scan(&c.serial, &c.assetIDs, &c.assetTags); err != nil {
			zap.L().Warn("dedup checker: row scan failed",
				zap.String("tenant_id", tenantID.String()), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDuplicateSerialCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		candidates = append(candidates, c)
	}
	iterErr := rows.Err()
	// Close the read cursor BEFORE starting per-candidate write txs so
	// pgxpool can reuse the same connection — see the same pattern in
	// checkShadowITForTenant above.
	rows.Close()
	if iterErr != nil {
		return fmt.Errorf("iterate assets: %w", iterErr)
	}

	for _, c := range candidates {
		if err := w.createDuplicateSerialWorkOrder(ctx, tenantID, c.serial, c.assetIDs, c.assetTags); err != nil {
			zap.L().Warn("dedup checker: WO creation skipped",
				zap.String("tenant_id", tenantID.String()),
				zap.String("serial", c.serial), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDuplicateSerialCheck, reasonWOCreateFailed).Inc()
			continue
		}
		zap.L().Info("dedup checker: created audit WO",
			zap.String("tenant_id", tenantID.String()),
			zap.String("serial", c.serial),
			zap.Int("count", len(c.assetIDs)))
	}
	return nil
}

// createDuplicateSerialWorkOrder atomically creates a dedup-audit WO
// and its matching work_order_dedup row. See createShadowITWorkOrder
// for the race-safety contract (identical shape, different dedup kind).
func (w *WorkflowSubscriber) createDuplicateSerialWorkOrder(ctx context.Context, tenantID uuid.UUID, serial string, assetIDs []uuid.UUID, assetTags []string) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceDuplicateSerialCheck)
	if !ok {
		return fmt.Errorf("resolve system user: skipped")
	}
	tagList := strings.Join(assetTags, ", ")
	order, err := w.maintenanceSvc.CreateTx(ctx, tx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Dedup Audit: Serial %s duplicated", serial),
		Type:        "dedup_audit",
		Priority:    "high",
		Description: fmt.Sprintf("Serial number '%s' appears on %d assets: [%s]. This violates data uniqueness. Action: identify the correct asset, merge or delete duplicates, investigate how the duplication occurred.", serial, len(assetIDs), tagList),
	})
	if err != nil {
		return fmt.Errorf("create dedup WO: %w", err)
	}

	n, err := w.queries.WithTx(tx).InsertWorkOrderDedup(ctx, dbgen.InsertWorkOrderDedupParams{
		TenantID:    tenantID,
		WorkOrderID: order.ID,
		DedupKind:   dedupKindDuplicateSerial,
		DedupKey:    serial,
	})
	if err != nil {
		return fmt.Errorf("insert dedup: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("dedup race: duplicate_serial key %s already exists", serial)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	w.maintenanceSvc.BumpSyncVersionAfterCreate(ctx, order.ID)
	return nil
}

// --- Auto Work Order 8: Missing Location → Location Completion ---

// checkMissingLocation iterates active tenants and runs the missing-
// location scan per tenant. A failure in one tenant is logged and does
// not abort the batch.
//
// Bonus fix over the old cross-tenant version: the "bulk" fallback WO
// is now correctly attributed to the tenant whose assets triggered it
// — previously firstTenantID was whichever tenant happened to be
// scanned first, which was a latent cross-tenant leak.
func (w *WorkflowSubscriber) checkMissingLocation(ctx context.Context) {
	tenants, err := w.queries.ListActiveTenants(ctx)
	if err != nil {
		zap.L().Warn("location completion checker: list active tenants failed", zap.Error(err))
		return
	}
	for _, t := range tenants {
		if err := w.checkMissingLocationForTenant(ctx, t.ID); err != nil {
			zap.L().Warn("location completion checker: tenant scan failed",
				zap.String("tenant_id", t.ID.String()),
				zap.Error(err))
			// Continue to next tenant — one failure must not abort the batch.
		}
	}
}

// checkMissingLocationForTenant runs the missing-location scan for a
// single tenant. Both the assets source and the work_orders dedup
// predicate are scoped to tenantID, and the bulk fallback WO is created
// under tenantID so its attribution is unambiguous.
func (w *WorkflowSubscriber) checkMissingLocationForTenant(ctx context.Context, tenantID uuid.UUID) error {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.asset_tag, a.name
		 FROM assets a
		 WHERE a.tenant_id = $1
		   AND a.location_id IS NULL
		   AND a.rack_id IS NULL
		   AND a.status NOT IN ('disposed', 'decommission', 'procurement')
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.tenant_id = $1
		     AND wo.asset_id = a.id
		     AND wo.type = 'location_completion'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`, tenantID)
	if err != nil {
		return fmt.Errorf("query assets: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var assetID uuid.UUID
		var assetTag, name string
		if err := rows.Scan(&assetID, &assetTag, &name); err != nil {
			zap.L().Warn("location completion checker: row scan failed",
				zap.String("tenant_id", tenantID.String()), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceMissingLocationCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		count++

		if count <= 10 {
			sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceMissingLocationCheck)
			if !ok {
				continue
			}
			_, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
				Title:       fmt.Sprintf("Location Missing: %s (%s)", name, assetTag),
				Type:        "location_completion",
				Priority:    "low",
				AssetID:     &assetID,
				Description: fmt.Sprintf("Asset '%s' has no location or rack assigned. An asset without a known location cannot be physically managed. Please assign location and rack.", name),
			})
			if err != nil {
				zap.L().Warn("location completion checker: WO creation skipped",
					zap.String("tenant_id", tenantID.String()),
					zap.String("asset", assetTag), zap.Error(err))
				telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceMissingLocationCheck, reasonWOCreateFailed).Inc()
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate assets: %w", err)
	}

	if count > 10 {
		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceMissingLocationCheck)
		if !ok {
			return nil
		}
		_, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Bulk Location Completion: %d assets missing location", count),
			Type:        "location_completion",
			Priority:    "medium",
			Description: fmt.Sprintf("%d assets have no location or rack assigned. Run a location audit to assign physical positions.", count),
		})
		if err != nil {
			zap.L().Warn("location completion checker: bulk WO creation skipped",
				zap.String("tenant_id", tenantID.String()), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceMissingLocationCheck, reasonWOCreateFailed).Inc()
		}
	}

	if count > 0 {
		zap.L().Info("location completion checker: found assets without location",
			zap.String("tenant_id", tenantID.String()),
			zap.Int("count", count))
	}
	return nil
}

// --- Auto Work Order 9: Firmware Outdated → Firmware Upgrade ---

// firmwareAssetRow is the per-asset projection the firmware scan loads
// from the database so the "latest firmware per bmc_type" aggregation
// can be computed in Go using semver ordering instead of SQL MAX(),
// which is lexicographic and would mis-order e.g. "1.10.0" < "1.9.0".
type firmwareAssetRow struct {
	assetID   uuid.UUID
	tenantID  uuid.UUID
	assetTag  string
	name      string
	bmcType   string
	firmware  string
	hasOpenWO bool
}

func (w *WorkflowSubscriber) checkFirmwareOutdated(ctx context.Context) {
	// NOTE: we deliberately avoid the old `WITH latest AS (SELECT MAX(bmc_firmware) …)`
	// pattern because SQL MAX() on text is lexicographic: "1.9.0" beats
	// "1.10.0". Instead, we pull the raw rows and compute the max per
	// bmc_type in Go via maxFirmwareVersion (semver with lex fallback).
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.bmc_type, a.bmc_firmware,
		        EXISTS (
		          SELECT 1 FROM work_orders wo
		          WHERE wo.asset_id = a.id
		            AND wo.type = 'firmware_upgrade'
		            AND wo.status NOT IN ('completed','verified','rejected')
		            AND wo.deleted_at IS NULL
		        )
		 FROM assets a
		 WHERE a.bmc_type IS NOT NULL
		   AND a.bmc_firmware IS NOT NULL
		   AND a.deleted_at IS NULL
		   AND a.status NOT IN ('disposed', 'decommission')`)
	if err != nil {
		zap.L().Warn("firmware checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	var assets []firmwareAssetRow
	versionsByType := make(map[string][]string)
	for rows.Next() {
		var r firmwareAssetRow
		if scanErr := rows.Scan(&r.assetID, &r.tenantID, &r.assetTag, &r.name, &r.bmcType, &r.firmware, &r.hasOpenWO); scanErr != nil {
			zap.L().Warn("firmware checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		assets = append(assets, r)
		versionsByType[r.bmcType] = append(versionsByType[r.bmcType], r.firmware)
	}
	if err := rows.Err(); err != nil {
		zap.L().Warn("firmware checker: row iteration failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, telemetry.ReasonRowsIterFailed).Inc()
		return
	}

	// Compute the "latest" firmware per bmc_type using semver ordering.
	latestByType := make(map[string]string, len(versionsByType))
	for t, vs := range versionsByType {
		latestByType[t] = maxFirmwareVersion(vs)
	}

	for _, a := range assets {
		if a.hasOpenWO {
			continue
		}
		latestFW := latestByType[a.bmcType]
		if latestFW == "" {
			continue
		}
		// Only create a WO when the asset's current firmware is STRICTLY
		// behind the latest known firmware for its BMC type under semver
		// comparison. Equal or ahead is a no-op.
		if compareFirmwareVersion(a.firmware, latestFW) >= 0 {
			continue
		}

		sysUID, ok := w.resolveSystemUser(ctx, a.tenantID, sourceFirmwareCheck)
		if !ok {
			continue
		}
		_, err := w.maintenanceSvc.Create(ctx, a.tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Firmware Upgrade: %s (%s)", a.name, a.assetTag),
			Type:        "firmware_upgrade",
			Priority:    "low",
			AssetID:     &a.assetID,
			Description: fmt.Sprintf("Asset '%s' BMC firmware (%s: %s) is behind the latest detected version (%s). Schedule firmware upgrade to maintain security compliance.", a.name, a.bmcType, a.firmware, latestFW),
		})
		if err != nil {
			zap.L().Warn("firmware checker: WO creation skipped", zap.String("asset", a.assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, reasonWOCreateFailed).Inc()
			continue
		}
		zap.L().Info("firmware checker: created upgrade WO", zap.String("asset", a.assetTag))
	}
}

// --- Auto Work Order 10: BMC Default Password → Security Hardening (event-driven) ---

// createBMCSecurityWO creates a critical security hardening work order when a BMC default
// password is detected. Called from the onBMCDefaultPassword event handler.
func (w *WorkflowSubscriber) createBMCSecurityWO(ctx context.Context, tenantID, assetID uuid.UUID, assetTag, name, bmcType string) {
	var existingCount int
	if scanErr := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM work_orders
		 WHERE asset_id = $1 AND type = 'security_hardening'
		 AND status NOT IN ('completed','verified','rejected')
		 AND deleted_at IS NULL`,
		assetID).Scan(&existingCount); scanErr != nil {
		// Prefer skipping over double-WOs — a broken dedup probe
		// must not flood the critical-security queue with replays.
		zap.L().Warn("security: BMC dedup probe failed", zap.String("asset", assetTag), zap.Error(scanErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceBMCSecurityCheck, telemetry.ReasonRowScanFailed).Inc()
		return
	}
	if existingCount > 0 {
		return
	}

	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceBMCSecurityCheck)
	if !ok {
		return
	}
	order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Security: Default BMC Password — %s (%s)", name, assetTag),
		Type:        "security_hardening",
		Priority:    "critical",
		AssetID:     &assetID,
		Description: fmt.Sprintf("CRITICAL: Asset '%s' BMC (%s) is accessible with default credentials. This is a severe security risk. Immediately change the BMC password and verify access controls.", name, bmcType),
	})
	if err != nil {
		zap.L().Warn("security: BMC hardening WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceBMCSecurityCheck, reasonWOCreateFailed).Inc()
		return
	}

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.warnNotify(ctx, sourceBMCSecurityCheck, tenantID, adminID, "security_hardening",
			fmt.Sprintf("CRITICAL: Default BMC password on %s", assetTag),
			fmt.Sprintf("Asset '%s' BMC uses default credentials. Immediate action required.", name),
			"work_order", order.ID)
	}
	zap.L().Warn("security: created BMC hardening WO", zap.String("asset", assetTag))
}

func (w *WorkflowSubscriber) onBMCDefaultPassword(ctx context.Context, event eventbus.Event) error {
	var payload struct {
		AssetID  string `json:"asset_id"`
		AssetTag string `json:"asset_tag"`
		Name     string `json:"name"`
		BMCType  string `json:"bmc_type"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		zap.L().Warn("workflow: failed to parse bmc_default_password payload", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceBMCDefaultEventParse, telemetry.ReasonJSONUnmarshal).Inc()
		return nil
	}

	tenantID, _ := uuid.Parse(event.TenantID)
	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil || tenantID == uuid.Nil {
		return nil
	}

	w.createBMCSecurityWO(ctx, tenantID, assetID, payload.AssetTag, payload.Name, payload.BMCType)
	return nil
}
