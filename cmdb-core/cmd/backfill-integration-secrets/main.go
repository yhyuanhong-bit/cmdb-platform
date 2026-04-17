// backfill-integration-secrets encrypts the plaintext that pre-dates
// migration 000038 (at-rest encryption for integration secrets) so every
// row has both a plaintext column and a matching ciphertext column.
//
// Dual-write already ensures new writes populate both columns. This command
// fills the gap for rows that were created before the encryption code
// shipped — their `config_encrypted` / `secret_encrypted` columns are NULL.
// Once this finishes cleanly, the plaintext columns can be cleared with
// db/scripts/cleanup/clear_integration_plaintext.sql.
//
// Defaults to dry-run. Pass --apply to actually write.
//
//	# Just count what would be affected:
//	go run ./cmd/backfill-integration-secrets
//
//	# Actually encrypt:
//	DATABASE_URL=... CMDB_SECRET_KEY=... \
//	  go run ./cmd/backfill-integration-secrets --apply
//
// The tool is idempotent: a second run after --apply finds 0 candidates.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tableStats struct {
	name      string
	candidate int64 // rows that qualify for backfill
	encrypted int64 // rows we successfully updated
	errors    int64 // rows that failed (logged, continued)
	// perTenant tracks encrypted counts per tenant so we can emit one
	// audit_events row per tenant (audit_events.tenant_id is NOT NULL
	// with an FK to tenants; there is no cross-tenant sentinel).
	perTenant map[uuid.UUID]int64
}

func main() {
	apply := flag.Bool("apply", false, "Actually write the backfilled ciphertext. Without this flag the command only counts candidates (dry-run).")
	progressEvery := flag.Int64("progress-every", 100, "Log a progress line every N rows.")
	target := flag.String("target", "all", "Which table to backfill: adapters | webhooks | all.")
	flag.Parse()

	if *target != "all" && *target != "adapters" && *target != "webhooks" {
		fmt.Fprintf(os.Stderr, "invalid --target %q (want adapters|webhooks|all)\n", *target)
		os.Exit(2)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}

	// Load the same KeyRing the server uses. Backfill writes new
	// ciphertext, so it must use the active version's key — exactly what
	// KeyRing.Encrypt does. Legacy deployments with only CMDB_SECRET_KEY
	// set get a v1-only ring, which matches pre-rotation behaviour.
	keyring, err := crypto.KeyRingFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load at-rest encryption key ring: %v\n", err)
		os.Exit(2)
	}
	var cipher crypto.Cipher = keyring

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	mode := "DRY-RUN"
	if *apply {
		mode = "APPLY"
	}
	fmt.Printf("=== backfill-integration-secrets (%s) ===\n", mode)

	var adapters, webhooks tableStats
	if *target == "all" || *target == "adapters" {
		adapters, err = backfillAdapters(ctx, pool, cipher, *apply, *progressEvery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "adapters backfill failed: %v\n", err)
			os.Exit(1)
		}
		printStats(adapters)
	}
	if *target == "all" || *target == "webhooks" {
		webhooks, err = backfillWebhooks(ctx, pool, cipher, *apply, *progressEvery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "webhooks backfill failed: %v\n", err)
			os.Exit(1)
		}
		printStats(webhooks)
	}

	if *apply && (adapters.encrypted > 0 || webhooks.encrypted > 0) {
		if err := writeAuditEvent(ctx, pool, adapters, webhooks); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write audit event: %v\n", err)
		}
	}

	if !*apply {
		fmt.Println()
		fmt.Println("Dry-run complete. Re-run with --apply to encrypt.")
	}
	if adapters.errors > 0 || webhooks.errors > 0 {
		fmt.Fprintln(os.Stderr, "completed with row-level errors; see log lines above")
		os.Exit(1)
	}
}

func printStats(s tableStats) {
	fmt.Printf("  %s: %d candidate, %d encrypted, %d errors\n",
		s.name, s.candidate, s.encrypted, s.errors)
}

