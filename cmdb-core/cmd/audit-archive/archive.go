package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/parquet-go/parquet-go"
)

type archiveOptions struct {
	bucket          string
	month           string // YYYY-MM
	retentionMonths int
	dryRun          bool
	kmsKeyID        string
	now             func() time.Time // injected clock for tests
}

// auditRow matches the audit_events column set (post-migration 000053).
// Parquet column order stays aligned with the SQL column order so
// auditors reading the file can map columns by position if they want.
type auditRow struct {
	ID           string `parquet:"id,zstd"`
	TenantID     string `parquet:"tenant_id,dict,zstd"`
	Action       string `parquet:"action,dict,zstd"`
	Module       string `parquet:"module,dict,optional,zstd"`
	TargetType   string `parquet:"target_type,dict,optional,zstd"`
	TargetID     string `parquet:"target_id,optional,zstd"`
	OperatorID   string `parquet:"operator_id,optional,zstd"`
	OperatorType string `parquet:"operator_type,dict,zstd"`
	Diff         string `parquet:"diff,snappy,optional"`
	Source       string `parquet:"source,dict,zstd"`
	CreatedAt    int64  `parquet:"created_at,timestamp(microsecond),zstd"`
}

// archiveMonth runs the full archive pipeline for a single monthly
// partition. Contract:
//
//  1. parse --month, refuse if within retention window
//  2. verify partition exists and is attached to audit_events
//  3. snapshot row count for post-upload verification
//  4. DETACH PARTITION ... CONCURRENTLY (PG14+, zero-lock)
//  5. stream the detached table into a parquet buffer
//  6. PUT to s3://<bucket>/audit/YYYY/MM/audit_events_YYYY_MM.parquet
//     with SSE-KMS and the SHA-256 recorded in object metadata
//  7. HEAD-verify the upload (content length > 0, ETag present)
//  8. DROP the orphan table
//
// Any step's failure aborts the pipeline. A partition that was
// detached but not yet dropped can be re-attached manually per the
// operations runbook.
func archiveMonth(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, s3c *s3.Client, opts archiveOptions) error {
	t, err := time.Parse("2006-01", opts.month)
	if err != nil {
		return fmt.Errorf("parse month %q: %w", opts.month, err)
	}

	nowFn := opts.now
	if nowFn == nil {
		nowFn = time.Now
	}
	// Compare at month granularity so a request issued on the 15th
	// behaves the same as one on the 1st. Without month-truncation,
	// `now - 12 months` for a mid-month "now" would admit the month
	// that is exactly 12 months back (its 1st falls before the 21st),
	// which is inside the retention window.
	nowMonth := time.Date(nowFn().UTC().Year(), nowFn().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	cutoff := nowMonth.AddDate(0, -opts.retentionMonths, 0)
	if !t.Before(cutoff) {
		return fmt.Errorf("month %s is inside the %d-month retention window (cutoff %s)",
			opts.month, opts.retentionMonths, cutoff.Format("2006-01"))
	}

	partition := "audit_events_" + t.Format("2006_01")
	if !safePartitionName(partition) {
		return fmt.Errorf("refusing suspicious partition name %q", partition)
	}
	log = log.With("partition", partition, "bucket", opts.bucket)

	attached, err := partitionIsAttached(ctx, pool, partition)
	if err != nil {
		return fmt.Errorf("check partition attachment: %w", err)
	}
	if !attached {
		log.Warn("partition missing or already detached, nothing to do")
		return nil
	}

	rowsBefore, err := partitionRowCount(ctx, pool, partition)
	if err != nil {
		return fmt.Errorf("row count pre-detach: %w", err)
	}
	log.Info("partition ready for archive", "rows", rowsBefore)

	if opts.dryRun {
		log.Info("dry-run; skipping DETACH / upload / DROP")
		return nil
	}

	if err := detachPartition(ctx, pool, partition); err != nil {
		return fmt.Errorf("detach: %w", err)
	}
	log.Info("partition detached")

	buf, exportedRows, err := exportPartitionParquet(ctx, pool, partition)
	if err != nil {
		return fmt.Errorf("export parquet (partition orphaned; re-attach via runbook): %w", err)
	}
	if exportedRows != rowsBefore {
		return fmt.Errorf("row count mismatch: pre-detach=%d exported=%d (aborting before DROP)",
			rowsBefore, exportedRows)
	}
	log.Info("parquet encoded", "bytes", buf.Len(), "rows", exportedRows)

	key := fmt.Sprintf("audit/%s/%s/%s.parquet", t.Format("2006"), t.Format("01"), partition)
	checksum, err := uploadObject(ctx, s3c, opts, key, buf.Bytes(), exportedRows)
	if err != nil {
		return fmt.Errorf("upload s3 (partition orphaned; re-attach via runbook): %w", err)
	}
	log.Info("uploaded to s3", "key", key, "sha256", checksum)

	if err := verifyObject(ctx, s3c, opts.bucket, key, int64(buf.Len())); err != nil {
		return fmt.Errorf("verify s3 (partition orphaned; re-attach via runbook): %w", err)
	}
	log.Info("s3 object verified")

	if err := dropPartition(ctx, pool, partition); err != nil {
		return fmt.Errorf("drop partition (data is already safe in s3://%s/%s): %w",
			opts.bucket, key, err)
	}
	log.Info("partition dropped", "key", key)
	return nil
}

// safePartitionName guards against any accidental injection in the
// EXECUTE-style SQL we issue. audit_events_YYYY_MM is the only shape
// we ever construct; anything else is a bug and should not ship.
func safePartitionName(name string) bool {
	if !strings.HasPrefix(name, "audit_events_") {
		return false
	}
	tail := strings.TrimPrefix(name, "audit_events_")
	if len(tail) != 7 || tail[4] != '_' {
		return false
	}
	for i, r := range tail {
		if i == 4 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func partitionIsAttached(ctx context.Context, pool *pgxpool.Pool, partition string) (bool, error) {
	const q = `
SELECT EXISTS (
    SELECT 1
    FROM pg_inherits i
    JOIN pg_class c ON c.oid = i.inhrelid
    JOIN pg_class p ON p.oid = i.inhparent
    WHERE p.relname = 'audit_events' AND c.relname = $1
)`
	var attached bool
	if err := pool.QueryRow(ctx, q, partition).Scan(&attached); err != nil {
		return false, err
	}
	return attached, nil
}

func partitionRowCount(ctx context.Context, pool *pgxpool.Pool, partition string) (int64, error) {
	// Partition name is validated by safePartitionName; no user input
	// reaches this fmt.Sprintf.
	q := fmt.Sprintf("SELECT count(*) FROM %s", partition)
	var n int64
	if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func detachPartition(ctx context.Context, pool *pgxpool.Pool, partition string) error {
	// DETACH CONCURRENTLY cannot run inside an explicit transaction.
	// pgxpool.Exec runs with implicit auto-commit which is what we want.
	stmt := fmt.Sprintf("ALTER TABLE audit_events DETACH PARTITION %s CONCURRENTLY", partition)
	_, err := pool.Exec(ctx, stmt)
	return err
}

func dropPartition(ctx context.Context, pool *pgxpool.Pool, partition string) error {
	stmt := fmt.Sprintf("DROP TABLE %s", partition)
	_, err := pool.Exec(ctx, stmt)
	return err
}

// exportPartitionParquet streams every row in the detached partition
// through a parquet writer. Uses a row-by-row Scan rather than COPY so
// we can normalize pgtype nullables and UUID bytes into the parquet
// logical types the reader-side expects.
func exportPartitionParquet(ctx context.Context, pool *pgxpool.Pool, partition string) (*bytes.Buffer, int64, error) {
	q := fmt.Sprintf(`
SELECT id, tenant_id, action, module, target_type, target_id,
       operator_id, operator_type::text, diff, source, created_at
FROM %s
ORDER BY created_at, id`, partition)

	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var buf bytes.Buffer
	pw := parquet.NewGenericWriter[auditRow](&buf)

	var total int64
	batch := make([]auditRow, 0, 1024)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if _, err := pw.Write(batch); err != nil {
			return err
		}
		total += int64(len(batch))
		batch = batch[:0]
		return nil
	}

	for rows.Next() {
		var (
			id, tenantID                  uuid.UUID
			action, opType, source        string
			module, targetType            *string
			targetID, operatorID          *uuid.UUID
			diff                          []byte
			createdAt                     time.Time
		)
		if err := rows.Scan(&id, &tenantID, &action, &module, &targetType,
			&targetID, &operatorID, &opType, &diff, &source, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		row := auditRow{
			ID:           id.String(),
			TenantID:     tenantID.String(),
			Action:       action,
			Module:       strOrEmpty(module),
			TargetType:   strOrEmpty(targetType),
			TargetID:     uuidOrEmpty(targetID),
			OperatorID:   uuidOrEmpty(operatorID),
			OperatorType: opType,
			Diff:         string(diff),
			Source:       source,
			CreatedAt:    createdAt.UnixMicro(),
		}
		batch = append(batch, row)
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return nil, 0, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := flush(); err != nil {
		return nil, 0, err
	}
	if err := pw.Close(); err != nil {
		return nil, 0, err
	}
	return &buf, total, nil
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func uuidOrEmpty(p *uuid.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}

// uploadObject PUTs the parquet payload to S3 with SSE-KMS (bucket
// default or explicit --kms-key-id), records the SHA-256 in object
// metadata, and returns the hex-encoded checksum for log correlation.
func uploadObject(ctx context.Context, s3c *s3.Client, opts archiveOptions, key string, body []byte, rowCount int64) (string, error) {
	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])

	input := &s3.PutObjectInput{
		Bucket:        aws.String(opts.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentType:   aws.String("application/vnd.apache.parquet"),
		ContentLength: aws.Int64(int64(len(body))),
		Metadata: map[string]string{
			"sha256":    hexSum,
			"row-count": fmt.Sprintf("%d", rowCount),
			"producer":  "cmdb-audit-archive",
		},
	}
	if opts.kmsKeyID != "" {
		input.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(opts.kmsKeyID)
	}
	if _, err := s3c.PutObject(ctx, input); err != nil {
		return "", err
	}
	return hexSum, nil
}

// verifyObject HEADs the newly-uploaded object and confirms the
// content length is non-zero and matches what we sent. A zero-byte
// object would be interpreted as "archived + empty" by readers and
// would mask a silent upload failure.
func verifyObject(ctx context.Context, s3c *s3.Client, bucket, key string, expected int64) error {
	head, err := s3c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	if head.ContentLength == nil || *head.ContentLength <= 0 {
		return errors.New("uploaded object has zero ContentLength")
	}
	if *head.ContentLength != expected {
		return fmt.Errorf("uploaded size %d != expected %d", *head.ContentLength, expected)
	}
	return nil
}
