//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newWOTestServer wires the bare minimum the maintenance handlers need,
// including the maintenanceSvc that CreateWorkOrderComment uses for the
// cross-tenant pre-check (audit D-H4, 2026-04-28).
func newWOTestServer(pool *pgxpool.Pool) *APIServer {
	q := dbgen.New(pool)
	return &APIServer{pool: pool, maintenanceSvc: maintenance.NewService(q, nil, pool)}
}

// Post-sqlc migration coverage for work_order_comments handlers.
//
// work_order_comments has no tenant_id column; tenancy is inherited
// from the parent work_orders row. These tests verify that:
//
//  1. ListWorkOrderComments returns comments only for the given
//     order_id (never cross-order leakage).
//  2. ListWorkOrderComments resolves author_name via LEFT JOIN and
//     returns null when the author row has been deleted.
//  3. CreateWorkOrderComment round-trips through the sqlc Insert
//     with the caller's user_id captured as author_id.
//
// Run with:
//   go test -tags integration -run TestIntegration_WorkOrderComments ./internal/api/...

type workOrderCommentFixture struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	orderA   uuid.UUID
	orderB   uuid.UUID
	// commentA1 and commentA2 belong to orderA and should be
	// returned in insertion order by the handler.
	commentA1 uuid.UUID
	commentA2 uuid.UUID
	// commentB belongs to orderB; used to assert cross-order
	// isolation (the ORDER BY + WHERE order_id filter).
	commentB uuid.UUID
}

func setupWorkOrderCommentFixture(t *testing.T, pool *pgxpool.Pool) workOrderCommentFixture {
	t.Helper()
	ctx := context.Background()
	fix := workOrderCommentFixture{
		tenantID:  uuid.New(),
		userID:    uuid.New(),
		orderA:    uuid.New(),
		orderB:    uuid.New(),
		commentA1: uuid.New(),
		commentA2: uuid.New(),
		commentB:  uuid.New(),
	}

	suffix := fix.tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		fix.tenantID, "wo-test-"+suffix, "wo-test-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, 'x')`,
		fix.userID, fix.tenantID, "wo-user-"+suffix, "WO User "+suffix, "wo-"+suffix+"@test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO work_orders (id, tenant_id, code, type, priority, status, title)
		 VALUES
		   ($1, $2, $3, 'maintenance', 'medium', 'draft', 'order A'),
		   ($4, $2, $5, 'maintenance', 'medium', 'draft', 'order B')`,
		fix.orderA, fix.tenantID, "WO-A-"+suffix,
		fix.orderB, "WO-B-"+suffix); err != nil {
		t.Fatalf("insert work_orders: %v", err)
	}
	// commentA1 is inserted a hair before commentA2 so ORDER BY
	// created_at ASC has a stable result.
	if _, err := pool.Exec(ctx,
		`INSERT INTO work_order_comments (id, order_id, author_id, text, created_at)
		 VALUES
		   ($1, $2, $3, 'first',  now() - interval '2 seconds'),
		   ($4, $5, $6, 'second', now() - interval '1 second'),
		   ($7, $8, NULL,         'orphan',   now())`,
		fix.commentA1, fix.orderA, fix.userID,
		fix.commentA2, fix.orderA, fix.userID,
		fix.commentB, fix.orderB); err != nil {
		t.Fatalf("insert comments: %v", err)
	}

	t.Cleanup(func() {
		// ON DELETE CASCADE on work_orders wipes work_order_comments,
		// but we delete everything explicitly in case the cascade
		// behavior changes in a future migration.
		_, _ = pool.Exec(ctx, `DELETE FROM work_order_comments WHERE order_id IN ($1, $2)`, fix.orderA, fix.orderB)
		_, _ = pool.Exec(ctx, `DELETE FROM work_orders WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, fix.tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, fix.tenantID)
	})
	return fix
}

func newWorkOrderCtx(t *testing.T, method, target string, fix workOrderCommentFixture, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var req *http.Request
	if body != nil {
		req, _ = http.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, target, nil)
	}
	c.Request = req
	c.Set("tenant_id", fix.tenantID.String())
	c.Set("user_id", fix.userID.String())
	return c, rec
}

