package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// subscriberWithFailingPool builds a WorkflowSubscriber whose pool and queries
// both point at an unreachable address. Every pool call fails fast with a
// connection error, exercising the warn+counter paths.
func subscriberWithFailingPool(t *testing.T) *WorkflowSubscriber {
	t.Helper()
	pool := newFailingPool(t)
	t.Cleanup(pool.Close)
	queries := dbgen.New(pool)
	return &WorkflowSubscriber{pool: pool, queries: queries}
}

func captureWarnLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(prev)
	return logs
}

func hasWarn(logs *observer.ObservedLogs) bool {
	for _, e := range logs.All() {
		if e.Level == zap.WarnLevel {
			return true
		}
	}
	return false
}

func TestRunDailyChecks_AllSubchecksFireOnFailingPool(t *testing.T) {
	logs := captureWarnLogs(t)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.runDailyChecks(ctx)
	if !hasWarn(logs) {
		t.Error("runDailyChecks: expected at least one Warn log with a failing pool, got none")
	}
}

func TestRunWeeklyChecks_AllSubchecksFireOnFailingPool(t *testing.T) {
	logs := captureWarnLogs(t)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.runWeeklyChecks(ctx)
	if !hasWarn(logs) {
		t.Error("runWeeklyChecks: expected at least one Warn log with a failing pool, got none")
	}
}

func TestCheckShadowIT_WarnsOnListActiveTenantsFailure(t *testing.T) {
	logs := captureWarnLogs(t)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkShadowIT(ctx)
	if !hasWarn(logs) {
		t.Error("checkShadowIT: expected Warn on ListActiveTenants failure, got none")
	}
}

func TestCheckShadowITForTenant_ErrorOnQueryFailure(t *testing.T) {
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := w.checkShadowITForTenant(ctx, uuid.New()); err == nil {
		t.Error("checkShadowITForTenant: expected error from failing pool, got nil")
	}
}

func TestCheckDuplicateSerials_WarnsOnListActiveTenantsFailure(t *testing.T) {
	logs := captureWarnLogs(t)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkDuplicateSerials(ctx)
	if !hasWarn(logs) {
		t.Error("checkDuplicateSerials: expected Warn on ListActiveTenants failure, got none")
	}
}

func TestCheckDuplicateSerialsForTenant_ErrorOnQueryFailure(t *testing.T) {
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := w.checkDuplicateSerialsForTenant(ctx, uuid.New()); err == nil {
		t.Error("checkDuplicateSerialsForTenant: expected error from failing pool, got nil")
	}
}

func TestCheckMissingLocation_WarnsOnListActiveTenantsFailure(t *testing.T) {
	logs := captureWarnLogs(t)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkMissingLocation(ctx)
	if !hasWarn(logs) {
		t.Error("checkMissingLocation: expected Warn on ListActiveTenants failure, got none")
	}
}

func TestCheckMissingLocationForTenant_ErrorOnQueryFailure(t *testing.T) {
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := w.checkMissingLocationForTenant(ctx, uuid.New()); err == nil {
		t.Error("checkMissingLocationForTenant: expected error from failing pool, got nil")
	}
}

func TestCheckLowQualityPersistent_WarnsOnListActiveTenantsFailure(t *testing.T) {
	logs := captureWarnLogs(t)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkLowQualityPersistent(ctx)
	if !hasWarn(logs) {
		t.Error("checkLowQualityPersistent: expected Warn on ListActiveTenants failure, got none")
	}
}

func TestCheckLowQualityPersistentForTenant_ErrorOnQueryFailure(t *testing.T) {
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := w.checkLowQualityPersistentForTenant(ctx, uuid.New()); err == nil {
		t.Error("checkLowQualityPersistentForTenant: expected error from failing pool, got nil")
	}
}

func TestCheckUnreviewedDiscoveries_WarnsAndCountsOnQueryFailure(t *testing.T) {
	logs := captureWarnLogs(t)
	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDiscoveryUnreviewed, telemetry.ReasonDBExecFailed),
	)
	w := subscriberWithFailingPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.checkUnreviewedDiscoveries(ctx)
	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceDiscoveryUnreviewed, telemetry.ReasonDBExecFailed),
	)
	if after <= before {
		t.Errorf("errors_suppressed_total not incremented (before=%v after=%v)", before, after)
	}
	if !hasWarn(logs) {
		t.Error("checkUnreviewedDiscoveries: expected Warn on query failure, got none")
	}
}

type stubTracker struct {
	registered []string
	recorded   []string
}

