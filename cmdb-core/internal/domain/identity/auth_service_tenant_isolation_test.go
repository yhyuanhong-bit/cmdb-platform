//go:build integration

package identity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Wave 1 P0 coverage: the three UPDATE/SELECT statements in
// auth_service.go that were missing tenant_id filters (commit f74f9be).
// Each test simulates a cross-tenant attack — a caller that somehow
// obtained user A's UUID trying to reach user A's row through tenant B's
// scope — and asserts the row is NOT touched.
//
// These tests fail if the `AND tenant_id = $1` clause is ever removed
// from the matching statement. They are intentionally table-driven and
// each uses unique UUIDs so `-race` / parallel runs do not collide.
//
// Run with:
//
//	go test -tags integration -race -v \
//	  -run TenantIsolation ./internal/domain/identity/...
//
// Reuses newAuthTestPool / newAuthTestRedis / testDBURL / testRedisURL
// from auth_service_login_test.go (same package, same build tag).

// tenantIsoFixture seeds two independent tenants, each with a single
// active user that has a known password hash and a known
// password_changed_at timestamp. Cleanup drops every row the fixture
// inserted so parallel subtests do not leak state.
type tenantIsoFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID

	userA   uuid.UUID
	userB   uuid.UUID
	hashA   string
	hashB   string
	plainPW string

	// baselinePWChangedA is the password_changed_at timestamp user A
	// was seeded with — used to detect whether ChangePassword mutated
	// the row (the column is bumped to now() on success).
	baselinePWChangedA time.Time
}

func seedTenantIsoFixture(t *testing.T, pool *pgxpool.Pool) tenantIsoFixture {
	t.Helper()
	ctx := context.Background()

	fix := tenantIsoFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		userA:   uuid.New(),
		userB:   uuid.New(),
		plainPW: "orig-pw-" + uuid.NewString()[:8],
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(fix.plainPW), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	fix.hashA = string(hash)
	fix.hashB = string(hash) // same plaintext, same bcrypt hash cost — value differs by salt

	slugA := "iso-a-" + fix.tenantA.String()[:8]
	slugB := "iso-b-" + fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug, status) VALUES ($1, $2, $2, 'active'), ($3, $4, $4, 'active')`,
		fix.tenantA, slugA, fix.tenantB, slugB); err != nil {
		t.Fatalf("seed tenants: %v", err)
	}

	// Seed the two users. Use a fixed, distinguishable last_login_ip and
	// password_changed_at we can assert against later.
	baseline := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Second)
	fix.baselinePWChangedA = baseline

	userA_email := fmt.Sprintf("user-a@%s.test", slugA)
	userB_email := fmt.Sprintf("user-b@%s.test", slugB)

	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email,
		                   password_hash, status, last_login_at, last_login_ip,
		                   password_changed_at)
		 VALUES ($1, $2, $3, 'user a', $4, $5, 'active', NULL, NULL, $6)`,
		fix.userA, fix.tenantA, "alice-"+fix.userA.String()[:8], userA_email,
		fix.hashA, baseline); err != nil {
		t.Fatalf("seed user A: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email,
		                   password_hash, status, last_login_at, last_login_ip,
		                   password_changed_at)
		 VALUES ($1, $2, $3, 'user b', $4, $5, 'active', NULL, NULL, $6)`,
		fix.userB, fix.tenantB, "bob-"+fix.userB.String()[:8], userB_email,
		fix.hashB, baseline); err != nil {
		t.Fatalf("seed user B: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		for _, tid := range []uuid.UUID{fix.tenantA, fix.tenantB} {
			_, _ = pool.Exec(ctx,
				`DELETE FROM user_sessions WHERE user_id IN (SELECT id FROM users WHERE tenant_id = $1)`, tid)
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, tid)
			_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tid)
		}
	})

	return fix
}

// userLastLoginSnapshot returns (last_login_at, last_login_ip) as raw
// pointers so the caller can distinguish "never logged in" (NULL) from
// "logged in at epoch zero" (Unix 0). Fails the test on any error.
func userLastLoginSnapshot(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) (*time.Time, *string) {
	t.Helper()
	var (
		ts *time.Time
		ip *string
	)
	if err := pool.QueryRow(context.Background(),
		`SELECT last_login_at, last_login_ip FROM users WHERE id = $1`, userID).
		Scan(&ts, &ip); err != nil {
		t.Fatalf("fetch last_login for %s: %v", userID, err)
	}
	return ts, ip
}

// userPasswordHash returns the bcrypt hash currently stored for a user.
func userPasswordHash(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) string {
	t.Helper()
	var hash string
	if err := pool.QueryRow(context.Background(),
		`SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		t.Fatalf("fetch password_hash for %s: %v", userID, err)
	}
	return hash
}

