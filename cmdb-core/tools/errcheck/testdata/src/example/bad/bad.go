package bad

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Suppressed-error patterns we flag. Each annotated call site
// corresponds to exactly one diagnostic emitted by the analyzer.

func blankExecBothResults(ctx context.Context, p *pgxpool.Pool) {
	_, _ = p.Exec(ctx, "UPDATE foo SET bar = 1") // want `pool\.Exec error is discarded`
}

func blankExecSingleResult(ctx context.Context, p *pgxpool.Pool) {
	_, _ = p.Query(ctx, "SELECT 1") // want `pool\.Query error is discarded`
}

func bareExecStatement(ctx context.Context, p *pgxpool.Pool) {
	p.Exec(ctx, "UPDATE foo SET bar = 1") // want `pool\.Exec error is discarded`
}

func bareQueryStatement(ctx context.Context, p *pgxpool.Pool) {
	p.Query(ctx, "SELECT 1") // want `pool\.Query error is discarded`
}

func blankUnmarshal(data []byte) {
	var out struct{ A int }
	_ = json.Unmarshal(data, &out) // want `json\.Unmarshal error is discarded`
}

func bareUnmarshal(data []byte) {
	var out struct{ A int }
	json.Unmarshal(data, &out) // want `json\.Unmarshal error is discarded`
}

func unusedUnmarshalErr(data []byte) {
	var out struct{ A int }
	err := json.Unmarshal(data, &out) // want `json\.Unmarshal error is assigned but never checked`
	_ = out
	_ = err
}

func scanErrUnchecked(ctx context.Context, p *pgxpool.Pool) {
	rows, err := p.Query(ctx, "SELECT 1")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var x int
		rows.Scan(&x) // want `rows\.Scan error is not checked`
		_ = x
	}
}

func scanErrBlankAssigned(ctx context.Context, p *pgxpool.Pool) {
	rows, err := p.Query(ctx, "SELECT 1")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var x int
		_ = rows.Scan(&x) // want `rows\.Scan error is not checked`
		_ = x
	}
}
