package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/workflows"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/cache"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"go.uber.org/zap"
	// config is imported transitively via the helpers in services.go etc;
	// main() only consumes config.Load() through the cfg variable.
	"github.com/cmdb-platform/cmdb-core/internal/config"
)


//tenantlint:allow-direct-pool — server bootstrap: seed data and schema init run outside request context

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		// Logger not available yet; use zap's built-in must-style.
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	// 2. Structured logger
	logger, err := telemetry.NewLogger(cfg.LogLevel)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	// 2a. Validate JWT signing secret strength before we accept any traffic.
	// A weak/short secret lets attackers forge arbitrary tokens, so treat
	// this as a hard startup failure rather than a warning.
	if jwtErr := validateJWTSecret(cfg.JWTSecret); jwtErr != nil {
		zap.L().Fatal("invalid JWT secret", zap.Error(jwtErr))
	}

	// Root server context — cancelled on SIGINT/SIGTERM. Every background
	// worker (alert evaluator, workflow tickers, webhook dispatcher
	// fan-out, sync reconciler, WS hub, MCP server, etc.) derives its
	// lifecycle from this ctx so a single signal unwinds the whole stack.
	// Per-request handlers keep using c.Request.Context(), which is
	// scoped to the HTTP request and unrelated to shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 3. OpenTelemetry tracing
	// Init/shutdown use a fresh context so a cancelled server ctx cannot
	// short-circuit tracer shutdown before spans are flushed.
	tracerInitCtx, tracerInitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer tracerInitCancel()
	shutdownTracer, err := telemetry.InitTracer(tracerInitCtx, cfg.OTELEndpoint, "cmdb-core", "1.0.0")
	if err != nil {
		zap.L().Fatal("failed to init tracer", zap.Error(err))
	}
	defer func() {
		tracerShutdownCtx, tracerShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer tracerShutdownCancel()
		_ = shutdownTracer(tracerShutdownCtx)
	}()

	// 3a. Load at-rest encryption key ring. Missing key is a hard failure —
	// we never want to silently run without encrypting adapter configs or
	// webhook secrets. Operators set CMDB_SECRET_KEY_V1..V{N} (64-char hex
	// each, 32-byte AES-256 keys; generate with crypto.GenerateKeyHex). The
	// legacy single CMDB_SECRET_KEY env var is still honoured as v1 when no
	// versioned vars are set, so existing deployments upgrade unchanged.
	keyring, err := crypto.KeyRingFromEnv()
	if err != nil {
		zap.L().Fatal("failed to load at-rest encryption key ring (set CMDB_SECRET_KEY or CMDB_SECRET_KEY_V{N})", zap.Error(err))
	}
	// Downstream call sites continue to take crypto.Cipher — the KeyRing
	// satisfies that interface, so they don't need to know about rotation.
	var cipher crypto.Cipher = keyring
	zap.L().Info("at-rest encryption configured",
		zap.Int("active_version", keyring.ActiveVersion()),
		zap.Ints("available_versions", keyring.Versions()))

	// 4. Create PG pool
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		zap.L().Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	// 4a. Auto-run pending migrations + verify schema version
	applyPendingMigrations(ctx, pool)
	verifyMigrationVersion(ctx, pool)

	// 5. Create dbgen.Queries from the pool
	queries := dbgen.New(pool)

	// 5a. Auto-seed default tenant + admin if DB is empty
	seedIfEmpty(ctx, pool)

	// 6. Create Redis client
	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		zap.L().Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	// 7. Create NATS event bus (log warning if not available, set bus to nil)
	var bus eventbus.Bus
	natsBus, err := eventbus.NewNATSBus(cfg.NatsURL)
	if err != nil {
		zap.L().Warn("NATS not available, event bus disabled", zap.Error(err))
	} else {
		bus = natsBus
		defer natsBus.Close()
	}

	// 7b. Build SSRF guard from config — outbound integration calls,
	// webhook deliveries, and custom REST adapters all route their URLs
	// through this guard to block loopback / RFC1918 / cloud-metadata
	// targets. Admin allowlist (CMDB_INTEGRATION_ALLOWED_HOSTS) bypasses
	// for intentional on-prem integrations.
	netGuard, err := netguard.New(nil, cfg.IntegrationAllowedOutboundHosts)
	if err != nil {
		zap.L().Fatal("failed to build netguard", zap.Error(err))
	}
	workflows.SetNetGuard(netGuard)
	zap.L().Info("SSRF outbound guard configured",
		zap.Int("allow_hosts", len(cfg.IntegrationAllowedOutboundHosts)))

	// 8. Build all domain services + alert evaluator + predictive scheduler.
	// See services.go for the full wire-up; the appServices struct keeps
	// downstream consumers (api.NewAPIServer, router middleware, workflow
	// subscribers) honest by giving each dependency a single source.
	svcs := buildServices(ctx, cfg, pool, queries, redisClient, bus)

	// 9. Create unified API server
	apiServer := api.NewAPIServer(
		pool, cfg, bus, redisClient, natsBus,
		svcs.authSvc, svcs.identitySvc, svcs.topologySvc, svcs.assetSvc, svcs.maintenanceSvc,
		svcs.monitoringSvc, svcs.inventorySvc, svcs.auditSvc, svcs.dashboardSvc, svcs.predictionSvc,
		svcs.integrationSvc, svcs.biaSvc, svcs.qualitySvc, svcs.discoverySvc, svcs.locationDetectSvc,
		svcs.serviceSvc, svcs.predictiveSvc, svcs.metricSourceSvc, svcs.schedTracker, cipher, netGuard,
	)

	// 9a. Load and freeze RBAC routing config (publicPaths, resourceMap)
	// BEFORE wiring any middleware. Fail-closed: invalid config is a hard
	// startup failure, never a runtime 403 storm. See
	// docs/reports/phase4/4.9-rbac-config-externalization.md.
	rbacCfg, err := middleware.LoadRBACConfig("")
	if err != nil {
		zap.L().Fatal("failed to load rbac config", zap.Error(err))
	}
	middleware.ConfigureRBAC(rbacCfg)
	zap.L().Info("rbac config loaded",
		zap.Int("public_paths", len(rbacCfg.PublicPaths)),
		zap.Int("resource_map_entries", len(rbacCfg.ResourceMap)),
	)

	// 10. Set up Gin router. See router_setup.go for the middleware chain
	// + handler registration; the wsHub it returns is wired into the
	// NATS broadcast bridge by startBackgroundWorkers below.
	rs := setupRouter(cfg, pool, queries, redisClient, natsBus, svcs, apiServer)
	if rs.Stop != nil {
		defer rs.Stop()
	}

	// 11. Launch every background goroutine (MCP listener, WS hub run-loop,
	// NATS→WS bridge, workflow subscribers, alert evaluator, webhook
	// dispatcher). All derive their lifecycle from ctx so SIGTERM stops
	// the whole stack atomically.
	startBackgroundWorkers(ctx, cfg, pool, queries, bus, cipher, netGuard, svcs, rs.WSHub)
	router := rs.Engine

	// 11. Start HTTP server with graceful shutdown
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	go func() {
		zap.L().Info("starting cmdb-core", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("server error", zap.Error(err))
		}
	}()

	// Wait for SIGINT/SIGTERM. signal.NotifyContext cancels ctx on signal
	// receipt, so every background worker (evaluator, workflow tickers,
	// webhook dispatcher, sync reconciler, WS hub, MCP server) starts
	// exiting through its own ctx.Done() case immediately.
	<-ctx.Done()

	zap.L().Info("shutting down server...")

	// The HTTP server uses a *fresh* timeout context so in-flight requests
	// have a full 30s to drain even though the server ctx is already
	// cancelled. Background goroutines observe the cancelled ctx in
	// parallel and unwind on their own.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("server forced to shutdown", zap.Error(err))
	}

	zap.L().Info("server exited gracefully")
}

// Helpers (envIntOr, runPredictiveScheduler, lateBoundRecorder) moved
// to helpers.go as part of the Phase 2 main.go split (2026-04-28).
