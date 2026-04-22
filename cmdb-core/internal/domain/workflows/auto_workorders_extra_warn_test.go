package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestCheckOverLifespan_WarnsAndCountsOnQueryFailure is the regression
// guard for the over-lifespan scanner's outer-query failure path —
// the symmetric partner of the warranty/EOL/firmware tests. A silent
// bail on query error would mask a corrupted assets table.
func TestCheckOverLifespan_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceLifespanCheck, telemetry.ReasonDBQueryFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkOverLifespan(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceLifespanCheck, telemetry.ReasonDBQueryFailed,
		),
	)
	if after <= before {
		t.Fatalf("errors_suppressed_total not incremented (before=%v after=%v)", before, after)
	}

	found := false
	for _, e := range logs.All() {
		if e.Level == zap.WarnLevel {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a Warn log, got none; entries=%v", logs.All())
	}
}

// TestCheckMissingAssets_WarnsAndCountsOnQueryFailure: parallel guard
// for the asset-verification scanner. A silent bail on a broken
// assets/discovered_assets join would hide the "unseen asset" alert
// entirely.
func TestCheckMissingAssets_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceAssetVerification, telemetry.ReasonDBQueryFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkMissingAssets(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceAssetVerification, telemetry.ReasonDBQueryFailed,
		),
	)
	if after <= before {
		t.Fatalf("errors_suppressed_total not incremented (before=%v after=%v)", before, after)
	}

	found := false
	for _, e := range logs.All() {
		if e.Level == zap.WarnLevel {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a Warn log, got none; entries=%v", logs.All())
	}
}