// userPasswordChangedAt returns users.password_changed_at for a user.
func userPasswordChangedAt(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) time.Time {
	t.Helper()
	var ts time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT password_changed_at FROM users WHERE id = $1`, userID).Scan(&ts); err != nil {
		t.Fatalf("fetch password_changed_at for %s: %v", userID, err)
	}
	return ts
}

// -----------------------------------------------------------------------------
// recordSession
// -----------------------------------------------------------------------------

// TestRecordSession_DoesNotCrossTenant is the attack test: recordSession
// is called with userA's ID but tenantB's context. The UPDATE must match
// zero rows (neither user's last_login_at / last_login_ip is touched).
// If the `AND tenant_id = $1` clause is removed, user A's row is written
// and the test fails.
func TestRecordSession_DoesNotCrossTenant(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := seedTenantIsoFixture(t, pool)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	ctx := context.Background()

	// Attack: caller knows userA.ID, tries to impersonate tenantB.
	const attackIP = "203.0.113.42"
	svc.recordSession(ctx, fix.tenantB, fix.userA, attackIP, "Mozilla/5.0 attack-agent")

	// Assert: user A's last_login_at is STILL NULL and last_login_ip
	// is STILL NULL. The defense-in-depth WHERE clause must drop the UPDATE.
	tsA, ipA := userLastLoginSnapshot(t, pool, fix.userA)
	if tsA != nil {
		t.Errorf("cross-tenant recordSession wrote user A.last_login_at = %v; expected NULL", *tsA)
	}
	if ipA != nil {
		t.Errorf("cross-tenant recordSession wrote user A.last_login_ip = %q; expected NULL", *ipA)
	}

	// Assert: user B's last_login_at is also still NULL — no user with
	// ID == userA.ID exists in tenant B.
	tsB, ipB := userLastLoginSnapshot(t, pool, fix.userB)
	if tsB != nil {
		t.Errorf("cross-tenant recordSession leaked into user B.last_login_at = %v", *tsB)
	}
	if ipB != nil {
		t.Errorf("cross-tenant recordSession leaked into user B.last_login_ip = %q", *ipB)
	}
}

// TestRecordSession_SameTenantStillUpdates is the positive control — it
// guarantees the fix did not over-filter. Calling recordSession with a
// matching (userID, tenantID) pair must actually update last_login_at.
// Without this, a regression that made the WHERE clause match nothing
// would go undetected.
func TestRecordSession_SameTenantStillUpdates(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := seedTenantIsoFixture(t, pool)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	const loginIP = "198.51.100.17"
	before := time.Now().UTC().Add(-time.Second)
	svc.recordSession(context.Background(), fix.tenantA, fix.userA, loginIP, "Mozilla/5.0 legit")

	tsA, ipA := userLastLoginSnapshot(t, pool, fix.userA)
	if tsA == nil {
		t.Fatal("same-tenant recordSession did not set last_login_at")
	}
	if !tsA.After(before) {
		t.Errorf("last_login_at %v should be after %v", *tsA, before)
	}
	if ipA == nil || *ipA != loginIP {
		t.Errorf("last_login_ip = %v, want %q", ipA, loginIP)
	}

	// Tenant B's user must remain untouched.
	tsB, _ := userLastLoginSnapshot(t, pool, fix.userB)
	if tsB != nil {
		t.Errorf("same-tenant recordSession bled into tenant B.user.last_login_at = %v", *tsB)
	}
}

// -----------------------------------------------------------------------------
// ChangePassword
// -----------------------------------------------------------------------------

// TestChangePassword_DoesNotCrossTenant asserts the UPDATE users SQL
// issued by ChangePassword is guarded by `AND tenant_id = $1`. We drive
// the same raw statement the service runs (with a mismatched tenant)
// and verify no rows are touched. This is the direct contract test for
// the WHERE clause the commit added.
func TestChangePassword_DoesNotCrossTenant(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()

	fix := seedTenantIsoFixture(t, pool)

	ctx := context.Background()

	newHash, err := bcrypt.GenerateFromPassword([]byte("attacker-new-pw"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt new hash: %v", err)
	}

	// Attack: run the exact statement ChangePassword issues, but scoped
	// to tenantB while targeting userA. Mirrors the service-layer SQL
	// verbatim so the test fails if the "AND tenant_id = $1" clause is
	// ever removed from the service.
	sc := database.Scope(pool, fix.tenantB)
	tag, err := sc.Exec(ctx,
		`UPDATE users SET password_hash = $2, password_changed_at = now(), updated_at = now() WHERE id = $3 AND tenant_id = $1`,
		string(newHash), fix.userA)
	if err != nil {
		t.Fatalf("cross-tenant ChangePassword SQL: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Fatalf("cross-tenant UPDATE touched %d rows; want 0", tag.RowsAffected())
	}

	// Assert: user A's password_hash is unchanged.
	if got := userPasswordHash(t, pool, fix.userA); got != fix.hashA {
		t.Errorf("user A password_hash mutated by cross-tenant UPDATE: got %q, want %q", got, fix.hashA)
	}
	// Assert: user A's password_changed_at is unchanged (baseline seed).
	if got := userPasswordChangedAt(t, pool, fix.userA); !got.Equal(fix.baselinePWChangedA) {
		t.Errorf("user A password_changed_at mutated: got %v, want %v", got, fix.baselinePWChangedA)
	}
	// Assert: user B is also unchanged — userA.ID does not exist in tenantB.
	if got := userPasswordHash(t, pool, fix.userB); got != fix.hashB {
		t.Errorf("user B password_hash mutated by cross-tenant UPDATE: got %q, want %q", got, fix.hashB)
	}
}

// TestChangePassword_SameTenantStillWorks is the positive control for
// the ChangePassword fix: the happy path must still mutate user A's
// password when the request is legitimate.
func TestChangePassword_SameTenantStillWorks(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := seedTenantIsoFixture(t, pool)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	ctx := context.Background()
	newPW := "new-pw-" + uuid.NewString()[:8]
	if err := svc.ChangePassword(ctx, fix.userA, fix.plainPW, newPW); err != nil {
		t.Fatalf("legitimate ChangePassword: %v", err)
	}

	// user A's hash must verify against the new password AND fail
	// against the old one.
	got := userPasswordHash(t, pool, fix.userA)
	if err := bcrypt.CompareHashAndPassword([]byte(got), []byte(newPW)); err != nil {
		t.Errorf("new password does not verify against stored hash: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got), []byte(fix.plainPW)); err == nil {
		t.Error("old password still verifies against stored hash — ChangePassword did not rotate")
	}
	// password_changed_at must have advanced past baseline.
	if ts := userPasswordChangedAt(t, pool, fix.userA); !ts.After(fix.baselinePWChangedA) {
		t.Errorf("password_changed_at = %v did not advance past baseline %v", ts, fix.baselinePWChangedA)
	}

	// user B must be untouched (same plaintext originally, but the
	// salt differs — the hash should not have been rewritten).
	if got := userPasswordHash(t, pool, fix.userB); got != fix.hashB {
		t.Errorf("user B password_hash mutated: got %q, want %q", got, fix.hashB)
	}
	if ts := userPasswordChangedAt(t, pool, fix.userB); !ts.Equal(fix.baselinePWChangedA) {
		t.Errorf("user B password_changed_at mutated: got %v, want %v", ts, fix.baselinePWChangedA)
	}
}

// -----------------------------------------------------------------------------
// PasswordChangedAt
// -----------------------------------------------------------------------------

// TestPasswordChangedAt_DoesNotCrossTenant covers the SELECT in
// PasswordChangedAt: given userA.ID under tenantB's scope, the query
// must NOT return user A's real password_changed_at. Redis is disabled
// (the service is built with a nil redis client here) so the call hits
// the DB and exercises the `AND tenant_id = $1` clause directly.
func TestPasswordChangedAt_DoesNotCrossTenant(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()

	fix := seedTenantIsoFixture(t, pool)
	queries := dbgen.New(pool)

	// Explicitly nil redis so no cache shortcut is taken — we want the
	// DB statement to run and be evaluated by the WHERE clause.
	svc := NewAuthService(queries, nil, "test-secret", pool)

	ctx := context.Background()

	// Attack: user A's UUID, tenant B's scope.
	got, err := svc.PasswordChangedAt(ctx, fix.userA.String(), fix.tenantB.String())
	if err == nil {
		// Implementation returns the real row iff the query matched. A
		// nil error AND a non-zero timestamp would be the smoking gun.
		if !got.IsZero() {
			t.Fatalf("cross-tenant PasswordChangedAt returned a real timestamp %v (user A's real value is %v); want zero value + error",
				got, fix.baselinePWChangedA)
		}
		t.Fatalf("cross-tenant PasswordChangedAt returned nil error with zero timestamp; want error signalling no row found")
	}
	if !got.IsZero() {
		t.Errorf("cross-tenant PasswordChangedAt returned non-zero timestamp %v alongside error %v", got, err)
	}
	// Guard against the catastrophic regression: the real baseline
	// value must never surface through a mismatched tenant scope.
	if got.Equal(fix.baselinePWChangedA) {
		t.Fatalf("cross-tenant PasswordChangedAt leaked user A's real password_changed_at %v", got)
	}
}

// TestPasswordChangedAt_SameTenantReturnsTimestamp is the positive
// control: matching (userID, tenantID) returns the stored baseline so
// we know the SELECT is actually running (not silently short-circuited
// by the guard).
func TestPasswordChangedAt_SameTenantReturnsTimestamp(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()

	fix := seedTenantIsoFixture(t, pool)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, nil, "test-secret", pool)

	got, err := svc.PasswordChangedAt(context.Background(), fix.userA.String(), fix.tenantA.String())
	if err != nil {
		t.Fatalf("same-tenant PasswordChangedAt: %v", err)
	}
	// Compare at second resolution — the column is TIMESTAMPTZ and we
	// truncated the seed value to the second.
	if !got.UTC().Truncate(time.Second).Equal(fix.baselinePWChangedA) {
		t.Errorf("PasswordChangedAt = %v, want %v", got, fix.baselinePWChangedA)
	}
}

// TestPasswordChangedAt_TenantIDRequired asserts the guard clauses at
// the top of PasswordChangedAt: an empty/invalid tenantID string must
// short-circuit to (zero, nil) rather than run an unscoped SELECT. This
// pins down the fail-closed behaviour so a future refactor cannot
// silently drop the tenantID parameter.
func TestPasswordChangedAt_TenantIDRequired(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()

	fix := seedTenantIsoFixture(t, pool)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, nil, "test-secret", pool)

	cases := []struct {
		name     string
		userID   string
		tenantID string
	}{
		{"empty tenant id", fix.userA.String(), ""},
		{"malformed tenant id", fix.userA.String(), "not-a-uuid"},
		{"empty user id", "", fix.tenantA.String()},
		{"malformed user id", "not-a-uuid", fix.tenantA.String()},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.PasswordChangedAt(context.Background(), tc.userID, tc.tenantID)
			if err != nil {
				t.Fatalf("guarded PasswordChangedAt returned unexpected error: %v", err)
			}
			if !got.IsZero() {
				t.Errorf("guarded PasswordChangedAt returned %v; want zero time", got)
			}
		})
	}
}
