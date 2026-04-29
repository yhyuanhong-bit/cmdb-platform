package main

//tenantlint:allow-direct-pool — server bootstrap: seed data and schema init run outside request context

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/auth"
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
	location_detect "github.com/cmdb-platform/cmdb-core/internal/domain/location_detect"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/metricsource"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
	"github.com/cmdb-platform/cmdb-core/internal/domain/predictive"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
	svcdomain "github.com/cmdb-platform/cmdb-core/internal/domain/service"
	"github.com/cmdb-platform/cmdb-core/internal/domain/settings"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/domain/workflows"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/cache"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/cmdb-platform/cmdb-core/internal/platform/schedhealth"
	cmdbws "github.com/cmdb-platform/cmdb-core/internal/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// appState holds every resource created during bootstrap.
type appState struct {
	pool          *pgxpool.Pool
	redisClient   *redis.Client
	natsBus       *eventbus.NATSBus
	bus           eventbus.Bus
	queries       *dbgen.Queries
	apiServer     *api.APIServer
	healthHandler *api.HealthHandler
	blacklist     *auth.Blacklist
	authSvc       *identity.AuthService
	schedTracker  *schedhealth.Tracker
	wsHub         *cmdbws.Hub
	netGuard      *netguard.Guard
	rateLimiter   *middleware.RateLimiter
}

// bootstrap initialises every infrastructure dependency and domain service,
// starts all background goroutines, and returns a fully-wired appState.
func bootstrap(ctx context.Context, cfg *config.Config, cipher crypto.Cipher) (*appState, error) {
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	applyMigrations(ctx, pool)
	checkMigrationVersion(ctx, pool)
	queries := dbgen.New(pool)
	seedIfEmpty(ctx, pool)

	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	var bus eventbus.Bus
	natsBus, natsErr := eventbus.NewNATSBus(cfg.NatsURL)
	if natsErr != nil {
		zap.L().Warn("NATS not available, event bus disabled", zap.Error(natsErr))
	} else {
		bus = natsBus
	}

	netGuard, err := netguard.New(nil, cfg.IntegrationAllowedOutboundHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to build netguard: %w", err)
	}
	workflows.SetNetGuard(netGuard)
	zap.L().Info("SSRF outbound guard configured", zap.Int("allow_hosts", len(cfg.IntegrationAllowedOutboundHosts)))

	blacklist := auth.NewBlacklist(redisClient)
	authSvc := identity.NewAuthService(queries, redisClient, cfg.JWTSecret, pool).WithBlacklist(blacklist)
	identitySvc := identity.NewService(queries).WithRefreshRevoker(authSvc)
	topologySvc := topology.NewService(queries, pool)
	assetSvc := asset.NewService(queries, bus, pool)
	maintenanceSvc := maintenance.NewService(queries, bus, pool)
	monitoringSvc := monitoring.NewService(queries, bus, pool)

	alertFwd := &lateBoundRecorder{}
	alertEvaluator := monitoring.NewEvaluator(
		queries, monitoring.NewPoolAdapter(pool), bus,
		monitoring.WithInterval(monitoring.DefaultEvaluatorInterval),
		monitoring.WithIncidentBridge(pool),
		monitoring.WithSchedHealth(alertFwd, "alert_evaluator"),
	)

	inventorySvc := inventory.NewService(queries, bus)
	auditSvc := audit.NewService(queries)
	dashboardSvc := dashboard.NewService(queries, pool, redisClient)
	integrationSvc := integration.NewService(queries)
	biaSvc := bia.NewService(queries, pool)
	qualitySvc := quality.NewService(queries, pool)
	discoverySvc := discovery.NewService(queries, pool)
	locationDetectSvc := location_detect.NewService(pool, bus)
	subscribeLocationDetect(ctx, bus, locationDetectSvc)

	aiRegistry := ai.NewRegistry()
	if aiErr := aiRegistry.LoadFromDB(ctx, &ai.QueriesAdapter{Q: queries}); aiErr != nil {
		zap.L().Warn("failed to load AI models", zap.Error(aiErr))
	}
	predictionSvc := prediction.NewService(queries, aiRegistry)
	serviceSvc := svcdomain.New(pool, queries, bus)
	predictiveSvc := predictive.NewService(queries, pool)
	metricSourceSvc := metricsource.NewService(queries, pool)
	settingsSvc := settings.NewService(pool)

	schedTracker := schedhealth.New()
	schedTracker.Register("alert_evaluator", monitoring.DefaultEvaluatorInterval)
	alertFwd.target = schedTracker

	apiServer := api.NewAPIServer(
		pool, cfg, bus, redisClient, natsBus,
		authSvc, identitySvc, topologySvc, assetSvc, maintenanceSvc,
		monitoringSvc, inventorySvc, auditSvc, dashboardSvc, predictionSvc,
		integrationSvc, biaSvc, qualitySvc, discoverySvc, locationDetectSvc,
		serviceSvc, predictiveSvc, metricSourceSvc, settingsSvc, schedTracker, cipher, netGuard,
	)

	rbacCfg, err := middleware.LoadRBACConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load rbac config: %w", err)
	}
	middleware.ConfigureRBAC(rbacCfg)
	zap.L().Info("rbac config loaded",
		zap.Int("public_paths", len(rbacCfg.PublicPaths)),
		zap.Int("resource_map_entries", len(rbacCfg.ResourceMap)))

	var wsHub *cmdbws.Hub
	if cfg.WSEnabled {
		wsHub = cmdbws.NewHub()
		go wsHub.Run(ctx)
		zap.L().Info("WebSocket hub started")
	}

	go runPredictiveScheduler(ctx, predictiveSvc, schedTracker)
	if bus != nil {
		startWorkflowSubscribers(ctx, pool, queries, bus, maintenanceSvc, qualitySvc, cipher, schedTracker, dashboardSvc)
		startWebhookDispatcher(ctx, queries, cipher, netGuard, bus)
		startNATSWebSocketBridge(bus, wsHub)
	}
	go alertEvaluator.Start(ctx)
	zap.L().Info("Alert rule evaluator launched", zap.Duration("interval", monitoring.DefaultEvaluatorInterval))

	return &appState{
		pool: pool, redisClient: redisClient, natsBus: natsBus, bus: bus,
		queries: queries, apiServer: apiServer,
		healthHandler: api.NewHealthHandler(pool, redisClient, natsBus),
		blacklist: blacklist, authSvc: authSvc,
		schedTracker: schedTracker, wsHub: wsHub, netGuard: netGuard,
	}, nil
}

