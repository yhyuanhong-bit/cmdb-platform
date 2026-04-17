package crypto_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
)

// keyV1Hex and keyV2Hex are fixed distinct 32-byte keys for cross-version
// decrypt tests. Reusing testKeyHex from crypto_test.go would couple these
// tests to its value; defining them locally keeps both files independently
// readable.
const (
	keyV1Hex = "1111111111111111111111111111111111111111111111111111111111111111"
	keyV2Hex = "2222222222222222222222222222222222222222222222222222222222222222"
	keyV3Hex = "3333333333333333333333333333333333333333333333333333333333333333"
)

func mustCipher(t *testing.T, hexKey string) crypto.Cipher {
	t.Helper()
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		t.Fatalf("decode %s: %v", hexKey, err)
	}
	c, err := crypto.NewAESGCMCipher(raw)
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}
	return c
}

// clearRotationEnv wipes every env var the KeyRing loader looks at so a
// single test case can set exactly the vars it cares about without leaking
// into siblings. t.Setenv handles cleanup.
func clearRotationEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CMDB_SECRET_KEY", "")
	t.Setenv("CMDB_SECRET_KEY_ACTIVE", "")
	for v := 1; v <= 8; v++ {
		t.Setenv(envVarVersionN(v), "")
	}
}

func envVarVersionN(n int) string {
	// Mirrors crypto.versionedKeyEnvPrefix. Kept small and inline so the
	// tests don't reach into unexported symbols.
	switch n {
	case 1:
		return "CMDB_SECRET_KEY_V1"
	case 2:
		return "CMDB_SECRET_KEY_V2"
	case 3:
		return "CMDB_SECRET_KEY_V3"
	case 4:
		return "CMDB_SECRET_KEY_V4"
	case 5:
		return "CMDB_SECRET_KEY_V5"
	case 6:
		return "CMDB_SECRET_KEY_V6"
	case 7:
		return "CMDB_SECRET_KEY_V7"
	case 8:
		return "CMDB_SECRET_KEY_V8"
	}
	return ""
}

func TestKeyRing_EncryptDecryptRoundtrip_PerVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		active int
		ring   map[int]crypto.Cipher
	}{
		{"v1 only", 1, map[int]crypto.Cipher{1: mustCipher(t, keyV1Hex)}},
		{"v1+v2 active v2", 2, map[int]crypto.Cipher{
			1: mustCipher(t, keyV1Hex),
			2: mustCipher(t, keyV2Hex),
		}},
		{"v1+v2+v3 active v3", 3, map[int]crypto.Cipher{
			1: mustCipher(t, keyV1Hex),
			2: mustCipher(t, keyV2Hex),
			3: mustCipher(t, keyV3Hex),
		}},
		{"non-contiguous v1+v3 active v3", 3, map[int]crypto.Cipher{
			1: mustCipher(t, keyV1Hex),
			3: mustCipher(t, keyV3Hex),
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kr, err := crypto.NewKeyRing(tc.ring, tc.active)
			if err != nil {
				t.Fatalf("NewKeyRing: %v", err)
			}
			plaintext := []byte(`{"secret":"rotated"}`)
			ct, err := kr.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			pt, err := kr.Decrypt(ct)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(pt, plaintext) {
				t.Fatalf("roundtrip mismatch: got %q want %q", pt, plaintext)
			}
		})
	}
}

func TestKeyRing_EncryptPrependsActiveVersionPrefix(t *testing.T) {
	t.Parallel()
	kr, err := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: mustCipher(t, keyV1Hex),
		2: mustCipher(t, keyV2Hex),
	}, 2)
	if err != nil {
		t.Fatalf("NewKeyRing: %v", err)
	}
	ct, err := kr.Encrypt([]byte("hi"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !bytes.HasPrefix(ct, []byte("v2:")) {
		t.Fatalf("expected v2: prefix, got %q", truncate(ct, 8))
	}
	// Flipping active to v1 must change the wire format on next encrypt.
	kr2, _ := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: mustCipher(t, keyV1Hex),
		2: mustCipher(t, keyV2Hex),
	}, 1)
	ct1, err := kr2.Encrypt([]byte("hi"))
	if err != nil {
		t.Fatalf("Encrypt v1 ring: %v", err)
	}
	if !bytes.HasPrefix(ct1, []byte("v1:")) {
		t.Fatalf("expected v1: prefix, got %q", truncate(ct1, 8))
	}
}

