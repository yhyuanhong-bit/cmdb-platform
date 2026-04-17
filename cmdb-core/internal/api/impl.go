package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/bia"
	"github.com/cmdb-platform/cmdb-core/internal/domain/dashboard"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/cmdb-platform/cmdb-core/internal/domain/inventory"
	location_detect "github.com/cmdb-platform/cmdb-core/internal/domain/location_detect"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/prediction"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
	"github.com/cmdb-platform/cmdb-core/internal/domain/sync"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

// Ensure APIServer implements ServerInterface at compile time.
var _ ServerInterface = (*APIServer)(nil)

// APIServer implements every method of the generated ServerInterface,
// delegating business logic to the domain services.
type APIServer struct {
	pool              *pgxpool.Pool
	cfg               *config.Config
	eventBus          eventbus.Bus
	authSvc           authService
	identitySvc       identityService
	topologySvc       *topology.Service
	assetSvc          *asset.Service
	maintenanceSvc    *maintenance.Service
	monitoringSvc     *monitoring.Service
	inventorySvc      *inventory.Service
	auditSvc          auditService
	dashboardSvc      *dashboard.Service
	predictionSvc     *prediction.Service
	integrationSvc    *integration.Service
	biaSvc            *bia.Service
	qualitySvc        *quality.Service
	discoverySvc      *discovery.Service
	syncSvc           *sync.Service
	locationDetectSvc *location_detect.Service
}

// NewAPIServer constructs an APIServer with all required domain services.
func NewAPIServer(
	pool *pgxpool.Pool,
	cfg *config.Config,
	bus eventbus.Bus,
	authSvc authService,
	identitySvc identityService,
	topologySvc *topology.Service,
	assetSvc *asset.Service,
	maintenanceSvc *maintenance.Service,
	monitoringSvc *monitoring.Service,
	inventorySvc *inventory.Service,
	auditSvc auditService,
	dashboardSvc *dashboard.Service,
	predictionSvc *prediction.Service,
	integrationSvc *integration.Service,
	biaSvc *bia.Service,
	qualitySvc *quality.Service,
	discoverySvc *discovery.Service,
	syncSvc *sync.Service,
	locationDetectSvc *location_detect.Service,
) *APIServer {
	return &APIServer{
		pool:              pool,
		cfg:               cfg,
		eventBus:          bus,
		authSvc:           authSvc,
		identitySvc:       identitySvc,
		topologySvc:       topologySvc,
		assetSvc:          assetSvc,
		maintenanceSvc:    maintenanceSvc,
		monitoringSvc:     monitoringSvc,
		inventorySvc:      inventorySvc,
		auditSvc:          auditSvc,
		dashboardSvc:      dashboardSvc,
		predictionSvc:     predictionSvc,
		integrationSvc:    integrationSvc,
		biaSvc:            biaSvc,
		qualitySvc:        qualitySvc,
		discoverySvc:      discoverySvc,
		syncSvc:           syncSvc,
		locationDetectSvc: locationDetectSvc,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func tenantIDFromContext(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("tenant_id"))
	return id
}

func userIDFromContext(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("user_id"))
	return id
}

func paginationDefaults(page, pageSize *int) (int, int, int32, int32) {
	p := 1
	ps := 20
	if page != nil && *page > 0 {
		p = *page
	}
	if pageSize != nil && *pageSize > 0 {
		ps = *pageSize
		if ps > 100 {
			ps = 100
		}
	}
	offset := (p - 1) * ps
	return p, ps, int32(ps), int32(offset)
}

func uuidPtrFromOAPI(v *openapi_types.UUID) *uuid.UUID {
	if v == nil {
		return nil
	}
	u := uuid.UUID(*v)
	return &u
}

func textFromPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func dateFromPtr(s *string) pgtype.Date {
	if s == nil || *s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func numericFromFloat64Ptr(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", *f))
	return n
}

func int4FromIntPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

func pguuidFromPtr(v *uuid.UUID) pgtype.UUID {
	if v == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *v, Valid: true}
}

func uuidPtrFromPGUUID(pg pgtype.UUID) *uuid.UUID {
	if !pg.Valid {
		return nil
	}
	id := uuid.UUID(pg.Bytes)
	return &id
}

// recordAudit logs an audit event. Errors are logged but don't fail the request.
func (s *APIServer) recordAudit(c *gin.Context, action, module, targetType string, targetID uuid.UUID, diff map[string]any) {
	tenantID := tenantIDFromContext(c)
	operatorID := userIDFromContext(c)
	if s.auditSvc == nil {
		return
	}
	if err := s.auditSvc.Record(c.Request.Context(), tenantID, action, module, targetType, targetID, operatorID, diff, "api"); err != nil {
		// Log but don't fail the request
		zap.L().Error("audit record error", zap.Error(err))
	}
}

// publishEvent publishes a domain event to the event bus. Errors are logged but don't fail the request.
func (s *APIServer) publishEvent(ctx context.Context, subject, tenantID string, payload any) {
	if s.eventBus == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		zap.L().Error("event marshal error", zap.Error(err))
		return
	}
	if err := s.eventBus.Publish(ctx, eventbus.Event{
		Subject:  subject,
		TenantID: tenantID,
		Payload:  data,
	}); err != nil {
		zap.L().Error("event publish error", zap.Error(err))
	}
}

// ---------------------------------------------------------------------------
// CIType soft validation helper
// ---------------------------------------------------------------------------

var assetTypeSchemas = map[string][]string{
	"server":  {"cpu", "memory", "storage", "os"},
	"network": {"ports", "firmware"},
	"storage": {"raw_capacity", "protocol"},
	"power":   {"capacity"},
}

func ciTypeSoftValidation(assetType string, attrs map[string]interface{}) []string {
	schema, ok := assetTypeSchemas[assetType]
	if !ok {
		return nil
	}
	var warnings []string
	if attrs == nil {
		warnings = append(warnings, fmt.Sprintf("type %s recommends attributes: %v", assetType, schema))
	} else {
		for _, field := range schema {
			if _, exists := attrs[field]; !exists {
				warnings = append(warnings, fmt.Sprintf("missing recommended attribute: %s", field))
			}
		}
	}
	return warnings
}
