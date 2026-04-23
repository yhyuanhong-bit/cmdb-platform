package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/auth"
	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/audit"
	"github.com/cmdb-platform/cmdb-core/internal/domain/bia"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	location_detect "github.com/cmdb-platform/cmdb-core/internal/domain/location_detect"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
	svcdomain "github.com/cmdb-platform/cmdb-core/internal/domain/service"
	"github.com/cmdb-platform/cmdb-core/internal/domain/sync"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/domain/workflows"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	cmdbmcp "github.com/cmdb-platform/cmdb-core/internal/mcp"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/cache"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	cmdbws "github.com/cmdb-platform/cmdb-core/internal/websocket"
	"github.com/gin-gonic/gin"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
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

	// 4a. Auto-run pending migrations
	{
		migrationsDir := os.Getenv("MIGRATIONS_DIR")
		if migrationsDir == "" {
			migrationsDir = "migrations"
		}
		if _, statErr := os.Stat(migrationsDir); statErr == nil {
			entries, _ := os.ReadDir(migrationsDir)
			for _, entry := range entries {
				if !strings.HasSuffix(entry.Name(), ".up.sql") {
					continue
				}
				// Extract version number
				var version int
				fmt.Sscanf(entry.Name(), "%06d", &version)

				// Check if already applied
				var exists bool
				pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
				if exists {
					continue
				}

				// Apply migration
				sqlBytes, readErr := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
				if readErr != nil {
					zap.L().Warn("migration: failed to read", zap.String("file", entry.Name()), zap.Error(readErr))
					continue
				}
				if _, applyErr := pool.Exec(ctx, string(sqlBytes)); applyErr != nil {
					zap.L().Error("migration: failed to apply", zap.String("file", entry.Name()), zap.Error(applyErr))
					continue
				}
				// A failed schema_migrations row means the migration
				// applied but the tracker didn't advance — on next boot
				// we'd try to apply it again. That's the kind of silent
				// divergence that leaves ops chasing phantom failures.
				if _, insErr := pool.Exec(ctx, "INSERT INTO schema_migrations (version, dirty) VALUES ($1, false) ON CONFLICT DO NOTHING", version); insErr != nil {
					zap.L().Error("migration: failed to record applied version — tracker is out of sync",
						zap.String("file", entry.Name()), zap.Int("version", version), zap.Error(insErr))
				}
				zap.L().Info("migration: applied", zap.String("file", entry.Name()), zap.Int("version", version))
			}
		}
	}

	// 4b. Verify database migration version matches code expectations
	{
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

	// 5. Create dbgen.Queries from the pool
	queries := dbgen.New(pool)

	// 5a. Auto-seed: create default tenant, admin user, and roles if DB is empty
	{
		var userCount int
		// If the user-count probe fails, we can't safely decide whether
		// to seed — re-seeding into a populated DB would stomp an
		// existing admin. Fatal is the correct response: stop startup so
		// ops can diagnose instead of racing into a half-initialized
		// state.
		if probeErr := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&userCount); probeErr != nil {
			zap.L().Fatal("seed: failed to probe users count — cannot safely decide whether to seed", zap.Error(probeErr))
		}
		if userCount == 0 {
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
			} else {
				// Seed file not found — create minimal admin only
				zap.L().Warn("seed file not found, creating minimal admin user", zap.String("path", seedFile))
				adminPassword := os.Getenv("ADMIN_DEFAULT_PASSWORD")
				if adminPassword == "" {
					adminPassword = "admin-" + uuid.New().String()[:8]
				}
				hash, hashErr := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
				if hashErr != nil {
					zap.L().Fatal("seed: failed to hash admin password", zap.Error(hashErr))
				}
				// Each INSERT failure here means the minimal-admin seed
				// never completed. We refuse to continue startup in that
				// case: the operator would otherwise be looking at a
				// half-seeded database with no usable login.
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
				// SECURITY: do NOT log the plaintext password — log aggregators
				// would archive the admin credential. Persist it to a 0600 file
				// and only log the path + username.
				credsPath, credsErr := writeSeedPasswordToFile(adminPassword, "admin")
				if credsErr != nil {
					// Fatal WITHOUT including the password in any field.
					zap.L().Fatal("failed to persist seeded admin password — cannot continue",
						zap.Error(credsErr))
				}
				zap.L().Warn("seed: minimal admin user created — change password immediately; credentials written to file",
					zap.String("username", "admin"),
					zap.String("credentials_file", credsPath))
			}
		}
	}

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

	// 8. Create all services
	// Redis-backed JWT blacklist — revocations (logout, admin-issued) self-
	// expire along with the tokens.
	blacklist := auth.NewBlacklist(redisClient)
	authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret, pool).
		WithBlacklist(blacklist)
	identitySvc := identity.NewService(queries)
	topologySvc := topology.NewService(queries, pool)
	assetSvc := asset.NewService(queries, bus, pool)
	maintenanceSvc := maintenance.NewService(queries, bus, pool)
	monitoringSvc := monitoring.NewService(queries, bus)

	// Alert Rule Evaluator (Phase 2.1 — REMEDIATION-ROADMAP.md). Scans
	// alert_rules every 60s, aggregates TimescaleDB metrics, and emits
	// alert_events rows on threshold breach. Strictly tenant-scoped per
	// rule. Launched as a background goroutine off the server context so
	// SIGTERM stops it within one interval.
	alertEvaluator := monitoring.NewEvaluator(
		queries,
		monitoring.NewPoolAdapter(pool),
		bus,
		monitoring.WithInterval(monitoring.DefaultEvaluatorInterval),
	)
	inventorySvc := inventory.NewService(queries, bus)
	auditSvc := audit.NewService(queries)
	dashboardSvc := dashboard.NewService(queries, pool, redisClient)

	integrationSvc := integration.NewService(queries)
	biaSvc := bia.NewService(queries, pool)
	qualitySvc := quality.NewService(queries, pool)
	discoverySvc := discovery.NewService(queries, pool)

	// Location detection
	locationDetectSvc := location_detect.NewService(pool, bus)

	// Subscribe to MAC table updates from ingestion-engine
	if bus != nil && locationDetectSvc != nil {
		bus.Subscribe("mac_table.updated", func(ctx context.Context, event eventbus.Event) error {
			var payload struct {
				TenantID string `json:"tenant_id"`
				Entries  []struct {
					SwitchAssetID string `json:"switch_asset_id"`
					PortName      string `json:"port_name"`
					MACAddress    string `json:"mac_address"`
				} `json:"entries"`
			}
			if unmarshalErr := json.Unmarshal(event.Payload, &payload); unmarshalErr != nil {
				return nil
			}
			tenantID, parseErr := uuid.Parse(payload.TenantID)
			if parseErr != nil {
				return nil
			}

			var entries []location_detect.MACEntry
			for _, e := range payload.Entries {
				switchID, _ := uuid.Parse(e.SwitchAssetID)
				entries = append(entries, location_detect.MACEntry{
					SwitchAssetID: switchID,
					PortName:      e.PortName,
					MACAddress:    e.MACAddress,
				})
			}

			if len(entries) > 0 {
				locationDetectSvc.UpdateMACCache(ctx, tenantID, entries)
				zap.L().Info("MAC cache updated from SNMP scan", zap.Int("entries", len(entries)))

				// Immediately run location comparison after cache update.
				// Use the server ctx, not context.Background(), so SIGTERM
				// cancels a comparison that was kicked off but not yet
				// completed when shutdown arrives.
				go func() {
					locationDetectSvc.RunDetection(ctx, tenantID)
				}()
			}
			return nil
		})
		zap.L().Info("Subscribed to mac_table.updated events")
	}

	// AI Registry
	aiRegistry := ai.NewRegistry()
	if aiErr := aiRegistry.LoadFromDB(ctx, &ai.QueriesAdapter{Q: queries}); aiErr != nil {
		zap.L().Warn("failed to load AI models", zap.Error(aiErr))
	}

	// Prediction
	predictionSvc := prediction.NewService(queries, aiRegistry)

	// 8b. Sync service
	var syncSvc *sync.Service
	var initialSyncDone atomic.Bool
	initialSyncDone.Store(true) // default: don't block (Central mode)

	if cfg.SyncEnabled && bus != nil {
		// Install the HMAC keyring BEFORE RegisterSubscribers / NewAgent
		// so the first envelope published / received is already covered
		// by the signing path. Unset env = no-op (rollout grace); a
		// misconfigured env (too-short key) is fatal at startup.
		keyRing, keyRingErr := sync.LoadKeyRingFromEnv()
		if keyRingErr != nil {
			zap.L().Fatal("sync HMAC keyring load failed", zap.Error(keyRingErr))
		}
		sync.ConfigureKeyRing(keyRing)
		if keyRing == nil {
			zap.L().Warn("sync HMAC signing disabled — CMDB_SYNC_HMAC_KEY not set; envelopes flow under checksum protection only")
		} else {
			zap.L().Info("sync HMAC signing enabled",
				zap.String("primary_kid", keyRing.PrimaryKID()),
				zap.Strings("previous_kids", keyRing.PreviousKIDs()))
		}

		syncSvc = sync.NewService(pool, bus, cfg)
		syncSvc.RegisterSubscribers()
		syncSvc.StartReconciliation(ctx)
		zap.L().Info("Sync service started")

		if cfg.DeployMode == "edge" && cfg.EdgeNodeID != "" {
			initialSyncDone.Store(false) // Edge: block until sync completes
			agent := sync.NewAgent(pool, bus, cfg)
			agent.InitialSyncDone = &initialSyncDone
			go agent.Start(ctx)
			zap.L().Info("Sync agent started", zap.String("node_id", cfg.EdgeNodeID))
		}
	}

	// Business Service entity (Wave 2). Domain service depends on the
	// sqlc Queries surface + the event bus for CRUD fan-out.
	serviceSvc := svcdomain.New(pool, queries, bus)

	// 9. Create unified API server
	apiServer := api.NewAPIServer(
		pool, cfg, bus, authSvc, identitySvc, topologySvc, assetSvc, maintenanceSvc,
		monitoringSvc, inventorySvc, auditSvc, dashboardSvc, predictionSvc,
		integrationSvc, biaSvc, qualitySvc, discoverySvc, syncSvc, locationDetectSvc,
		serviceSvc, cipher, netGuard,
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

	// 10. Set up Gin router. Middleware chain + public routes live in
	// routes.go to keep main.go focused on process lifecycle. The Wave 1
	// skeleton only moves infra middleware + /healthz /readyz /metrics;
	// the /api/v1 group stays here until Wave 11 because it needs
	// deeper dependency threading.
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	infraMiddleware(router, cfg.DeployMode, &initialSyncDone)

	healthHandler := api.NewHealthHandler(pool, redisClient, natsBus)
	registerPublicRoutes(router, healthHandler)

	// API v1 group with auth middleware that skips public endpoints.
	// The blacklist revokes access tokens on logout; PasswordChangedAt
	// invalidates tokens issued before the user last rotated their password.
	v1 := router.Group("/api/v1")
	authMW := middleware.Auth(
		cfg.JWTSecret,
		middleware.WithBlacklist(blacklist),
		middleware.WithPasswordChangeChecker(authSvc),
	)
	// Derive the auth-bypass set from the same RBAC config that drives
	// publicPaths. Pre-4.9 this list was a second hardcoded string triple
	// ("login"/"refresh"/"ws") that drifted from rbac.go's publicPaths.
	// AuthBypassPaths returns every RBAC-public path except /auth/logout,
	// which requires a valid access token to revoke its jti.
	authBypass := middleware.AuthBypassPaths()
	v1.Use(func(c *gin.Context) {
		if _, ok := authBypass[c.Request.URL.Path]; ok {
			c.Next()
			return
		}
		authMW(c)
	})

	// Per-IP rate limit for login and refresh only. These endpoints are
	// unauthenticated so we cannot key on user_id (the global limiter below
	// would fall back to IP as well, but expresses its budget per-second
	// rather than per-minute which is the useful granularity for brute-force
	// mitigation). The wrapper ensures the limiter runs ONLY for these two
	// paths and never for the rest of the API surface.
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

	// Rate limiter runs after auth so user_id keying beats shared-IP NAT collisions.
	if cfg.RateLimitEnabled {
		rl := middleware.NewRateLimiter(middleware.RateLimiterConfig{
			RequestsPerSecond: cfg.RateLimitRPS,
			Burst:             cfg.RateLimitBurst,
			IdleTTL:           10 * time.Minute,
		})
		defer rl.Stop()
		v1.Use(rl.Middleware())
		zap.L().Info("Rate limiting enabled",
			zap.Float64("rps", cfg.RateLimitRPS),
			zap.Int("burst", cfg.RateLimitBurst))
	}

	v1.Use(middleware.RBAC(queries, redisClient))

	// Register all API routes via generated handler
	api.RegisterHandlers(v1, apiServer)

	// One-time data migration: draft/pending → submitted (admin-only, not in spec).
	// Discarding the UPDATE error used to let a broken work_orders
	// table report a fake 200 — the caller would think the one-time
	// migration succeeded while no rows actually moved.
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

		// Wrap in an http.Server so Shutdown can tear it down on SIGTERM
		// instead of orphaning the listener goroutine.
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
		// When the root ctx is cancelled (SIGTERM), shut the MCP listener
		// down with its own bounded timeout.
		go func() {
			<-ctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			_ = mcpHTTPSrv.Shutdown(shutdownCtx)
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

	// Workflow subscribers (cross-module reactions). Register() wires the
	// event-bus handlers; StartAll spawns every background loop. Phase 4.1
	// consolidated the 8 individual Start* calls behind StartAll — see
	// workflows/start.go for the full list and rationale.
	if bus != nil {
		wfSub := workflows.New(pool, queries, bus, maintenanceSvc, cipher).
			WithQualityScanner(qualitySvc)
		wfSub.Register()
		wfSub.StartAll(ctx)

		// Dashboard cache invalidator. Subscribes to asset/rack/alert/
		// order events so the next GetStats call sees fresh numbers
		// instead of waiting out the 60-second Redis TTL.
		dashInval := dashboard.NewInvalidationSubscriber(dashboardSvc, bus, nil)
		if err := dashInval.Start(); err != nil {
			zap.L().Warn("dashboard invalidation subscribe failed", zap.Error(err))
		}
	}

	// Alert evaluator goroutine. Uses the same server context as every
	// other background worker so a single SIGTERM stops the whole stack.
	// Starts unconditionally (no feature flag) — an empty alert_rules table
	// is a zero-cost scan.
	go alertEvaluator.Start(ctx)
	zap.L().Info("Alert rule evaluator launched", zap.Duration("interval", monitoring.DefaultEvaluatorInterval))

	// Webhook dispatcher. Each fan-out delivery goroutine derives from the
	// server ctx via WithBaseContext, so SIGTERM cancels in-flight retries
	// (including the 1s / 5s backoff sleeps) instead of pinning them until
	// the per-request HTTP timeout fires.
	if bus != nil {
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

// envIntOr reads a positive integer from the named env var, returning
// fallback if the var is unset, non-numeric, or non-positive. Wrong
// values do not brick startup — they fall back with a warning log.
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
