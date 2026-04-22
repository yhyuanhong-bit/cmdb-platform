//go:build integration

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// D9-P1 integration tests: verify asset read paths bump the heat
// counter AND that heat-only updates do NOT fire the snapshot trigger
// (the 000058 WHEN clause is the lynchpin — without it, every read
// would insert a snapshot row and poison the history table).

type heatFixture struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	assetID  uuid.UUID
	assetID2 uuid.UUID
}

func setupHeatFixture(t *testing.T, pool *pgxpool.Pool) heatFixture {
	t.Helper()
	ctx := context.Background()
	fix := heatFixture{
		tenantID: uuid.New(),
		userID:   uuid.New(),
		assetID:  uuid.New(),
		assetID2: uuid.New(),
	}
	suf := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "heat-"+suf, "heat-"+suf); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userID, fix.tenantID, "heat-u-"+suf, "Heat U", "heat-"+suf+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type)
		 VALUES ($1, $3, $4, 'heat asset 1', 'server'),
		        ($2, $3, $5, 'heat asset 2', 'server')`,
		fix.assetID, fix.assetID2, fix.tenantID,
		"HEAT-A-"+suf, "HEAT-B-"+suf,
	); err != nil {
		t.Fatalf("insert assets: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM asset_snapshots WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

func newHeatServer(pool *pgxpool.Pool) *APIServer {
	q := dbgen.New(pool)
	svc := asset.NewService(q, nil, pool)
	return &APIServer{pool: pool, assetSvc: svc}
}

// TestIntegration_AssetHeat_BumpOnGet: N calls to GetAsset must leave
// access_count_24h = N on that row. BumpAccess is fire-and-forget so we
// poll for the counter to converge rather than asserting immediately.
func TestIntegration_AssetHeat_BumpOnGet(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupHeatFixture(t, pool)
	s := newHeatServer(pool)

	// The insert trigger creates one snapshot per new asset (it has no
	// WHEN clause, by design). Capture that baseline — heat-only
	// updates must not grow it.
	snapsBefore := countSnapshots(t, pool, fix.assetID)

	const N = 5
	for i := 0; i < N; i++ {
		c, rec := newDepCtx(t, http.MethodGet,
			"/assets/"+fix.assetID.String(),
			fix.tenantID, fix.userID, nil)
		s.GetAsset(c, IdPath(fix.assetID))
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d: status = %d, body=%s", i, rec.Code, rec.Body.String())
		}
	}

	if got := waitForCounter(t, pool, fix.assetID, N); got != N {
		t.Errorf("access_count_24h = %d, want %d", got, N)
	}

	// The real payoff: snapshot count should be unchanged. That's the
	// 000058 WHEN-filtered UPDATE trigger earning its keep.
	snapsAfter := countSnapshots(t, pool, fix.assetID)
	if snapsAfter != snapsBefore {
		t.Errorf("snapshot rows grew from %d to %d across %d GETs (WHEN clause should skip heat-only updates)", snapsBefore, snapsAfter, N)
	}
}

// TestIntegration_AssetHeat_BumpOnList: a successful /assets list must
// bump the heat counter on every returned asset in one batch UPDATE.
func TestIntegration_AssetHeat_BumpOnList(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupHeatFixture(t, pool)
	s := newHeatServer(pool)

	snapsBefore := countSnapshotsByTenant(t, pool, fix.tenantID)

	c, rec := newDepCtx(t, http.MethodGet,
		"/assets",
		fix.tenantID, fix.userID, nil)
	s.ListAssets(c, ListAssetsParams{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if got := waitForCounter(t, pool, fix.assetID, 1); got != 1 {
		t.Errorf("asset1 access_count_24h = %d, want 1", got)
	}
	if got := waitForCounter(t, pool, fix.assetID2, 1); got != 1 {
		t.Errorf("asset2 access_count_24h = %d, want 1", got)
	}

	snapsAfter := countSnapshotsByTenant(t, pool, fix.tenantID)
	if snapsAfter != snapsBefore {
		t.Errorf("snapshot rows grew from %d to %d across list bump (expected 0 growth)", snapsBefore, snapsAfter)
	}
}

// TestIntegration_AssetHeat_TenantScoped: bumping an asset must not
// leak across tenants even if the asset_id exists in a different
// tenant (defense-in-depth — the bump query has tenant_id in WHERE).
func TestIntegration_AssetHeat_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupHeatFixture(t, pool)
	s := newHeatServer(pool)

	otherTenant := uuid.New()
	suf := otherTenant.String()[:8]
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		otherTenant, "heat-other-"+suf, "heat-other-"+suf); err != nil {
		t.Fatalf("insert other tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, otherTenant)
	})

	c, rec := newDepCtx(t, http.MethodGet,
		"/assets/"+fix.assetID.String(),
		otherTenant, uuid.New(), nil)
	s.GetAsset(c, IdPath(fix.assetID))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant GET status = %d, want 404 — body=%s", rec.Code, rec.Body.String())
	}

	// Give any erroneous bump goroutine a chance to land, then assert
	// the counter is still zero.
	if got := waitForCounter(t, pool, fix.assetID, 0); got != 0 {
		t.Errorf("cross-tenant GET leaked into counter: got %d, want 0", got)
	}
}

func countSnapshots(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM asset_snapshots WHERE asset_id = $1`, assetID).Scan(&n); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	return n
}

func countSnapshotsByTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM asset_snapshots WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
		t.Fatalf("count snapshots by tenant: %v", err)
	}
	return n
}

// waitForCounter polls access_count_24h until it reaches want or the
// budget elapses. Returns the last observed value. Used instead of a
// fixed sleep because the fire-and-forget bump completes in
// microseconds on a warm connection but can slip to tens of ms under
// load. 500ms is generous for a local DB, short enough that a
// regression won't hang CI.
func waitForCounter(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID, want int) int {
	t.Helper()
	ctx := context.Background()
	const maxAttempts = 50
	var got int
	for i := 0; i < maxAttempts; i++ {
		if err := pool.QueryRow(ctx,
			`SELECT access_count_24h FROM assets WHERE id = $1`, assetID).Scan(&got); err != nil {
			t.Fatalf("select access_count_24h: %v", err)
		}
		if got == want {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	return got
}
