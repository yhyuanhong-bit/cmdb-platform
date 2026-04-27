//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/energy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Wave 6.1 coverage. Three things matter:
//
//   1. Tariff CRUD enforces no-overlap on the same (tenant, location)
//      regardless of whether the existing or new range is open-ended.
//   2. AggregateDay is idempotent — re-running it for a (asset, day)
//      overwrites rather than duplicating, and it correctly handles
//      the missing-samples case.
//   3. CalculateBill picks the right tariff for each asset, sums kWh,
//      and surfaces currency_mixed correctly.

func newEnergyTestPool(t *testing.T) *pgxpool.Pool {
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

func seedTwoTenantsForEnergy(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "en-A-"+suffix, "en-a-"+suffix,
		b, "en-B-"+suffix, "en-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM energy_daily_kwh WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM energy_tariffs   WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM metrics          WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM assets           WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants          WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

// seedLocation inserts a row in `locations` with all NOT NULL columns
// satisfied. The locations table requires slug; we synthesise a unique
// one from the UUID so concurrent test runs don't collide.
func seedLocation(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, name, level string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	slug := name + "-" + id.String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO locations (id, tenant_id, name, slug, level) VALUES ($1, $2, $3, $4, $5)`,
		id, tenantID, name, slug, level,
	); err != nil {
		t.Fatalf("seed location: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM locations WHERE id = $1`, id)
	})
	return id
}

func seedAssetForEnergy(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, locationID *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	tag := "EN-" + id.String()[:8]
	var locArg interface{}
	if locationID != nil {
		locArg = *locationID
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, location_id)
		 VALUES ($1, $2, $3, $3, 'server', 'rack_mount', 'operational', $4)`,
		id, tenantID, tag, locArg,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

// insertPowerSamples writes N power_kw samples spaced one hour apart,
// starting from `start`. The sample value is constant `kw` per sample
// — when the aggregator buckets by hour and SUMs, the result is N*kw
// kWh (one hour per bucket).
func insertPowerSamples(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID, start time.Time, n int, kw float64) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		ts := start.Add(time.Duration(i) * time.Hour)
		if _, err := pool.Exec(ctx,
			`INSERT INTO metrics (time, tenant_id, asset_id, name, value)
			 VALUES ($1, $2, $3, 'power_kw', $4)`,
			ts, tenantID, assetID, kw,
		); err != nil {
			t.Fatalf("insert metric: %v", err)
		}
	}
}

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// pgNumericDecimalForTest unwraps a pgtype.Numeric to a decimal.Decimal
// for assertion purposes.
func pgNumericDecimalForTest(n pgtype.Numeric) decimal.Decimal {
	raw, err := n.Value()
	if err != nil || raw == nil {
		return decimal.Zero
	}
	if s, ok := raw.(string); ok {
		d, _ := decimal.NewFromString(s)
		return d
	}
	return decimal.Zero
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEnergyTariff_OverlapRejection(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)
	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	from1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to1 := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID:      tenantA,
		RatePerKWh:    dec("0.12"),
		EffectiveFrom: from1,
		EffectiveTo:   &to1,
	}); err != nil {
		t.Fatalf("first tariff: %v", err)
	}

	// Second tariff overlapping the first by one day → rejected.
	overFrom := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	overTo := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID:      tenantA,
		RatePerKWh:    dec("0.13"),
		EffectiveFrom: overFrom,
		EffectiveTo:   &overTo,
	}); err != energy.ErrTariffOverlap {
		t.Errorf("overlapping tariff: want ErrTariffOverlap, got %v", err)
	}

	// Adjacent (starts the day after the first one ends) → accepted.
	adjFrom := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID:      tenantA,
		RatePerKWh:    dec("0.14"),
		EffectiveFrom: adjFrom,
	}); err != nil {
		t.Errorf("adjacent tariff: %v", err)
	}

	// Open-ended tariff with start before existing → rejected.
	earlyFrom := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID:      tenantA,
		RatePerKWh:    dec("0.15"),
		EffectiveFrom: earlyFrom,
	}); err != energy.ErrTariffOverlap {
		t.Errorf("open-ended early tariff: want ErrTariffOverlap, got %v", err)
	}
}

func TestEnergyTariff_DifferentLocationsDontOverlap(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)
	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	loc1 := seedLocation(t, pool, tenantA, "IDC-A", "idc")
	loc2 := seedLocation(t, pool, tenantA, "IDC-B", "idc")

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	// Same dates but different locations → not an overlap.
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc1, RatePerKWh: dec("0.10"),
		EffectiveFrom: from, EffectiveTo: &to,
	}); err != nil {
		t.Fatalf("loc1 tariff: %v", err)
	}
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc2, RatePerKWh: dec("0.20"),
		EffectiveFrom: from, EffectiveTo: &to,
	}); err != nil {
		t.Errorf("loc2 tariff (different location, should be allowed): %v", err)
	}
}

func TestEnergyAggregate_IdempotentReRun(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)
	asset := seedAssetForEnergy(t, pool, tenantA, nil)
	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// 24 hourly samples at 0.5kw each → 24h * 0.5kw = 12 kWh expected.
	day := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	insertPowerSamples(t, pool, tenantA, asset, day, 24, 0.5)

	if err := svc.AggregateDay(ctx, tenantA, asset, day); err != nil {
		t.Fatalf("aggregate first run: %v", err)
	}

	var kwh1, peak1, avg1 string
	var n1 int
	if err := pool.QueryRow(ctx,
		`SELECT kwh_total::text, kw_peak::text, kw_avg::text, sample_count
		 FROM energy_daily_kwh WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, day,
	).Scan(&kwh1, &peak1, &avg1, &n1); err != nil {
		t.Fatalf("read agg: %v", err)
	}
	if n1 != 24 {
		t.Errorf("sample_count = %d, want 24", n1)
	}
	if dec(kwh1).Cmp(dec("12")) != 0 {
		t.Errorf("kwh_total = %s, want 12", kwh1)
	}
	if dec(peak1).Cmp(dec("0.5")) != 0 {
		t.Errorf("kw_peak = %s, want 0.5", peak1)
	}

	// Re-run for the same day → exactly one row, same values.
	if err := svc.AggregateDay(ctx, tenantA, asset, day); err != nil {
		t.Fatalf("aggregate second run: %v", err)
	}
	var rowCount int
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM energy_daily_kwh WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, day,
	).Scan(&rowCount)
	if rowCount != 1 {
		t.Errorf("idempotency: rows = %d, want 1", rowCount)
	}
}

