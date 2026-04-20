// Package pgxpool is a minimal stub used only by analysistest
// fixtures. The real package lives in github.com/jackc/pgx/v5. For
// the analyzer we only need the type and method signatures so that
// the fixture type-checks and the selector expression resolves to
// *pgxpool.Pool.
package pgxpool

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Pool is a stand-in for the real pgxpool.Pool struct.
type Pool struct{}

func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgx.CommandTag, error) {
	return pgx.CommandTag{}, nil
}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
