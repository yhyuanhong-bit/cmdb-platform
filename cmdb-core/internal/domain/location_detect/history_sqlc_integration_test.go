//go:build integration

package location_detect

import (
	"context"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Post-sqlc migration coverage for asset_location_history. The table
// has a first-class tenant_id column, and every query in this file
// except GetLocationHistory enforces it. These tests pin that:
//
//  1. RecordLocationChange stamps the caller's tenant_id.
//  2. CountRelocationsSince24h and DetectFrequentRelocations never
//     count rows from another tenant, even when a noisy neighbor has
//     a burst of history rows in the same window.
//  3. GetLocationHistory is deliberately scoped by asset_id only
//     (pre-migration behavior) — documented here so an accidental
//     future change that adds tenant_id surfaces as a test break.
//
// Run with:
//   go test -tags integration -run TestIntegration_AssetLocationHistory ./internal/domain/location_detect/...
//
// testDBURL() and newTestPool() live in detector_workorderlog_integration_test.go
// (same package, same build tag).

type alhFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID
	assetA  uuid.UUID
	assetB  uuid.UUID
}

// setupALHFixture creates two tenants, one asset in each, and inserts
// 5 relocation history rows for tenantB's asset (all within 24h) as a
// noisy-neighbor. tenantA's asset starts with zero history rows so we
// can assert that its RecordLocationChange calls are the only things
// counted.
func setupALHFixture(t *testing.T, pool *pgxpool.Pool) alhFixture {
	t.Helper()
	ctx := context.Background()
	fix := alhFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		assetA:  uuid.New(),
		assetB:  uuid.New(),
	}

	suffA := fix.tenantA.String()[:8]
	suffB := fix.tenantB.String()[:8]

	for _, tu := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.tenantA, "alh-a-" + suffA},
		{fix.tenantB, "alh-b-" + suffB},
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
			tu.id, tu.name, tu.name); err != nil {
			t.Fatalf("insert tenant %s: %v", tu.name, err)
		}
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type)
		 VALUES ($1, $2, $3, 'asset A', 'server'),
		        ($4, $5, $6, 'asset B', 'server')`,
		fix.assetA, fix.tenantA, "ALH-A-"+suffA,
		fix.assetB, fix.tenantB, "ALH-B-"+suffB,
	); err != nil {
		t.Fatalf("insert assets: %v", err)
	}

	// Five history rows for tenantB's asset in the last 24h — this is
	// the noisy neighbor whose rows must NOT leak into tenantA queries.
	// Also enough (>=3) to trip DetectFrequentRelocations if tenant
	// scoping were broken.
	for range 5 {
		if _, err := pool.Exec(ctx,
			`INSERT INTO asset_location_history (tenant_id, asset_id, detected_by)
			 VALUES ($1, $2, 'snmp_auto')`,
			fix.tenantB, fix.assetB); err != nil {
			t.Fatalf("insert noisy-neighbor history: %v", err)
		}
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM asset_location_history WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

// TestIntegration_AssetLocationHistory_RecordAndCount_TenantScoped
// exercises the full RecordLocationChange -> CountRelocationsSince24h
// loop for tenantA and asserts tenantB's 5 noisy-neighbor rows never
// appear in tenantA's count.
func TestIntegration_AssetLocationHistory_RecordAndCount_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupALHFixture(t, pool)

	ctx := context.Background()
	q := dbgen.New(pool)

	// Record two relocations for tenantA's asset. Both use NULL rack
	// IDs — the test doesn't need real rack FKs, and nil-safe
	// pgtype.UUID is the common case in the auto-confirm path (where
	// the detector has already confirmed the new rack and writes via
	// a separate UPDATE).
	for range 2 {
		if err := q.RecordLocationChange(ctx, dbgen.RecordLocationChangeParams{
			TenantID:    fix.tenantA,
			AssetID:     fix.assetA,
			FromRackID:  pgtype.UUID{},
			ToRackID:    pgtype.UUID{},
			DetectedBy:  "snmp_auto",
			WorkOrderID: pgtype.UUID{},
		}); err != nil {
			t.Fatalf("RecordLocationChange: %v", err)
		}
	}

	countA, err := q.CountRelocationsSince24h(ctx, fix.tenantA)
	if err != nil {
		t.Fatalf("CountRelocationsSince24h tenantA: %v", err)
	}
	if countA != 2 {
		t.Errorf("tenantA count = %d, want 2 (noisy-neighbor tenantB leak?)", countA)
	}

	countB, err := q.CountRelocationsSince24h(ctx, fix.tenantB)
	if err != nil {
		t.Fatalf("CountRelocationsSince24h tenantB: %v", err)
	}
	if countB != 5 {
		t.Errorf("tenantB count = %d, want 5", countB)
	}
}

// TestIntegration_AssetLocationHistory_DetectFrequent_TenantScoped
// verifies that tenantA sees zero frequent-relocation anomalies even
// when tenantB has 5 moves in the 30-day window. The tenant_id WHERE
// clause is the only thing stopping a cross-tenant alert storm.
func TestIntegration_AssetLocationHistory_DetectFrequent_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupALHFixture(t, pool)

	ctx := context.Background()
	q := dbgen.New(pool)

	rows, err := q.DetectFrequentRelocations(ctx, fix.tenantA)
	if err != nil {
		t.Fatalf("DetectFrequentRelocations tenantA: %v", err)
	}
	for _, r := range rows {
		if r.AssetID == fix.assetB {
			t.Errorf("tenantA saw tenantB's asset %s in frequent-relocation list — tenant scope leak!", r.AssetID)
		}
	}

	rowsB, err := q.DetectFrequentRelocations(ctx, fix.tenantB)
	if err != nil {
		t.Fatalf("DetectFrequentRelocations tenantB: %v", err)
	}
	var seenB bool
	for _, r := range rowsB {
		if r.AssetID == fix.assetB {
			seenB = true
			if r.MoveCount != 5 {
				t.Errorf("tenantB assetB move_count = %d, want 5", r.MoveCount)
			}
		}
	}
	if !seenB {
		t.Errorf("tenantB did not see its own assetB in frequent-relocation list — rows=%+v", rowsB)
	}
}

// TestIntegration_AssetLocationHistory_GetHistory_AssetScoped pins
// the deliberate pre-migration behavior: GetLocationHistory filters by
// asset_id only. If a future sqlc edit adds a tenant_id clause, this
// test fails — either because the row disappears, or because the
// generated signature changes. This is cross-tenant: on purpose.
func TestIntegration_AssetLocationHistory_GetHistory_AssetScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupALHFixture(t, pool)

	ctx := context.Background()
	q := dbgen.New(pool)

	// Seed a single tenantA history row so GetLocationHistory has
	// something to return.
	if err := q.RecordLocationChange(ctx, dbgen.RecordLocationChangeParams{
		TenantID:    fix.tenantA,
		AssetID:     fix.assetA,
		FromRackID:  pgtype.UUID{},
		ToRackID:    pgtype.UUID{},
		DetectedBy:  "snmp_auto",
		WorkOrderID: pgtype.UUID{},
	}); err != nil {
		t.Fatalf("seed RecordLocationChange: %v", err)
	}

	rows, err := q.GetLocationHistory(ctx, dbgen.GetLocationHistoryParams{
		AssetID: fix.assetA,
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("GetLocationHistory: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("GetLocationHistory returned 0 rows for assetA — did a future edit add a tenant filter?")
	}
	if rows[0].DetectedBy != "snmp_auto" {
		t.Errorf("rows[0].DetectedBy = %q, want snmp_auto", rows[0].DetectedBy)
	}
	// detected_at round-trip (server-side now())
	if !rows[0].DetectedAt.Valid || time.Since(rows[0].DetectedAt.Time) > 5*time.Minute {
		t.Errorf("rows[0].DetectedAt = %+v, looks wrong", rows[0].DetectedAt)
	}
}
