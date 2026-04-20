//go:build integration

package workflows

import (
	"context"
	"fmt"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Phase 2.15 integration tests: the work_order_dedup table replaces the
// old `description LIKE '%IP:%' / '%Serial:%'` probe. These tests lock
// down the four behaviors that the prior LIKE-hack silently got wrong
// or did not guarantee at all:
//
//  1. Same tenant + same IP scanned twice → exactly 1 WO across two
//     scan runs (idempotent dedup, was previously "probably 1 if the
//     description prose didn't vary").
//  2. Same IP in two tenants → 1 WO per tenant (cross-tenant isolation,
//     enforced by the compound PK (tenant_id, dedup_kind, dedup_key)).
//  3. Pre-existing backfilled dedup row → re-scan emits zero WOs.
//     This is the migration-compat guarantee: historical WOs produced
//     by the old code path get backfilled into work_order_dedup and
//     the new code must honor them.
//  4. Lost race (two writers land on the same key): the second INSERT
//     hits ON CONFLICT DO NOTHING, gets RowsAffected=0, and the caller
//     rolls back so no orphan WO lands.
//
// Run with:
//
//	go test -tags integration -race ./internal/domain/workflows/... -run Dedup

// countShadowITDedup counts work_order_dedup rows for a tenant's
// shadow_it scan. Used to prove the table is the source of truth.
func countShadowITDedup(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM work_order_dedup
		 WHERE tenant_id = $1 AND dedup_kind = 'shadow_it'`,
		tenantID).Scan(&n)
	if err != nil {
		t.Fatalf("count shadow_it dedup rows: %v", err)
	}
	return n
}

// TestShadowITDedup_SameTenantSameIPSingleWO: the original LIKE-hack
// was only "mostly" idempotent — string formatting drift between
// scanner versions could cause the LIKE pattern to miss a prior WO.
// The dedup table makes this a hard guarantee: scan, rescan → 1 WO.
func TestShadowITDedup_SameTenantSameIPSingleWO(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	w := newTestSubscriber(t, pool)
	ctx := context.Background()

	fix := setupTenant(t, pool, "dedup-same-ip")
	ip := fmt.Sprintf("10.66.%d.10", uniqueOctet(fix.tenantID))
	seedShadowITDiscovery(t, pool, fix.tenantID, "rogue-repeat", ip)

	// First scan: creates 1 WO + 1 dedup row.
	w.checkShadowIT(ctx)

	if got := countShadowITWOs(t, pool, fix.tenantID); got != 1 {
		t.Fatalf("after first scan: WO count = %d, want 1", got)
	}
	if got := countShadowITDedup(t, pool, fix.tenantID); got != 1 {
		t.Fatalf("after first scan: dedup row count = %d, want 1", got)
	}

	// Second scan on the unchanged discovered_asset: the NOT EXISTS
	// against work_order_dedup must suppress it. WO count stays at 1.
	w.checkShadowIT(ctx)

	if got := countShadowITWOs(t, pool, fix.tenantID); got != 1 {
		t.Errorf("after second scan: WO count = %d, want 1 (dedup failed)", got)
	}
	if got := countShadowITDedup(t, pool, fix.tenantID); got != 1 {
		t.Errorf("after second scan: dedup row count = %d, want 1", got)
	}
}

// TestShadowITDedup_CrossTenantIsolation: two different tenants each
// own a device at the SAME IP (10.0.0.5 is a valid private-range IP
// in every tenant's world). Each tenant must get its own WO. Under
// the old LIKE-hack this was only enforced indirectly via the outer
// tenant_id filter on work_orders; the compound PK
// (tenant_id, dedup_kind, dedup_key) now makes it structural.
func TestShadowITDedup_CrossTenantIsolation(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	w := newTestSubscriber(t, pool)
	ctx := context.Background()

	fixA := setupTenant(t, pool, "dedup-iso-a")
	fixB := setupTenant(t, pool, "dedup-iso-b")

	// IDENTICAL IP in both tenants — this is the crux of the test.
	sharedIP := fmt.Sprintf("10.55.%d.100", uniqueOctet(fixA.tenantID))
	seedShadowITDiscovery(t, pool, fixA.tenantID, "device-in-a", sharedIP)
	seedShadowITDiscovery(t, pool, fixB.tenantID, "device-in-b", sharedIP)

	w.checkShadowIT(ctx)

	if got := countShadowITWOs(t, pool, fixA.tenantID); got != 1 {
		t.Errorf("tenant A: WO count = %d, want 1 (same-IP-in-other-tenant must NOT suppress)", got)
	}
	if got := countShadowITWOs(t, pool, fixB.tenantID); got != 1 {
		t.Errorf("tenant B: WO count = %d, want 1 (same-IP-in-other-tenant must NOT suppress)", got)
	}
	if got := countShadowITDedup(t, pool, fixA.tenantID); got != 1 {
		t.Errorf("tenant A: dedup rows = %d, want 1", got)
	}
	if got := countShadowITDedup(t, pool, fixB.tenantID); got != 1 {
		t.Errorf("tenant B: dedup rows = %d, want 1", got)
	}

	// Same scan run again: both tenants' dedup rows now suppress
	// the re-emission. Still 1 WO per tenant.
	w.checkShadowIT(ctx)

	if got := countShadowITWOs(t, pool, fixA.tenantID); got != 1 {
		t.Errorf("tenant A after rescan: WO count = %d, want 1", got)
	}
	if got := countShadowITWOs(t, pool, fixB.tenantID); got != 1 {
		t.Errorf("tenant B after rescan: WO count = %d, want 1", got)
	}
}

// TestShadowITDedup_BackfilledHistoricalRowSuppresses: the migration
// backfill parses the IP out of the historical WO description and
// writes a dedup row. Here we simulate that exact state: a pre-
// existing (tenant, 'shadow_it', ip) row in work_order_dedup but NO
// matching WO reachable by our scan (e.g. the WO was closed, or the
// backfill parsed it from an archived WO). The scan must still see
// the dedup row and NOT emit a new WO.
//
// Without this behavior, every deployment that runs the migration
// would immediately double-emit WOs for every historical discovery.
func TestShadowITDedup_BackfilledHistoricalRowSuppresses(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	w := newTestSubscriber(t, pool)
	ctx := context.Background()

	fix := setupTenant(t, pool, "dedup-backfill")
	ip := fmt.Sprintf("10.44.%d.200", uniqueOctet(fix.tenantID))
	seedShadowITDiscovery(t, pool, fix.tenantID, "historical-host", ip)

	// Seed the state that the migration backfill would leave behind:
	// a WO from a prior scanner run AND its matching dedup row.
	// `code` is NOT NULL with no default, so we generate one — real
	// code is maintenance.Service.nextCode(), but any unique string
	// suffices for the test fixture.
	historicalCode := "WO-HIST-" + uuid.NewString()[:8]
	var historicalWOID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO work_orders (tenant_id, code, title, type, priority, status, description)
		 VALUES ($1, $2, 'historical shadow IT', 'shadow_it_registration', 'medium', 'submitted',
		         'Legacy description referencing IP: '||$3||' for backfill.')
		 RETURNING id`,
		fix.tenantID, historicalCode, ip).Scan(&historicalWOID)
	if err != nil {
		t.Fatalf("seed historical WO: %v", err)
	}
	queries := dbgen.New(pool)
	n, err := queries.InsertWorkOrderDedup(ctx, dbgen.InsertWorkOrderDedupParams{
		TenantID:    fix.tenantID,
		WorkOrderID: historicalWOID,
		DedupKind:   "shadow_it",
		DedupKey:    ip,
	})
	if err != nil {
		t.Fatalf("seed historical dedup row: %v", err)
	}
	if n != 1 {
		t.Fatalf("seed historical dedup: RowsAffected = %d, want 1", n)
	}

	// Now scan. The discovered_asset is still pending/unmatched and >7
	// days old, so it WOULD emit a WO if dedup weren't in play. Because
	// the backfill row exists, scan must produce zero new WOs.
	w.checkShadowIT(ctx)

	if got := countShadowITWOs(t, pool, fix.tenantID); got != 1 {
		// 1 == just the historical one; anything higher means we
		// double-emitted and the backfill contract is broken.
		t.Errorf("WO count after scan = %d, want 1 (only the pre-seeded historical WO)", got)
	}
	if got := countShadowITDedup(t, pool, fix.tenantID); got != 1 {
		t.Errorf("dedup row count after scan = %d, want 1", got)
	}
}