// applyMigrations runs any pending .up.sql files from the migrations directory.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) {
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	if _, err := os.Stat(dir); err != nil {
		return
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		var ver int
		fmt.Sscanf(e.Name(), "%06d", &ver)
		var exists bool
		pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", ver).Scan(&exists)
		if exists {
			continue
		}
		sql, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			zap.L().Warn("migration: failed to read", zap.String("file", e.Name()), zap.Error(readErr))
			continue
		}
		if _, applyErr := pool.Exec(ctx, string(sql)); applyErr != nil {
			zap.L().Error("migration: failed to apply", zap.String("file", e.Name()), zap.Error(applyErr))
			continue
		}
		if _, insErr := pool.Exec(ctx, "INSERT INTO schema_migrations (version, dirty) VALUES ($1, false) ON CONFLICT DO NOTHING", ver); insErr != nil {
			zap.L().Error("migration: failed to record applied version", zap.String("file", e.Name()), zap.Error(insErr))
		}
		zap.L().Info("migration: applied", zap.String("file", e.Name()), zap.Int("version", ver))
	}
}

// checkMigrationVersion verifies the DB schema version matches the binary.
func checkMigrationVersion(ctx context.Context, pool *pgxpool.Pool) {
	const expected = 50
	var ver int
	if err := pool.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&ver); err != nil {
		zap.L().Fatal("failed to check migration version — is the database initialized?", zap.Error(err))
	}
	if ver < expected {
		zap.L().Fatal("database schema is behind code — run pending migrations before starting the server",
			zap.Int("db_version", ver), zap.Int("expected_version", expected), zap.Int("migrations_behind", expected-ver))
	}
	if ver > expected {
		zap.L().Warn("database schema is ahead of code — is this the right binary?",
			zap.Int("db_version", ver), zap.Int("expected_version", expected))
	}
}

