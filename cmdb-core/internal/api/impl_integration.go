package api

import (
	"encoding/json"
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
