// audit-archive drains a monthly audit_events partition to S3 cold
// storage, then DROPs it from the hot database.
//
// Contract: DETACH CONCURRENTLY → export Parquet → HEAD-verify upload →
// DROP. Any step's failure aborts before the next, leaving state that
// can be resumed (re-ATTACH the orphan partition, or re-run the CLI).
//
// Usage (see /cmdb-platform/docs/reports/phase4/4.2-audit-monthly-partition-and-archival.md
// for the full operations runbook):
//
//	audit-archive --month 2025-04 --bucket cmdb-audit-archive --retention-months 12
//	audit-archive --month 2025-04 --bucket cmdb-audit-archive --dry-run
//
// Required env:
//
//	DATABASE_URL         — pgxpool DSN to the cmdb_core primary
//	AWS_REGION           — target S3 region (aws-sdk-go-v2 also respects the shared config)
//	AWS_ACCESS_KEY_ID    — or IRSA / instance role
//	AWS_SECRET_ACCESS_KEY
//
// Exit codes:
//
//	0 success (including no-op when partition is missing)
//	1 runtime failure (DB, S3, parquet)
//	2 usage error
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		month     = flag.String("month", "", "YYYY-MM partition to archive (required)")
		bucket    = flag.String("bucket", "", "S3 bucket receiving the parquet export (required)")
		retention = flag.Int("retention-months", 12, "refuse to archive partitions newer than N months")
		dryRun    = flag.Bool("dry-run", false, "plan only; do not detach, upload, or drop")
		kmsKey    = flag.String("kms-key-id", "", "KMS key ARN/alias for SSE-KMS (empty = bucket default)")
	)
	flag.Parse()

	if *month == "" || *bucket == "" {
		fmt.Fprintln(os.Stderr, "usage: audit-archive --month YYYY-MM --bucket NAME [--retention-months 12] [--dry-run]")
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = logger.With("cmd", "audit-archive", "month", *month)

	// Interrupt handling: a SIGTERM in the middle of DETACH/DROP would
	// leave the partition in an undefined state. Cancel the context so
	// the current SQL statement aborts and we exit before the next step.
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Warn("signal received, cancelling in-flight work")
		cancel()
	}()
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is empty")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("load aws config", "err", err)
		os.Exit(1)
	}
	s3Client := s3.NewFromConfig(awsCfg)

	opts := archiveOptions{
		bucket:          *bucket,
		month:           *month,
		retentionMonths: *retention,
		dryRun:          *dryRun,
		kmsKeyID:        *kmsKey,
		now:             time.Now,
	}
	if err := archiveMonth(ctx, logger, pool, s3Client, opts); err != nil {
		logger.Error("archive failed", "err", err)
		os.Exit(1)
	}
	logger.Info("archive complete", "dry_run", *dryRun)
}
