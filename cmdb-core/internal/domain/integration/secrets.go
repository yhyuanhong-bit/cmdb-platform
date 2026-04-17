package integration

import (
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
)

// DecryptConfigWithFallback returns the adapter config plaintext bytes.
// If ciphertext is non-empty it decrypts with the provided cipher; otherwise
// the plaintext fallback is returned as-is for rows written before encryption
// rollout.
func DecryptConfigWithFallback(cipher crypto.Cipher, ciphertext, plaintext []byte) ([]byte, error) {
	if len(ciphertext) > 0 {
		if cipher == nil {
			return nil, fmt.Errorf("integration: ciphertext present but cipher is nil")
		}
		return cipher.Decrypt(ciphertext)
	}
	return plaintext, nil
}

// DecryptSecretWithFallback mirrors DecryptConfigWithFallback for webhook
// HMAC signing secrets. Returns an empty string when neither ciphertext nor
// plaintext is set.
func DecryptSecretWithFallback(cipher crypto.Cipher, ciphertext []byte, plaintext string) (string, error) {
	if len(ciphertext) > 0 {
		if cipher == nil {
			return "", fmt.Errorf("integration: ciphertext present but cipher is nil")
		}
		out, err := cipher.Decrypt(ciphertext)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return plaintext, nil
}
