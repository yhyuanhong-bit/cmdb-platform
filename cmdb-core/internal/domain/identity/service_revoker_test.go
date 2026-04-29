package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// captureRevoker is a test double for RefreshTokenRevoker that records
// every RevokeAllRefreshTokens call. It satisfies the RefreshTokenRevoker
// interface without requiring Redis.
type captureRevoker struct {
	calls []uuid.UUID
}

func (r *captureRevoker) RevokeAllRefreshTokens(_ context.Context, userID uuid.UUID) {
	r.calls = append(r.calls, userID)
}

// TestWithRefreshRevoker_WiresAndReturnsReceiver covers two contract points:
//  1. WithRefreshRevoker installs the revoker on the receiver.
//  2. It returns the same *Service pointer for fluent chaining.
//
// A regression that allocates a new Service inside the setter silently
// loses config set before the call.
func TestWithRefreshRevoker_WiresAndReturnsReceiver(t *testing.T) {
	t.Parallel()

	svc := &Service{queries: newExtendedFake()}
	rev := &captureRevoker{}

	got := svc.WithRefreshRevoker(rev)

	if got != svc {
		t.Error("WithRefreshRevoker must return the receiver — fluent chain broken")
	}
	if svc.revoker == nil {
		t.Error("WithRefreshRevoker did not install the revoker field")
	}
}

// TestWithRefreshRevoker_NilClears documents the nil-is-valid contract:
// passing nil removes any previously installed revoker.
func TestWithRefreshRevoker_NilClears(t *testing.T) {
	t.Parallel()

	svc := &Service{queries: newExtendedFake()}
	// Install, then explicitly unset — must not panic.
	svc.WithRefreshRevoker(&captureRevoker{}).WithRefreshRevoker(nil)

	if svc.revoker != nil {
		t.Error("WithRefreshRevoker(nil) should clear the revoker")
	}
}

// TestDeactivate_WithRevoker_CallsRevoker is the unit-level regression
// guard for audit finding H1 (2026-04-28): after Deactivate, the service
// must call RevokeAllRefreshTokens so outstanding refresh tokens are
// invalidated immediately rather than surviving for their full TTL.
func TestDeactivate_WithRevoker_CallsRevoker(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	q := newExtendedFake()
	rev := &captureRevoker{}
	svc := &Service{queries: q, revoker: rev}

	if err := svc.Deactivate(context.Background(), tenantID, userID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if len(rev.calls) != 1 {
		t.Fatalf("expected 1 RevokeAllRefreshTokens call, got %d", len(rev.calls))
	}
	if rev.calls[0] != userID {
		t.Errorf("RevokeAllRefreshTokens called with userID=%s, want %s", rev.calls[0], userID)
	}
}

// TestDeactivate_NoRevoker_IsNoop: when no revoker is wired in, Deactivate
// silently skips the revocation step (nil-guard). A regression that
// dereferences the nil interface causes a panic on every deactivation.
func TestDeactivate_NoRevoker_IsNoop(t *testing.T) {
	t.Parallel()

	q := newExtendedFake()
	svc := &Service{queries: q} // revoker intentionally nil

	// Reaching this line without panic is the assertion.
	if err := svc.Deactivate(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("Deactivate without revoker: %v", err)
	}
}

// TestDeactivate_WithRevoker_DBErrorSkipsRevoke: a failing DeactivateUser
// query must NOT invoke the revoker — the user was not actually deactivated,
// so revoking tokens would lock out a still-active account.
func TestDeactivate_WithRevoker_DBErrorSkipsRevoke(t *testing.T) {
	t.Parallel()

	q := newExtendedFake()
	q.deactivateUserErr = errForTest("db unavailable")
	rev := &captureRevoker{}
	svc := &Service{queries: q, revoker: rev}

	err := svc.Deactivate(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error when DeactivateUser fails")
	}
	if len(rev.calls) != 0 {
		t.Errorf("revoker must NOT be called when deactivation fails; got %d calls", len(rev.calls))
	}
}
