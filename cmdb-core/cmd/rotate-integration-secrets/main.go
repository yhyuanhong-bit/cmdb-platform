// rotate-integration-secrets re-encrypts integration_adapters.config_encrypted
// and webhook_subscriptions.secret_encrypted rows that were sealed under an
// older key version so every row ends up at the active KeyRing version.
//
// Prerequisites (see docs/integration-encryption-deployment.md §8):
//
//  1. A new CMDB_SECRET_KEY_V{N+1} has been provisioned.
//  2. The running server has restarted and picked up the new ring.
//  3. CMDB_SECRET_KEY_ACTIVE has been flipped to the new version (or omitted
//     so the highest-version default takes effect).
//
// Behaviour:
//
//   - Defaults to dry-run. Pass --apply to actually write.
//   - --target selects adapters, webhooks, or all (default all).
//   - A "candidate" is a row whose encrypted column is non-NULL AND whose
//     version prefix is anything other than the active version. Unprefixed
//     (pre-KeyRing) ciphertext counts as v1 per the KeyRing wire format.
//   - Idempotent: re-running picks up nothing after a clean apply run.
//   - Row-level errors are logged and counted but do not abort the batch.
//   - On apply, one audit_events row per affected tenant is inserted with
//     action='integration_key_rotated', module='integration',
//     source='admin-cli' (same shape as backfill-integration-secrets).
//
// Usage:
//
//	# Dry-run:
//	DATABASE_URL=... CMDB_SECRET_KEY_V1=... CMDB_SECRET_KEY_V2=... \
//	  go run ./cmd/rotate-integration-secrets
//
//	# Actually rotate:
//	DATABASE_URL=... CMDB_SECRET_KEY_V1=... CMDB_SECRET_KEY_V2=... \
//	  go run ./cmd/rotate-integration-secrets --apply
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tableStats struct {
	name       string
	candidate  int64
	rotated    int64
	errors     int64
	perTenant  map[uuid.UUID]int64
	perVersion map[int]int64 // source-version histogram for diagnostics
}

func newTableStats(name string) tableStats {
	return tableStats{
		name:       name,
		perTenant:  map[uuid.UUID]int64{},
		perVersion: map[int]int64{},
	}
}

func main() {
	apply := flag.Bool("apply", false, "Actually re-encrypt. Without this flag, the command only counts candidates (dry-run).")
	progressEvery := flag.Int64("progress-every", 100, "Log a progress line every N rows.")
	target := flag.String("target", "all", "Which table to rotate: adapters | webhooks | all.")
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

	keyring, err := crypto.KeyRingFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load at-rest encryption key ring: %v\n", err)
		os.Exit(2)
	}

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
	active := keyring.ActiveVersion()
	fmt.Printf("=== rotate-integration-secrets (%s, active=v%d, available=%v) ===\n",
		mode, active, keyring.Versions())

	var adapters, webhooks tableStats
	if *target == "all" || *target == "adapters" {
		adapters, err = rotateAdapters(ctx, pool, keyring, *apply, *progressEvery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "adapters rotation failed: %v\n", err)
			os.Exit(1)
		}
		printStats(adapters)
	}
	if *target == "all" || *target == "webhooks" {
		webhooks, err = rotateWebhooks(ctx, pool, keyring, *apply, *progressEvery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "webhooks rotation failed: %v\n", err)
			os.Exit(1)
		}
		printStats(webhooks)
	}

	if *apply && (adapters.rotated > 0 || webhooks.rotated > 0) {
		if err := writeAuditEvent(ctx, pool, active, adapters, webhooks); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write audit event: %v\n", err)
		}
	}

	if !*apply {
		fmt.Println()
		fmt.Println("Dry-run complete. Re-run with --apply to rotate.")
	}
	if adapters.errors > 0 || webhooks.errors > 0 {
		fmt.Fprintln(os.Stderr, "completed with row-level errors; see log lines above")
		os.Exit(1)
	}
}

func printStats(s tableStats) {
	fmt.Printf("  %s: %d candidate, %d rotated, %d errors\n",
		s.name, s.candidate, s.rotated, s.errors)
	if len(s.perVersion) > 0 {
		for v, n := range s.perVersion {
			fmt.Printf("    from v%d: %d\n", v, n)
		}
	}
}

