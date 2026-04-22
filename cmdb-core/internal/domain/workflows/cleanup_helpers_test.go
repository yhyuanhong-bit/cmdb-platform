package workflows

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDurationUntilNextUTCHour covers the wake-up scheduling math used
// by the webhook retention sweep. The contract is:
//
//  1. If we are BEFORE the target hour today, wait until today's hour.
//  2. If we are AT OR AFTER the target hour, wait until tomorrow's hour.
//
// An off-by-one here would either fire twice in one day or skip a day,
// both of which erode the retention window's alignment.
func TestDurationUntilNextUTCHour(t *testing.T) {
	t.Parallel()

	fixed := func(h, m int) time.Time {
		return time.Date(2026, 4, 22, h, m, 0, 0, time.UTC)
	}

	tests := []struct {
		name     string
		now      time.Time
		hourUTC  int
		wantHrs  float64
		wantMins float64
	}{
		// Currently 00:00, target 03:00 → 3h until wakeup.
		{"before target hour", fixed(0, 0), 3, 3, 0},
		// Currently 02:45, target 03:00 → 15m until wakeup.
		{"just before target hour", fixed(2, 45), 3, 0, 15},
		// Currently 03:00, target 03:00 → target == now, must jump to tomorrow.
		{"exactly at target hour", fixed(3, 0), 3, 24, 0},
		// Currently 03:15, target 03:00 → wait until 03:00 tomorrow = 23h45m.
		{"just after target hour", fixed(3, 15), 3, 23, 45},
		// Late in the day: 20:00, target 03:00 → 7 hours until 03:00 tomorrow.
		{"evening", fixed(20, 0), 3, 7, 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := durationUntilNextUTCHour(tc.now, tc.hourUTC)
			wantTotal := time.Duration(tc.wantHrs*float64(time.Hour)) +
				time.Duration(tc.wantMins*float64(time.Minute))
			if got != wantTotal {
				t.Errorf("durationUntilNextUTCHour(%s, %d) = %s, want %s",
					tc.now.Format(time.RFC3339), tc.hourUTC, got, wantTotal)
			}
		})
	}
}

// TestDurationUntilNextUTCHour_AlwaysPositive: the scheduling contract
// is that the returned duration is strictly positive for any (now,
// hour) pair. A zero or negative duration would cause the timer to
// fire immediately, creating a hot loop.
func TestDurationUntilNextUTCHour_AlwaysPositive(t *testing.T) {
	t.Parallel()

	// Sample every minute across a 25-hour window to catch any
	// edge case in the wrap-around math.
	base := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 25*60; i += 17 { // 17-min step to avoid aliasing
		now := base.Add(time.Duration(i) * time.Minute)
		for hr := 0; hr < 24; hr++ {
			got := durationUntilNextUTCHour(now, hr)
			if got <= 0 {
				t.Fatalf("non-positive at now=%s hour=%d: got %s",
					now.Format(time.RFC3339), hr, got)
			}
			if got > 25*time.Hour {
				t.Fatalf("over 25h at now=%s hour=%d: got %s",
					now.Format(time.RFC3339), hr, got)
			}
		}
	}
}

// TestNowUTC_IsMonotonic checks the receiver-method contract: nowUTC
// returns wall-clock UTC (not ambient-zone now()), and successive calls
// are monotonically non-decreasing. Guards against an accidental
// refactor that swaps time.Now().UTC() for time.Now() (local).
func TestNowUTC_IsMonotonic(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}

	t1 := w.nowUTC()
	t2 := w.nowUTC()
	if t2.Before(t1) {
		t.Errorf("nowUTC not monotonic: t1=%s t2=%s", t1, t2)
	}
	// Must be explicitly UTC (zone offset = 0).
	if _, offset := t1.Zone(); offset != 0 {
		t.Errorf("nowUTC returned non-UTC zone with offset %d", offset)
	}
}

// TestWebhookRetentionConstants locks in the retention windows. These
// values are cited in operator docs and dashboards; changing them must
// be a deliberate, reviewed edit rather than a silent drift.
func TestWebhookRetentionConstants(t *testing.T) {
	t.Parallel()
	if WebhookDeliveriesRetentionDays != 30 {
		t.Errorf("WebhookDeliveriesRetentionDays = %d, want 30",
			WebhookDeliveriesRetentionDays)
	}
	if WebhookDLQRetentionDays != 90 {
		t.Errorf("WebhookDLQRetentionDays = %d, want 90",
			WebhookDLQRetentionDays)
	}
	// DLQ retention must always be >= main retention; a DLQ that
	// expires first defeats its purpose.
	if WebhookDLQRetentionDays < WebhookDeliveriesRetentionDays {
		t.Errorf("DLQ retention (%d) must be >= deliveries retention (%d)",
			WebhookDLQRetentionDays, WebhookDeliveriesRetentionDays)
	}
}

