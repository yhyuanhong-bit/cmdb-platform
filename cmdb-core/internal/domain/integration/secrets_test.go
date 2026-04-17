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
