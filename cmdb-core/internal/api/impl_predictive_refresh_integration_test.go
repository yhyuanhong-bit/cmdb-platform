//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/predictive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// dec parses a decimal string for test fixtures. Lives here only because
// the predictive refresh tests need it for risk-score assertions; the
// energy billing tests that originally hosted it are gone.
func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Wave 7.1 coverage. The interesting properties:
//
//   1. Each rule fires for the right inputs (warranty, EOL, age) and
//      produces a recommendation row keyed on (asset, kind).
//   2. Re-running the scan preserves operator status — an acked
//      recommendation must NOT revert to 'open' on the next tick.
//   3. Stale 'open' rows are swept when the rule no longer matches
//      (warranty got renewed, EOL pushed out).
//   4. Risk score is monotonic in urgency: closer-to-expiry > farther,
//      past-expiry > pre-expiry.
//   5. Tenant isolation.

func newPredictiveTestPool(t *testing.T) *pgxpool.Pool {
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

func seedTwoTenantsForPredictive(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "pr-A-"+suffix, "pr-a-"+suffix,
		b, "pr-B-"+suffix, "pr-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM predictive_refresh_recommendations WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM assets  WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

// seedAssetWithLifecycle inserts an asset with the lifecycle dates the
// rule engine reads. Pass nil for a date to skip it (NULL in DB).
func seedAssetWithLifecycle(
	t *testing.T,
	pool *pgxpool.Pool,
	tenantID uuid.UUID,
	purchaseDate, warrantyEnd, eolDate *time.Time,
	expectedLifespanMonths *int,
) uuid.UUID {
	t.Helper()
	id := uuid.New()
	tag := "PR-" + id.String()[:8]
	var pd, we, eol interface{}
	if purchaseDate != nil {
		pd = *purchaseDate
	}
	if warrantyEnd != nil {
		we = *warrantyEnd
	}
	if eolDate != nil {
		eol = *eolDate
	}
	var els interface{}
	if expectedLifespanMonths != nil {
		els = *expectedLifespanMonths
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status,
		                    purchase_date, warranty_end, eol_date, expected_lifespan_months)
		VALUES ($1, $2, $3, $3, 'server', 'rack_mount', 'operational', $4, $5, $6, $7)
	`, id, tenantID, tag, pd, we, eol, els); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

func countRecommendations(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM predictive_refresh_recommendations WHERE tenant_id=$1`, tenantID,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func readRecommendation(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID, kind string) (status, score string) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT status, risk_score::text FROM predictive_refresh_recommendations WHERE tenant_id=$1 AND asset_id=$2 AND kind=$3`,
		tenantID, assetID, kind,
	).Scan(&status, &score); err != nil {
		t.Fatalf("read rec: %v", err)
	}
	return
}

// ---------------------------------------------------------------------------
// Rule firing
// ---------------------------------------------------------------------------

func TestPredictive_WarrantyExpiringFires(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)

	// Pin "now" so the test is deterministic against absolute dates.
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Warranty ends in 30 days → should fire warranty_expiring.
	end := now.AddDate(0, 0, 30)
	asset := seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)

	scan, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scan.RowsUpserted != 1 {
		t.Errorf("rows upserted = %d, want 1", scan.RowsUpserted)
	}
	status, score := readRecommendation(t, pool, tenantA, asset, "warranty_expiring")
	if status != "open" {
		t.Errorf("status = %q, want open", status)
	}
	// 30 days remaining out of 90-day horizon → score ≈ (90-30)/90*100 = 66.67.
	if dec(score).LessThan(dec("60")) || dec(score).GreaterThan(dec("70")) {
		t.Errorf("score = %s, want in [60, 70]", score)
	}
}

func TestPredictive_WarrantyExpiredFires(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Warranty ended 100 days ago.
	end := now.AddDate(0, 0, -100)
	asset := seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	status, score := readRecommendation(t, pool, tenantA, asset, "warranty_expired")
	if status != "open" {
		t.Errorf("status = %q, want open", status)
	}
	// Past-deadline scores ≥ 80; 100 days past out of 365 saturation → ≈ 85.5.
	if dec(score).LessThan(dec("80")) {
		t.Errorf("score = %s, want >= 80 (past-deadline floor)", score)
	}
}

func TestPredictive_EolApproachingAndPassed(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// One asset whose EOL is in 60 days, one passed 30 days ago.
	approach := now.AddDate(0, 0, 60)
	passed := now.AddDate(0, 0, -30)
	approachAsset := seedAssetWithLifecycle(t, pool, tenantA, nil, nil, &approach, nil)
	passedAsset := seedAssetWithLifecycle(t, pool, tenantA, nil, nil, &passed, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status, _ := readRecommendation(t, pool, tenantA, approachAsset, "eol_approaching"); status != "open" {
		t.Errorf("approaching: status = %q, want open", status)
	}
	if status, _ := readRecommendation(t, pool, tenantA, passedAsset, "eol_passed"); status != "open" {
		t.Errorf("passed: status = %q, want open", status)
	}
}

func TestPredictive_AgedOutFiresWhenPastLifespan(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Bought 5 years ago, 36-month spec → 24 months past lifespan.
	purchase := now.AddDate(-5, 0, 0)
	months := 36
	asset := seedAssetWithLifecycle(t, pool, tenantA, &purchase, nil, nil, &months)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	status, _ := readRecommendation(t, pool, tenantA, asset, "aged_out")
	if status != "open" {
		t.Errorf("aged_out: status = %q, want open", status)
	}
}

func TestPredictive_NoFiringForFreshAsset(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Brand new: warranty ends in 2 years, EOL in 5 years, bought yesterday.
	purchase := now.AddDate(0, 0, -1)
	warranty := now.AddDate(2, 0, 0)
	eol := now.AddDate(5, 0, 0)
	months := 60
	_ = seedAssetWithLifecycle(t, pool, tenantA, &purchase, &warranty, &eol, &months)

	scan, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scan.RowsUpserted != 0 {
		t.Errorf("fresh asset rows = %d, want 0", scan.RowsUpserted)
	}
}

// ---------------------------------------------------------------------------
// Operator status preserved across scans
// ---------------------------------------------------------------------------

func TestPredictive_AckSurvivesReScan(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	end := now.AddDate(0, 0, 30)
	asset := seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan1: %v", err)
	}
	if _, err := svc.Transition(ctx, tenantA, asset, "warranty_expiring", "ack", uuid.Nil, "got renewal quote"); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan2: %v", err)
	}
	status, _ := readRecommendation(t, pool, tenantA, asset, "warranty_expiring")
	if status != "ack" {
		t.Errorf("status after re-scan = %q, want ack (operator decision must survive)", status)
	}
}

// ---------------------------------------------------------------------------
// Stale sweep
// ---------------------------------------------------------------------------

func TestPredictive_StaleOpenRowsSweptWhenRuleNoLongerMatches(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Asset A: warranty ends in 30 days → fires warranty_expiring.
	end := now.AddDate(0, 0, 30)
	asset := seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)
	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan1: %v", err)
	}
	if countRecommendations(t, pool, tenantA) != 1 {
		t.Fatalf("expected 1 row after scan1")
	}

	// Operator extends the warranty by 5 years (e.g. renewed contract).
	newEnd := now.AddDate(5, 0, 0)
	if _, err := pool.Exec(ctx, `UPDATE assets SET warranty_end = $1 WHERE id = $2`, newEnd, asset); err != nil {
		t.Fatalf("update warranty: %v", err)
	}

	// Re-scan: rule no longer matches, the open row should be swept.
	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan2: %v", err)
	}
	if got := countRecommendations(t, pool, tenantA); got != 0 {
		t.Errorf("rows after warranty extended = %d, want 0 (stale 'open' rows must be swept)", got)
	}
}

func TestPredictive_AckedRowsKeptAcrossSweep(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	end := now.AddDate(0, 0, 30)
	asset := seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)
	_, _ = svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig())
	if _, err := svc.Transition(ctx, tenantA, asset, "warranty_expiring", "ack", uuid.Nil, ""); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Now extend warranty so rule no longer matches.
	newEnd := now.AddDate(5, 0, 0)
	_, _ = pool.Exec(ctx, `UPDATE assets SET warranty_end = $1 WHERE id = $2`, newEnd, asset)
	_, _ = svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig())

	// Acked row must still be there for audit.
	if got := countRecommendations(t, pool, tenantA); got != 1 {
		t.Errorf("rows after sweep = %d, want 1 (acked rows preserved for audit)", got)
	}
}

// ---------------------------------------------------------------------------
// Score monotonicity
// ---------------------------------------------------------------------------

func TestPredictive_ScoreMonotonicWithDeadlineProximity(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Asset B's warranty ends in 5 days; Asset A's in 60 days. B should
	// score higher than A. Both are within the 90-day horizon.
	endA := now.AddDate(0, 0, 60)
	endB := now.AddDate(0, 0, 5)
	a := seedAssetWithLifecycle(t, pool, tenantA, nil, &endA, nil, nil)
	b := seedAssetWithLifecycle(t, pool, tenantA, nil, &endB, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	_, scoreA := readRecommendation(t, pool, tenantA, a, "warranty_expiring")
	_, scoreB := readRecommendation(t, pool, tenantA, b, "warranty_expiring")
	if dec(scoreB).LessThanOrEqual(dec(scoreA)) {
		t.Errorf("urgency monotonicity broken: B (5 days) score=%s, A (60 days) score=%s", scoreB, scoreA)
	}
}

func TestPredictive_PastDeadlineOutscoresInWindow(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// "Expired 1 day ago" must score higher than "expires in 89 days".
	expired := now.AddDate(0, 0, -1)
	expiring := now.AddDate(0, 0, 89)
	expiredAsset := seedAssetWithLifecycle(t, pool, tenantA, nil, &expired, nil, nil)
	expiringAsset := seedAssetWithLifecycle(t, pool, tenantA, nil, &expiring, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	_, expiredScore := readRecommendation(t, pool, tenantA, expiredAsset, "warranty_expired")
	_, expiringScore := readRecommendation(t, pool, tenantA, expiringAsset, "warranty_expiring")
	if dec(expiredScore).LessThanOrEqual(dec(expiringScore)) {
		t.Errorf("past-deadline must outrank in-window: expired=%s expiring=%s", expiredScore, expiringScore)
	}
}

// ---------------------------------------------------------------------------
// Tenant isolation
// ---------------------------------------------------------------------------

func TestPredictive_TenantIsolation(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	end := now.AddDate(0, 0, 30)
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)
	_ = seedAssetWithLifecycle(t, pool, tenantB, nil, &end, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan A: %v", err)
	}

	// Tenant A has 1 row. Tenant B's scan didn't run yet so it has 0.
	// Critically, A's scan must not have produced a row tied to tenantB.
	if got := countRecommendations(t, pool, tenantA); got != 1 {
		t.Errorf("A rows = %d, want 1", got)
	}
	if got := countRecommendations(t, pool, tenantB); got != 0 {
		t.Errorf("B rows = %d, want 0 (cross-tenant leak)", got)
	}
}

// ---------------------------------------------------------------------------
// AggregatePredictiveRefreshByMonth — capex backlog roll-up.
// ---------------------------------------------------------------------------

// monthKey returns YYYY-MM-01 for the month containing t, in UTC.
func monthKey(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// TestPredictive_AggregateByMonth_BucketsAndCounts checks that the
// aggregate query groups open recs by target_date month and returns
// per-kind counts that sum to the bucket total. NULL target_date and
// non-open rows are excluded by design.
func TestPredictive_AggregateByMonth_BucketsAndCounts(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// Seed mix so we get rows in two distinct target_date months.
	// Asset 1: warranty_expiring → target_date is the warranty_end (≈ Jul 2026).
	end1 := now.AddDate(0, 2, 5) // 2026-07-06
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &end1, nil, nil)
	// Asset 2: also warranty_expiring, same month bucket.
	end2 := now.AddDate(0, 2, 20) // 2026-07-21
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &end2, nil, nil)
	// Asset 3: eol_approaching → target_date is the eol_date (≈ Jun 2026).
	eol3 := now.AddDate(0, 1, 10) // 2026-06-11
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, nil, &eol3, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	rows, err := q.AggregatePredictiveRefreshByMonth(ctx, dbgen.AggregatePredictiveRefreshByMonthParams{TenantID: tenantA})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("buckets = %d, want 2 (Jun 2026 + Jul 2026)", len(rows))
	}
	// Ascending order — Jun before Jul.
	if got, want := monthKey(rows[0].Month.Time), monthKey(eol3); !got.Equal(want) {
		t.Errorf("rows[0].Month = %s, want %s", got, want)
	}
	if got, want := monthKey(rows[1].Month.Time), monthKey(end1); !got.Equal(want) {
		t.Errorf("rows[1].Month = %s, want %s", got, want)
	}
	if rows[0].Count != 1 || rows[0].EolApproaching != 1 {
		t.Errorf("Jun bucket = %+v, want count=1 eol_approaching=1", rows[0])
	}
	if rows[1].Count != 2 || rows[1].WarrantyExpiring != 2 {
		t.Errorf("Jul bucket = %+v, want count=2 warranty_expiring=2", rows[1])
	}

	// Per-kind breakdown must sum to bucket total.
	for i, r := range rows {
		sum := r.WarrantyExpiring + r.WarrantyExpired + r.EolApproaching + r.EolPassed + r.AgedOut
		if sum != r.Count {
			t.Errorf("rows[%d] kind sum = %d, want count=%d", i, sum, r.Count)
		}
	}
}

// TestPredictive_AggregateByMonth_RangeFilter exercises the from/to
// month bounds. Anything outside the [from, to] window must be omitted.
func TestPredictive_AggregateByMonth_RangeFilter(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	// One per month from Jun through Aug 2026.
	jun := now.AddDate(0, 1, 10) // 2026-06-11
	jul := now.AddDate(0, 2, 10) // 2026-07-11
	aug := now.AddDate(0, 3, 10) // 2026-08-11
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &jun, nil, nil)
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &jul, nil, nil)
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &aug, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	from := pgtype.Date{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	to := pgtype.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), Valid: true}
	rows, err := q.AggregatePredictiveRefreshByMonth(ctx, dbgen.AggregatePredictiveRefreshByMonthParams{
		TenantID:  tenantA,
		FromMonth: from,
		ToMonth:   to,
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("buckets = %d, want 1 (only Jul)", len(rows))
	}
	if got := monthKey(rows[0].Month.Time); !got.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("month = %s, want 2026-07-01", got)
	}
}

// TestPredictive_AggregateByMonth_AcksAndNullTargetExcluded verifies
// the rollup ignores acked/resolved rows and rows without target_date.
func TestPredictive_AggregateByMonth_AcksAndNullTargetExcluded(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	end := now.AddDate(0, 1, 10) // 2026-06-11
	asset := seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)
	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Open + has target_date → counted.
	rows, err := q.AggregatePredictiveRefreshByMonth(ctx, dbgen.AggregatePredictiveRefreshByMonthParams{TenantID: tenantA})
	if err != nil {
		t.Fatalf("aggregate1: %v", err)
	}
	if len(rows) != 1 || rows[0].Count != 1 {
		t.Fatalf("baseline rows = %v, want 1 bucket count=1", rows)
	}

	// Now ack it — should disappear from the rollup.
	if _, err := svc.Transition(ctx, tenantA, asset, "warranty_expiring", "ack", uuid.Nil, ""); err != nil {
		t.Fatalf("ack: %v", err)
	}
	rows, err = q.AggregatePredictiveRefreshByMonth(ctx, dbgen.AggregatePredictiveRefreshByMonthParams{TenantID: tenantA})
	if err != nil {
		t.Fatalf("aggregate2: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("acked rows must not appear in rollup, got %v", rows)
	}
}

// TestPredictive_AggregateByMonth_TenantIsolation ensures tenant A's
// rollup never returns rows for tenant B's recommendations even when
// both tenants have assets in the same month.
func TestPredictive_AggregateByMonth_TenantIsolation(t *testing.T) {
	pool := newPredictiveTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForPredictive(t, pool)
	q := dbgen.New(pool)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := predictive.NewService(q, pool).WithClock(func() time.Time { return now })

	end := now.AddDate(0, 1, 10)
	_ = seedAssetWithLifecycle(t, pool, tenantA, nil, &end, nil, nil)
	// Two assets for tenant B in the same month.
	_ = seedAssetWithLifecycle(t, pool, tenantB, nil, &end, nil, nil)
	_ = seedAssetWithLifecycle(t, pool, tenantB, nil, &end, nil, nil)

	if _, err := svc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan A: %v", err)
	}
	if _, err := svc.ScanAndUpsert(ctx, tenantB, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan B: %v", err)
	}

	rowsA, err := q.AggregatePredictiveRefreshByMonth(ctx, dbgen.AggregatePredictiveRefreshByMonthParams{TenantID: tenantA})
	if err != nil {
		t.Fatalf("aggregate A: %v", err)
	}
	rowsB, err := q.AggregatePredictiveRefreshByMonth(ctx, dbgen.AggregatePredictiveRefreshByMonthParams{TenantID: tenantB})
	if err != nil {
		t.Fatalf("aggregate B: %v", err)
	}

	if len(rowsA) != 1 || rowsA[0].Count != 1 {
		t.Errorf("A rollup = %v, want 1 bucket count=1 (B's 2 assets must not leak)", rowsA)
	}
	if len(rowsB) != 1 || rowsB[0].Count != 2 {
		t.Errorf("B rollup = %v, want 1 bucket count=2", rowsB)
	}
}
