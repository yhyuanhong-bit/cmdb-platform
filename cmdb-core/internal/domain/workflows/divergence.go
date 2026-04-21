// Package workflows: divergence.go hosts the dual-write divergence sampler
// for integration secrets (migration 000038). While the encrypted and
// plaintext columns coexist, a background job periodically samples rows
// where both are populated, decrypts the ciphertext, and compares. Any
// mismatch increments telemetry.IntegrationDualWriteDivergenceTotal and
// emits a structured error log.
//
// The job is gated behind the CMDB_INTEGRATION_DIVERGENCE_CHECK=1 env flag
// so we don't surprise-deploy a new recurring DB-touching job. Default off.
package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// divergenceCheckInterval is how often the sampler runs when enabled.
	divergenceCheckInterval = 15 * time.Minute

	// divergenceSampleSize is the per-table row cap per tick. Starts
	// conservative — the job is observational, not exhaustive.
	divergenceSampleSize = 500

	// divergenceFlagEnv enables the job. Any value other than "1" leaves
	// the sampler quiescent; default-off matches rollout policy for new
	// recurring jobs.
	divergenceFlagEnv = "CMDB_INTEGRATION_DIVERGENCE_CHECK"
)

// StartDivergenceChecker starts the periodic dual-write divergence sampler.
//
// No-op (returns immediately, starts no goroutine) unless the feature flag
// env var is explicitly "1". Logs the decision once at startup so operators
// can confirm the job's state from the boot log.
func (w *WorkflowSubscriber) StartDivergenceChecker(ctx context.Context) {
	if os.Getenv(divergenceFlagEnv) != "1" {
		zap.L().Info("integration divergence checker disabled",
			zap.String("flag_env", divergenceFlagEnv))
		return
	}
	if w.cipher == nil {
		// Without a cipher we cannot decrypt; refuse to start rather
		// than flood the counter with spurious mismatches.
		zap.L().Warn("integration divergence checker: no cipher configured, not starting")
		return
	}

	ticker := time.NewTicker(divergenceCheckInterval)
	go func() {
		// Run once immediately so the first signal arrives without
		// waiting 15 minutes post-boot.
		func() {
			tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.divergence_check")
			defer end()
			w.runDivergenceCheck(tickCtx)
		}()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				tickCtx, end := telemetry.StartTickSpan(ctx, "workflow.tick.divergence_check")
				w.runDivergenceCheck(tickCtx)
				end()
			}
		}
	}()
	zap.L().Info("integration divergence checker started",
		zap.Duration("interval", divergenceCheckInterval),
		zap.Int("sample_size", divergenceSampleSize))
}

// runDivergenceCheck samples both tables and records any mismatches. Each
// table scan is logged at debug with counts so operators can verify the
// sampler ran without spamming info-level logs.
func (w *WorkflowSubscriber) runDivergenceCheck(ctx context.Context) {
	scanned, diverged := w.checkAdapterDivergence(ctx, divergenceSampleSize)
	zap.L().Debug("integration divergence scan: adapters",
		zap.Int("scanned", scanned), zap.Int("diverged", diverged))

	scanned, diverged = w.checkWebhookDivergence(ctx, divergenceSampleSize)
	zap.L().Debug("integration divergence scan: webhooks",
		zap.Int("scanned", scanned), zap.Int("diverged", diverged))
}

