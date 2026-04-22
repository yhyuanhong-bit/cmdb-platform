//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// D10-P1 integration tests: verify the ?at=<RFC3339> query param on
// topology endpoints resolves against the soft-closed history stamped
// on asset_dependencies by migration 000057.
//
// Graph timeline (all within tenantA):
//
//	T1: INSERT edge A→B  (open)
//	T2: soft-close A→B   (UPDATE ... SET valid_to=now())
//	T3: INSERT edge A→C  (open)
//
// Expected point-in-time reads:
//
//	at=T1.5 → {A→B}           (old topology)
//	at=T2.5 → {}              (A→B closed, A→C not yet open)
//	at=T3.5 → {A→C}           (current)
//	at=<omitted> → {A→C}      (open edges only)
//
// Run with:
//
//	go test -tags integration -run TestIntegration_TopologyAt ./internal/api/...

type topoAtFixture struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	assetA   uuid.UUID
	assetB   uuid.UUID
	assetC   uuid.UUID
	depAB    uuid.UUID
	depAC    uuid.UUID
	t1       time.Time // moment A→B exists as the only edge
	t25      time.Time // moment between close(AB) and insert(AC)
	t35      time.Time // moment A→C exists as the only open edge
}

func setupTopoAtFixture(t *testing.T, pool *pgxpool.Pool) topoAtFixture {
	t.Helper()
	ctx := context.Background()
	fix := topoAtFixture{
		tenantID: uuid.New(),
		userID:   uuid.New(),
		assetA:   uuid.New(),
		assetB:   uuid.New(),
		assetC:   uuid.New(),
		depAB:    uuid.New(),
		depAC:    uuid.New(),
	}
	suf := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "topoat-"+suf, "topoat-"+suf); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userID, fix.tenantID, "topoat-u-"+suf, "Topo At U", "topoat-"+suf+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type)
		 VALUES ($1, $4, $5, 'asset A', 'server'),
		        ($2, $4, $6, 'asset B', 'server'),
		        ($3, $4, $7, 'asset C', 'server')`,
		fix.assetA, fix.assetB, fix.assetC,
		fix.tenantID,
		"TAG-A-"+suf, "TAG-B-"+suf, "TAG-C-"+suf,
	); err != nil {
		t.Fatalf("insert assets: %v", err)
	}

	// T1: open edge A→B with explicit valid_from so the test's
	// checkpoint times can sit on deterministic boundaries instead of
	// racing wall-clock `now()` jitter.
	baseT := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	t1 := baseT
	t2 := baseT.Add(10 * time.Minute)
	t3 := baseT.Add(20 * time.Minute)

	if _, err := pool.Exec(ctx,
		`INSERT INTO asset_dependencies
		 (id, tenant_id, source_asset_id, target_asset_id, dependency_type, description, valid_from, valid_to, created_at)
		 VALUES ($1, $2, $3, $4, 'depends_on', 'A depends on B', $5, $6, $5)`,
		fix.depAB, fix.tenantID, fix.assetA, fix.assetB,
		t1, t2,
	); err != nil {
		t.Fatalf("insert A→B: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO asset_dependencies
		 (id, tenant_id, source_asset_id, target_asset_id, dependency_type, description, valid_from, valid_to, created_at)
		 VALUES ($1, $2, $3, $4, 'depends_on', 'A depends on C', $5, NULL, $5)`,
		fix.depAC, fix.tenantID, fix.assetA, fix.assetC,
		t3,
	); err != nil {
		t.Fatalf("insert A→C: %v", err)
	}

	fix.t1 = t1.Add(5 * time.Minute) // halfway between t1 and t2 — A→B open
	fix.t25 = t2.Add(5 * time.Minute) // between t2 and t3 — gap: no open edges
	fix.t35 = t3.Add(5 * time.Minute) // after t3 — A→C open

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM asset_dependencies WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

type depsEnvelope struct {
	Data struct {
		Dependencies []struct {
			ID            string `json:"id"`
			SourceAssetID string `json:"source_asset_id"`
			TargetAssetID string `json:"target_asset_id"`
		} `json:"dependencies"`
	} `json:"data"`
}

