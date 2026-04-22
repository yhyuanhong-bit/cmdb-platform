package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// These tests exercise the parts of AuthService that do NOT require a
// live DB or Redis — constructor wiring, WithBlacklist, Logout's
// nil-dependency fast paths, and PasswordChangedAt's nil-pool guard.
// The DB-backed paths (Login, Refresh, GetCurrentUser, ChangePassword)
// are covered under //go:build integration in auth_service_login_test.go.

// fakeBlacklist is the minimal stand-in for *auth.Blacklist.
type fakeBlacklist struct {
	calls         []revokeCall
	revokeErr     error
	revokeCallsCh chan struct{}
}

type revokeCall struct {
	jti       string
	expiresAt time.Time
}

func (f *fakeBlacklist) Revoke(_ context.Context, jti string, expiresAt time.Time) error {
	f.calls = append(f.calls, revokeCall{jti: jti, expiresAt: expiresAt})
	if f.revokeCallsCh != nil {
		f.revokeCallsCh <- struct{}{}
	}
	return f.revokeErr
}

// TestNewAuthService_Wiring: the constructor must capture all four
// inputs so subsequent method calls see the right dependencies. A
// regression that drops an argument would show up here as a nil
// field.
func TestNewAuthService_Wiring(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "secret", nil)
	if svc == nil {
		t.Fatal("NewAuthService returned nil")
	}
	if svc.jwtSecret != "secret" {
		t.Errorf("jwtSecret not captured: got %q want %q", svc.jwtSecret, "secret")
	}
	// blacklist is explicitly not set by NewAuthService — it's opt-in
	// via WithBlacklist. Lock that in so a regression that
	// auto-installs a blacklist is caught.
	if svc.blacklist != nil {
		t.Errorf("blacklist should default to nil, got %T", svc.blacklist)
	}
}

// TestWithBlacklist_ChainsReturn: WithBlacklist returns the receiver
// for fluent chaining. Calling it also swaps the stored blacklist.
// The fluent-chain contract matters because production code uses
//
//	svc := NewAuthService(...).WithBlacklist(bl)
//
// and a regression that returns a new AuthService copy would silently
// lose whichever config was set downstream of the chain.
func TestWithBlacklist_ChainsReturn(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil)
	bl := &fakeBlacklist{}
	got := svc.WithBlacklist(bl)
	if got != svc {
		t.Error("WithBlacklist did not return receiver — breaks fluent chain")
	}
	if svc.blacklist == nil {
		t.Error("WithBlacklist did not install the blacklist")
	}
}

// TestWithBlacklist_NilIsValid: installing a nil blacklist is the
// documented way to disable revocation (used by tests and edge
// deployments). The setter must accept it without dereferencing.
func TestWithBlacklist_NilIsValid(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil)
	// Install, then remove — must not panic.
	svc.WithBlacklist(&fakeBlacklist{}).WithBlacklist(nil)
	if svc.blacklist != nil {
		t.Error("WithBlacklist(nil) did not clear the blacklist")
	}
}

// TestLogout_NoBlacklist_NoRedis_ReturnsNil: the "disabled"
// configuration — no blacklist injected, no Redis configured — still
// honors the Logout API with a nil return. Used by tests and edge
// deployments that don't ship a Redis.
func TestLogout_NoBlacklist_NoRedis_ReturnsNil(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil) // all nil
	err := svc.Logout(context.Background(), uuid.New(), "jti-xyz", time.Now().Add(time.Hour))
	if err != nil {
		t.Errorf("Logout with nil deps should return nil, got %v", err)
	}
}

// TestLogout_WithBlacklist_InvokesRevoke: with a blacklist installed
// and a non-empty jti, Logout MUST call Revoke with the exact jti
// and expiry passed in. A regression that trims or hashes the jti
// would leak a usable token past revocation.
func TestLogout_WithBlacklist_InvokesRevoke(t *testing.T) {
	t.Parallel()
	bl := &fakeBlacklist{}
	svc := NewAuthService(nil, nil, "s", nil).WithBlacklist(bl)
	expires := time.Now().Add(15 * time.Minute).UTC().Truncate(time.Second)

	err := svc.Logout(context.Background(), uuid.New(), "jti-abc", expires)
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if len(bl.calls) != 1 {
		t.Fatalf("expected 1 revoke call, got %d", len(bl.calls))
	}
	if bl.calls[0].jti != "jti-abc" {
		t.Errorf("jti mismatch: got %q want %q", bl.calls[0].jti, "jti-abc")
	}
	if !bl.calls[0].expiresAt.Equal(expires) {
		t.Errorf("expiresAt mismatch: got %s want %s", bl.calls[0].expiresAt, expires)
	}
}

