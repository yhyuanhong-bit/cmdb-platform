//go:build integration

package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/energy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 6.2 coverage. Three concerns:
//
//   1. PUE rollup math — IT vs non-IT split is correct, PUE column is
//      total/IT, NULL when IT is zero.
//   2. Anomaly detector flags high/low correctly against the trailing
//      median; respects MinSampleCount; idempotent re-run preserves
//      operator-set status.
//   3. Tenant isolation across both rollup tables.

func seedTwoTenantsForEnergyPhase2(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "p2-A-"+suffix, "p2-a-"+suffix,
		b, "p2-B-"+suffix, "p2-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM energy_anomalies      WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM energy_location_daily WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM energy_daily_kwh      WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM energy_tariffs        WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM metrics               WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM assets                WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants               WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

// seedAssetTyped is like seedAssetForEnergy but lets us pick the asset
// type so we can put both IT and non-IT assets in a single location.
func seedAssetTyped(t *testing.T, pool *pgxpool.Pool, tenantID, locationID uuid.UUID, assetType, subType string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	tag := assetType[:3] + "-" + id.String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, location_id)
		 VALUES ($1, $2, $3, $3, $4, $5, 'operational', $6)`,
		id, tenantID, tag, assetType, subType, locationID,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

// upsertDailyKwh writes/overwrites a row in energy_daily_kwh directly.
// Useful for setting up a baseline window of historic days without
// having to insert N×24 metric rows.
func upsertDailyKwh(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID, day time.Time, kwh float64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO energy_daily_kwh (tenant_id, asset_id, day, kwh_total, kw_peak, kw_avg, sample_count)
		VALUES ($1, $2, $3, $4, $5, $6, 24)
		ON CONFLICT (tenant_id, asset_id, day) DO UPDATE SET
			kwh_total = EXCLUDED.kwh_total,
			kw_peak   = EXCLUDED.kw_peak,
			kw_avg    = EXCLUDED.kw_avg,
			computed_at = now()
	`,
		tenantID, assetID, day, kwh, kwh/24+0.1, kwh/24,
	); err != nil {
		t.Fatalf("upsert daily kwh: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PUE rollup
// ---------------------------------------------------------------------------

func TestEnergyPhase2_PueMathSplitsItAndNonIt(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "IDC-PUE", "idc")

	// 2 servers + 1 cooling unit (using 'power' type as non-IT).
	srv1 := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")
	srv2 := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")
	cooling := seedAssetTyped(t, pool, tenantA, loc, "power", "ups")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, srv1, day, 10.0)
	upsertDailyKwh(t, pool, tenantA, srv2, day, 14.0)
	upsertDailyKwh(t, pool, tenantA, cooling, day, 6.0)

	if err := svc.AggregateLocationDay(ctx, tenantA, day); err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	rows, err := svc.ListLocationDailyPue(ctx, tenantA, &loc, day, day)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if pgNumericDecimalForTest(r.ItKwh).Cmp(dec("24")) != 0 {
		t.Errorf("it_kwh = %s, want 24", pgNumericDecimalForTest(r.ItKwh).String())
	}
	if pgNumericDecimalForTest(r.NonItKwh).Cmp(dec("6")) != 0 {
		t.Errorf("non_it_kwh = %s, want 6", pgNumericDecimalForTest(r.NonItKwh).String())
	}
	if pgNumericDecimalForTest(r.TotalKwh).Cmp(dec("30")) != 0 {
		t.Errorf("total_kwh = %s, want 30", pgNumericDecimalForTest(r.TotalKwh).String())
	}
	if r.ItAssetCount != 2 || r.NonItAssetCount != 1 {
		t.Errorf("asset counts = (%d, %d), want (2, 1)", r.ItAssetCount, r.NonItAssetCount)
	}
	// PUE = 30 / 24 = 1.25
	if !r.Pue.Valid {
		t.Errorf("pue is NULL, want 1.25")
	} else if pgNumericDecimalForTest(r.Pue).Cmp(dec("1.25")) != 0 {
		t.Errorf("pue = %s, want 1.25", pgNumericDecimalForTest(r.Pue).String())
	}
}

