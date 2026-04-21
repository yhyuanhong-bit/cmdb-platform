package dashboard

import (
	"context"
	"log/slog"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
)

// InvalidationSubscriber evicts cached dashboard stats when domain
// events mutate the underlying counts. Without this the 60-second
// Redis TTL means operators watch a stale dashboard for up to a full
// minute after, say, creating a work order or acknowledging an alert.
//
// Subscribes to every event that can change any of the eight Stats
// fields. Invalidation is best-effort: a Redis failure is logged and
// counted but does not surface as an event-processing error, because
// retrying the whole event just to drop a cache entry would be worse
// than serving a slightly stale stats page.
type InvalidationSubscriber struct {
	svc *Service
	bus eventbus.Bus
	log *slog.Logger
}

// NewInvalidationSubscriber wires a subscriber that drops dashboard
// cache entries for every domain event whose outcome can shift any
// of the eight aggregated fields.
func NewInvalidationSubscriber(svc *Service, bus eventbus.Bus, log *slog.Logger) *InvalidationSubscriber {
	if log == nil {
		log = slog.Default()
	}
	return &InvalidationSubscriber{svc: svc, bus: bus, log: log}
}

// invalidationSubjects is the full set of events that flip one of the
// eight Stats fields. Kept as a package-level var so it's inspectable
// from tests and can't diverge from what Start actually subscribes to.
var invalidationSubjects = []string{
	// assets — TotalAssets, EnergyCurrentKW, AvgQualityScore
	eventbus.SubjectAssetCreated,
	eventbus.SubjectAssetUpdated,
	eventbus.SubjectAssetStatusChanged,
	eventbus.SubjectAssetDeleted,

	// racks — TotalRacks, RackUtilizationPct
	eventbus.SubjectRackCreated,
	eventbus.SubjectRackUpdated,
	eventbus.SubjectRackDeleted,
	eventbus.SubjectRackOccupancyChanged,

	// alerts — CriticalAlerts
	eventbus.SubjectAlertFired,
	eventbus.SubjectAlertResolved,

	// work orders — ActiveOrders, PendingWorkOrders
	eventbus.SubjectOrderCreated,
	eventbus.SubjectOrderUpdated,
	eventbus.SubjectOrderTransitioned,
}

// Start registers the handler against the bus for every subject in
// invalidationSubjects. Safe to call once at boot.
func (i *InvalidationSubscriber) Start() error {
	for _, subject := range invalidationSubjects {
		if err := i.bus.Subscribe(subject, i.onEvent); err != nil {
			return err
		}
	}
	return nil
}

// onEvent is the shared handler for every subscribed subject. It
// parses the tenant, DEL's the cache key, and logs failures.
func (i *InvalidationSubscriber) onEvent(ctx context.Context, event eventbus.Event) error {
	tenantID, err := uuid.Parse(event.TenantID)
	if err != nil || tenantID == uuid.Nil {
		// Events without a tenant (e.g. system-wide signals) don't
		// map to a cache key — silently skip rather than error.
		return nil
	}
	if err := i.svc.Invalidate(ctx, tenantID); err != nil {
		i.log.Warn("dashboard cache invalidate failed",
			"tenant_id", tenantID.String(),
			"subject", event.Subject,
			"err", err)
	}
	return nil
}
