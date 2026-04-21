package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// writeTextfileMetrics emits Prometheus textfile-collector metrics for
// the archive run. Meant to be written to a host path scraped by
// node_exporter --collector.textfile.directory, which is the standard
// way to expose metrics from short-lived CronJobs without running a
// pushgateway.
//
// On success, both metrics are written. On failure, only the
// stage-labelled failures counter is emitted so a pre-existing
// last-success timestamp from a previous run is not overwritten.
//
// Textfile format:
//
//	# HELP cmdb_audit_archive_last_success_unix Unix timestamp of last successful archive run.
//	# TYPE cmdb_audit_archive_last_success_unix gauge
//	cmdb_audit_archive_last_success_unix{month="2025-04"} 1735689600
//	# HELP cmdb_audit_archive_failures_total Archive failures by stage.
//	# TYPE cmdb_audit_archive_failures_total counter
//	cmdb_audit_archive_failures_total{stage="upload",month="2025-04"} 1
//
// node_exporter's textfile collector picks up any *.prom file in its
// configured directory, so the filename must be stable across runs
// (audit-archive-<month>.prom). A write-then-rename keeps the file
// atomic on the scraper's side.
func writeTextfileMetrics(path, month string, success bool, failureStage string) error {
	if path == "" {
		return nil
	}
	var content string
	if success {
		content = fmt.Sprintf(
			"# HELP cmdb_audit_archive_last_success_unix Unix timestamp of last successful archive run.\n"+
				"# TYPE cmdb_audit_archive_last_success_unix gauge\n"+
				"cmdb_audit_archive_last_success_unix{month=%q} %d\n",
			month, time.Now().Unix(),
		)
	} else {
		if failureStage == "" {
			failureStage = "unknown"
		}
		content = fmt.Sprintf(
			"# HELP cmdb_audit_archive_failures_total Archive failures by stage.\n"+
				"# TYPE cmdb_audit_archive_failures_total counter\n"+
				"cmdb_audit_archive_failures_total{stage=%q,month=%q} 1\n",
			failureStage, month,
		)
	}

	// Atomic rename: write to a sibling tmp file in the same directory
	// so the rename stays within one filesystem. node_exporter reading
	// a half-written .prom file would emit a malformed metric and
	// poison the scrape for one interval.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".audit-archive-*.prom.tmp")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	defer func() {
		if tmp != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.WriteString(content); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	tmpName := tmp.Name()
	tmp = nil
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// classifyFailureStage maps an archiveMonth error to the pipeline
// stage label. We key off substrings in the wrapped error messages
// rather than bubbling a typed error up because the CLI's one and
// only consumer of these labels is the textfile output.
func classifyFailureStage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case contains(msg, "parse month"), contains(msg, "retention window"), contains(msg, "suspicious partition"):
		return "validate"
	case contains(msg, "detach:"):
		return "detach"
	case contains(msg, "export parquet"), contains(msg, "row count mismatch"):
		return "export"
	case contains(msg, "upload s3"):
		return "upload"
	case contains(msg, "verify s3"):
		return "verify"
	case contains(msg, "drop partition"):
		return "drop"
	default:
		return "unknown"
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
