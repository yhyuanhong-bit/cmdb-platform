package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTextfileMetrics_EmptyPathIsNoop(t *testing.T) {
	if err := writeTextfileMetrics("", "2025-04", true, ""); err != nil {
		t.Fatalf("empty path should be no-op, got %v", err)
	}
}

func TestWriteTextfileMetrics_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-archive.prom")
	if err := writeTextfileMetrics(path, "2025-04", true, ""); err != nil {
		t.Fatalf("write success: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	body := string(got)
	if !strings.Contains(body, "cmdb_audit_archive_last_success_unix{month=\"2025-04\"}") {
		t.Errorf("missing last-success metric in %q", body)
	}
	if strings.Contains(body, "cmdb_audit_archive_failures_total") {
		t.Errorf("success file should not emit failures counter: %q", body)
	}
	// Atomic write: the tmp sibling should not linger.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("atomic write leaked tmp file: %s", e.Name())
		}
	}
}

func TestWriteTextfileMetrics_Failure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-archive.prom")
	if err := writeTextfileMetrics(path, "2025-04", false, "upload"); err != nil {
		t.Fatalf("write failure: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `stage="upload",month="2025-04"`) {
		t.Errorf("missing failure labels in %q", string(body))
	}
	if strings.Contains(string(body), "last_success_unix") {
		t.Errorf("failure file should not emit success gauge: %q", string(body))
	}
}

func TestClassifyFailureStage(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("parse month \"foo\": bad format"), "validate"},
		{errors.New("month 2026-01 is inside the 12-month retention window (cutoff 2025-04)"), "validate"},
		{errors.New("refusing suspicious partition name"), "validate"},
		{errors.New("detach: connection refused"), "detach"},
		{errors.New("export parquet (partition orphaned; re-attach via runbook): io: short write"), "export"},
		{errors.New("row count mismatch: pre-detach=10 exported=9"), "export"},
		{errors.New("upload s3 (partition orphaned; re-attach via runbook): 403"), "upload"},
		{errors.New("verify s3 (partition orphaned; re-attach via runbook): ETag mismatch"), "verify"},
		{errors.New("drop partition (data is already safe in s3://b/k): permission denied"), "drop"},
		{errors.New("something else"), "unknown"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := classifyFailureStage(tc.err); got != tc.want {
			var msg string
			if tc.err != nil {
				msg = tc.err.Error()
			}
			t.Errorf("classifyFailureStage(%q) = %q, want %q", msg, got, tc.want)
		}
	}
}
