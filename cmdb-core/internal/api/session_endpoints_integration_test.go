//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Post-sqlc migration coverage for session handlers. user_sessions has
// no tenant_id column — isolation is by user_id. The tests verify the
// handler authorization gate (canListUserSessions) still refuses
// cross-user reads after the query moved to dbgen.
//
// Run with:
//   go test -tags integration -run TestIntegration_Sessions ./internal/api/...

type sessionsFixture struct {
	tenantID uuid.UUID
	userA    uuid.UUID
	userB    uuid.UUID
	sessionA uuid.UUID
	sessionB uuid.UUID
}

// setupSessionsFixture creates one tenant with two users, and gives
// each user a single session row so cross-user read attempts have
// something real to be blocked from.
func setupSessionsFixture(t *testing.T, pool *pgxpool.Pool) sessionsFixture {
	t.Helper()
	ctx := context.Background()
	fix := sessionsFixture{
		tenantID: uuid.New(),
		userA:    uuid.New(),
		userB:    uuid.New(),
		sessionA: uuid.New(),
		sessionB: uuid.New(),
	}

	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "session-test-"+suffix, "session-test-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES
		   ($1, $2, $3, $4, $5, 'x'),
		   ($6, $2, $7, $8, $9, 'x')`,
		fix.userA, fix.tenantID,
		"session-a-"+suffix, "User A "+suffix, "a-"+suffix+"@test.local",
		fix.userB,
		"session-b-"+suffix, "User B "+suffix, "b-"+suffix+"@test.local",
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO user_sessions (id, user_id, ip_address, device_type, browser, is_current)
		 VALUES
		   ($1, $2, '10.0.0.1', 'desktop', 'Chrome',  true),
		   ($3, $4, '10.0.0.2', 'mobile',  'Firefox', true)`,
		fix.sessionA, fix.userA,
		fix.sessionB, fix.userB,
	); err != nil {
		t.Fatalf("insert sessions: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM user_sessions WHERE user_id IN ($1, $2)`, fix.userA, fix.userB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

func newSessionHandlerCtx(t *testing.T, method, target string, tenantID, userID uuid.UUID, admin bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	c.Set("tenant_id", tenantID.String())
	c.Set("user_id", userID.String())
	if admin {
		c.Set("is_admin", true)
	}
	return c, rec
}

// TestIntegration_ListUserSessions_OwnerSeesOwn asserts a non-admin
// user can read their own session list via the sqlc-backed handler.
func TestIntegration_ListUserSessions_OwnerSeesOwn(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSessionsFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newSessionHandlerCtx(t, http.MethodGet, "/users/"+fix.userA.String()+"/sessions", fix.tenantID, fix.userA, false)
	s.ListUserSessions(c, IdPath(fix.userA))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}

	var env struct {
		Data struct {
			Sessions []struct {
				ID      string `json:"id"`
				Device  string `json:"device"`
				Browser string `json:"browser"`
			} `json:"sessions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Sessions); got != 1 {
		t.Fatalf("got %d sessions, want 1 — body=%s", got, rec.Body.String())
	}
	if env.Data.Sessions[0].ID != fix.sessionA.String() {
		t.Errorf("returned session id = %s, want userA session %s", env.Data.Sessions[0].ID, fix.sessionA)
	}
}

// TestIntegration_ListUserSessions_ForbidsCrossUser asserts a non-admin
// cannot read another user's sessions even within the same tenant —
// the handler must return 403 before it ever queries the DB.
func TestIntegration_ListUserSessions_ForbidsCrossUser(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSessionsFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newSessionHandlerCtx(t, http.MethodGet, "/users/"+fix.userB.String()+"/sessions", fix.tenantID, fix.userA, false)
	s.ListUserSessions(c, IdPath(fix.userB))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 — body=%s", rec.Code, rec.Body.String())
	}
}

// TestIntegration_ListUserSessions_AdminCanRead asserts admins keep
// their privileged cross-user read — needed by admin session
// management UI.
func TestIntegration_ListUserSessions_AdminCanRead(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSessionsFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newSessionHandlerCtx(t, http.MethodGet, "/users/"+fix.userB.String()+"/sessions", fix.tenantID, fix.userA, true)
	s.ListUserSessions(c, IdPath(fix.userB))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Sessions []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Sessions); got != 1 {
		t.Fatalf("got %d sessions, want 1 (userB's only) — body=%s", got, rec.Body.String())
	}
	if env.Data.Sessions[0].ID != fix.sessionB.String() {
		t.Errorf("returned session id = %s, want userB session %s", env.Data.Sessions[0].ID, fix.sessionB)
	}
}
