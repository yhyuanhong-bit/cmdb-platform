//go:build integration

package location_detect

import (
	"context"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestIntegration_CreateWorkOrderLog_AutoClose pins the shape of the
// work_order_logs row emitted by the location_detect auto-close path,
// now that it goes through the sqlc-generated CreateWorkOrderLog. A
// future change to the query source that drops from_status/to_status
// or re-orders the columns will flip this test immediately.
//
// Run with:
//   go test -tags integration -run TestIntegration_CreateWorkOrderLog_AutoClose ./internal/domain/location_detect/...

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

func TestIntegration_CreateWorkOrderLog_AutoClose(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID := uuid.New()
	orderID := uuid.New()
	suffix := tenantID.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		tenantID, "ld-wol-"+suffix, "ld-wol-"+suffix); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO work_orders (id, tenant_id, code, type, priority, status, title)
		 VALUES ($1, $2, $3, 'relocation', 'medium', 'in_progress', 'ld-wol-title')`,
		orderID, tenantID, "WO-LD-"+suffix); err != nil {
		t.Fatalf("insert work_orders: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_order_logs WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM work_orders WHERE id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	// Exercise the exact same CreateWorkOrderLogParams shape the
	// auto-close path uses — from_status/to_status both Valid, no
	// operator, no comment.
	result, err := dbgen.New(pool).CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
		OrderID:    orderID,
		Action:     "auto_completed_by_location_detect",
		FromStatus: pgtype.Text{String: "in_progress", Valid: true},
		ToStatus:   pgtype.Text{String: "completed", Valid: true},
		OperatorID: pgtype.UUID{},
		Comment:    pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("CreateWorkOrderLog: %v", err)
	}

	// The generated helper returns the inserted row; verify the
	// load-bearing fields round-tripped as expected.
	if result.OrderID != orderID {
		t.Errorf("result.OrderID = %s, want %s", result.OrderID, orderID)
	}
	if result.Action != "auto_completed_by_location_detect" {
		t.Errorf("result.Action = %q, want %q", result.Action, "auto_completed_by_location_detect")
	}
	if !result.FromStatus.Valid || result.FromStatus.String != "in_progress" {
		t.Errorf("result.FromStatus = %+v, want 'in_progress'", result.FromStatus)
	}
	if !result.ToStatus.Valid || result.ToStatus.String != "completed" {
		t.Errorf("result.ToStatus = %+v, want 'completed'", result.ToStatus)
	}
	if result.OperatorID.Valid {
		t.Errorf("result.OperatorID should be NULL for auto-close, got %+v", result.OperatorID)
	}
	if result.Comment.Valid {
		t.Errorf("result.Comment should be NULL for auto-close, got %+v", result.Comment)
	}
}
