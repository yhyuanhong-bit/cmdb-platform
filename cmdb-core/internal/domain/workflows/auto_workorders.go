package workflows

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"go.uber.org/zap"
)

// Dedup kinds written to the work_order_dedup table by the auto-WO
// scans (Phase 2.15). Keep these in sync with the backfill in
// 000049_work_order_dedup.up.sql — a typo here silently defeats dedup.
const (
	dedupKindShadowIT              = "shadow_it"
	dedupKindDuplicateSerial       = "duplicate_serial"
	dedupKindDiscoveryUnreviewed   = "discovery_unreviewed"
)

// Source labels for telemetry.ErrorsSuppressedTotal emitted from the
// auto-work-order scans. Each scan gets its own label so a broken
// fixture (e.g. a renamed column) lights up the exact source instead
// of being aggregated into a single unactionable bucket.
const (
	sourceWarrantyCheck        = "workflows.auto_wo.warranty"
	sourceAssetVerification    = "workflows.auto_wo.asset_verification"
	sourceDataCorrection       = "workflows.auto_wo.data_correction"
	sourceEOLCheck             = "workflows.auto_wo.eol"
	sourceLifespanCheck        = "workflows.auto_wo.lifespan"
	sourceShadowITCheck        = "workflows.auto_wo.shadow_it"
	sourceDuplicateSerialCheck = "workflows.auto_wo.duplicate_serial"
	sourceMissingLocationCheck = "workflows.auto_wo.missing_location"
	sourceFirmwareCheck        = "workflows.auto_wo.firmware"
	sourceBMCSecurityCheck     = "workflows.auto_wo.bmc_security"
	sourceScanDiffEventHandler = "workflows.auto_wo.scan_diff_event"
	sourceBMCDefaultEventParse = "workflows.auto_wo.bmc_default_event"
	sourceLowQualityCheck      = "workflows.auto_wo.low_quality"
	sourceDiscoveryUnreviewed  = "workflows.auto_wo.discovery_unreviewed"
)

// Reason used when maintenanceSvc.Create fails in an auto-WO scan.
// These failures were previously Debug-logged, which made a broken
// maintenance service invisible in production dashboards. Promote to
// Warn + counter so a spike shows up on the errors_suppressed panel.
const reasonWOCreateFailed = telemetry.ReasonWOCreationFailed

// StartWarrantyChecker runs daily and drives every data-governance
// scan that should fire once per day (warranty expiry, EOL, lifespan,
// firmware). The individual scans live in auto_workorders_warranty.go
// and auto_workorders_security.go.
func (w *WorkflowSubscriber) StartWarrantyChecker(ctx context.Context) {
	const interval = 24 * time.Hour
	w.registerScheduler(SchedNameWarrantyChecker, interval)
	ticker := time.NewTicker(interval)
	go func() {
		func() {
			w.recordTick(SchedNameWarrantyChecker)
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
				w.recordTick(SchedNameWarrantyChecker)
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
	w.checkLowQualityPersistent(ctx)
}

// StartAssetVerificationChecker runs weekly and drives every scan
// that should fire once per week (missing assets, shadow IT,
// duplicate serials, missing location). The individual scans live
// in auto_workorders_governance.go.
func (w *WorkflowSubscriber) StartAssetVerificationChecker(ctx context.Context) {
	const interval = 7 * 24 * time.Hour
	w.registerScheduler(SchedNameAssetVerification, interval)
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.recordTick(SchedNameAssetVerification)
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
