//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/bia"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// These tests exercise the tier-propagation behavior of the BIA dependency
// handlers against a real Postgres. The contract under test (Phase 2.3 of
// the 2026-04-19 remediation roadmap) is:
//
//   - CreateBIADependency: after insert, every asset linked to the assessment
//     has its assets.bia_level rewritten to the highest connected tier.
//   - DeleteBIADependency: after delete, the dependency's former asset has
//     its bia_level recomputed from remaining links (fallback 'normal').
//   - Cross-tenant: a caller in tenant A cannot create or delete a
//     dependency that belongs to tenant B; 404 is returned and nothing in
//     tenant B changes.
//
// Run with:
//   go test -tags integration -run TestIntegration_BIA ./internal/api/...

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

type biaFixture struct {
	tenantID     uuid.UUID
	userID       uuid.UUID
	assessmentID uuid.UUID // tier = 'critical'
	assetID      uuid.UUID // starts at 'normal'
	// second assessment in the same tenant used to prove a delete falls
	// back to the OTHER assessment's tier rather than all the way to default.
	assessmentB uuid.UUID // tier = 'important'
}

// setupBIAFixture builds a tenant + user + asset + two assessments with
// different tiers. Cleanup runs via t.Cleanup so rows never leak.
func setupBIAFixture(t *testing.T, pool *pgxpool.Pool) biaFixture {
	t.Helper()
	ctx := context.Background()
	fix := biaFixture{
		tenantID:     uuid.New(),
		userID:       uuid.New(),
		assessmentID: uuid.New(),
		assetID:      uuid.New(),
		assessmentB:  uuid.New(),
	}
	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "bia-test-"+suffix, "bia-test-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userID, fix.tenantID,
		"bia-u-"+suffix, "BIA User "+suffix, "bia-"+suffix+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status, bia_level)
		 VALUES ($1, $2, $3, $4, 'server', 'deployed', 'normal')`,
		fix.assetID, fix.tenantID, "BIA-AST-"+suffix, "bia-asset-"+suffix); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO bia_assessments (id, tenant_id, system_name, system_code, bia_score, tier)
		 VALUES ($1, $2, $3, $4, 90, 'critical')`,
		fix.assessmentID, fix.tenantID, "bia-sys-"+suffix, "SYS-"+suffix); err != nil {
		t.Fatalf("insert assessment A: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO bia_assessments (id, tenant_id, system_name, system_code, bia_score, tier)
		 VALUES ($1, $2, $3, $4, 60, 'important')`,
		fix.assessmentB, fix.tenantID, "bia-sys-b-"+suffix, "SYSB-"+suffix); err != nil {
		t.Fatalf("insert assessment B: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM bia_dependencies WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM bia_assessments WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

// newBIATestServer wires an APIServer with a real bia.Service on top of pool.
// Everything else on the APIServer stays zero-valued; the BIA handlers never
// touch the other services.
func newBIATestServer(pool *pgxpool.Pool) *APIServer {
	queries := dbgen.New(pool)
	return &APIServer{
		pool:   pool,
		biaSvc: bia.NewService(queries, pool),
	}
}

// newBIACtx builds a gin context with the tenant/user IDs the handlers read
// out of auth middleware.
func newBIACtx(t *testing.T, method, target string, tenantID, userID uuid.UUID, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, target, nil)
	}
	c.Request = req
	c.Set("tenant_id", tenantID.String())
	c.Set("user_id", userID.String())
	return c, rec
}

// assetBIALevel reads back the raw bia_level column — the source of truth
// for "did propagation run".
func assetBIALevel(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID) string {
	t.Helper()
	var level string
	if err := pool.QueryRow(context.Background(),
		`SELECT bia_level FROM assets WHERE id = $1`, assetID,
	).Scan(&level); err != nil {
		t.Fatalf("read bia_level: %v", err)
	}
	return level
}

// ---------------------------------------------------------------------------
// 1. CreateBIADependency propagates 'critical' to the dependent asset.
// ---------------------------------------------------------------------------

func TestIntegration_BIA_CreateDependency_PropagatesTier(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupBIAFixture(t, pool)

	s := newBIATestServer(pool)

	// Precondition: asset starts at 'normal'.
	if got := assetBIALevel(t, pool, fix.assetID); got != "normal" {
		t.Fatalf("precondition: bia_level=%q, want 'normal'", got)
	}

	body := fmt.Sprintf(`{"asset_id":"%s","dependency_type":"runs_on"}`, fix.assetID)
	c, rec := newBIACtx(t, http.MethodPost,
		"/bia/assessments/"+fix.assessmentID.String()+"/dependencies",
		fix.tenantID, fix.userID, body)

	s.CreateBIADependency(c, IdPath(fix.assessmentID))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := assetBIALevel(t, pool, fix.assetID); got != "critical" {
		t.Fatalf("after create: bia_level=%q, want 'critical'", got)
	}
}

// ---------------------------------------------------------------------------
// 2. DeleteBIADependency recomputes tier from remaining links.
// ---------------------------------------------------------------------------
//
// Scenario:
//   - asset linked to assessmentA (critical) AND assessmentB (important)
//   - asset.bia_level should be 'critical' after both links exist
//   - delete the critical link → bia_level must drop to 'important'
//   - delete the important link → bia_level must drop to 'normal' (default)

func TestIntegration_BIA_DeleteDependency_RecomputesTier(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupBIAFixture(t, pool)

	ctx := context.Background()
	// Seed two dependencies directly — the create path is covered by the
	// test above, this one is about delete-time behavior.
	depCritical := uuid.New()
	depImportant := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO bia_dependencies (id, tenant_id, assessment_id, asset_id, dependency_type)
		 VALUES ($1, $2, $3, $4, 'runs_on'), ($5, $2, $6, $4, 'runs_on')`,
		depCritical, fix.tenantID, fix.assessmentID, fix.assetID,
		depImportant, fix.assessmentB); err != nil {
		t.Fatalf("seed deps: %v", err)
	}
	// Force the current bia_level to 'critical' to reflect the seeded
	// graph — propagation on future deletes must walk it back down.
	if _, err := pool.Exec(ctx,
		`UPDATE assets SET bia_level = 'critical' WHERE id = $1`, fix.assetID); err != nil {
		t.Fatalf("prime asset bia_level: %v", err)
	}

	s := newBIATestServer(pool)

	// Delete the 'critical' dep → bia_level should drop to 'important'.
	c, rec := newBIACtx(t, http.MethodDelete,
		"/bia/assessments/"+fix.assessmentID.String()+"/dependencies/"+depCritical.String(),
		fix.tenantID, fix.userID, "")
	s.DeleteBIADependency(c, IdPath(fix.assessmentID), openapi_types.UUID(depCritical))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete critical: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := assetBIALevel(t, pool, fix.assetID); got != "important" {
		t.Fatalf("after delete critical: bia_level=%q, want 'important'", got)
	}

	// Delete the remaining dep → asset has no more links, should fall to default.
	c2, rec2 := newBIACtx(t, http.MethodDelete,
		"/bia/assessments/"+fix.assessmentB.String()+"/dependencies/"+depImportant.String(),
		fix.tenantID, fix.userID, "")
	s.DeleteBIADependency(c2, IdPath(fix.assessmentB), openapi_types.UUID(depImportant))

	if rec2.Code != http.StatusNoContent {
		t.Fatalf("delete important: status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if got := assetBIALevel(t, pool, fix.assetID); got != "normal" {
		t.Fatalf("after delete all: bia_level=%q, want 'normal'", got)
	}
}

// ---------------------------------------------------------------------------
// 3. Cross-tenant create: tenant A cannot attach to tenant B's assessment.
// ---------------------------------------------------------------------------

func TestIntegration_BIA_CreateDependency_CrossTenant_Returns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	tenantA := setupBIAFixture(t, pool)
	tenantB := setupBIAFixture(t, pool)

	// Baseline: tenantB's asset starts at 'normal'.
	if got := assetBIALevel(t, pool, tenantB.assetID); got != "normal" {
		t.Fatalf("baseline tenantB: bia_level=%q", got)
	}

	s := newBIATestServer(pool)

	// Caller authenticated as tenantA targets tenantB's assessment +
	// tenantB's asset. Must 404 and leave tenantB untouched.
	body := fmt.Sprintf(`{"asset_id":"%s","dependency_type":"runs_on"}`, tenantB.assetID)
	c, rec := newBIACtx(t, http.MethodPost,
		"/bia/assessments/"+tenantB.assessmentID.String()+"/dependencies",
		tenantA.tenantID, tenantA.userID, body)
	s.CreateBIADependency(c, IdPath(tenantB.assessmentID))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant create: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	// No propagation happened on tenantB.
	if got := assetBIALevel(t, pool, tenantB.assetID); got != "normal" {
		t.Fatalf("tenantB asset leaked: bia_level=%q, want 'normal'", got)
	}
	// And no dependency row was inserted under either tenant.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM bia_dependencies WHERE assessment_id = $1`,
		tenantB.assessmentID).Scan(&count); err != nil {
		t.Fatalf("count deps: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 deps on tenantB assessment, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// 4. Cross-tenant delete: tenant A cannot delete tenant B's dependency.
// ---------------------------------------------------------------------------

func TestIntegration_BIA_DeleteDependency_CrossTenant_Returns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	tenantA := setupBIAFixture(t, pool)
	tenantB := setupBIAFixture(t, pool)

	ctx := context.Background()
	// Seed a dep in tenantB + prime its asset to 'critical' to make any
	// accidental recompute visible in the final assertion.
	depID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO bia_dependencies (id, tenant_id, assessment_id, asset_id, dependency_type)
		 VALUES ($1, $2, $3, $4, 'runs_on')`,
		depID, tenantB.tenantID, tenantB.assessmentID, tenantB.assetID); err != nil {
		t.Fatalf("seed dep: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE assets SET bia_level = 'critical' WHERE id = $1`, tenantB.assetID); err != nil {
		t.Fatalf("prime asset: %v", err)
	}

	s := newBIATestServer(pool)

	// tenantA tries to delete tenantB's dep.
	c, rec := newBIACtx(t, http.MethodDelete,
		"/bia/assessments/"+tenantB.assessmentID.String()+"/dependencies/"+depID.String(),
		tenantA.tenantID, tenantA.userID, "")
	s.DeleteBIADependency(c, IdPath(tenantB.assessmentID), openapi_types.UUID(depID))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant delete: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	// Dep still exists on tenantB, asset level unchanged.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM bia_dependencies WHERE id = $1`, depID).Scan(&count); err != nil {
		t.Fatalf("count deps: %v", err)
	}
	if count != 1 {
		t.Fatalf("dep deleted across tenants: count=%d, want 1", count)
	}
	if got := assetBIALevel(t, pool, tenantB.assetID); got != "critical" {
		t.Fatalf("tenantB asset mutated: bia_level=%q, want 'critical'", got)
	}
}

