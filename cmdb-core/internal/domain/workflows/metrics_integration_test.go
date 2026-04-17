//go:build integration

package workflows

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the persisted backoff + auto-disable behavior
// against a real Postgres so the SQL-computed next_attempt_at schedule
// and the ListDuePullAdapters filter are covered end-to-end.
//
// Run with:
//   go test -tags integration -race ./internal/domain/workflows/...
//
// TEST_DATABASE_URL can override the default docker-compose connection.
// Tests Skip when the DB is unreachable so `go test ./...` stays green
// on machines without the stack up.

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

// adapterFixture stands up an isolated tenant + inbound adapter per
// test. t.Cleanup drops everything so parallel tests do not leak.
type adapterFixture struct {
	tenantID  uuid.UUID
	adapterID uuid.UUID
}

func setupAdapterFixture(t *testing.T, pool *pgxpool.Pool) adapterFixture {
	t.Helper()
	ctx := context.Background()
	fix := adapterFixture{tenantID: uuid.New(), adapterID: uuid.New()}
	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "puller-test-"+suffix, "puller-test-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO integration_adapters (id, tenant_id, name, type, direction, endpoint, enabled)
		 VALUES ($1, $2, $3, 'rest', 'inbound', 'http://127.0.0.1:1/metrics', true)`,
		fix.adapterID, fix.tenantID, "puller-test-"+suffix); err != nil {
		t.Fatalf("insert adapter: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM audit_events WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM integration_adapters WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

// TestRecordAdapterFailure_Backoff asserts the SQL-computed
// next_attempt_at matches the documented schedule: 30s / 2m / 10m / 30m cap.
func TestRecordAdapterFailure_Backoff(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	queries := dbgen.New(pool)

	tests := []struct {
		name        string
		failures    int // how many times to record failure
		wantApprox  time.Duration
		wantCounter int32
	}{
		{"first failure → ~30s", 1, 30 * time.Second, 1},
		{"second failure → ~2m", 2, 2 * time.Minute, 2},
		{"third failure → ~10m", 3, 10 * time.Minute, 3},
		{"fourth failure → ~30m cap", 4, 30 * time.Minute, 4},
		{"fifth failure → still ~30m cap", 5, 30 * time.Minute, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fix := setupAdapterFixture(t, pool)
			ctx := context.Background()

			var row dbgen.RecordAdapterFailureRow
			var err error
			for i := 0; i < tt.failures; i++ {
				row, err = queries.RecordAdapterFailure(ctx, dbgen.RecordAdapterFailureParams{
					ID:                fix.adapterID,
					TenantID:          fix.tenantID,
					LastFailureReason: pgtype.Text{String: "boom", Valid: true},
				})
				if err != nil {
					t.Fatalf("RecordAdapterFailure: %v", err)
				}
			}

			if row.ConsecutiveFailures != tt.wantCounter {
				t.Errorf("consecutive_failures = %d, want %d", row.ConsecutiveFailures, tt.wantCounter)
			}
			if !row.NextAttemptAt.Valid {
				t.Fatalf("next_attempt_at not set")
			}
			gotDelay := time.Until(row.NextAttemptAt.Time)
			// Allow a generous tolerance: SQL now() fired a moment ago.
			if gotDelay < tt.wantApprox-5*time.Second || gotDelay > tt.wantApprox+5*time.Second {
				t.Errorf("backoff delay = %v, want ~%v", gotDelay, tt.wantApprox)
			}
		})
	}
}

// TestRecordAdapterSuccess_ResetsState verifies a successful pull after
// prior failures clears the counter, reason, and next_attempt_at so the
// adapter re-enters the normal poll cadence immediately.
func TestRecordAdapterSuccess_ResetsState(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	queries := dbgen.New(pool)
	fix := setupAdapterFixture(t, pool)
	ctx := context.Background()

	// Seed two failures.
	for i := 0; i < 2; i++ {
		if _, err := queries.RecordAdapterFailure(ctx, dbgen.RecordAdapterFailureParams{
			ID:                fix.adapterID,
			TenantID:          fix.tenantID,
			LastFailureReason: pgtype.Text{String: "timeout", Valid: true},
		}); err != nil {
			t.Fatalf("seed failure: %v", err)
		}
	}

	if err := queries.RecordAdapterSuccess(ctx, dbgen.RecordAdapterSuccessParams{
		ID:       fix.adapterID,
		TenantID: fix.tenantID,
	}); err != nil {
		t.Fatalf("RecordAdapterSuccess: %v", err)
	}

	var count int32
	var reason pgtype.Text
	var nextAt pgtype.Timestamptz
	err := pool.QueryRow(ctx,
		`SELECT consecutive_failures, last_failure_reason, next_attempt_at
		   FROM integration_adapters WHERE id = $1`, fix.adapterID,
	).Scan(&count, &reason, &nextAt)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if count != 0 {
		t.Errorf("consecutive_failures = %d after success, want 0", count)
	}
	if reason.Valid {
		t.Errorf("last_failure_reason should be NULL after success, got %q", reason.String)
	}
	if nextAt.Valid {
		t.Errorf("next_attempt_at should be NULL after success, got %v", nextAt.Time)
	}
}

// TestListDuePullAdapters_SkipsBackoff confirms the SQL filter excludes
// adapters whose next_attempt_at is still in the future — the core fix:
// a broken adapter is not re-polled every tick.
func TestListDuePullAdapters_SkipsBackoff(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	queries := dbgen.New(pool)
	fix := setupAdapterFixture(t, pool)
	ctx := context.Background()

	// Record a failure so next_attempt_at is ~30s in the future.
	if _, err := queries.RecordAdapterFailure(ctx, dbgen.RecordAdapterFailureParams{
		ID:                fix.adapterID,
		TenantID:          fix.tenantID,
		LastFailureReason: pgtype.Text{String: "unreachable", Valid: true},
	}); err != nil {
		t.Fatalf("RecordAdapterFailure: %v", err)
	}

	due, err := queries.ListDuePullAdapters(ctx)
	if err != nil {
		t.Fatalf("ListDuePullAdapters: %v", err)
	}
	for _, a := range due {
		if a.ID == fix.adapterID {
			t.Fatalf("adapter in backoff window was returned by ListDuePullAdapters")
		}
	}

	// Force the backoff window to expire and re-query.
	if _, err := pool.Exec(ctx,
		`UPDATE integration_adapters SET next_attempt_at = now() - INTERVAL '1 second' WHERE id = $1`,
		fix.adapterID); err != nil {
		t.Fatalf("expire backoff: %v", err)
	}
	due, err = queries.ListDuePullAdapters(ctx)
	if err != nil {
		t.Fatalf("ListDuePullAdapters #2: %v", err)
	}
	found := false
	for _, a := range due {
		if a.ID == fix.adapterID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("adapter past backoff window was not returned")
	}
}

// TestHandleAdapterFailure_ThresholdDisablesAndAudits exercises the full
// workflow-level flow: three failures → adapter disabled + audit row.
// This replaces the lost behavior of the old in-memory threshold check.
func TestHandleAdapterFailure_ThresholdDisablesAndAudits(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	queries := dbgen.New(pool)
	fix := setupAdapterFixture(t, pool)
	ctx := context.Background()

	w := &WorkflowSubscriber{pool: pool, queries: queries}

	for i := 0; i < adapterDisableThreshold; i++ {
		w.handleAdapterFailure(ctx, fix.adapterID, fix.tenantID,
			"puller-test", "rest", errors.New("connection refused"))
	}

	var enabled bool
	if err := pool.QueryRow(ctx,
		`SELECT enabled FROM integration_adapters WHERE id = $1`, fix.adapterID,
	).Scan(&enabled); err != nil {
		t.Fatalf("read adapter: %v", err)
	}
	if enabled {
		t.Fatalf("adapter should be disabled after %d failures", adapterDisableThreshold)
	}

	var auditCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events
		  WHERE tenant_id = $1 AND action = 'adapter_auto_disabled'
		    AND target_id = $2`,
		fix.tenantID, fix.adapterID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit_events rows = %d, want 1", auditCount)
	}
}
