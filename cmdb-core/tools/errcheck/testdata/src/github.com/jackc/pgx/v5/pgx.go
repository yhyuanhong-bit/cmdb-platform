// Package pgx is a minimal stub used only by analysistest fixtures.
// The real package lives in github.com/jackc/pgx/v5. We only need
// the Rows interface so that *pgxpool.Pool.Query's return value
// resolves to pgx.Rows and the analyzer's receiver-type match works.
package pgx

// Rows mirrors the real pgx.Rows interface closely enough for the
// fixtures to type-check. Scan is the method we watch.
type Rows interface {
	Next() bool
	Close()
	Err() error
	Scan(dest ...any) error
}

// Row mirrors pgx.Row.
type Row interface {
	Scan(dest ...any) error
}

// CommandTag mirrors pgconn.CommandTag — we only need a return type
// for Exec.
type CommandTag struct{}

// RowsAffected is present because call sites reference it, but the
// analyzer doesn't watch it.
func (CommandTag) RowsAffected() int64 { return 0 }