// seedIfEmpty creates default tenant, admin user, and roles when the DB has no users.
func seedIfEmpty(ctx context.Context, pool *pgxpool.Pool) {
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		zap.L().Fatal("seed: failed to probe users count", zap.Error(err))
	}
	if count != 0 {
		return
	}
	zap.L().Info("database is empty — running initial seed")

	seedDir := os.Getenv("SEED_DIR")
	if seedDir == "" {
		seedDir = "db/seed"
	}
	seedFile := filepath.Join(seedDir, "seed.sql")
	if sql, err := os.ReadFile(seedFile); err == nil {
		if _, execErr := pool.Exec(ctx, string(sql)); execErr != nil {
			zap.L().Error("seed: failed to apply", zap.Error(execErr))
		} else {
			zap.L().Info("seed: initial data loaded successfully")
		}
		return
	}

	zap.L().Warn("seed file not found, creating minimal admin user", zap.String("path", seedFile))
	password := os.Getenv("ADMIN_DEFAULT_PASSWORD")
	if password == "" {
		password = "admin-" + uuid.New().String()[:8]
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		zap.L().Fatal("seed: failed to hash admin password", zap.Error(err))
	}
	for _, s := range []struct {
		label string
		sql   string
		args  []any
	}{
		{"tenant", `INSERT INTO tenants (id, name, slug) VALUES ('a0000000-0000-0000-0000-000000000001', 'Default', 'default') ON CONFLICT DO NOTHING`, nil},
		{"user", `INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source) VALUES ('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'admin', 'System Admin', 'admin@example.com', $1, 'active', 'local') ON CONFLICT DO NOTHING`, []any{string(hash)}},
		{"role", `INSERT INTO roles (id, tenant_id, name, description, permissions, is_system) VALUES ('c0000000-0000-0000-0000-000000000001', NULL, 'super-admin', 'Full system access', '{"*": ["*"]}', true) ON CONFLICT DO NOTHING`, nil},
		{"user_role", `INSERT INTO user_roles (user_id, role_id) VALUES ('b0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001') ON CONFLICT DO NOTHING`, nil},
	} {
		if _, seedErr := pool.Exec(ctx, s.sql, s.args...); seedErr != nil {
			zap.L().Fatal("seed: minimal-admin insert failed", zap.String("step", s.label), zap.Error(seedErr))
		}
	}
	credsPath, credsErr := writeSeedPasswordToFile(password, "admin")
	if credsErr != nil {
		zap.L().Fatal("failed to persist seeded admin password", zap.Error(credsErr))
	}
	zap.L().Warn("seed: minimal admin user created — change password immediately",
		zap.String("username", "admin"), zap.String("credentials_file", credsPath))
}

