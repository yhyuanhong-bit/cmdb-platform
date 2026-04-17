//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the notification handlers against a real Postgres,
// because the handlers go directly to s.pool and their behavior (tenant
// scoping, read semantics, ordering) is only meaningful against SQL.
//
// Run with:
//   go test -tags integration -run TestNotifications ./internal/api/...
//
// TEST_DATABASE_URL can override the default docker-compose connection.

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

// newTestPool connects to the integration DB, skipping the test if
// unreachable so `go test ./...` without a DB never fails.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

type notifFixture struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	otherID  uuid.UUID
	notifIDs []uuid.UUID
}

// setupNotifFixture creates an isolated tenant + user + another-user +
// three notifications owned by the primary user. Cleanup is registered
// via t.Cleanup so failures never leak rows.
func setupNotifFixture(t *testing.T, pool *pgxpool.Pool) notifFixture {
	t.Helper()
	ctx := context.Background()
	fix := notifFixture{
		tenantID: uuid.New(),
		userID:   uuid.New(),
		otherID:  uuid.New(),
	}

	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "notif-test-"+suffix, "notif-test-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x'), ($6, $2, $7, $8, $9, 'x')`,
		fix.userID, fix.tenantID,
		"notif-primary-"+suffix, "Primary "+suffix, "primary-"+suffix+"@test.local",
		fix.otherID,
		"notif-other-"+suffix, "Other "+suffix, "other-"+suffix+"@test.local",
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}

	for i := 0; i < 3; i++ {
		id := uuid.New()
		fix.notifIDs = append(fix.notifIDs, id)
		if _, err := pool.Exec(ctx,
			`INSERT INTO notifications (id, tenant_id, user_id, type, title, is_read)
			 VALUES ($1, $2, $3, 'test', $4, false)`,
			id, fix.tenantID, fix.userID, "test notification"); err != nil {
			t.Fatalf("insert notification: %v", err)
		}
	}
	// One notification for the other user — used to verify user-scoping.
	if _, err := pool.Exec(ctx,
		`INSERT INTO notifications (id, tenant_id, user_id, type, title, is_read)
		 VALUES (gen_random_uuid(), $1, $2, 'test', 'not yours', false)`,
		fix.tenantID, fix.otherID); err != nil {
		t.Fatalf("insert other notification: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

// newHandlerCtx wires a gin.Context with the tenant/user IDs the handler
// expects from auth middleware.
func newHandlerCtx(t *testing.T, method, target string, fix notifFixture) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	c.Set("tenant_id", fix.tenantID.String())
	c.Set("user_id", fix.userID.String())
	return c, rec
}

func unwrapData(t *testing.T, body []byte) any {
	t.Helper()
	var env struct {
		Data any `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal body: %v — body=%s", err, string(body))
	}
	return env.Data
}

// ---------------------------------------------------------------------------
// ListNotifications
// ---------------------------------------------------------------------------

func TestIntegration_ListNotifications_ReturnsOnlyCurrentUsersUnread(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodGet, "/notifications", fix)
	s.ListNotifications(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	data := unwrapData(t, rec.Body.Bytes())
	items, ok := data.([]any)
	if !ok {
		t.Fatalf("data is not a list: %T", data)
	}
	if len(items) != 3 {
		t.Errorf("got %d notifications, want 3 (primary user only)", len(items))
	}
}

func TestIntegration_ListNotifications_ExcludesRead(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	// Mark one as read directly.
	if _, err := pool.Exec(context.Background(),
		`UPDATE notifications SET is_read = true WHERE id = $1`, fix.notifIDs[0]); err != nil {
		t.Fatalf("prep: %v", err)
	}

	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodGet, "/notifications", fix)
	s.ListNotifications(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	items := unwrapData(t, rec.Body.Bytes()).([]any)
	if len(items) != 2 {
		t.Errorf("got %d unread, want 2", len(items))
	}
}

// ---------------------------------------------------------------------------
// CountUnreadNotifications
// ---------------------------------------------------------------------------

func TestIntegration_CountUnread_MatchesFixture(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodGet, "/notifications/count", fix)
	s.CountUnreadNotifications(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	payload := unwrapData(t, rec.Body.Bytes()).(map[string]any)
	got, ok := payload["count"].(float64)
	if !ok {
		t.Fatalf("count is not a number: %T", payload["count"])
	}
	if got != 3 {
		t.Errorf("count = %v, want 3", got)
	}
}

func TestIntegration_CountUnread_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	// Swap tenant in context to simulate a different tenant's user asking.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodGet, "/notifications/count", nil)
	c.Request = req
	c.Set("tenant_id", uuid.New().String()) // wrong tenant
	c.Set("user_id", fix.userID.String())

	s := &APIServer{pool: pool}
	s.CountUnreadNotifications(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	payload := unwrapData(t, rec.Body.Bytes()).(map[string]any)
	if got := payload["count"].(float64); got != 0 {
		t.Errorf("cross-tenant count leaked %v notifications — tenant scope broken", got)
	}
}

// ---------------------------------------------------------------------------
// MarkNotificationRead
// ---------------------------------------------------------------------------

func TestIntegration_MarkNotificationRead_UpdatesRow(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	target := fix.notifIDs[0]
	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodPost, "/notifications/"+target.String()+"/read", fix)
	c.Params = gin.Params{{Key: "id", Value: target.String()}}
	s.MarkNotificationRead(c)
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 — body=%s", rec.Code, rec.Body.String())
	}
	var isRead bool
	if err := pool.QueryRow(context.Background(),
		`SELECT is_read FROM notifications WHERE id = $1`, target).Scan(&isRead); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !isRead {
		t.Error("notification not marked read in DB")
	}
}

func TestIntegration_MarkNotificationRead_InvalidID(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodPost, "/notifications/not-a-uuid/read", fix)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	s.MarkNotificationRead(c)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestIntegration_MarkNotificationRead_OtherUsersNotificationIsNoop(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	// Find the notification owned by the OTHER user.
	var otherID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM notifications WHERE user_id = $1`, fix.otherID).Scan(&otherID); err != nil {
		t.Fatalf("prep: %v", err)
	}

	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodPost, "/notifications/"+otherID.String()+"/read", fix)
	c.Params = gin.Params{{Key: "id", Value: otherID.String()}}
	s.MarkNotificationRead(c)
	c.Writer.WriteHeaderNow()

	// Handler returns 204 either way (UPDATE … WHERE matches 0 rows).
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	var isRead bool
	if err := pool.QueryRow(context.Background(),
		`SELECT is_read FROM notifications WHERE id = $1`, otherID).Scan(&isRead); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if isRead {
		t.Error("cross-user mark-read mutated the other user's notification")
	}
}

// ---------------------------------------------------------------------------
// MarkAllNotificationsRead
// ---------------------------------------------------------------------------

func TestIntegration_MarkAllNotificationsRead_UpdatesOnlyCurrentUsers(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupNotifFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newHandlerCtx(t, http.MethodPost, "/notifications/read-all", fix)
	s.MarkAllNotificationsRead(c)
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 — body=%s", rec.Code, rec.Body.String())
	}

	var primaryUnread, otherUnread int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		fix.userID).Scan(&primaryUnread); err != nil {
		t.Fatalf("count primary: %v", err)
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		fix.otherID).Scan(&otherUnread); err != nil {
		t.Fatalf("count other: %v", err)
	}

	if primaryUnread != 0 {
		t.Errorf("primary still has %d unread, want 0", primaryUnread)
	}
	if otherUnread != 1 {
		t.Errorf("other user unread count = %d, want 1 (mark-all leaked across users)", otherUnread)
	}
}
