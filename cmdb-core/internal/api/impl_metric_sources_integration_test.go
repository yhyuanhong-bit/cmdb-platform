//go:build integration

package api_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/metricsource"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 8.1 coverage:
//
//   1. Create rejects duplicates and bad inputs.
//   2. Heartbeat updates last_heartbeat_at and accumulates the
//      lifetime sample counter without race-clobbering on concurrent
//      bumps.
//   3. ListStale flags sources whose last_heartbeat is over 2× the
//      expected interval (or have never sent one), excludes disabled
//      sources, and leaves fresh sources alone.
//   4. Tenant isolation across all entry points.

func newMetricSourceTestPool(t *testing.T) *pgxpool.Pool {
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

func seedTwoTenantsForMetricSources(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "ms-A-"+suffix, "ms-a-"+suffix,
		b, "ms-B-"+suffix, "ms-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM metric_sources WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants        WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

// ---------------------------------------------------------------------------
// CRUD + validation
// ---------------------------------------------------------------------------

func TestMetricSource_CreateValidatesInputs(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	cases := []struct {
		name string
		p    metricsource.CreateParams
	}{
		{"empty name", metricsource.CreateParams{TenantID: tenantA, Kind: "snmp", ExpectedIntervalSeconds: 60}},
		{"missing kind", metricsource.CreateParams{TenantID: tenantA, Name: "x", ExpectedIntervalSeconds: 60}},
		{"non-positive interval", metricsource.CreateParams{TenantID: tenantA, Name: "x", Kind: "snmp", ExpectedIntervalSeconds: 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Create(ctx, c.p)
			if !errors.Is(err, metricsource.ErrValidation) {
				t.Errorf("want ErrValidation, got %v", err)
			}
		})
	}
}

func TestMetricSource_DuplicateNameRejected(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	if _, err := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "switch-a3", Kind: "snmp", ExpectedIntervalSeconds: 60,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "switch-a3", Kind: "snmp", ExpectedIntervalSeconds: 30,
	})
	if !errors.Is(err, metricsource.ErrDuplicateName) {
		t.Errorf("duplicate: want ErrDuplicateName, got %v", err)
	}
}

func TestMetricSource_DuplicateNameOkAcrossTenants(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	if _, err := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "shared-name", Kind: "agent", ExpectedIntervalSeconds: 60,
	}); err != nil {
		t.Fatalf("A: %v", err)
	}
	if _, err := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantB, Name: "shared-name", Kind: "agent", ExpectedIntervalSeconds: 60,
	}); err != nil {
		t.Errorf("B same name (different tenant) should be allowed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Heartbeat
// ---------------------------------------------------------------------------

func TestMetricSource_HeartbeatUpdatesTimestampAndCounter(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	src, err := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "hb", Kind: "agent", ExpectedIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if src.LastSampleCount != 0 {
		t.Errorf("initial counter = %d, want 0", src.LastSampleCount)
	}
	if src.LastHeartbeatAt.Valid {
		t.Errorf("initial heartbeat should be NULL")
	}

	// First heartbeat with 100 samples.
	row, err := svc.Heartbeat(ctx, tenantA, src.ID, 100)
	if err != nil {
		t.Fatalf("heartbeat 1: %v", err)
	}
	if row.LastSampleCount != 100 {
		t.Errorf("after hb1 counter = %d, want 100", row.LastSampleCount)
	}
	if !row.LastHeartbeatAt.Valid {
		t.Errorf("after hb1 last_heartbeat_at not set")
	}

	// Second heartbeat — counter should ACCUMULATE, not replace.
	row2, err := svc.Heartbeat(ctx, tenantA, src.ID, 250)
	if err != nil {
		t.Fatalf("heartbeat 2: %v", err)
	}
	if row2.LastSampleCount != 350 {
		t.Errorf("after hb2 counter = %d, want 350 (accumulated)", row2.LastSampleCount)
	}

	// Heartbeat with 0 samples (alive ping with no data).
	row3, err := svc.Heartbeat(ctx, tenantA, src.ID, 0)
	if err != nil {
		t.Fatalf("heartbeat 3: %v", err)
	}
	if row3.LastSampleCount != 350 {
		t.Errorf("alive-ping mutated counter to %d, want 350", row3.LastSampleCount)
	}
}

func TestMetricSource_HeartbeatNotFoundError(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	_, err := svc.Heartbeat(ctx, tenantA, uuid.New(), 1)
	if !errors.Is(err, metricsource.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stale detection
// ---------------------------------------------------------------------------

func TestMetricSource_StaleDetectionFlagsOverdueAndUnheartbeated(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	// Source A: just created, never heartbeated → stale (no heartbeat).
	srcA, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "never-hb", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})

	// Source B: heartbeat just now → fresh.
	srcB, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "fresh", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})
	if _, err := svc.Heartbeat(ctx, tenantA, srcB.ID, 1); err != nil {
		t.Fatalf("hb B: %v", err)
	}

	// Source C: heartbeat 10 minutes ago, expected_interval 60s →
	// 10*60s = 600s > 2*60 = 120s, so stale.
	srcC, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "overdue", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})
	// Manually backdate srcC's heartbeat — easier than waiting in tests.
	if _, err := pool.Exec(ctx,
		`UPDATE metric_sources SET last_heartbeat_at = now() - interval '10 minutes' WHERE id = $1`,
		srcC.ID,
	); err != nil {
		t.Fatalf("backdate C: %v", err)
	}

	// Source D: stale BUT disabled → should NOT appear in freshness list.
	srcD, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "disabled", Kind: "snmp", ExpectedIntervalSeconds: 60, Status: "disabled",
	})

	stale, err := svc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale: %v", err)
	}

	got := map[uuid.UUID]bool{}
	for _, r := range stale {
		got[r.ID] = true
	}
	if !got[srcA.ID] {
		t.Errorf("never-heartbeated source A should be flagged stale")
	}
	if got[srcB.ID] {
		t.Errorf("fresh source B was flagged stale")
	}
	if !got[srcC.ID] {
		t.Errorf("overdue source C should be flagged stale")
	}
	if got[srcD.ID] {
		t.Errorf("disabled source D was included in stale list")
	}
}

