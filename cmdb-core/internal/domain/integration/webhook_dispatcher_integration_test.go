//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the persisted circuit breaker, DLQ, attempt
// tracking, and retention sweeps against a real Postgres so the SQL is
// covered end-to-end.
//
// Run with:
//
//	go test -tags integration -race ./internal/domain/integration/...
//
// TEST_DATABASE_URL can override the default docker-compose connection.
// Tests Skip when the DB is unreachable so `go test ./...` stays green
// on machines without the stack up.

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"
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

// webhookFixture creates an isolated tenant + webhook subscription per test.
// t.Cleanup drops everything so parallel tests do not leak state.
type webhookFixture struct {
	tenantID uuid.UUID
	subID    uuid.UUID
	pool     *pgxpool.Pool
	queries  *dbgen.Queries
}

func setupWebhookFixture(t *testing.T, pool *pgxpool.Pool, targetURL string) *webhookFixture {
	t.Helper()
	ctx := context.Background()
	fix := &webhookFixture{
		tenantID: uuid.New(),
		subID:    uuid.New(),
		pool:     pool,
		queries:  dbgen.New(pool),
	}
	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug, status) VALUES ($1, $2, $3, 'active')`,
		fix.tenantID, "t-"+suffix, "t-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO webhook_subscriptions (id, tenant_id, name, url, events, enabled)
		 VALUES ($1, $2, $3, $4, ARRAY['asset.created'], true)`,
		fix.subID, fix.tenantID, "hook-"+suffix, targetURL); err != nil {
		t.Fatalf("insert webhook: %v", err)
	}

	t.Cleanup(func() {
		cctx := context.Background()
		_, _ = pool.Exec(cctx, `DELETE FROM webhook_deliveries WHERE subscription_id = $1`, fix.subID)
		_, _ = pool.Exec(cctx, `DELETE FROM webhook_deliveries_dlq WHERE subscription_id = $1`, fix.subID)
		_, _ = pool.Exec(cctx, `DELETE FROM webhook_subscriptions WHERE id = $1`, fix.subID)
		_, _ = pool.Exec(cctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

// loadSubscription returns the current DB row for the fixture's subscription.
func (f *webhookFixture) loadSubscription(t *testing.T) dbgen.WebhookSubscription {
	t.Helper()
	sub, err := f.queries.GetWebhookByID(context.Background(), dbgen.GetWebhookByIDParams{
		ID: f.subID, TenantID: f.tenantID,
	})
	if err != nil {
		t.Fatalf("GetWebhookByID: %v", err)
	}
	return sub
}

// TestCircuitBreaker_TripsAfterThreeFailures is the headline test for
// Phase 2.5: three failed deliveries in a row must flip disabled_at, park
// the payload in the DLQ, and cause subsequent deliver() calls to return
// without hitting the network.
func TestCircuitBreaker_TripsAfterThreeFailures(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	fix := setupWebhookFixture(t, pool, srv.URL)
	d := NewWebhookDispatcher(fix.queries, nil, netguard.Permissive())

	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: fix.tenantID.String(),
		Payload:  []byte(`{"asset_id":"abc"}`),
	}

	// Three fully-exhausted deliveries (3 HTTP attempts each) — that's
	// 3 recordFailure calls, so consecutive_failures should climb 1→2→3
	// and the last one trips the breaker.
	for i := 0; i < 3; i++ {
		sub := fix.loadSubscription(t)
		if sub.DisabledAt.Valid {
			break // breaker already tripped mid-sequence
		}
		d.deliver(sub, event)
	}

	sub := fix.loadSubscription(t)
	if !sub.DisabledAt.Valid {
		t.Fatal("expected disabled_at to be set after 3 failures")
	}
	if sub.ConsecutiveFailures < 3 {
		t.Fatalf("expected consecutive_failures >= 3, got %d", sub.ConsecutiveFailures)
	}

	// DLQ row must have been inserted with tenant_id + error.
	var dlqCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM webhook_deliveries_dlq WHERE subscription_id = $1 AND tenant_id = $2`,
		fix.subID, fix.tenantID).Scan(&dlqCount); err != nil {
		t.Fatalf("count DLQ: %v", err)
	}
	if dlqCount < 1 {
		t.Fatal("expected at least one DLQ row after breaker trip")
	}

	// Further deliver() calls after the trip must be no-ops at the
	// network level — the dispatcher sees disabled_at.Valid and bails.
	before := atomic.LoadInt32(&hits)
	d.deliver(sub, event)
	after := atomic.LoadInt32(&hits)
	if before != after {
		t.Fatalf("disabled webhook still dialed the receiver (hits before=%d after=%d)", before, after)
	}
}

// TestRecovery_ResetsConsecutiveFailures asserts that a single successful
// delivery wipes the failure counter so a flaky-but-recovering receiver
// doesn't accrete toward the threshold over many days.
func TestRecovery_ResetsConsecutiveFailures(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	// Receiver returns 500 for the first 2 attempts then 200. We run
	// deliver() once (which does up to 3 attempts with backoff) and
	// expect the final 200 to reset the counter.
	var call int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&call, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fix := setupWebhookFixture(t, pool, srv.URL)

	// Pre-seed a non-zero counter to prove recordSuccess zeroes it.
	if _, err := pool.Exec(context.Background(),
		`UPDATE webhook_subscriptions SET consecutive_failures = 2 WHERE id = $1`,
		fix.subID); err != nil {
		t.Fatalf("seed counter: %v", err)
	}

	d := NewWebhookDispatcher(fix.queries, nil, netguard.Permissive())
	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: fix.tenantID.String(),
		Payload:  []byte(`{}`),
	}
	d.deliver(fix.loadSubscription(t), event)

	sub := fix.loadSubscription(t)
	if sub.ConsecutiveFailures != 0 {
		t.Fatalf("recovery should reset counter, got %d", sub.ConsecutiveFailures)
	}
	if sub.LastFailureAt.Valid {
		t.Fatal("recovery should clear last_failure_at")
	}
	if sub.DisabledAt.Valid {
		t.Fatal("successful delivery must not flip disabled_at")
	}
}

// TestAttemptTracking_InsertsRowPerRetry proves that retries insert new
// webhook_deliveries rows with ascending attempt_number rather than
// overwriting a single "final status" row.
func TestAttemptTracking_InsertsRowPerRetry(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	var call int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&call, 1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fix := setupWebhookFixture(t, pool, srv.URL)
	d := NewWebhookDispatcher(fix.queries, nil, netguard.Permissive())
	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: fix.tenantID.String(),
		Payload:  []byte(`{}`),
	}
	d.deliver(fix.loadSubscription(t), event)

	// Expect exactly 2 rows: attempt 1 (500) and attempt 2 (200). No
	// row for attempt 3 because the dispatcher stops on success.
	rows, err := pool.Query(context.Background(),
		`SELECT attempt_number, status_code FROM webhook_deliveries
		 WHERE subscription_id = $1 ORDER BY attempt_number ASC`, fix.subID)
	if err != nil {
		t.Fatalf("query deliveries: %v", err)
	}
	defer rows.Close()
	var attempts []struct {
		n      int32
		status pgtype.Int4
	}
	for rows.Next() {
		var a struct {
			n      int32
			status pgtype.Int4
		}
		if err := rows.Scan(&a.n, &a.status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		attempts = append(attempts, a)
	}
	if len(attempts) != 2 {
		t.Fatalf("want 2 attempt rows, got %d", len(attempts))
	}
	if attempts[0].n != 1 || attempts[1].n != 2 {
		t.Fatalf("attempt_number sequence wrong: %+v", attempts)
	}
	if attempts[0].status.Int32 != 500 || attempts[1].status.Int32 != 200 {
		t.Fatalf("status codes wrong: %+v", attempts)
	}
}

// TestDLQ_IncludesTenantID confirms that the DLQ row carries tenant_id.
// This is the load-bearing invariant from the spec — cross-tenant DLQ
// leakage is a CRITICAL defect.
func TestDLQ_IncludesTenantID(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	fix := setupWebhookFixture(t, pool, srv.URL)
	d := NewWebhookDispatcher(fix.queries, nil, netguard.Permissive())
	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: fix.tenantID.String(),
		Payload:  []byte(`{"k":"v"}`),
	}

	// Run three full deliver() cycles to guarantee we cross the threshold.
	for i := 0; i < 3; i++ {
		sub := fix.loadSubscription(t)
		if sub.DisabledAt.Valid {
			break
		}
		d.deliver(sub, event)
	}

	var tenantID uuid.UUID
	var payload []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT tenant_id, payload FROM webhook_deliveries_dlq
		 WHERE subscription_id = $1 LIMIT 1`, fix.subID,
	).Scan(&tenantID, &payload); err != nil {
		t.Fatalf("load DLQ row: %v", err)
	}
	if tenantID != fix.tenantID {
		t.Fatalf("DLQ tenant_id mismatch: want %s, got %s", fix.tenantID, tenantID)
	}
	// Payload must round-trip as valid JSON — otherwise replay is broken.
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("DLQ payload not valid JSON: %v", err)
	}
}

