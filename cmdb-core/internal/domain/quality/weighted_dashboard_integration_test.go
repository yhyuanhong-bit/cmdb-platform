//go:build integration

package quality

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// D9-P1 weighted dashboard: a hot asset's score must pull the tenant
// average more than a cold asset's. The unit tests cover the
// accessWeightFor curve; this test pins the end-to-end SQL path:
// scanner writes access_weight alongside the score, GetDashboard
// reports the weighted average.

func testDBURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"
}

func newIntegrationPool(t *testing.T) *pgxpool.Pool {
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

// TestIntegration_WeightedDashboard: given two assets — one cold
// (access_count=0, score=100) and one hot (access_count=500, score=50) —
// the weighted average must lie close to the hot asset's score, not
// the midpoint 75 that an un-weighted AVG would produce.
func TestIntegration_WeightedDashboard(t *testing.T) {
	pool := newIntegrationPool(t)
	defer pool.Close()

	ctx := context.Background()
	tenantID := uuid.New()
	coldAsset := uuid.New()
	hotAsset := uuid.New()
	suf := tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		tenantID, "wdash-"+suf, "wdash-"+suf); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, access_count_24h)
		 VALUES ($1, $3, $4, 'cold asset', 'server', 0),
		        ($2, $3, $5, 'hot asset', 'server', 500)`,
		coldAsset, hotAsset, tenantID,
		"WDASH-C-"+suf, "WDASH-H-"+suf,
	); err != nil {
		t.Fatalf("insert assets: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM quality_scores WHERE tenant_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	q := dbgen.New(pool)
	coldWeight := accessWeightFor(0)
	hotWeight := accessWeightFor(500)

	if err := q.CreateQualityScore(ctx, dbgen.CreateQualityScoreParams{
		TenantID:     tenantID,
		AssetID:      coldAsset,
		Completeness: numFromFloat(100),
		Accuracy:     numFromFloat(100),
		Timeliness:   numFromFloat(100),
		Consistency:  numFromFloat(100),
		TotalScore:   numFromFloat(100),
		IssueDetails: []byte("[]"),
		AccessWeight: numFromFloat(coldWeight),
	}); err != nil {
		t.Fatalf("insert cold score: %v", err)
	}
	if err := q.CreateQualityScore(ctx, dbgen.CreateQualityScoreParams{
		TenantID:     tenantID,
		AssetID:      hotAsset,
		Completeness: numFromFloat(50),
		Accuracy:     numFromFloat(50),
		Timeliness:   numFromFloat(50),
		Consistency:  numFromFloat(50),
		TotalScore:   numFromFloat(50),
		IssueDetails: []byte("[]"),
		AccessWeight: numFromFloat(hotWeight),
	}); err != nil {
		t.Fatalf("insert hot score: %v", err)
	}

	row, err := q.GetQualityDashboard(ctx, tenantID)
	if err != nil {
		t.Fatalf("get dashboard: %v", err)
	}

	expected := (100*coldWeight + 50*hotWeight) / (coldWeight + hotWeight)
	got := toFloat(t, row.AvgTotal)
	// NUMERIC(5,2) rounding tolerance.
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("avg_total = %v, want ~%v (weighted: 100*%.3f + 50*%.3f)",
			got, expected, coldWeight, hotWeight)
	}
	// Un-weighted would be 75. If the weighting isn't in effect, the
	// result will sit near 75 and this guard catches the regression.
	if got >= 74 {
		t.Errorf("avg_total = %v indistinguishable from un-weighted 75 — hot-asset weighting not applied", got)
	}
}

func numFromFloat(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	// NUMERIC(5,2) precision in the schema; format with 2 decimals.
	if err := n.Scan(fmt.Sprintf("%.2f", v)); err != nil {
		panic(fmt.Sprintf("numFromFloat scan: %v", err))
	}
	return n
}

func toFloat(t *testing.T, v any) float64 {
	t.Helper()
	switch x := v.(type) {
	case float64:
		return x
	case []byte:
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			t.Fatalf("parse %q: %v", x, err)
		}
		return f
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			t.Fatalf("parse %q: %v", x, err)
		}
		return f
	case pgtype.Numeric:
		f, err := x.Float64Value()
		if err != nil {
			t.Fatalf("numeric→float: %v", err)
		}
		return f.Float64
	}
	t.Fatalf("unexpected dashboard avg type: %T (%v)", v, v)
	return 0
}