func TestKeyRing_DecryptUnprefixedRoutesToV1(t *testing.T) {
	t.Parallel()

	// Produce raw aes-gcm ciphertext the legacy way — no version prefix —
	// and confirm the KeyRing still decrypts it when v1 is configured.
	legacyCipher := mustCipher(t, keyV1Hex)
	plaintext := []byte("legacy-unprefixed")
	legacyCt, err := legacyCipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("legacy Encrypt: %v", err)
	}

	kr, err := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: legacyCipher,
		2: mustCipher(t, keyV2Hex),
	}, 2)
	if err != nil {
		t.Fatalf("NewKeyRing: %v", err)
	}

	got, err := kr.Decrypt(legacyCt)
	if err != nil {
		t.Fatalf("Decrypt legacy: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("legacy roundtrip: got %q want %q", got, plaintext)
	}
}

func TestKeyRing_CrossVersion_V1CiphertextDecryptsAfterV2Added(t *testing.T) {
	t.Parallel()

	// Phase 1: server has only v1 configured. It encrypts something.
	ringV1, err := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: mustCipher(t, keyV1Hex),
	}, 1)
	if err != nil {
		t.Fatalf("NewKeyRing v1: %v", err)
	}
	plaintext := []byte("written-under-v1")
	atRestV1, err := ringV1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt under v1: %v", err)
	}
	if !bytes.HasPrefix(atRestV1, []byte("v1:")) {
		t.Fatalf("expected v1: prefix from single-key ring, got %q", truncate(atRestV1, 8))
	}

	// Phase 2: operator adds v2, flips active. Old v1 ciphertext on disk
	// must still decrypt.
	ringV1V2, err := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: mustCipher(t, keyV1Hex),
		2: mustCipher(t, keyV2Hex),
	}, 2)
	if err != nil {
		t.Fatalf("NewKeyRing v1+v2: %v", err)
	}
	got, err := ringV1V2.Decrypt(atRestV1)
	if err != nil {
		t.Fatalf("Decrypt old v1 ciphertext on v1+v2 ring: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("cross-version roundtrip: got %q want %q", got, plaintext)
	}

	// New encrypts use v2.
	atRestV2, err := ringV1V2.Encrypt([]byte("written-under-v2"))
	if err != nil {
		t.Fatalf("Encrypt under v2: %v", err)
	}
	if !bytes.HasPrefix(atRestV2, []byte("v2:")) {
		t.Fatalf("expected v2: prefix, got %q", truncate(atRestV2, 8))
	}
}

func TestKeyRing_DecryptUnknownVersionFails(t *testing.T) {
	t.Parallel()
	kr, err := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: mustCipher(t, keyV1Hex),
	}, 1)
	if err != nil {
		t.Fatalf("NewKeyRing: %v", err)
	}
	// v7 is in the ciphertext but the ring only has v1.
	fake := []byte("v7:whatever")
	if _, err := kr.Decrypt(fake); err == nil {
		t.Fatal("expected error for unknown version, got nil")
	}
}

