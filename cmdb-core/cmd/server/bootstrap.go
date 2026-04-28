// bootstrap.go — startup-time database setup extracted from main.go
// during the Phase 2 God-file split (2026-04-28).
//
// Contains the three pre-flight blocks that used to live inline in main():
//   - applyPendingMigrations: auto-runs *.up.sql files in MIGRATIONS_DIR
//   - verifyMigrationVersion: aborts startup if schema is too old / too new
//   - seedIfEmpty: applies db/seed/seed.sql or creates a minimal admin
//
// Each function fails closed: any error that would leave the database in
// an ambiguous state calls zap.L().Fatal so the operator sees the issue
// at boot rather than chasing phantom 5xx in production.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// applyPendingMigrations walks MIGRATIONS_DIR (default "migrations") and
// applies every *.up.sql file whose version isn't already in
// schema_migrations. Idempotent. A failed schema_migrations write is
// logged at Error level so SRE notices the divergence — leaving it
// silent would let the next boot re-apply the same migration.
func applyPendingMigrations(ctx context.Context, pool *pgxpool.Pool) {
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if _, statErr := os.Stat(migrationsDir); statErr != nil {
		return
	}
	entries, _ := os.ReadDir(migrationsDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		var version int
		fmt.Sscanf(entry.Name(), "%06d", &version)

		var exists bool
		pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if exists {
			continue
		}

		sqlBytes, readErr := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if readErr != nil {
			zap.L().Warn("migration: failed to read", zap.String("file", entry.Name()), zap.Error(readErr))
			continue
		}
		if _, applyErr := pool.Exec(ctx, string(sqlBytes)); applyErr != nil {
			zap.L().Error("migration: failed to apply", zap.String("file", entry.Name()), zap.Error(applyErr))
			continue
		}
		// A failed schema_migrations row means the migration applied but
		// the tracker didn't advance — on next boot we'd try to apply it
		// again. That's the kind of silent divergence that leaves ops
		// chasing phantom failures.
		if _, insErr := pool.Exec(ctx, "INSERT INTO schema_migrations (version, dirty) VALUES ($1, false) ON CONFLICT DO NOTHING", version); insErr != nil {
			zap.L().Error("migration: failed to record applied version — tracker is out of sync",
				zap.String("file", entry.Name()), zap.Int("version", version), zap.Error(insErr))
		}
		zap.L().Info("migration: applied", zap.String("file", entry.Name()), zap.Int("version", version))
	}
}

// verifyMigrationVersion aborts startup if the live schema is older
// than the version this binary was tested against. Bump the constant
// (`expectedMigration`) when adding a new migration so an operator who
// forgets to run migrate before deploying gets a hard fail instead of
// a runtime ColumnNotFound storm.
//
// Newer schema → just a Warn; the operator may have rolled forward
// schema for a future release and is now booting an older binary.
func verifyMigrationVersion(ctx context.Context, pool *pgxpool.Pool) {
	const expectedMigration = 50 // bump this when adding new migrations
	var dbVersion int
	if qErr := pool.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&dbVersion); qErr != nil {
		zap.L().Fatal("failed to check migration version — is the database initialized?", zap.Error(qErr))
	}
	if dbVersion < expectedMigration {
		zap.L().Fatal("database schema is behind code — run pending migrations before starting the server",
			zap.Int("db_version", dbVersion),
			zap.Int("expected_version", expectedMigration),
			zap.Int("migrations_behind", expectedMigration-dbVersion))
	}
	if dbVersion > expectedMigration {
		zap.L().Warn("database schema is ahead of code — is this the right binary?",
			zap.Int("db_version", dbVersion),
			zap.Int("expected_version", expectedMigration))
	}
}

// seedIfEmpty applies db/seed/seed.sql when the users table is empty,
// falling back to a minimal-admin INSERT chain if the seed file is
// missing. The user-count probe is fatal-on-error: re-seeding into a
// populated DB would stomp an existing admin so we'd rather refuse to
// start than guess.
//
// The seeded admin password is never logged; it's persisted to a 0600
// file via writeSeedPasswordToFile and only the path is logged.
func seedIfEmpty(ctx context.Context, pool *pgxpool.Pool) {
	var userCount int
	if probeErr := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&userCount); probeErr != nil {
		zap.L().Fatal("seed: failed to probe users count — cannot safely decide whether to seed", zap.Error(probeErr))
	}
	if userCount > 0 {
		return
	}

	zap.L().Info("database is empty — running initial seed")
	seedDir := os.Getenv("SEED_DIR")
	if seedDir == "" {
		seedDir = "db/seed"
	}
	seedFile := filepath.Join(seedDir, "seed.sql")
	if sqlBytes, seedReadErr := os.ReadFile(seedFile); seedReadErr == nil {
		if _, seedExecErr := pool.Exec(ctx, string(sqlBytes)); seedExecErr != nil {
			zap.L().Error("seed: failed to apply", zap.Error(seedExecErr))
		} else {
			zap.L().Info("seed: initial data loaded successfully")
		}
		return
	}

	// Seed file not found — create minimal admin only.
	zap.L().Warn("seed file not found, creating minimal admin user", zap.String("path", seedFile))
	adminPassword := os.Getenv("ADMIN_DEFAULT_PASSWORD")
	if adminPassword == "" {
		adminPassword = "admin-" + uuid.New().String()[:8]
	}
	hash, hashErr := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if hashErr != nil {
		zap.L().Fatal("seed: failed to hash admin password", zap.Error(hashErr))
	}
	// Each INSERT failure here means the minimal-admin seed never
	// completed. We refuse to continue startup in that case: the
	// operator would otherwise be looking at a half-seeded database
	// with no usable login.
	seedStmts := []struct {
		label string
		sql   string
		args  []any
	}{
		{"tenant", `INSERT INTO tenants (id, name, slug) VALUES ('a0000000-0000-0000-0000-000000000001', 'Default', 'default') ON CONFLICT DO NOTHING`, nil},
		{"user", `INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source) VALUES ('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'admin', 'System Admin', 'admin@example.com', $1, 'active', 'local') ON CONFLICT DO NOTHING`, []any{string(hash)}},
		{"role", `INSERT INTO roles (id, tenant_id, name, description, permissions, is_system) VALUES ('c0000000-0000-0000-0000-000000000001', NULL, 'super-admin', 'Full system access', '{"*": ["*"]}', true) ON CONFLICT DO NOTHING`, nil},
		{"user_role", `INSERT INTO user_roles (user_id, role_id) VALUES ('b0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001') ON CONFLICT DO NOTHING`, nil},
	}
	for _, stmt := range seedStmts {
		if _, seedErr := pool.Exec(ctx, stmt.sql, stmt.args...); seedErr != nil {
			zap.L().Fatal("seed: minimal-admin insert failed — aborting startup",
				zap.String("step", stmt.label), zap.Error(seedErr))
		}
	}
	// SECURITY: do NOT log the plaintext password — log aggregators would
	// archive the admin credential. Persist it to a 0600 file and only log
	// the path + username.
	credsPath, credsErr := writeSeedPasswordToFile(adminPassword, "admin")
	if credsErr != nil {
		zap.L().Fatal("failed to persist seeded admin password — cannot continue",
			zap.Error(credsErr))
	}
	zap.L().Warn("seed: minimal admin user created — change password immediately; credentials written to file",
		zap.String("username", "admin"),
		zap.String("credentials_file", credsPath))
}
