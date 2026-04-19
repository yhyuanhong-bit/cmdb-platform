//go:build integration

package workflows

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests verify Phase 2.8: the SLA-breach scan is atomic. The old
// SELECT → UPDATE flow had a TOCTOU window where two scheduler
// instances (or a restart mid-loop) could each see the same row as
// "not yet breached" and double-publish the notification. The new
// UPDATE ... RETURNING closes that window because the `NOT sla_breached`
// guard lives inside the UPDATE's WHERE clause and Postgres's row-level
// lock serialises concurrent attempts.
//
// Run with:
//
//	go test -tags integration -race ./internal/domain/workflows/...

// slaFixture provisions an isolated tenant + user + work-order stock
// per test. t.Cleanup drops everything in reverse-FK order so parallel
// tests cannot leak state into one another.
type slaFixture struct {
	tenantID uuid.UUID
	userID   uuid.UUID
}

func setupSLAFixture(t *testing.T, pool *pgxpool.Pool) slaFixture {
	t.Helper()
	ctx := context.Background()
	fix := slaFixture{tenantID: uuid.New(), userID: uuid.New()}
	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "sla-test-"+suffix, "sla-test-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status)
		 VALUES ($1, $2, $3, 'SLA Test', $4, 'x', 'active')`,
		fix.userID, fix.tenantID, "sla-"+suffix, "sla-"+suffix+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM work_order_logs WHERE order_id IN (SELECT id FROM work_orders WHERE tenant_id = $1)`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM work_orders WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

// insertWO seeds a work order with explicit status / deadline / breach
// state. Returns the row id.
func insertWO(t *testing.T, pool *pgxpool.Pool, fix slaFixture, status string, deadline time.Time, breached bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	code := fmt.Sprintf("WO-%s", id.String()[:8])
	if _, err := pool.Exec(ctx, `
		INSERT INTO work_orders
			(id, tenant_id, code, title, type, status, priority,
			 assignee_id, requestor_id, sla_deadline, sla_breached)
		VALUES ($1, $2, $3, 'SLA test', 'maintenance', $4, 'high',
		        $5, $5, $6, $7)`,
		id, fix.tenantID, code, status, fix.userID, deadline, breached,
	); err != nil {
		t.Fatalf("insert work_order: %v", err)
	}
	return id
}

// newSubscriber returns a minimal WorkflowSubscriber — no event bus,
// no maintenance service, no cipher. createNotification tolerates a
// nil bus (it just skips the publish). That keeps these tests focused
// on the SQL contract.
func newSubscriber(pool *pgxpool.Pool) *WorkflowSubscriber {
	return &WorkflowSubscriber{pool: pool}
}

// TestCheckSLABreaches_FreshBreach covers the happy path: a WO whose
// deadline is in the past and which has not yet been marked breached.
// One pass through checkSLABreaches should flip the flag and leave a
// persisted notifications row for the assignee.
func TestCheckSLABreaches_FreshBreach(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSLAFixture(t, pool)
	ctx := context.Background()

	woID := insertWO(t, pool, fix, "in_progress", time.Now().Add(-time.Hour), false)

	w := newSubscriber(pool)
	w.checkSLABreaches(ctx)

	var breached bool
	if err := pool.QueryRow(ctx,
		`SELECT sla_breached FROM work_orders WHERE id = $1`, woID,
	).Scan(&breached); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !breached {
		t.Fatalf("expected sla_breached=true after sweep, got false")
	}

	var notifCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications
		 WHERE tenant_id = $1 AND user_id = $2 AND type = 'sla_breach'
		   AND resource_type = 'work_order' AND resource_id = $3`,
		fix.tenantID, fix.userID, woID,
	).Scan(&notifCount); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if notifCount != 1 {
		t.Fatalf("expected 1 sla_breach notification, got %d", notifCount)
	}
}

// TestCheckSLABreaches_AlreadyBreached verifies idempotency: a WO that
// is already marked sla_breached=true must not re-notify on the next
// tick, even if its deadline is still in the past. The new UPDATE's
// `NOT sla_breached` guard is what enforces this — without it, the
// old code would have re-matched and re-notified every minute.
func TestCheckSLABreaches_AlreadyBreached(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSLAFixture(t, pool)
	ctx := context.Background()

	woID := insertWO(t, pool, fix, "in_progress", time.Now().Add(-time.Hour), true)

	w := newSubscriber(pool)
	w.checkSLABreaches(ctx)

	var notifCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications
		 WHERE tenant_id = $1 AND resource_id = $2`,
		fix.tenantID, woID,
	).Scan(&notifCount); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if notifCount != 0 {
		t.Fatalf("expected 0 notifications for already-breached WO, got %d", notifCount)
	}
}

