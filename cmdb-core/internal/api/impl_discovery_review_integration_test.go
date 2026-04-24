//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 3 review-gate regression tests. Three classes of regression we
// want to catch automatically:
//
//  1. Cross-tenant Ignore — pre-3 the SQL was id-only, so a caller in
//     tenant B could reject tenant A's discovery queue by guessing the
//     UUID. The fix was a tenant-scoped UPDATE + domain-level tenant
//     check; the first test here enforces the contract.
//
//  2. review_reason requirement — approve/ignore must carry a reason so
//     the audit trail answers "who decided what, and why". Ignoring
//     without a reason must return ErrReviewReasonRequired at the
//     domain layer.
//
//  3. 24h overdue query — the governance scheduler reads
//     ListUnreviewedOverdue and opens a WO per row. Verify the query
//     filters correctly (status + age threshold).

func newDiscoveryTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

// Bus stub — unit tests don't need NATS; the service publish path is a
// no-op when the bus is nil. We still instantiate the recording variant
// so tests that care about event fan-out can inspect .events.
type recorderBus struct{ events []eventbus.Event }

func (r *recorderBus) Publish(_ context.Context, ev eventbus.Event) error {
	r.events = append(r.events, ev)
	return nil
}
func (r *recorderBus) Subscribe(_ string, _ eventbus.Handler) error { return nil }
func (r *recorderBus) Close() error                                 { return nil }

func seedTwoTenantsForDiscovery(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "disc-A-"+suffix, "disc-a-"+suffix,
		b, "disc-B-"+suffix, "disc-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM discovered_assets WHERE tenant_id IN ($1, $2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, a, b)
	})
	return a, b
}

func insertDiscovery(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, source, status string, discoveredAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	extID := "test-" + id.String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO discovered_assets (id, tenant_id, source, external_id, hostname, ip_address, raw_data, status, discovered_at)
		 VALUES ($1, $2, $3, $4, 'h1', '192.0.2.1', '{}', $5, $6)`,
		id, tenantID, source, extID, status, discoveredAt,
	); err != nil {
		t.Fatalf("insert discovery: %v", err)
	}
	return id
}

func TestDiscoveryReview_IgnoreDoesNotCrossTenant(t *testing.T) {
	pool := newDiscoveryTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForDiscovery(t, pool)
	q := dbgen.New(pool)
	svc := discovery.NewService(q, pool)

	// tenant A owns a pending discovery. tenant B tries to reject it.
	discID := insertDiscovery(t, pool, tenantA, "snmp", "pending", time.Now())
	reviewer := uuid.New() // doesn't need to exist in users since we skip FK on testing DB

	_, err := svc.Ignore(ctx, discID, tenantB, reviewer, "not ours")
	if err == nil {
		t.Fatalf("Ignore across tenants must fail; got success")
	}

	// Confirm A's row was not mutated.
	var status, reason string
	if err := pool.QueryRow(ctx,
		`SELECT status, COALESCE(review_reason, '') FROM discovered_assets WHERE id = $1`, discID,
	).Scan(&status, &reason); err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if status != "pending" {
		t.Errorf("status = %q, want pending (cross-tenant ignore leaked)", status)
	}
	if reason != "" {
		t.Errorf("review_reason = %q, want empty", reason)
	}
}

func TestDiscoveryReview_IgnoreRequiresReason(t *testing.T) {
	pool := newDiscoveryTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForDiscovery(t, pool)
	q := dbgen.New(pool)
	svc := discovery.NewService(q, pool)

	discID := insertDiscovery(t, pool, tenantA, "snmp", "pending", time.Now())
	reviewer := uuid.New()

	_, err := svc.Ignore(ctx, discID, tenantA, reviewer, "")
	if err == nil {
		t.Fatalf("Ignore with empty reason must fail; got success")
	}
}

func TestDiscoveryReview_IgnoreWritesAudit(t *testing.T) {
	pool := newDiscoveryTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForDiscovery(t, pool)
	q := dbgen.New(pool)
	svc := discovery.NewService(q, pool)

	// The audit_events FK on operator_id needs the user to exist. Insert
	// one for this tenant (system user from 000052 already exists for
	// seeded tenants, but the two we just created are fresh).
	reviewer := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source)
		 VALUES ($1, $2, 'reviewer-'||$3, 'Reviewer', 'r@example.com', '!', 'active', 'test')`,
		reviewer, tenantA, reviewer.String()[:8],
	); err != nil {
		t.Fatalf("seed reviewer user: %v", err)
	}

	discID := insertDiscovery(t, pool, tenantA, "snmp", "pending", time.Now())
	if _, err := svc.Ignore(ctx, discID, tenantA, reviewer, "false positive — test IP"); err != nil {
		t.Fatalf("Ignore: %v", err)
	}

	// Audit event must exist with action=discovery.ignored and the
	// reason embedded in the diff JSON.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events
		 WHERE tenant_id = $1 AND action = 'discovery.ignored'
		   AND target_id = $2 AND diff::text LIKE '%false positive%'`,
		tenantA, discID,
	).Scan(&count); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if count != 1 {
		t.Errorf("audit events for Ignore = %d, want 1", count)
	}

	// review_reason must be persisted on the row too.
	var reason pgtype.Text
	_ = pool.QueryRow(ctx,
		`SELECT review_reason FROM discovered_assets WHERE id = $1`, discID,
	).Scan(&reason)
	if !reason.Valid || reason.String == "" {
		t.Errorf("review_reason not persisted on row")
	}
}

func TestDiscoveryReview_UnreviewedOverdueQuery(t *testing.T) {
	pool := newDiscoveryTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForDiscovery(t, pool)

	// Seed 3 rows: two old-enough-to-be-overdue, one fresh.
	oldEnough := time.Now().Add(-48 * time.Hour)
	fresh := time.Now().Add(-1 * time.Hour)
	oldPending := insertDiscovery(t, pool, tenantA, "snmp", "pending", oldEnough)
	oldConflict := insertDiscovery(t, pool, tenantA, "ssh", "conflict", oldEnough)
	_ = insertDiscovery(t, pool, tenantA, "ipmi", "pending", fresh)

	q := dbgen.New(pool)
	// Limit big enough to survive whatever seeded fixtures already
	// live in the DB — with Limit=10 the ORDER BY discovered_at ASC
	// returns the OLDEST rows, which are seed fixtures from months
	// earlier, not the two rows we just inserted.
	rows, err := q.ListUnreviewedOverdue(ctx, dbgen.ListUnreviewedOverdueParams{
		Hours: 24,
		Limit: 5000,
	})
	if err != nil {
		t.Fatalf("ListUnreviewedOverdue: %v", err)
	}

	// Filter to our test tenant so other fixture data does not spoil
	// the assertion.
	var ids []uuid.UUID
	for _, r := range rows {
		if r.TenantID == tenantA {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("overdue count = %d, want 2 — got %v (rows=%d tenantA=%s)", len(ids), ids, len(rows), tenantA)
	}
	want := map[uuid.UUID]bool{oldPending: true, oldConflict: true}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected id %s in overdue set", id)
		}
	}
}
