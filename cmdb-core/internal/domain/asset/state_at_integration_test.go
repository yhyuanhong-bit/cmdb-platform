package asset_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// D10-P0 integration test: verifies the assets_snapshot_after_write
// trigger fires on every mutation AND that GetStateAt correctly picks
// the last snapshot at or before a requested instant. Uses a real DB
// because the whole point of the feature is the trigger — a sqlmock
// would hide the most load-bearing piece of the implementation.

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

func newTestPool(t *testing.T) *pgxpool.Pool {
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

// seedTenant creates a tenant row owned by the test so rows insert-able
// cleanly under the tenant_id FK. Returns the tenant UUID.
func seedTenant(t *testing.T, pool *pgxpool.Pool, ctx context.Context) uuid.UUID {
	t.Helper()
	tid := uuid.New()
	slug := "snap-" + tid.String()[:8]
	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		tid, slug, slug)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	t.Cleanup(func() {
		// cascade cleanup via tenant_id FK; delete children first where FK
		// is not ON DELETE CASCADE.
		_, _ = pool.Exec(context.Background(), `DELETE FROM asset_snapshots WHERE tenant_id = $1`, tid)
		_, _ = pool.Exec(context.Background(), `DELETE FROM assets WHERE tenant_id = $1`, tid)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tid)
	})
	return tid
}

func TestGetStateAt_PicksLatestBeforeRequestedTime(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID := seedTenant(t, pool, ctx)
	queries := dbgen.New(pool)
	svc := asset.NewService(queries, nil, pool)

	// 1. Initial create — trigger fires, produces snapshot #1.
	assetID := uuid.New()
	_, err := pool.Exec(ctx, `
        INSERT INTO assets (id, tenant_id, asset_tag, name, type, status, bia_level)
        VALUES ($1, $2, $3, 'test-asset', 'server', 'inventoried', 'normal')
    `, assetID, tenantID, "SNAP-"+assetID.String()[:8])
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // guarantee monotonic valid_at ordering

	// 2. First mutation: status=operational at T2.
	if _, err := pool.Exec(ctx,
		`UPDATE assets SET status='operational' WHERE id=$1`, assetID); err != nil {
		t.Fatalf("update #1: %v", err)
	}
	midCheckpoint := time.Now()
	time.Sleep(50 * time.Millisecond)

	// 3. Second mutation: status=retired at T3. GetStateAt(midCheckpoint)
	//    must still return 'operational', not this row.
	if _, err := pool.Exec(ctx,
		`UPDATE assets SET status='retired' WHERE id=$1`, assetID); err != nil {
		t.Fatalf("update #2: %v", err)
	}

	// 4. Query at midCheckpoint — must return the 'operational' snapshot,
	//    proving the trigger wrote all three states AND the <= predicate
	//    correctly filters out the newer retirement.
	snap, err := svc.GetStateAt(ctx, tenantID, assetID, midCheckpoint)
	if err != nil {
		t.Fatalf("GetStateAt: %v", err)
	}
	if snap.Status != "operational" {
		t.Errorf("status at mid-checkpoint = %q, want operational", snap.Status)
	}
	if snap.ValidAt.After(midCheckpoint) {
		t.Errorf("valid_at %v is after checkpoint %v — <= predicate broken",
			snap.ValidAt, midCheckpoint)
	}

	// 5. Query "before everything" returns ErrSnapshotNotFound.
	_, err = svc.GetStateAt(ctx, tenantID, assetID, time.Unix(0, 0))
	if !errors.Is(err, asset.ErrSnapshotNotFound) {
		t.Errorf("pre-creation query should return ErrSnapshotNotFound, got: %v", err)
	}

	// 6. Query "now" returns the latest (retired) state.
	snap, err = svc.GetStateAt(ctx, tenantID, assetID, time.Now())
	if err != nil {
		t.Fatalf("GetStateAt now: %v", err)
	}
	if snap.Status != "retired" {
		t.Errorf("latest state = %q, want retired", snap.Status)
	}
}

func TestListSnapshots_ReturnsNewestFirst(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID := seedTenant(t, pool, ctx)
	queries := dbgen.New(pool)
	svc := asset.NewService(queries, nil, pool)

	assetID := uuid.New()
	if _, err := pool.Exec(ctx, `
        INSERT INTO assets (id, tenant_id, asset_tag, name, type, status, bia_level)
        VALUES ($1, $2, $3, 'list-test', 'server', 'inventoried', 'normal')
    `, assetID, tenantID, "LIST-"+assetID.String()[:8]); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	for _, s := range []string{"operational", "maintenance", "retired"} {
		time.Sleep(20 * time.Millisecond)
		if _, err := pool.Exec(ctx,
			`UPDATE assets SET status=$1 WHERE id=$2`, s, assetID); err != nil {
			t.Fatalf("update to %s: %v", s, err)
		}
	}

	snaps, err := svc.ListSnapshots(ctx, tenantID, assetID, 0)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 4 {
		t.Fatalf("expected 4 snapshots (insert + 3 updates), got %d", len(snaps))
	}
	// Newest first — the first row must be retired.
	if snaps[0].Status != "retired" {
		t.Errorf("first row = %q, want retired (newest-first ordering broken)", snaps[0].Status)
	}
	// Timestamps must be strictly decreasing.
	for i := 1; i < len(snaps); i++ {
		if !snaps[i].ValidAt.Before(snaps[i-1].ValidAt) {
			t.Errorf("non-monotonic ordering at i=%d: %v then %v",
				i, snaps[i-1].ValidAt, snaps[i].ValidAt)
		}
	}
}
