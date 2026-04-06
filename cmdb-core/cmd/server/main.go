package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/audit"
	"github.com/cmdb-platform/cmdb-core/internal/domain/bia"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	cmdbmcp "github.com/cmdb-platform/cmdb-core/internal/mcp"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/cache"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	cmdbws "github.com/cmdb-platform/cmdb-core/internal/websocket"
	"github.com/gin-gonic/gin"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

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

	ctx := context.Background()

	// 3. OpenTelemetry tracing
	shutdownTracer, err := telemetry.InitTracer(ctx, cfg.OTELEndpoint, "cmdb-core", "1.0.0")
	if err != nil {
		zap.L().Fatal("failed to init tracer", zap.Error(err))
	}
	defer shutdownTracer(ctx)

	// 4. Create PG pool
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		zap.L().Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	// 5. Create dbgen.Queries from the pool
	queries := dbgen.New(pool)

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

	// 8. Create all services
	authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret)
	identitySvc := identity.NewService(queries)
	topologySvc := topology.NewService(queries)
	assetSvc := asset.NewService(queries, bus)
	maintenanceSvc := maintenance.NewService(queries, bus)
	monitoringSvc := monitoring.NewService(queries)
	inventorySvc := inventory.NewService(queries)
	auditSvc := audit.NewService(queries)
	dashboardSvc := dashboard.NewService(queries)

	integrationSvc := integration.NewService(queries)
	biaSvc := bia.NewService(queries)
	qualitySvc := quality.NewService(queries)
	discoverySvc := discovery.NewService(queries)

	// AI Registry
	aiRegistry := ai.NewRegistry()
	if err := aiRegistry.LoadFromDB(ctx, &ai.QueriesAdapter{Q: queries}); err != nil {
		zap.L().Warn("failed to load AI models", zap.Error(err))
	}

	// Prediction
	predictionSvc := prediction.NewService(queries, aiRegistry)

	// 9. Create unified API server
	apiServer := api.NewAPIServer(
		pool, bus, authSvc, identitySvc, topologySvc, assetSvc, maintenanceSvc,
		monitoringSvc, inventorySvc, auditSvc, dashboardSvc, predictionSvc,
		integrationSvc, biaSvc, qualitySvc, discoverySvc,
	)

	// 10. Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Tracing middleware first so spans wrap everything
	router.Use(telemetry.TracingMiddleware("cmdb-core"))
	router.Use(middleware.Recovery(), middleware.CORS(), middleware.RequestID())
	router.Use(telemetry.PrometheusMiddleware())

	// Health check
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Prometheus metrics endpoint (no auth)
	router.GET("/metrics", telemetry.MetricsHandler())

	// API v1 group with auth middleware that skips public endpoints
	v1 := router.Group("/api/v1")
	authMW := middleware.Auth(cfg.JWTSecret)
	v1.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/api/v1/auth/login" || path == "/api/v1/auth/refresh" {
			c.Next()
			return
		}
		authMW(c)
	})
	v1.Use(middleware.RBAC(queries, redisClient))

	// Register all API routes via generated handler
	api.RegisterHandlers(v1, apiServer)

	// Protected sub-group for non-API routes that need auth (e.g. WebSocket)
	protected := v1.Group("", authMW)

	// MCP Server
	if cfg.MCPEnabled {
		mcpSrv := cmdbmcp.New(queries)
		sseServer := mcpserver.NewSSEServer(mcpSrv.Server())

		// Wrap with API key auth if configured
		var mcpHandler http.Handler = sseServer
		if cfg.MCPApiKey != "" {
			mcpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer "+cfg.MCPApiKey {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}
				sseServer.ServeHTTP(w, r)
			})
			zap.L().Info("MCP Server auth enabled")
		}

		go func() {
			addr := fmt.Sprintf(":%d", cfg.MCPPort)
			zap.L().Info("MCP Server starting", zap.String("addr", addr))
			if err := http.ListenAndServe(addr, mcpHandler); err != nil {
				zap.L().Error("MCP Server error", zap.Error(err))
			}
		}()
	}

	// WebSocket Hub
	var wsHub *cmdbws.Hub
	if cfg.WSEnabled {
		wsHub = cmdbws.NewHub()
		go wsHub.Run(ctx)

		// Register WS endpoint (needs auth)
		protected.GET("/ws", cmdbws.HandleWS(wsHub))
		zap.L().Info("WebSocket hub started")
	}

	// NATS -> WebSocket bridge
	if bus != nil && wsHub != nil {
		subjects := []string{"alert.>", "asset.>", "maintenance.>", "import.>"}
		for _, subj := range subjects {
			subj := subj // capture
			bus.Subscribe(subj, func(ctx context.Context, event eventbus.Event) error {
				wsHub.Broadcast(cmdbws.BroadcastMessage{
					TenantID: event.TenantID,
					Type:     event.Subject,
					Payload:  event.Payload,
				})
				return nil
			})
		}
		zap.L().Info("NATS -> WebSocket bridge active")
	}

	// Webhook dispatcher
	if bus != nil {
		dispatcher := integration.NewWebhookDispatcher(queries)
		webhookSubjects := []string{"asset.>", "maintenance.>", "alert.>", "prediction.>"}
		for _, subj := range webhookSubjects {
			subj := subj
			_ = bus.Subscribe(subj, dispatcher.HandleEvent)
		}
		zap.L().Info("Webhook dispatcher active")
	}

	// 11. Start HTTP server with graceful shutdown
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		zap.L().Info("starting cmdb-core", zap.String("addr", addr), zap.String("deploy_mode", cfg.DeployMode))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zap.L().Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zap.L().Fatal("server forced to shutdown", zap.Error(err))
	}

	zap.L().Info("server exited gracefully")
}
