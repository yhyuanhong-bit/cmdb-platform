// services.go — domain-service construction extracted from main.go
// during the Phase 2 God-file split (2026-04-28).
//
// Holds every wired-up service the API server, router, and
// background goroutines depend on. The single struct keeps main()'s
// argument lists (especially api.NewAPIServer's 25-argument call)
// readable and makes the dependency graph easy to scan.
//
// Side effects intentionally kept inside buildServices:
//   - alert evaluator: schedTracker.Register("alert_evaluator", …)
//   - mac_table.updated subscription: bridges ingestion-engine SNMP
//     scans into location_detect.UpdateMACCache
//   - predictive scheduler goroutine: launched here so the wire-up
//     stays adjacent to predictiveSvc construction
package main

import (
	"context"
	"encoding/json"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
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
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/schedhealth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// appServices bundles every domain service the API server, router, and
// background workers need. Built once by buildServices, passed by
// pointer through main(). Adding a new service: add a field, populate
// it in buildServices, and pass it down to wherever it's consumed.
type appServices struct {
	blacklist         *auth.Blacklist
	authSvc           *identity.AuthService
	identitySvc       *identity.Service
	topologySvc       *topology.Service
	assetSvc          *asset.Service
	maintenanceSvc    *maintenance.Service
	monitoringSvc     *monitoring.Service
	alertEvaluator    *monitoring.Evaluator
	inventorySvc      *inventory.Service
	auditSvc          *audit.Service
	dashboardSvc      *dashboard.Service
	integrationSvc    *integration.Service
	biaSvc            *bia.Service
	qualitySvc        *quality.Service
	discoverySvc      *discovery.Service
	locationDetectSvc *location_detect.Service
	aiRegistry        *ai.Registry
	predictionSvc     *prediction.Service
	serviceSvc        *svcdomain.Service
	schedTracker      *schedhealth.Tracker
	predictiveSvc     *predictive.Service
	metricSourceSvc   *metricsource.Service
}

// buildServices wires every domain service from infra primitives.
// Order is significant: services that need each other are built in
// dependency order (authSvc before identitySvc.WithRefreshRevoker,
// schedTracker before alertEvalTrackerForwarder.target = …, etc.).
//
// The alert evaluator is constructed BEFORE the schedTracker exists
// using a lateBoundRecorder forwarder; the forwarder's target is set
// later in this function once the tracker is created. See the
// `alertEvalTrackerForwarder` block + comment for why.
//
// The predictive scheduler goroutine is launched from here so the
// "construct svc + start ticker" pair stays atomic.
func buildServices(
	ctx context.Context,
	cfg *config.Config,
	pool *pgxpool.Pool,
	queries *dbgen.Queries,
	redisClient *redis.Client,
	bus eventbus.Bus,
) *appServices {
	s := &appServices{}

	// Redis-backed JWT blacklist — revocations (logout, admin-issued)
	// self-expire along with the tokens.
	s.blacklist = auth.NewBlacklist(redisClient)

	s.authSvc = identity.NewAuthService(queries, redisClient, cfg.JWTSecret, pool).
		WithBlacklist(s.blacklist)
	// Wire authSvc as the refresh-token revoker so identitySvc.Deactivate
	// invalidates outstanding refresh tokens (audit finding H1, 2026-04-28).
	s.identitySvc = identity.NewService(queries).WithRefreshRevoker(s.authSvc)
	s.topologySvc = topology.NewService(queries, pool)
	s.assetSvc = asset.NewService(queries, bus, pool)
	s.maintenanceSvc = maintenance.NewService(queries, bus, pool)
	s.monitoringSvc = monitoring.NewService(queries, bus, pool)

	// Alert Rule Evaluator (Phase 2.1 — REMEDIATION-ROADMAP.md). Scans
	// alert_rules every 60s, aggregates TimescaleDB metrics, and emits
	// alert_events rows on threshold breach. Strictly tenant-scoped
	// per rule. The schedTracker is created later, so register the
	// evaluator with a once-resolvable forwarder; we set its target
	// after schedTracker exists. See Wave 9.1.
	alertEvalTrackerForwarder := &lateBoundRecorder{}
	s.alertEvaluator = monitoring.NewEvaluator(
		queries,
		monitoring.NewPoolAdapter(pool),
		bus,
		monitoring.WithInterval(monitoring.DefaultEvaluatorInterval),
		// Wave 5.4: high-signal alerts (critical/high/warning) on a
		// known asset auto-spawn or attach to an open incident so
		// operators get a single coordination surface.
		monitoring.WithIncidentBridge(pool),
		// Wave 9.1: scheduler heartbeat for the readiness dashboard.
		monitoring.WithSchedHealth(alertEvalTrackerForwarder, "alert_evaluator"),
	)
	s.inventorySvc = inventory.NewService(queries, bus)
	s.auditSvc = audit.NewService(queries)
	s.dashboardSvc = dashboard.NewService(queries, pool, redisClient)
	s.integrationSvc = integration.NewService(queries)
	s.biaSvc = bia.NewService(queries, pool)
	s.qualitySvc = quality.NewService(queries, pool)
	s.discoverySvc = discovery.NewService(queries, pool)
	s.locationDetectSvc = location_detect.NewService(pool, bus)

	// Subscribe to MAC table updates from ingestion-engine.
	if bus != nil && s.locationDetectSvc != nil {
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
				s.locationDetectSvc.UpdateMACCache(ctx, tenantID, entries)
				zap.L().Info("MAC cache updated from SNMP scan", zap.Int("entries", len(entries)))

				// Immediately run location comparison after cache update.
				// Use the server ctx, not context.Background(), so SIGTERM
				// cancels a comparison that was kicked off but not yet
				// completed when shutdown arrives.
				go func() {
					s.locationDetectSvc.RunDetection(ctx, tenantID)
				}()
			}
			return nil
		})
		zap.L().Info("Subscribed to mac_table.updated events")
	}

	// AI Registry
	s.aiRegistry = ai.NewRegistry()
	if aiErr := s.aiRegistry.LoadFromDB(ctx, &ai.QueriesAdapter{Q: queries}); aiErr != nil {
		zap.L().Warn("failed to load AI models", zap.Error(aiErr))
	}

	s.predictionSvc = prediction.NewService(queries, s.aiRegistry)

	// Business Service entity (Wave 2). Domain service depends on the
	// sqlc Queries surface + the event bus for CRUD fan-out.
	s.serviceSvc = svcdomain.New(pool, queries, bus)

	// Wave 9.1: scheduler-health tracker. Schedulers Record() at the
	// top of each tick so /admin/scheduler-health can detect a stuck
	// loop. Register alert_evaluator now and resolve the forwarder
	// target so the evaluator's Record() calls reach the live tracker.
	s.schedTracker = schedhealth.New()
	s.schedTracker.Register("alert_evaluator", monitoring.DefaultEvaluatorInterval)
	alertEvalTrackerForwarder.target = s.schedTracker

	// Wave 7.1: predictive refresh recommendations. Hourly tick scans
	// every tenant's lifecycle-bearing assets. Idempotent — duplicate
	// ticks are no-ops on top of UPSERT semantics.
	s.predictiveSvc = predictive.NewService(queries, pool)
	go runPredictiveScheduler(ctx, s.predictiveSvc, s.schedTracker)

	// Wave 8.1: metric-source registry. CRUD + heartbeat + freshness.
	// No background scheduler needed — freshness is computed on read.
	s.metricSourceSvc = metricsource.NewService(queries, pool)

	return s
}
