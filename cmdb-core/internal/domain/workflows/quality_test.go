package workflows

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fakeTenantLister returns a canned ListActiveTenants result without
// needing a real *dbgen.Queries. Used by every scheduler unit test so
// we can exercise the fan-out loop in isolation from the DB.
type fakeTenantLister struct {
	rows []dbgen.ListActiveTenantsRow
	err  error
}

func (f *fakeTenantLister) ListActiveTenants(_ context.Context) ([]dbgen.ListActiveTenantsRow, error) {
	return f.rows, f.err
}

// fakeQualityScanner records which tenants were scanned and returns an
// optional per-tenant error so tests can assert the loop keeps going
// after a single-tenant failure.
type fakeQualityScanner struct {
	mu      sync.Mutex
	scanned []uuid.UUID
	errs    map[uuid.UUID]error
	count   int64
}

func (f *fakeQualityScanner) ScanTenant(_ context.Context, tenantID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scanned = append(f.scanned, tenantID)
	atomic.AddInt64(&f.count, 1)
	if f.errs != nil {
		if err, ok := f.errs[tenantID]; ok {
			return err
		}
	}
	return nil
}

func (f *fakeQualityScanner) scanCount() int {
	return int(atomic.LoadInt64(&f.count))
}

// cancelOnFirstScanner cancels the supplied context the moment its
// first tenant is scanned, then returns nil. Used to prove that the
// tenant loop stops mid-iteration when ctx is cancelled — the second
// tenant must never see ScanTenant called.
type cancelOnFirstScanner struct {
	mu      sync.Mutex
	scanned []uuid.UUID
	cancel  context.CancelFunc
}

func (c *cancelOnFirstScanner) ScanTenant(_ context.Context, tenantID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scanned = append(c.scanned, tenantID)
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func tenantRow(id uuid.UUID) dbgen.ListActiveTenantsRow {
	return dbgen.ListActiveTenantsRow{ID: id}
}

// TestRunQualityScan_TwoTenantsOneErrorOtherStillScanned proves the
// Phase 1.4 contract: one tenant's error must not starve the others.
// Both tenants must see ScanTenant called; the metric must gain one ok
// and one error observation.
func TestRunQualityScan_TwoTenantsOneErrorOtherStillScanned(t *testing.T) {
	// Not t.Parallel(): this test reads deltas on the global
	// QualityScannerRunsTotal counter, which would race with any
	// other test in the package that also touches it.
	a := uuid.New()
	b := uuid.New()
	lister := &fakeTenantLister{rows: []dbgen.ListActiveTenantsRow{tenantRow(a), tenantRow(b)}}
	scanner := &fakeQualityScanner{errs: map[uuid.UUID]error{a: errors.New("boom")}}

	okBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok"))
	errBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error"))

	runQualityScanWith(context.Background(), lister, scanner)

	if scanner.scanCount() != 2 {
		t.Fatalf("expected 2 scans, got %d (scanned=%v)", scanner.scanCount(), scanner.scanned)
	}

	okAfter := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok"))
	errAfter := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error"))

	if got := okAfter - okBefore; got != 1 {
		t.Errorf("expected ok counter to advance by 1, got %v", got)
	}
	if got := errAfter - errBefore; got != 1 {
		t.Errorf("expected error counter to advance by 1, got %v", got)
	}
}

// TestRunQualityScan_NoTenantsNoPanic ensures the loop is a no-op when
// there are no active tenants — no metrics should move, nothing should
// panic, and the scanner must never be invoked.
func TestRunQualityScan_NoTenantsNoPanic(t *testing.T) {
	// Not t.Parallel(): metric-delta test, see note above.
	lister := &fakeTenantLister{rows: nil}
	scanner := &fakeQualityScanner{}

	okBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok"))
	errBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error"))

	runQualityScanWith(context.Background(), lister, scanner)

	if scanner.scanCount() != 0 {
		t.Fatalf("expected zero scans, got %d", scanner.scanCount())
	}
	if got := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok")) - okBefore; got != 0 {
		t.Errorf("expected ok counter unchanged, got delta %v", got)
	}
	if got := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error")) - errBefore; got != 0 {
		t.Errorf("expected error counter unchanged, got delta %v", got)
	}
}

