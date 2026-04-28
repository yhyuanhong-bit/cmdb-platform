//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cross-tenant isolation tests for inventory_scan_history and inventory_notes.
// Migration 000076 added tenant_id to both tables and the handlers in
// impl_inventory_items.go now scope every read and write to the caller's
// tenant — closing audit finding H5 (2026-04-28).
//
// Run with:
//   go test -tags integration -run TestIntegration_InventoryItemsTenantIsolation \
//     ./internal/api/...

type invItemIsoFixture struct {
	tenantA  uuid.UUID
	tenantB  uuid.UUID
	userA    uuid.UUID
	userB    uuid.UUID
	taskA    uuid.UUID
	taskB    uuid.UUID
	itemA    uuid.UUID
	itemB    uuid.UUID
}

func setupInvItemIsoFixture(t *testing.T, pool *pgxpool.Pool) invItemIsoFixture {
	t.Helper()
	ctx := context.Background()
	fix := invItemIsoFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		userA:   uuid.New(),
		userB:   uuid.New(),
		taskA:   uuid.New(),
		taskB:   uuid.New(),
		itemA:   uuid.New(),
		itemB:   uuid.New(),
	}
	sufA := fix.tenantA.String()[:8]
	sufB := fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "inv-iso-A-"+sufA, "inv-iso-A-"+sufA,
		fix.tenantB, "inv-iso-B-"+sufB, "inv-iso-B-"+sufB,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, 'iso UA', $4, 'x'),
		        ($5, $6, $7, 'iso UB', $8, 'x')`,
		fix.userA, fix.tenantA, "inv-iso-uA-"+sufA, "iso-A-"+sufA+"@test.local",
		fix.userB, fix.tenantB, "inv-iso-uB-"+sufB, "iso-B-"+sufB+"@test.local",
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO inventory_tasks (id, tenant_id, code, name, status)
		 VALUES ($1, $2, $3, $4, 'planned'), ($5, $6, $7, $8, 'planned')`,
		fix.taskA, fix.tenantA, "iso-tA-"+sufA, "task-A-"+sufA,
		fix.taskB, fix.tenantB, "iso-tB-"+sufB, "task-B-"+sufB,
	); err != nil {
		t.Fatalf("insert inventory_tasks: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO inventory_items (id, task_id, status)
		 VALUES ($1, $2, 'pending'), ($3, $4, 'pending')`,
		fix.itemA, fix.taskA,
		fix.itemB, fix.taskB,
	); err != nil {
		t.Fatalf("insert inventory_items: %v", err)
	}
	// Seed one scan_history and one note in each tenant so the read tests
	// have something to assert non-empty against.
	if _, err := pool.Exec(ctx,
		`INSERT INTO inventory_scan_history (id, tenant_id, item_id, scanned_by, method, result)
		 VALUES (gen_random_uuid(), $2, $1, $3, 'manual', 'ok'),
		        (gen_random_uuid(), $5, $4, $6, 'manual', 'ok')`,
		fix.itemA, fix.tenantA, fix.userA,
		fix.itemB, fix.tenantB, fix.userB,
	); err != nil {
		t.Fatalf("insert scan_history: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO inventory_notes (id, tenant_id, item_id, author_id, severity, text)
		 VALUES (gen_random_uuid(), $2, $1, $3, 'info', 'iso A note'),
		        (gen_random_uuid(), $5, $4, $6, 'info', 'iso B note')`,
		fix.itemA, fix.tenantA, fix.userA,
		fix.itemB, fix.tenantB, fix.userB,
	); err != nil {
		t.Fatalf("insert notes: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_notes WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_scan_history WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_items WHERE id IN ($1, $2)`, fix.itemA, fix.itemB)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_tasks WHERE id IN ($1, $2)`, fix.taskA, fix.taskB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, fix.userA, fix.userB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

// TestIntegration_InventoryItemsTenantIsolation_ScanHistoryReadBlocked
// pins the read side: tenantA cannot read tenantB's scan history via
// /inventory/tasks/.../items/{itemB}/scan-history.
func TestIntegration_InventoryItemsTenantIsolation_ScanHistoryReadBlocked(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInvItemIsoFixture(t, pool)
	s := &APIServer{pool: pool}

	c, rec := newDepCtx(t, http.MethodGet,
		"/inventory/tasks/"+fix.taskB.String()+"/items/"+fix.itemB.String()+"/scan-history",
		fix.tenantA, fix.userA, nil)
	s.GetItemScanHistory(c, IdPath(fix.taskB), fix.itemB)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			ScanHistory []struct{ ID string `json:"id"` } `json:"scan_history"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.ScanHistory); got != 0 {
		t.Fatalf("CRITICAL: tenantA leaked %d scan_history rows from tenantB — body=%s", got, rec.Body.String())
	}
}

// TestIntegration_InventoryItemsTenantIsolation_ScanHistoryWriteBlocked
// pins the write side: tenantA cannot write a scan record against
// tenantB's item; the item is invisible across tenants and the
// pre-flight SELECT returns 404.
func TestIntegration_InventoryItemsTenantIsolation_ScanHistoryWriteBlocked(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInvItemIsoFixture(t, pool)
	s := &APIServer{pool: pool}

	body := []byte(`{"method":"manual","result":"ok","note":"hostile"}`)
	c, rec := newDepCtx(t, http.MethodPost,
		"/inventory/tasks/"+fix.taskB.String()+"/items/"+fix.itemB.String()+"/scan-history",
		fix.tenantA, fix.userA, body)
	c.Request, _ = http.NewRequest(http.MethodPost, "/inventory/tasks/.../scan-history", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("tenant_id", fix.tenantA.String())
	c.Set("user_id", fix.userA.String())
	s.CreateItemScanRecord(c, IdPath(fix.taskB), fix.itemB)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("CRITICAL: tenantA write succeeded against tenantB's item — status=%d body=%s", rec.Code, rec.Body.String())
	}
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM inventory_scan_history WHERE item_id = $1 AND note = 'hostile'`,
		fix.itemB).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("CRITICAL: hostile scan record landed on tenantB's item (count=%d)", count)
	}
}

// TestIntegration_InventoryItemsTenantIsolation_NotesReadBlocked mirrors
// the scan-history read test for inventory_notes.
func TestIntegration_InventoryItemsTenantIsolation_NotesReadBlocked(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInvItemIsoFixture(t, pool)
	s := &APIServer{pool: pool}

	c, rec := newDepCtx(t, http.MethodGet,
		"/inventory/tasks/"+fix.taskB.String()+"/items/"+fix.itemB.String()+"/notes",
		fix.tenantA, fix.userA, nil)
	s.GetItemNotes(c, IdPath(fix.taskB), fix.itemB)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Notes []struct{ ID string `json:"id"` } `json:"notes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Notes); got != 0 {
		t.Fatalf("CRITICAL: tenantA leaked %d notes from tenantB — body=%s", got, rec.Body.String())
	}
}

// TestIntegration_InventoryItemsTenantIsolation_OwnTenantOK confirms the
// happy paths still work after the migration.
func TestIntegration_InventoryItemsTenantIsolation_OwnTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupInvItemIsoFixture(t, pool)
	s := &APIServer{pool: pool}

	c, rec := newDepCtx(t, http.MethodGet,
		"/inventory/tasks/"+fix.taskA.String()+"/items/"+fix.itemA.String()+"/scan-history",
		fix.tenantA, fix.userA, nil)
	s.GetItemScanHistory(c, IdPath(fix.taskA), fix.itemA)
	if rec.Code != http.StatusOK {
		t.Fatalf("own-tenant scan-history: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			ScanHistory []struct{ ID string `json:"id"` } `json:"scan_history"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Data.ScanHistory) != 1 {
		t.Fatalf("own-tenant scan-history len=%d, want 1", len(env.Data.ScanHistory))
	}
}
