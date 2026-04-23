package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// marshalJSON is a thin wrapper used by publish to keep the import
// surface minimal in that function.
func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }

// Errors exposed to handlers so the HTTP layer can map them to status
// codes without inspecting strings.
var (
	ErrInvalidCode   = errors.New("service code must match ^[A-Z][A-Z0-9_-]{1,63}$")
	ErrInvalidTier   = errors.New("tier must be one of critical / important / normal / low / minor")
	ErrInvalidStatus = errors.New("status must be one of active / deprecated / decommissioned")
	ErrInvalidRole   = errors.New("role must be one of primary / replica / cache / proxy / storage / dependency / component")
	ErrNotFound      = errors.New("service not found")
	ErrDuplicateCode = errors.New("a service with this code already exists in this tenant")
)

// Queries is the narrow interface the domain needs from sqlc. Defining
// it here keeps unit tests independent of the full dbgen surface — they
// can pass a fake implementing only these methods.
type Queries interface {
	CreateService(ctx context.Context, arg dbgen.CreateServiceParams) (dbgen.Service, error)
	GetService(ctx context.Context, arg dbgen.GetServiceParams) (dbgen.Service, error)
	GetServiceByCode(ctx context.Context, arg dbgen.GetServiceByCodeParams) (dbgen.Service, error)
	ListServices(ctx context.Context, arg dbgen.ListServicesParams) ([]dbgen.Service, error)
	CountServices(ctx context.Context, arg dbgen.CountServicesParams) (int64, error)
	UpdateService(ctx context.Context, arg dbgen.UpdateServiceParams) (dbgen.Service, error)
	SoftDeleteService(ctx context.Context, arg dbgen.SoftDeleteServiceParams) error

	AddServiceAsset(ctx context.Context, arg dbgen.AddServiceAssetParams) (dbgen.ServiceAsset, error)
	RemoveServiceAsset(ctx context.Context, arg dbgen.RemoveServiceAssetParams) error
	UpdateServiceAssetRole(ctx context.Context, arg dbgen.UpdateServiceAssetRoleParams) (dbgen.ServiceAsset, error)
	ListServiceAssets(ctx context.Context, arg dbgen.ListServiceAssetsParams) ([]dbgen.ListServiceAssetsRow, error)
	ListServicesForAsset(ctx context.Context, arg dbgen.ListServicesForAssetParams) ([]dbgen.ListServicesForAssetRow, error)
	CountCriticalServiceAssets(ctx context.Context, arg dbgen.CountCriticalServiceAssetsParams) (dbgen.CountCriticalServiceAssetsRow, error)
}

// Service is the business-layer entry point for services and their
// asset membership.
type Service struct {
	pool    *pgxpool.Pool
	queries Queries
	bus     eventbus.Bus
}

// New constructs the domain service. bus may be nil — events just won't
// be published.
func New(pool *pgxpool.Pool, queries Queries, bus eventbus.Bus) *Service {
	return &Service{pool: pool, queries: queries, bus: bus}
}

// validTiers enumerates the tier vocabulary from bia_scoring_rules.
// 'minor' only appears through BIA backfill; new services should use
// 'low' to match the asset BIA vocabulary.
var validTiers = map[string]bool{
	TierCritical: true, TierImportant: true, TierNormal: true,
	TierLow: true, TierMinor: true,
}

var validStatuses = map[string]bool{
	StatusActive: true, StatusDeprecated: true, StatusDecommissioned: true,
}

var validRoles = map[string]bool{
	RolePrimary: true, RoleReplica: true, RoleCache: true,
	RoleProxy: true, RoleStorage: true, RoleDependency: true,
	RoleComponent: true,
}

// Create inserts a new service, validating all enum fields up-front so
// the DB CHECK constraint never gets to reject a typo that the handler
// could have caught.
func (s *Service) Create(ctx context.Context, p CreateParams) (dbgen.Service, error) {
	if !IsValidCode(p.Code) {
		return dbgen.Service{}, ErrInvalidCode
	}
	if p.Tier == "" {
		p.Tier = TierNormal
	}
	if !validTiers[p.Tier] {
		return dbgen.Service{}, ErrInvalidTier
	}

	arg := dbgen.CreateServiceParams{
		TenantID:    p.TenantID,
		Code:        p.Code,
		Name:        p.Name,
		Description: textOrNull(p.Description),
		Tier:        p.Tier,
		OwnerTeam:   textOrNull(p.OwnerTeam),
		Status:      StatusActive,
		Tags:        ensureTags(p.Tags),
		CreatedBy:   uuidOrNull(p.CreatedBy),
	}
	if p.BIAAssessmentID != nil {
		arg.BiaAssessmentID = pgtype.UUID{Bytes: *p.BIAAssessmentID, Valid: true}
	}

	created, err := s.queries.CreateService(ctx, arg)
	if err != nil {
		if isUniqueViolation(err) {
			return dbgen.Service{}, ErrDuplicateCode
		}
		return dbgen.Service{}, fmt.Errorf("create service: %w", err)
	}
	s.publish(ctx, eventbus.SubjectServiceCreated, created.TenantID, map[string]any{
		"service_id": created.ID.String(),
		"tenant_id":  created.TenantID.String(),
		"code":       created.Code,
		"tier":       created.Tier,
	})
	return created, nil
}

