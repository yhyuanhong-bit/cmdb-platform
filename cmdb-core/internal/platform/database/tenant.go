package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//tenantlint:allow-direct-pool — TenantScoped implementation itself

// TenantScoped wraps a pgxpool.Pool and enforces that every SQL statement
// passed to Exec/Query/QueryRow references `tenant_id` and receives the
// current tenant as its first positional argument ($1).
type TenantScoped struct {
	pool     *pgxpool.Pool
	tenantID uuid.UUID
}

// Scope returns a TenantScoped bound to a tenant. Panics on uuid.Nil —
// code that does not have a concrete tenant must not use this.
func Scope(pool *pgxpool.Pool, tenantID uuid.UUID) *TenantScoped {
	if pool == nil {
		panic("database.Scope: nil pool")
	}
	if tenantID == uuid.Nil {
		panic("database.Scope: tenant scope requires non-nil tenant; use pool directly for cross-tenant work with explicit justification")
	}
	return &TenantScoped{pool: pool, tenantID: tenantID}
}

// TenantID returns the bound tenant ID (useful for logging / metrics).
func (s *TenantScoped) TenantID() uuid.UUID { return s.tenantID }

var errNoTenantReference = errors.New("database: tenant-scoped query must reference tenant_id in SQL")

// requireTenantReference is a heuristic guard. It rejects statements that
// do not textually contain "tenant_id". This is intentionally strict —
// SQL that truly does not filter by tenant must either use the raw pool
// with a comment explaining why, or go through ListActiveTenants-style
// cross-tenant APIs.
func requireTenantReference(sql string) error {
	if !strings.Contains(strings.ToLower(sql), "tenant_id") {
		return fmt.Errorf("%w: %q", errNoTenantReference, truncate(sql, 160))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// prepareArgs prepends the bound tenantID so the SQL should reference it as $1.
func (s *TenantScoped) prepareArgs(args []any) []any {
	out := make([]any, 0, len(args)+1)
	out = append(out, s.tenantID)
	out = append(out, args...)
	return out
}

// Exec runs a tenant-scoped Exec. The SQL must reference `tenant_id`; the
// bound tenant is prepended as $1.
func (s *TenantScoped) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := requireTenantReference(sql); err != nil {
		return pgconn.CommandTag{}, err
	}
	return s.pool.Exec(ctx, sql, s.prepareArgs(args)...)
}

// Query runs a tenant-scoped Query. The SQL must reference `tenant_id`; the
// bound tenant is prepended as $1.
func (s *TenantScoped) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := requireTenantReference(sql); err != nil {
		return nil, err
	}
	return s.pool.Query(ctx, sql, s.prepareArgs(args)...)
}

// QueryRow runs a tenant-scoped QueryRow. The SQL must reference `tenant_id`;
// the bound tenant is prepended as $1. On guard failure, returns a row whose
// Scan returns the guard error — so callers never observe a cross-tenant
// result by accident.
func (s *TenantScoped) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := requireTenantReference(sql); err != nil {
		return &errRow{err: err}
	}
	return s.pool.QueryRow(ctx, sql, s.prepareArgs(args)...)
}

// Pool returns the underlying pool for cases where a cross-tenant query is
// deliberate (e.g. background schedulers iterating all tenants).
// Every caller MUST add a "// cross-tenant:" justification comment.
func (s *TenantScoped) Pool() *pgxpool.Pool { return s.pool }

type errRow struct{ err error }

func (r *errRow) Scan(dest ...any) error { return r.err }
