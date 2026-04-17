package workflows

import (
	"strings"
	"testing"
	"time"
)

// TestComputeAdapterBackoff verifies the escalation schedule:
// 1 → 30s, 2 → 2m, 3 → 10m, 4+ → 30m cap. Keeping this as a pure-function
// test means the schedule is locked in even if the calling workflow
// layer is rewritten.
func TestComputeAdapterBackoff(t *testing.T) {
	tests := []struct {
		name     string
		failures int32
		want     time.Duration
	}{
		{"n=0 floors to first step", 0, 30 * time.Second},
		{"n=1 first failure", 1, 30 * time.Second},
		{"n=2 second failure", 2, 2 * time.Minute},
		{"n=3 third failure", 3, 10 * time.Minute},
		{"n=4 caps at 30m", 4, 30 * time.Minute},
		{"n=10 still capped", 10, 30 * time.Minute},
		{"n=100 still capped", 100, 30 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeAdapterBackoff(tt.failures)
			if got != tt.want {
				t.Errorf("computeAdapterBackoff(%d) = %v, want %v", tt.failures, got, tt.want)
			}
		})
	}
}

// TestComputeAdapterBackoffMonotonic — the schedule must never decrease
// as the failure count grows; regressions here would re-introduce the
// original "retry constantly" bug on the way up.
func TestComputeAdapterBackoffMonotonic(t *testing.T) {
	var prev time.Duration
	for n := int32(1); n <= 10; n++ {
		got := computeAdapterBackoff(n)
		if got < prev {
			t.Errorf("backoff decreased at n=%d: %v < %v", n, got, prev)
		}
		prev = got
	}
}

func TestTruncateReason(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantLen int
	}{
		{"empty stays empty", "", 0},
		{"short passes through", "timeout", len("timeout")},
		{"exact limit unchanged", strings.Repeat("x", adapterFailureReasonMaxLen), adapterFailureReasonMaxLen},
		{"over limit truncated", strings.Repeat("x", adapterFailureReasonMaxLen+50), adapterFailureReasonMaxLen},
		{"way over limit truncated", strings.Repeat("x", 5000), adapterFailureReasonMaxLen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateReason(tt.in)
			if len(got) != tt.wantLen {
				t.Errorf("len(truncateReason(len=%d)) = %d, want %d", len(tt.in), len(got), tt.wantLen)
			}
			// Truncation must preserve a prefix (no reordering / replacement).
			if len(tt.in) > 0 && tt.wantLen > 0 && !strings.HasPrefix(tt.in, got) {
				t.Errorf("truncateReason did not return a prefix of input")
			}
		})
	}
}

// TestAdapterDisableThresholdConstant locks the disable threshold so that
// changing it is a deliberate, reviewable edit — not an accident. The
// audit message and documentation both cite "3 consecutive failures".
func TestAdapterDisableThresholdConstant(t *testing.T) {
	if adapterDisableThreshold != 3 {
		t.Fatalf("adapterDisableThreshold = %d, want 3 (see audit message + docs)", adapterDisableThreshold)
	}
}
