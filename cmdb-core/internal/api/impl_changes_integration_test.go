//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/change"
	"github.com/cmdb-platform/cmdb-core/internal/domain/problem"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 5.3 coverage. Three classes of behaviour worth pinning down:
//
//   1. Lifecycle correctness for each transition, including the
//      illegal-transition guard (zero rows surfaced as
//      ErrInvalidStateTransition, no ghost comments written).
//   2. CAB approval auto-resolution: standard/emergency types skip CAB
//      and go submitted→approved on submit; normal types tally votes
//      and auto-approve at threshold or auto-reject on a single reject;
//      changing your mind overwrites the vote rather than duplicating.
//   3. M:N linkage with assets / services / problems is tenant-isolated
//      and idempotent.

func newChangesTestPool(t *testing.T) *pgxpool.Pool {
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

func seedTwoTenantsForChanges(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "chg-A-"+suffix, "chg-a-"+suffix,
		b, "chg-B-"+suffix, "chg-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM change_comments  WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM change_problems  WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM change_services  WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM change_assets    WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM change_approvals WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM problems          WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM changes           WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM assets            WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants           WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

func seedUserForChanges(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, label string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source)
		 VALUES ($1, $2, $3, $4, $5, '!', 'active', 'test')`,
		id, tenantID, label+"-"+id.String()[:8], label, label+"@example.com",
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedAssetForChanges(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	tag := "CHG-" + id.String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status)
		 VALUES ($1, $2, $3, $3, 'server', 'rack_mount', 'operational')`,
		id, tenantID, tag,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

func readChangeStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM changes WHERE id=$1`, id,
	).Scan(&s); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return s
}

func countChangeComments(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM change_comments WHERE change_id=$1`, id,
	).Scan(&n); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestChanges_StandardTypeAutoApprovesOnSubmit(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "patch openssl", Type: "standard", Risk: "low",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ApprovalThreshold != 0 {
		t.Errorf("standard threshold = %d, want 0", created.ApprovalThreshold)
	}
	if created.Status != "draft" {
		t.Errorf("just-created status=%q want draft", created.Status)
	}

	submitted, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != "approved" {
		t.Errorf("standard after submit status=%q want approved", submitted.Status)
	}
	// Two system comments: "submitted" + "auto-approved (standard change)".
	if n := countChangeComments(t, pool, created.ID); n != 2 {
		t.Errorf("comments=%d want 2 (submit + auto-approve)", n)
	}
}

func TestChanges_NormalTypeWaitsForCABApproval(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	voter1 := seedUserForChanges(t, pool, tenantA, "voter1")
	voter2 := seedUserForChanges(t, pool, tenantA, "voter2")

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "schema migration", Type: "normal", Risk: "high",
		ApprovalThreshold: 2, // require 2 approve votes
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	submitted, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != "submitted" {
		t.Fatalf("normal after submit status=%q want submitted", submitted.Status)
	}

	// First approve — still in submitted (need 2).
	after1, err := svc.CastVote(ctx, tenantA, created.ID, voter1, "approve", "looks safe")
	if err != nil {
		t.Fatalf("vote1: %v", err)
	}
	if after1.Status != "submitted" {
		t.Errorf("after vote1 status=%q want submitted (threshold not met)", after1.Status)
	}

	// Second approve — threshold met, auto-approve.
	after2, err := svc.CastVote(ctx, tenantA, created.ID, voter2, "approve", "+1")
	if err != nil {
		t.Fatalf("vote2: %v", err)
	}
	if after2.Status != "approved" {
		t.Errorf("after vote2 status=%q want approved", after2.Status)
	}
}

func TestChanges_SingleRejectAutoRejects(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	voter1 := seedUserForChanges(t, pool, tenantA, "approver")
	voter2 := seedUserForChanges(t, pool, tenantA, "rejecter")

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "risky migration", Type: "normal", Risk: "critical",
		ApprovalThreshold: 3, // wouldn't normally auto-approve
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := svc.CastVote(ctx, tenantA, created.ID, voter1, "approve", ""); err != nil {
		t.Fatalf("approve vote: %v", err)
	}
	// One reject: should immediately auto-reject the change.
	final, err := svc.CastVote(ctx, tenantA, created.ID, voter2, "reject", "DB load too high during business hours")
	if err != nil {
		t.Fatalf("reject vote: %v", err)
	}
	if final.Status != "rejected" {
		t.Errorf("after reject status=%q want rejected", final.Status)
	}
}

func TestChanges_VoteOverwritePreservesOneRowPerVoter(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	voter := seedUserForChanges(t, pool, tenantA, "flipper")

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "x", Type: "normal", Risk: "low", ApprovalThreshold: 1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// First vote: approve. Threshold is 1 so this would normally auto-approve.
	if _, err := svc.CastVote(ctx, tenantA, created.ID, voter, "approve", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// The change auto-approved on the first approve, so further votes
	// hit the "must be in submitted" guard. That's the right behaviour:
	// once a decision is locked, voters can't undo it through the vote
	// mechanism. We assert that here so any future relaxation of the
	// rule has to consciously update this test.
	if _, err := svc.CastVote(ctx, tenantA, created.ID, voter, "reject", "changed my mind"); err != change.ErrInvalidStateTransition {
		t.Errorf("flip vote on approved change: want ErrInvalidStateTransition, got %v", err)
	}

	// Confirm one row per voter still — even though we tried to change
	// the vote after auto-approval, the upsert wasn't reached because
	// of the pre-check.
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM change_approvals WHERE change_id=$1 AND voter_id=$2`, created.ID, voter).Scan(&n)
	if n != 1 {
		t.Errorf("approvals rows for voter=%d, want 1", n)
	}
}

func TestChanges_VoteOverwriteWhileStillSubmitted(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	voter := seedUserForChanges(t, pool, tenantA, "flipper")

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "x", Type: "normal", Risk: "low",
		ApprovalThreshold: 5, // way above what we'll cast — change stays submitted
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Vote abstain — not enough to resolve.
	if _, err := svc.CastVote(ctx, tenantA, created.ID, voter, "abstain", "thinking"); err != nil {
		t.Fatalf("abstain: %v", err)
	}
	// Same voter switches to approve — the UNIQUE on (change_id, voter_id)
	// means upsert overwrites; we should still have one row per voter.
	if _, err := svc.CastVote(ctx, tenantA, created.ID, voter, "approve", "convinced"); err != nil {
		t.Fatalf("flip to approve: %v", err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM change_approvals WHERE change_id=$1`, created.ID).Scan(&n)
	if n != 1 {
		t.Errorf("rows after vote-flip = %d, want 1 (upsert)", n)
	}
	var vote string
	_ = pool.QueryRow(ctx, `SELECT vote FROM change_approvals WHERE change_id=$1 AND voter_id=$2`, created.ID, voter).Scan(&vote)
	if vote != "approve" {
		t.Errorf("vote after flip=%q, want approve", vote)
	}
}

func TestChanges_FullExecutionLifecycle(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "redeploy auth-svc", Type: "standard", Risk: "low",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// approved (standard auto-approves on submit). approved → in_progress.
	if _, err := svc.Start(ctx, tenantA, created.ID, uuid.Nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := readChangeStatus(t, pool, created.ID); got != "in_progress" {
		t.Errorf("after start status=%q want in_progress", got)
	}
	// in_progress → succeeded.
	if _, err := svc.MarkSucceeded(ctx, tenantA, created.ID, uuid.Nil, "rolled out cleanly"); err != nil {
		t.Fatalf("succeeded: %v", err)
	}
	if got := readChangeStatus(t, pool, created.ID); got != "succeeded" {
		t.Errorf("after succeeded status=%q want succeeded", got)
	}
}

func TestChanges_RolledBackFromFailed(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "flaky migration", Type: "standard", Risk: "high",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, _ = svc.Submit(ctx, tenantA, created.ID, uuid.Nil)
	_, _ = svc.Start(ctx, tenantA, created.ID, uuid.Nil)
	if _, err := svc.MarkFailed(ctx, tenantA, created.ID, uuid.Nil, "constraint check at row 1.2M"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if _, err := svc.MarkRolledBack(ctx, tenantA, created.ID, uuid.Nil, "ran rollback.sql"); err != nil {
		t.Fatalf("roll back: %v", err)
	}
	if got := readChangeStatus(t, pool, created.ID); got != "rolled_back" {
		t.Errorf("after rollback status=%q want rolled_back", got)
	}
}

func TestChanges_IllegalTransitions(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "x", Type: "normal", Risk: "low",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Start from draft must fail (only valid from approved).
	if _, err := svc.Start(ctx, tenantA, created.ID, uuid.Nil); err != change.ErrInvalidStateTransition {
		t.Errorf("start from draft: want ErrInvalidStateTransition, got %v", err)
	}
	// Mark succeeded from draft must fail.
	if _, err := svc.MarkSucceeded(ctx, tenantA, created.ID, uuid.Nil, ""); err != change.ErrInvalidStateTransition {
		t.Errorf("succeeded from draft: want ErrInvalidStateTransition, got %v", err)
	}

	if got := readChangeStatus(t, pool, created.ID); got != "draft" {
		t.Errorf("status mutated by failed transitions: %q", got)
	}
	if n := countChangeComments(t, pool, created.ID); n != 0 {
		t.Errorf("failed transitions wrote %d comments, want 0", n)
	}
}

func TestChanges_TenantIsolation(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "tenant-A change", Type: "normal", Risk: "low",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Tenant B can't see it.
	if _, err := svc.Get(ctx, tenantB, created.ID); err != change.ErrNotFound {
		t.Errorf("cross-tenant Get: want ErrNotFound, got %v", err)
	}
	// Tenant B can't transition it (no row for tenant B → ErrInvalidStateTransition).
	if _, err := svc.Submit(ctx, tenantB, created.ID, uuid.Nil); err != change.ErrInvalidStateTransition {
		t.Errorf("cross-tenant submit: want ErrInvalidStateTransition, got %v", err)
	}
}

func TestChanges_AssetServiceProblemLinkage(t *testing.T) {
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	csvc := change.NewService(q, pool)
	psvc := problem.NewService(q, pool)

	chg, err := csvc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "fix auth leak", Type: "normal", Risk: "high",
	})
	if err != nil {
		t.Fatalf("create change: %v", err)
	}
	assetA := seedAssetForChanges(t, pool, tenantA)
	assetB := seedAssetForChanges(t, pool, tenantB)
	prob, err := psvc.Create(ctx, problem.CreateParams{
		TenantID: tenantA, Title: "auth memory leak", Severity: "high",
	})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}

	// Link tenant A's asset — succeeds.
	if err := csvc.LinkAsset(ctx, tenantA, chg.ID, assetA); err != nil {
		t.Fatalf("link assetA: %v", err)
	}
	// Idempotent: linking again no-ops.
	if err := csvc.LinkAsset(ctx, tenantA, chg.ID, assetA); err != nil {
		t.Fatalf("re-link assetA: %v", err)
	}
	// Cross-tenant: linking tenant B's asset must fail.
	if err := csvc.LinkAsset(ctx, tenantA, chg.ID, assetB); err != change.ErrNotFound {
		t.Errorf("cross-tenant asset link: want ErrNotFound, got %v", err)
	}
	// Link a problem.
	if err := csvc.LinkProblem(ctx, tenantA, chg.ID, prob.ID); err != nil {
		t.Fatalf("link problem: %v", err)
	}

	// Forward lookups.
	assets, _ := csvc.ListAssets(ctx, tenantA, chg.ID)
	if len(assets) != 1 || assets[0].ID != assetA {
		t.Errorf("ListAssets = %v, want [%s]", assets, assetA)
	}
	probs, _ := csvc.ListProblems(ctx, tenantA, chg.ID)
	if len(probs) != 1 || probs[0].ID != prob.ID {
		t.Errorf("ListProblems = %v, want [%s]", probs, prob.ID)
	}

	// Reverse lookup: changes-for-problem.
	chgs, _ := csvc.ListChangesForProblem(ctx, tenantA, prob.ID)
	if len(chgs) != 1 || chgs[0].ID != chg.ID {
		t.Errorf("ListChangesForProblem = %v, want [%s]", chgs, chg.ID)
	}

	// Unlink does NOT cascade-delete the entities.
	if err := csvc.UnlinkAsset(ctx, tenantA, chg.ID, assetA); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	var assetTitle string
	if err := pool.QueryRow(ctx, `SELECT name FROM assets WHERE id=$1`, assetA).Scan(&assetTitle); err != nil {
		t.Errorf("asset deleted by unlink: %v", err)
	}
}

func TestChanges_VoteOnlyWhileSubmitted(t *testing.T) {
	// Belt-and-braces test: voting on a change that's already approved
	// (because it auto-approved on submit) must return ErrInvalidStateTransition.
	pool := newChangesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForChanges(t, pool)
	q := dbgen.New(pool)
	svc := change.NewService(q, pool)
	voter := seedUserForChanges(t, pool, tenantA, "voter")

	created, err := svc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "x", Type: "standard", Risk: "low",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, tenantA, created.ID, uuid.Nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Already approved — vote rejected.
	if _, err := svc.CastVote(ctx, tenantA, created.ID, voter, "approve", ""); err != change.ErrInvalidStateTransition {
		t.Errorf("vote on approved: want ErrInvalidStateTransition, got %v", err)
	}
}
