// Package pgxpool is a minimal stub used only by analysistest fixtures.
// The real package lives in github.com/jackc/pgx/v5. For the analyzer we
// only need the type and method signatures so that the fixture type-checks
// and the selector expression resolves to *pgxpool.Pool.
package pgxpool

import "context"

// Pool is a stand-in for the real pgxpool.Pool struct.
type Pool struct{}

// CommandTag mirrors the real pgconn.CommandTag just enough to return.
type CommandTag struct{}

// Row mirrors pgx.Row.
type Row interface {
	Scan(dest ...any) error
}

// Rows mirrors pgx.Rows.
type Rows interface {
	Next() bool
	Close()
}

func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	return CommandTag{}, nil
}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return nil, nil
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return nil
}
