package workflows

import (
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestCompareAdapterConfig covers the JSONB-normalized comparison for
// adapter config. Whitespace and key-order differences must NOT count as
// divergence; value differences must.
func TestCompareAdapterConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		plaintext []byte
		decrypted []byte
		wantEqual bool
	}{
		{
			name:      "byte-identical",
			plaintext: []byte(`{"api_token":"abc","host":"h"}`),
			decrypted: []byte(`{"api_token":"abc","host":"h"}`),
			wantEqual: true,
		},
		{
			name:      "key order shuffled",
			plaintext: []byte(`{"host":"h","api_token":"abc"}`),
			decrypted: []byte(`{"api_token":"abc","host":"h"}`),
			wantEqual: true,
		},
		{
			name:      "whitespace differs",
			plaintext: []byte(`{ "api_token" : "abc" , "host": "h" }`),
			decrypted: []byte(`{"api_token":"abc","host":"h"}`),
			wantEqual: true,
		},
		{
			name:      "value differs",
			plaintext: []byte(`{"api_token":"abc"}`),
			decrypted: []byte(`{"api_token":"xyz"}`),
			wantEqual: false,
		},
		{
			name:      "missing key",
			plaintext: []byte(`{"api_token":"abc","host":"h"}`),
			decrypted: []byte(`{"api_token":"abc"}`),
			wantEqual: false,
		},
		{
			name:      "empty objects match",
			plaintext: []byte(`{}`),
			decrypted: []byte(`{}`),
			wantEqual: true,
		},
		{
			name:      "non-json falls back to byte equal — equal",
			plaintext: []byte(`not-json`),
			decrypted: []byte(`not-json`),
			wantEqual: true,
		},
		{
			name:      "non-json falls back to byte equal — diff",
			plaintext: []byte(`not-json-a`),
			decrypted: []byte(`not-json-b`),
			wantEqual: false,
		},
		{
			name:      "nested objects semantic equal",
			plaintext: []byte(`{"a":{"x":1,"y":2}}`),
			decrypted: []byte(`{"a":{"y":2,"x":1}}`),
			wantEqual: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CompareAdapterConfig(tc.plaintext, tc.decrypted)
			if got != tc.wantEqual {
				t.Fatalf("CompareAdapterConfig(%q, %q) = %v, want %v",
					tc.plaintext, tc.decrypted, got, tc.wantEqual)
			}
		})
	}
}

// TestCompareWebhookSecret covers byte-equal comparison for webhook
// secrets. No normalization: a single trailing newline is a real mismatch.
func TestCompareWebhookSecret(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		plaintext string
		decrypted string
		wantEqual bool
	}{
		{"identical", "hmac-secret", "hmac-secret", true},
		{"both empty", "", "", true},
		{"trailing newline mismatch", "secret", "secret\n", false},
		{"case matters", "Secret", "secret", false},
		{"one side empty", "secret", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CompareWebhookSecret(tc.plaintext, tc.decrypted)
			if got != tc.wantEqual {
				t.Fatalf("CompareWebhookSecret(%q, %q) = %v, want %v",
					tc.plaintext, tc.decrypted, got, tc.wantEqual)
			}
		})
	}
}

// TestDivergenceCounter_IncrementsOnMismatch verifies that the counter
// matches the compare-result policy: matching pair => no increment,
// mismatching pair => increment. One-sided (ciphertext-NULL) rows never
// reach this code path in production (filtered by the SQL WHERE clause),
// so we simulate "skip" by not calling the increment at all and asserting
// the counter stays flat.
func TestDivergenceCounter_IncrementsOnMismatch(t *testing.T) {
	// Cannot run in parallel: asserts on a process-wide Prometheus counter.

	table := telemetry.IntegrationTableAdapters
	before := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table))

	// Case 1: matching pair — counter must NOT advance.
	if !CompareAdapterConfig(
		[]byte(`{"a":1,"b":2}`),
		[]byte(`{"b":2,"a":1}`),
	) {
		telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table).Inc()
	}
	afterMatch := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table))
	if afterMatch != before {
		t.Fatalf("counter advanced on matching pair: before=%v after=%v", before, afterMatch)
	}

	// Case 2: mismatching pair — counter MUST advance by exactly 1.
	if !CompareAdapterConfig(
		[]byte(`{"a":1}`),
		[]byte(`{"a":2}`),
	) {
		telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table).Inc()
	}
	afterMismatch := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table))
	if afterMismatch != afterMatch+1 {
		t.Fatalf("counter did not advance by 1 on mismatch: after_match=%v after_mismatch=%v",
			afterMatch, afterMismatch)
	}

	// Case 3: one-sided row (ciphertext NULL) — the SQL sampler filters
	// these out, so the compare function is never called. Simulate that by
	// not invoking the counter and asserting it stays flat.
	// (No-op here; assertion is the counter hasn't moved since case 2.)
	afterSkip := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table))
	if afterSkip != afterMismatch {
		t.Fatalf("counter unexpectedly advanced during skip case: %v -> %v",
			afterMismatch, afterSkip)
	}
}

// TestDivergenceCounter_WebhookMismatch mirrors the adapter test for
// webhook rows to confirm per-table labeling works independently.
func TestDivergenceCounter_WebhookMismatch(t *testing.T) {
	table := telemetry.IntegrationTableWebhooks
	before := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table))

	// Matching secret — no increment.
	if !CompareWebhookSecret("signing-secret", "signing-secret") {
		telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table).Inc()
	}
	if got := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table)); got != before {
		t.Fatalf("webhook counter advanced on match: %v -> %v", before, got)
	}

	// Mismatched secret — +1.
	if !CompareWebhookSecret("old-secret", "new-secret") {
		telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table).Inc()
	}
	if got := testutil.ToFloat64(telemetry.IntegrationDualWriteDivergenceTotal.WithLabelValues(table)); got != before+1 {
		t.Fatalf("webhook counter did not advance by 1 on mismatch: before=%v got=%v", before, got)
	}
}
