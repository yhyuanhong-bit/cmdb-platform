// Package crypto provides authenticated at-rest encryption for secrets stored
// by cmdb-core (adapter configs, webhook secrets, and similar credential
// material).
//
// Algorithm: AES-256-GCM. A fresh random 12-byte nonce is generated per
// encryption and prepended to the ciphertext on the wire:
//
//	output = nonce(12) || aead.Seal(plaintext)   // Seal appends a 16-byte tag
//
// Key management: keys are 32 raw bytes, supplied to the process as a
// hex-encoded string (64 hex chars) via an environment variable. This matches
// the convention used by the sibling ingestion-engine service so the same key
// material (or a rotated successor) can be provisioned via shared secret
// infrastructure. The key env var name is chosen by the caller
// (for example CMDB_ENCRYPTION_KEY).
//
// Key rotation: generate a fresh key with GenerateKeyHex, provision it
// alongside the current key, re-encrypt stored ciphertexts with the new key,
// then retire the old key. This package intentionally exposes a single-key
// Cipher; multi-key rotation should be layered above it by callers that hold
// both the retiring and incoming ciphers.
//
// This package never logs; it is secrets-adjacent and must not leak key or
// plaintext material through telemetry. It also has no plaintext-fallback
// mode: misconfiguration fails closed.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// KeySize is the required symmetric key length in bytes (AES-256).
const KeySize = 32

// NonceSize is the GCM nonce length in bytes. AES-GCM uses a 96-bit nonce.
const NonceSize = 12

// Cipher encrypts and decrypts byte slices with authenticated encryption.
// Implementations must be safe for concurrent use.
type Cipher interface {
	// Encrypt returns nonce || sealed_ciphertext_with_tag. A fresh random
	// nonce is generated on every call; callers must not reuse the output
	// across keys.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt expects the format produced by Encrypt: a 12-byte nonce
	// followed by GCM output. It verifies the authentication tag and
	// returns an error on any tampering, truncation, or wrong-key use.
	Decrypt(ciphertext []byte) ([]byte, error)
}

// aesGCMCipher is the stdlib AES-256-GCM Cipher implementation.
//
// cipher.AEAD returned by cipher.NewGCM is safe for concurrent use, so a
// single aesGCMCipher value can back many goroutines.
type aesGCMCipher struct {
	aead cipher.AEAD
}

// NewAESGCMCipher constructs a Cipher from a 32-byte AES-256 key. It returns
// an error if the key length is not exactly KeySize. The key bytes are not
// copied; callers should treat the passed slice as owned by the cipher and
// avoid mutating it.
func NewAESGCMCipher(key []byte) (Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("crypto: invalid key length %d, want %d (AES-256)", len(key), KeySize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		// aes.NewCipher only fails on invalid key size, which we already
		// rejected above. Forward the error defensively.
		return nil, fmt.Errorf("crypto: aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: cipher.NewGCM: %w", err)
	}

	return &aesGCMCipher{aead: aead}, nil
}

// Encrypt implements Cipher.
func (c *aesGCMCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}

	// Pre-allocate output: nonce || sealed(plaintext).
	// Seal appends to its first argument; we pass the nonce slice so the
	// final layout is exactly nonce || ciphertext_with_tag.
	out := make([]byte, NonceSize, NonceSize+len(plaintext)+c.aead.Overhead())
	copy(out, nonce)
	return c.aead.Seal(out, nonce, plaintext, nil), nil
}

// ErrCiphertextTooShort is returned when the input to Decrypt is smaller than
// the minimum envelope (nonce + GCM tag).
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")

// Decrypt implements Cipher.
func (c *aesGCMCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	minLen := NonceSize + c.aead.Overhead()
	if len(ciphertext) < minLen {
		return nil, fmt.Errorf("%w: got %d bytes, need at least %d", ErrCiphertextTooShort, len(ciphertext), minLen)
	}

	nonce := ciphertext[:NonceSize]
	sealed := ciphertext[NonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		// Do not wrap the raw AEAD error verbatim into user-facing logs
		// elsewhere — it may vary by stdlib version and is not useful
		// for callers beyond "this ciphertext is not valid under this key."
		return nil, fmt.Errorf("crypto: decrypt failed: %w", err)
	}
	return plaintext, nil
}

// CipherFromEnv reads a hex-encoded 32-byte key from the named environment
// variable and returns a Cipher. It returns an error if the variable is unset
// or empty, if the value is not valid hex, or if the decoded key is not
// exactly KeySize bytes.
func CipherFromEnv(envVar string) (Cipher, error) {
	raw, ok := os.LookupEnv(envVar)
	if !ok || raw == "" {
		return nil, fmt.Errorf("crypto: environment variable %q is not set", envVar)
	}

	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("crypto: environment variable %q is not valid hex: %w", envVar, err)
	}

	if len(key) != KeySize {
		return nil, fmt.Errorf("crypto: environment variable %q decodes to %d bytes, want %d", envVar, len(key), KeySize)
	}

	return NewAESGCMCipher(key)
}

// GenerateKeyHex returns a fresh hex-encoded 32-byte key suitable for
// NewAESGCMCipher or for populating the key environment variable read by
// CipherFromEnv. It reads from crypto/rand and panics only if the OS entropy
// source fails, which is treated as unrecoverable.
func GenerateKeyHex() string {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		// crypto/rand.Read failures indicate a broken OS RNG; continuing
		// would produce weak keys, so fail loudly. Callers should treat
		// key generation as a startup/ops-time concern.
		panic(fmt.Sprintf("crypto: rand.Read failed: %v", err))
	}
	return hex.EncodeToString(key)
}
