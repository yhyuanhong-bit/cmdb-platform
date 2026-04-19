package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestScope_PanicsOnNilTenant(t *testing.T) {
	t.Run("nil pool panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic for nil pool, got none")
			}
		}()
		_ = Scope(nil, uuid.Nil)
	})

	t.Run("nil tenant panics even with non-nil pool", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for uuid.Nil tenant, got none")
			}
			msg, ok := r.(string)
			if !ok {
				t.Fatalf("expected string panic, got %T: %v", r, r)
			}
			if !strings.Contains(msg, "non-nil tenant") {
				t.Fatalf("panic message missing 'non-nil tenant' hint: %q", msg)
			}
		}()
		// A zero-valued *pgxpool.Pool is non-nil as a pointer, so the first
		// guard passes and the second (uuid.Nil) fires. We never dereference it.
		fakePool := &pgxpool.Pool{}
		_ = Scope(fakePool, uuid.Nil)
	})
}

func TestRequireTenantReference_Rejects(t *testing.T) {
	cases := []string{
		"SELECT * FROM assets WHERE id=$1",
		"DELETE FROM incidents WHERE id=$1",
		"UPDATE users SET name=$2 WHERE id=$1",
		"",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			err := requireTenantReference(sql)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", sql)
			}
			if !errors.Is(err, errNoTenantReference) {
				t.Fatalf("expected errNoTenantReference, got %v", err)
			}
		})
	}
}

func TestRequireTenantReference_Accepts(t *testing.T) {
	cases := []string{
		"SELECT * FROM assets WHERE tenant_id=$1 AND id=$2",
		"INSERT INTO assets (tenant_id, name) VALUES ($1, $2)",
		"SELECT * FROM a JOIN b ON a.tenant_id = b.tenant_id WHERE a.id=$1",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			if err := requireTenantReference(sql); err != nil {
				t.Fatalf("expected nil error for %q, got %v", sql, err)
			}
		})
	}
}

func TestRequireTenantReference_CaseInsensitive(t *testing.T) {
	cases := []string{
		"SELECT * FROM a WHERE TENANT_ID=$1",
		"SELECT * FROM a WHERE Tenant_Id=$1",
		"select * from a where tenant_id=$1",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			if err := requireTenantReference(sql); err != nil {
				t.Fatalf("expected nil error for %q, got %v", sql, err)
			}
		})
	}
}

func TestPrepareArgs_PrependsTenant(t *testing.T) {
	tenant := uuid.New()
	s := &TenantScoped{tenantID: tenant}

	out := s.prepareArgs([]any{"asset-123", 42})

	if len(out) != 3 {
		t.Fatalf("expected 3 args, got %d", len(out))
	}
	gotTenant, ok := out[0].(uuid.UUID)
	if !ok {
		t.Fatalf("expected uuid.UUID at position 0, got %T", out[0])
	}
	if gotTenant != tenant {
		t.Fatalf("expected tenant %s at position 0, got %s", tenant, gotTenant)
	}
	if out[1] != "asset-123" {
		t.Fatalf("expected original args[0] at position 1, got %v", out[1])
	}
	if out[2] != 42 {
		t.Fatalf("expected original args[1] at position 2, got %v", out[2])
	}
}

func TestPrepareArgs_NoArgs(t *testing.T) {
	tenant := uuid.New()
	s := &TenantScoped{tenantID: tenant}

	out := s.prepareArgs(nil)

	if len(out) != 1 {
		t.Fatalf("expected 1 arg (tenant only), got %d", len(out))
	}
	if out[0] != tenant {
		t.Fatalf("expected tenant, got %v", out[0])
	}
}

// TestQueryRow_ReturnsErrRowOnMissingReference manually constructs a
// TenantScoped with no pool (we never call into it because the guard
// short-circuits), confirming QueryRow returns an errRow whose Scan
// surfaces the guard error without panicking.
func TestQueryRow_ReturnsErrRowOnMissingReference(t *testing.T) {
	s := &TenantScoped{tenantID: uuid.New()}

	row := s.QueryRow(nil, "SELECT * FROM assets WHERE id=$1", "x")
	if row == nil {
		t.Fatalf("expected non-nil row")
	}

	var dest string
	err := row.Scan(&dest)
	if err == nil {
		t.Fatalf("expected guard error from Scan, got nil")
	}
	if !errors.Is(err, errNoTenantReference) {
		t.Fatalf("expected errNoTenantReference, got %v", err)
	}
}

func TestExec_ReturnsErrorOnMissingReference(t *testing.T) {
	s := &TenantScoped{tenantID: uuid.New()}

	tag, err := s.Exec(nil, "DELETE FROM assets WHERE id=$1", "x")
	if err == nil {
		t.Fatalf("expected guard error, got nil")
	}
	if !errors.Is(err, errNoTenantReference) {
		t.Fatalf("expected errNoTenantReference, got %v", err)
	}
	// pgconn.CommandTag zero value stringifies to "" — confirm we returned it.
	if tag.String() != "" {
		t.Fatalf("expected zero CommandTag on guard error, got %q", tag.String())
	}
}

func TestQuery_ReturnsErrorOnMissingReference(t *testing.T) {
	s := &TenantScoped{tenantID: uuid.New()}

	rows, err := s.Query(nil, "SELECT id FROM assets")
	if err == nil {
		t.Fatalf("expected guard error, got nil")
	}
	if !errors.Is(err, errNoTenantReference) {
		t.Fatalf("expected errNoTenantReference, got %v", err)
	}
	if rows != nil {
		t.Fatalf("expected nil rows on guard error, got %v", rows)
	}
}

func TestTenantID_Exposed(t *testing.T) {
	tenant := uuid.New()
	s := &TenantScoped{tenantID: tenant}
	if got := s.TenantID(); got != tenant {
		t.Fatalf("TenantID() = %s, want %s", got, tenant)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("truncate short: got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Fatalf("truncate long: got %q", got)
	}
}