// TestIntegration_ListWorkOrderComments_OrderScoped asserts the
// handler returns only the comments for the requested order and in
// created_at ASC order.
func TestIntegration_ListWorkOrderComments_OrderScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupWorkOrderCommentFixture(t, pool)

	s := newWOTestServer(pool)
	c, rec := newWorkOrderCtx(t, http.MethodGet, "/maintenance/orders/"+fix.orderA.String()+"/comments", fix, nil)
	s.ListWorkOrderComments(c, IdPath(fix.orderA))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Comments []struct {
				ID         string  `json:"id"`
				AuthorName *string `json:"author_name"`
				Text       string  `json:"text"`
			} `json:"comments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Comments); got != 2 {
		t.Fatalf("got %d comments for orderA, want 2 (orderB leaked?) — body=%s", got, rec.Body.String())
	}
	if env.Data.Comments[0].Text != "first" || env.Data.Comments[1].Text != "second" {
		t.Errorf("comments not in ASC order: got [%q, %q]", env.Data.Comments[0].Text, env.Data.Comments[1].Text)
	}
	if env.Data.Comments[0].AuthorName == nil || *env.Data.Comments[0].AuthorName == "" {
		t.Errorf("expected non-null author_name for commentA1, got nil/empty")
	}
}

// TestIntegration_ListWorkOrderComments_NullAuthor asserts that when
// author_id is NULL (author user deleted), author_name arrives as
// JSON null rather than a crash or empty struct.
func TestIntegration_ListWorkOrderComments_NullAuthor(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupWorkOrderCommentFixture(t, pool)

	s := newWOTestServer(pool)
	c, rec := newWorkOrderCtx(t, http.MethodGet, "/maintenance/orders/"+fix.orderB.String()+"/comments", fix, nil)
	s.ListWorkOrderComments(c, IdPath(fix.orderB))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Comments []struct {
				ID         string  `json:"id"`
				AuthorName *string `json:"author_name"`
				Text       string  `json:"text"`
			} `json:"comments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Comments); got != 1 {
		t.Fatalf("got %d comments for orderB, want 1 — body=%s", got, rec.Body.String())
	}
	if env.Data.Comments[0].AuthorName != nil {
		t.Errorf("expected null author_name for orphan comment, got %q", *env.Data.Comments[0].AuthorName)
	}
	if env.Data.Comments[0].Text != "orphan" {
		t.Errorf("got text=%q, want %q", env.Data.Comments[0].Text, "orphan")
	}
}

// TestIntegration_CreateWorkOrderComment_RoundTrip asserts the
// handler inserts a comment that lands in the DB with the caller as
// author.
func TestIntegration_CreateWorkOrderComment_RoundTrip(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupWorkOrderCommentFixture(t, pool)

	s := newWOTestServer(pool)
	payload, _ := json.Marshal(map[string]string{"text": "hello via sqlc"})
	c, rec := newWorkOrderCtx(t, http.MethodPost, "/maintenance/orders/"+fix.orderA.String()+"/comments", fix, payload)
	s.CreateWorkOrderComment(c, IdPath(fix.orderA))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	newID, err := uuid.Parse(env.Data.ID)
	if err != nil {
		t.Fatalf("parse returned id: %v (got %q)", err, env.Data.ID)
	}

	var gotText string
	var gotAuthor uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT text, author_id FROM work_order_comments WHERE id = $1`,
		newID).Scan(&gotText, &gotAuthor); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if gotText != "hello via sqlc" {
		t.Errorf("text = %q, want %q", gotText, "hello via sqlc")
	}
	if gotAuthor != fix.userID {
		t.Errorf("author_id = %s, want %s", gotAuthor, fix.userID)
	}
}

// TestIntegration_CreateWorkOrderComment_CrossTenant_Returns404 pins
// audit D-H4 (2026-04-28). Before v3.3.11 CreateWorkOrderComment had no
// pre-check that the parent work order belonged to the caller's tenant,
// so any authenticated user could post comments on another tenant's WOs
// by guessing a UUID. work_order_comments has no tenant_id column so
// without this check there is no other defense.
func TestIntegration_CreateWorkOrderComment_CrossTenant_Returns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	tenantA := setupWorkOrderCommentFixture(t, pool)
	tenantB := setupWorkOrderCommentFixture(t, pool)
	s := newWOTestServer(pool)

	// Caller authenticated as tenantA targets tenantB's work order.
	payload, _ := json.Marshal(map[string]string{"text": "PWNED-from-A"})
	c, rec := newWorkOrderCtx(t, http.MethodPost,
		"/maintenance/orders/"+tenantB.orderA.String()+"/comments", tenantA, payload)
	s.CreateWorkOrderComment(c, IdPath(tenantB.orderA))

	if rec.Code == http.StatusCreated {
		t.Fatalf("CRITICAL: tenantA was allowed to post comment on tenantB WO (status=201, body=%s)",
			rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — body=%s", rec.Code, rec.Body.String())
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM work_order_comments WHERE order_id = $1 AND text = 'PWNED-from-A'`,
		tenantB.orderA).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("CRITICAL: hostile comment landed on tenantB's WO (count=%d)", count)
	}
}