// TestRetention_DeletesOldRowsPreservesNew seeds rows with explicit
// timestamps, runs the queries with a short retention window, and asserts
// that only old rows are gone.
func TestRetention_DeletesOldRowsPreservesNew(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	fix := setupWebhookFixture(t, pool, "https://example.invalid/hook")
	q := fix.queries
	ctx := context.Background()

	// Seed: one "old" delivery (40d ago) and one "new" (1d ago). With a
	// 30-day retention window only the old one should vanish.
	oldID := uuid.New()
	newID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO webhook_deliveries (id, subscription_id, event_type, payload, status_code, delivered_at)
		 VALUES ($1, $2, 'asset.created', '{}'::jsonb, 200, now() - interval '40 days'),
		        ($3, $2, 'asset.created', '{}'::jsonb, 200, now() - interval '1 day')`,
		oldID, fix.subID, newID); err != nil {
		t.Fatalf("seed deliveries: %v", err)
	}

	deleted, err := q.DeleteOldWebhookDeliveries(ctx, 30)
	if err != nil {
		t.Fatalf("DeleteOldWebhookDeliveries: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("expected at least 1 deletion, got %d", deleted)
	}

	// Verify: old gone, new preserved.
	var oldGone, newKept int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM webhook_deliveries WHERE id = $1`, oldID,
	).Scan(&oldGone); err != nil {
		t.Fatalf("count old: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM webhook_deliveries WHERE id = $1`, newID,
	).Scan(&newKept); err != nil {
		t.Fatalf("count new: %v", err)
	}
	if oldGone != 0 {
		t.Fatal("old delivery row should have been deleted")
	}
	if newKept != 1 {
		t.Fatal("new delivery row should have been preserved")
	}

	// DLQ retention: seed at 100 days and 1 day, 90-day retention window.
	oldDLQ := uuid.New()
	newDLQ := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO webhook_deliveries_dlq (id, subscription_id, tenant_id, event_type, payload, last_error, attempt_count, created_at)
		 VALUES ($1, $2, $3, 'asset.created', '{}'::jsonb, 'boom', 3, now() - interval '100 days'),
		        ($4, $2, $3, 'asset.created', '{}'::jsonb, 'boom', 3, now() - interval '1 day')`,
		oldDLQ, fix.subID, fix.tenantID, newDLQ); err != nil {
		t.Fatalf("seed DLQ: %v", err)
	}

	deletedDLQ, err := q.DeleteOldWebhookDLQ(ctx, 90)
	if err != nil {
		t.Fatalf("DeleteOldWebhookDLQ: %v", err)
	}
	if deletedDLQ < 1 {
		t.Fatalf("expected at least 1 DLQ deletion, got %d", deletedDLQ)
	}

	var oldDLQGone, newDLQKept int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM webhook_deliveries_dlq WHERE id = $1`, oldDLQ,
	).Scan(&oldDLQGone); err != nil {
		t.Fatalf("count old DLQ: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM webhook_deliveries_dlq WHERE id = $1`, newDLQ,
	).Scan(&newDLQKept); err != nil {
		t.Fatalf("count new DLQ: %v", err)
	}
	if oldDLQGone != 0 {
		t.Fatal("old DLQ row should have been deleted")
	}
	if newDLQKept != 1 {
		t.Fatal("new DLQ row should have been preserved")
	}
}