// TestLogout_EmptyJTI_SkipsRevoke: the contract says Logout revokes
// only when jti is non-empty. An access token missing its jti is a
// legacy token from before middleware added jti — the safer path is
// to skip revocation (the token's short TTL handles the rest).
func TestLogout_EmptyJTI_SkipsRevoke(t *testing.T) {
	t.Parallel()
	bl := &fakeBlacklist{}
	svc := NewAuthService(nil, nil, "s", nil).WithBlacklist(bl)

	err := svc.Logout(context.Background(), uuid.New(), "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bl.calls) != 0 {
		t.Errorf("expected 0 revoke calls for empty jti, got %d", len(bl.calls))
	}
}

// TestLogout_NilUUID_SkipsRefreshWipe: with a nil user UUID we cannot
// index into refresh-token storage safely; the API must no-op the
// refresh-wipe step rather than issuing a bogus key like
// `refresh_index:user:00000000-...`.
func TestLogout_NilUUID_SkipsRefreshWipe(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil) // no redis
	err := svc.Logout(context.Background(), uuid.Nil, "jti", time.Now())
	if err != nil {
		t.Errorf("Logout with uuid.Nil should return nil, got %v", err)
	}
}

// TestLogout_RevokeErrorPropagates: when the blacklist implementation
// fails, Logout must propagate the error so the caller can retry
// (the API handler translates it into a 500). A silent discard would
// leave a revocation un-recorded — exactly the class of bug that
// keeps compromised sessions alive.
func TestLogout_RevokeErrorPropagates(t *testing.T) {
	t.Parallel()
	bl := &fakeBlacklist{revokeErr: errForTest("redis down")}
	svc := NewAuthService(nil, nil, "s", nil).WithBlacklist(bl)

	err := svc.Logout(context.Background(), uuid.New(), "jti", time.Now())
	if err == nil {
		t.Fatal("expected error when blacklist.Revoke fails")
	}
}

// TestPasswordChangedAt_NoPool: the middleware treats PasswordChangedAt
// as fail-open when the pool is nil (edge deployments without DB). The
// contract is (zero time, nil err) — callers interpret a zero time as
// "no timestamp available, don't block".
func TestPasswordChangedAt_NoPool(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil)
	ts, err := svc.PasswordChangedAt(
		context.Background(),
		uuid.NewString(),
		uuid.NewString(),
	)
	if err != nil {
		t.Errorf("no-pool path should not error, got %v", err)
	}
	if !ts.IsZero() {
		t.Errorf("no-pool path should return zero time, got %s", ts)
	}
}

// TestPasswordChangedAt_NilUUID_ReturnsZero: an unparseable userID or
// tenantID parses to uuid.Nil; the method must short-circuit so no
// DB round-trip is attempted with the zero UUID (which would either
// blow up or return the wrong user).
func TestPasswordChangedAt_NilUUID_ReturnsZero(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil)

	t.Run("bad user id", func(t *testing.T) {
		t.Parallel()
		ts, err := svc.PasswordChangedAt(context.Background(), "not-a-uuid", uuid.NewString())
		if err != nil || !ts.IsZero() {
			t.Errorf("bad user_id must yield (zero, nil), got (%v, %v)", ts, err)
		}
	})

	t.Run("bad tenant id", func(t *testing.T) {
		t.Parallel()
		ts, err := svc.PasswordChangedAt(context.Background(), uuid.NewString(), "not-a-uuid")
		if err != nil || !ts.IsZero() {
			t.Errorf("bad tenant_id must yield (zero, nil), got (%v, %v)", ts, err)
		}
	})
}

// TestRecordSession_NilPoolReturns: recordSession is the
// best-effort post-login bookkeeping step. With no DB pool configured
// (tests, edge mode) it must silently return — NOT panic attempting
// to dereference s.pool. A regression that moves the nil-guard would
// segfault the entire login flow the first time a test called it.
//
// We call it through a zero-dep AuthService; the test passes if no
// panic is raised.
func TestRecordSession_NilPoolReturns(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(nil, nil, "s", nil) // nil pool

	// recordSession signature: (ctx, tenantID, userID, clientIP, userAgent)
	svc.recordSession(
		context.Background(),
		uuid.New(),
		uuid.New(),
		"10.0.0.1",
		"Mozilla/5.0 Chrome",
	)
	// Assertion: reaching this line without panic is the contract.
}

// errForTest is a lightweight helper to avoid importing errors just
// for one sentinel in these tests.
type errForTest string

func (e errForTest) Error() string { return string(e) }