// activeVersionPrefix is the literal bytea prefix of ciphertext written
// under the active version. Used in SQL to exclude already-current rows
// cheaply (Postgres can use a bytea LIKE on the prefix without materializing
// plaintext).
func activeVersionPrefix(active int) []byte {
	return []byte("v" + strconv.Itoa(active) + ":")
}

// rotateAdapters walks integration_adapters rows whose config_encrypted
// is not already prefixed with the active version. It decrypts with the
// ring (which dispatches by the existing prefix, or routes unprefixed
// legacy rows to v1) and re-encrypts under the active version. Tenant
// scoping is preserved in the UPDATE predicate so a misbehaving row can't
// leak across tenants.
func rotateAdapters(ctx context.Context, pool *pgxpool.Pool, ring *crypto.KeyRing, apply bool, progressEvery int64) (tableStats, error) {
	stats := newTableStats("integration_adapters")
	active := ring.ActiveVersion()
	prefix := activeVersionPrefix(active)

	// Count: rows with non-NULL ciphertext that do NOT already start with
	// the active version prefix. starts_with lets Postgres use a simple
	// byte comparison; bytea indexing isn't useful here because the
	// cardinality is low but the query is also bounded by tenant scope
	// at the UPDATE site.
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM integration_adapters
		WHERE config_encrypted IS NOT NULL
		  AND NOT starts_with(config_encrypted, $1)
	`, prefix).Scan(&stats.candidate)
	if err != nil {
		return stats, fmt.Errorf("count adapters: %w", err)
	}

	if !apply {
		return stats, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT id, tenant_id, config_encrypted
		FROM integration_adapters
		WHERE config_encrypted IS NOT NULL
		  AND NOT starts_with(config_encrypted, $1)
	`, prefix)
	if err != nil {
		return stats, fmt.Errorf("select adapters: %w", err)
	}
	defer rows.Close()

	var processed int64
	for rows.Next() {
		var id, tenantID uuid.UUID
		var oldCt []byte
		if err := rows.Scan(&id, &tenantID, &oldCt); err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  scan error: %v\n", err)
			continue
		}

		// Record source version before re-encrypt for diagnostics. The
		// parser is conservative — unrecognized prefix falls through to
		// v1 matching the KeyRing's own decrypt behaviour.
		srcVersion := detectVersion(oldCt)
		stats.perVersion[srcVersion]++

		plaintext, err := ring.Decrypt(oldCt)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  decrypt error (adapter %s, from v%d): %v\n", id, srcVersion, err)
			continue
		}

		newCt, err := ring.Encrypt(plaintext)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  encrypt error (adapter %s): %v\n", id, err)
			continue
		}

		// The UPDATE predicate still guards against the active-version
		// prefix so a concurrent writer that beats us to the row doesn't
		// get clobbered back to v{active}. Tenant filter is defence in
		// depth — the SELECT already scoped to specific rows by id.
		tag, err := pool.Exec(ctx, `
			UPDATE integration_adapters
			SET config_encrypted = $1
			WHERE id = $2 AND tenant_id = $3
			  AND config_encrypted IS NOT NULL
			  AND NOT starts_with(config_encrypted, $4)
		`, newCt, id, tenantID, prefix)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  update error (adapter %s): %v\n", id, err)
			continue
		}
		if tag.RowsAffected() == 0 {
			// Row was already rotated by a racing process. Not an
			// error — idempotency means we can skip it.
			continue
		}

		stats.rotated++
		stats.perTenant[tenantID]++
		processed++
		if progressEvery > 0 && processed%progressEvery == 0 {
			fmt.Printf("  ...adapters: %d processed\n", processed)
		}
	}
	return stats, rows.Err()
}

