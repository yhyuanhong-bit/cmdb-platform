package integration_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
)

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const otherKeyHex = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

func newCipher(t *testing.T, hexKey string) crypto.Cipher {
	t.Helper()
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	c, err := crypto.NewAESGCMCipher(key)
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}
	return c
}

func TestDecryptConfigWithFallback(t *testing.T) {
	t.Parallel()
	cipher := newCipher(t, testKeyHex)
	wrong := newCipher(t, otherKeyHex)

	plaintext := []byte(`{"api_token":"secret"}`)
	ct, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	cases := []struct {
		name       string
		cipher     crypto.Cipher
		ciphertext []byte
		plaintext  []byte
		want       []byte
		wantErr    bool
	}{
		{"encrypted path", cipher, ct, []byte(`{"legacy":true}`), plaintext, false},
		{"plaintext fallback", cipher, nil, plaintext, plaintext, false},
		{"empty ciphertext falls back", cipher, []byte{}, plaintext, plaintext, false},
		{"wrong key fails", wrong, ct, nil, nil, true},
		{"ciphertext with nil cipher fails", nil, ct, nil, nil, true},
		{"plaintext only, nil cipher ok", nil, nil, plaintext, plaintext, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := integration.DecryptConfigWithFallback(tc.cipher, tc.ciphertext, tc.plaintext)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (out=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDecryptConfigWithFallback_KeyRing exercises the fallback helper with
// a full KeyRing, mirroring the real server setup where the cipher passed
// down from main.go is actually a KeyRing. This catches regressions where
// the helper unintentionally special-cases single-key ciphers.
func TestDecryptConfigWithFallback_KeyRing(t *testing.T) {
	t.Parallel()
	v1 := newCipher(t, testKeyHex)
	v2 := newCipher(t, otherKeyHex)

	// Ciphertext written under v1-only legacy cipher (no version prefix).
	legacyCt, err := v1.Encrypt([]byte(`{"token":"legacy"}`))
	if err != nil {
		t.Fatalf("legacy encrypt: %v", err)
	}

	// Ring that has both v1 and v2, active v2. Mirrors a post-rotation
	// server where old rows still point at v1 keys.
	ring, err := crypto.NewKeyRing(map[int]crypto.Cipher{
		1: v1,
		2: v2,
	}, 2)
	if err != nil {
		t.Fatalf("NewKeyRing: %v", err)
	}

	// Legacy unprefixed ciphertext decrypts through the ring because
	// parseVersionPrefix routes to v1.
	got, err := integration.DecryptConfigWithFallback(ring, legacyCt, nil)
	if err != nil {
		t.Fatalf("legacy ciphertext through ring: %v", err)
	}
	if string(got) != `{"token":"legacy"}` {
		t.Fatalf("legacy path: got %q", got)
	}

	// New ciphertext written under the active v2 key also decrypts.
	v2Ct, err := ring.Encrypt([]byte(`{"token":"fresh"}`))
	if err != nil {
		t.Fatalf("ring encrypt: %v", err)
	}
	if !bytes.HasPrefix(v2Ct, []byte("v2:")) {
		t.Fatalf("expected v2: prefix, got %q", v2Ct[:4])
	}
	got, err = integration.DecryptConfigWithFallback(ring, v2Ct, nil)
	if err != nil {
		t.Fatalf("v2 ciphertext through ring: %v", err)
	}
	if string(got) != `{"token":"fresh"}` {
		t.Fatalf("v2 path: got %q", got)
	}
}

func TestDecryptSecretWithFallback(t *testing.T) {
	t.Parallel()
	cipher := newCipher(t, testKeyHex)
	wrong := newCipher(t, otherKeyHex)

	plaintext := "hmac-signing-secret"
	ct, err := cipher.Encrypt([]byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	cases := []struct {
		name       string
		cipher     crypto.Cipher
		ciphertext []byte
		plaintext  string
		want       string
		wantErr    bool
	}{
		{"encrypted path", cipher, ct, "legacy", plaintext, false},
		{"plaintext fallback", cipher, nil, plaintext, plaintext, false},
		{"empty everything returns empty", cipher, nil, "", "", false},
		{"wrong key fails", wrong, ct, "", "", true},
		{"ciphertext with nil cipher fails", nil, ct, "", "", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := integration.DecryptSecretWithFallback(tc.cipher, tc.ciphertext, tc.plaintext)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (out=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