func TestKeyRing_RejectsEmptyAndBadConstructor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		ring    map[int]crypto.Cipher
		active  int
		wantErr bool
	}{
		{"empty ring", map[int]crypto.Cipher{}, 1, true},
		{"active missing", map[int]crypto.Cipher{1: mustCipher(t, keyV1Hex)}, 2, true},
		{"zero version key", map[int]crypto.Cipher{0: mustCipher(t, keyV1Hex)}, 0, true},
		{"negative version", map[int]crypto.Cipher{-1: mustCipher(t, keyV1Hex)}, -1, true},
		{"nil cipher in ring", map[int]crypto.Cipher{1: nil}, 1, true},
		{"valid", map[int]crypto.Cipher{1: mustCipher(t, keyV1Hex)}, 1, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := crypto.NewKeyRing(tc.ring, tc.active)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestKeyRingFromEnv_LegacySingleKeyStillWorks(t *testing.T) {
	// Critical back-compat: a deployment with only CMDB_SECRET_KEY set
	// (no versioned vars, no ACTIVE) must start and encrypt under v1.
	clearRotationEnv(t)
	t.Setenv("CMDB_SECRET_KEY", keyV1Hex)

	kr, err := crypto.KeyRingFromEnv()
	if err != nil {
		t.Fatalf("KeyRingFromEnv (legacy): %v", err)
	}
	if kr.ActiveVersion() != 1 {
		t.Fatalf("expected active v1 from legacy fallback, got v%d", kr.ActiveVersion())
	}

	ct, err := kr.Encrypt([]byte("legacy-deploy"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !bytes.HasPrefix(ct, []byte("v1:")) {
		t.Fatalf("expected v1: prefix, got %q", truncate(ct, 8))
	}
	pt, err := kr.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, []byte("legacy-deploy")) {
		t.Fatalf("roundtrip: got %q", pt)
	}
}

func TestKeyRingFromEnv_VersionedVarsPreferredOverLegacy(t *testing.T) {
	clearRotationEnv(t)
	// Legacy var is set, but so is V1 with a DIFFERENT key. V1 must win
	// so operators who deliberately migrated to versioned config aren't
	// silently overridden by a stale CMDB_SECRET_KEY leftover.
	t.Setenv("CMDB_SECRET_KEY", keyV2Hex)
	t.Setenv("CMDB_SECRET_KEY_V1", keyV1Hex)

	kr, err := crypto.KeyRingFromEnv()
	if err != nil {
		t.Fatalf("KeyRingFromEnv: %v", err)
	}

	// Encrypt+Decrypt roundtrip with the ring, then try to decrypt the
	// same ciphertext with an independently-built v1 cipher. If the ring
	// honoured V1 (the correct behaviour), this roundtrip succeeds.
	plaintext := []byte("versioned wins")
	ct, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	independentV1 := mustCipher(t, keyV1Hex)
	// Strip the v1: prefix before handing raw bytes to the stdalone cipher.
	if !bytes.HasPrefix(ct, []byte("v1:")) {
		t.Fatalf("expected v1: prefix, got %q", truncate(ct, 8))
	}
	pt, err := independentV1.Decrypt(ct[3:])
	if err != nil {
		t.Fatalf("v1-key decrypt of ring ciphertext: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("roundtrip: got %q", pt)
	}
}

func TestKeyRingFromEnv_ActiveOverride(t *testing.T) {
	clearRotationEnv(t)
	t.Setenv("CMDB_SECRET_KEY_V1", keyV1Hex)
	t.Setenv("CMDB_SECRET_KEY_V2", keyV2Hex)

	// Default active is highest (v2).
	kr, err := crypto.KeyRingFromEnv()
	if err != nil {
		t.Fatalf("KeyRingFromEnv default: %v", err)
	}
	if kr.ActiveVersion() != 2 {
		t.Fatalf("default active: want v2, got v%d", kr.ActiveVersion())
	}

	// Explicit override pins to v1 (e.g. a staged rollback).
	t.Setenv("CMDB_SECRET_KEY_ACTIVE", "1")
	kr, err = crypto.KeyRingFromEnv()
	if err != nil {
		t.Fatalf("KeyRingFromEnv override: %v", err)
	}
	if kr.ActiveVersion() != 1 {
		t.Fatalf("active override: want v1, got v%d", kr.ActiveVersion())
	}
}

func TestKeyRingFromEnv_ActiveMustExist(t *testing.T) {
	clearRotationEnv(t)
	t.Setenv("CMDB_SECRET_KEY_V1", keyV1Hex)
	t.Setenv("CMDB_SECRET_KEY_ACTIVE", "5")

	if _, err := crypto.KeyRingFromEnv(); err == nil {
		t.Fatal("expected error when ACTIVE points at missing version, got nil")
	}
}

func TestKeyRingFromEnv_NoKeysConfigured(t *testing.T) {
	clearRotationEnv(t)
	if _, err := crypto.KeyRingFromEnv(); err == nil {
		t.Fatal("expected error when no keys configured, got nil")
	}
}

func TestKeyRingFromEnv_InvalidHexInVersionedVar(t *testing.T) {
	clearRotationEnv(t)
	t.Setenv("CMDB_SECRET_KEY_V1", strings.Repeat("z", 64))
	if _, err := crypto.KeyRingFromEnv(); err == nil {
		t.Fatal("expected error for invalid hex in V1, got nil")
	}
}

func TestKeyRingFromEnv_ActiveNonInteger(t *testing.T) {
	clearRotationEnv(t)
	t.Setenv("CMDB_SECRET_KEY_V1", keyV1Hex)
	t.Setenv("CMDB_SECRET_KEY_ACTIVE", "latest")
	if _, err := crypto.KeyRingFromEnv(); err == nil {
		t.Fatal("expected error for non-integer ACTIVE, got nil")
	}
}

func TestKeyRing_ImplementsCipher(t *testing.T) {
	// Compile-time-ish assertion: a KeyRing must satisfy the Cipher
	// interface so call sites that hold crypto.Cipher keep working. Doing
	// this at runtime avoids needing a separate _ = (Cipher)(nil) hack.
	t.Parallel()
	var _ crypto.Cipher = (*crypto.KeyRing)(nil)
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
