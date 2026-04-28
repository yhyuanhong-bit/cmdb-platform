//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cross-tenant isolation tests for the asset/rack write/read paths
// flagged by the 2026-04-28 Team A audit:
//
//   C1: UpdateAsset sqlc query "WHERE id = $X" had no tenant_id —
//       any caller could PATCH any asset by UUID.
//   C2: ListAssetsByRack sqlc query "WHERE rack_id = $1" had no
//       tenant_id — any caller could enumerate any rack's assets.
//
// Run with:
//   go test -tags integration -run TestIntegration_AssetTenantIsolation ./internal/api/...

type assetIsoFixture struct {
	tenantA  uuid.UUID
	tenantB  uuid.UUID
	userA    uuid.UUID
	rackA    uuid.UUID
	rackB    uuid.UUID
	locA     uuid.UUID
	locB     uuid.UUID
	assetA   uuid.UUID
	assetB   uuid.UUID // mounted in rackB
}

func setupAssetIsoFixture(t *testing.T, pool *pgxpool.Pool) assetIsoFixture {
	t.Helper()
	ctx := context.Background()
	fix := assetIsoFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		userA:   uuid.New(),
		rackA:   uuid.New(),
		rackB:   uuid.New(),
		locA:    uuid.New(),
		locB:    uuid.New(),
		assetA:  uuid.New(),
		assetB:  uuid.New(),
	}
	sufA := fix.tenantA.String()[:8]
	sufB := fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "iso-A-"+sufA, "iso-A-"+sufA,
		fix.tenantB, "iso-B-"+sufB, "iso-B-"+sufB,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userA, fix.tenantA, "iso-uA-"+sufA, "Iso UA", "iso-A-"+sufA+"@test.local",
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO locations (id, tenant_id, name, slug, level)
		 VALUES ($1, $2, $3, $4, 'room'), ($5, $6, $7, $8, 'room')`,
		fix.locA, fix.tenantA, "loc-A-"+sufA, "loc-A-"+sufA,
		fix.locB, fix.tenantB, "loc-B-"+sufB, "loc-B-"+sufB,
	); err != nil {
		t.Fatalf("insert locations: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO racks (id, tenant_id, location_id, name, total_u)
		 VALUES ($1, $2, $3, $4, 42), ($5, $6, $7, $8, 42)`,
		fix.rackA, fix.tenantA, fix.locA, "rack-A-"+sufA,
		fix.rackB, fix.tenantB, fix.locB, "rack-B-"+sufB,
	); err != nil {
		t.Fatalf("insert racks: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, rack_id, status)
		 VALUES ($1, $2, $3, 'iso asset A', 'server', $4, 'active'),
		        ($5, $6, $7, 'iso asset B', 'server', $8, 'active')`,
		fix.assetA, fix.tenantA, "ISO-A-"+sufA, fix.rackA,
		fix.assetB, fix.tenantB, "ISO-B-"+sufB, fix.rackB,
	); err != nil {
		t.Fatalf("insert assets: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM asset_snapshots WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM racks WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM locations WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

func newIsoServer(pool *pgxpool.Pool) *APIServer {
	q := dbgen.New(pool)
	assetSvc := asset.NewService(q, nil, pool)
	topoSvc := topology.NewService(q, pool)
	return &APIServer{pool: pool, assetSvc: assetSvc, topologySvc: topoSvc}
}

// TestIntegration_AssetTenantIsolation_UpdateAssetCrossTenantBlocked
// pins C1: tenantA cannot PATCH tenantB's asset. Before the fix this
// returned 200 with the row mutated. After the fix it must return 404
// and tenantB's row must be unchanged.
func TestIntegration_AssetTenantIsolation_UpdateAssetCrossTenantBlocked(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupAssetIsoFixture(t, pool)
	s := newIsoServer(pool)

	body := []byte(`{"name":"PWNED-from-tenantA"}`)
	c, rec := newDepCtx(t, http.MethodPut,
		"/assets/"+fix.assetB.String(),
		fix.tenantA, fix.userA, body)
	c.Request.Body = http.NoBody
	// reattach so handler's ShouldBindJSON sees the payload
	c.Request, _ = http.NewRequest(http.MethodPut, "/assets/"+fix.assetB.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.UpdateAsset(c, IdPath(fix.assetB))

	if rec.Code == http.StatusOK {
		t.Fatalf("CRITICAL: tenantA was allowed to PATCH tenantB's asset (status=200, body=%s)", rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — body=%s", rec.Code, rec.Body.String())
	}

	var name string
	if err := pool.QueryRow(context.Background(),
		`SELECT name FROM assets WHERE id = $1`, fix.assetB).Scan(&name); err != nil {
		t.Fatalf("select tenantB asset: %v", err)
	}
	if name == "PWNED-from-tenantA" {
		t.Fatalf("CRITICAL: tenantB asset name was overwritten by tenantA — got %q", name)
	}
}

// TestIntegration_AssetTenantIsolation_ListAssetsByRackCrossTenantBlocked
// pins C2: tenantA cannot enumerate tenantB's rack assets via the
// /racks/{id}/assets endpoint. Before the fix the JSON body would
// contain assetB. After the fix it must be empty.
func TestIntegration_AssetTenantIsolation_ListAssetsByRackCrossTenantBlocked(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupAssetIsoFixture(t, pool)
	s := newIsoServer(pool)

	c, rec := newDepCtx(t, http.MethodGet,
		"/racks/"+fix.rackB.String()+"/assets",
		fix.tenantA, fix.userA, nil)
	s.ListRackAssets(c, IdPath(fix.rackB))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if len(env.Data) != 0 {
		t.Fatalf("CRITICAL: tenantA got %d assets back from tenantB rack — body=%s",
			len(env.Data), rec.Body.String())
	}
}

// TestIntegration_AssetTenantIsolation_UpdateAssetOwnTenantOK confirms
// the legitimate path still works: tenantA can PATCH its own asset.
func TestIntegration_AssetTenantIsolation_UpdateAssetOwnTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupAssetIsoFixture(t, pool)
	s := newIsoServer(pool)

	body := []byte(`{"name":"renamed-by-owner"}`)
	c, rec := newDepCtx(t, http.MethodPut,
		"/assets/"+fix.assetA.String(),
		fix.tenantA, fix.userA, body)

	s.UpdateAsset(c, IdPath(fix.assetA))

	if rec.Code != http.StatusOK {
		t.Fatalf("own-tenant update failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var name string
	if err := pool.QueryRow(context.Background(),
		`SELECT name FROM assets WHERE id = $1`, fix.assetA).Scan(&name); err != nil {
		t.Fatalf("select tenantA asset: %v", err)
	}
	if name != "renamed-by-owner" {
		t.Fatalf("own-tenant update did not persist — got %q, want renamed-by-owner", name)
	}
}

// TestIntegration_AssetTenantIsolation_ListAssetsByRackOwnTenantOK
// confirms tenantA still sees its own rack's assets.
func TestIntegration_AssetTenantIsolation_ListAssetsByRackOwnTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupAssetIsoFixture(t, pool)
	s := newIsoServer(pool)

	c, rec := newDepCtx(t, http.MethodGet,
		"/racks/"+fix.rackA.String()+"/assets",
		fix.tenantA, fix.userA, nil)
	s.ListRackAssets(c, IdPath(fix.rackA))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if len(env.Data) != 1 || env.Data[0].ID != fix.assetA.String() {
		t.Fatalf("own-rack list wrong: got %+v, want [%s]", env.Data, fix.assetA)
	}
}
