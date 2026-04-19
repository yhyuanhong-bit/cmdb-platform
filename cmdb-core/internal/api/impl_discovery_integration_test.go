//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise POST /discovery/{id}/approve end-to-end against a
// real Postgres. They verify the four contracts the remediation roadmap 2.2
// calls out:
//
//   1. Approve actually creates an assets row (the missing INSERT was the
//      whole reason for the refactor).
//   2. Idempotency on approved_asset_id — second approve returns same asset
//      without double-creating.
//   3. Cross-tenant approve returns 404 and writes no asset.
//   4. Duplicate asset_tag collision returns 409 AND rolls back the
//      discovered_assets.status flip.
//
// Run with:
//   go test -tags integration -run TestApproveDiscoveredAsset_Integration ./internal/api/...

type approveFixture struct {
	tenantA      uuid.UUID
	tenantB      uuid.UUID
	userA        uuid.UUID
	discoveredID uuid.UUID
}

// setupApproveFixture inserts two tenants, one user for tenant A, and one
// pending discovered_asset for tenant A. Cleanup deletes the created asset
// (if any), the discovered_asset, the user, and the tenants.
func setupApproveFixture(t *testing.T, pool *pgxpool.Pool) approveFixture {
	t.Helper()
	ctx := context.Background()
	fix := approveFixture{
		tenantA:      uuid.New(),
		tenantB:      uuid.New(),
		userA:        uuid.New(),
		discoveredID: uuid.New(),
	}
	suffix := fix.tenantA.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "app-A-"+suffix, "app-a-"+suffix,
		fix.tenantB, "app-B-"+suffix, "app-b-"+suffix); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userA, fix.tenantA,
		"app-user-"+suffix, "User "+suffix, "app-"+suffix+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO discovered_assets (id, tenant_id, source, hostname, ip_address, raw_data, status)
		 VALUES ($1, $2, 'integration-test', $3, '10.0.0.5', '{"cpu":"8 cores"}'::jsonb, 'pending')`,
		fix.discoveredID, fix.tenantA, "host-"+suffix); err != nil {
		t.Fatalf("insert discovered_asset: %v", err)
	}

	t.Cleanup(func() {
		// Delete any asset produced by approval first (FK from discovered_assets.approved_asset_id).
		_, _ = pool.Exec(ctx, `UPDATE discovered_assets SET approved_asset_id = NULL WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_events WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM discovered_assets WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, fix.userA)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

// newApproveTestServer wires an APIServer pointed at the real pool with a
// real discovery.Service on top. Event publish is disabled (nil bus) to
// keep tests deterministic — the handler tolerates a nil bus.
func newApproveTestServer(pool *pgxpool.Pool) *APIServer {
	queries := dbgen.New(pool)
	discSvc := discovery.NewService(queries, pool)
	return &APIServer{
		pool:         pool,
		discoverySvc: discSvc,
	}
}

func newApproveCtx(t *testing.T, tenantID, userID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodPost, "/discovery/approve", nil)
	c.Request = req
	c.Set("tenant_id", tenantID.String())
	c.Set("user_id", userID.String())
	return c, rec
}

