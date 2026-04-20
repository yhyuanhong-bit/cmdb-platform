package workflows

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// qualityScanner is the subset of quality.Service used by the scheduled
// per-tenant quality scanner (Phase 2.11). Declared at the use site so
// unit tests can substitute a fake without standing up a DB or the full
// quality Service.
type qualityScanner interface {
	ScanTenant(ctx context.Context, tenantID uuid.UUID) error
}

// WorkflowSubscriber handles cross-module reactions to domain events.
type WorkflowSubscriber struct {
	pool           *pgxpool.Pool
	queries        *dbgen.Queries
	bus            eventbus.Bus
	maintenanceSvc *maintenance.Service
	cipher         crypto.Cipher
	qualitySvc     qualityScanner
	// qualityTenantListerOverride is a test seam: when non-nil it
	// replaces w.queries as the source of ListActiveTenants for the
	// scheduled quality scanner. Production paths never set it, so the
	// live *dbgen.Queries path is the only one that runs in a real
	// deployment. Kept unexported so callers cannot accidentally
	// depend on the override from outside the package.
	qualityTenantListerOverride qualityTenantLister
}

// New creates a WorkflowSubscriber.
func New(pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus, maintenanceSvc *maintenance.Service, cipher crypto.Cipher) *WorkflowSubscriber {
	return &WorkflowSubscriber{
		pool:           pool,
		queries:        queries,
		bus:            bus,
		maintenanceSvc: maintenanceSvc,
		cipher:         cipher,
	}
}

// WithQualityScanner injects the quality scanner dependency used by the
// scheduled full-tenant quality scan loop (Phase 2.11). Returns the
// receiver so construction can chain:
//
//	wf := workflows.New(...).WithQualityScanner(qualitySvc)
//
// A nil scanner is accepted and disables the loop (StartQualityScanner
// becomes a no-op); that lets tests or minimal deployments skip the
// feature without wrapping every call site in a nil guard.
func (w *WorkflowSubscriber) WithQualityScanner(qs qualityScanner) *WorkflowSubscriber {
	w.qualitySvc = qs
	return w
}

// Register subscribes to all relevant event subjects.
func (w *WorkflowSubscriber) Register() {
	if w.bus == nil {
		return
	}

	w.bus.Subscribe(eventbus.SubjectOrderTransitioned, w.onOrderTransitioned)
	w.bus.Subscribe("alert.fired", w.onAlertFired)
	w.bus.Subscribe(eventbus.SubjectAssetCreated, w.onAssetCreatedNotify)
	w.bus.Subscribe(eventbus.SubjectInventoryTaskCompleted, w.onInventoryCompletedNotify)
	w.bus.Subscribe(eventbus.SubjectImportCompleted, w.onImportCompletedNotify)
	w.bus.Subscribe(eventbus.SubjectScanDifferencesDetected, w.onScanDifferencesDetected)
	w.bus.Subscribe(eventbus.SubjectBMCDefaultPassword, w.onBMCDefaultPassword)

	zap.L().Info("workflow subscribers registered")
}
