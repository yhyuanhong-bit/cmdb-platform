package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/domain/settings"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// GetAssetLifespanSettings handles GET /settings/asset-lifespan.
//
// Returns the per-asset-type expected useful life (years) for the
// current tenant. Tenants with no row yet receive the canonical
// defaults (server=5, network=7, storage=5, power=10) so the frontend
// always renders something meaningful on first load.
func (s *APIServer) GetAssetLifespanSettings(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	if tenantID == uuid.Nil {
		response.Unauthorized(c, "tenant context missing")
		return
	}

	cfg, err := s.settingsSvc.GetAssetLifespan(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to load asset-lifespan settings")
		return
	}

	merged := cfg.MergedWithDefaults()
	response.OK(c, toAssetLifespanWire(merged))
}

// UpdateAssetLifespanSettings handles PUT /settings/asset-lifespan.
//
// Validates each value is in [1, 30] and upserts the tenant row.
// Records an audit event so config drift is traceable. Returns the
// merged config so the client can refresh its local cache without an
// extra GET round-trip.
func (s *APIServer) UpdateAssetLifespanSettings(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	if tenantID == uuid.Nil {
		response.Unauthorized(c, "tenant context missing")
		return
	}
	userID := userIDFromContext(c)

	var body UpdateAssetLifespanSettingsJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	cfg := fromAssetLifespanWire(body)

	if err := s.settingsSvc.UpdateAssetLifespan(c.Request.Context(), tenantID, cfg, userID); err != nil {
		if errors.Is(err, settings.ErrInvalidLifespan) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update asset-lifespan settings")
		return
	}

	// Re-read to return the merged view (custom + defaults). Cheap:
	// single-row primary-key lookup.
	merged, err := s.settingsSvc.GetAssetLifespan(c.Request.Context(), tenantID)
	if err != nil {
		// The write succeeded, so don't fail the request — return the
		// caller's submitted values, merged with defaults locally.
		merged = cfg.MergedWithDefaults()
	} else {
		merged = merged.MergedWithDefaults()
	}

	// Audit trail. Write happened against a non-tenant-scoped table
	// only conceptually — the row is keyed by tenant_id, so we use
	// tenantID as the target_id so audit queries can filter it cleanly.
	s.recordAudit(c, "tenant_settings.asset_lifespan.updated", "settings", "tenant_settings", tenantID, map[string]any{
		"server":  merged.Server,
		"network": merged.Network,
		"storage": merged.Storage,
		"power":   merged.Power,
	})

	response.OK(c, toAssetLifespanWire(merged))
}

// toAssetLifespanWire converts the domain config into the OpenAPI
// pointer-style wire format. We always return all four fields populated
// so the client doesn't have to know about defaults.
func toAssetLifespanWire(c settings.AssetLifespanConfig) AssetLifespanConfig {
	server := c.Server
	network := c.Network
	storage := c.Storage
	power := c.Power
	return AssetLifespanConfig{
		Server:  &server,
		Network: &network,
		Storage: &storage,
		Power:   &power,
	}
}

// fromAssetLifespanWire converts the OpenAPI struct (pointer fields,
// every field optional) into the domain struct. Nil pointers translate
// to zero, and the service layer treats zero as "leave at default" on
// write.
func fromAssetLifespanWire(w UpdateAssetLifespanSettingsJSONRequestBody) settings.AssetLifespanConfig {
	out := settings.AssetLifespanConfig{}
	if w.Server != nil {
		out.Server = *w.Server
	}
	if w.Network != nil {
		out.Network = *w.Network
	}
	if w.Storage != nil {
		out.Storage = *w.Storage
	}
	if w.Power != nil {
		out.Power = *w.Power
	}
	return out
}