func TestEnergyPhase2_PueIsNullWhenNoIt(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "no-it-loc", "idc")

	// Only non-IT assets — PUE should be NULL, not ∞.
	cooling := seedAssetTyped(t, pool, tenantA, loc, "power", "ups")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	day := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, cooling, day, 12.0)

	if err := svc.AggregateLocationDay(ctx, tenantA, day); err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	rows, err := svc.ListLocationDailyPue(ctx, tenantA, &loc, day, day)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Pue.Valid {
		t.Errorf("pue should be NULL when no IT load, got %s", pgNumericDecimalForTest(rows[0].Pue).String())
	}
}

func TestEnergyPhase2_PueIdempotentReRun(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "idem", "idc")

	srv := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")
	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	day := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, srv, day, 8.0)

	if err := svc.AggregateLocationDay(ctx, tenantA, day); err != nil {
		t.Fatalf("agg1: %v", err)
	}
	if err := svc.AggregateLocationDay(ctx, tenantA, day); err != nil {
		t.Fatalf("agg2: %v", err)
	}
	var n int
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM energy_location_daily WHERE tenant_id=$1 AND location_id=$2 AND day=$3`,
		tenantA, loc, day,
	).Scan(&n)
	if n != 1 {
		t.Errorf("rows after re-run = %d, want 1", n)
	}
}

// ---------------------------------------------------------------------------
// Anomaly detector
// ---------------------------------------------------------------------------

func TestEnergyPhase2_AnomalyDetector_FlagsHighSpike(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "anom", "idc")
	asset := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// 7 days of baseline at 10 kWh, then a 25 kWh spike on day 8.
	// Median = 10, score = 25/10 = 2.5 → high (threshold 2.0).
	for i := 1; i <= 7; i++ {
		day := time.Date(2026, 5, i, 0, 0, 0, 0, time.UTC)
		upsertDailyKwh(t, pool, tenantA, asset, day, 10.0)
	}
	spikeDay := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, asset, spikeDay, 25.0)

	flagged, err := svc.DetectAnomaliesForDay(ctx, tenantA, spikeDay, energy.DefaultAnomalyConfig())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if flagged != 1 {
		t.Errorf("flagged = %d, want 1", flagged)
	}

	// Row in energy_anomalies should be 'high' with score=2.5.
	var kind, score string
	_ = pool.QueryRow(ctx,
		`SELECT kind, score::text FROM energy_anomalies WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, spikeDay,
	).Scan(&kind, &score)
	if kind != "high" {
		t.Errorf("kind = %q, want high", kind)
	}
	if dec(score).Cmp(dec("2.5")) != 0 {
		t.Errorf("score = %s, want 2.5", score)
	}
}

