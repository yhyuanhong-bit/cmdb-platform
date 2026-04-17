package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Tests here cover the pre-DB validation paths in sync_endpoints.go.
// These are the security-critical guards: the allowlist protecting
// dynamic SQL from injection, and input sanitization for conflict
// resolution. DB-path tests are out of scope (require integration env).
//
// Note: oapi-codegen's ServerInterfaceWrapper validates UUID params and enum
// values before the handler runs, so "invalid UUID" and "invalid enum" cases
// are no longer reachable at the handler level. We still test the allowlist
// guard — it's defense-in-depth against future enum drift.

// ---------------------------------------------------------------------------
// SyncGetChanges — entity_type allowlist is defense-in-depth beyond the
// generated enum validation.
// ---------------------------------------------------------------------------

func TestSyncGetChanges_EmptyEntityType(t *testing.T) {
	s := &APIServer{}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("tenant_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodGet, "/sync/changes", nil)
	c.Request = req
	s.SyncGetChanges(c, SyncGetChangesParams{EntityType: ""})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSyncGetChanges_InvalidEntityType(t *testing.T) {
	// These bypass the wrapper (we call the handler directly), so the
	// allowlist inside the handler must still reject them.
	s := &APIServer{}
	injections := []SyncGetChangesParamsEntityType{
		"users; DROP TABLE users;--",
		"assets'; SELECT * FROM pg_user;--",
		"pg_catalog.pg_tables",
		"unknown_table",
		"ASSETS", // case-sensitive allowlist
		"assets UNION SELECT",
		"../secrets",
	}
	for _, injected := range injections {
		t.Run(string(injected), func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set("tenant_id", uuid.New().String())
			req, _ := http.NewRequest(http.MethodGet, "/sync/changes", nil)
			c.Request = req
			s.SyncGetChanges(c, SyncGetChangesParams{EntityType: injected})

			if rec.Code != http.StatusBadRequest {
				t.Errorf("entity_type=%q accepted (status %d) — allowlist bypassed", injected, rec.Code)
			}
		})
	}
}

func TestSyncGetChanges_AllowlistEntriesAreRecognized(t *testing.T) {
	// Positive test: the allowlist entries reach the DB layer. We can't run
	// the real query without a DB, so we assert the handler does NOT return
	// a 400. With a nil pool, it will panic or return 500 — either way, it
	// got past the allowlist guard, which is what we're testing.
	allowed := []SyncGetChangesParamsEntityType{
		"assets", "locations", "racks", "work_orders",
		"alert_events", "inventory_tasks", "alert_rules",
		"inventory_items", "audit_events",
	}
	for _, entity := range allowed {
		t.Run(string(entity), func(t *testing.T) {
			defer func() {
				_ = recover() // expected: nil pool dereference
			}()
			s := &APIServer{}
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set("tenant_id", uuid.New().String())
			req, _ := http.NewRequest(http.MethodGet, "/sync/changes", nil)
			c.Request = req
			s.SyncGetChanges(c, SyncGetChangesParams{EntityType: entity})

			if rec.Code == http.StatusBadRequest {
				t.Errorf("entity_type=%q rejected by allowlist — should be permitted", entity)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SyncResolveConflict — validation paths before any DB call.
// Invalid UUIDs are caught by the generated wrapper; we only cover the
// resolution-value validation inside the handler.
// ---------------------------------------------------------------------------

func TestSyncResolveConflict_MissingResolution(t *testing.T) {
	s := &APIServer{}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("user_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	s.SyncResolveConflict(c, IdPath(uuid.New()))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSyncResolveConflict_MalformedJSON(t *testing.T) {
	s := &APIServer{}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("user_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{not-json`)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	s.SyncResolveConflict(c, IdPath(uuid.New()))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSyncResolveConflict_InvalidResolutionValue(t *testing.T) {
	s := &APIServer{}
	tests := []string{"", "LOCAL_WINS", "always_wins", "local", "remote", "both", "../etc/passwd"}
	for _, val := range tests {
		t.Run("resolution="+val, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set("user_id", uuid.New().String())
			body, _ := json.Marshal(map[string]string{"resolution": val})
			req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			s.SyncResolveConflict(c, IdPath(uuid.New()))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("resolution=%q accepted (status %d) — should be rejected", val, rec.Code)
			}
		})
	}
}

func TestSyncResolveConflict_ValidResolutionValues(t *testing.T) {
	// Positive validation test: confirm "local_wins" and "remote_wins" pass
	// the validation gate. They'll fail downstream at the DB query (nil pool),
	// which surfaces as a 404 ("conflict not found") because the QueryRow.Scan
	// error is treated as not-found. That's good enough to prove we got past
	// validation.
	s := &APIServer{}
	for _, val := range []string{"local_wins", "remote_wins"} {
		t.Run(val, func(t *testing.T) {
			defer func() { _ = recover() }()
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set("user_id", uuid.New().String())
			body, _ := json.Marshal(map[string]string{"resolution": val})
			req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			s.SyncResolveConflict(c, IdPath(uuid.New()))

			if rec.Code == http.StatusBadRequest {
				t.Errorf("resolution=%q rejected by validation — should be permitted", val)
			}
		})
	}
}
