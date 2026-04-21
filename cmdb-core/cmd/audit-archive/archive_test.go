package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestSafePartitionName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"audit_events_2025_04", true},
		{"audit_events_2099_12", true},
		{"audit_events_legacy_partition", false},
		{"audit_events_2025-04", false},
		{"audit_events_2025_4", false},
		{"audit_events_20250_04", false},
		{"audit_events_", false},
		{"users_2025_04", false},
		{"audit_events_2025_04; DROP TABLE audit_events", false},
		{"audit_events_2025_ab", false},
	}
	for _, tc := range cases {
		if got := safePartitionName(tc.name); got != tc.want {
			t.Errorf("safePartitionName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestArchiveMonth_RetentionGuard(t *testing.T) {
	// With now = 2026-04-21 and --retention-months 12, the cutoff is
	// 2025-04-21. The guard must refuse any month >= 2025-04 and
	// accept any month <  2025-04. Refused months must error before
	// any DB/S3 call, so a nil pool doesn't crash the test.
	now := func() time.Time { return time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC) }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Only exercise the reject branch here — the accept branch
	// requires a real pgxpool and is covered by a staging smoke test.
	cases := []struct {
		month string
		label string
	}{
		{"2025-04", "exactly at cutoff"},
		{"2026-01", "inside retention window"},
		{"2026-04", "current month"},
	}
	for _, tc := range cases {
		opts := archiveOptions{
			bucket:          "cmdb-audit-archive",
			month:           tc.month,
			retentionMonths: 12,
			dryRun:          true,
			now:             now,
		}
		err := archiveMonth(t.Context(), log, nil, nil, opts)
		if err == nil {
			t.Errorf("%s: archiveMonth(%q) returned nil, want retention rejection", tc.label, tc.month)
			continue
		}
		if !strings.Contains(err.Error(), "retention window") {
			t.Errorf("%s: archiveMonth(%q) returned %v, want retention-window error", tc.label, tc.month, err)
		}
	}
}

func TestArchiveMonth_MalformedMonth(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC) }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	opts := archiveOptions{
		bucket:          "cmdb-audit-archive",
		month:           "not-a-date",
		retentionMonths: 12,
		dryRun:          true,
		now:             now,
	}
	err := archiveMonth(t.Context(), log, nil, nil, opts)
	if err == nil || !strings.Contains(err.Error(), "parse month") {
		t.Fatalf("expected parse-month error, got %v", err)
	}
}
