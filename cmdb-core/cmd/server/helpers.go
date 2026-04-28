// helpers.go — small process-lifecycle utilities extracted from main.go
// during the Phase 2 God-file split (2026-04-28). Pure logic, no side
// effects beyond logging; safe to test in isolation.
package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/domain/predictive"
	"github.com/cmdb-platform/cmdb-core/internal/platform/schedhealth"
	"go.uber.org/zap"
)

// envIntOr reads a positive integer from the named env var, returning
// fallback if the var is unset, non-numeric, or non-positive. Wrong
// values do not brick startup — they fall back with a warning log.
func envIntOr(envKey string, fallback int) int {
	raw := os.Getenv(envKey)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		zap.L().Warn("invalid env int, using fallback",
			zap.String("env", envKey),
			zap.String("raw", raw),
			zap.Int("fallback", fallback))
		return fallback
	}
	return n
}

// runPredictiveScheduler runs the Wave 7.1 hardware-refresh rule engine
// on a 1h ticker. Idempotent on each pass; per-tenant errors don't
// abort the tick.
func runPredictiveScheduler(ctx context.Context, svc *predictive.Service, tracker *schedhealth.Tracker) {
	const interval = time.Hour
	const trackerName = "predictive_refresh"
	cfg := predictive.DefaultRuleConfig()
	if tracker != nil {
		tracker.Register(trackerName, interval)
	}
	zap.L().Info("predictive refresh scheduler started", zap.Duration("interval", interval))

	tick := func() {
		if tracker != nil {
			tracker.Record(trackerName)
		}
		res := svc.RunScanTick(ctx, cfg)
		zap.L().Info("predictive refresh tick",
			zap.Int("tenants_scanned", res.TenantsScanned),
			zap.Int("assets_scanned", res.AssetsScanned),
			zap.Int("rows_upserted", res.RowsUpserted),
			zap.Int("errors", len(res.Errors)),
		)
		for _, err := range res.Errors {
			zap.L().Warn("predictive refresh tick error", zap.Error(err))
		}
	}

	tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			zap.L().Info("predictive refresh scheduler stopped")
			return
		case <-t.C:
			tick()
		}
	}
}

// lateBoundRecorder forwards Record() to a target tracker that may not
// exist yet at the moment the evaluator is constructed. Wave 9.1 needs
// the alert evaluator (built early in main) to share a tracker that's
// only created later in main, after the rest of the schedulers are
// known. The forwarder keeps the evaluator constructor signature stable
// while letting startup wire-up happen in any order.
type lateBoundRecorder struct {
	target *schedhealth.Tracker
}

func (r *lateBoundRecorder) Record(name string) {
	if r.target != nil {
		r.target.Record(name)
	}
}
