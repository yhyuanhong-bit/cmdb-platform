package good

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Good patterns — none of these should be flagged.

func checkedExec(ctx context.Context, p *pgxpool.Pool) error {
	_, err := p.Exec(ctx, "UPDATE foo SET bar = 1")
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

func checkedQuery(ctx context.Context, p *pgxpool.Pool) error {
	rows, err := p.Query(ctx, "SELECT 1")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var x int
		if err := rows.Scan(&x); err != nil {
			return err
		}
		_ = x
	}
	return rows.Err()
}

func checkedUnmarshal(data []byte) error {
	var out struct{ A int }
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	_ = out
	return nil
}

func checkedUnmarshalReturningErr(data []byte) error {
	var out struct{ A int }
	err := json.Unmarshal(data, &out)
	if err != nil {
		return err
	}
	_ = out
	return nil
}

// rows.Scan checked via an early-return assignment is also fine —
// the analyzer walks up to three statements forward looking for
// any reference to the assigned err variable.
func scanErrCheckedViaAssignment(ctx context.Context, p *pgxpool.Pool) error {
	rows, err := p.Query(ctx, "SELECT 1")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var x int
		err := rows.Scan(&x)
		if err != nil {
			return err
		}
		_ = x
	}
	return nil
}

// Scan inside an if-clause is the canonical form and must not trip
// the analyzer.
func scanErrCheckedInline(ctx context.Context, p *pgxpool.Pool) error {
	rows, err := p.Query(ctx, "SELECT 1")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var x int
		if err := rows.Scan(&x); err != nil {
			return err
		}
		_ = x
	}
	return nil
}