// subscribeLocationDetect wires the MAC-table NATS event to the location detector.
func subscribeLocationDetect(ctx context.Context, bus eventbus.Bus, svc *location_detect.Service) {
	if bus == nil || svc == nil {
		return
	}
	bus.Subscribe("mac_table.updated", func(ctx context.Context, event eventbus.Event) error {
		var payload struct {
			TenantID string `json:"tenant_id"`
			Entries  []struct {
				SwitchAssetID string `json:"switch_asset_id"`
				PortName      string `json:"port_name"`
				MACAddress    string `json:"mac_address"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil
		}
		tenantID, err := uuid.Parse(payload.TenantID)
		if err != nil {
			return nil
		}
		var entries []location_detect.MACEntry
		for _, e := range payload.Entries {
			switchID, _ := uuid.Parse(e.SwitchAssetID)
			entries = append(entries, location_detect.MACEntry{SwitchAssetID: switchID, PortName: e.PortName, MACAddress: e.MACAddress})
		}
		if len(entries) > 0 {
			svc.UpdateMACCache(ctx, tenantID, entries)
			zap.L().Info("MAC cache updated from SNMP scan", zap.Int("entries", len(entries)))
			go func() { svc.RunDetection(ctx, tenantID) }()
		}
		return nil
	})
	zap.L().Info("Subscribed to mac_table.updated events")
}

func startWorkflowSubscribers(ctx context.Context, pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus,
	maintenanceSvc *maintenance.Service, qualitySvc *quality.Service, cipher crypto.Cipher,
	schedTracker *schedhealth.Tracker, dashboardSvc *dashboard.Service) {
	wfSub := workflows.New(pool, queries, bus, maintenanceSvc, cipher).
		WithQualityScanner(qualitySvc).WithSchedHealth(schedTracker)
	wfSub.Register()
	wfSub.StartAll(ctx)
	dashInval := dashboard.NewInvalidationSubscriber(dashboardSvc, bus, nil)
	if err := dashInval.Start(); err != nil {
		zap.L().Warn("dashboard invalidation subscribe failed", zap.Error(err))
	}
}

func startWebhookDispatcher(ctx context.Context, queries *dbgen.Queries, cipher crypto.Cipher, netGuard *netguard.Guard, bus eventbus.Bus) {
	dispatcher := integration.NewWebhookDispatcher(queries, cipher, netGuard).WithEventBus(bus).WithBaseContext(ctx)
	for _, subj := range []string{"asset.>", "maintenance.>", "alert.>", "prediction.>"} {
		subj := subj
		_ = bus.Subscribe(subj, dispatcher.HandleEvent)
	}
	zap.L().Info("Webhook dispatcher active")
}

func startNATSWebSocketBridge(bus eventbus.Bus, wsHub *cmdbws.Hub) {
	if wsHub == nil {
		return
	}
	for _, subj := range []string{"alert.>", "asset.>", "maintenance.>", "import.>", "notification.>"} {
		subj := subj
		bus.Subscribe(subj, func(ctx context.Context, event eventbus.Event) error {
			wsHub.Broadcast(cmdbws.BroadcastMessage{TenantID: event.TenantID, Type: event.Subject, Payload: event.Payload})
			return nil
		})
	}
	zap.L().Info("NATS -> WebSocket bridge active")
}

// runPredictiveScheduler runs the hardware-refresh rule engine on a 1h ticker.
func runPredictiveScheduler(ctx context.Context, svc *predictive.Service, tracker *schedhealth.Tracker) {
	const interval = time.Hour
	const name = "predictive_refresh"
	cfg := predictive.DefaultRuleConfig()
	if tracker != nil {
		tracker.Register(name, interval)
	}
	zap.L().Info("predictive refresh scheduler started", zap.Duration("interval", interval))
	tick := func() {
		if tracker != nil {
			tracker.Record(name)
		}
		res := svc.RunScanTick(ctx, cfg)
		zap.L().Info("predictive refresh tick",
			zap.Int("tenants_scanned", res.TenantsScanned),
			zap.Int("assets_scanned", res.AssetsScanned),
			zap.Int("rows_upserted", res.RowsUpserted),
			zap.Int("errors", len(res.Errors)))
		for _, err := range res.Errors {
			zap.L().Warn("predictive refresh tick error", zap.Error(err))
		}
	}
	tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			zap.L().Info("predictive refresh scheduler stopped")
			return
		case <-t.C:
			tick()
		}
	}
}

// lateBoundRecorder forwards Record() to a tracker that is wired after the
// evaluator is constructed — keeps constructor signatures stable.
type lateBoundRecorder struct{ target *schedhealth.Tracker }

func (r *lateBoundRecorder) Record(name string) {
	if r.target != nil {
		r.target.Record(name)
	}
}
