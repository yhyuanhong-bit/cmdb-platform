package workflows

import (
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// WorkflowSubscriber handles cross-module reactions to domain events.
type WorkflowSubscriber struct {
	pool            *pgxpool.Pool
	queries         *dbgen.Queries
	bus             eventbus.Bus
	maintenanceSvc  *maintenance.Service
	cipher          crypto.Cipher
	adapterFailures map[uuid.UUID]int
}

// New creates a WorkflowSubscriber.
func New(pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus, maintenanceSvc *maintenance.Service, cipher crypto.Cipher) *WorkflowSubscriber {
	return &WorkflowSubscriber{
		pool:            pool,
		queries:         queries,
		bus:             bus,
		maintenanceSvc:  maintenanceSvc,
		cipher:          cipher,
		adapterFailures: make(map[uuid.UUID]int),
	}
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