// countAssetsForTenant returns the number of non-deleted assets for the
// given tenant. Used to assert idempotent approve does not double-insert.
func countAssetsForTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID,
	).Scan(&n); err != nil {
		t.Fatalf("count assets: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// 1. Happy path
// ---------------------------------------------------------------------------

// TestApproveDiscoveredAsset_Integration_CreatesAssetRow verifies the core
// fix: a successful approve produces exactly one new assets row with the
// right tenant_id and a 'discovery' marker in attributes.
func TestApproveDiscoveredAsset_Integration_CreatesAssetRow(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupApproveFixture(t, pool)

	s := newApproveTestServer(pool)
	c, rec := newApproveCtx(t, fix.tenantA, fix.userA)

	before := countAssetsForTenant(t, pool, fix.tenantA)
	s.ApproveDiscoveredAsset(c, IdPath(fix.discoveredID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	after := countAssetsForTenant(t, pool, fix.tenantA)
	if after != before+1 {
		t.Fatalf("assets count for tenant A = %d, want %d (before=%d)", after, before+1, before)
	}

	// Verify the link was recorded on discovered_assets.
	var approvedAssetID *uuid.UUID
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, approved_asset_id FROM discovered_assets WHERE id = $1`, fix.discoveredID,
	).Scan(&status, &approvedAssetID); err != nil {
		t.Fatalf("fetch discovered_asset: %v", err)
	}
	if status != "approved" {
		t.Errorf("discovered_asset.status = %q, want 'approved'", status)
	}
	if approvedAssetID == nil {
		t.Fatalf("approved_asset_id is NULL — link not written")
	}

	// Verify the new asset's tenant_id + provenance.
	var tenantID uuid.UUID
	var attrsRaw []byte
	var assetTag string
	if err := pool.QueryRow(context.Background(),
		`SELECT tenant_id, asset_tag, attributes FROM assets WHERE id = $1`, *approvedAssetID,
	).Scan(&tenantID, &assetTag, &attrsRaw); err != nil {
		t.Fatalf("fetch new asset: %v", err)
	}
	if tenantID != fix.tenantA {
		t.Errorf("new asset tenant_id = %v, want %v", tenantID, fix.tenantA)
	}
	var attrs map[string]any
	if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
		t.Fatalf("attributes not JSON: %v", err)
	}
	disc, _ := attrs["discovery"].(map[string]any)
	if disc == nil {
		t.Fatalf("attributes.discovery missing: %s", string(attrsRaw))
	}
	if disc["source"] != "integration-test" {
		t.Errorf("attributes.discovery.source = %v, want 'integration-test'", disc["source"])
	}
	if disc["discovered_asset_id"] != fix.discoveredID.String() {
		t.Errorf("attributes.discovery.discovered_asset_id = %v, want %s",
			disc["discovered_asset_id"], fix.discoveredID.String())
	}
}

// ---------------------------------------------------------------------------
// 2. Idempotency
// ---------------------------------------------------------------------------

// TestApproveDiscoveredAsset_Integration_IdempotentOnRetry verifies that
// calling approve a second time on an already-approved discovered_asset
// returns 200 and does NOT create a second assets row.
func TestApproveDiscoveredAsset_Integration_IdempotentOnRetry(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupApproveFixture(t, pool)

	s := newApproveTestServer(pool)

	// First approve.
	c1, rec1 := newApproveCtx(t, fix.tenantA, fix.userA)
	s.ApproveDiscoveredAsset(c1, IdPath(fix.discoveredID))
	c1.Writer.WriteHeaderNow()
	if rec1.Code != http.StatusOK {
		t.Fatalf("first approve status = %d, want 200 — body=%s", rec1.Code, rec1.Body.String())
	}
	afterFirst := countAssetsForTenant(t, pool, fix.tenantA)
	if afterFirst != 1 {
		t.Fatalf("after first approve: assets = %d, want 1", afterFirst)
	}

	// Second approve — same id, same tenant, same user. Must return 200
	// without creating another asset row.
	c2, rec2 := newApproveCtx(t, fix.tenantA, fix.userA)
	s.ApproveDiscoveredAsset(c2, IdPath(fix.discoveredID))
	c2.Writer.WriteHeaderNow()
	if rec2.Code != http.StatusOK {
		t.Fatalf("second approve status = %d, want 200 — body=%s", rec2.Code, rec2.Body.String())
	}
	afterSecond := countAssetsForTenant(t, pool, fix.tenantA)
	if afterSecond != 1 {
		t.Fatalf("after idempotent retry: assets = %d, want 1 (double-creation bug)", afterSecond)
	}
}

// ---------------------------------------------------------------------------
// 3. Cross-tenant isolation
// ---------------------------------------------------------------------------

// TestApproveDiscoveredAsset_Integration_CrossTenantReturns404 verifies
// tenant B cannot approve tenant A's discovered_asset. The response must
// be 404 (not 403 — that would leak existence), no asset row is created,
// and the discovered_asset stays 'pending'.
func TestApproveDiscoveredAsset_Integration_CrossTenantReturns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupApproveFixture(t, pool)

	// Tenant B, impersonating a random user, tries to approve tenant A's row.
	attackerUser := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, 'Attacker', $4, 'x')`,
		attackerUser, fix.tenantB, "attacker-"+fix.tenantB.String()[:8],
		"attacker-"+fix.tenantB.String()[:8]+"@test.local"); err != nil {
		t.Fatalf("insert attacker: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, attackerUser)
	})

	s := newApproveTestServer(pool)
	c, rec := newApproveCtx(t, fix.tenantB, attackerUser)
	s.ApproveDiscoveredAsset(c, IdPath(fix.discoveredID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant approve leaked: status = %d, want 404 — body=%s",
			rec.Code, rec.Body.String())
	}

	// No asset created for EITHER tenant.
	if n := countAssetsForTenant(t, pool, fix.tenantA); n != 0 {
		t.Errorf("tenant A assets after cross-tenant approve = %d, want 0", n)
	}
	if n := countAssetsForTenant(t, pool, fix.tenantB); n != 0 {
		t.Errorf("tenant B assets after cross-tenant approve = %d, want 0", n)
	}

	// discovered_asset unchanged.
	var status string
	var approvedAssetID *uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT status, approved_asset_id FROM discovered_assets WHERE id = $1`, fix.discoveredID,
	).Scan(&status, &approvedAssetID); err != nil {
		t.Fatalf("fetch discovered_asset: %v", err)
	}
	if status != "pending" {
		t.Errorf("discovered_asset.status = %q, want 'pending' (cross-tenant must not mutate)", status)
	}
	if approvedAssetID != nil {
		t.Errorf("approved_asset_id = %v, want NULL", *approvedAssetID)
	}
}

// ---------------------------------------------------------------------------
// 4. Duplicate asset rollback
// ---------------------------------------------------------------------------

// TestApproveDiscoveredAsset_Integration_DuplicateAssetRollsBack verifies
// that if the INSERT into assets fails because of a unique-constraint
// violation (synthesized here by pre-inserting an asset with the exact
// asset_tag the synthesizer will produce), the whole transaction rolls
// back: discovered_assets.status stays 'pending' and no partial asset row
// leaks.
func TestApproveDiscoveredAsset_Integration_DuplicateAssetRollsBack(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupApproveFixture(t, pool)

	// Pre-create an asset with the asset_tag that the synthesizer will
	// produce for fix.discoveredID. This forces the CreateAsset INSERT to
	// fail with a unique-constraint violation mid-tx.
	collidingID := uuid.New()
	collidingTag := "DSC-" + func(u uuid.UUID) string {
		return uppercaseFirst8(u.String())
	}(fix.discoveredID)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, 'collider', 'server', 'active')`,
		collidingID, fix.tenantA, collidingTag); err != nil {
		t.Fatalf("insert colliding asset: %v", err)
	}

	assetsBefore := countAssetsForTenant(t, pool, fix.tenantA)

	s := newApproveTestServer(pool)
	c, rec := newApproveCtx(t, fix.tenantA, fix.userA)
	s.ApproveDiscoveredAsset(c, IdPath(fix.discoveredID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 — body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if errObj, _ := body["error"].(map[string]any); errObj["code"] != "ASSET_ALREADY_EXISTS" {
		t.Errorf("error code = %v, want ASSET_ALREADY_EXISTS", errObj["code"])
	}

	assetsAfter := countAssetsForTenant(t, pool, fix.tenantA)
	if assetsAfter != assetsBefore {
		t.Errorf("assets count changed from %d to %d — rollback failed", assetsBefore, assetsAfter)
	}

	// discovered_asset must still be 'pending' with NULL approved_asset_id.
	var status string
	var approvedAssetID *uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT status, approved_asset_id FROM discovered_assets WHERE id = $1`, fix.discoveredID,
	).Scan(&status, &approvedAssetID); err != nil {
		t.Fatalf("fetch discovered_asset: %v", err)
	}
	if status != "pending" {
		t.Errorf("discovered_asset.status = %q, want 'pending' (rollback must restore)", status)
	}
	if approvedAssetID != nil {
		t.Errorf("approved_asset_id = %v, want NULL (rollback must restore)", *approvedAssetID)
	}
}

// uppercaseFirst8 replicates the asset_tag synth prefix logic used in the
// discovery service. Kept local so a drift in the production function is
// caught by the collision test (if the synth changes, collision no longer
// collides and this test fails loud).
func uppercaseFirst8(s string) string {
	// s is a UUID: "xxxxxxxx-xxxx-..."; take first 8 hex chars.
	if len(s) < 8 {
		return s
	}
	out := make([]byte, 8)
	for i := 0; i < 8; i++ {
		ch := s[i]
		if ch >= 'a' && ch <= 'z' {
			ch -= 'a' - 'A'
		}
		out[i] = ch
	}
	return string(out)
}
