package api

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Integration endpoints
// ---------------------------------------------------------------------------

// ListAdapters returns all integration adapters for the tenant.
// (GET /integration/adapters)
func (s *APIServer) ListAdapters(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	adapters, err := s.integrationSvc.ListAdapters(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list adapters")
		return
	}
	response.OK(c, convertSlice(adapters, toAPIAdapter))
}

// CreateAdapter creates a new integration adapter.
// (POST /integration/adapters)
func (s *APIServer) CreateAdapter(c *gin.Context) {
	var req CreateAdapterJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var configBytes []byte
	if req.Config != nil {
		configBytes, _ = json.Marshal(*req.Config)
	} else {
		configBytes = []byte(`{}`)
	}

	// Dual-write: keep plaintext for one release while readers migrate; the
	// authoritative column is config_encrypted.
	encrypted, err := s.cipher.Encrypt(configBytes)
	if err != nil {
		response.InternalError(c, "failed to encrypt adapter config")
		return
	}

	params := dbgen.CreateAdapterParams{
		TenantID:        tenantID,
		Name:            req.Name,
		Type:            req.Type,
		Direction:       req.Direction,
		Config:          configBytes,
		ConfigEncrypted: encrypted,
	}
	if req.Endpoint != nil {
		params.Endpoint = pgtype.Text{String: *req.Endpoint, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	} else {
		params.Enabled = pgtype.Bool{Bool: true, Valid: true}
	}

	adapter, err := s.integrationSvc.CreateAdapter(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create adapter")
		return
	}
	s.recordAudit(c, "adapter.created", "integration", "integration_adapter", adapter.ID, map[string]any{
		"name":      req.Name,
		"type":      req.Type,
		"direction": req.Direction,
	})
	response.Created(c, toAPIAdapter(adapter))
}

// ListWebhooks returns all webhook subscriptions for the tenant.
// (GET /integration/webhooks)
func (s *APIServer) ListWebhooks(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	webhooks, err := s.integrationSvc.ListWebhooks(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list webhooks")
		return
	}
	response.OK(c, convertSlice(webhooks, toAPIWebhook))
}

// CreateWebhook creates a new webhook subscription.
// (POST /integration/webhooks)
func (s *APIServer) CreateWebhook(c *gin.Context) {
	var req CreateWebhookJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	parsed, err := url.Parse(req.Url)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		response.BadRequest(c, "invalid webhook url: must be http(s) URL with host")
		return
	}

	tenantID := tenantIDFromContext(c)

	params := dbgen.CreateWebhookParams{
		TenantID: tenantID,
		Name:     req.Name,
		Url:      req.Url,
		Events:   req.Events,
	}
	if req.Secret != nil {
		params.Secret = pgtype.Text{String: *req.Secret, Valid: true}
		enc, err := s.cipher.Encrypt([]byte(*req.Secret))
		if err != nil {
			response.InternalError(c, "failed to encrypt webhook secret")
			return
		}
		params.SecretEncrypted = enc
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	} else {
		params.Enabled = pgtype.Bool{Bool: true, Valid: true}
	}

	webhook, err := s.integrationSvc.CreateWebhook(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create webhook")
		return
	}
	s.recordAudit(c, "webhook.created", "integration", "webhook", webhook.ID, map[string]any{
		"name":   req.Name,
		"url":    req.Url,
		"events": req.Events,
	})
	response.Created(c, toAPIWebhook(webhook))
}

// ListWebhookDeliveries returns delivery history for a webhook.
// (GET /integration/webhooks/{id}/deliveries)
func (s *APIServer) ListWebhookDeliveries(c *gin.Context, id IdPath) {
	ctx := c.Request.Context()
	webhookID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	if _, err := s.integrationSvc.GetWebhookByID(ctx, webhookID, tenantID); err != nil {
		response.NotFound(c, "webhook not found")
		return
	}

	deliveries, err := s.integrationSvc.ListDeliveries(ctx, webhookID, 100)
	if err != nil {
		response.InternalError(c, "failed to list deliveries")
		return
	}
	response.OK(c, convertSlice(deliveries, toAPIWebhookDelivery))
}

