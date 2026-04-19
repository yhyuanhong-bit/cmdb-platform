//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integration tests for SyncResolveConflict. These verify the tenant_id
// scoping that prevents cross-tenant IDOR on sync_conflicts.
//
// Run with:
//   go test -tags integration -run TestSyncResolveConflict ./internal/api/...

type syncConflictFixture struct {
	tenantA    uuid.UUID
	tenantB    uuid.UUID
	userA      uuid.UUID
	conflictID uuid.UUID
	assetID    uuid.UUID
}

// setupSyncConflictFixture creates two tenants, one user in tenant A, one
// asset owned by tenant A, and one pending sync_conflicts row owned by
// tenant A. Cleanup is registered on t.Cleanup.
func setupSyncConflictFixture(t *testing.T, pool *pgxpool.Pool) syncConflictFixture {
	t.Helper()
	ctx := context.Background()
	fix := syncConflictFixture{
		tenantA:    uuid.New(),
		tenantB:    uuid.New(),
		userA:      uuid.New(),
		conflictID: uuid.New(),
		assetID:    uuid.New(),
	}
	suffix := fix.tenantA.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "sync-A-"+suffix, "sync-a-"+suffix,
		fix.tenantB, "sync-B-"+suffix, "sync-b-"+suffix); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userA, fix.tenantA,
		"sync-user-"+suffix, "User "+suffix, "user-"+suffix+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, $4, 'server', 'active')`,
		fix.assetID, fix.tenantA, "TAG-"+suffix, "orig-name"); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO sync_conflicts (id, tenant_id, entity_type, entity_id,
		    local_version, remote_version, local_diff, remote_diff, resolution)
		 VALUES ($1, $2, 'assets', $3, 1, 2, '{}'::jsonb, $4::jsonb, 'pending')`,
		fix.conflictID, fix.tenantA, fix.assetID,
		`{"name":"remote-name"}`); err != nil {
		t.Fatalf("insert sync_conflict: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sync_conflicts WHERE id = $1`, fix.conflictID)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, fix.assetID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, fix.userA)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

func newResolveRequest(t *testing.T, resolution string) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"resolution": resolution})
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestSyncResolveConflict_BlocksCrossTenant verifies IDOR protection: a
// user authenticated as tenant B cannot resolve a conflict owned by tenant
// A. The handler must return 404 and must not mark the conflict resolved.
func TestSyncResolveConflict_BlocksCrossTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSyncConflictFixture(t, pool)

	s := &APIServer{pool: pool}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	// Attacker: authenticated as a different tenant.
	c.Set("tenant_id", fix.tenantB.String())
	c.Set("user_id", uuid.New().String())
	c.Request = newResolveRequest(t, "remote_wins")

	s.SyncResolveConflict(c, IdPath(fix.conflictID))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant resolve returned %d, want 404; body=%s",
			rec.Code, rec.Body.String())
	}

	// And the conflict must remain 'pending'.
	var resolution string
	if err := pool.QueryRow(context.Background(),
		`SELECT resolution FROM sync_conflicts WHERE id = $1`, fix.conflictID,
	).Scan(&resolution); err != nil {
		t.Fatalf("reread conflict: %v", err)
	}
	if resolution != "pending" {
		t.Errorf("conflict resolution = %q after cross-tenant attempt, want 'pending'", resolution)
	}

	// And the asset must not have been mutated.
	var name string
	if err := pool.QueryRow(context.Background(),
		`SELECT name FROM assets WHERE id = $1`, fix.assetID,
	).Scan(&name); err != nil {
		t.Fatalf("reread asset: %v", err)
	}
	if name != "orig-name" {
		t.Errorf("asset name = %q after cross-tenant attempt, want 'orig-name'", name)
	}
}

// TestSyncResolveConflict_RejectsBadColumn_EndToEnd confirms the full
// handler path returns 400/INVALID_FIELD when a pending conflict carries
// a remote_diff whose keys are not in the per-entity whitelist.
func TestSyncResolveConflict_RejectsBadColumn_EndToEnd(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID := uuid.New()
	userID := uuid.New()
	conflictID := uuid.New()
	assetID := uuid.New()
	suffix := tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		tenantID, "sync-bc-"+suffix, "sync-bc-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		userID, tenantID,
		"bc-user-"+suffix, "User "+suffix, "bc-"+suffix+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, 'a', 'server', 'active')`,
		assetID, tenantID, "BC-TAG-"+suffix); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	// Craft a malicious remote_diff whose JSON key is an injection attempt.
	maliciousDiff := `{"name=x,password_hash='injected'--": "x"}`
	if _, err := pool.Exec(ctx,
		`INSERT INTO sync_conflicts (id, tenant_id, entity_type, entity_id,
		    local_version, remote_version, local_diff, remote_diff, resolution)
		 VALUES ($1, $2, 'assets', $3, 1, 2, '{}'::jsonb, $4::jsonb, 'pending')`,
		conflictID, tenantID, assetID, maliciousDiff); err != nil {
		t.Fatalf("insert sync_conflict: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sync_conflicts WHERE id = $1`, conflictID)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, assetID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	s := &APIServer{pool: pool}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("tenant_id", tenantID.String())
	c.Set("user_id", userID.String())
	c.Request = newResolveRequest(t, "remote_wins")

	s.SyncResolveConflict(c, IdPath(conflictID))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malicious column returned %d, want 400; body=%s",
			rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("INVALID_FIELD")) {
		t.Errorf("response body %q missing INVALID_FIELD code", rec.Body.String())
	}
}
