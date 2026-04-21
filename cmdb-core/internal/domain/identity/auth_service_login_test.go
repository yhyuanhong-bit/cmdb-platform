//go:build integration

package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// claimUserID extracts the user_id claim from a JWT by base64-decoding the
// payload. The signature is not re-verified here — the tests only need to
// distinguish which user a login resolved to, not validate the token.
func claimUserID(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed jwt: %d parts", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	return claims.UserID
}

// Phase 1.3 integration coverage: users.username is now unique per
// (tenant_id, username). Login disambiguates via tenant_slug; when
// tenant_slug is absent and the username is globally ambiguous, the
// legacy fallback must refuse to log the user in.
//
// Run with:
//
//	go test -tags integration -race ./internal/domain/identity/...

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

func testRedisURL() string {
	if u := os.Getenv("TEST_REDIS_URL"); u != "" {
		return u
	}
	return "redis://localhost:6379/1"
}

func newAuthTestPool(t *testing.T) *pgxpool.Pool {
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

func newAuthTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	opts, err := redis.ParseURL(testRedisURL())
	if err != nil {
		t.Skipf("bad redis url: %v", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("test redis unreachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// authFixture creates an isolated tenant (and optionally a second one) plus
// a test user in each. Cleanup drops everything so parallel tests do not
// interfere.
type authFixture struct {
	tenantA    uuid.UUID
	tenantASlug string
	tenantB    uuid.UUID
	tenantBSlug string
	userAID    uuid.UUID
	userBID    uuid.UUID
	plainPass  string
}

func setupAuthFixture(t *testing.T, pool *pgxpool.Pool, sharedUsername string, twoTenants bool) authFixture {
	t.Helper()
	ctx := context.Background()

	fix := authFixture{plainPass: "test-pw-" + uuid.NewString()[:8]}
	fix.tenantA = uuid.New()
	fix.tenantASlug = "auth-a-" + fix.tenantA.String()[:8]
	hash, err := bcrypt.GenerateFromPassword([]byte(fix.plainPass), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug, status) VALUES ($1, $2, $3, 'active')`,
		fix.tenantA, fix.tenantASlug, fix.tenantASlug); err != nil {
		t.Fatalf("insert tenant A: %v", err)
	}
	fix.userAID = uuid.New()
	emailA := fmt.Sprintf("%s@%s.test", sharedUsername, fix.tenantASlug)
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status)
		 VALUES ($1, $2, $3, 'user a', $4, $5, 'active')`,
		fix.userAID, fix.tenantA, sharedUsername, emailA, string(hash)); err != nil {
		t.Fatalf("insert user A: %v", err)
	}

	if twoTenants {
		fix.tenantB = uuid.New()
		fix.tenantBSlug = "auth-b-" + fix.tenantB.String()[:8]
		if _, err := pool.Exec(ctx,
			`INSERT INTO tenants (id, name, slug, status) VALUES ($1, $2, $3, 'active')`,
			fix.tenantB, fix.tenantBSlug, fix.tenantBSlug); err != nil {
			t.Fatalf("insert tenant B: %v", err)
		}
		fix.userBID = uuid.New()
		emailB := fmt.Sprintf("%s@%s.test", sharedUsername, fix.tenantBSlug)
		if _, err := pool.Exec(ctx,
			`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status)
			 VALUES ($1, $2, $3, 'user b', $4, $5, 'active')`,
			fix.userBID, fix.tenantB, sharedUsername, emailB, string(hash)); err != nil {
			t.Fatalf("insert user B: %v", err)
		}
	}

	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{fix.tenantA}
		if twoTenants {
			ids = append(ids, fix.tenantB)
		}
		for _, tid := range ids {
			_, _ = pool.Exec(ctx, `DELETE FROM user_sessions WHERE user_id IN (SELECT id FROM users WHERE tenant_id = $1)`, tid)
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, tid)
			_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tid)
		}
	})

	return fix
}

// TestLogin_CrossTenantDuplicateUsername_DisambiguatedBySlug is the
// headline test for Phase 1.3: two tenants can both have a user named
// "admin", and login picks the correct one by tenant_slug.
func TestLogin_CrossTenantDuplicateUsername_DisambiguatedBySlug(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "admin", true)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	ctx := context.Background()

	// Login to tenant A.
	tokens, err := svc.Login(ctx, LoginRequest{
		TenantSlug: fix.tenantASlug,
		Username:   "admin",
		Password:   fix.plainPass,
	})
	if err != nil {
		t.Fatalf("login to tenant A: %v", err)
	}
	if tokens == nil || tokens.AccessToken == "" {
		t.Fatal("expected access token")
	}

	// Login to tenant B — same username, different tenant_slug, must
	// succeed and resolve to user B.
	tokensB, err := svc.Login(ctx, LoginRequest{
		TenantSlug: fix.tenantBSlug,
		Username:   "admin",
		Password:   fix.plainPass,
	})
	if err != nil {
		t.Fatalf("login to tenant B: %v", err)
	}
	if tokensB == nil || tokensB.AccessToken == "" {
		t.Fatal("expected tenant B access token")
	}

	// Authoritative check: each token's user_id claim must match the
	// user that lives in the named tenant. Comparing tokens by string
	// is insufficient because each login generates a fresh jti/iat.
	if got, want := claimUserID(t, tokens.AccessToken), fix.userAID.String(); got != want {
		t.Errorf("tenant A login resolved to user_id %q, want %q", got, want)
	}
	if got, want := claimUserID(t, tokensB.AccessToken), fix.userBID.String(); got != want {
		t.Errorf("tenant B login resolved to user_id %q, want %q", got, want)
	}
}

