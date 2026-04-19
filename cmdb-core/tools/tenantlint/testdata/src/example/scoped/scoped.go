package scoped

import (
	"context"
)

// TenantScoped is a stand-in for database.TenantScoped. The real analyzer
// checks the receiver type against *pgxpool.Pool, so any other receiver
// (including a project-local wrapper) must not be flagged.
type TenantScoped struct{}

func (s *TenantScoped) Exec(ctx context.Context, sql string, args ...any) error {
	return nil
}

func (s *TenantScoped) Query(ctx context.Context, sql string, args ...any) error {
	return nil
}

func (s *TenantScoped) QueryRow(ctx context.Context, sql string, args ...any) error {
	return nil
}

// HandlerScoped is the pattern we want — domain code only talks to the
// wrapper and never sees the raw pool. None of these calls should be flagged.
func HandlerScoped(ctx context.Context, s *TenantScoped, id string) error {
	if err := s.Exec(ctx, "DELETE FROM assets WHERE tenant_id=$1 AND id=$2", id); err != nil {
		return err
	}
	if err := s.Query(ctx, "SELECT id FROM assets WHERE tenant_id=$1"); err != nil {
		return err
	}
	return s.QueryRow(ctx, "SELECT id FROM assets WHERE tenant_id=$1")
}