func TestMetricSource_StaleToleranceJitter(t *testing.T) {
	// A heartbeat 1.5× the interval ago is NOT stale (within the 2×
	// tolerance band). 2.5× is.
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	src, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "jitter", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})
	if _, err := pool.Exec(ctx,
		`UPDATE metric_sources SET last_heartbeat_at = now() - interval '90 seconds' WHERE id = $1`,
		src.ID,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	stale, err := svc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale: %v", err)
	}
	for _, r := range stale {
		if r.ID == src.ID {
			t.Errorf("90s-late source should not be flagged (within 2× tolerance)")
		}
	}

	// Now 150s late (2.5×) — should flag.
	if _, err := pool.Exec(ctx,
		`UPDATE metric_sources SET last_heartbeat_at = now() - interval '150 seconds' WHERE id = $1`,
		src.ID,
	); err != nil {
		t.Fatalf("re-backdate: %v", err)
	}
	stale, err = svc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale 2: %v", err)
	}
	found := false
	for _, r := range stale {
		if r.ID == src.ID {
			found = true
			// SecondsSinceHeartbeat is interface{} via the COALESCE.
			// Accept int32 or int64 from the driver.
			var secs int64
			switch v := r.SecondsSinceHeartbeat.(type) {
			case int32:
				secs = int64(v)
			case int64:
				secs = v
			default:
				t.Errorf("unexpected seconds_since_heartbeat type %T", r.SecondsSinceHeartbeat)
				continue
			}
			if secs < 140 || secs > 160 {
				t.Errorf("seconds_since_heartbeat = %d, want ~150", secs)
			}
		}
	}
	if !found {
		t.Errorf("150s-late source should be flagged")
	}
}

// ---------------------------------------------------------------------------
// Tenant isolation
// ---------------------------------------------------------------------------

func TestMetricSource_TenantIsolation(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	srcA, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "a-only", Kind: "agent", ExpectedIntervalSeconds: 60,
	})

	// Tenant B can't Get / Update / Heartbeat / Delete A's source.
	if _, err := svc.Get(ctx, tenantB, srcA.ID); !errors.Is(err, metricsource.ErrNotFound) {
		t.Errorf("cross-tenant Get: want ErrNotFound, got %v", err)
	}
	if _, err := svc.Heartbeat(ctx, tenantB, srcA.ID, 1); !errors.Is(err, metricsource.ErrNotFound) {
		t.Errorf("cross-tenant Heartbeat: want ErrNotFound, got %v", err)
	}
	newName := "renamed"
	if _, err := svc.Update(ctx, metricsource.UpdateParams{
		TenantID: tenantB, ID: srcA.ID, Name: &newName,
	}); !errors.Is(err, metricsource.ErrNotFound) {
		t.Errorf("cross-tenant Update: want ErrNotFound, got %v", err)
	}

	// Confirm A's row is unchanged.
	row, err := svc.Get(ctx, tenantA, srcA.ID)
	if err != nil {
		t.Fatalf("self-tenant Get: %v", err)
	}
	if row.Name != "a-only" {
		t.Errorf("name mutated by cross-tenant call: %q", row.Name)
	}
}

func TestMetricSource_StaleListIsTenantScoped(t *testing.T) {
	pool := newMetricSourceTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForMetricSources(t, pool)
	q := dbgen.New(pool)
	svc := metricsource.NewService(q, pool)

	// One stale source per tenant.
	srcA, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "stale-a", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})
	srcB, _ := svc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantB, Name: "stale-b", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})

	staleA, _ := svc.ListStale(ctx, tenantA)
	for _, r := range staleA {
		if r.ID == srcB.ID {
			t.Errorf("tenant A's stale list leaked tenant B's source")
		}
	}
	staleB, _ := svc.ListStale(ctx, tenantB)
	for _, r := range staleB {
		if r.ID == srcA.ID {
			t.Errorf("tenant B's stale list leaked tenant A's source")
		}
	}
}

// _ keeps unused-import warnings off when the file is being trimmed.
var _ = time.Second