func listDepsAt(t *testing.T, s *APIServer, fix topoAtFixture, assetID uuid.UUID, at *time.Time) depsEnvelope {
	t.Helper()
	q := url.Values{}
	q.Set("asset_id", assetID.String())
	if at != nil {
		q.Set("at", at.Format(time.RFC3339Nano))
	}
	c, rec := newDepCtx(t, http.MethodGet,
		"/topology/dependencies?"+q.Encode(),
		fix.tenantID, fix.userID, nil)

	assetUUID := openapi_types.UUID(assetID)
	params := ListAssetDependenciesParams{AssetId: &assetUUID}
	if at != nil {
		atCopy := *at
		params.At = &atCopy
	}
	s.ListAssetDependencies(c, params)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d — body=%s", rec.Code, rec.Body.String())
	}
	var env depsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	return env
}

// TestIntegration_TopologyAt_ListAssetDependencies is the core of D10-P1:
// the same asset_id at different `at` instants returns different graphs.
func TestIntegration_TopologyAt_ListAssetDependencies(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupTopoAtFixture(t, pool)
	s := &APIServer{pool: pool}

	// 1. Historical: at T1.5, only A→B is in effect.
	env := listDepsAt(t, s, fix, fix.assetA, &fix.t1)
	if len(env.Data.Dependencies) != 1 || env.Data.Dependencies[0].ID != fix.depAB.String() {
		t.Errorf("at T1.5 deps = %+v, want [%s]", env.Data.Dependencies, fix.depAB)
	}

	// 2. Gap: at T2.5, A→B is closed and A→C has not started yet.
	env = listDepsAt(t, s, fix, fix.assetA, &fix.t25)
	if len(env.Data.Dependencies) != 0 {
		t.Errorf("at T2.5 (gap) deps = %+v, want []", env.Data.Dependencies)
	}

	// 3. Future: at T3.5, A→C is the only open edge.
	env = listDepsAt(t, s, fix, fix.assetA, &fix.t35)
	if len(env.Data.Dependencies) != 1 || env.Data.Dependencies[0].ID != fix.depAC.String() {
		t.Errorf("at T3.5 deps = %+v, want [%s]", env.Data.Dependencies, fix.depAC)
	}

	// 4. No `at` param: current open edges — only A→C.
	env = listDepsAt(t, s, fix, fix.assetA, nil)
	if len(env.Data.Dependencies) != 1 || env.Data.Dependencies[0].ID != fix.depAC.String() {
		t.Errorf("current deps = %+v, want [%s]", env.Data.Dependencies, fix.depAC)
	}
}

// TestIntegration_TopologyAt_Impact verifies the recursive CTE variant
// honors valid_from/valid_to at every hop, using the same fixture.
// Impact from A with direction=downstream:
//
//	at T1.5 → 1 edge (A→B)
//	at T3.5 → 1 edge (A→C)
//	at T2.5 → 0 edges
func TestIntegration_TopologyAt_Impact(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupTopoAtFixture(t, pool)
	s := newImpactServer(pool)

	type impactEnvelope struct {
		Data struct {
			Edges []struct {
				Id            string `json:"id"`
				SourceAssetId string `json:"source_asset_id"`
				TargetAssetId string `json:"target_asset_id"`
			} `json:"edges"`
		} `json:"data"`
	}

	callImpact := func(at *time.Time) impactEnvelope {
		t.Helper()
		c, rec := newDepCtx(t, http.MethodGet, "/topology/impact",
			fix.tenantID, fix.userID, nil)
		rootID := openapi_types.UUID(fix.assetA)
		dir := GetTopologyImpactParamsDirectionDownstream
		params := GetTopologyImpactParams{
			RootAssetId: rootID,
			Direction:   &dir,
		}
		if at != nil {
			atCopy := *at
			params.At = &atCopy
		}
		s.GetTopologyImpact(c, params)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d — body=%s", rec.Code, rec.Body.String())
		}
		var env impactEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return env
	}

	env := callImpact(&fix.t1)
	if len(env.Data.Edges) != 1 || env.Data.Edges[0].Id != fix.depAB.String() {
		t.Errorf("impact at T1.5 = %+v, want [%s]", env.Data.Edges, fix.depAB)
	}

	env = callImpact(&fix.t25)
	if len(env.Data.Edges) != 0 {
		t.Errorf("impact at T2.5 (gap) = %+v, want []", env.Data.Edges)
	}

	env = callImpact(&fix.t35)
	if len(env.Data.Edges) != 1 || env.Data.Edges[0].Id != fix.depAC.String() {
		t.Errorf("impact at T3.5 = %+v, want [%s]", env.Data.Edges, fix.depAC)
	}
}