// TestCheckSLABreaches_SkipsNonEligibleStatuses tables the status
// filter: only approved + in_progress rows can breach. draft,
// submitted, completed, rejected, verified must be untouched even
// with past deadlines.
func TestCheckSLABreaches_SkipsNonEligibleStatuses(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSLAFixture(t, pool)
	ctx := context.Background()

	pastDeadline := time.Now().Add(-time.Hour)
	cases := []struct {
		status     string
		wantBreach bool
	}{
		{"draft", false},
		{"submitted", false},
		{"approved", true},
		{"rejected", false},
		{"in_progress", true},
		{"completed", false},
		{"verified", false},
	}
	ids := make(map[string]uuid.UUID, len(cases))
	for _, c := range cases {
		ids[c.status] = insertWO(t, pool, fix, c.status, pastDeadline, false)
	}

	w := newSubscriber(pool)
	w.checkSLABreaches(ctx)

	for _, c := range cases {
		var got bool
		if err := pool.QueryRow(ctx,
			`SELECT sla_breached FROM work_orders WHERE id = $1`, ids[c.status],
		).Scan(&got); err != nil {
			t.Fatalf("status=%s readback: %v", c.status, err)
		}
		if got != c.wantBreach {
			t.Errorf("status=%s: sla_breached = %v, want %v", c.status, got, c.wantBreach)
		}
	}
}

// TestCheckSLABreaches_ConcurrentRace spins up N goroutines that all
// call checkSLABreaches simultaneously against the SAME pre-breach
// row. Because the `NOT sla_breached` guard lives inside the UPDATE's
// WHERE clause, Postgres's row-level lock guarantees exactly ONE
// goroutine sees the row in its RETURNING set — the other N-1 see an
// empty result and publish nothing. This is the core TOCTOU fix.
func TestCheckSLABreaches_ConcurrentRace(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSLAFixture(t, pool)
	ctx := context.Background()

	woID := insertWO(t, pool, fix, "in_progress", time.Now().Add(-time.Hour), false)

	// Also seed 2 unrelated non-breaching rows to prove the query
	// doesn't confuse race contention with over-matching.
	_ = insertWO(t, pool, fix, "in_progress", time.Now().Add(time.Hour), false) // future
	_ = insertWO(t, pool, fix, "draft", time.Now().Add(-time.Hour), false)      // wrong status

	const parallel = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(parallel)

	for i := 0; i < parallel; i++ {
		go func() {
			defer wg.Done()
			w := newSubscriber(pool)
			<-start
			w.checkSLABreaches(ctx)
		}()
	}
	close(start)
	wg.Wait()

	// Database: exactly one notification row for the target WO.
	var notifCount int64
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications
		 WHERE tenant_id = $1 AND resource_id = $2 AND type = 'sla_breach'`,
		fix.tenantID, woID,
	).Scan(&notifCount); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if notifCount != 1 {
		t.Fatalf("concurrent race: expected exactly 1 notification, got %d", notifCount)
	}

	// Database: target row is now breached.
	var breached bool
	if err := pool.QueryRow(ctx,
		`SELECT sla_breached FROM work_orders WHERE id = $1`, woID,
	).Scan(&breached); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if !breached {
		t.Fatalf("concurrent race: row should be breached after sweep")
	}
}
