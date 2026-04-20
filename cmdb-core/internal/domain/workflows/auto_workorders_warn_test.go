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

// TestCheckWarrantyExpiry_WarnsAndCountsOnQueryFailure proves the
// outer SELECT failure path in checkWarrantyExpiry now emits a Warn
// log line plus an errors_suppressed_total increment. Before the
// Phase 3.1 fix, the scanner bailed with a bare return so a broken
// assets table or warranty_end column would have been invisible.
func TestCheckWarrantyExpiry_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceWarrantyCheck, telemetry.ReasonDBQueryFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkWarrantyExpiry(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceWarrantyCheck, telemetry.ReasonDBQueryFailed,
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

// TestCheckEOLReached_WarnsAndCountsOnQueryFailure is the parallel
// regression guard for the EOL-reached scanner. A silent bail on
// outer query failure would again mask a corrupted assets table.
func TestCheckEOLReached_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceEOLCheck, telemetry.ReasonDBQueryFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkEOLReached(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceEOLCheck, telemetry.ReasonDBQueryFailed,
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

// TestCheckFirmwareOutdated_WarnsAndCountsOnQueryFailure proves the
// firmware-scanner's outer-query failure path emits a Warn +
// counter. The pre-fix code returned silently.
func TestCheckFirmwareOutdated_WarnsAndCountsOnQueryFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceFirmwareCheck, telemetry.ReasonDBQueryFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkFirmwareOutdated(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceFirmwareCheck, telemetry.ReasonDBQueryFailed,
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
