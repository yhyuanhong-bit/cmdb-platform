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
	"github.com/cmdb-platform/cmdb-core/internal/domain/sync"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/domain/workflows"
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

	// 4b. Verify database migration version matches code expectations
	{
		const expectedMigration = 31 // bump this when adding new migrations
		var dbVersion int
		err := pool.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&dbVersion)
		if err != nil {
			zap.L().Fatal("failed to check migration version — is the database initialized?", zap.Error(err))
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
	authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret, pool)
	identitySvc := identity.NewService(queries)
	topologySvc := topology.NewService(queries, pool)
	assetSvc := asset.NewService(queries, bus, pool)
	maintenanceSvc := maintenance.NewService(queries, bus, pool)
	monitoringSvc := monitoring.NewService(queries, bus)
	inventorySvc := inventory.NewService(queries)
	auditSvc := audit.NewService(queries)
	dashboardSvc := dashboard.NewService(queries, pool, redisClient)

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

	// 8b. Sync service
	var syncSvc *sync.Service
	if cfg.SyncEnabled && bus != nil {
		syncSvc = sync.NewService(pool, bus, cfg)
		syncSvc.RegisterSubscribers()
		syncSvc.StartReconciliation(ctx)
		zap.L().Info("Sync service started")

		if cfg.DeployMode == "edge" && cfg.EdgeNodeID != "" {
			agent := sync.NewAgent(pool, bus, cfg)
			go agent.Start(ctx)
			zap.L().Info("Sync agent started", zap.String("node_id", cfg.EdgeNodeID))
		}
	}

	// 9. Create unified API server
	apiServer := api.NewAPIServer(
		pool, cfg, bus, authSvc, identitySvc, topologySvc, assetSvc, maintenanceSvc,
		monitoringSvc, inventorySvc, auditSvc, dashboardSvc, predictionSvc,
		integrationSvc, biaSvc, qualitySvc, discoverySvc, syncSvc,
	)

	// 10. Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Tracing middleware first so spans wrap everything
	router.Use(telemetry.TracingMiddleware("cmdb-core"))
	router.Use(middleware.Recovery(), middleware.CORS(), middleware.SecurityHeaders(), middleware.RequestID())
	router.Use(telemetry.PrometheusMiddleware())

	// Health & readiness probes
	healthHandler := api.NewHealthHandler(pool, redisClient)
	router.GET("/healthz", healthHandler.Liveness)
	router.GET("/readyz", healthHandler.Readiness)

	// Prometheus metrics endpoint (no auth)
	router.GET("/metrics", telemetry.MetricsHandler())

	// API v1 group with auth middleware that skips public endpoints
	v1 := router.Group("/api/v1")
	authMW := middleware.Auth(cfg.JWTSecret)
	v1.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/api/v1/auth/login" || path == "/api/v1/auth/refresh" || path == "/api/v1/ws" {
			c.Next()
			return
		}
		authMW(c)
	})
	v1.Use(middleware.RBAC(queries, redisClient))

	// Register all API routes via generated handler
	api.RegisterHandlers(v1, apiServer)

	// Energy monitoring endpoints
	v1.GET("/energy/breakdown", apiServer.GetEnergyBreakdown)
	v1.GET("/energy/summary", apiServer.GetEnergySummary)
	v1.GET("/energy/trend", apiServer.GetEnergyTrend)

	// Custom endpoints (Phase 2)
	v1.GET("/racks/stats", apiServer.GetRackStats)
	v1.GET("/assets/lifecycle-stats", apiServer.GetAssetLifecycleStats)
	v1.GET("/monitoring/alerts/trend", apiServer.GetAlertsTrend)
	v1.GET("/racks/:id/maintenance", apiServer.GetRackMaintenance)

	// Custom inventory endpoints (not in generated spec)
	v1.GET("/inventory/tasks/:id/racks-summary", apiServer.GetInventoryRacksSummary)
	v1.GET("/inventory/tasks/:id/discrepancies", apiServer.GetInventoryDiscrepancies)

	// Phase 3 routes
	v1.GET("/inventory/tasks/:id/items/:itemId/scan-history", apiServer.GetItemScanHistory)
	v1.POST("/inventory/tasks/:id/items/:itemId/scan-history", apiServer.CreateItemScanRecord)
	v1.GET("/inventory/tasks/:id/items/:itemId/notes", apiServer.GetItemNotes)
	v1.POST("/inventory/tasks/:id/items/:itemId/notes", apiServer.CreateItemNote)
	v1.POST("/inventory/tasks/:id/items/:itemId/resolve", apiServer.ResolveInventoryDiscrepancy)
	v1.GET("/maintenance/orders/:id/comments", apiServer.GetWorkOrderComments)
	v1.POST("/maintenance/orders/:id/comments", apiServer.CreateWorkOrderComment)
	v1.GET("/topology/dependencies", apiServer.GetAssetDependencies)
	v1.POST("/topology/dependencies", apiServer.CreateAssetDependency)
	v1.DELETE("/topology/dependencies/:id", apiServer.DeleteAssetDependency)
	v1.GET("/topology/graph", apiServer.GetTopologyGraph)
	v1.GET("/racks/:id/network-connections", apiServer.GetRackNetworkConnections)
	v1.POST("/racks/:id/network-connections", apiServer.CreateRackNetworkConnection)
	v1.DELETE("/racks/:id/network-connections/:connectionId", apiServer.DeleteRackNetworkConnection)
	v1.GET("/activity-feed", apiServer.GetActivityFeed)
	v1.GET("/audit/events/:id", apiServer.GetAuditEventDetail)

	// Phase 4 Group 1 routes
	v1.GET("/prediction/rul/:id", apiServer.GetAssetRUL)
	v1.GET("/prediction/failure-distribution", apiServer.GetFailureDistribution)
	v1.GET("/assets/:id/upgrade-recommendations", apiServer.GetAssetUpgradeRecommendations)
	v1.POST("/assets/:id/upgrade-recommendations/:category/accept", apiServer.AcceptUpgradeRecommendation)
	v1.GET("/upgrade-rules", apiServer.GetUpgradeRules)
	v1.POST("/upgrade-rules", apiServer.CreateUpgradeRule)

	// One-time data migration: draft/pending → submitted
	v1.POST("/admin/migrate-statuses", func(c *gin.Context) {
		res1, _ := pool.Exec(c.Request.Context(), "UPDATE work_orders SET status = 'submitted' WHERE status IN ('draft', 'pending')")
		res2, _ := pool.Exec(c.Request.Context(), "UPDATE work_orders SET status = 'verified' WHERE status = 'closed'")
		c.JSON(200, gin.H{"migrated_to_submitted": res1.RowsAffected(), "migrated_to_verified": res2.RowsAffected()})
	})

	// Role assignment + user deletion routes
	v1.GET("/users/:id/roles", apiServer.ListUserRoles)
	v1.POST("/users/:id/roles", apiServer.AssignRoleToUser)
	v1.DELETE("/users/:id/roles/:roleId", apiServer.RemoveRoleFromUser)
	v1.DELETE("/users/:id", apiServer.DeleteUser)

	// Notification routes
	v1.GET("/notifications", apiServer.ListNotifications)
	v1.GET("/notifications/count", apiServer.CountUnreadNotifications)
	v1.POST("/notifications/:id/read", apiServer.MarkNotificationRead)
	v1.POST("/notifications/read-all", apiServer.MarkAllNotificationsRead)

	// Phase 4 Group 2 routes
	v1.GET("/users/:id/sessions", apiServer.GetUserSessions)
	v1.POST("/auth/change-password", apiServer.ChangePassword)
	v1.GET("/sensors", apiServer.ListSensors)
	v1.POST("/sensors", apiServer.CreateSensor)
	v1.PUT("/sensors/:id", apiServer.UpdateSensor)
	v1.DELETE("/sensors/:id", apiServer.DeleteSensor)
	v1.POST("/sensors/:id/heartbeat", apiServer.SensorHeartbeat)

	// Sync endpoints
	v1.GET("/sync/changes", apiServer.SyncGetChanges)
	v1.GET("/sync/state", apiServer.SyncGetState)
	v1.GET("/sync/conflicts", apiServer.SyncGetConflicts)
	v1.POST("/sync/conflicts/:id/resolve", apiServer.SyncResolveConflict)
	v1.GET("/sync/snapshot", apiServer.SyncSnapshot)

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

		// Register WS endpoint with WSAuth (supports Sec-WebSocket-Protocol auth)
		wsAuthMW := middleware.WSAuth(cfg.JWTSecret)
		v1.GET("/ws", wsAuthMW, cmdbws.HandleWS(wsHub))
		zap.L().Info("WebSocket hub started")
	}

	// NATS -> WebSocket bridge
	if bus != nil && wsHub != nil {
		subjects := []string{"alert.>", "asset.>", "maintenance.>", "import.>", "notification.>"}
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

	// Workflow subscribers (cross-module reactions)
	if bus != nil {
		wfSub := workflows.New(pool, queries, bus, maintenanceSvc)
		wfSub.Register()
		wfSub.StartSLAChecker(ctx)
		wfSub.StartSessionCleanup(ctx)
		wfSub.StartConflictAndDiscoveryCleanup(ctx)
		wfSub.StartMetricsPuller(ctx)
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
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
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
