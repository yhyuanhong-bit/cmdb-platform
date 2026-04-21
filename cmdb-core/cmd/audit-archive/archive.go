package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// archiveOptions bundles the CLI flags so the core entrypoint is
// testable without reaching into package-level globals.
type archiveOptions struct {
	bucket          string
	month           string // YYYY-MM
	retentionMonths int
	dryRun          bool
	kmsKeyID        string
	now             func() time.Time // injected clock for tests
}

// archiveMonth is the skeleton that the follow-up commit wires to real
// DETACH/upload/DROP logic. It currently validates inputs and logs the
// plan — intentionally no mutations — so migration 000053 can be
// rolled out and then this CLI can be extended without blocking a
// second migration on the archive impl.
func archiveMonth(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, s3c *s3.Client, opts archiveOptions) error {
	t, err := time.Parse("2006-01", opts.month)
	if err != nil {
		return fmt.Errorf("parse month %q: %w", opts.month, err)
	}

	now := opts.now
	if now == nil {
		now = time.Now
	}
	cutoff := now().UTC().AddDate(0, -opts.retentionMonths, 0)
	if !t.Before(cutoff) {
		return fmt.Errorf("month %s is inside the %d-month retention window (cutoff %s); refusing to archive",
			opts.month, opts.retentionMonths, cutoff.Format("2006-01"))
	}

	partition := "audit_events_" + t.Format("2006_01")
	log = log.With("partition", partition, "bucket", opts.bucket)

	_ = pool
	_ = s3c
	_ = ctx

	log.Info("archive plan",
		"retention_months", opts.retentionMonths,
		"dry_run", opts.dryRun,
		"kms_key_id", opts.kmsKeyID,
		"cutoff", cutoff.Format("2006-01"),
	)
	log.Warn("archive business logic not yet implemented; this skeleton only validates inputs")
	return nil
}