// TestRunQualityScan_ListerErrorAborts: if ListActiveTenants itself
// fails we log-and-bail; no per-tenant scan is attempted and no
// outcome metric fires because we never entered the per-tenant loop.
func TestRunQualityScan_ListerErrorAborts(t *testing.T) {
	// Not t.Parallel(): metric-delta test, see note above.
	lister := &fakeTenantLister{err: errors.New("db down")}
	scanner := &fakeQualityScanner{}

	okBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok"))
	errBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error"))

	runQualityScanWith(context.Background(), lister, scanner)

	if scanner.scanCount() != 0 {
		t.Fatalf("expected zero scans on lister error, got %d", scanner.scanCount())
	}
	if got := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok")) - okBefore; got != 0 {
		t.Errorf("ok counter must not move when lister fails, delta=%v", got)
	}
	if got := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error")) - errBefore; got != 0 {
		t.Errorf("error counter must not move when lister fails, delta=%v", got)
	}
}

// TestRunQualityScan_StopsMidLoopOnCancel proves the per-tenant loop
// respects ctx cancellation: after the first tenant is scanned, the
// context is cancelled and the second tenant must never be touched.
func TestRunQualityScan_StopsMidLoopOnCancel(t *testing.T) {
	t.Parallel()
	// This test does not read metric deltas — only verifies scanner
	// invocation count — so running in parallel with other tests is
	// safe.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := uuid.New()
	b := uuid.New()
	lister := &fakeTenantLister{rows: []dbgen.ListActiveTenantsRow{tenantRow(a), tenantRow(b)}}
	scanner := &cancelOnFirstScanner{cancel: cancel}

	runQualityScanWith(ctx, lister, scanner)

	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	if len(scanner.scanned) != 1 {
		t.Fatalf("expected loop to stop after first tenant, got %d scans (%v)",
			len(scanner.scanned), scanner.scanned)
	}
	if scanner.scanned[0] != a {
		t.Errorf("expected first tenant %s to be scanned, got %s", a, scanner.scanned[0])
	}
}

// TestQualityScanLoop_ContextCancelExitsWithinOneTick asserts the
// scheduler goroutine exits promptly when its parent context is
// cancelled (Phase 2.7 ctx-wiring contract). We drive the exported
// loop through a WorkflowSubscriber stub with the tenant-lister
// override set, then cancel and check the goroutine returned.
func TestQualityScanLoop_ContextCancelExitsWithinOneTick(t *testing.T) {
	t.Parallel()
	// No metric reads — safe to parallelise.
	ctx, cancel := context.WithCancel(context.Background())

	lister := &fakeTenantLister{rows: nil}
	scanner := &fakeQualityScanner{}
	w := &WorkflowSubscriber{
		qualitySvc:                  scanner,
		qualityTenantListerOverride: lister,
	}

	done := make(chan struct{})
	go func() {
		w.qualityScanLoop(ctx)
		close(done)
	}()

	// Let the goroutine install its timers before cancelling so we
	// exercise the ctx.Done() branch of the select, not a pre-entry
	// short-circuit.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("qualityScanLoop did not exit within 2s of context cancel")
	}
}

// TestStartQualityScanner_NoScannerIsNoOp: if WithQualityScanner was
// never called, StartQualityScanner must be a silent no-op (no panic,
// no goroutine, no metric move). This guards against crashes on edge
// nodes / minimal deployments that deliberately disable the scan loop.
func TestStartQualityScanner_NoScannerIsNoOp(t *testing.T) {
	// Not t.Parallel(): metric-delta test, see note above.
	w := &WorkflowSubscriber{} // qualitySvc nil

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	okBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok"))
	errBefore := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error"))

	// Must not panic and must return promptly.
	w.StartQualityScanner(ctx)

	// Give any (incorrectly-spawned) goroutine a window to fire.
	time.Sleep(50 * time.Millisecond)

	if got := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("ok")) - okBefore; got != 0 {
		t.Errorf("no-op path must not move ok counter, delta=%v", got)
	}
	if got := testutil.ToFloat64(telemetry.QualityScannerRunsTotal.WithLabelValues("error")) - errBefore; got != 0 {
		t.Errorf("no-op path must not move error counter, delta=%v", got)
	}
}