// GetByID returns a service by UUID, scoped to tenant.
func (s *Service) GetByID(ctx context.Context, tenantID, id uuid.UUID) (dbgen.Service, error) {
	svc, err := s.queries.GetService(ctx, dbgen.GetServiceParams{ID: id, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.Service{}, ErrNotFound
	}
	return svc, err
}

// List returns services matching the filter. pageSize is capped at 500
// to match other resources.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, tier, status, ownerTeam *string, page, pageSize int) ([]dbgen.Service, int64, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}
	if page <= 0 {
		page = 1
	}
	offset := int32((page - 1) * pageSize)

	tierFilter := strOrEmpty(tier)
	statusFilter := strOrEmpty(status)
	ownerFilter := strOrEmpty(ownerTeam)

	arg := dbgen.ListServicesParams{
		TenantID: tenantID,
		Column2:  tierFilter,
		Column3:  statusFilter,
		Column4:  ownerFilter,
		Limit:    int32(pageSize),
		Offset:   offset,
	}
	items, err := s.queries.ListServices(ctx, arg)
	if err != nil {
		return nil, 0, fmt.Errorf("list services: %w", err)
	}
	total, err := s.queries.CountServices(ctx, dbgen.CountServicesParams{
		TenantID: tenantID,
		Column2:  tierFilter,
		Column3:  statusFilter,
		Column4:  ownerFilter,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count services: %w", err)
	}
	return items, total, nil
}

// Update applies a partial update. Nil fields stay unchanged.
func (s *Service) Update(ctx context.Context, p UpdateParams) (dbgen.Service, error) {
	if p.Tier != nil && !validTiers[*p.Tier] {
		return dbgen.Service{}, ErrInvalidTier
	}
	if p.Status != nil && !validStatuses[*p.Status] {
		return dbgen.Service{}, ErrInvalidStatus
	}

	arg := dbgen.UpdateServiceParams{ID: p.ID, TenantID: p.TenantID}
	if p.Name != nil {
		arg.Name = *p.Name
	}
	if p.Description != nil {
		arg.Description = pgtype.Text{String: *p.Description, Valid: true}
	}
	if p.Tier != nil {
		arg.Tier = *p.Tier
	}
	if p.OwnerTeam != nil {
		arg.OwnerTeam = pgtype.Text{String: *p.OwnerTeam, Valid: true}
	}
	if p.BIAAssessmentID != nil {
		arg.BiaAssessmentID = pgtype.UUID{Bytes: *p.BIAAssessmentID, Valid: true}
	}
	if p.Status != nil {
		arg.Status = *p.Status
	}
	if p.Tags != nil {
		arg.Tags = *p.Tags
	}

	updated, err := s.queries.UpdateService(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.Service{}, ErrNotFound
	}
	if err != nil {
		return dbgen.Service{}, fmt.Errorf("update service: %w", err)
	}
	s.publish(ctx, eventbus.SubjectServiceUpdated, updated.TenantID, map[string]any{
		"service_id": updated.ID.String(),
		"tenant_id":  updated.TenantID.String(),
	})
	return updated, nil
}

// Delete soft-deletes the service. service_assets rows remain per Q2
// sign-off (decommission = historical record).
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.SoftDeleteService(ctx, dbgen.SoftDeleteServiceParams{
		ID: id, TenantID: tenantID,
	}); err != nil {
		return fmt.Errorf("soft delete service: %w", err)
	}
	s.publish(ctx, eventbus.SubjectServiceDeleted, tenantID, map[string]any{
		"service_id": id.String(),
		"tenant_id":  tenantID.String(),
	})
	return nil
}

// ErrAssetNotInTenant is returned when a caller tries to attach an asset
// that belongs to a different tenant than the service. service_assets
// itself has no FK that enforces assets.tenant_id = services.tenant_id —
// the DB allows the insert; tenant isolation has to come from this
// domain-layer check. Without it, a tenant could attach a foreign asset
// to their own service by guessing its UUID.
var ErrAssetNotInTenant = errors.New("asset does not belong to this tenant")

