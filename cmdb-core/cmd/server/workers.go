// workers.go — background goroutines launched at startup, extracted
// from main.go during the Phase 2 God-file split (2026-04-28).
//
// Each worker derives its lifecycle from the root server context, so
// a single SIGTERM unwinds the entire stack: HTTP server, MCP server,
// WebSocket hub, NATS subscribers, alert evaluator, workflow tickers,
// and webhook dispatcher. main() only needs to cancel ctx and wait.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/domain/workflows"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	cmdbmcp "github.com/cmdb-platform/cmdb-core/internal/mcp"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	cmdbws "github.com/cmdb-platform/cmdb-core/internal/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

// startBackgroundWorkers launches every long-running side-channel a
// running cmdb-core needs. None of these block; each spawns its own
// goroutine and listens on ctx.Done() for shutdown.
//
// wsHub may be nil (cfg.WSEnabled == false) — in that case the
// NATS→WS bridge is silently skipped.
func startBackgroundWorkers(
	ctx context.Context,
	cfg *config.Config,
	pool *pgxpool.Pool,
	queries *dbgen.Queries,
	bus eventbus.Bus,
	cipher crypto.Cipher,
	netGuard *netguard.Guard,
	svcs *appServices,
	wsHub *cmdbws.Hub,
) {
	startMCPServer(ctx, cfg, queries)
	startWebSocketHub(ctx, wsHub, bus)
	registerWorkflowSubscribers(ctx, pool, queries, bus, cipher, svcs)
	go svcs.alertEvaluator.Start(ctx)
	zap.L().Info("Alert rule evaluator launched")
	startWebhookDispatcher(ctx, queries, bus, cipher, netGuard)
}

// startMCPServer spins up the optional MCP listener as its own
// http.Server (separate from the main Gin server) when cfg.MCPEnabled
// is true. The listener is shut down with a bounded timeout when the
// server context cancels — leaving it would orphan the goroutine.
func startMCPServer(ctx context.Context, cfg *config.Config, queries *dbgen.Queries) {
	if !cfg.MCPEnabled {
		return
	}

	mcpSrv := cmdbmcp.New(queries)
	sseServer := mcpserver.NewSSEServer(mcpSrv.Server())

	// Wrap with API key auth if configured. No header → 401 before
	// the SSE handler sees the request.
	var mcpHandler http.Handler = sseServer
	if cfg.MCPApiKey != "" {
		mcpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHdr := r.Header.Get("Authorization")
			if authHdr != "Bearer "+cfg.MCPApiKey {
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
		// Bounded shutdown so the listener actually returns instead
		// of pinning the goroutine on a slow client.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = mcpHTTPSrv.Shutdown(shutdownCtx)
	}()
}

// startWebSocketHub launches the WS hub goroutine and wires the
// NATS→WS broadcast bridge. Both no-op when wsHub is nil.
func startWebSocketHub(ctx context.Context, wsHub *cmdbws.Hub, bus eventbus.Bus) {
	if wsHub == nil {
		return
	}
	go wsHub.Run(ctx)
	zap.L().Info("WebSocket hub started")

	// NATS → WebSocket bridge. Only the topics most operators want
	// pushed live; webhook deliveries handle the rest.
	if bus != nil {
		subjects := []string{"alert.>", "asset.>", "maintenance.>", "import.>", "notification.>"}
		for _, subj := range subjects {
			subj := subj // capture
			bus.Subscribe(subj, func(_ context.Context, event eventbus.Event) error {
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
}

// registerWorkflowSubscribers wires the cross-module reactions
// (auto-WO openers, sync reconcilers, audit partition sampler, etc.)
// AND the dashboard cache invalidator. Both are bus subscribers so
// they no-op when bus is nil.
func registerWorkflowSubscribers(
	ctx context.Context,
	pool *pgxpool.Pool,
	queries *dbgen.Queries,
	bus eventbus.Bus,
	cipher crypto.Cipher,
	svcs *appServices,
) {
	if bus == nil {
		return
	}
	// Workflow subscribers: Register() wires the event-bus handlers;
	// StartAll spawns every background loop. Phase 4.1 consolidated
	// the 8 individual Start* calls behind StartAll — see
	// workflows/start.go for the full list and rationale.
	wfSub := workflows.New(pool, queries, bus, svcs.maintenanceSvc, cipher).
		WithQualityScanner(svcs.qualitySvc).
		WithSchedHealth(svcs.schedTracker)
	wfSub.Register()
	wfSub.StartAll(ctx)

	// Dashboard cache invalidator. Subscribes to asset/rack/alert/
	// order events so the next GetStats call sees fresh numbers
	// instead of waiting out the 60-second Redis TTL.
	dashInval := dashboard.NewInvalidationSubscriber(svcs.dashboardSvc, bus, nil)
	if err := dashInval.Start(); err != nil {
		zap.L().Warn("dashboard invalidation subscribe failed", zap.Error(err))
	}
}

// startWebhookDispatcher registers the user-defined webhook
// dispatcher as a NATS subscriber. Each fan-out delivery goroutine
// derives from the server ctx via WithBaseContext, so SIGTERM cancels
// in-flight retries (including the 1s / 5s backoff sleeps) instead
// of pinning them until the per-request HTTP timeout fires.
func startWebhookDispatcher(
	ctx context.Context,
	queries *dbgen.Queries,
	bus eventbus.Bus,
	cipher crypto.Cipher,
	netGuard *netguard.Guard,
) {
	if bus == nil {
		return
	}
	dispatcher := integration.NewWebhookDispatcher(queries, cipher, netGuard).
		WithEventBus(bus).
		WithBaseContext(ctx)
	webhookSubjects := []string{"asset.>", "maintenance.>", "alert.>", "prediction.>"}
	for _, subj := range webhookSubjects {
		subj := subj
		_ = bus.Subscribe(subj, dispatcher.HandleEvent)
	}
	zap.L().Info("Webhook dispatcher active")
}
