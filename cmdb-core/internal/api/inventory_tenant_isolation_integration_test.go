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

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
)

// These tests are regressions for four cross-tenant IDOR vulnerabilities
// on inventory_tasks state transitions, enumerated in the Phase 1.6
// remediation roadmap:
//
//   - POST /inventory/tasks/:id/complete          (impl_inventory.go CompleteInventoryTask)
//   - POST /inventory/tasks/:id/items/:itemId/scan     (impl_inventory.go:199 auto-activate)
//   - POST /inventory/tasks/:id/import                 (impl_inventory.go:311 auto-activate, inside tx)
//   - POST /inventory/tasks/:id/items/:itemId/resolve  (inventory_resolve_endpoint.go:74 auto-activate)
//
// The common vulnerability shape was `UPDATE inventory_tasks SET status=...
// WHERE id = $1 [AND status = 'planned']` — missing `AND tenant_id = $2`,
// so a tenant-B caller holding a tenant-A task UUID could flip status.
//
// We return 404 (not 403) on cross-tenant access to avoid leaking
// "exists in another tenant" as an information oracle.
//
// Run with:
//   go test -tags integration -run TestInventoryTenantIsolation \
//     ./internal/api/...

// invFixture holds two tenants with a planned inventory task owned by A.
type invFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID
	taskA   uuid.UUID
	itemA   uuid.UUID // an item on taskA, used by resolve/scan tests
	userA   uuid.UUID // real user in tenant A (FK target for scanned_by / inventory_notes.author_id)
	userB   uuid.UUID // real user in tenant B (FK target when acting as B)
}

// setupInventoryFixture creates tenants A+B, one planned task owned by A,
// and one pending item on that task. Cleanup fires via t.Cleanup.
func setupInventoryFixture(t *testing.T, pool *pgxpool.Pool) invFixture {
	t.Helper()
	ctx := context.Background()
	fix := invFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		taskA:   uuid.New(),
		itemA:   uuid.New(),
		userA:   uuid.New(),
		userB:   uuid.New(),
	}
	suffix := fix.tenantA.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "inv-iso-A-"+suffix, "inv-iso-a-"+suffix,
		fix.tenantB, "inv-iso-B-"+suffix, "inv-iso-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x'), ($6, $7, $8, $9, $10, 'x')`,
		fix.userA, fix.tenantA,
		"inv-iso-user-a-"+suffix, "User A "+suffix, "inv-iso-a-"+suffix+"@test.local",
		fix.userB, fix.tenantB,
		"inv-iso-user-b-"+suffix, "User B "+suffix, "inv-iso-b-"+suffix+"@test.local",
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO inventory_tasks (id, tenant_id, code, name, status)
		 VALUES ($1, $2, $3, $4, 'planned')`,
		fix.taskA, fix.tenantA, "INV-ISO-"+suffix, "iso-task",
	); err != nil {
		t.Fatalf("insert inventory_task: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO inventory_items (id, task_id, expected, status)
		 VALUES ($1, $2, '{}'::jsonb, 'pending')`,
		fix.itemA, fix.taskA,
	); err != nil {
		t.Fatalf("insert inventory_item: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_scan_history WHERE item_id = $1`, fix.itemA)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_notes WHERE item_id = $1`, fix.itemA)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_items WHERE task_id = $1`, fix.taskA)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_tasks WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

// newInventoryServer wires the minimum APIServer (pool + inventorySvc) that
// the inventory handlers touch. auditSvc is nil (recordAudit no-ops), bus
// is nil (scan event is skipped), which is fine for these tenancy checks.
func newInventoryServer(pool *pgxpool.Pool) *APIServer {
	q := dbgen.New(pool)
	return &APIServer{
		pool:         pool,
		inventorySvc: inventory.NewService(q, nil),
	}
}

// taskStatus reads the current status of a task, used to assert no mutation.
func taskStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM inventory_tasks WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read task status: %v", err)
	}
	return s
}

// ---------------------------------------------------------------------------
// CompleteInventoryTask — impl_inventory.go
// ---------------------------------------------------------------------------