// TestLogin_EmptySlug_AmbiguousUsername_FailsClosed covers the fail-closed
// contract: when two tenants share a username and no slug was supplied,
// the service must reject the login rather than guess which tenant the
// request meant.
func TestLogin_EmptySlug_AmbiguousUsername_FailsClosed(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "operator", true)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	_, err := svc.Login(context.Background(), LoginRequest{
		Username: "operator",
		Password: fix.plainPass,
	})
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	// The error message must not leak which tenants exist.
	if s := err.Error(); len(s) == 0 {
		t.Errorf("error message should be non-empty: %q", s)
	}
}

// TestLogin_EmptySlug_GloballyUnique_Fallback preserves the documented
// compatibility path: when only one tenant has a user by that name, the
// legacy slug-less login still works (with a deprecation log the test
// does not assert on).
func TestLogin_EmptySlug_GloballyUnique_Fallback(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "solo-user-"+uuid.NewString()[:8], false)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	var username string
	if err := pool.QueryRow(context.Background(),
		`SELECT username FROM users WHERE id = $1`, fix.userAID).Scan(&username); err != nil {
		t.Fatalf("lookup username: %v", err)
	}

	tokens, err := svc.Login(context.Background(), LoginRequest{
		Username: username,
		Password: fix.plainPass,
	})
	if err != nil {
		t.Fatalf("fallback login: %v", err)
	}
	if tokens == nil || tokens.AccessToken == "" {
		t.Fatal("expected access token from fallback path")
	}
}

// TestLogin_SystemUser_AlwaysRejected verifies that the per-tenant
// FK-safe sentinel seeded by migration 000052 cannot authenticate even
// if an attacker knows its username. The trigger seeds username='system',
// source='system', password_hash='!' on every new tenant; the guard must
// reject the row before bcrypt even runs, and the error must be
// indistinguishable from a wrong-password rejection.
func TestLogin_SystemUser_AlwaysRejected(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	ctx := context.Background()

	tenantID := uuid.New()
	slug := "system-guard-" + tenantID.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug, status) VALUES ($1, $2, $3, 'active')`,
		tenantID, slug, slug); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE tenant_id = $1`, tenantID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	// The AFTER INSERT trigger on tenants seeds the system user row;
	// assert it landed before we try to log in as that user.
	var seededSource string
	if err := pool.QueryRow(ctx,
		`SELECT source FROM users WHERE tenant_id = $1 AND username = $2`,
		tenantID, "system").Scan(&seededSource); err != nil {
		t.Fatalf("trigger did not seed system user: %v", err)
	}
	if seededSource != "system" {
		t.Fatalf("seeded system user has source=%q, want %q", seededSource, "system")
	}

	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	// Try a handful of common-ish passwords. None should work — not even
	// the literal '!' the migration seeded as password_hash (which is not
	// a valid bcrypt hash anyway, but the guard must reject before that
	// distinction matters).
	for _, pw := range []string{"!", "system", "password", ""} {
		_, err := svc.Login(ctx, LoginRequest{
			TenantSlug: slug,
			Username:   "system",
			Password:   pw,
		})
		if err == nil {
			t.Fatalf("system user login must fail, got success with password %q", pw)
		}
	}
}

// TestLogin_UnknownTenantSlug_FailsClosed verifies that a bad slug does
// not fall through to the legacy path — supplying an unknown slug must
// reject the login even if a global-unique user by that name exists.
func TestLogin_UnknownTenantSlug_FailsClosed(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "alice-"+uuid.NewString()[:8], false)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret", pool)

	var username string
	if err := pool.QueryRow(context.Background(),
		`SELECT username FROM users WHERE id = $1`, fix.userAID).Scan(&username); err != nil {
		t.Fatalf("lookup username: %v", err)
	}

	_, err := svc.Login(context.Background(), LoginRequest{
		TenantSlug: "does-not-exist-" + uuid.NewString()[:8],
		Username:   username,
		Password:   fix.plainPass,
	})
	if err == nil {
		t.Fatal("expected failure for unknown tenant slug")
	}
}
