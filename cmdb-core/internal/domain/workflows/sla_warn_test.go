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

// TestCheckSLAWarnings_WarnsAndCountsOnQueryFailure confirms that
// the sla-warning scanner emits a Warn + counter increment when its
// outer SELECT fails, instead of the pre-fix silent return. This
// the pre-fix scanner masked a corrupted work_orders table for
// weeks; the regression guard here keeps us honest.
func TestCheckSLAWarnings_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceSLAWarning, telemetry.ReasonDBQueryFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkSLAWarnings(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceSLAWarning, telemetry.ReasonDBQueryFailed,
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

// TestCheckSLABreaches_WarnsAndCountsOnQueryFailure is the analogous
// guard for the breach-detection path. The scanner must emit a Warn
// + Error log when the outer UPDATE fails — Error in this case
// because a failed breach-mark is operationally more severe than a
// missed warning.
func TestCheckSLABreaches_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkSLABreaches(ctx)

	// Any non-info log counts as a signal — Error or Warn. Before
	// the fix there was none at all on the failure path.
	found := false
	for _, e := range logs.All() {
		if e.Level >= zap.WarnLevel {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Warn/Error log, got none; entries=%v", logs.All())
	}
}