// TestShadowITDedup_RaceLosesRollback: simulates two scanners landing
// on the same (tenant, ip) concurrently. Scanner A wins: inserts WO
// + dedup row. Scanner B loses: its dedup INSERT ... ON CONFLICT DO
// NOTHING returns RowsAffected=0, and createShadowITWorkOrder rolls
// back its own WO insert so no orphan row lands in work_orders.
//
// We drive the race deterministically by manually inserting the
// winning dedup row first (simulating scanner A having already
// committed), then calling the loser helper directly. It must return
// an error AND leave work_orders unchanged.
func TestShadowITDedup_RaceLosesRollback(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	w := newTestSubscriber(t, pool)
	ctx := context.Background()

	fix := setupTenant(t, pool, "dedup-race")
	ip := fmt.Sprintf("10.33.%d.42", uniqueOctet(fix.tenantID))

	// Seed the "scanner A already won" state: an existing WO + dedup
	// row for (tenant, shadow_it, ip). Scanner B about to fire has
	// already passed its NOT EXISTS pre-check (stale read).
	winnerCode := "WO-WIN-" + uuid.NewString()[:8]
	var winnerWOID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO work_orders (tenant_id, code, title, type, priority, status, description)
		 VALUES ($1, $2, 'scanner A winner', 'shadow_it_registration', 'medium', 'submitted', 'Winner WO')
		 RETURNING id`,
		fix.tenantID, winnerCode).Scan(&winnerWOID)
	if err != nil {
		t.Fatalf("seed winner WO: %v", err)
	}
	queries := dbgen.New(pool)
	if _, err := queries.InsertWorkOrderDedup(ctx, dbgen.InsertWorkOrderDedupParams{
		TenantID:    fix.tenantID,
		WorkOrderID: winnerWOID,
		DedupKind:   "shadow_it",
		DedupKey:    ip,
	}); err != nil {
		t.Fatalf("seed winner dedup row: %v", err)
	}

	// The WO table currently has exactly 1 row for this tenant.
	before := countShadowITWOs(t, pool, fix.tenantID)
	if before != 1 {
		t.Fatalf("pre-race WO count = %d, want 1", before)
	}

	// Scanner B's create attempt: should error out on RowsAffected=0
	// from the dedup insert, and roll its tx back.
	err = w.createShadowITWorkOrder(ctx, fix.tenantID, "loser-host", ip, "test-scan", 10)
	if err == nil {
		t.Fatal("createShadowITWorkOrder: expected race error, got nil")
	}

	// CRITICAL: WO count must NOT have grown. If it did, the tx wasn't
	// rolled back and we have an orphan WO with no matching dedup row.
	after := countShadowITWOs(t, pool, fix.tenantID)
	if after != before {
		t.Errorf("post-race WO count = %d, want %d (race loser leaked an orphan WO)", after, before)
	}
	if got := countShadowITDedup(t, pool, fix.tenantID); got != 1 {
		t.Errorf("post-race dedup row count = %d, want 1 (uniqueness violated)", got)
	}
}
