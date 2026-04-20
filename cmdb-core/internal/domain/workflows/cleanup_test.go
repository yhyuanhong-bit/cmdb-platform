package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// newFailingPool returns a *pgxpool.Pool that never connects — the
// DSN points at an unreachable port so every Exec/Query round-trip
// synchronously returns a dial error. We use a short pool timeout so
// the test doesn't linger on the retry backoff.
//
// We deliberately avoid t.Fatal here when the pool fails to parse —
// the caller owns the failure decision so the helper stays re-usable
// from multiple tests.
func newFailingPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// 127.0.0.1:1 is reserved TCPMUX — nothing listens there on any
	// reasonable host, so connect() fails fast. pool_max_conns=1
	// keeps the test cheap; connect_timeout=1 bounds the wait.
	cfg, err := pgxpool.ParseConfig(
		"postgres://cmdb:changeme@127.0.0.1:1/cmdb?pool_max_conns=1&connect_timeout=1",
	)
	if err != nil {
		t.Fatalf("parse failing pool config: %v", err)
	}
	cfg.ConnConfig.ConnectTimeout = 1 * time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("build failing pool: %v", err)
	}
	return pool
}

// TestCleanupSessions_PropagatesWarnAndCounter proves that when the
// underlying pgxpool call fails, the new errors-suppressed wiring
// both emits a Warn line and bumps the telemetry counter instead of
// silently swallowing the error as the pre-fix code did.
//
// We run cleanupSessions against a pool pointed at a dead address
// under a short-deadline context so every pool.Exec returns an error
// quickly, and assert:
//
//  1. at least one Warn line landed on the observer
//  2. errors_suppressed_total{source="workflows.cleanup.sessions"}
//     incremented by >= 1 relative to the pre-call snapshot
//
// The point is to prove the NEW code path is reachable, not to
// measure pgxpool behaviour. Keeping the test unit-scope (no DB)
// means it stays on the default `go test` target so CI catches any
// future regression that re-introduces a silent discard.
func TestCleanupSessions_PropagatesWarnAndCounter(t *testing.T) {
	// Swap zap's global logger for an observer so we can inspect
	// the Warn output. Restore at the end so other tests in the
	// package keep their usual logger.
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceCleanupSessions,
			telemetry.ReasonDBExecFailed,
		),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.cleanupSessions(ctx)

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(
			sourceCleanupSessions,
			telemetry.ReasonDBExecFailed,
		),
	)
	if after <= before {
		t.Fatalf("errors_suppressed_total did not increment (before=%v after=%v)", before, after)
	}

	warnCount := 0
	for _, entry := range logs.All() {
		if entry.Level == zap.WarnLevel {
			warnCount++
		}
	}
	if warnCount == 0 {
		t.Fatalf("expected at least one Warn log line, got 0; entries=%v", logs.All())
	}
}
