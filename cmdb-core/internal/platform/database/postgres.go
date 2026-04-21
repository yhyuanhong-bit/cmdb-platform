package database

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PoolOption configures NewPool. Zero options keeps the historical
// behavior (default 500ms slow-query threshold, global zap logger), so
// every existing caller of NewPool(ctx, url) keeps working unchanged.
type PoolOption func(*poolConfig)

type poolConfig struct {
	slowQueryThreshold time.Duration
	logger             *zap.Logger
	// disableSlowQueryTracer is only used by tests that want a bare
	// pool (e.g. to avoid polluting the Prometheus registry counter
	// in counter-sensitive assertions).
	disableSlowQueryTracer bool
}

// WithSlowQueryThreshold overrides the env/default-derived threshold.
// Use for tests or per-command tuning.
func WithSlowQueryThreshold(d time.Duration) PoolOption {
	return func(c *poolConfig) { c.slowQueryThreshold = d }
}

// WithLogger supplies the zap.Logger used by the slow-query tracer.
// When unset, the global zap.L() is used — which is usually correct
// because main() wires the global logger early.
func WithLogger(l *zap.Logger) PoolOption {
	return func(c *poolConfig) { c.logger = l }
}

// WithoutSlowQueryTracer disables the tracer. Intended for tests only.
// Production code should never use this.
func WithoutSlowQueryTracer() PoolOption {
	return func(c *poolConfig) { c.disableSlowQueryTracer = true }
}

// NewPool constructs the application's primary pgxpool.
//
// Behavior:
//   - Parses databaseURL, sets MaxConns=50, MinConns=5.
//   - Attaches a SlowQueryTracer on the ConnConfig so every Query /
//     QueryRow / Exec is timed, and queries above the threshold get a
//     zap.Warn + Prometheus counter bump. The threshold is resolved
//     from the CMDB_SLOW_QUERY_THRESHOLD_MS env var (see
//     ResolveSlowQueryThreshold) unless WithSlowQueryThreshold is
//     supplied.
//   - Pings the pool before returning.
//
// The call signature is variadic — existing callers
// (`database.NewPool(ctx, url)`) keep working with no source change.
func NewPool(ctx context.Context, databaseURL string, opts ...PoolOption) (*pgxpool.Pool, error) {
	pc := poolConfig{
		slowQueryThreshold: ResolveSlowQueryThreshold(),
		logger:             zap.L(),
	}
	for _, opt := range opts {
		opt(&pc)
	}
	// An option explicitly passing 0 should still fall back to the
	// env-derived / default threshold; zero would mean "log every query".
	if pc.slowQueryThreshold <= 0 {
		pc.slowQueryThreshold = ResolveSlowQueryThreshold()
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 50
	cfg.MinConns = 5

	// Tracer wiring: always install otelpgx so trace context from the
	// upstream HTTP/NATS handler flows through every Query/Exec as a
	// child span. Compose with SlowQueryTracer unless explicitly
	// disabled, so both observability layers share one ConnConfig.Tracer
	// slot.
	otelTracer := otelpgx.NewTracer(
		otelpgx.WithTrimSQLInSpanName(),
		otelpgx.WithIncludeQueryParameters(),
	)
	if pc.disableSlowQueryTracer {
		cfg.ConnConfig.Tracer = otelTracer
	} else {
		cfg.ConnConfig.Tracer = NewCompositeTracer(
			otelTracer,
			NewSlowQueryTracer(pc.slowQueryThreshold, pc.logger),
		)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if pc.logger != nil {
		pc.logger.Info("db pool initialized",
			zap.Int32("max_conns", cfg.MaxConns),
			zap.Int32("min_conns", cfg.MinConns),
			zap.Duration("slow_query_threshold", pc.slowQueryThreshold),
			zap.Bool("slow_query_tracer", !pc.disableSlowQueryTracer),
		)
	}
	return pool, nil
}