func TestInventoryTenantIsolation_CompleteInventoryTask(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInventoryFixture(t, pool)
	s := newInventoryServer(pool)

	tests := []struct {
		name        string
		callerTID   uuid.UUID
		wantStatus  int
		wantTaskEnd string
	}{
		{
			name:        "same tenant flip succeeds",
			callerTID:   fix.tenantA,
			wantStatus:  http.StatusOK,
			wantTaskEnd: "completed",
		},
		{
			name:        "cross tenant returns 404 and leaves task planned",
			callerTID:   fix.tenantB,
			wantStatus:  http.StatusNotFound,
			wantTaskEnd: "planned",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset task status before each sub-test so one subtest doesn't
			// observe a status the previous subtest set.
			if _, err := pool.Exec(context.Background(),
				`UPDATE inventory_tasks SET status = 'planned', completed_date = NULL WHERE id = $1`,
				fix.taskA); err != nil {
				t.Fatalf("reset task: %v", err)
			}

			c, rec := newCtxAsTenant(t, http.MethodPost,
				"/inventory/tasks/"+fix.taskA.String()+"/complete", tc.callerTID)
			s.CompleteInventoryTask(c, IdPath(fix.taskA))
			c.Writer.WriteHeaderNow()

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d — body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if got := taskStatus(t, pool, fix.taskA); got != tc.wantTaskEnd {
				t.Errorf("task status = %q, want %q", got, tc.wantTaskEnd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ScanInventoryItem auto-activate — impl_inventory.go:199
// ---------------------------------------------------------------------------

func TestInventoryTenantIsolation_ScanInventoryItem_AutoActivate(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInventoryFixture(t, pool)
	s := newInventoryServer(pool)

	tests := []struct {
		name        string
		callerTID   uuid.UUID
		callerUID   uuid.UUID
		wantTaskEnd string
	}{
		{
			name:        "same tenant auto-activates planned task",
			callerTID:   fix.tenantA,
			callerUID:   fix.userA,
			wantTaskEnd: "in_progress",
		},
		{
			name:        "cross tenant leaves task planned",
			callerTID:   fix.tenantB,
			callerUID:   fix.userB,
			wantTaskEnd: "planned",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(context.Background(),
				`UPDATE inventory_tasks SET status = 'planned' WHERE id = $1`, fix.taskA); err != nil {
				t.Fatalf("reset task: %v", err)
			}

			// Build scan body. Status 'scanned' is a valid inventory_items status.
			body := []byte(`{"status":"scanned","actual":{"asset_tag":"x"}}`)
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req, _ := http.NewRequest(http.MethodPost,
				"/inventory/tasks/"+fix.taskA.String()+"/items/"+fix.itemA.String()+"/scan",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			c.Set("tenant_id", tc.callerTID.String())
			c.Set("user_id", tc.callerUID.String())

			s.ScanInventoryItem(c, IdPath(fix.taskA), fix.itemA)
			c.Writer.WriteHeaderNow()

			// Regardless of tenant, the item UPDATE currently succeeds (ScanItem
			// IDOR is out of scope for this phase). What we assert is that the
			// TASK status flip does NOT happen for a cross-tenant caller.
			if got := taskStatus(t, pool, fix.taskA); got != tc.wantTaskEnd {
				t.Errorf("task status = %q, want %q (cross-tenant caller should not auto-activate)",
					got, tc.wantTaskEnd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ImportInventoryItems auto-activate (inside tx) — impl_inventory.go:311
// ---------------------------------------------------------------------------

func TestInventoryTenantIsolation_ImportInventoryItems_AutoActivate(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInventoryFixture(t, pool)
	s := newInventoryServer(pool)

	tests := []struct {
		name        string
		callerTID   uuid.UUID
		callerUID   uuid.UUID
		wantTaskEnd string
	}{
		{
			name:        "same tenant import auto-activates",
			callerTID:   fix.tenantA,
			callerUID:   fix.userA,
			wantTaskEnd: "in_progress",
		},
		{
			name:        "cross tenant import leaves task planned",
			callerTID:   fix.tenantB,
			callerUID:   fix.userB,
			wantTaskEnd: "planned",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(context.Background(),
				`UPDATE inventory_tasks SET status = 'planned' WHERE id = $1`, fix.taskA); err != nil {
				t.Fatalf("reset task: %v", err)
			}

			// Empty import body — the item loop is skipped (avoiding the
			// assetSvc path, which isn't wired in this minimal test server),
			// but the final auto-activate UPDATE at the end of the handler
			// still fires, which is what we're testing.
			body := []byte(`{"items":[]}`)
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req, _ := http.NewRequest(http.MethodPost,
				"/inventory/tasks/"+fix.taskA.String()+"/import",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			c.Set("tenant_id", tc.callerTID.String())
			c.Set("user_id", tc.callerUID.String())

			s.ImportInventoryItems(c, IdPath(fix.taskA))
			c.Writer.WriteHeaderNow()

			if got := taskStatus(t, pool, fix.taskA); got != tc.wantTaskEnd {
				t.Errorf("task status = %q, want %q", got, tc.wantTaskEnd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveInventoryDiscrepancy auto-activate — inventory_resolve_endpoint.go:74
// ---------------------------------------------------------------------------

func TestInventoryTenantIsolation_ResolveInventoryDiscrepancy_AutoActivate(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInventoryFixture(t, pool)
	s := newInventoryServer(pool)

	tests := []struct {
		name        string
		callerTID   uuid.UUID
		callerUID   uuid.UUID
		wantTaskEnd string
	}{
		{
			name:        "same tenant resolve auto-activates",
			callerTID:   fix.tenantA,
			callerUID:   fix.userA,
			wantTaskEnd: "in_progress",
		},
		{
			name:        "cross tenant resolve leaves task planned",
			callerTID:   fix.tenantB,
			callerUID:   fix.userB,
			wantTaskEnd: "planned",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(context.Background(),
				`UPDATE inventory_tasks SET status = 'planned' WHERE id = $1`, fix.taskA); err != nil {
				t.Fatalf("reset task: %v", err)
			}

			body := []byte(`{"action":"verify","note":"test"}`)
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req, _ := http.NewRequest(http.MethodPost,
				"/inventory/tasks/"+fix.taskA.String()+"/items/"+fix.itemA.String()+"/resolve",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			c.Set("tenant_id", tc.callerTID.String())
			c.Set("user_id", tc.callerUID.String())

			s.ResolveInventoryDiscrepancy(c, IdPath(fix.taskA), fix.itemA)
			c.Writer.WriteHeaderNow()

			if got := taskStatus(t, pool, fix.taskA); got != tc.wantTaskEnd {
				t.Errorf("task status = %q, want %q", got, tc.wantTaskEnd)
			}
		})
	}
}