// rotateWebhooks mirrors rotateAdapters for the webhook_subscriptions
// table. Webhooks carry the HMAC signing secret rather than adapter JSON
// but the ciphertext shape is identical — same KeyRing, same prefix,
// same DB pattern.
func rotateWebhooks(ctx context.Context, pool *pgxpool.Pool, ring *crypto.KeyRing, apply bool, progressEvery int64) (tableStats, error) {
	stats := newTableStats("webhook_subscriptions")
	active := ring.ActiveVersion()
	prefix := activeVersionPrefix(active)

	err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_subscriptions
		WHERE secret_encrypted IS NOT NULL
		  AND NOT starts_with(secret_encrypted, $1)
	`, prefix).Scan(&stats.candidate)
	if err != nil {
		return stats, fmt.Errorf("count webhooks: %w", err)
	}

	if !apply {
		return stats, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT id, tenant_id, secret_encrypted
		FROM webhook_subscriptions
		WHERE secret_encrypted IS NOT NULL
		  AND NOT starts_with(secret_encrypted, $1)
	`, prefix)
	if err != nil {
		return stats, fmt.Errorf("select webhooks: %w", err)
	}
	defer rows.Close()

	var processed int64
	for rows.Next() {
		var id, tenantID uuid.UUID
		var oldCt []byte
		if err := rows.Scan(&id, &tenantID, &oldCt); err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  scan error: %v\n", err)
			continue
		}

		srcVersion := detectVersion(oldCt)
		stats.perVersion[srcVersion]++

		plaintext, err := ring.Decrypt(oldCt)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  decrypt error (webhook %s, from v%d): %v\n", id, srcVersion, err)
			continue
		}

		newCt, err := ring.Encrypt(plaintext)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  encrypt error (webhook %s): %v\n", id, err)
			continue
		}

		tag, err := pool.Exec(ctx, `
			UPDATE webhook_subscriptions
			SET secret_encrypted = $1
			WHERE id = $2 AND tenant_id = $3
			  AND secret_encrypted IS NOT NULL
			  AND NOT starts_with(secret_encrypted, $4)
		`, newCt, id, tenantID, prefix)
		if err != nil {
			stats.errors++
			fmt.Fprintf(os.Stderr, "  update error (webhook %s): %v\n", id, err)
			continue
		}
		if tag.RowsAffected() == 0 {
			continue
		}

		stats.rotated++
		stats.perTenant[tenantID]++
		processed++
		if progressEvery > 0 && processed%progressEvery == 0 {
			fmt.Printf("  ...webhooks: %d processed\n", processed)
		}
	}
	return stats, rows.Err()
}

// detectVersion inspects a ciphertext and returns the integer version its
// ASCII prefix advertises, defaulting to 1 for unprefixed legacy rows.
// Kept in sync with internal/platform/crypto/keyring.go parseVersionPrefix.
// Unknown prefixes fall back to 0 which is a signal for "couldn't parse"
// without colliding with a real v1 legacy row.
func detectVersion(ct []byte) int {
	const minPrefix = 3 // v1:
	if len(ct) < minPrefix || ct[0] != 'v' {
		return 1
	}
	i := 1
	for i < len(ct) && ct[i] >= '0' && ct[i] <= '9' {
		i++
	}
	if i == 1 || i >= len(ct) || ct[i] != ':' {
		return 1
	}
	v, err := strconv.Atoi(string(ct[1:i]))
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// writeAuditEvent records the rotation as one audit_events row per
// affected tenant (audit_events.tenant_id is NOT NULL with an FK to
// tenants — there is no cross-tenant sentinel, same rule as the backfill
// CLI). The diff includes the target version so downstream dashboards
// can show the rotation trail over time.
func writeAuditEvent(ctx context.Context, pool *pgxpool.Pool, active int, adapters, webhooks tableStats) error {
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
			`{"adapters_rotated":%d,"webhooks_rotated":%d,"active_version":%d,"tool":"rotate-integration-secrets","version":"1"}`,
			t.adapters, t.webhooks, active,
		)
		if _, err := pool.Exec(ctx, `
			INSERT INTO audit_events (tenant_id, action, module, target_type, diff, source)
			VALUES ($1, 'integration_key_rotated', 'integration', 'system', $2::jsonb, 'admin-cli')
		`, tid, diff); err != nil {
			return errors.New("audit insert failed for tenant " + tid.String() + ": " + err.Error())
		}
	}
	return nil
}
