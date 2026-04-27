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