// backfillAdapters encrypts integration_adapters.config where config_encrypted
// is NULL. Treats empty-object configs as skipped — they carry no secrets
// worth encrypting and the cleanup script ignores them too.
func backfillAdapters(ctx context.Context, pool *pgxpool.Pool, cipher crypto.Cipher, apply bool, progressEvery int64) (tableStats, error) {
	stats := tableStats{name: "integration_adapters", perTenant: map[uuid.UUID]int64{}}

	// Count candidates first so dry-run and apply paths share the number.
	// Exclude empty-object configs — they carry no secrets worth encrypting
	// and the apply path would skip them anyway, so counting them would
	// make the dry-run number misleading ("9 candidate" that never drops).
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM integration_adapters
		WHERE config IS NOT NULL
		  AND config::text <> '{}'
		  AND config_encrypted IS NULL
	`).Scan(&stats.candidate)
	if err != nil {
		return stats, fmt.Errorf("count adapters: %w", err)
	}

	if !apply {
		return stats, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT id, tenant_id, config
		FROM integration_adapters
		WHERE config IS NOT NULL
		  AND config::text <> '{}'
		  AND config_encrypted IS NULL
	`)
	if err != nil {
		return stats, fmt.Errorf("select adapters: %w", err)
	}
	defer rows.Close()

	var processed int64
	for rows.Next() {
		var id, tenantID uuid.UUID
		var cfg []byte
		if err := rows.Scan(&id, &tenantID, &cfg); err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  scan error: %v\n", err)
			continue
		}

		enc, err := cipher.Encrypt(cfg)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  encrypt error (adapter %s): %v\n", id, err)
			continue
		}

		if _, err := pool.Exec(ctx, `
			UPDATE integration_adapters
			SET config_encrypted = $1
			WHERE id = $2 AND tenant_id = $3 AND config_encrypted IS NULL
		`, enc, id, tenantID); err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  update error (adapter %s): %v\n", id, err)
			continue
		}

		stats.encrypted++
		stats.perTenant[tenantID]++
		processed++
		if progressEvery > 0 && processed%progressEvery == 0 {
			fmt.Printf("  ...adapters: %d processed\n", processed)
		}
	}
	return stats, rows.Err()
}

// backfillWebhooks encrypts webhook_subscriptions.secret where
// secret_encrypted is NULL. NULL/empty secrets are skipped — a webhook
// without a secret simply doesn't sign payloads.
func backfillWebhooks(ctx context.Context, pool *pgxpool.Pool, cipher crypto.Cipher, apply bool, progressEvery int64) (tableStats, error) {
	stats := tableStats{name: "webhook_subscriptions", perTenant: map[uuid.UUID]int64{}}

	err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_subscriptions
		WHERE secret IS NOT NULL
		  AND secret <> ''
		  AND secret_encrypted IS NULL
	`).Scan(&stats.candidate)
	if err != nil {
		return stats, fmt.Errorf("count webhooks: %w", err)
	}

	if !apply {
		return stats, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT id, tenant_id, secret
		FROM webhook_subscriptions
		WHERE secret IS NOT NULL
		  AND secret <> ''
		  AND secret_encrypted IS NULL
	`)
	if err != nil {
		return stats, fmt.Errorf("select webhooks: %w", err)
	}
	defer rows.Close()

	var processed int64
	for rows.Next() {
		var id, tenantID uuid.UUID
		var secret string
		if err := rows.Scan(&id, &tenantID, &secret); err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  scan error: %v\n", err)
			continue
		}

		enc, err := cipher.Encrypt([]byte(secret))
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  encrypt error (webhook %s): %v\n", id, err)
			continue
		}

		if _, err := pool.Exec(ctx, `
			UPDATE webhook_subscriptions
			SET secret_encrypted = $1
			WHERE id = $2 AND tenant_id = $3 AND secret_encrypted IS NULL
		`, enc, id, tenantID); err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  update error (webhook %s): %v\n", id, err)
			continue
		}

		stats.encrypted++
		stats.perTenant[tenantID]++
		processed++
		if progressEvery > 0 && processed%progressEvery == 0 {
			fmt.Printf("  ...webhooks: %d processed\n", processed)
		}
	}
	return stats, rows.Err()
}

// writeAuditEvent records the backfill result as one audit_events row per
// affected tenant. audit_events.tenant_id is NOT NULL with an FK to
// tenants(id), so a single cross-tenant summary row is not representable;
// each tenant gets an event scoped to its own data.
func writeAuditEvent(ctx context.Context, pool *pgxpool.Pool, adapters, webhooks tableStats) error {
	totals := map[uuid.UUID]struct{ adapters, webhooks int64 }{}
	for tid, n := range adapters.perTenant {
		t := totals[tid]
		t.adapters = n
		totals[tid] = t
	}
	for tid, n := range webhooks.perTenant {
		t := totals[tid]
		t.webhooks = n
		totals[tid] = t
	}

	for tid, t := range totals {
		diff := fmt.Sprintf(
			`{"adapters_encrypted":%d,"webhooks_encrypted":%d,"tool":"backfill-integration-secrets","version":"1"}`,
			t.adapters, t.webhooks,
		)
		if _, err := pool.Exec(ctx, `
			INSERT INTO audit_events (tenant_id, action, module, target_type, diff, source)
			VALUES ($1, 'integration_backfill_completed', 'integration', 'system', $2::jsonb, 'admin-cli')
		`, tid, diff); err != nil {
			return errors.New("audit insert failed for tenant " + tid.String() + ": " + err.Error())
		}
	}
	return nil
}