// ---------------------------------------------------------------------------
// 5. Propagation is atomic with the dependency write (sanity).
//
// This asserts the observable post-condition: after a successful create,
// the dep and the asset update are both visible. If the propagation SQL
// errored, the service returns an error and the dep is rolled back — this
// test just makes sure the happy path commits both sides.
// ---------------------------------------------------------------------------

func TestIntegration_BIA_CreateDependency_AtomicWithPropagation(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupBIAFixture(t, pool)

	s := newBIATestServer(pool)

	body := fmt.Sprintf(`{"asset_id":"%s","dependency_type":"runs_on"}`, fix.assetID)
	c, rec := newBIACtx(t, http.MethodPost,
		"/bia/assessments/"+fix.assessmentID.String()+"/dependencies",
		fix.tenantID, fix.userID, body)
	s.CreateBIADependency(c, IdPath(fix.assessmentID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Read the dep back by decoding the response (proves the write happened
	// with a real id) and confirm its asset is now 'critical'.
	var env struct {
		Data struct {
			ID openapi_types.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — %s", err, rec.Body.String())
	}
	if uuid.UUID(env.Data.ID) == uuid.Nil {
		t.Fatalf("created dep id is zero — write did not commit")
	}
	if got := assetBIALevel(t, pool, fix.assetID); got != "critical" {
		t.Fatalf("after atomic create: bia_level=%q, want 'critical'", got)
	}
}