// UpdateAdapter applies a partial update to an integration adapter.
// (PATCH /integration/adapters/{id})
func (s *APIServer) UpdateAdapter(c *gin.Context, id IdPath) {
	var req UpdateAdapterJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	ctx := c.Request.Context()
	adapterID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	// Ownership check: 404 if the row doesn't belong to this tenant.
	if _, err := s.integrationSvc.GetAdapterByID(ctx, adapterID, tenantID); err != nil {
		response.NotFound(c, "adapter not found")
		return
	}

	params := dbgen.UpdateAdapterParams{
		ID:       adapterID,
		TenantID: tenantID,
	}
	diff := make(map[string]any)

	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
		diff["name"] = *req.Name
	}
	if req.Type != nil {
		params.Type = pgtype.Text{String: *req.Type, Valid: true}
		diff["type"] = *req.Type
	}
	if req.Direction != nil {
		params.Direction = pgtype.Text{String: *req.Direction, Valid: true}
		diff["direction"] = *req.Direction
	}
	if req.Endpoint != nil {
		params.Endpoint = pgtype.Text{String: *req.Endpoint, Valid: true}
		diff["endpoint"] = *req.Endpoint
	}
	if req.Config != nil {
		configBytes, err := json.Marshal(*req.Config)
		if err != nil {
			response.BadRequest(c, "invalid adapter config")
			return
		}
		// Dual-write: plaintext + encrypted stay in sync (see CreateAdapter).
		encrypted, err := s.cipher.Encrypt(configBytes)
		if err != nil {
			response.InternalError(c, "failed to encrypt adapter config")
			return
		}
		params.Config = configBytes
		params.ConfigEncrypted = encrypted
		diff["config"] = "updated"
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
		diff["enabled"] = *req.Enabled
	}

	updated, err := s.integrationSvc.UpdateAdapter(ctx, params)
	if err != nil {
		response.InternalError(c, "failed to update adapter")
		return
	}
	s.recordAudit(c, "adapter.updated", "integration", "integration_adapter", adapterID, diff)
	response.OK(c, toAPIAdapter(updated))
}

// DeleteAdapter removes an integration adapter.
// (DELETE /integration/adapters/{id})
func (s *APIServer) DeleteAdapter(c *gin.Context, id IdPath) {
	ctx := c.Request.Context()
	adapterID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	existing, err := s.integrationSvc.GetAdapterByID(ctx, adapterID, tenantID)
	if err != nil {
		response.NotFound(c, "adapter not found")
		return
	}

	if err := s.integrationSvc.DeleteAdapter(ctx, adapterID, tenantID); err != nil {
		response.InternalError(c, "failed to delete adapter")
		return
	}
	s.recordAudit(c, "adapter.deleted", "integration", "integration_adapter", adapterID, map[string]any{
		"name":      existing.Name,
		"type":      existing.Type,
		"direction": existing.Direction,
	})
	c.Status(http.StatusNoContent)
}

// UpdateWebhook applies a partial update to a webhook subscription.
// (PATCH /integration/webhooks/{id})
func (s *APIServer) UpdateWebhook(c *gin.Context, id IdPath) {
	var req UpdateWebhookJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	ctx := c.Request.Context()
	webhookID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	if _, err := s.integrationSvc.GetWebhookByID(ctx, webhookID, tenantID); err != nil {
		response.NotFound(c, "webhook not found")
		return
	}

	params := dbgen.UpdateWebhookParams{
		ID:       webhookID,
		TenantID: tenantID,
	}
	diff := make(map[string]any)

	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
		diff["name"] = *req.Name
	}
	if req.Url != nil {
		parsed, err := url.Parse(*req.Url)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			response.BadRequest(c, "invalid webhook url: must be http(s) URL with host")
			return
		}
		params.Url = pgtype.Text{String: *req.Url, Valid: true}
		diff["url"] = *req.Url
	}
	if req.Secret != nil {
		params.Secret = pgtype.Text{String: *req.Secret, Valid: true}
		encrypted, err := s.cipher.Encrypt([]byte(*req.Secret))
		if err != nil {
			response.InternalError(c, "failed to encrypt webhook secret")
			return
		}
		params.SecretEncrypted = encrypted
		diff["secret"] = "updated"
	}
	if req.Events != nil {
		params.Events = *req.Events
		diff["events"] = *req.Events
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
		diff["enabled"] = *req.Enabled
	}

	updated, err := s.integrationSvc.UpdateWebhook(ctx, params)
	if err != nil {
		response.InternalError(c, "failed to update webhook")
		return
	}
	s.recordAudit(c, "webhook.updated", "integration", "webhook", webhookID, diff)
	response.OK(c, toAPIWebhook(updated))
}

// DeleteWebhook removes a webhook subscription.
// (DELETE /integration/webhooks/{id})
func (s *APIServer) DeleteWebhook(c *gin.Context, id IdPath) {
	ctx := c.Request.Context()
	webhookID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	existing, err := s.integrationSvc.GetWebhookByID(ctx, webhookID, tenantID)
	if err != nil {
		response.NotFound(c, "webhook not found")
		return
	}

	if err := s.integrationSvc.DeleteWebhook(ctx, webhookID, tenantID); err != nil {
		response.InternalError(c, "failed to delete webhook")
		return
	}
	s.recordAudit(c, "webhook.deleted", "integration", "webhook", webhookID, map[string]any{
		"name":   existing.Name,
		"url":    existing.Url,
		"events": existing.Events,
	})
	c.Status(http.StatusNoContent)
}