// AddAsset attaches an asset to a service with the given role.
// Verifies both service and asset belong to tenantID before inserting.
func (s *Service) AddAsset(ctx context.Context, tenantID, serviceID, assetID uuid.UUID, role string, isCritical bool, createdBy uuid.UUID) (dbgen.ServiceAsset, error) {
	if role == "" {
		role = RoleComponent
	}
	if !validRoles[role] {
		return dbgen.ServiceAsset{}, ErrInvalidRole
	}
	// Confirm the service exists in this tenant. GetByID returns
	// ErrNotFound for soft-deleted or foreign-tenant services.
	if _, err := s.GetByID(ctx, tenantID, serviceID); err != nil {
		return dbgen.ServiceAsset{}, err
	}
	// Confirm the asset exists in this tenant. Cheap EXISTS query — we
	// don't need the row, just a yes/no. Goes through TenantScoped so
	// the tenantlint guard is happy and the $1 arg is auto-prepended
	// with tenantID (SQL references tenant_id = $1 = tenantID, id = $2
	// = assetID).
	scoped := database.Scope(s.pool, tenantID)
	var exists bool
	if err := scoped.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM assets WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL)`,
		assetID,
	).Scan(&exists); err != nil {
		return dbgen.ServiceAsset{}, fmt.Errorf("verify asset tenancy: %w", err)
	}
	if !exists {
		return dbgen.ServiceAsset{}, ErrAssetNotInTenant
	}
	return s.queries.AddServiceAsset(ctx, dbgen.AddServiceAssetParams{
		ServiceID:  serviceID,
		AssetID:    assetID,
		TenantID:   tenantID,
		Role:       role,
		IsCritical: isCritical,
		CreatedBy:  uuidOrNull(createdBy),
	})
}

// RemoveAsset detaches an asset from a service.
func (s *Service) RemoveAsset(ctx context.Context, tenantID, serviceID, assetID uuid.UUID) error {
	return s.queries.RemoveServiceAsset(ctx, dbgen.RemoveServiceAssetParams{
		ServiceID: serviceID,
		AssetID:   assetID,
		TenantID:  tenantID,
	})
}

// ListAssets returns every asset in a service with role + critical flag.
func (s *Service) ListAssets(ctx context.Context, tenantID, serviceID uuid.UUID) ([]dbgen.ListServiceAssetsRow, error) {
	return s.queries.ListServiceAssets(ctx, dbgen.ListServiceAssetsParams{
		ServiceID: serviceID,
		TenantID:  tenantID,
	})
}

// ServicesForAsset is the reverse-lookup the Asset detail page uses.
func (s *Service) ServicesForAsset(ctx context.Context, tenantID, assetID uuid.UUID) ([]dbgen.ListServicesForAssetRow, error) {
	return s.queries.ListServicesForAsset(ctx, dbgen.ListServicesForAssetParams{
		AssetID:  assetID,
		TenantID: tenantID,
	})
}

// Health aggregates the service's critical assets into a single
// HealthStatus. The computation is:
//   - no critical assets tagged          → unknown (operator hasn't
//     classified what matters yet)
//   - every critical asset operational   → healthy
//   - any critical asset off-operational → degraded
//
// Non-critical assets are intentionally excluded — they can flap
// without the service going degraded, matching reality in any real
// data center.
func (s *Service) Health(ctx context.Context, tenantID, serviceID uuid.UUID) (HealthStatus, int64, int64, error) {
	row, err := s.queries.CountCriticalServiceAssets(ctx, dbgen.CountCriticalServiceAssetsParams{
		ServiceID: serviceID,
		TenantID:  tenantID,
	})
	if err != nil {
		return HealthUnknown, 0, 0, fmt.Errorf("count critical: %w", err)
	}
	total := row.CriticalTotal
	bad := row.CriticalUnhealthy
	switch {
	case total == 0:
		return HealthUnknown, total, bad, nil
	case bad == 0:
		return HealthHealthy, total, bad, nil
	default:
		return HealthDegraded, total, bad, nil
	}
}

// publish emits an event bus message. No-op when bus is nil (allows
// unit tests to construct the service without a NATS dependency).
func (s *Service) publish(ctx context.Context, subject string, tenantID uuid.UUID, payload map[string]any) {
	if s.bus == nil {
		return
	}
	body, err := marshalJSON(payload)
	if err != nil {
		return
	}
	_ = s.bus.Publish(ctx, eventbus.Event{
		Subject:  subject,
		TenantID: tenantID.String(),
		Payload:  body,
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func uuidOrNull(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func ensureTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

// strOrEmpty dereferences an optional string filter; nil becomes "".
// The SQL uses `$n::text = '' OR column = $n` so empty string means
// "no filter".
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// isUniqueViolation checks for PG unique_violation (code 23505) so we
// can map duplicate-code inserts to ErrDuplicateCode.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "23505") || contains(msg, "duplicate key value") || contains(msg, "unique constraint")
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
