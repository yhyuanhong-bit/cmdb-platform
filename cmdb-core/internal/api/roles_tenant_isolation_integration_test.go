//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
)

// W6.3 — sqlc cross-tenant integration coverage for the roles surface.
//
// roles.sql has three statements where a missing tenant_id WHERE clause
// would let tenantA mutate tenantB's custom roles by guessing UUIDs:
//
//   - UpdateRole : `WHERE id = $4 AND tenant_id = $5 AND is_system = false`
//   - DeleteRole : `WHERE id = $1 AND tenant_id = $2 AND is_system = false`
//   - AssignRole : enforced in identity.Service via GetUserScoped + role.tenant_id check
//
// These tests pin those guarantees at the API layer against real Postgres
// so a future sqlc edit that drops the AND tenant_id clause fails CI.
//
// We also pin the AssignRole cross-tenant defence (CROSS_TENANT_ROLE → 400)
// because it's the privilege-escalation path that motivated the
// project_tenantlint_blindspot.md memory.
//
// Run with:
//
//	go test -tags integration -run TestRolesTenantIsolation \
//	  ./internal/api/...

// roleIsoFixture seeds two tenants, one custom role per tenant, plus one
// user per tenant. Cleanup fires via t.Cleanup so parallel runs do not leak.
type roleIsoFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID
	userA   uuid.UUID
	userB   uuid.UUID
	roleA   uuid.UUID // custom (tenant-scoped) role in tenantA
	roleB   uuid.UUID // custom (tenant-scoped) role in tenantB
}