func TestEnergyPhase2_AnomalyDetector_FlagsLowDrop(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "low", "idc")
	asset := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// 7 days at 10 kWh, then 2 kWh on day 8 → score 0.2 ≤ 0.3 → low.
	for i := 1; i <= 7; i++ {
		day := time.Date(2026, 6, i, 0, 0, 0, 0, time.UTC)
		upsertDailyKwh(t, pool, tenantA, asset, day, 10.0)
	}
	dropDay := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, asset, dropDay, 2.0)

	flagged, err := svc.DetectAnomaliesForDay(ctx, tenantA, dropDay, energy.DefaultAnomalyConfig())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if flagged != 1 {
		t.Errorf("flagged = %d, want 1", flagged)
	}

	var kind string
	_ = pool.QueryRow(ctx,
		`SELECT kind FROM energy_anomalies WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, dropDay,
	).Scan(&kind)
	if kind != "low" {
		t.Errorf("kind = %q, want low", kind)
	}
}

func TestEnergyPhase2_AnomalyDetector_SkipsZeroKwhDays(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "zero", "idc")
	asset := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// 7 days baseline, then 0 kWh on day 8 — planned downtime, NOT an
	// anomaly (the missing-power signal is owned by alert rules).
	for i := 1; i <= 7; i++ {
		day := time.Date(2026, 7, i, 0, 0, 0, 0, time.UTC)
		upsertDailyKwh(t, pool, tenantA, asset, day, 10.0)
	}
	zeroDay := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, asset, zeroDay, 0.0)

	flagged, err := svc.DetectAnomaliesForDay(ctx, tenantA, zeroDay, energy.DefaultAnomalyConfig())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if flagged != 0 {
		t.Errorf("flagged = %d, want 0 (zero kWh is not an anomaly)", flagged)
	}
}

func TestEnergyPhase2_AnomalyDetector_RespectsMinSampleCount(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "minsamples", "idc")
	asset := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// Only 1 day of history — below MinSampleCount (3). Even a wild
	// spike on day 2 must NOT flag, because the median is meaningless
	// with that little data.
	upsertDailyKwh(t, pool, tenantA, asset, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), 10.0)
	upsertDailyKwh(t, pool, tenantA, asset, time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), 100.0)

	flagged, err := svc.DetectAnomaliesForDay(ctx, tenantA, time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), energy.DefaultAnomalyConfig())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if flagged != 0 {
		t.Errorf("flagged = %d, want 0 (insufficient baseline)", flagged)
	}
}

func TestEnergyPhase2_AnomalyDetector_ReRunPreservesOperatorAck(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergyPhase2(t, pool)
	loc := seedLocation(t, pool, tenantA, "ack", "idc")
	asset := seedAssetTyped(t, pool, tenantA, loc, "server", "rack_mount")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// Set up a flag.
	for i := 1; i <= 7; i++ {
		day := time.Date(2026, 9, i, 0, 0, 0, 0, time.UTC)
		upsertDailyKwh(t, pool, tenantA, asset, day, 10.0)
	}
	spikeDay := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, asset, spikeDay, 25.0)
	if _, err := svc.DetectAnomaliesForDay(ctx, tenantA, spikeDay, energy.DefaultAnomalyConfig()); err != nil {
		t.Fatalf("detect1: %v", err)
	}

	// Operator acks.
	if _, err := svc.TransitionAnomaly(ctx, tenantA, asset, spikeDay, "ack", uuid.Nil, "expected — load test"); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Re-run detector — must NOT revert status to 'open'.
	if _, err := svc.DetectAnomaliesForDay(ctx, tenantA, spikeDay, energy.DefaultAnomalyConfig()); err != nil {
		t.Fatalf("detect2: %v", err)
	}
	var status, note string
	_ = pool.QueryRow(ctx,
		`SELECT status, COALESCE(note, '') FROM energy_anomalies WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, spikeDay,
	).Scan(&status, &note)
	if status != "ack" {
		t.Errorf("status after re-run = %q, want ack", status)
	}
	if note != "expected — load test" {
		t.Errorf("note clobbered: %q", note)
	}
}

func TestEnergyPhase2_TenantIsolation(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForEnergyPhase2(t, pool)
	locA := seedLocation(t, pool, tenantA, "loc-A", "idc")
	locB := seedLocation(t, pool, tenantB, "loc-B", "idc")
	assetA := seedAssetTyped(t, pool, tenantA, locA, "server", "rack_mount")
	assetB := seedAssetTyped(t, pool, tenantB, locB, "server", "rack_mount")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	upsertDailyKwh(t, pool, tenantA, assetA, day, 10.0)
	upsertDailyKwh(t, pool, tenantB, assetB, day, 50.0)

	if err := svc.AggregateLocationDay(ctx, tenantA, day); err != nil {
		t.Fatalf("agg A: %v", err)
	}

	// Tenant A's PUE list must not include tenant B's location.
	rowsA, err := svc.ListLocationDailyPue(ctx, tenantA, nil, day, day)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	for _, r := range rowsA {
		if uuid.UUID(r.LocationID) == locB {
			t.Errorf("tenant A's list leaked tenant B's location")
		}
	}

	// Tenant B's PUE list is empty (we didn't run AggregateLocationDay
	// for B), confirming there's no shared rollup either.
	rowsB, _ := svc.ListLocationDailyPue(ctx, tenantB, nil, day, day)
	if len(rowsB) != 0 {
		t.Errorf("tenant B has %d PUE rows before its own aggregator runs, want 0", len(rowsB))
	}
}
