// Package allowed demonstrates the per-line escape hatch. Each
// suppressed-error call carries a leading `//errcheck:allow` comment
// on the line immediately above, so the analyzer emits zero
// diagnostics against this file even though the patterns are
// otherwise exactly the shapes flagged in the bad fixture.
//
// Note: the word that triggers analysistest's annotation parser is
// deliberately absent from this prose so the golden file stays
// stable as a pure-negative fixture.
package allowed

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

func allowedBlankExec(ctx context.Context, p *pgxpool.Pool) {
	//errcheck:allow
	_, _ = p.Exec(ctx, "UPDATE foo SET bar = 1")
}

func allowedBareExec(ctx context.Context, p *pgxpool.Pool) {
	//errcheck:allow
	p.Exec(ctx, "UPDATE foo SET bar = 1")
}

func allowedBlankUnmarshal(data []byte) {
	var out struct{ A int }
	//errcheck:allow
	_ = json.Unmarshal(data, &out)
	_ = out
}

func allowedBareUnmarshal(data []byte) {
	var out struct{ A int }
	//errcheck:allow
	json.Unmarshal(data, &out)
	_ = out
}

func allowedScan(ctx context.Context, p *pgxpool.Pool) {
	rows, err := p.Query(ctx, "SELECT 1")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var x int
		//errcheck:allow
		rows.Scan(&x)
		_ = x
	}
}