func setupRoleIsoFixture(t *testing.T, pool *pgxpool.Pool) roleIsoFixture {
	t.Helper()
	ctx := context.Background()
	fix := roleIsoFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		userA:   uuid.New(),
		userB:   uuid.New(),
		roleA:   uuid.New(),
		roleB:   uuid.New(),
	}
	sufA := fix.tenantA.String()[:8]
	sufB := fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "role-iso-A-"+sufA, "role-iso-a-"+sufA,
		fix.tenantB, "role-iso-B-"+sufB, "role-iso-b-"+sufB,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, 'role-iso-uA-'||$3, 'role iso UA', 'a-'||$3||'@t.local', 'x'),
		        ($4, $5, 'role-iso-uB-'||$6, 'role iso UB', 'b-'||$6||'@t.local', 'x')`,
		fix.userA, fix.tenantA, sufA,
		fix.userB, fix.tenantB, sufB,
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO roles (id, tenant_id, name, description, permissions, is_system)
		 VALUES ($1, $2, 'role-iso-A-'||$3, 'tenantA custom role', '{"can_read":true}'::jsonb, false),
		        ($4, $5, 'role-iso-B-'||$6, 'tenantB custom role', '{"can_read":true}'::jsonb, false)`,
		fix.roleA, fix.tenantA, sufA,
		fix.roleB, fix.tenantB, sufB,
	); err != nil {
		t.Fatalf("insert roles: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id IN ($1, $2)`, fix.userA, fix.userB)
		_, _ = pool.Exec(ctx, `DELETE FROM roles WHERE id IN ($1, $2)`, fix.roleA, fix.roleB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, fix.userA, fix.userB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

// newRoleIsoServer wires the minimum APIServer (pool + identitySvc) that
// the role handlers touch. auditSvc is nil; recordAudit no-ops.
func newRoleIsoServer(pool *pgxpool.Pool) *APIServer {
	q := dbgen.New(pool)
	return &APIServer{
		pool:        pool,
		identitySvc: identity.NewService(q),
	}
}

// roleName reads back a role's current name — the source of truth for
// "did the cross-tenant UPDATE silently land".
func roleName(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var n string
	if err := pool.QueryRow(context.Background(),
		`SELECT name FROM roles WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("read role name: %v", err)
	}
	return n
}

// roleExists returns whether a row with this id is still present in roles.
func roleExists(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM roles WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count role: %v", err)
	}
	return n == 1
}

// ---------------------------------------------------------------------------
// 1. UpdateRole — tenantA must NOT be able to PUT tenantB's custom role.
// ---------------------------------------------------------------------------

func TestRolesTenantIsolation_UpdateRole_CrossTenantReturns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRoleIsoFixture(t, pool)
	s := newRoleIsoServer(pool)

	originalName := roleName(t, pool, fix.roleB)

	body := []byte(`{"name":"PWNED-from-A","description":"hijacked"}`)
	c, rec := newDepCtx(t, http.MethodPut,
		"/roles/"+fix.roleB.String(),
		fix.tenantA, fix.userA, body)

	s.UpdateRole(c, IdPath(fix.roleB))
	c.Writer.WriteHeaderNow()

	if rec.Code == http.StatusOK {
		t.Fatalf("CRITICAL: tenantA was allowed to PUT tenantB's role (status=200, body=%s)",
			rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 — body=%s", rec.Code, rec.Body.String())
	}

	if got := roleName(t, pool, fix.roleB); got != originalName {
		t.Fatalf("CRITICAL: tenantB role name overwritten by tenantA — got %q want %q",
			got, originalName)
	}
}

// ---------------------------------------------------------------------------
// 2. UpdateRole — same-tenant flow still works (control case).
// ---------------------------------------------------------------------------

func TestRolesTenantIsolation_UpdateRole_SameTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRoleIsoFixture(t, pool)
	s := newRoleIsoServer(pool)

	body := []byte(`{"description":"updated by owner"}`)
	c, rec := newDepCtx(t, http.MethodPut,
		"/roles/"+fix.roleA.String(),
		fix.tenantA, fix.userA, body)

	s.UpdateRole(c, IdPath(fix.roleA))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusOK {
		t.Fatalf("same-tenant update failed: status=%d body=%s",
			rec.Code, rec.Body.String())
	}

	var desc string
	if err := pool.QueryRow(context.Background(),
		`SELECT description FROM roles WHERE id = $1`, fix.roleA).Scan(&desc); err != nil {
		t.Fatalf("read description: %v", err)
	}
	if desc != "updated by owner" {
		t.Errorf("description = %q, want %q", desc, "updated by owner")
	}
}

// ---------------------------------------------------------------------------
// 3. DeleteRole — tenantA must NOT be able to DELETE tenantB's role.
// ---------------------------------------------------------------------------

func TestRolesTenantIsolation_DeleteRole_CrossTenantReturns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRoleIsoFixture(t, pool)
	s := newRoleIsoServer(pool)

	c, rec := newDepCtx(t, http.MethodDelete,
		"/roles/"+fix.roleB.String(),
		fix.tenantA, fix.userA, nil)

	s.DeleteRole(c, IdPath(fix.roleB))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant DeleteRole accepted: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	if !roleExists(t, pool, fix.roleB) {
		t.Fatalf("CRITICAL: tenantB role was deleted by tenantA caller")
	}
}

// ---------------------------------------------------------------------------
// 4. AssignRoleToUser — privilege escalation across tenants must be blocked.
//
// Three cross-tenant attack shapes are covered by AssignRole's defence:
//
//   - tenantA caller targets tenantB's userID with tenantA's roleID
//     → ErrUserNotFound (404), because GetUserScoped(userB, tenantA) fails.
//
//   - tenantA caller targets tenantA's userID but with tenantB's roleID
//     → ErrCrossTenantRole (400, code=CROSS_TENANT_ROLE), because the
//       application layer compares user.tenant_id != role.tenant_id.
//
// We assert no user_roles row was inserted for either case — the
// trg_user_roles_tenant_check trigger from migration 000045 is the last
// line of defence and we want to fail closed before reaching it.
// ---------------------------------------------------------------------------

func TestRolesTenantIsolation_AssignRole_CrossTenantUser_Returns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRoleIsoFixture(t, pool)
	s := newRoleIsoServer(pool)

	// tenantA caller tries to attach tenantA's roleA to tenantB's userB.
	// This is the "force-grant a role on a foreign user" shape.
	body := []byte(`{"role_id":"` + fix.roleA.String() + `"}`)
	c, rec := newDepCtx(t, http.MethodPost,
		"/users/"+fix.userB.String()+"/roles",
		fix.tenantA, fix.userA, body)

	s.AssignRoleToUser(c, IdPath(fix.userB))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant AssignRole accepted: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	// No row should have landed in user_roles — without this the trigger
	// would still catch it, but we want application-layer defence in depth.
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM user_roles WHERE user_id = $1 AND role_id = $2`,
		fix.userB, fix.roleA).Scan(&n); err != nil {
		t.Fatalf("count user_roles: %v", err)
	}
	if n != 0 {
		t.Fatalf("CRITICAL: cross-tenant role assignment landed in user_roles (count=%d)", n)
	}
}

func TestRolesTenantIsolation_AssignRole_CrossTenantRole_Returns400(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRoleIsoFixture(t, pool)
	s := newRoleIsoServer(pool)

	// tenantA caller tries to attach tenantB's roleB to tenantA's own userA.
	// This is the "smuggle a foreign role onto my own user" shape — the
	// path that originally motivated identity.ErrCrossTenantRole.
	body := []byte(`{"role_id":"` + fix.roleB.String() + `"}`)
	c, rec := newDepCtx(t, http.MethodPost,
		"/users/"+fix.userA.String()+"/roles",
		fix.tenantA, fix.userA, body)

	s.AssignRoleToUser(c, IdPath(fix.userA))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-tenant role assignment: status=%d body=%s, want 400",
			rec.Code, rec.Body.String())
	}

	// The error envelope should carry the CROSS_TENANT_ROLE code so
	// frontend can render a useful message rather than a generic 400.
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if env.Error.Code != "CROSS_TENANT_ROLE" {
		t.Errorf("error.code = %q, want CROSS_TENANT_ROLE — body=%s",
			env.Error.Code, rec.Body.String())
	}

	// And no row in user_roles.
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM user_roles WHERE user_id = $1 AND role_id = $2`,
		fix.userA, fix.roleB).Scan(&n); err != nil {
		t.Fatalf("count user_roles: %v", err)
	}
	if n != 0 {
		t.Fatalf("CRITICAL: foreign role landed on tenantA user (count=%d)", n)
	}
}

// ---------------------------------------------------------------------------
// 5. RemoveRoleFromUser — tenantA must NOT be able to strip roles off
// tenantB's user (DoS / lockout shape).
// ---------------------------------------------------------------------------

func TestRolesTenantIsolation_RemoveRole_CrossTenantUser_Returns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRoleIsoFixture(t, pool)
	s := newRoleIsoServer(pool)

	// Plant a real assignment on tenantB so a leak/strip would be observable.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
		fix.userB, fix.roleB,
	); err != nil {
		t.Fatalf("plant user_roles row: %v", err)
	}

	// tenantA caller tries to DELETE the assignment off tenantB's user.
	c, rec := newDepCtx(t, http.MethodDelete,
		"/users/"+fix.userB.String()+"/roles/"+fix.roleB.String(),
		fix.tenantA, fix.userA, nil)

	s.RemoveRoleFromUser(c, IdPath(fix.userB), openapi_types.UUID(fix.roleB))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant RemoveRole accepted: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	// Assignment must still exist.
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM user_roles WHERE user_id = $1 AND role_id = $2`,
		fix.userB, fix.roleB).Scan(&n); err != nil {
		t.Fatalf("count user_roles: %v", err)
	}
	if n != 1 {
		t.Fatalf("CRITICAL: tenantB user_roles row stripped by tenantA — count=%d, want 1", n)
	}
}
