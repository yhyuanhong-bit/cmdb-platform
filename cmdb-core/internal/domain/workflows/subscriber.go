package workflows

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/identity"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// schedHealthTracker is the narrow interface the workflow tickers need
// from schedhealth.Tracker. Defining it locally keeps this package
// decoupled from the platform/schedhealth import. nil-safe: the helper
// methods on WorkflowSubscriber check for nil before dispatching.
type schedHealthTracker interface {
	Register(name string, expectedInterval time.Duration)
	Record(name string)
}

// resolveSystemUser returns the per-tenant system user UUID or (uuid.Nil, false)
// on failure. Callers in workflow loops should check the bool and `continue`
// on false rather than falling back to uuid.Nil, which would reinstate the
// FK-violation the whole 000052 migration exists to prevent. The source tag
// routes the suppressed-error counter to the originating workflow module.
func (w *WorkflowSubscriber) resolveSystemUser(ctx context.Context, tenantID uuid.UUID, source string) (uuid.UUID, bool) {
	if w.systemUsers == nil {
		return uuid.Nil, false
	}
	id, err := w.systemUsers.SystemUserID(ctx, tenantID)
	if err != nil {
		zap.L().Warn("system user resolve failed; skipping workflow write",
			zap.String("tenant_id", tenantID.String()),
			zap.String("source", source),
			zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(source, telemetry.ReasonSystemUserUnresolved).Inc()
		return uuid.Nil, false
	}
	return id, true
}

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
	// systemUsers resolves the per-tenant FK-safe 'system' user UUID
	// for workflow-triggered writes that have no real requestor. See
	// migration 000052 and docs/reports/phase4/4.8-operator-id-fk-design-spike.md.
	systemUsers *identity.SystemUserResolver
	// qualityTenantListerOverride is a test seam: when non-nil it
	// replaces w.queries as the source of ListActiveTenants for the
	// scheduled quality scanner. Production paths never set it, so the
	// live *dbgen.Queries path is the only one that runs in a real
	// deployment. Kept unexported so callers cannot accidentally
	// depend on the override from outside the package.
	qualityTenantListerOverride qualityTenantLister
	// tracker reports per-loop heartbeats to the platform scheduler-
	// health registry. nil is allowed; the helper methods short-circuit
	// when unset so tests can omit the wiring.
	tracker schedHealthTracker
}

// New creates a WorkflowSubscriber. The SystemUserResolver is built from
// the same *dbgen.Queries handle so callers don't need to wire it through
// main.go explicitly — auto_workorders and notifications both rely on it
// to satisfy the FK on work_orders.requestor_id.
func New(pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus, maintenanceSvc *maintenance.Service, cipher crypto.Cipher) *WorkflowSubscriber {
	return &WorkflowSubscriber{
		pool:           pool,
		queries:        queries,
		bus:            bus,
		maintenanceSvc: maintenanceSvc,
		cipher:         cipher,
		systemUsers:    identity.NewSystemUserResolver(queries, 0),
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

// WithSchedHealth wires the platform scheduler-health tracker so every
// long-running ticker in this package surfaces on /admin/scheduler-health.
// Returns the receiver so construction can chain. nil is allowed and
// disables registration (the helpers turn into no-ops).
func (w *WorkflowSubscriber) WithSchedHealth(tracker schedHealthTracker) *WorkflowSubscriber {
	w.tracker = tracker
	return w
}

// registerScheduler announces a ticker to the scheduler-health tracker.
// Safe to call when no tracker is wired.
func (w *WorkflowSubscriber) registerScheduler(name string, interval time.Duration) {
	if w.tracker == nil {
		return
	}
	w.tracker.Register(name, interval)
}

// recordTick stamps a heartbeat for the named ticker. Call at the top
// of each tick body so a stuck loop is observable as stale even if it
// hangs mid-work. Safe to call when no tracker is wired.
func (w *WorkflowSubscriber) recordTick(name string) {
	if w.tracker == nil {
		return
	}
	w.tracker.Record(name)
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
