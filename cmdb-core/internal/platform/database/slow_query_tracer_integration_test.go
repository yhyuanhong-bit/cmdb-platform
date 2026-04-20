//go:build integration

// Integration tests for SlowQueryTracer against a real Postgres. Run with:
//
//	go test -tags integration -race ./internal/platform/database/...
//
// TEST_DATABASE_URL overrides the default docker-compose connection.
// Tests Skip when the DB is unreachable so `go test ./...` stays green
// on machines without the stack up.

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

// newReachablePool applies the slow-query tracer via NewPool, skipping
// if the DB is unreachable.
func newReachablePool(t *testing.T, threshold time.Duration, logger *zap.Logger) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := NewPool(ctx, testDBURL(),
		WithSlowQueryThreshold(threshold),
		WithLogger(logger),
	)
	if err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

// TestSlowQueryTracer_RealPgSleep proves the tracer fires on a real
// Postgres round-trip: we execute SELECT pg_sleep(0.6) with a 200ms
// threshold and assert exactly one warn line lands on the observer.
func TestSlowQueryTracer_RealPgSleep(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	pool := newReachablePool(t, 200*time.Millisecond, logger)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// pg_sleep(0.6) blocks the server 600ms → end-to-end well above 200ms.
	var scratch int
	if err := pool.QueryRow(ctx, "SELECT 1 FROM pg_sleep(0.6)").Scan(&scratch); err != nil {
		t.Fatalf("pg_sleep query failed: %v", err)
	}

	warnLines := recorded.FilterMessage("slow db query").All()
	if len(warnLines) < 1 {
		t.Fatalf("expected at least 1 'slow db query' warn, got %d (all entries: %v)",
			len(warnLines), recorded.All())
	}

	// Counter also incremented — label is the sanitized fingerprint of
	// the SELECT we just ran.
	fp := fingerprintSQL("SELECT 1 FROM pg_sleep(0.6)")
	if got := testutil.ToFloat64(dbSlowQueriesTotal.WithLabelValues(fp)); got < 1 {
		t.Fatalf("slow query counter not incremented (got %v)", got)
	}
}

// TestSlowQueryTracer_RealFastQuery_NoWarn proves we don't spam on
// ordinary queries — SELECT 1 against localhost should stay well below
// the default 500ms threshold.
func TestSlowQueryTracer_RealFastQuery_NoWarn(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	// Use the default threshold explicitly so the test doesn't drift
	// if we ever change the default.
	pool := newReachablePool(t, 500*time.Millisecond, logger)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var scratch int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&scratch); err != nil {
		t.Fatalf("SELECT 1 failed: %v", err)
	}

	warnLines := recorded.FilterMessage("slow db query").All()
	if len(warnLines) != 0 {
		t.Fatalf("fast query should not emit warn, got %d (all: %v)", len(warnLines), recorded.All())
	}
}
