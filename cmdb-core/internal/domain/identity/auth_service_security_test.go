//go:build integration

package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Phase 2 security fixes from 2026-04-28 Team F audit:
//
//   H1: Deactivate() must revoke all refresh tokens. Without this, a
//       deleted user can still mint new access tokens for up to 7 days.
//   H2: Login() must return the same generic error for inactive
//       accounts as for wrong password — otherwise the distinct
//       message leaks password validity.
//   H3: Refresh() must (a) propagate Redis Del/SRem errors instead of
//       silently allowing token replay, and (b) reject tokens for
//       deactivated/deleted users.
//
// Run with:
//   go test -tags integration -run TestAuthSecurity ./internal/domain/identity/...

// TestAuthSecurity_H2_InactiveAccount_GenericError pins H2: an inactive
// user with a CORRECT password must receive the same generic error as
// a wrong-password attempt. Before fix: "account is not active".
// After fix: "invalid username or password".
func TestAuthSecurity_H2_InactiveAccount_GenericError(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "h2-user-"+uuid.NewString()[:8], false)
	queries := dbgen.New(pool)
	svc := NewAuthService(queries, rdb, "test-secret-32bytes-min-padding---", pool)

	ctx := context.Background()

	// Mark the user inactive (status='deleted'), but keep the row so the
	// fixture cleanup still works.
	if _, err := pool.Exec(ctx,
		`UPDATE users SET status = 'deleted', deleted_at = now() WHERE id = $1`,
		fix.userAID); err != nil {
		t.Fatalf("mark inactive: %v", err)
	}

	_, err := svc.Login(ctx, LoginRequest{
		TenantSlug: fix.tenantASlug,
		Username:   "h2-user-" + fix.tenantASlug[len("auth-a-"):], // not used; correct user resolved via slug+username pair
		Password:   fix.plainPass,
	})
	// We intentionally pass a non-matching username so resolveLoginUser
	// fails first — wait, actually we want CORRECT credentials here.
	// Re-set the username to exactly what's in the DB.
	_, err = svc.Login(ctx, LoginRequest{
		TenantSlug: fix.tenantASlug,
		Username:   getUsername(t, pool, fix.userAID),
		Password:   fix.plainPass,
	})
	if err == nil {
		t.Fatal("expected login failure for inactive user, got nil")
	}
	if got := err.Error(); got != "invalid username or password" {
		t.Fatalf("CRITICAL: error message leaks password validity — got %q, want %q",
			got, "invalid username or password")
	}
}

// TestAuthSecurity_H1_Deactivate_RevokesRefreshTokens pins H1: after
// Deactivate, the user's refresh tokens must be gone from Redis. A
// subsequent Refresh with the old token must fail.
func TestAuthSecurity_H1_Deactivate_RevokesRefreshTokens(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "h1-user-"+uuid.NewString()[:8], false)
	queries := dbgen.New(pool)
	authSvc := NewAuthService(queries, rdb, "test-secret-32bytes-min-padding---", pool)
	idSvc := NewService(queries).WithRefreshRevoker(authSvc)

	ctx := context.Background()
	username := getUsername(t, pool, fix.userAID)

	// Step 1: legitimate login → captures refresh token in Redis.
	tokens, err := authSvc.Login(ctx, LoginRequest{
		TenantSlug: fix.tenantASlug,
		Username:   username,
		Password:   fix.plainPass,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tokens.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}

	// Sanity: the refresh key must be present in Redis right now.
	if exists, _ := rdb.Exists(ctx, refreshPrefix+tokens.RefreshToken).Result(); exists != 1 {
		t.Fatalf("refresh key not in redis after login")
	}

	// Step 2: deactivate the user via the identity service.
	if err := idSvc.Deactivate(ctx, fix.tenantA, fix.userAID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// Step 3: the refresh key MUST be gone. Before the fix this stayed
	// alive for the full 7-day TTL.
	if exists, _ := rdb.Exists(ctx, refreshPrefix+tokens.RefreshToken).Result(); exists != 0 {
		t.Fatalf("CRITICAL: refresh key survived deactivation — exists=%d, want 0", exists)
	}

	// Step 4: an attempt to mint new tokens with the old refresh must
	// fail. Without this the deactivated user can stay logged in.
	if _, err := authSvc.Refresh(ctx, tokens.RefreshToken); err == nil {
		t.Fatalf("CRITICAL: Refresh succeeded for deactivated user")
	}
}

// TestAuthSecurity_H3_Refresh_RejectsDeactivatedUser pins the H3
// extension: even if a refresh token somehow survives in Redis (TTL
// race, partial revoke), Refresh() must NOT issue new tokens to a user
// whose status != 'active' or deleted_at is set.
//
// We simulate by inserting a refresh key directly to Redis (bypassing
// Login), then marking the user deleted in the DB.
func TestAuthSecurity_H3_Refresh_RejectsDeactivatedUser(t *testing.T) {
	pool := newAuthTestPool(t)
	defer pool.Close()
	rdb := newAuthTestRedis(t)

	fix := setupAuthFixture(t, pool, "h3-user-"+uuid.NewString()[:8], false)
	queries := dbgen.New(pool)
	authSvc := NewAuthService(queries, rdb, "test-secret-32bytes-min-padding---", pool)

	ctx := context.Background()

	// Plant a refresh token directly (without calling Login).
	refreshTok := "test-refresh-" + uuid.NewString()
	if err := rdb.Set(ctx, refreshPrefix+refreshTok, fix.userAID.String(), 0).Err(); err != nil {
		t.Fatalf("plant refresh: %v", err)
	}
	defer rdb.Del(ctx, refreshPrefix+refreshTok)

	// Mark user deleted in DB.
	if _, err := pool.Exec(ctx,
		`UPDATE users SET status = 'deleted', deleted_at = now() WHERE id = $1`,
		fix.userAID); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}

	// Refresh MUST refuse to issue tokens.
	if _, err := authSvc.Refresh(ctx, refreshTok); err == nil {
		t.Fatalf("CRITICAL: Refresh issued tokens for deleted user")
	} else if !errors.Is(err, errRefreshUserInactive) && err.Error() != "invalid or expired refresh token" {
		// We accept either a specific sentinel or the generic message;
		// the important contract is that no tokens are issued.
		t.Logf("(observed err: %v)", err)
	}
}

// getUsername reads the username back from the DB given a user_id.
// setupAuthFixture writes a generated username, so we have to query.
func getUsername(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) string {
	t.Helper()
	var username string
	if err := pool.QueryRow(context.Background(),
		`SELECT username FROM users WHERE id = $1`, userID).Scan(&username); err != nil {
		t.Fatalf("get username: %v", err)
	}
	return username
}