// TestListWebhooksByEvent_SkipsDisabled asserts that a tripped subscription
// is invisible to the event dispatcher's lookup query — same defense in
// depth as the deliver()-level `if sub.DisabledAt.Valid` check.
func TestListWebhooksByEvent_SkipsDisabled(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	fix := setupWebhookFixture(t, pool, "https://example.invalid/hook")
	q := fix.queries
	ctx := context.Background()

	// Baseline: subscription is listable.
	subs, err := q.ListWebhooksByEvent(ctx, dbgen.ListWebhooksByEventParams{
		TenantID: fix.tenantID, Column2: "asset.created",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(subs))
	}

	// Trip it.
	if err := q.DisableWebhook(ctx, fix.subID); err != nil {
		t.Fatalf("DisableWebhook: %v", err)
	}

	subs, err = q.ListWebhooksByEvent(ctx, dbgen.ListWebhooksByEventParams{
		TenantID: fix.tenantID, Column2: "asset.created",
	})
	if err != nil {
		t.Fatalf("list after disable: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("disabled sub must not appear in ListWebhooksByEvent, got %d", len(subs))
	}
}

// helper: unused import silencer (fmt is imported for future use)
var _ = fmt.Sprintf

// startSrv returns a test server and a clean shutdown hook for use from
// tests that only need a responding endpoint, so test bodies stay terse.
// (Left as a small helper in case more tests land.)
func startSrv(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = time.Now
		w.WriteHeader(status)
	}))
}

var _ = startSrv
