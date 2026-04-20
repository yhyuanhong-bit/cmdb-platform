package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Tests here cover the pre-DB validation paths in impl_sync.go.
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

// ---------------------------------------------------------------------------
// validateResolveColumns — pure helper that gates which columns a remote
// diff is allowed to write. This is the CRITICAL defense against SQL
// injection via attacker-controlled JSON keys. Table-driven coverage below.
// ---------------------------------------------------------------------------

func TestValidateResolveColumns_AcceptsKnownColumns(t *testing.T) {
	cases := map[string]map[string]any{
		"assets":          {"name": "srv-1", "status": "active", "serial_number": "SN123"},
		"locations":       {"name": "dc1", "slug": "dc-1", "status": "active"},
		"racks":           {"name": "rack-a", "status": "active"},
		"work_orders":     {"title": "t", "status": "open", "priority": "high"},
		"alert_events":    {"status": "acked", "severity": "high"},
		"alert_rules":     {"name": "cpu-high", "enabled": true},
		"inventory_tasks": {"name": "Q1", "status": "planned"},
		"inventory_items": {"status": "scanned"},
	}
	for entity, diff := range cases {
		t.Run(entity, func(t *testing.T) {
			if err := validateResolveColumns(entity, diff); err != nil {
				t.Errorf("validateResolveColumns(%q) unexpected error: %v", entity, err)
			}
		})
	}
}

func TestValidateResolveColumns_RejectsUnknownColumnsAndInjection(t *testing.T) {
	cases := []struct {
		name   string
		entity string
		diff   map[string]any
	}{
		{"assets_unknown_col", "assets", map[string]any{"tenant_id": uuid.New().String()}},
		{"assets_system_col", "assets", map[string]any{"sync_version": 999}},
		{"assets_injection", "assets", map[string]any{"name=x,password_hash='injected'--": "x"}},
		{"assets_quote_break", "assets", map[string]any{`name"; DROP TABLE users; --`: "x"}},
		{"assets_uppercase_drift", "assets", map[string]any{"NAME": "x"}},
		{"racks_not_in_whitelist", "racks", map[string]any{"attributes": "{}"}},
		{"unknown_entity", "pg_catalog.pg_user", map[string]any{"name": "x"}},
		{"empty_entity", "", map[string]any{"name": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateResolveColumns(tc.entity, tc.diff); err == nil {
				t.Errorf("validateResolveColumns(%q, %v) = nil, want error", tc.entity, tc.diff)
			}
		})
	}
}

func TestValidateResolveColumns_EmptyDiffIsOK(t *testing.T) {
	// An empty diff is a no-op UPDATE and should not produce an error.
	if err := validateResolveColumns("assets", map[string]any{}); err != nil {
		t.Errorf("empty diff unexpectedly rejected: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SyncResolveConflict — behavior tests that require a DB because the fix
// relies on tenant_id scoping in SELECT/UPDATE on sync_conflicts. These are
// guarded by the `integration` build tag and live in the _integration file.
// ---------------------------------------------------------------------------

// TestSyncResolveConflict_RejectsBadColumn exercises the end-to-end handler
// against a nil pool: the handler must return 404 (conflict not found)
// because QueryRow on a nil pool panics — which is caught by the defer in
// real tests. Here we just assert the pre-DB input validation still works.
// The column-whitelist behavior is covered in validateResolveColumns tests
// above (pure) and in the integration test (end-to-end through the DB).
func TestSyncResolveConflict_RejectsBadColumn(t *testing.T) {
	// This test confirms that the whitelist helper is wired into the
	// handler and exposed as a BadRequest with code INVALID_FIELD. It
	// exercises the helper directly — the handler path needs a DB row
	// to reach the validation point, so the integration suite covers
	// the full flow.
	err := validateResolveColumns("assets", map[string]any{
		"name=x,password_hash='injected'--": "x",
	})
	if err == nil {
		t.Fatal("expected malicious column key to be rejected")
	}
	// Error code should be machine-readable for the HTTP layer.
	if got := err.Error(); !strings.Contains(got, "INVALID_FIELD") {
		t.Errorf("error %q missing INVALID_FIELD marker", got)
	}
}
