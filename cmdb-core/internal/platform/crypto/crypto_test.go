package crypto_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
)

// testKey is a deterministic 32-byte key (hex-encoded) for table-driven tests.
const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func mustKey(t *testing.T) []byte {
	t.Helper()
	key, err := hex.DecodeString(testKeyHex)
	if err != nil {
		t.Fatalf("decode test key: %v", err)
	}
	return key
}

func TestNewAESGCMCipher_RejectsShortKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"16 bytes (AES-128)", make([]byte, 16)},
		{"24 bytes (AES-192)", make([]byte, 24)},
		{"31 bytes (short by one)", make([]byte, 31)},
		{"33 bytes (long by one)", make([]byte, 33)},
		{"64 bytes (too long)", make([]byte, 64)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := crypto.NewAESGCMCipher(tc.key); err == nil {
				t.Fatalf("expected error for key length %d, got nil", len(tc.key))
			}
		})
	}
}

func TestNewAESGCMCipher_AcceptsValidKey(t *testing.T) {
	t.Parallel()

	c, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cipher")
	}
}

func TestCipher_Roundtrip(t *testing.T) {
	t.Parallel()

	c, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}

	cases := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short ASCII", []byte("hello")},
		{"json-like", []byte(`{"user":"admin","password":"s3cret"}`)},
		{"binary", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}},
		{"large 8KiB", bytes.Repeat([]byte("A"), 8192)},
		{"utf-8 multibyte", []byte("héllo wörld — 日本語")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ct, err := c.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			pt, err := c.Decrypt(ct)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(pt, tc.plaintext) {
				t.Fatalf("roundtrip mismatch: got %q, want %q", pt, tc.plaintext)
			}
		})
	}
}

func TestCipher_EncryptIsNondeterministic(t *testing.T) {
	t.Parallel()

	c, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}

	plaintext := []byte("same plaintext, different nonces")
	ct1, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	ct2, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Fatal("expected distinct ciphertexts due to random nonce, got identical")
	}

	// Both must still decrypt back to the original.
	for i, ct := range [][]byte{ct1, ct2} {
		pt, err := c.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt %d: %v", i, err)
		}
		if !bytes.Equal(pt, plaintext) {
			t.Fatalf("Decrypt %d: got %q, want %q", i, pt, plaintext)
		}
	}
}

func TestCipher_WrongKeyFailsDecrypt(t *testing.T) {
	t.Parallel()

	c1, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher c1: %v", err)
	}

	otherKey := make([]byte, 32)
	for i := range otherKey {
		otherKey[i] = 0xAA
	}
	c2, err := crypto.NewAESGCMCipher(otherKey)
	if err != nil {
		t.Fatalf("NewAESGCMCipher c2: %v", err)
	}

	ct, err := c1.Encrypt([]byte("top secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := c2.Decrypt(ct); err == nil {
		t.Fatal("expected decrypt with wrong key to fail, got nil error")
	}
}

func TestCipher_TamperedCiphertextFailsDecrypt(t *testing.T) {
	t.Parallel()

	c, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}

	ct, err := c.Encrypt([]byte("integrity matters"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	cases := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{
			name: "flip bit in nonce",
			mutate: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				out[0] ^= 0x01
				return out
			},
		},
		{
			name: "flip bit in ciphertext body",
			mutate: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				out[len(out)/2] ^= 0x01
				return out
			},
		},
		{
			name: "flip bit in auth tag (last byte)",
			mutate: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				out[len(out)-1] ^= 0x01
				return out
			},
		},
		{
			name: "truncate last byte of auth tag",
			mutate: func(b []byte) []byte {
				return append([]byte(nil), b[:len(b)-1]...)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tampered := tc.mutate(ct)
			if _, err := c.Decrypt(tampered); err == nil {
				t.Fatal("expected decrypt of tampered ciphertext to fail, got nil")
			}
		})
	}
}

func TestCipher_DecryptRejectsShortCiphertext(t *testing.T) {
	t.Parallel()

	c, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}

	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"1 byte", []byte{0x00}},
		{"11 bytes (one short of nonce)", make([]byte, 11)},
		{"12 bytes (only nonce, no tag)", make([]byte, 12)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := c.Decrypt(tc.data); err == nil {
				t.Fatalf("expected decrypt of %d-byte input to fail", len(tc.data))
			}
		})
	}
}

func TestCipherFromEnv_MissingVar(t *testing.T) {
	// Does not run in parallel because it mutates process env.
	const envVar = "CRYPTO_TEST_MISSING_KEY"
	t.Setenv(envVar, "") // t.Setenv ensures cleanup; setting to empty mimics unset for our check.

	if _, err := crypto.CipherFromEnv(envVar); err == nil {
		t.Fatal("expected error when env var is empty, got nil")
	}
}

func TestCipherFromEnv_InvalidHex(t *testing.T) {
	const envVar = "CRYPTO_TEST_INVALID_HEX"

	cases := []struct {
		name  string
		value string
	}{
		{"non-hex chars", "zzzz" + strings.Repeat("0", 60)},
		{"odd length", strings.Repeat("a", 63)},
		{"too short (valid hex)", strings.Repeat("ab", 16)},  // 16 bytes
		{"too long (valid hex)", strings.Repeat("ab", 48)},   // 48 bytes
		{"empty-ish whitespace", "   "},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envVar, tc.value)
			if _, err := crypto.CipherFromEnv(envVar); err == nil {
				t.Fatalf("expected error for value %q, got nil", tc.value)
			}
		})
	}
}

func TestCipherFromEnv_ValidKey(t *testing.T) {
	const envVar = "CRYPTO_TEST_VALID_KEY"
	t.Setenv(envVar, testKeyHex)

	c, err := crypto.CipherFromEnv(envVar)
	if err != nil {
		t.Fatalf("CipherFromEnv: %v", err)
	}

	plaintext := []byte("env-loaded key works")
	ct, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("roundtrip: got %q, want %q", pt, plaintext)
	}
}

func TestGenerateKeyHex_ProducesUsableKey(t *testing.T) {
	t.Parallel()

	hk := crypto.GenerateKeyHex()

	// Format checks: 64 hex chars.
	if len(hk) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(hk))
	}
	raw, err := hex.DecodeString(hk)
	if err != nil {
		t.Fatalf("GenerateKeyHex produced non-hex output: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("expected 32 bytes decoded, got %d", len(raw))
	}

	// Feed it back into the constructor and do a roundtrip.
	c, err := crypto.NewAESGCMCipher(raw)
	if err != nil {
		t.Fatalf("NewAESGCMCipher with generated key: %v", err)
	}
	plaintext := []byte("generated key works end-to-end")
	ct, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("roundtrip with generated key: got %q, want %q", pt, plaintext)
	}
}

func TestGenerateKeyHex_ProducesDistinctKeys(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 16)
	for i := 0; i < 16; i++ {
		k := crypto.GenerateKeyHex()
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate key from GenerateKeyHex after %d iterations: %s", i, k)
		}
		seen[k] = struct{}{}
	}
}

func TestCipher_NoncePrependedToCiphertext(t *testing.T) {
	// Sanity check of the on-disk format: first 12 bytes must be the nonce,
	// i.e. encrypting an empty plaintext yields exactly 12 (nonce) + 16 (GCM tag) = 28 bytes.
	t.Parallel()

	c, err := crypto.NewAESGCMCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}

	ct, err := c.Encrypt(nil)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	const want = 12 + 16
	if len(ct) != want {
		t.Fatalf("empty plaintext ciphertext length: got %d, want %d", len(ct), want)
	}
}
