//go:build integration

package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests are regressions for four cross-tenant IDOR vulnerabilities
// discovered in impl_prediction_upgrades.go and impl_topology.go where
// the WHERE clause matched on id without also filtering by tenant_id.
//
// Run with:
//   go test -tags integration -run TestTenantIsolation ./internal/api/...
//
// TEST_DATABASE_URL can override the default docker-compose connection.

// tenantIsolationFixture holds two independent tenants (A and B) plus a single
// owned asset for tenant A, used as the target of cross-tenant access attempts.
type tenantIsolationFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID
	assetA  uuid.UUID
}

// setupTenantIsolationFixture creates two tenants and one asset owned by tenant A.
// The asset is used both as the RUL target and as source/target for dependency tests.
func setupTenantIsolationFixture(t *testing.T, pool *pgxpool.Pool) tenantIsolationFixture {
	t.Helper()
	ctx := context.Background()
	fix := tenantIsolationFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		assetA:  uuid.New(),
	}
	suffix := fix.tenantA.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "iso-A-"+suffix, "iso-a-"+suffix,
		fix.tenantB, "iso-B-"+suffix, "iso-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, 'iso-asset', 'server', 'active')`,
		fix.assetA, fix.tenantA, "ISO-TAG-"+suffix,
	); err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM asset_dependencies WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM upgrade_rules WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

// newCtxAsTenant wires a gin.Context that simulates auth middleware having
// injected the given tenant_id and a random user_id.
func newCtxAsTenant(t *testing.T, method, target string, tenantID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	c.Set("tenant_id", tenantID.String())
	c.Set("user_id", uuid.New().String())
	return c, rec
}

// ---------------------------------------------------------------------------
// GetAssetRUL — impl_prediction_upgrades.go:39
// ---------------------------------------------------------------------------

func TestTenantIsolation_GetAssetRUL_BlocksCrossTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupTenantIsolationFixture(t, pool)

	s := &APIServer{pool: pool}
	// Caller is tenant B, target asset belongs to tenant A.
	c, rec := newCtxAsTenant(t, http.MethodGet, "/prediction/rul/"+fix.assetA.String(), fix.tenantB)
	s.GetAssetRUL(c, IdPath(fix.assetA))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant GetAssetRUL leaked: status = %d, want 404 — body=%s",
			rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// UpdateUpgradeRule — impl_prediction_upgrades.go:668
// ---------------------------------------------------------------------------

func TestTenantIsolation_UpdateUpgradeRule_BlocksCrossTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupTenantIsolationFixture(t, pool)

	// Insert a rule owned by tenant A.
	ruleID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO upgrade_rules (id, tenant_id, asset_type, category, metric_name, threshold, recommendation)
		 VALUES ($1, $2, 'server', 'cpu', 'cpu_usage', 80.00, 'orig recommendation')`,
		ruleID, fix.tenantA,
	); err != nil {
		t.Fatalf("insert rule: %v", err)
	}

	// Tenant B attempts to update it via JSON body.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodPut,
		"/upgrade-rules/"+ruleID.String(),
		bytes.NewReader([]byte(`{"recommendation":"hijacked"}`)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("tenant_id", fix.tenantB.String())
	c.Set("user_id", uuid.New().String())

	s := &APIServer{pool: pool}
	s.UpdateUpgradeRule(c, IdPath(ruleID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant UpdateUpgradeRule accepted: status = %d, want 404 — body=%s",
			rec.Code, rec.Body.String())
	}

	// Verify the rule's recommendation was NOT modified.
	var rec2 string
	if err := pool.QueryRow(context.Background(),
		`SELECT recommendation FROM upgrade_rules WHERE id = $1`, ruleID).Scan(&rec2); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if rec2 != "orig recommendation" {
		t.Errorf("rule mutated across tenants: got %q, want %q", rec2, "orig recommendation")
	}
}

// ---------------------------------------------------------------------------
// DeleteUpgradeRule — impl_prediction_upgrades.go:704
// ---------------------------------------------------------------------------

func TestTenantIsolation_DeleteUpgradeRule_BlocksCrossTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupTenantIsolationFixture(t, pool)

	// Insert a rule owned by tenant A.
	ruleID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO upgrade_rules (id, tenant_id, asset_type, category, metric_name, threshold, recommendation)
		 VALUES ($1, $2, 'server', 'memory', 'memory_usage', 85.00, 'rec')`,
		ruleID, fix.tenantA,
	); err != nil {
		t.Fatalf("insert rule: %v", err)
	}

	// Tenant B attempts to delete it.
	c, rec := newCtxAsTenant(t, http.MethodDelete, "/upgrade-rules/"+ruleID.String(), fix.tenantB)
	s := &APIServer{pool: pool}
	s.DeleteUpgradeRule(c, IdPath(ruleID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant DeleteUpgradeRule accepted: status = %d, want 404 — body=%s",
			rec.Code, rec.Body.String())
	}

	// Verify the rule still exists.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM upgrade_rules WHERE id = $1`, ruleID).Scan(&count); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 1 {
		t.Errorf("rule was deleted across tenants: count=%d, want 1", count)
	}
}

// ---------------------------------------------------------------------------
// DeleteAssetDependency — impl_topology.go:115
// ---------------------------------------------------------------------------

func TestTenantIsolation_DeleteAssetDependency_BlocksCrossTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupTenantIsolationFixture(t, pool)

	// Second asset required to satisfy CHECK(source != target).
	assetA2 := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, 'iso-asset-2', 'server', 'active')`,
		assetA2, fix.tenantA, "ISO-TAG-2-"+fix.tenantA.String()[:8],
	); err != nil {
		t.Fatalf("insert asset2: %v", err)
	}

	// Dependency owned by tenant A.
	depID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO asset_dependencies (id, tenant_id, source_asset_id, target_asset_id, dependency_type)
		 VALUES ($1, $2, $3, $4, 'depends_on')`,
		depID, fix.tenantA, fix.assetA, assetA2,
	); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// Tenant B attempts to delete it.
	c, rec := newCtxAsTenant(t, http.MethodDelete, "/topology/dependencies/"+depID.String(), fix.tenantB)
	s := &APIServer{pool: pool}
	s.DeleteAssetDependency(c, IdPath(depID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant DeleteAssetDependency accepted: status = %d, want 404 — body=%s",
			rec.Code, rec.Body.String())
	}

	// Verify the dependency still exists.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM asset_dependencies WHERE id = $1`, depID).Scan(&count); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 1 {
		t.Errorf("dependency was deleted across tenants: count=%d, want 1", count)
	}
}
