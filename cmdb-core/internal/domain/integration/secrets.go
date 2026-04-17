package integration

import (
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
)

// DecryptConfigWithFallback returns the adapter config plaintext bytes.
// If ciphertext is non-empty it decrypts with the provided cipher; otherwise
// the plaintext fallback is returned as-is for rows written before encryption
// rollout.
//
// Observability: every fallback to the plaintext column and every decrypt
// failure increments telemetry.IntegrationDecryptFallbackTotal under the
// integration_adapters table. The counter is purely observational — it does
// not change return semantics.
func DecryptConfigWithFallback(cipher crypto.Cipher, ciphertext, plaintext []byte) ([]byte, error) {
	if len(ciphertext) > 0 {
		if cipher == nil {
			telemetry.IntegrationDecryptFallbackTotal.WithLabelValues(
				telemetry.IntegrationTableAdapters,
				telemetry.IntegrationFallbackReasonDecryptFailed,
			).Inc()
			return nil, fmt.Errorf("integration: ciphertext present but cipher is nil")
		}
		pt, err := cipher.Decrypt(ciphertext)
		if err != nil {
			telemetry.IntegrationDecryptFallbackTotal.WithLabelValues(
				telemetry.IntegrationTableAdapters,
				telemetry.IntegrationFallbackReasonDecryptFailed,
			).Inc()
			return nil, err
		}
		return pt, nil
	}
	telemetry.IntegrationDecryptFallbackTotal.WithLabelValues(
		telemetry.IntegrationTableAdapters,
		telemetry.IntegrationFallbackReasonCiphertextNull,
	).Inc()
	return plaintext, nil
}

// DecryptSecretWithFallback mirrors DecryptConfigWithFallback for webhook
// HMAC signing secrets. Returns an empty string when neither ciphertext nor
// plaintext is set.
//
// Observability: every fallback and every decrypt failure increments
// telemetry.IntegrationDecryptFallbackTotal under the webhook_subscriptions
// table. Observational only.
func DecryptSecretWithFallback(cipher crypto.Cipher, ciphertext []byte, plaintext string) (string, error) {
	if len(ciphertext) > 0 {
		if cipher == nil {
			telemetry.IntegrationDecryptFallbackTotal.WithLabelValues(
				telemetry.IntegrationTableWebhooks,
				telemetry.IntegrationFallbackReasonDecryptFailed,
			).Inc()
			return "", fmt.Errorf("integration: ciphertext present but cipher is nil")
		}
		out, err := cipher.Decrypt(ciphertext)
		if err != nil {
			telemetry.IntegrationDecryptFallbackTotal.WithLabelValues(
				telemetry.IntegrationTableWebhooks,
				telemetry.IntegrationFallbackReasonDecryptFailed,
			).Inc()
			return "", err
		}
		return string(out), nil
	}
	telemetry.IntegrationDecryptFallbackTotal.WithLabelValues(
		telemetry.IntegrationTableWebhooks,
		telemetry.IntegrationFallbackReasonCiphertextNull,
	).Inc()
	return plaintext, nil
}
