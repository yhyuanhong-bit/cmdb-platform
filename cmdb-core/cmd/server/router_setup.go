// router_setup.go — Gin router construction extracted from main.go
// during the Phase 2 God-file split (2026-04-28).
//
// Wires the full middleware chain (auth, login limiter, optional global
// rate limiter, RBAC), registers the generated handler set, and tacks
// on the one-off admin/migrate-statuses route. WebSocket /ws lives
// here too because it needs the v1 group; the WS hub itself is
// returned so workers.go can build the NATS→WS bridge.
package main

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	cmdbws "github.com/cmdb-platform/cmdb-core/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// routerSetup bundles the wired Gin engine plus a few side-channels
// the rest of main() needs:
//   - WSHub: nil unless cfg.WSEnabled. Workers.go uses this to bridge
//     NATS events into broadcast frames.
//   - Stop: cleanup function (currently the rate limiter's Stop) or
//     nil. Caller should `defer routerSetup.Stop()` after build.
type routerSetup struct {
	Engine *gin.Engine
	WSHub  *cmdbws.Hub
	Stop   func()
}

// setupRouter builds the production Gin router including every
// middleware layer and the v1 handler set. Pure construction; nothing
// blocks or starts goroutines that aren't owned by the engine itself.
//
// natsBus may be nil (NATS disabled). It's threaded into the
// HealthHandler for the /readyz NATS check; the public-route helper
// short-circuits to "not_configured" when nil.
func setupRouter(
	cfg *config.Config,
	pool *pgxpool.Pool,
	queries *dbgen.Queries,
	redisClient *redis.Client,
	natsBus *eventbus.NATSBus,
	svcs *appServices,
	apiServer *api.APIServer,
) *routerSetup {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	infraMiddleware(router)

	healthHandler := api.NewHealthHandler(pool, redisClient, natsBus)
	registerPublicRoutes(router, healthHandler)

	// API v1 group with auth middleware that skips public endpoints.
	// The blacklist revokes access tokens on logout; PasswordChangedAt
	// invalidates tokens issued before the user last rotated their
	// password.
	v1 := router.Group("/api/v1")
	authMW := middleware.Auth(
		cfg.JWTSecret,
		middleware.WithBlacklist(svcs.blacklist),
		middleware.WithPasswordChangeChecker(svcs.authSvc),
	)
	// Derive the auth-bypass set from the same RBAC config that drives
	// publicPaths. Pre-4.9 this list was a second hardcoded string
	// triple ("login"/"refresh"/"ws") that drifted from rbac.go's
	// publicPaths. AuthBypassPaths returns every RBAC-public path
	// except /auth/logout, which requires a valid access token to
	// revoke its jti.
	authBypass := middleware.AuthBypassPaths()
	v1.Use(func(c *gin.Context) {
		if _, ok := authBypass[c.Request.URL.Path]; ok {
			c.Next()
			return
		}
		authMW(c)
	})

	// Per-IP rate limit for login and refresh only. These endpoints are
	// unauthenticated so we cannot key on user_id (the global limiter
	// below would fall back to IP as well, but expresses its budget
	// per-second rather than per-minute which is the useful granularity
	// for brute-force mitigation). The wrapper ensures the limiter runs
	// ONLY for these two paths and never for the rest of the API surface.
	//
	// Budget: 20/min/IP. Was 5/min, but real users behind shared NAT
	// (office gateway, VPN egress) hit it just by mistyping a password
	// twice — and the frontend showed "invalid credentials" instead of
	// "rate limited", so users assumed their password was wrong and
	// rotated it. 20/min still blocks credential-stuffing (which targets
	// thousands per second) without harming legit retries. Override per
	// environment via LOGIN_RATE_PER_MIN.
	loginLimiter := middleware.NewIPRateLimiter(envIntOr("LOGIN_RATE_PER_MIN", 20))
	loginLimiterMW := loginLimiter.Middleware()
	v1.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/api/v1/auth/login" || path == "/api/v1/auth/refresh" {
			loginLimiterMW(c)
			// When the limiter aborts, c.Next() is a no-op; when it
			// allows, gin will advance to the next middleware after we
			// return. We must NOT call c.Next() here or the chain runs
			// twice.
		}
	})

	// Rate limiter runs after auth so user_id keying beats shared-IP
	// NAT collisions. Returned via routerSetup.Stop so main() can
	// `defer` it without main needing to import middleware.
	var stop func()
	if cfg.RateLimitEnabled {
		rl := middleware.NewRateLimiter(middleware.RateLimiterConfig{
			RequestsPerSecond: cfg.RateLimitRPS,
			Burst:             cfg.RateLimitBurst,
			IdleTTL:           10 * time.Minute,
		})
		stop = rl.Stop
		v1.Use(rl.Middleware())
		zap.L().Info("Rate limiting enabled",
			zap.Float64("rps", cfg.RateLimitRPS),
			zap.Int("burst", cfg.RateLimitBurst))
	}

	v1.Use(middleware.RBAC(queries, redisClient))

	// Register all API routes via generated handler.
	api.RegisterHandlers(v1, apiServer)

	// One-time data migration: draft/pending → submitted (admin-only,
	// not in spec). Kept here rather than as a regular handler because
	// it's a temporary surface that should disappear once every
	// deployment has run it. Discarding the UPDATE error used to let
	// a broken work_orders table report a fake 200 — caller would
	// think the migration succeeded while no rows actually moved.
	v1.POST("/admin/migrate-statuses", func(c *gin.Context) {
		res1, err1 := pool.Exec(c.Request.Context(), "UPDATE work_orders SET status = 'submitted' WHERE status IN ('draft', 'pending')")
		if err1 != nil {
			zap.L().Error("admin migrate-statuses: submitted update failed", zap.Error(err1))
			c.JSON(500, gin.H{"error": "submitted migration failed", "detail": err1.Error()})
			return
		}
		res2, err2 := pool.Exec(c.Request.Context(), "UPDATE work_orders SET status = 'verified' WHERE status = 'closed'")
		if err2 != nil {
			zap.L().Error("admin migrate-statuses: verified update failed", zap.Error(err2))
			c.JSON(500, gin.H{"error": "verified migration failed", "detail": err2.Error()})
			return
		}
		c.JSON(200, gin.H{"migrated_to_submitted": res1.RowsAffected(), "migrated_to_verified": res2.RowsAffected()})
	})

	// WebSocket Hub. Lives here because /ws is a v1-group route
	// (auth middleware is skipped via the bypass set; WSAuth handles
	// the Sec-WebSocket-Protocol token in its place). The hub itself
	// is returned so workers.go can wire the NATS→WS bridge.
	var wsHub *cmdbws.Hub
	if cfg.WSEnabled {
		wsHub = cmdbws.NewHub()
		// Note: hub.Run(ctx) is started in workers.go alongside the
		// other background goroutines so all workers share one
		// "where are the goroutines launched" file.
		wsAuthMW := middleware.WSAuth(cfg.JWTSecret)
		v1.GET("/ws", wsAuthMW, cmdbws.HandleWS(wsHub))
		zap.L().Info("WebSocket /ws route registered")
	}

	return &routerSetup{Engine: router, WSHub: wsHub, Stop: stop}
}