func (s *stubTracker) Register(name string, _ time.Duration) {
	s.registered = append(s.registered, name)
}
func (s *stubTracker) Record(name string) { s.recorded = append(s.recorded, name) }

func TestWithSchedHealth_WiresTrackerAndChains(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	tracker := &stubTracker{}
	got := w.WithSchedHealth(tracker)
	if got != w {
		t.Error("WithSchedHealth must return receiver for chaining")
	}
	if w.tracker != tracker {
		t.Error("tracker field not set by WithSchedHealth")
	}
}

func TestRegisterAndRecordTick_DispatchesToTracker(t *testing.T) {
	t.Parallel()
	tracker := &stubTracker{}
	w := &WorkflowSubscriber{tracker: tracker}
	w.registerScheduler("sched-a", time.Minute)
	w.recordTick("sched-a")
	w.recordTick("sched-b")
	if len(tracker.registered) != 1 || tracker.registered[0] != "sched-a" {
		t.Errorf("registered = %v, want [sched-a]", tracker.registered)
	}
	if len(tracker.recorded) != 2 || tracker.recorded[0] != "sched-a" || tracker.recorded[1] != "sched-b" {
		t.Errorf("recorded = %v, want [sched-a sched-b]", tracker.recorded)
	}
}

func TestRegisterAndRecordTick_NilTrackerIsNoOp(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	w.registerScheduler("x", time.Minute)
	w.recordTick("x")
}

func TestDedupKindDiscoveryUnreviewed_IsDistinct(t *testing.T) {
	t.Parallel()
	all := []string{
		dedupKindShadowIT, dedupKindDuplicateSerial,
		dedupKindLowQualityPersistent, dedupKindDiscoveryUnreviewed,
	}
	seen := make(map[string]struct{}, len(all))
	for _, k := range all {
		if k == "" {
			t.Fatal("empty dedup kind — would silently disable dedup")
		}
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate dedup kind %q", k)
		}
		seen[k] = struct{}{}
	}
}

func TestDiscoveryReviewConstants_InRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		v    int
		min  int
		max  int
	}{
		{"discoveryReviewThresholdHours", discoveryReviewThresholdHours, 1, 72},
		{"discoveryReviewBatchLimit", discoveryReviewBatchLimit, 1, 10000},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.v < tc.min || tc.v > tc.max {
				t.Errorf("%s = %d, want [%d, %d]", tc.name, tc.v, tc.min, tc.max)
			}
		})
	}
}

func TestLowQualityConstants_InRange(t *testing.T) {
	t.Parallel()
	if lowQualityThreshold <= 0 || lowQualityThreshold >= 100 {
		t.Errorf("lowQualityThreshold = %.1f, must be in (0, 100)", lowQualityThreshold)
	}
	if lowQualityLookbackDays <= 0 || lowQualityLookbackDays > 90 {
		t.Errorf("lowQualityLookbackDays = %d, must be in (1, 90]", lowQualityLookbackDays)
	}
}

func TestSourceLabels_Discovery_LowQuality_Distinct(t *testing.T) {
	t.Parallel()
	if sourceDiscoveryUnreviewed == "" || sourceLowQualityCheck == "" {
		t.Error("one of the source labels is empty")
	}
	if sourceDiscoveryUnreviewed == sourceLowQualityCheck {
		t.Errorf("sourceDiscoveryUnreviewed == sourceLowQualityCheck (%q)", sourceDiscoveryUnreviewed)
	}
}

// TestStarters_LaunchAndShutdown_Promptly exercises the long-running goroutine
// launchers via context cancellation. Each launcher schedules a ticker and a
// background loop; cancelling ctx must let the goroutine return so the
// process can shut down cleanly. Without these tests StartWarrantyChecker /
// StartDiscoveryReviewChecker / StartAuditPartitionSampler stayed at 0%.
func TestStarters_LaunchAndShutdown_Promptly(t *testing.T) {
	cases := []struct {
		name string
		fn   func(*WorkflowSubscriber, context.Context)
	}{
		{"StartWarrantyChecker", func(w *WorkflowSubscriber, ctx context.Context) { w.StartWarrantyChecker(ctx) }},
		{"StartDiscoveryReviewChecker", func(w *WorkflowSubscriber, ctx context.Context) { w.StartDiscoveryReviewChecker(ctx) }},
		{"StartAuditPartitionSampler", func(w *WorkflowSubscriber, ctx context.Context) { w.StartAuditPartitionSampler(ctx) }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := subscriberWithFailingPool(t)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				tc.fn(w, ctx)
				close(done)
			}()
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s did not return within 2s after ctx cancel", tc.name)
			}
		})
	}
}
