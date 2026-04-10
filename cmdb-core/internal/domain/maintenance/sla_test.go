package maintenance

import (
	"testing"
	"time"
)

func TestSLADeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		priority string
		expected time.Duration
	}{
		{"critical", 4 * time.Hour},
		{"high", 8 * time.Hour},
		{"medium", 24 * time.Hour},
		{"low", 72 * time.Hour},
		{"unknown", 24 * time.Hour}, // defaults to medium
	}

	for _, tt := range tests {
		deadline := SLADeadline(tt.priority, now)
		got := deadline.Sub(now)
		if got != tt.expected {
			t.Errorf("SLADeadline(%q) = %v, want %v", tt.priority, got, tt.expected)
		}
	}
}

func TestSLAWarningThreshold(t *testing.T) {
	got := SLAWarningThreshold("critical")
	if got != 3*time.Hour {
		t.Errorf("SLAWarningThreshold(critical) = %v, want 3h", got)
	}
}
