package sync

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const (
	testKeyPrimary  = "test-primary-key-32-bytes-minimum-length-xxxxxxxx"
	testKeyPrevious = "test-previous-key-32-bytes-minimum-length-yyyyyyy"
	testKeyOther    = "unrelated-attacker-key-32-bytes-minimum-zzzzzzzzz"
)

// envForSigning returns a freshly-constructed envelope with the common
// fields populated so every signing test shares the same baseline.
func envForSigning(t *testing.T) SyncEnvelope {
	t.Helper()
	return NewEnvelope("central", "tenant-A", "assets", "asset-1", "update", 7,
		json.RawMessage(`{"status":"active"}`))
}

// TestKeyRing_Sign_Verify_RoundTrip is the happy path: a keyring signs,
// the same keyring verifies, and the envelope is accepted.
func TestKeyRing_Sign_Verify_RoundTrip(t *testing.T) {
	kr, err := buildKeyRing(testKeyPrimary, "")
	if err != nil {
		t.Fatalf("build keyring: %v", err)
	}
	env := envForSigning(t)
	kr.Sign(&env)
	if env.Signature == "" {
		t.Fatal("Sign did not populate Signature")
	}
	if env.SigKID == "" {
		t.Fatal("Sign did not populate SigKID")
	}
	if got := kr.Verify(&env); got != VerifyOK {
		t.Errorf("Verify = %v, want VerifyOK", got)
	}
}

// TestKeyRing_Verify_NilKeyRing — the rollout grace window: no keyring
// configured means Verify returns VerifyOK unconditionally.
func TestKeyRing_Verify_NilKeyRing(t *testing.T) {
	var kr *KeyRing
	env := envForSigning(t)
	if got := kr.Verify(&env); got != VerifyOK {
		t.Errorf("nil keyring Verify = %v, want VerifyOK (grace window)", got)
	}
}

// TestKeyRing_Sign_NilKeyRing — signing with no keyring is a no-op. The
// publish path MUST not panic in the grace-window configuration.
func TestKeyRing_Sign_NilKeyRing(t *testing.T) {
	var kr *KeyRing
	env := envForSigning(t)
	kr.Sign(&env)
	if env.Signature != "" {
		t.Errorf("nil Sign populated Signature = %q, want empty", env.Signature)
	}
}

// TestKeyRing_Verify_SigMissing — when a keyring IS configured, an
// unsigned envelope must be rejected with the sig_missing verdict so the
// reason label on the drop counter is specific.
func TestKeyRing_Verify_SigMissing(t *testing.T) {
	kr, _ := buildKeyRing(testKeyPrimary, "")
	env := envForSigning(t) // NOT signed
	if got := kr.Verify(&env); got != VerifySigMissing {
		t.Errorf("Verify unsigned = %v, want VerifySigMissing", got)
	}
}

// TestKeyRing_Verify_UnknownKID — envelope signed by a third party whose
// kid does not match any key in our ring must reject as sig_unknown_kid
// (not bad_signature) so operators can tell "key rotated me out" from
// "tampering attempt".
func TestKeyRing_Verify_UnknownKID(t *testing.T) {
	kr, _ := buildKeyRing(testKeyPrimary, "")
	// Sign with an attacker key ring.
	attacker, _ := buildKeyRing(testKeyOther, "")
	env := envForSigning(t)
	attacker.Sign(&env)

	if got := kr.Verify(&env); got != VerifyUnknownKID {
		t.Errorf("Verify attacker-signed = %v, want VerifyUnknownKID", got)
	}
}

// TestKeyRing_Verify_BadSignature — same kid but mangled tag bytes. This
// would happen if someone intercepted a valid envelope, modified Diff,
// and forgot to recompute HMAC (or lacked the key to do so).
func TestKeyRing_Verify_BadSignature(t *testing.T) {
	kr, _ := buildKeyRing(testKeyPrimary, "")
	env := envForSigning(t)
	kr.Sign(&env)
	// Flip one hex char of the signature.
	if len(env.Signature) == 0 {
		t.Fatal("envelope not signed")
	}
	if env.Signature[0] == '0' {
		env.Signature = "1" + env.Signature[1:]
	} else {
		env.Signature = "0" + env.Signature[1:]
	}
	if got := kr.Verify(&env); got != VerifyBadSignature {
		t.Errorf("Verify tampered = %v, want VerifyBadSignature", got)
	}
}

// TestKeyRing_Verify_TamperedDiff — the most important test: after the
// publisher signs, an on-wire attacker flips a bit in Diff WITHOUT
// recomputing the HMAC. Verify must refuse.
func TestKeyRing_Verify_TamperedDiff(t *testing.T) {
	kr, _ := buildKeyRing(testKeyPrimary, "")
	env := envForSigning(t)
	kr.Sign(&env)
	env.Diff = json.RawMessage(`{"status":"hacked"}`)
	if got := kr.Verify(&env); got != VerifyBadSignature {
		t.Errorf("Verify tampered-diff = %v, want VerifyBadSignature", got)
	}
}