// checkAdapterDivergence samples up to `limit` integration_adapters rows
// where both config and config_encrypted are populated. Returns counts for
// (scanned, diverged).
func (w *WorkflowSubscriber) checkAdapterDivergence(ctx context.Context, limit int) (int, int) {
	rows, err := w.pool.Query(ctx, `
		SELECT id, tenant_id, config, config_encrypted
		FROM integration_adapters
		WHERE config IS NOT NULL
		  AND config::text <> '{}'
		  AND config_encrypted IS NOT NULL
		ORDER BY id
		LIMIT $1
	`, limit)
	if err != nil {
		zap.L().Warn("divergence check: query adapters failed", zap.Error(err))
		return 0, 0
	}
	defer rows.Close()

	scanned, diverged := 0, 0
	for rows.Next() {
		var id, tenantID uuid.UUID
		var plaintext, ciphertext []byte
		if err := rows.Scan(&id, &tenantID, &plaintext, &ciphertext); err != nil {
			zap.L().Warn("divergence check: adapter scan error", zap.Error(err))
			continue
		}
		scanned++

		decrypted, err := integration.DecryptConfigWithFallback(w.cipher, ciphertext, nil)
		if err != nil {
			zap.L().Error("divergence check: adapter decrypt failed",
				zap.String("adapter_id", id.String()),
				zap.String("tenant_id", tenantID.String()),
				zap.Error(err))
			telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(
				telemetry.IntegrationTableAdapters).Inc()
			diverged++
			continue
		}

		if !jsonEqual(decrypted, plaintext) {
			telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(
				telemetry.IntegrationTableAdapters).Inc()
			diverged++
			// Never log secret contents — only IDs and structural info.
			zap.L().Error("integration adapter dual-write divergence",
				zap.String("adapter_id", id.String()),
				zap.String("tenant_id", tenantID.String()),
				zap.String("table", telemetry.IntegrationTableAdapters),
				zap.Int("plaintext_len", len(plaintext)),
				zap.Int("decrypted_len", len(decrypted)))
		}
	}
	if err := rows.Err(); err != nil {
		zap.L().Warn("divergence check: adapter row iteration error", zap.Error(err))
	}
	return scanned, diverged
}

// checkWebhookDivergence samples up to `limit` webhook_subscriptions rows
// where both secret and secret_encrypted are populated.
func (w *WorkflowSubscriber) checkWebhookDivergence(ctx context.Context, limit int) (int, int) {
	rows, err := w.queries.SampleWebhookSecretsForDivergence(ctx, int32(limit))
	if err != nil {
		zap.L().Warn("divergence check: query webhooks failed", zap.Error(err))
		return 0, 0
	}

	scanned, diverged := 0, 0
	for _, r := range rows {
		scanned++

		// r.Secret is pgtype.Text — the query filters on NOT NULL and
		// non-empty, but double-check before dereferencing so a future
		// query edit that relaxes the filter doesn't crash here.
		if !r.Secret.Valid {
			continue
		}
		plaintext := r.Secret.String

		decrypted, derr := integration.DecryptSecretWithFallback(w.cipher, r.SecretEncrypted, "")
		if derr != nil {
			zap.L().Error("divergence check: webhook decrypt failed",
				zap.String("webhook_id", r.ID.String()),
				zap.String("tenant_id", r.TenantID.String()),
				zap.Error(derr))
			telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(
				telemetry.IntegrationTableWebhooks).Inc()
			diverged++
			continue
		}

		if decrypted != plaintext {
			telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(
				telemetry.IntegrationTableWebhooks).Inc()
			diverged++
			zap.L().Error("integration webhook dual-write divergence",
				zap.String("webhook_id", r.ID.String()),
				zap.String("tenant_id", r.TenantID.String()),
				zap.String("table", telemetry.IntegrationTableWebhooks),
				zap.Int("plaintext_len", len(plaintext)),
				zap.Int("decrypted_len", len(decrypted)))
		}
	}
	return scanned, diverged
}

// jsonEqual compares two JSON byte slices semantically: whitespace and key
// order are ignored, values are deep-equal. On parse failure of either side
// it falls back to byte-equal so we don't over-report divergence from
// non-JSON legacy data (the adapter column is JSONB so both sides should
// parse, but defensive coding keeps false positives out of the counter).
func jsonEqual(a, b []byte) bool {
	var av, bv any
	aErr := json.Unmarshal(a, &av)
	bErr := json.Unmarshal(b, &bv)
	if aErr != nil || bErr != nil {
		return bytes.Equal(a, b)
	}
	// Re-marshal canonically. encoding/json emits map keys in sorted order
	// for map[string]any, giving us a stable comparison without pulling in
	// a deep-equal dependency.
	ac, err1 := json.Marshal(av)
	bc, err2 := json.Marshal(bv)
	if err1 != nil || err2 != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ac, bc)
}

// CompareAdapterConfig returns true if the decrypted ciphertext matches the
// plaintext config semantically. Exported for unit tests so the pure
// comparison logic can be exercised without a live DB.
func CompareAdapterConfig(plaintext, decrypted []byte) bool {
	return jsonEqual(plaintext, decrypted)
}

// CompareWebhookSecret returns true if the decrypted webhook secret matches
// the plaintext secret byte-for-byte. Thin wrapper so tests can reference
// the same equality function the runtime uses.
func CompareWebhookSecret(plaintext, decrypted string) bool {
	return plaintext == decrypted
}
