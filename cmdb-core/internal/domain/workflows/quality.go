package workflows

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"go.uber.org/zap"
)

// Scheduled per-tenant quality scanner (Phase 2.11 — REMEDIATION-ROADMAP).
//
// Until this loop was added, the only way to run the quality engine over
// every asset in a tenant was the on-demand POST /quality/scan API. That
// meant tenants whose operators never hit the endpoint were never scored,
// so the `quality_scores` table (and the dashboard that reads from it)
// went stale the moment anyone stopped pressing the button.
//
// The scheduler adopts the Phase 1.4 per-tenant loop pattern used by
// checkShadowIT / checkDuplicateSerials / checkMissingLocation: it
// enumerates `ListActiveTenants` and fans out to one `ScanTenant` call
// per tenant so a single tenant's failure cannot starve the others of
// scan coverage. Every per-tenant outcome increments
// `quality_scanner_runs_total{outcome}` so a dead loop is observable.
//
// Cadence: first tick at startup + 30s (so fresh deploys produce an
// observable scan quickly), then every 24h. Per-tenant cron
// customization is optional and is explicitly deferred to a later
// roadmap phase; a single daily cadence keeps this commit small and the
// metric interpretation simple.

// qualityTenantLister is the tiny subset of *dbgen.Queries used by the
// scheduled scanner. Declared at the use site (Phase 1.4 pattern + Go
// idiom: accept interfaces, return structs) so unit tests can swap in a
// fake without touching the real DB.
type qualityTenantLister interface {
	ListActiveTenants(ctx context.Context) ([]dbgen.ListActiveTenantsRow, error)
}

const (
	// qualityScanInitialDelay is how long the loop waits after startup
	// before running the first scan. Short enough to be observable on
	// deploys (so operators don't have to wait a full day to see the
	// first scan), long enough to not fight migrations, seed jobs, or
	// warm-up queries for the request path.
	qualityScanInitialDelay = 30 * time.Second

	// qualityScanInterval is the steady-state cadence between full
	// tenant sweeps. Daily is the right default: the underlying
	// evaluateAsset only moves scores when asset data changes, and
	// running more often would pile quality_scores rows on top of an
	// unchanged snapshot.
	qualityScanInterval = 24 * time.Hour
)

// StartQualityScanner spawns the daily quality-scan goroutine and
// returns immediately. The goroutine exits when ctx is cancelled so a
// single SIGTERM stops every workflow worker together (see Phase 2.7
// ctx-wiring contract).
//
// If no quality scanner was injected via WithQualityScanner this becomes
// a no-op; production deployments always inject one in main.go, but
// tests and edge nodes that disable quality scanning can skip the
// dependency without crashing.
func (w *WorkflowSubscriber) StartQualityScanner(ctx context.Context) {
	if w.qualitySvc == nil {
		zap.L().Info("quality scanner not started: no qualityScanner injected")
		return
	}
	go w.qualityScanLoop(ctx)
	zap.L().Info("quality scanner started",
		zap.Duration("initial_delay", qualityScanInitialDelay),
		zap.Duration("interval", qualityScanInterval))
}

// qualityScanLoop drives the scheduled scan. The timer handles the
// initial-delay fire so deploys observe a scan within ~30s; after that
// the ticker takes over for the daily cadence. Both channels select
// against ctx.Done() so cancellation is respected regardless of which
// phase we're in.
func (w *WorkflowSubscriber) qualityScanLoop(ctx context.Context) {
	ticker := time.NewTicker(qualityScanInterval)
	defer ticker.Stop()

	initial := time.NewTimer(qualityScanInitialDelay)
	defer initial.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
			tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.quality_scan")
			w.runQualityScan(tickCtx)
			end()
		case <-ticker.C:
			tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.quality_scan")
			w.runQualityScan(tickCtx)
			end()
		}
	}
}

// runQualityScan lists active tenants and scans each one in turn. A
// tenant failure is logged and counted under outcome=error, but the
// loop continues so one bad tenant cannot starve the rest of the fleet
// of their scan.
//
// The `ListActiveTenants` call itself is the only cross-tenant read
// (Phase 1.4 convention): everything downstream of it is strictly
// tenant-scoped through ScanTenant. If ctx is cancelled mid-iteration
// we stop early — the next tick will catch up on any skipped tenants.
func (w *WorkflowSubscriber) runQualityScan(ctx context.Context) {
	runQualityScanWith(ctx, w.tenantLister(), w.qualitySvc)
}

// tenantLister returns the qualityTenantLister used by the scanner.
// Prefers the test-injected override (qualityTenantListerOverride) when
// set; otherwise falls back to the real *dbgen.Queries. Kept as a
// method so the override is always discovered through the receiver —
// tests never see a stale snapshot of the lister field.
func (w *WorkflowSubscriber) tenantLister() qualityTenantLister {
	if w.qualityTenantListerOverride != nil {
		return w.qualityTenantListerOverride
	}
	return w.queries
}

// runQualityScanWith is the pure-loop core extracted so unit tests can
// drive it without constructing a WorkflowSubscriber or standing up a
// DB. Logs + metrics identical to the method version.
func runQualityScanWith(ctx context.Context, lister qualityTenantLister, scanner qualityScanner) {
	tenants, err := lister.ListActiveTenants(ctx)
	if err != nil {
		zap.L().Error("quality scan: list tenants failed", zap.Error(err))
		return
	}

	for _, t := range tenants {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := scanner.ScanTenant(ctx, t.ID); err != nil {
			telemetry.QualityScannerRunsTotal.WithLabelValues("error").Inc()
			zap.L().Warn("quality scan failed",
				zap.String("tenant_id", t.ID.String()),
				zap.Error(err))
			continue
		}
		telemetry.QualityScannerRunsTotal.WithLabelValues("ok").Inc()
	}
}
