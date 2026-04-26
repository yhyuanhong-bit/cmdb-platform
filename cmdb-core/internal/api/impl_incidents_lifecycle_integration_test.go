//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 5.1 lifecycle coverage. The five transition helpers (Acknowledge,
// StartInvestigating, Resolve, Close, Reopen) have exactly two things the
// caller cares about:
//
//   1. Valid transitions succeed, write the right fields, and leave a
//      system comment in the timeline in the same tx.
//   2. Invalid transitions return ErrInvalidStateTransition and mutate
//      nothing — the row stays in its previous state, no ghost comment.
//
// Tenant isolation is checked on a representative subset; ackowledge is
// enough since all five share the same WHERE tenant_id guard.

func newIncidentsTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

func seedTwoTenantsForIncidents(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "inc-A-"+suffix, "inc-a-"+suffix,
		b, "inc-B-"+suffix, "inc-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM incident_comments WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM incidents WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

func insertIncident(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, severity string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO incidents (id, tenant_id, title, status, severity, started_at)
		 VALUES ($1, $2, $3, 'open', $4, now())`,
		id, tenantID, "inc-"+id.String()[:8], severity,
	); err != nil {
		t.Fatalf("insert incident: %v", err)
	}
	return id
}

func readIncidentStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM incidents WHERE id = $1`, id,
	).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

func countComments(t *testing.T, pool *pgxpool.Pool, incidentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM incident_comments WHERE incident_id = $1`, incidentID,
	).Scan(&n); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	return n
}

func TestIncidents_FullLifecycle_OpenToClosed(t *testing.T) {
	pool := newIncidentsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForIncidents(t, pool)
	q := dbgen.New(pool)
	svc := monitoring.NewService(q, nil, pool)

	id := insertIncident(t, pool, tenantA, "high")
	// Reviewer user not required for integration — we pass uuid.Nil and
	// expect AuthorID to be stored as a null pgtype.UUID (the schema
	// allows nullable author_id for system/automation-driven transitions).
	reviewer := uuid.Nil

	// open → acknowledged
	if _, err := svc.AcknowledgeIncident(ctx, tenantA, id, reviewer, "saw the page"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if got := readIncidentStatus(t, pool, id); got != "acknowledged" {
		t.Errorf("after ack status=%q want acknowledged", got)
	}
	if n := countComments(t, pool, id); n != 1 {
		t.Errorf("after ack comments=%d want 1", n)
	}

	// acknowledged → investigating
	if _, err := svc.StartInvestigatingIncident(ctx, tenantA, id, reviewer); err != nil {
		t.Fatalf("start investigating: %v", err)
	}
	if got := readIncidentStatus(t, pool, id); got != "investigating" {
		t.Errorf("after investigating status=%q want investigating", got)
	}
	if n := countComments(t, pool, id); n != 2 {
		t.Errorf("comments=%d want 2", n)
	}

	// investigating → resolved (with root_cause)
	if _, err := svc.ResolveIncident(ctx, tenantA, id, reviewer, "bad config push", ""); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := readIncidentStatus(t, pool, id); got != "resolved" {
		t.Errorf("after resolve status=%q want resolved", got)
	}
	// Verify root_cause stored on the row.
	var rc pgtype.Text
	_ = pool.QueryRow(ctx, `SELECT root_cause FROM incidents WHERE id = $1`, id).Scan(&rc)
	if !rc.Valid || rc.String != "bad config push" {
		t.Errorf("root_cause not persisted: valid=%v str=%q", rc.Valid, rc.String)
	}
	// Verify resolved_at stamped.
	var resolvedAt pgtype.Timestamptz
	_ = pool.QueryRow(ctx, `SELECT resolved_at FROM incidents WHERE id = $1`, id).Scan(&resolvedAt)
	if !resolvedAt.Valid {
		t.Error("resolved_at not stamped")
	}

	// resolved → closed (post-mortem lock)
	if _, err := svc.CloseIncident(ctx, tenantA, id, reviewer); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := readIncidentStatus(t, pool, id); got != "closed" {
		t.Errorf("after close status=%q want closed", got)
	}

	// Closed is terminal — resolve again must fail with invalid transition,
	// status stays closed, and no extra comment is written.
	before := countComments(t, pool, id)
	if _, err := svc.ResolveIncident(ctx, tenantA, id, reviewer, "", ""); err != monitoring.ErrInvalidStateTransition {
		t.Fatalf("resolve on closed: want ErrInvalidStateTransition, got %v", err)
	}
	if got := readIncidentStatus(t, pool, id); got != "closed" {
		t.Errorf("after failed resolve status mutated to %q", got)
	}
	if after := countComments(t, pool, id); after != before {
		t.Errorf("failed transition still wrote comment: before=%d after=%d", before, after)
	}
}

func TestIncidents_Reopen_ResolvedToOpen(t *testing.T) {
	pool := newIncidentsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForIncidents(t, pool)
	q := dbgen.New(pool)
	svc := monitoring.NewService(q, nil, pool)

	id := insertIncident(t, pool, tenantA, "medium")
	if _, err := svc.ResolveIncident(ctx, tenantA, id, uuid.Nil, "premature", ""); err != nil {
		t.Fatalf("resolve from open: %v", err)
	}
	// Reopen must clear resolved_at/resolved_by so downstream reports don't
	// see a ghost resolution on a live incident.
	if _, err := svc.ReopenIncident(ctx, tenantA, id, uuid.Nil, "regression surfaced"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	var status string
	var resolvedAt pgtype.Timestamptz
	var resolvedBy pgtype.UUID
	_ = pool.QueryRow(ctx,
		`SELECT status, resolved_at, resolved_by FROM incidents WHERE id=$1`, id,
	).Scan(&status, &resolvedAt, &resolvedBy)
	if status != "open" {
		t.Errorf("after reopen status=%q want open", status)
	}
	if resolvedAt.Valid {
		t.Errorf("after reopen resolved_at should be NULL, got %v", resolvedAt.Time)
	}
	if resolvedBy.Valid {
		t.Errorf("after reopen resolved_by should be NULL")
	}
}

func TestIncidents_IllegalTransitions(t *testing.T) {
	pool := newIncidentsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForIncidents(t, pool)
	q := dbgen.New(pool)
	svc := monitoring.NewService(q, nil, pool)

	id := insertIncident(t, pool, tenantA, "low")

	// close from open — skips the resolved state, must fail.
	if _, err := svc.CloseIncident(ctx, tenantA, id, uuid.Nil); err != monitoring.ErrInvalidStateTransition {
		t.Errorf("close from open: want ErrInvalidStateTransition, got %v", err)
	}
	// reopen from open — only valid from resolved.
	if _, err := svc.ReopenIncident(ctx, tenantA, id, uuid.Nil, ""); err != monitoring.ErrInvalidStateTransition {
		t.Errorf("reopen from open: want ErrInvalidStateTransition, got %v", err)
	}
	// start-investigating from open — only valid from acknowledged.
	if _, err := svc.StartInvestigatingIncident(ctx, tenantA, id, uuid.Nil); err != monitoring.ErrInvalidStateTransition {
		t.Errorf("start-investigating from open: want ErrInvalidStateTransition, got %v", err)
	}

	// status must still be open after all those failed attempts.
	if got := readIncidentStatus(t, pool, id); got != "open" {
		t.Errorf("status mutated by failed transitions: got %q", got)
	}
	// And no comments should have been written either.
	if n := countComments(t, pool, id); n != 0 {
		t.Errorf("failed transitions wrote %d comments, want 0", n)
	}
}

func TestIncidents_AcknowledgeIsTenantScoped(t *testing.T) {
	pool := newIncidentsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForIncidents(t, pool)
	q := dbgen.New(pool)
	svc := monitoring.NewService(q, nil, pool)

	// Incident lives in tenant A. Tenant B calls acknowledge with the same
	// incident id — must fail, row must not be touched, no comment written.
	id := insertIncident(t, pool, tenantA, "critical")
	if _, err := svc.AcknowledgeIncident(ctx, tenantB, id, uuid.Nil, "nice try"); err != monitoring.ErrInvalidStateTransition {
		t.Fatalf("cross-tenant ack: want ErrInvalidStateTransition, got %v", err)
	}
	if got := readIncidentStatus(t, pool, id); got != "open" {
		t.Errorf("cross-tenant ack mutated status: got %q", got)
	}
	if n := countComments(t, pool, id); n != 0 {
		t.Errorf("cross-tenant ack wrote %d comments, want 0", n)
	}
}

func TestIncidents_TimelineOrdering(t *testing.T) {
	pool := newIncidentsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForIncidents(t, pool)
	q := dbgen.New(pool)
	svc := monitoring.NewService(q, nil, pool)

	id := insertIncident(t, pool, tenantA, "high")

	// Acknowledge then resolve with a small sleep between — the timeline
	// should come back ascending by created_at.
	if _, err := svc.AcknowledgeIncident(ctx, tenantA, id, uuid.Nil, ""); err != nil {
		t.Fatalf("ack: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // clock tick so ordering is deterministic
	if _, err := svc.ResolveIncident(ctx, tenantA, id, uuid.Nil, "cause", ""); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// Append a human comment — must land after both system comments.
	time.Sleep(10 * time.Millisecond)
	if _, err := svc.AddIncidentComment(ctx, tenantA, id, uuid.Nil, "human", "post-mortem scheduled"); err != nil {
		t.Fatalf("add comment: %v", err)
	}

	rows, err := svc.ListIncidentComments(ctx, tenantA, id)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("timeline size=%d want 3", len(rows))
	}
	// system, system, human (ascending by created_at).
	if rows[0].Kind != "system" || rows[1].Kind != "system" || rows[2].Kind != "human" {
		t.Errorf("timeline kinds = %v, want [system system human]", []string{rows[0].Kind, rows[1].Kind, rows[2].Kind})
	}
	// created_at strictly non-decreasing.
	for i := 1; i < len(rows); i++ {
		if rows[i].CreatedAt.Before(rows[i-1].CreatedAt) {
			t.Errorf("row %d created_at %v before row %d %v", i, rows[i].CreatedAt, i-1, rows[i-1].CreatedAt)
		}
	}
}
