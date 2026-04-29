package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"go.uber.org/zap"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	// 2. Structured logger
	logger, err := telemetry.NewLogger(cfg.LogLevel)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	// 2a. Validate JWT signing secret strength before accepting any traffic.
	if jwtErr := validateJWTSecret(cfg.JWTSecret); jwtErr != nil {
		zap.L().Fatal("invalid JWT secret", zap.Error(jwtErr))
	}

	// Root server context — cancelled on SIGINT/SIGTERM. Every background
	// worker derives its lifecycle from this ctx so a single signal unwinds
	// the whole stack.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 3. OpenTelemetry tracing
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

	// 3a. Load at-rest encryption key ring.
	keyring, err := crypto.KeyRingFromEnv()
	if err != nil {
		zap.L().Fatal("failed to load at-rest encryption key ring (set CMDB_SECRET_KEY or CMDB_SECRET_KEY_V{N})", zap.Error(err))
	}
	var cipher crypto.Cipher = keyring
	zap.L().Info("at-rest encryption configured",
		zap.Int("active_version", keyring.ActiveVersion()),
		zap.Ints("available_versions", keyring.Versions()))

	// 4. Bootstrap all infrastructure, services, and background workers.
	app, err := bootstrap(ctx, cfg, cipher)
	if err != nil {
		zap.L().Fatal("bootstrap failed", zap.Error(err))
	}
	defer app.pool.Close()
	defer app.redisClient.Close()
	if app.natsBus != nil {
		defer app.natsBus.Close()
	}

	// 5. Build HTTP router (may set app.rateLimiter).
	router := buildRouter(app, cfg)
	if app.rateLimiter != nil {
		defer app.rateLimiter.Stop()
	}

	// 5a. Start MCP server (no-op when MCPEnabled is false).
	startMCPServer(ctx, app, cfg)

	// 6. Start HTTP server with graceful shutdown.
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	go func() {
		zap.L().Info("starting cmdb-core", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	zap.L().Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("server forced to shutdown", zap.Error(err))
	}

	zap.L().Info("server exited gracefully")
}

// envIntOr reads a positive integer from the named env var, returning
// fallback if the var is unset, non-numeric, or non-positive.
func envIntOr(envKey string, fallback int) int {
	raw := os.Getenv(envKey)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		zap.L().Warn("invalid env int, using fallback",
			zap.String("env", envKey),
			zap.String("raw", raw),
			zap.Int("fallback", fallback))
		return fallback
	}
	return n
}
