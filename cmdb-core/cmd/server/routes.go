package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/config"
	cmdbmcp "github.com/cmdb-platform/cmdb-core/internal/mcp"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	cmdbws "github.com/cmdb-platform/cmdb-core/internal/websocket"
	"github.com/gin-gonic/gin"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

// buildRouter constructs the full gin engine: infra middleware, public
// health/metrics routes, and the /api/v1 group with auth, rate limiting,
// RBAC, all registered handlers, the admin status migration route, and
// the WebSocket endpoint.
func buildRouter(a *appState, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	infraMiddleware(router)
	registerPublicRoutes(router, a.healthHandler)
	registerV1Group(router, a, cfg)

	return router
}

// infraMiddleware installs the per-request middleware every route must run
// through, in dependency order. Tracing comes first so spans wrap every
// subsequent middleware. Recovery is next so a panic in any later middleware
// still produces a 500 instead of crashing the process. Prometheus middleware
// comes last so it measures the fully-wrapped request duration.
func infraMiddleware(router *gin.Engine) {
	router.Use(telemetry.TracingMiddleware("cmdb-core"))
	router.Use(
		middleware.Recovery(),
		middleware.CORS(),
		middleware.SecurityHeaders(),
		middleware.RequestID(),
	)
	router.Use(telemetry.PrometheusMiddleware())
}

// registerPublicRoutes mounts health probes and the Prometheus scrape target.
// These must be reachable without any /api/v1 machinery.
func registerPublicRoutes(router *gin.Engine, health *api.HealthHandler) {
	router.GET("/healthz", health.Liveness)
	router.GET("/readyz", health.Readiness)
	router.GET("/metrics", telemetry.MetricsHandler())
}

// registerV1Group wires the /api/v1 group: auth bypass, login rate limiter,
// optional global rate limiter, RBAC, all generated handlers, the admin
// status migration endpoint, and the WebSocket upgrade endpoint.
func registerV1Group(router *gin.Engine, a *appState, cfg *config.Config) {
	v1 := router.Group("/api/v1")

	authMW := middleware.Auth(
		cfg.JWTSecret,
		middleware.WithBlacklist(a.blacklist),
		middleware.WithPasswordChangeChecker(a.authSvc),
	)
	authBypass := middleware.AuthBypassPaths()
	v1.Use(func(c *gin.Context) {
		if _, ok := authBypass[c.Request.URL.Path]; ok {
			c.Next()
			return
		}
		authMW(c)
	})

	// Per-IP rate limit for login and refresh only.
	// Budget: 20/min/IP — blocks credential-stuffing without harming legit
	// retries behind shared NAT. Override via LOGIN_RATE_PER_MIN.
	loginLimiter := middleware.NewIPRateLimiter(envIntOr("LOGIN_RATE_PER_MIN", 20))
	loginLimiterMW := loginLimiter.Middleware()
	v1.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/api/v1/auth/login" || path == "/api/v1/auth/refresh" {
			loginLimiterMW(c)
		}
	})

	if cfg.RateLimitEnabled {
		rl := middleware.NewRateLimiter(middleware.RateLimiterConfig{
			RequestsPerSecond: cfg.RateLimitRPS,
			Burst:             cfg.RateLimitBurst,
			IdleTTL:           10 * time.Minute,
		})
		a.rateLimiter = rl
		v1.Use(rl.Middleware())
		zap.L().Info("Rate limiting enabled",
			zap.Float64("rps", cfg.RateLimitRPS),
			zap.Int("burst", cfg.RateLimitBurst))
	}

	v1.Use(middleware.RBAC(a.queries, a.redisClient))

	api.RegisterHandlers(v1, a.apiServer)

	// One-time data migration: draft/pending → submitted (admin-only, not in spec).
	// Cross-tenant by design — this is an operator-initiated catch-up for legacy
	// rows produced before the work_orders status vocabulary was tightened, so
	// it intentionally bypasses tenant scoping. Admin endpoint, RBAC-gated above.
	v1.POST("/admin/migrate-statuses", func(c *gin.Context) {
		//tenantlint:allow-direct-pool
		res1, err1 := a.pool.Exec(c.Request.Context(), "UPDATE work_orders SET status = 'submitted' WHERE status IN ('draft', 'pending')")
		if err1 != nil {
			zap.L().Error("admin migrate-statuses: submitted update failed", zap.Error(err1))
			c.JSON(500, gin.H{"error": "submitted migration failed", "detail": err1.Error()})
			return
		}
		//tenantlint:allow-direct-pool
		res2, err2 := a.pool.Exec(c.Request.Context(), "UPDATE work_orders SET status = 'verified' WHERE status = 'closed'")
		if err2 != nil {
			zap.L().Error("admin migrate-statuses: verified update failed", zap.Error(err2))
			c.JSON(500, gin.H{"error": "verified migration failed", "detail": err2.Error()})
			return
		}
		c.JSON(200, gin.H{"migrated_to_submitted": res1.RowsAffected(), "migrated_to_verified": res2.RowsAffected()})
	})

	if cfg.WSEnabled && a.wsHub != nil {
		wsAuthMW := middleware.WSAuth(cfg.JWTSecret)
		v1.GET("/ws", wsAuthMW, cmdbws.HandleWS(a.wsHub))
	}
}

// startMCPServer starts the MCP SSE server on cfg.MCPPort and shuts it down
// when ctx is cancelled. It is a no-op when cfg.MCPEnabled is false.
func startMCPServer(ctx context.Context, a *appState, cfg *config.Config) {
	if !cfg.MCPEnabled {
		return
	}
	mcpSrv := cmdbmcp.New(a.queries)
	sseServer := mcpserver.NewSSEServer(mcpSrv.Server())

	var mcpHandler http.Handler = sseServer
	if cfg.MCPApiKey != "" {
		mcpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+cfg.MCPApiKey {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			sseServer.ServeHTTP(w, r)
		})
		zap.L().Info("MCP Server auth enabled")
	}

	mcpHTTPSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MCPPort),
		Handler: mcpHandler,
	}
	go func() {
		zap.L().Info("MCP Server starting", zap.String("addr", mcpHTTPSrv.Addr))
		if err := mcpHTTPSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Error("MCP Server error", zap.Error(err))
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = mcpHTTPSrv.Shutdown(shutdownCtx)
	}()
}
