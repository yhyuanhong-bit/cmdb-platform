//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/problem"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 5.2: Problem entity coverage.
//
// Same shape as Wave 5.1 incident lifecycle tests, plus the M:N linkage
// behaviour that makes Problem useful in the first place.

func newProblemsTestPool(t *testing.T) *pgxpool.Pool {
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

func seedTwoTenantsForProblems(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "prob-A-"+suffix, "prob-a-"+suffix,
		b, "prob-B-"+suffix, "prob-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM problem_comments WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM incident_problem_links WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM incident_comments WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM problems WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM incidents WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

func insertIncidentForProblem(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO incidents (id, tenant_id, title, status, severity, started_at)
		 VALUES ($1, $2, $3, 'open', 'medium', now())`,
		id, tenantID, "linked-incident-"+id.String()[:8],
	); err != nil {
		t.Fatalf("insert incident: %v", err)
	}
	return id
}

func readProblemStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM problems WHERE id = $1`, id,
	).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

func countProblemComments(t *testing.T, pool *pgxpool.Pool, problemID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM problem_comments WHERE problem_id = $1`, problemID,
	).Scan(&n); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	return n
}

func TestProblems_FullLifecycle_OpenToClosed(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	created, err := svc.Create(ctx, problem.CreateParams{
		TenantID: tenantA, Title: "auth memory leak", Severity: "high",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.ID
	if created.Status != "open" {
		t.Errorf("create: status=%q want open", created.Status)
	}

	// open → investigating
	if _, err := svc.StartInvestigation(ctx, tenantA, id, uuid.Nil, "looking into the heap"); err != nil {
		t.Fatalf("start investigation: %v", err)
	}
	if got := readProblemStatus(t, pool, id); got != "investigating" {
		t.Errorf("status=%q want investigating", got)
	}
	if n := countProblemComments(t, pool, id); n != 1 {
		t.Errorf("comments=%d want 1", n)
	}

	// investigating → known_error (workaround required)
	if _, err := svc.MarkKnownError(ctx, tenantA, id, uuid.Nil, "", ""); err == nil {
		t.Errorf("mark known error with empty workaround should fail")
	}
	if _, err := svc.MarkKnownError(ctx, tenantA, id, uuid.Nil, "rolling restart every 4h", ""); err != nil {
		t.Fatalf("mark known error: %v", err)
	}
	if got := readProblemStatus(t, pool, id); got != "known_error" {
		t.Errorf("status=%q want known_error", got)
	}
	// Verify workaround persisted on the row.
	var workaround pgtype.Text
	_ = pool.QueryRow(ctx, `SELECT workaround FROM problems WHERE id = $1`, id).Scan(&workaround)
	if !workaround.Valid || workaround.String != "rolling restart every 4h" {
		t.Errorf("workaround not persisted: %v", workaround)
	}

	// known_error → resolved
	if _, err := svc.Resolve(ctx, tenantA, id, uuid.Nil, "leaky cache eviction", "patch v2.4.1 ships heap fix", ""); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := readProblemStatus(t, pool, id); got != "resolved" {
		t.Errorf("status=%q want resolved", got)
	}
	var rootCause, resolution pgtype.Text
	var resolvedAt pgtype.Timestamptz
	_ = pool.QueryRow(ctx, `SELECT root_cause, resolution, resolved_at FROM problems WHERE id = $1`, id).Scan(&rootCause, &resolution, &resolvedAt)
	if !rootCause.Valid || rootCause.String != "leaky cache eviction" {
		t.Errorf("root_cause not persisted")
	}
	if !resolution.Valid || resolution.String != "patch v2.4.1 ships heap fix" {
		t.Errorf("resolution not persisted")
	}
	if !resolvedAt.Valid {
		t.Errorf("resolved_at not stamped")
	}

	// resolved → closed
	if _, err := svc.Close(ctx, tenantA, id, uuid.Nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := readProblemStatus(t, pool, id); got != "closed" {
		t.Errorf("status=%q want closed", got)
	}

	// closed is terminal — close again rejected, no extra comment written.
	before := countProblemComments(t, pool, id)
	if _, err := svc.Close(ctx, tenantA, id, uuid.Nil); err != problem.ErrInvalidStateTransition {
		t.Errorf("re-close: want ErrInvalidStateTransition, got %v", err)
	}
	if after := countProblemComments(t, pool, id); after != before {
		t.Errorf("re-close wrote a comment: before=%d after=%d", before, after)
	}
}

func TestProblems_IllegalTransitions(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	created, err := svc.Create(ctx, problem.CreateParams{TenantID: tenantA, Title: "x", Severity: "low"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.ID

	// Skipping investigating → resolved direct from open is intentionally
	// blocked. We want the investigation step recorded even if it's brief.
	if _, err := svc.Resolve(ctx, tenantA, id, uuid.Nil, "", "", ""); err != problem.ErrInvalidStateTransition {
		t.Errorf("resolve from open: want ErrInvalidStateTransition, got %v", err)
	}
	// Close from open: must fail.
	if _, err := svc.Close(ctx, tenantA, id, uuid.Nil); err != problem.ErrInvalidStateTransition {
		t.Errorf("close from open: want ErrInvalidStateTransition, got %v", err)
	}
	// MarkKnownError from open: must fail (only valid from investigating).
	if _, err := svc.MarkKnownError(ctx, tenantA, id, uuid.Nil, "wa", ""); err != problem.ErrInvalidStateTransition {
		t.Errorf("known_error from open: want ErrInvalidStateTransition, got %v", err)
	}

	if got := readProblemStatus(t, pool, id); got != "open" {
		t.Errorf("status mutated by failed transitions: %q", got)
	}
	if n := countProblemComments(t, pool, id); n != 0 {
		t.Errorf("failed transitions wrote %d comments, want 0", n)
	}
}

func TestProblems_ReopenClearsResolution(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	created, err := svc.Create(ctx, problem.CreateParams{TenantID: tenantA, Title: "x", Severity: "low"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.StartInvestigation(ctx, tenantA, created.ID, uuid.Nil, ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := svc.Resolve(ctx, tenantA, created.ID, uuid.Nil, "rc", "rez", ""); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := svc.Reopen(ctx, tenantA, created.ID, uuid.Nil, "regression spotted"); err != nil {
		t.Fatalf("reopen: %v", err)
	}

	var status string
	var resolvedAt pgtype.Timestamptz
	var resolvedBy pgtype.UUID
	var resolution pgtype.Text
	_ = pool.QueryRow(ctx,
		`SELECT status, resolved_at, resolved_by, resolution FROM problems WHERE id=$1`, created.ID,
	).Scan(&status, &resolvedAt, &resolvedBy, &resolution)
	if status != "investigating" {
		t.Errorf("after reopen status=%q want investigating", status)
	}
	if resolvedAt.Valid {
		t.Errorf("after reopen resolved_at should be NULL")
	}
	if resolvedBy.Valid {
		t.Errorf("after reopen resolved_by should be NULL")
	}
	if resolution.Valid {
		t.Errorf("after reopen resolution should be NULL, got %q", resolution.String)
	}
}

func TestProblems_TenantIsolation(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	created, err := svc.Create(ctx, problem.CreateParams{TenantID: tenantA, Title: "tenant-A only", Severity: "medium"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Tenant B can't see it.
	if _, err := svc.Get(ctx, tenantB, created.ID); err != problem.ErrNotFound {
		t.Errorf("cross-tenant Get: want ErrNotFound, got %v", err)
	}
	// Tenant B can't transition it (the WHERE-tenant guard makes this hit
	// zero rows, surfaced as ErrInvalidStateTransition — same effect: the
	// row is untouched).
	if _, err := svc.StartInvestigation(ctx, tenantB, created.ID, uuid.Nil, "nice try"); err != problem.ErrInvalidStateTransition {
		t.Errorf("cross-tenant StartInvestigation: want ErrInvalidStateTransition, got %v", err)
	}
	if got := readProblemStatus(t, pool, created.ID); got != "open" {
		t.Errorf("cross-tenant transition leaked: status=%q", got)
	}
}

func TestProblems_LinkageWithIncidents(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	prob, err := svc.Create(ctx, problem.CreateParams{TenantID: tenantA, Title: "DB exhaustion", Severity: "high"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	inc1 := insertIncidentForProblem(t, pool, tenantA)
	inc2 := insertIncidentForProblem(t, pool, tenantA)

	// Link both incidents.
	if err := svc.LinkIncident(ctx, tenantA, inc1, prob.ID, uuid.Nil); err != nil {
		t.Fatalf("link inc1: %v", err)
	}
	if err := svc.LinkIncident(ctx, tenantA, inc2, prob.ID, uuid.Nil); err != nil {
		t.Fatalf("link inc2: %v", err)
	}

	// Idempotent: re-linking does not error and does not duplicate.
	if err := svc.LinkIncident(ctx, tenantA, inc1, prob.ID, uuid.Nil); err != nil {
		t.Fatalf("re-link inc1: %v", err)
	}

	// Reverse lookup: problem has 2 incidents.
	rows, err := svc.ListIncidentsForProblem(ctx, tenantA, prob.ID)
	if err != nil {
		t.Fatalf("ListIncidentsForProblem: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("incidents for problem = %d, want 2", len(rows))
	}

	// Forward lookup: each incident shows the problem.
	for _, incID := range []uuid.UUID{inc1, inc2} {
		probs, err := svc.ListProblemsForIncident(ctx, tenantA, incID)
		if err != nil {
			t.Fatalf("ListProblemsForIncident(%s): %v", incID, err)
		}
		if len(probs) != 1 || probs[0].ID != prob.ID {
			t.Errorf("incident %s problems=%v want [%s]", incID, probs, prob.ID)
		}
	}

	// Unlink one — the other stays.
	if err := svc.UnlinkIncident(ctx, tenantA, inc1, prob.ID); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	rows, _ = svc.ListIncidentsForProblem(ctx, tenantA, prob.ID)
	if len(rows) != 1 || rows[0].ID != inc2 {
		t.Errorf("after unlink incidents=%v want [%s]", rows, inc2)
	}

	// Unlink does NOT delete the underlying entities.
	if got := readProblemStatus(t, pool, prob.ID); got != "open" {
		t.Errorf("unlink mutated problem status: %q", got)
	}
	var incTitle string
	if err := pool.QueryRow(ctx, `SELECT title FROM incidents WHERE id = $1`, inc1).Scan(&incTitle); err != nil {
		t.Errorf("incident deleted by unlink: %v", err)
	}
}

func TestProblems_Linkage_TenantIsolation(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	probA, err := svc.Create(ctx, problem.CreateParams{TenantID: tenantA, Title: "A's problem", Severity: "low"})
	if err != nil {
		t.Fatalf("create A's problem: %v", err)
	}
	incB := insertIncidentForProblem(t, pool, tenantB)

	// Tenant A tries to link tenant B's incident to A's problem — must fail.
	if err := svc.LinkIncident(ctx, tenantA, incB, probA.ID, uuid.Nil); err != problem.ErrNotFound {
		t.Errorf("cross-tenant link: want ErrNotFound, got %v", err)
	}

	// Confirm no link row landed.
	var n int
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM incident_problem_links WHERE problem_id=$1`, probA.ID,
	).Scan(&n)
	if n != 0 {
		t.Errorf("cross-tenant link leaked: %d rows", n)
	}
}

func TestProblems_TimelineMixesSystemAndHumanComments(t *testing.T) {
	pool := newProblemsTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForProblems(t, pool)
	q := dbgen.New(pool)
	svc := problem.NewService(q, pool)

	prob, err := svc.Create(ctx, problem.CreateParams{TenantID: tenantA, Title: "x", Severity: "low"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.StartInvestigation(ctx, tenantA, prob.ID, uuid.Nil, ""); err != nil {
		t.Fatalf("investigate: %v", err)
	}
	if _, err := svc.AddComment(ctx, tenantA, prob.ID, uuid.Nil, "human", "I've got a hunch about the cache TTL"); err != nil {
		t.Fatalf("add human comment: %v", err)
	}

	rows, err := svc.ListComments(ctx, tenantA, prob.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("timeline size=%d want 2", len(rows))
	}
	if rows[0].Kind != "system" || rows[1].Kind != "human" {
		t.Errorf("kinds=%v want [system, human]", []string{rows[0].Kind, rows[1].Kind})
	}
}