func TestEnergyAggregate_MissingSamplesAreZero(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)
	asset := seedAssetForEnergy(t, pool, tenantA, nil)
	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// Only 6 hourly samples (the asset went offline mid-day). Aggregator
	// should produce 6 * 1.0 = 6 kWh, not extrapolate to 24h.
	day := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	insertPowerSamples(t, pool, tenantA, asset, day, 6, 1.0)

	if err := svc.AggregateDay(ctx, tenantA, asset, day); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	var kwh string
	var n int
	_ = pool.QueryRow(ctx,
		`SELECT kwh_total::text, sample_count FROM energy_daily_kwh WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, day,
	).Scan(&kwh, &n)
	if n != 6 {
		t.Errorf("sample_count = %d, want 6", n)
	}
	if dec(kwh).Cmp(dec("6")) != 0 {
		t.Errorf("kwh_total = %s, want 6 (no gap-filling)", kwh)
	}
}

func TestEnergyResolveTariff_LocationOverridesDefault(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)
	loc := seedLocation(t, pool, tenantA, "IDC-prio", "idc")

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	// Tenant default — should be the fallback.
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, RatePerKWh: dec("0.10"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("default tariff: %v", err)
	}
	// Per-location tariff — should win when day is in range.
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc, RatePerKWh: dec("0.18"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("location tariff: %v", err)
	}

	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Resolve for the location → location-specific rate wins.
	got, err := svc.ResolveTariff(ctx, tenantA, &loc, day)
	if err != nil {
		t.Fatalf("resolve loc: %v", err)
	}
	if rate := pgNumericDecimalForTest(got.RatePerKwh); rate.Cmp(dec("0.18")) != 0 {
		t.Errorf("location resolve rate = %s, want 0.18", rate)
	}

	// Resolve for an asset whose location has no specific tariff →
	// falls back to tenant default.
	otherLoc := uuid.New()
	got2, err := svc.ResolveTariff(ctx, tenantA, &otherLoc, day)
	if err != nil {
		t.Fatalf("resolve fallback: %v", err)
	}
	if rate := pgNumericDecimalForTest(got2.RatePerKwh); rate.Cmp(dec("0.10")) != 0 {
		t.Errorf("fallback rate = %s, want 0.10 (tenant default)", rate)
	}
}

func TestEnergyBill_PerAssetCorrectness(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)

	loc1 := seedLocation(t, pool, tenantA, "IDC-1", "idc")
	loc2 := seedLocation(t, pool, tenantA, "IDC-2", "idc")

	asset1 := seedAssetForEnergy(t, pool, tenantA, &loc1)
	asset2 := seedAssetForEnergy(t, pool, tenantA, &loc2)

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc1, RatePerKWh: dec("0.10"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("loc1 tariff: %v", err)
	}
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc2, RatePerKWh: dec("0.25"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("loc2 tariff: %v", err)
	}

	day := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	// asset1 burns 24 kWh that day, asset2 burns 12 kWh.
	insertPowerSamples(t, pool, tenantA, asset1, day, 24, 1.0)
	insertPowerSamples(t, pool, tenantA, asset2, day, 24, 0.5)

	if err := svc.AggregateDay(ctx, tenantA, asset1, day); err != nil {
		t.Fatalf("agg asset1: %v", err)
	}
	if err := svc.AggregateDay(ctx, tenantA, asset2, day); err != nil {
		t.Fatalf("agg asset2: %v", err)
	}

	bill, err := svc.CalculateBill(ctx, tenantA, day, day)
	if err != nil {
		t.Fatalf("bill: %v", err)
	}

	// Expected: asset1 24 kWh * 0.10 = $2.40 ; asset2 12 kWh * 0.25 = $3.00
	// Total: 36 kWh, $5.40 USD.
	if bill.TotalKWh.Cmp(dec("36")) != 0 {
		t.Errorf("total kWh = %s, want 36", bill.TotalKWh)
	}
	if bill.TotalCost.Cmp(dec("5.4")) != 0 {
		t.Errorf("total cost = %s, want 5.4", bill.TotalCost)
	}
	if bill.Currency != "USD" {
		t.Errorf("currency = %s, want USD", bill.Currency)
	}
	if bill.CurrencyMixed {
		t.Error("currency_mixed should be false")
	}
	if len(bill.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(bill.Lines))
	}
}

func TestEnergyBill_MixedCurrenciesFlagged(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForEnergy(t, pool)

	loc1 := seedLocation(t, pool, tenantA, "TW-1", "idc")
	loc2 := seedLocation(t, pool, tenantA, "US-1", "idc")

	a1 := seedAssetForEnergy(t, pool, tenantA, &loc1)
	a2 := seedAssetForEnergy(t, pool, tenantA, &loc2)

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc1, Currency: "TWD", RatePerKWh: dec("3.5"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("TWD tariff: %v", err)
	}
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, LocationID: &loc2, Currency: "USD", RatePerKWh: dec("0.12"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("USD tariff: %v", err)
	}

	day := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	insertPowerSamples(t, pool, tenantA, a1, day, 24, 1.0)
	insertPowerSamples(t, pool, tenantA, a2, day, 24, 1.0)
	_ = svc.AggregateDay(ctx, tenantA, a1, day)
	_ = svc.AggregateDay(ctx, tenantA, a2, day)

	bill, err := svc.CalculateBill(ctx, tenantA, day, day)
	if err != nil {
		t.Fatalf("bill: %v", err)
	}
	if !bill.CurrencyMixed {
		t.Errorf("expected currency_mixed=true when TWD + USD lines coexist")
	}
	if bill.Currency != "MIXED" {
		t.Errorf("currency = %s, want MIXED", bill.Currency)
	}
}

func TestEnergyBill_TenantIsolation(t *testing.T) {
	pool := newEnergyTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForEnergy(t, pool)
	assetA := seedAssetForEnergy(t, pool, tenantA, nil)
	assetB := seedAssetForEnergy(t, pool, tenantB, nil)

	q := dbgen.New(pool)
	svc := energy.NewService(q, pool)

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantA, RatePerKWh: dec("0.10"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("A tariff: %v", err)
	}
	if _, err := svc.CreateTariff(ctx, energy.CreateTariffParams{
		TenantID: tenantB, RatePerKWh: dec("0.20"), EffectiveFrom: from,
	}); err != nil {
		t.Fatalf("B tariff: %v", err)
	}

	day := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	insertPowerSamples(t, pool, tenantA, assetA, day, 24, 1.0)
	insertPowerSamples(t, pool, tenantB, assetB, day, 24, 2.0)
	_ = svc.AggregateDay(ctx, tenantA, assetA, day)
	_ = svc.AggregateDay(ctx, tenantB, assetB, day)

	billA, _ := svc.CalculateBill(ctx, tenantA, day, day)
	billB, _ := svc.CalculateBill(ctx, tenantB, day, day)

	// A: 24 * 0.10 = 2.40 ; B: 48 * 0.20 = 9.60. Cross-tenant bill must
	// not include the other tenant's kWh.
	if billA.TotalKWh.Cmp(dec("24")) != 0 {
		t.Errorf("A kWh = %s, want 24", billA.TotalKWh)
	}
	if billB.TotalKWh.Cmp(dec("48")) != 0 {
		t.Errorf("B kWh = %s, want 48", billB.TotalKWh)
	}
	if billA.TotalCost.Cmp(dec("2.4")) != 0 {
		t.Errorf("A cost = %s, want 2.4", billA.TotalCost)
	}
	if billB.TotalCost.Cmp(dec("9.6")) != 0 {
		t.Errorf("B cost = %s, want 9.6", billB.TotalCost)
	}
}
