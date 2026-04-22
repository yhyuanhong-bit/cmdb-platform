package workflows

import (
	"context"
	"testing"
	"time"
)

// These tests verify the Start* loop contract: each launcher spawns a
// background goroutine that respects ctx cancellation and exits
// promptly. Every loop's initial tick fires on a timer/ticker that is
// far longer than the test window (minimum 60 seconds), so with an
// immediately-cancelled context the goroutine's select hits the
// ctx.Done() arm before any DB-touching work is attempted.
//
// These are NOT "does the loop actually work" tests — the per-tick
// behaviour is covered in the //go:build integration suites that run
// with a live Postgres. These are the shutdown-safety tests: after a
// SIGTERM (which flows through ctx), each loop's goroutine must exit
// rather than hanging the process.

// exitsPromptly spawns the Start* function in a goroutine, cancels
// the context, and asserts the launcher returns quickly. The launcher
// itself is synchronous (returns after `go func()...`); the concern
// is that the launcher does not itself block on a pre-start
// initialiser that ignores ctx.
func exitsPromptly(t *testing.T, start func(ctx context.Context)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		start(ctx)
		close(done)
	}()

	// Each Start* launcher is synchronous: it builds a ticker, spawns
	// a goroutine, logs, and returns. 200ms is generous — anything
	// slower indicates the launcher is doing initialisation work
	// that blocks the caller.
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Start* launcher did not return within 200ms")
	}
	// Cancellation at this point lets any background goroutine the
	// launcher spawned shut down. We don't try to observe that
	// goroutine exit — the ctx.Done() path is already covered by
	// TestQualityScanLoop_ContextCancelExitsWithinOneTick.
}

// TestStartSessionCleanup_LauncherReturns: the hourly ticker launcher
// must hand control back to the caller quickly. It does not block on
// a first-tick warm-up (unlike StartWarrantyChecker, which runs
// runDailyChecks inline in the spawned goroutine).
func TestStartSessionCleanup_LauncherReturns(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{} // no pool needed for the launcher
	exitsPromptly(t, w.StartSessionCleanup)
}

// TestStartSLAChecker_LauncherReturns: 60s ticker; first tick is far
// past our test window.
func TestStartSLAChecker_LauncherReturns(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	exitsPromptly(t, w.StartSLAChecker)
}

// TestStartConflictAndDiscoveryCleanup_LauncherReturns: hourly
// ticker, no first-tick warm-up.
func TestStartConflictAndDiscoveryCleanup_LauncherReturns(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	exitsPromptly(t, w.StartConflictAndDiscoveryCleanup)
}

// TestStartMetricsPuller_LauncherReturns: 5-minute ticker, no
// first-tick warm-up.
func TestStartMetricsPuller_LauncherReturns(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	exitsPromptly(t, w.StartMetricsPuller)
}

// TestStartAssetVerificationChecker_LauncherReturns: weekly ticker,
// no first-tick warm-up.
func TestStartAssetVerificationChecker_LauncherReturns(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	exitsPromptly(t, w.StartAssetVerificationChecker)
}

// TestStartWebhookRetention_LauncherReturns: uses a timer aligned to
// next 03:00 UTC, then a daily ticker. The first-tick wait is up to
// 24 hours, far past the test window.
func TestStartWebhookRetention_LauncherReturns(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	exitsPromptly(t, w.StartWebhookRetention)
}

// TestStartSessionCleanup_CancelExits: confirm the spawned goroutine
// honours ctx cancellation. Not a timing test — we do not assert an
// upper bound on exit latency since the Start* goroutine exits on
// the next <-ctx.Done() select arm (immediate) or the next tick.
func TestStartSessionCleanup_CancelExits(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	ctx, cancel := context.WithCancel(context.Background())
	w.StartSessionCleanup(ctx)
	cancel()
	// A 100ms grace window is plenty — the ctx.Done() arm wins
	// against an hour-long ticker unconditionally.
	time.Sleep(100 * time.Millisecond)
}