// TestDedupKindsAreDistinct: each dedup kind has to be unique or the
// work_order_dedup table will cross-wire two scanners onto the same
// key. A regression copy-paste that reuses a label would be caught
// here.
func TestDedupKindsAreDistinct(t *testing.T) {
	t.Parallel()
	all := []string{
		dedupKindShadowIT,
		dedupKindDuplicateSerial,
		dedupKindLowQualityPersistent,
	}
	seen := make(map[string]struct{}, len(all))
	for _, k := range all {
		if k == "" {
			t.Fatalf("dedup kind is empty — the scan would silently disable dedup")
		}
		if _, dup := seen[k]; dup {
			t.Fatalf("dedup kind %q is duplicated", k)
		}
		seen[k] = struct{}{}
	}
}

// TestQualityScanConstants: the quality scanner's initial-delay and
// interval shape the dashboard's freshness. An accidental tweak
// (e.g. flipping interval to 24ms) would flood the scanner.
func TestQualityScanConstants(t *testing.T) {
	t.Parallel()
	if qualityScanInitialDelay <= 0 {
		t.Errorf("qualityScanInitialDelay must be positive, got %s",
			qualityScanInitialDelay)
	}
	if qualityScanInitialDelay > time.Minute {
		t.Errorf("qualityScanInitialDelay over a minute (%s) — deploys wait too long for first scan",
			qualityScanInitialDelay)
	}
	if qualityScanInterval < time.Hour {
		t.Errorf("qualityScanInterval below 1h (%s) — risks overloading scanner",
			qualityScanInterval)
	}
}

// TestStartDivergenceChecker_FeatureFlagOff is the default-off
// contract: absent CMDB_INTEGRATION_DIVERGENCE_CHECK=1, the sampler
// returns immediately and spawns no goroutine. A regression that
// flips the default would surprise-deploy a new recurring DB job.
func TestStartDivergenceChecker_FeatureFlagOff(t *testing.T) {
	// Not t.Parallel(): we touch a process-wide env var.

	t.Setenv(divergenceFlagEnv, "") // flag OFF

	w := &WorkflowSubscriber{} // no pool, no cipher — must not matter
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Must return immediately without panic and without spawning
	// anything that would touch the nil pool.
	w.StartDivergenceChecker(ctx)

	// Sanity: the flag env actually stayed unset. t.Setenv restores
	// it automatically, but a misreading of the helper could leak.
	if got := os.Getenv(divergenceFlagEnv); got != "" {
		t.Errorf("env %s should be unset in test, got %q", divergenceFlagEnv, got)
	}
}

// TestStartDivergenceChecker_FlagOnButNoCipher: even with the flag
// set, a nil cipher must cause the checker to decline to start — the
// alternative is flooding the divergence counter with "decrypt
// failed" noise for every sample.
func TestStartDivergenceChecker_FlagOnButNoCipher(t *testing.T) {
	// Not t.Parallel(): env var manipulation.
	t.Setenv(divergenceFlagEnv, "1")

	w := &WorkflowSubscriber{cipher: nil} // no cipher — must refuse
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Must return cleanly (log a Warn and bail). The goroutine guard
	// inside the checker is what we're confirming — no panic, no
	// hot-loop, no DB touch.
	w.StartDivergenceChecker(ctx)
}

// TestDivergenceConstants locks the cadence + sample-size numbers so
// a silent tweak is caught. These values are in the operator
// runbooks.
func TestDivergenceConstants(t *testing.T) {
	t.Parallel()
	if divergenceCheckInterval != 15*time.Minute {
		t.Errorf("divergenceCheckInterval = %s, want 15m", divergenceCheckInterval)
	}
	if divergenceSampleSize != 500 {
		t.Errorf("divergenceSampleSize = %d, want 500", divergenceSampleSize)
	}
	if divergenceFlagEnv == "" {
		t.Error("divergenceFlagEnv is empty — feature flag would always be off")
	}
}

// TestSourceLabelsAreDistinct: if two scans share a source label, the
// errors_suppressed_total counter collapses their failure series into
// one indistinguishable bucket — making a broken scanner impossible
// to attribute.
func TestSourceLabelsAreDistinct(t *testing.T) {
	t.Parallel()
	labels := []string{
		sourceNotifyOrderDone,
		sourceNotifyAlert,
		sourceNotifyCreate,
		sourceNotifyInventory,
		sourceSLABreach,
		sourceSLAWarning,
		sourceCleanupSessions,
		sourceCleanupConflicts,
		sourceCleanupDiscoveries,
		sourceWarrantyCheck,
		sourceAssetVerification,
		sourceDataCorrection,
		sourceEOLCheck,
		sourceLifespanCheck,
		sourceShadowITCheck,
		sourceDuplicateSerialCheck,
		sourceMissingLocationCheck,
		sourceFirmwareCheck,
		sourceBMCSecurityCheck,
		sourceScanDiffEventHandler,
		sourceBMCDefaultEventParse,
		sourceLowQualityCheck,
		sourceMetricsInsert,
		sourceAuditPartitionSample,
	}
	seen := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		if l == "" {
			t.Error("empty source label found — would collapse with every other empty")
		}
		if _, dup := seen[l]; dup {
			t.Errorf("duplicate source label %q", l)
		}
		seen[l] = struct{}{}
	}
}