// TestKeyRing_Rotation_AcceptsPreviousKey — primary is new, previous is
// old; an envelope signed with the old key during a rotation window
// still verifies. Without this, every mid-rotation message would be
// dropped until the entire fleet was cut over.
func TestKeyRing_Rotation_AcceptsPreviousKey(t *testing.T) {
	newPrimary, _ := buildKeyRing(testKeyPrimary, "")
	env := envForSigning(t)

	// Old key signed an in-flight envelope.
	oldPrimary, _ := buildKeyRing(testKeyPrevious, "")
	oldPrimary.Sign(&env)

	// After rotation the ring is {primary=new, previous=[old]}.
	rotated, err := buildKeyRing(testKeyPrimary, testKeyPrevious)
	if err != nil {
		t.Fatalf("build rotated keyring: %v", err)
	}
	_ = newPrimary

	if got := rotated.Verify(&env); got != VerifyOK {
		t.Errorf("Verify post-rotation = %v, want VerifyOK", got)
	}
}

// TestLoadKeyRingFromEnv_Unset — no env var set must return (nil, nil)
// so callers can distinguish "not configured" from "misconfigured".
func TestLoadKeyRingFromEnv_Unset(t *testing.T) {
	t.Setenv(envSyncHMACKey, "")
	t.Setenv(envSyncHMACKeyPrevious, "")
	kr, err := LoadKeyRingFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kr != nil {
		t.Errorf("expected nil keyring when env unset, got %+v", kr)
	}
}

// TestLoadKeyRingFromEnv_TooShort — a too-short primary key is a config
// mistake; fail at startup instead of booting with weak security.
func TestLoadKeyRingFromEnv_TooShort(t *testing.T) {
	t.Setenv(envSyncHMACKey, "short")
	_, err := LoadKeyRingFromEnv()
	if err == nil {
		t.Fatal("expected error for too-short key")
	}
	if !strings.Contains(err.Error(), "at least") {
		t.Errorf("error did not mention length requirement: %v", err)
	}
}

// TestLoadKeyRingFromEnv_PreviousList — comma-separated previous keys
// land in the ring in config order with their kids computed.
func TestLoadKeyRingFromEnv_PreviousList(t *testing.T) {
	t.Setenv(envSyncHMACKey, testKeyPrimary)
	t.Setenv(envSyncHMACKeyPrevious, testKeyPrevious+","+testKeyOther)
	kr, err := LoadKeyRingFromEnv()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if kr == nil {
		t.Fatal("expected non-nil keyring")
	}
	if got := len(kr.previous); got != 2 {
		t.Errorf("previous key count = %d, want 2", got)
	}
	if got := len(kr.PreviousKIDs()); got != 2 {
		t.Errorf("PreviousKIDs length = %d, want 2", got)
	}
}

// TestConfigureKeyRing_Atomic_NoRace is a smoke check: the package-level
// keyring is stored atomically so concurrent reads during test setup do
// not tear. Exercised under `-race` by the standard workflow.
func TestConfigureKeyRing_Atomic_NoRace(t *testing.T) {
	kr, _ := buildKeyRing(testKeyPrimary, "")
	ConfigureKeyRing(kr)
	t.Cleanup(func() { ConfigureKeyRing(nil) })

	if got := ActiveKeyRing(); got != kr {
		t.Errorf("ActiveKeyRing() = %p, want %p", got, kr)
	}

	// Confirm the env-var load path writes into the same slot end-to-end.
	os.Setenv(envSyncHMACKey, testKeyPrimary)
	defer os.Unsetenv(envSyncHMACKey)
	loaded, err := LoadKeyRingFromEnv()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ConfigureKeyRing(loaded)
	if got := ActiveKeyRing(); got.PrimaryKID() != loaded.PrimaryKID() {
		t.Errorf("active primary_kid = %s, want %s", got.PrimaryKID(), loaded.PrimaryKID())
	}
}

// buildKeyRing is a test helper that loads a ring from literal key
// material, using the same env-var parser the production code uses so
// validation rules (min length, kid derivation) are covered in-line.
func buildKeyRing(primary, previous string) (*KeyRing, error) {
	os.Setenv(envSyncHMACKey, primary)
	if previous == "" {
		os.Unsetenv(envSyncHMACKeyPrevious)
	} else {
		os.Setenv(envSyncHMACKeyPrevious, previous)
	}
	defer os.Unsetenv(envSyncHMACKey)
	defer os.Unsetenv(envSyncHMACKeyPrevious)
	return LoadKeyRingFromEnv()
}
